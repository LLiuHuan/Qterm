package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"changeme/internal/ai"
	"changeme/internal/errno"
	"changeme/internal/model"
	"changeme/internal/remoteedit"
)

const (
	agentSftpEntryLimit = 300

	agentSftpTextBytes = 6 << 10

	agentSftpObservationBytes = 6 << 10

	agentSftpPreviewBytes = 64 << 10
)

type agentSftpEntry struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode,omitempty"`
	ModTime string `json:"modTime,omitempty"`
}

type agentSftpListResult struct {
	Path      string           `json:"path"`
	Entries   []agentSftpEntry `json:"entries"`
	Total     int              `json:"total"`
	Truncated bool             `json:"truncated"`
}

func (executor sessionAgentExecutor) executeSftpTool(ctx context.Context, request ai.ToolRequest) (ai.StepResult, error) {
	if executor.sftp == nil || executor.remoteEdits == nil || executor.sessions == nil {
		return ai.StepResult{}, ai.ErrCapabilityUnavailable
	}
	kind, err := executor.sessions.SessionKind(ctx, request.SessionID)
	if err != nil {
		return ai.StepResult{}, err
	}
	if kind != model.SessionKindSSH {
		return ai.StepResult{}, ai.ErrCapabilityUnavailable
	}
	switch request.Kind {
	case ai.StepKindSftpList:
		return executor.sftpListTool(ctx, request)
	case ai.StepKindSftpReadFile:
		return executor.sftpReadFileTool(ctx, request)
	case ai.StepKindSftpWriteFile:
		return executor.sftpWriteFileTool(ctx, request)
	case ai.StepKindSftpUploadFile, ai.StepKindSftpDownloadFile:
		return executor.sftpTransferTool(ctx, request)
	default:
		return ai.StepResult{}, ai.ErrCapabilityUnavailable
	}
}

func (executor sessionAgentExecutor) sftpListTool(ctx context.Context, request ai.ToolRequest) (ai.StepResult, error) {
	path, err := sftpToolPath(request.Args)
	if err != nil {
		return ai.StepResult{}, err
	}
	entries, err := executor.sftp.ListRemoteDirectory(ctx, request.SessionID, path)
	if err != nil {

		return sftpFailureStep(err)
	}
	result := agentSftpListResult{
		Path:    path,
		Total:   len(entries),
		Entries: make([]agentSftpEntry, 0, min(agentSftpEntryLimit, len(entries))),
	}
	for _, entry := range entries {
		if len(result.Entries) >= agentSftpEntryLimit {
			break
		}
		result.Entries = append(result.Entries, projectSftpEntry(entry))
	}
	if len(result.Entries) < len(entries) {
		result.Truncated = true
	}
	encoded, err := marshalSftpListResult(result)
	if err != nil {
		return ai.StepResult{}, err
	}
	return ai.StepResult{Observation: encoded}, nil
}

func (executor sessionAgentExecutor) sftpReadFileTool(ctx context.Context, request ai.ToolRequest) (ai.StepResult, error) {
	path, err := sftpToolPath(request.Args)
	if err != nil {
		return ai.StepResult{}, err
	}
	content, err := executor.remoteEdits.ReadText(ctx, request.SessionID, path)
	if err != nil {
		return sftpFailureStep(err)
	}
	if content.Binary {
		return sftpObservation(struct {
			Error string `json:"error"`
			Path  string `json:"path"`
			Size  int64  `json:"size"`
		}{Error: "binary_file", Path: path, Size: content.Size})
	}
	if content.Truncated {

		return sftpObservation(struct {
			Error string `json:"error"`
			Path  string `json:"path"`
			Size  int64  `json:"size"`
		}{Error: "file_too_large", Path: path, Size: content.Size})
	}
	data, decodeErr := base64.StdEncoding.DecodeString(content.ContentBase64)
	if decodeErr != nil {
		return sftpObservation(struct {
			Error string `json:"error"`
			Path  string `json:"path"`
		}{Error: "decode_failed", Path: path})
	}
	text, textTruncated := truncateTextToBytes(string(data), agentSftpTextBytes)
	return sftpObservation(struct {
		Path      string `json:"path"`
		Content   string `json:"content"`
		Size      int64  `json:"size"`
		Truncated bool   `json:"truncated"`
		Hash      string `json:"hash,omitempty"`
		ModTime   string `json:"modTime,omitempty"`
	}{
		Path:      path,
		Content:   text,
		Size:      content.Size,
		Truncated: textTruncated,
		Hash:      content.Hash,
		ModTime:   formatAgentModTime(content.ModTime),
	})
}

