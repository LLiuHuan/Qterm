package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"changeme/internal/ai"
	"changeme/internal/errno"
	"changeme/internal/model"
)

const (
	agentLocalEntryLimit = 300

	agentLocalTextBytes = 6 << 10

	agentLocalObservationBytes = 6 << 10

	agentLocalReadLimit = 64 << 10

	agentLocalBinaryProbeBytes = 8 << 10
)

type agentLocalEntry struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Size    int64  `json:"size,omitempty"`
	ModTime string `json:"modTime,omitempty"`
}

type agentLocalListResult struct {
	Path      string            `json:"path"`
	Entries   []agentLocalEntry `json:"entries"`
	Total     int               `json:"total"`
	Truncated bool              `json:"truncated"`
}

func (executor sessionAgentExecutor) executeLocalTool(ctx context.Context, request ai.ToolRequest) (ai.StepResult, error) {
	if executor.sessions == nil {
		return ai.StepResult{}, ai.ErrCapabilityUnavailable
	}
	kind, err := executor.sessions.SessionKind(ctx, request.SessionID)
	if err != nil {
		return ai.StepResult{}, err
	}
	if kind != model.SessionKindLocal {
		return ai.StepResult{}, ai.ErrCapabilityUnavailable
	}
	switch request.Kind {
	case ai.StepKindLocalListDir:
		return executor.localListDir(request)
	case ai.StepKindLocalReadFile:
		return executor.localReadFile(request)
	default:
		return ai.StepResult{}, ai.ErrCapabilityUnavailable
	}
}

func (executor sessionAgentExecutor) localListDir(request ai.ToolRequest) (ai.StepResult, error) {
	path, err := localToolPath(request.Args)
	if err != nil {
		return ai.StepResult{}, err
	}
	if sensitiveLocalPath(path) {
		return localFailureStep(errno.New(errno.CodePermissionDenied, "ai.agent.local.sensitive_path", "")), nil
	}
	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return localFailureStep(localIOError(err)), nil
	}
	result := agentLocalListResult{
		Path:    path,
		Total:   len(dirEntries),
		Entries: make([]agentLocalEntry, 0, min(agentLocalEntryLimit, len(dirEntries))),
	}
	for _, entry := range dirEntries {
		if len(result.Entries) >= agentLocalEntryLimit {
			break
		}
		result.Entries = append(result.Entries, projectLocalEntry(path, entry))
	}
	if len(result.Entries) < len(dirEntries) {
		result.Truncated = true
	}
	encoded, marshalErr := marshalLocalListResult(result)
	if marshalErr != nil {
		return ai.StepResult{}, marshalErr
	}
	return ai.StepResult{Observation: encoded}, nil
}

func (executor sessionAgentExecutor) localReadFile(request ai.ToolRequest) (ai.StepResult, error) {
	path, err := localToolPath(request.Args)
	if err != nil {
		return ai.StepResult{}, err
	}
	if sensitiveLocalPath(path) {
		return localFailureStep(errno.New(errno.CodePermissionDenied, "ai.agent.local.sensitive_path", "")), nil
	}
	info, statErr := os.Stat(path)
	if statErr != nil {
		return localFailureStep(localIOError(statErr)), nil
	}
	if !info.Mode().IsRegular() {
		return localFailureStep(errno.New(errno.CodeInvalidArg, "ai.agent.local.not_regular", "")), nil
	}
	if info.Size() > agentLocalReadLimit {
		return localObservation(struct {
			Error string `json:"error"`
			Path  string `json:"path"`
			Size  int64  `json:"size"`
		}{Error: "file_too_large", Path: path, Size: info.Size()})
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return localFailureStep(localIOError(readErr)), nil
	}
	if looksBinaryBytes(data) {
		return localObservation(struct {
			Error string `json:"error"`
			Path  string `json:"path"`
			Size  int64  `json:"size"`
		}{Error: "binary_file", Path: path, Size: info.Size()})
	}
	text, truncated := truncateTextToBytes(string(data), agentLocalTextBytes)
	return localObservation(struct {
		Path      string `json:"path"`
		Content   string `json:"content"`
		Size      int64  `json:"size"`
		Truncated bool   `json:"truncated"`
		ModTime   string `json:"modTime,omitempty"`
	}{
		Path:      path,
		Content:   text,
		Size:      info.Size(),
		Truncated: truncated,
		ModTime:   formatAgentModTime(info.ModTime()),
	})
}