func sftpToolPath(raw json.RawMessage) (string, error) {
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
	return path, nil
}

func projectSftpEntry(entry model.RemoteFileEntry) agentSftpEntry {
	return agentSftpEntry{
		Name:    entry.Name,
		Type:    string(entry.Type),
		Size:    entry.Size,
		Mode:    entry.Mode,
		ModTime: formatAgentModTime(entry.ModTime),
	}
}

func formatAgentModTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func marshalSftpListResult(result agentSftpListResult) (string, error) {
	for {
		encoded, err := json.Marshal(result)
		if err != nil {
			return "", errno.Wrap(errno.CodeUnknown, "ai.agent.tool.encode_failed", err)
		}
		if len(encoded) <= agentSftpObservationBytes || len(result.Entries) == 0 {
			return string(encoded), nil
		}
		result.Entries = result.Entries[:len(result.Entries)-1]
		result.Truncated = true
	}
}

func sftpObservation(value any) (ai.StepResult, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ai.StepResult{}, errno.Wrap(errno.CodeUnknown, "ai.agent.tool.encode_failed", err)
	}
	return ai.StepResult{Observation: string(encoded)}, nil
}

func sftpFailureStep(err error) (ai.StepResult, error) {
	code := errno.CodeUnknown
	messageKey := ""
	if appError := errno.Extract(err); appError != nil {
		code = appError.Code
		messageKey = appError.MessageKey
	}
	result, obsErr := sftpObservation(struct {
		Error      string `json:"error"`
		Code       string `json:"code"`
		MessageKey string `json:"messageKey,omitempty"`
		Hint       string `json:"hint,omitempty"`
	}{
		Error:      "sftp_failed",
		Code:       string(code),
		MessageKey: messageKey,
		Hint:       sftpFailureHint(code),
	})
	if obsErr != nil {
		return ai.StepResult{}, obsErr
	}

	result.Failure = "code=" + string(code)
	if messageKey != "" {
		result.Failure += " key=" + messageKey
	}
	return result, nil
}

func sftpFailureHint(code errno.Code) string {
	switch code {
	case errno.CodePermissionDenied:
		return "the target is not writable; ask the user before trying another path"
	case errno.CodeNotFound:
		return "the remote path does not exist; list the parent directory to confirm it"
	case errno.CodeInvalidArg:
		return "the arguments are invalid; check that the path is absolute and the file is a regular text file"
	case errno.CodeSessionClosed:
		return "the session is no longer connected; ask the user to reconnect"
	default:
		return "the file operation failed; do not retry the same call unchanged"
	}
}

func truncateTextToBytes(value string, maxBytes int) (string, bool) {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value, false
	}
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(value[cut]) {
		cut--
	}
	return value[:cut], true
}

type agentSftpWritePreview struct {
	Path    string `json:"path"`
	OldText string `json:"oldText"`
	NewText string `json:"newText"`

	Created bool `json:"created"`
}

func (executor sessionAgentExecutor) PrepareTool(ctx context.Context, request ai.ToolRequest) (ai.ToolPrep, error) {
	if request.Kind == ai.StepKindSftpUploadFile || request.Kind == ai.StepKindSftpDownloadFile {
		return executor.prepareSftpTransfer(ctx, request)
	}
	if request.Kind != ai.StepKindSftpWriteFile {
		return ai.ToolPrep{}, ai.ErrCapabilityUnavailable
	}

	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(request.Args, &args); err != nil {
		return ai.ToolPrep{}, errno.New(errno.CodeInvalidArg, "ai.agent.tool.invalid_args", "")
	}
	path := strings.TrimSpace(args.Path)
	if path == "" {
		return ai.ToolPrep{}, errno.New(errno.CodeInvalidArg, "ai.agent.tool.invalid_args", "")
	}
	if len(args.Content) > agentSftpPreviewBytes {
		return ai.ToolPrep{}, errno.New(errno.CodeInvalidArg, "ai.agent.tool.write_too_large", "")
	}
	if executor.sessions == nil || executor.remoteEdits == nil {
		return ai.ToolPrep{}, ai.ErrCapabilityUnavailable
	}
	kind, err := executor.sessions.SessionKind(ctx, request.SessionID)
	if err != nil {
		return ai.ToolPrep{}, err
	}
	if kind != model.SessionKindSSH {
		return ai.ToolPrep{}, ai.ErrCapabilityUnavailable
	}

	preview := agentSftpWritePreview{Path: path, NewText: args.Content}
	expectedHash := ""
	existing, readErr := executor.remoteEdits.ReadText(ctx, request.SessionID, path)
	switch {
	case readErr == nil:
		if existing.Binary {
			return ai.ToolPrep{}, errno.New(errno.CodeInvalidArg, "ai.agent.tool.binary_target", "")
		}
		if existing.Truncated || existing.Size > agentSftpPreviewBytes {
			return ai.ToolPrep{}, errno.New(errno.CodeInvalidArg, "ai.agent.tool.write_too_large", "")
		}
		data, decodeErr := base64.StdEncoding.DecodeString(existing.ContentBase64)
		if decodeErr != nil {
			return ai.ToolPrep{}, errno.New(errno.CodeUnknown, "ai.agent.tool.decode_failed", "")
		}
		preview.OldText = string(data)
		expectedHash = existing.Hash
	case remoteedit.IsRemoteMissing(readErr):

		preview.Created = true
	default:
		return ai.ToolPrep{}, readErr
	}

	previewData, err := json.Marshal(preview)
	if err != nil {
		return ai.ToolPrep{}, errno.Wrap(errno.CodeUnknown, "ai.agent.tool.encode_failed", err)
	}
	commitArgs, err := json.Marshal(struct {
		Path         string `json:"path"`
		Content      string `json:"content"`
		ExpectedHash string `json:"expected_hash"`
	}{Path: path, Content: args.Content, ExpectedHash: expectedHash})
	if err != nil {
		return ai.ToolPrep{}, errno.Wrap(errno.CodeUnknown, "ai.agent.tool.encode_failed", err)
	}
	return ai.ToolPrep{
		Preview: ai.ToolPreview{Kind: "sftp_write", Data: previewData},
		Args:    commitArgs,
	}, nil
}

func (executor sessionAgentExecutor) sftpWriteFileTool(ctx context.Context, request ai.ToolRequest) (ai.StepResult, error) {
	var args struct {
		Path         string `json:"path"`
		Content      string `json:"content"`
		ExpectedHash string `json:"expected_hash"`
	}
	if err := json.Unmarshal(request.Args, &args); err != nil {
		return ai.StepResult{}, errno.New(errno.CodeInvalidArg, "ai.agent.tool.invalid_args", "")
	}
	if len(args.Content) > agentSftpPreviewBytes {
		return ai.StepResult{}, errno.New(errno.CodeInvalidArg, "ai.agent.tool.write_too_large", "")
	}
	result, err := executor.remoteEdits.WriteText(
		ctx,
		request.SessionID,
		args.Path,
		base64.StdEncoding.EncodeToString([]byte(args.Content)),
		args.ExpectedHash,
	)
	if err != nil {
		return sftpFailureStep(err)
	}
	if result.Conflict {

		return sftpObservation(struct {
			Error string `json:"error"`
			Path  string `json:"path"`
		}{Error: "conflict", Path: args.Path})
	}
	return sftpObservation(struct {
		Saved bool   `json:"saved"`
		Path  string `json:"path"`
		Bytes int    `json:"bytes"`
	}{Saved: result.Saved, Path: args.Path, Bytes: len(args.Content)})
}

type agentSftpTransferPreview struct {
	Direction  string `json:"direction"`
	LocalPath  string `json:"localPath"`
	RemotePath string `json:"remotePath"`
	Size       int64  `json:"size"`
}