func localToolPath(raw json.RawMessage) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", errno.New(errno.CodeInvalidArg, "ai.agent.tool.invalid_args", "")
	}
	path := strings.TrimSpace(args.Path)
	if path == "" {
		return "", errno.New(errno.CodeInvalidArg, "ai.agent.tool.invalid_args", "")
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", errno.New(errno.CodeInvalidArg, "ai.agent.tool.invalid_args", "")
		}
		path = filepath.Join(home, path[2:])
	}
	if !filepath.IsAbs(path) {
		return "", errno.New(errno.CodeInvalidArg, "ai.agent.tool.invalid_args", "")
	}
	return filepath.Clean(path), nil
}

func sensitiveLocalPath(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	for _, directory := range []string{"/.ssh", "/.aws", "/.kube", "/.gnupg", "/.docker", "/.config/gh"} {
		if strings.Contains(lower, directory+"/") || strings.HasSuffix(lower, directory) {
			return true
		}
	}
	base := strings.ToLower(filepath.Base(lower))
	switch base {
	case ".netrc", ".git-credentials", ".npmrc", ".pypirc", "credentials.json", "id_rsa", "id_ed25519", "id_ecdsa", "id_dsa":
		return true
	}
	return strings.HasSuffix(base, ".pem") || strings.HasSuffix(base, ".key")
}

func projectLocalEntry(directory string, entry os.DirEntry) agentLocalEntry {
	view := agentLocalEntry{Name: entry.Name(), Type: "other"}
	switch {
	case entry.IsDir():
		view.Type = "directory"
	case entry.Type()&os.ModeSymlink != 0:
		view.Type = "symlink"
	case entry.Type().IsRegular():
		view.Type = "file"
	}
	if info, err := entry.Info(); err == nil {
		view.Size = info.Size()
		view.ModTime = formatAgentModTime(info.ModTime())
	}
	_ = directory
	return view
}

func looksBinaryBytes(data []byte) bool {
	limit := len(data)
	if limit > agentLocalBinaryProbeBytes {
		limit = agentLocalBinaryProbeBytes
	}
	for _, value := range data[:limit] {
		if value == 0 {
			return true
		}
	}
	return false
}

func localIOError(err error) error {
	switch {
	case os.IsNotExist(err):
		return errno.New(errno.CodeNotFound, "ai.agent.local.not_found", "")
	case os.IsPermission(err):
		return errno.New(errno.CodePermissionDenied, "ai.agent.local.permission_denied", "")
	default:
		return errno.Wrap(errno.CodeUnknown, "ai.agent.local.io_failed", err)
	}
}

func localObservation(value any) (ai.StepResult, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ai.StepResult{}, errno.Wrap(errno.CodeUnknown, "ai.agent.tool.encode_failed", err)
	}
	return ai.StepResult{Observation: string(encoded)}, nil
}

func localFailureStep(err error) ai.StepResult {
	code := errno.CodeUnknown
	messageKey := ""
	if appError := errno.Extract(err); appError != nil {
		code = appError.Code
		messageKey = appError.MessageKey
	}
	result, obsErr := localObservation(struct {
		Error      string `json:"error"`
		Code       string `json:"code"`
		MessageKey string `json:"messageKey,omitempty"`
		Hint       string `json:"hint,omitempty"`
	}{
		Error:      "local_failed",
		Code:       string(code),
		MessageKey: messageKey,
		Hint:       localFailureHint(code),
	})
	if obsErr != nil {

		return ai.StepResult{Observation: `{"error":"local_failed"}`, Failure: "code=UNKNOWN"}
	}
	result.Failure = "code=" + string(code)
	if messageKey != "" {
		result.Failure += " key=" + messageKey
	}
	return result
}

func localFailureHint(code errno.Code) string {
	switch code {
	case errno.CodePermissionDenied:
		return "access is denied (or the path holds credentials and is intentionally blocked); ask the user"
	case errno.CodeNotFound:
		return "the local path does not exist; list the parent directory to confirm it"
	case errno.CodeInvalidArg:
		return "the path is not a regular file or is not readable as text"
	default:
		return "the local file operation failed; do not retry the same call unchanged"
	}
}

func marshalLocalListResult(result agentLocalListResult) (string, error) {
	for {
		encoded, err := json.Marshal(result)
		if err != nil {
			return "", errno.Wrap(errno.CodeUnknown, "ai.agent.tool.encode_failed", err)
		}
		if len(encoded) <= agentLocalObservationBytes || len(result.Entries) == 0 {
			return string(encoded), nil
		}
		result.Entries = result.Entries[:len(result.Entries)-1]
		result.Truncated = true
	}
}

func formatLocalTime(value time.Time) string {
	return formatAgentModTime(value)
}