func (executor sessionAgentExecutor) sftpTransferTool(ctx context.Context, request ai.ToolRequest) (ai.StepResult, error) {
	if executor.sftp == nil || executor.sessions == nil {
		return ai.StepResult{}, ai.ErrCapabilityUnavailable
	}
	kind, err := executor.sessions.SessionKind(ctx, request.SessionID)
	if err != nil {
		return ai.StepResult{}, err
	}
	if kind != model.SessionKindSSH {
		return ai.StepResult{}, ai.ErrCapabilityUnavailable
	}
	var args struct {
		LocalPath  string `json:"localPath"`
		RemotePath string `json:"remotePath"`
	}
	if err := json.Unmarshal(request.Args, &args); err != nil {
		return ai.StepResult{}, errno.New(errno.CodeInvalidArg, "ai.agent.tool.invalid_args", "")
	}
	var size int64
	switch request.Kind {
	case ai.StepKindSftpUploadFile:
		size, err = executor.sftp.AgentUploadLocalFile(ctx, request.SessionID, args.LocalPath, args.RemotePath)
	case ai.StepKindSftpDownloadFile:
		size, err = executor.sftp.AgentDownloadRemoteFile(ctx, request.SessionID, args.RemotePath, args.LocalPath)
	default:
		return ai.StepResult{}, ai.ErrCapabilityUnavailable
	}
	if err != nil {

		return sftpFailureStep(err)
	}
	payload, marshalErr := json.Marshal(struct {
		Direction  string `json:"direction"`
		LocalPath  string `json:"localPath"`
		RemotePath string `json:"remotePath"`
		Size       int64  `json:"size"`
	}{
		Direction:  string(request.Kind),
		LocalPath:  args.LocalPath,
		RemotePath: args.RemotePath,
		Size:       size,
	})
	if marshalErr != nil {
		return ai.StepResult{}, marshalErr
	}
	return ai.StepResult{Observation: string(payload)}, nil
}

func (executor sessionAgentExecutor) prepareSftpTransfer(ctx context.Context, request ai.ToolRequest) (ai.ToolPrep, error) {
	var args struct {
		LocalPath  string `json:"localPath"`
		RemotePath string `json:"remotePath"`
	}
	if err := json.Unmarshal(request.Args, &args); err != nil {
		return ai.ToolPrep{}, errno.New(errno.CodeInvalidArg, "ai.agent.tool.invalid_args", "")
	}
	if executor.sftp == nil || executor.sessions == nil {
		return ai.ToolPrep{}, ai.ErrCapabilityUnavailable
	}
	kind, err := executor.sessions.SessionKind(ctx, request.SessionID)
	if err != nil {
		return ai.ToolPrep{}, err
	}
	if kind != model.SessionKindSSH {
		return ai.ToolPrep{}, ai.ErrCapabilityUnavailable
	}
	preview := agentSftpTransferPreview{LocalPath: args.LocalPath, RemotePath: args.RemotePath}
	switch request.Kind {
	case ai.StepKindSftpUploadFile:
		preview.Direction = "upload"
		info, statErr := os.Stat(args.LocalPath)
		if statErr != nil || !info.Mode().IsRegular() {
			return ai.ToolPrep{}, errno.New(errno.CodeNotFound, "ai.agent.tool.local_file_not_found", "")
		}
		preview.Size = info.Size()
	case ai.StepKindSftpDownloadFile:
		preview.Direction = "download"

		entries, listErr := executor.sftp.ListRemoteDirectory(ctx, request.SessionID, path.Dir(args.RemotePath))
		if listErr != nil {
			return ai.ToolPrep{}, listErr
		}
		name := path.Base(args.RemotePath)
		found := false
		for _, entry := range entries {
			if entry.Name == name {
				preview.Size = entry.Size
				found = true
				break
			}
		}
		if !found {
			return ai.ToolPrep{}, errno.New(errno.CodeNotFound, "sftp.remote_path_not_found", "")
		}
	default:
		return ai.ToolPrep{}, ai.ErrCapabilityUnavailable
	}
	data, marshalErr := json.Marshal(preview)
	if marshalErr != nil {
		return ai.ToolPrep{}, marshalErr
	}
	return ai.ToolPrep{Preview: ai.ToolPreview{Kind: "sftp_transfer", Data: data}, Args: request.Args}, nil
}
