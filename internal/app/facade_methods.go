package app

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"sort"

	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
	"path/filepath"
	"strings"

	"changeme/internal/ai"
	"changeme/internal/applock"
	"changeme/internal/credential"
	"changeme/internal/errno"
	"changeme/internal/fonts"
	"changeme/internal/host"
	"changeme/internal/hostimport"
	"changeme/internal/model"
	"changeme/internal/session"
	"changeme/internal/sftp"
	"changeme/internal/x11"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func (s *Service) ListTerminalFonts(ctx context.Context, refresh bool) ([]fonts.Family, error) {
	return publicValue(s.fonts.ListTerminalFonts(ctx, refresh))
}

func (s *Service) ListHosts() ([]model.Host, error) {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicValue(s.hosts.ListHosts())
}

func (s *Service) ListGroups() ([]model.Group, error) {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicValue(s.hosts.ListGroups())
}

func (s *Service) ListSessionHistory() ([]model.SessionHistory, error) {
	return publicValue(s.hosts.ListSessionHistory())
}

func (s *Service) SaveSessionHistory(item model.SessionHistory) error {
	return publicError(s.hosts.SaveSessionHistory(item))
}

func (s *Service) ArchiveSessionHistory(id string, archived bool) error {
	return publicError(s.hosts.ArchiveSessionHistory(id, archived))
}

func (s *Service) TrimSessionHistory(limit int) error {
	return publicError(s.hosts.TrimSessionHistory(limit))
}

func (s *Service) SetSessionHistoryLimit(limit int) error {
	return publicError(s.hosts.SetSessionHistoryLimit(limit))
}

func (s *Service) SetSessionHistoryArchived(ids []string, archived bool) (model.SessionHistoryBatchArchiveResult, error) {
	return publicValue(s.hosts.SetSessionHistoryArchived(ids, archived))
}

func (s *Service) DeleteSessionHistory(id string) error {
	return publicError(s.hosts.DeleteSessionHistory(id))
}

func (s *Service) ClearUnarchivedSessionHistory() (model.SessionHistoryClearResult, error) {
	return publicValue(s.hosts.ClearUnarchivedSessionHistory())
}

func (s *Service) ListCommandHistory(hostID string) ([]model.CommandHistory, error) {
	return publicValue(s.hosts.ListCommandHistory(hostID))
}

func (s *Service) SaveCommandHistory(item model.CommandHistory) error {
	return publicError(s.hosts.SaveCommandHistory(item))
}

func (s *Service) DeleteCommandHistory(id string) error {
	return publicError(s.hosts.DeleteCommandHistory(id))
}

func (s *Service) SuppressCommandHistory(hostID, command string) (int64, error) {
	return publicValue(s.hosts.SuppressCommandHistory(hostID, command))
}

func (s *Service) ClearCommandHistory(hostID string) (int64, error) {
	return publicValue(s.hosts.ClearCommandHistory(hostID))
}

func (s *Service) ListTerminalRecordings() ([]model.TerminalRecording, error) {
	return publicValue(s.hosts.ListTerminalRecordings())
}

func (s *Service) ListTerminalRecordingsPage(page, pageSize int, query string) (model.TerminalRecordingPage, error) {
	return publicValue(s.hosts.ListTerminalRecordingsPage(page, pageSize, query))
}

func (s *Service) DeleteTerminalRecording(id string) error {
	return publicError(s.hosts.DeleteTerminalRecording(id))
}

func (s *Service) DeleteTerminalRecordings(ids []string) (model.TerminalRecordingClearResult, error) {
	return publicValue(s.hosts.DeleteTerminalRecordings(ids))
}

func (s *Service) ReadTerminalRecording(id string) ([]model.TerminalRecordingEvent, error) {
	return publicValue(s.hosts.ReadTerminalRecording(id))
}

func (s *Service) UpdateTerminalRecordingNote(id, note string) error {
	return publicError(s.hosts.UpdateTerminalRecordingNote(id, note))
}

func (s *Service) ExportSessionHistoryText(historyID string) (bool, error) {
	if s.fileDialogs == nil {
		return false, publicError(errno.New(errno.CodeStoreFailed, "host.recording_dialog_unavailable", ""))
	}
	target, err := s.fileDialogs.ChooseSessionHistoryTextFile("qterm-session-history.txt")
	if err != nil {
		return false, publicError(err)
	}
	if strings.TrimSpace(target) == "" {
		return false, nil
	}
	if _, err := s.hosts.ExportSessionHistoryText(historyID, target); err != nil {
		return false, publicError(err)
	}
	return true, nil
}

func (s *Service) ExportTerminalRecordings(ids []string, format string) (bool, error) {
	return s.exportTerminalRecordings(ids, format, true)
}

func (s *Service) ExportTerminalRecordingsWithOptions(ids []string, format string, includeOutput bool) (bool, error) {
	return s.exportTerminalRecordings(ids, format, includeOutput)
}

func (s *Service) ExportTerminalRecordingsText(ids []string, filename string) (bool, error) {
	if s.fileDialogs == nil {
		return false, publicError(errno.New(errno.CodeStoreFailed, "host.recording_dialog_unavailable", ""))
	}
	if len(ids) == 0 {
		return false, publicError(errno.New(errno.CodeInvalidArg, "host.recording_ids_required", ""))
	}
	filename = strings.TrimSpace(filename)
	if filename == "" {
		filename = "qterm-recordings.txt"
	}
	target, err := s.fileDialogs.ChooseSessionHistoryTextFile(filename)
	if err != nil {
		return false, publicError(err)
	}
	if strings.TrimSpace(target) == "" {
		return false, nil
	}
	if _, err := s.hosts.ExportTerminalRecordingsText(ids, target); err != nil {
		return false, publicError(err)
	}
	return true, nil
}

func (s *Service) exportTerminalRecordings(ids []string, format string, includeOutput bool) (bool, error) {
	if len(ids) == 0 {
		return false, publicError(errno.New(errno.CodeInvalidArg, "host.recording_ids_required", ""))
	}
	if s.fileDialogs == nil {
		return false, publicError(errno.New(errno.CodeStoreFailed, "host.recording_dialog_unavailable", ""))
	}
	targetDir, err := s.fileDialogs.ChooseTerminalRecordingExportDirectory()
	if err != nil {
		return false, publicError(err)
	}
	if strings.TrimSpace(targetDir) == "" {
		return false, nil
	}
	if err := s.hosts.ExportTerminalRecordingsWithOptions(ids, format, targetDir, includeOutput); err != nil {
		return false, publicError(err)
	}
	return true, nil
}

func (s *Service) StartRecording(ctx context.Context, id string) (model.TerminalRecording, error) {
	return publicValue(s.sessions.StartRecording(ctx, id))
}

func (s *Service) StartFullRecording(ctx context.Context, id string) (model.TerminalRecording, error) {
	return publicValue(s.sessions.StartFullRecording(ctx, id))
}

func (s *Service) StopRecording(ctx context.Context, id string) (model.TerminalRecording, error) {
	return publicValue(s.sessions.StopRecording(ctx, id))
}

func (s *Service) PauseRecording(ctx context.Context, id string) (model.TerminalRecording, error) {
	return publicValue(s.sessions.PauseRecording(ctx, id))
}

func (s *Service) ResumeRecording(ctx context.Context, id string) (model.TerminalRecording, error) {
	return publicValue(s.sessions.ResumeRecording(ctx, id))
}

func (s *Service) ReadActiveRecordingEvents(ctx context.Context, id string, cursor int64) (model.TerminalRecordingEventBatch, error) {
	return publicValue(s.sessions.ReadActiveRecordingEvents(ctx, id, cursor))
}

func (s *Service) AIProviderKeyConfigured(providerID string) bool {
	return s.ai.KeyConfigured(providerID)
}

func (s *Service) SaveAIProviderKey(providerID string, key string) error {
	if err := s.ai.SetKey(providerID, key); err != nil {
		return publicError(errno.New(errno.CodeStoreFailed, "ai.key_save_failed", ""))
	}
	return nil
}

func (s *Service) ClearAIProviderKey(providerID string) error {
	if err := s.ai.ClearKey(providerID); err != nil {
		return publicError(errno.New(errno.CodeStoreFailed, "ai.key_clear_failed", ""))
	}
	return nil
}

type AIModelListInput struct {
	ProviderID string          `json:"providerId"`
	Type       ai.ProviderType `json:"type"`
	BaseURL    string          `json:"baseUrl"`
}

type AIProviderPreviewInput struct {
	ProviderID string          `json:"providerId"`
	Type       ai.ProviderType `json:"type"`
	BaseURL    string          `json:"baseUrl"`
	Model      string          `json:"model"`
	DraftKey   string          `json:"draftKey"`
}

func (s *Service) ListAIModels(ctx context.Context, input AIModelListInput) ([]string, error) {
	models, err := s.ai.ListModels(ctx, ai.Provider{ID: input.ProviderID, Type: input.Type, BaseURL: input.BaseURL})
	if err != nil {
		if errors.Is(err, ai.ErrNotConfigured) {
			return nil, publicError(errno.New(errno.CodeInvalidArg, "ai.not_configured", ""))
		}
		return nil, publicError(aiRequestError(err))
	}
	return models, nil
}

func (s *Service) ListAIModelsPreview(ctx context.Context, input AIProviderPreviewInput) ([]string, error) {
	models, err := s.ai.ListModelsWithTemporaryKey(ctx, ai.Provider{
		ID:      input.ProviderID,
		Type:    input.Type,
		BaseURL: input.BaseURL,
	}, input.DraftKey)
	if err != nil {
		if errors.Is(err, ai.ErrNotConfigured) {
			return nil, publicError(errno.New(errno.CodeInvalidArg, "ai.not_configured", ""))
		}
		return nil, publicError(aiRequestError(err))
	}
	return models, nil
}

type AIChatInput struct {
	ProviderID string          `json:"providerId"`
	Type       ai.ProviderType `json:"type"`
	BaseURL    string          `json:"baseUrl"`
	Model      string          `json:"model"`
	Messages   []ai.Message    `json:"messages"`

	ContextText string `json:"contextText"`
}

func aiChatRequestMessages(input AIChatInput) ([]ai.Message, ai.ContextBreakdown) {
	messages := make([]ai.Message, 0, len(input.Messages)+1)
	if message, ok := ai.ChatContextMessage(input.ContextText); ok {
		messages = append(messages, message)
	}
	messages = append(messages, input.Messages...)
	return messages, ai.MeasureChatContext(messages, 0)
}

type AIChatResult struct {
	Reply string `json:"reply"`

	ErrorKey string `json:"errorKey,omitempty"`

	ErrorDetail string `json:"errorDetail,omitempty"`
}

const aiProviderTestPrompt = "Reply with OK only."

func (s *Service) TestAIProvider(ctx context.Context, input AIProviderPreviewInput) (AIChatResult, error) {
	reply, err := s.ai.ChatWithTemporaryKey(ctx, ai.ChatRequest{
		Provider: ai.Provider{
			ID:      input.ProviderID,
			Type:    input.Type,
			BaseURL: input.BaseURL,
			Model:   input.Model,
		},
		Messages: []ai.Message{{Role: "user", Content: aiProviderTestPrompt}},
	}, input.DraftKey)
	if err != nil {
		if errors.Is(err, ai.ErrNotConfigured) {
			return AIChatResult{ErrorKey: "ai.not_configured"}, nil
		}
		key, detail := aiFailure(err)
		return AIChatResult{ErrorKey: key, ErrorDetail: detail}, nil
	}
	return AIChatResult{Reply: reply}, nil
}

func (s *Service) AIChat(ctx context.Context, input AIChatInput) (AIChatResult, error) {
	messages, breakdown := aiChatRequestMessages(input)

	slog.Info("ai chat request",
		"event", "ai.chat_request",
		"provider", input.ProviderID,
		"type", input.Type,
		"model", input.Model,
		"messages", len(messages),
		"promptBytes", breakdown.Total(),
	)
	reply, err := s.ai.Chat(ctx, ai.ChatRequest{
		Provider: ai.Provider{
			ID:      input.ProviderID,
			Type:    input.Type,
			BaseURL: input.BaseURL,
			Model:   input.Model,
		},
		Messages: messages,
		Context:  breakdown,
	})
	if err != nil {
		if errors.Is(err, ai.ErrNotConfigured) {
			return AIChatResult{ErrorKey: "ai.not_configured"}, nil
		}
		key, detail := aiFailure(err)
		return AIChatResult{ErrorKey: key, ErrorDetail: detail}, nil
	}
	return AIChatResult{Reply: reply}, nil
}

func aiFailure(cause error) (string, string) {
	var status *ai.StatusError
	if errors.As(cause, &status) {
		slog.Error("ai provider request failed",
			"event", "ai.chat_failed",
			"status", status.Status,
			"detail", status.Detail,
			"err", cause,
		)
		switch {
		case status.Status == 401:
			return "ai.http_401", status.Detail
		case status.Status == 403:
			return "ai.http_403", status.Detail
		case status.Status == 404:
			return "ai.http_404", status.Detail
		case status.Status == 429:
			return "ai.http_429", status.Detail
		case status.Status >= 500:
			return "ai.http_5xx", status.Detail
		default:
			return "ai.http_4xx", status.Detail
		}
	}
	slog.Error("ai provider request failed", "event", "ai.chat_failed", "err", cause)
	return "ai.network_failed", ""
}

func aiRequestError(cause error) error {
	var status *ai.StatusError
	if errors.As(cause, &status) {
		slog.Error("ai provider request failed",
			"event", "ai.chat_failed",
			"status", status.Status,
			"detail", status.Detail,
			"err", cause,
		)
		switch {
		case status.Status == 401:
			return errno.New(errno.CodePermissionDenied, "ai.http_401", "")
		case status.Status == 403:
			return errno.New(errno.CodePermissionDenied, "ai.http_403", "")
		case status.Status == 404:
			return errno.New(errno.CodeNotFound, "ai.http_404", "")
		case status.Status == 429:
			return errno.New(errno.CodeUnknown, "ai.http_429", "")
		case status.Status >= 500:
			return errno.New(errno.CodeUnknown, "ai.http_5xx", "")
		default:
			return errno.New(errno.CodeInvalidArg, "ai.http_4xx", "")
		}
	}
	slog.Error("ai provider request failed",
		"event", "ai.chat_failed",
		"err", cause,
	)
	return errno.New(errno.CodeUnknown, "ai.network_failed", "")
}

type HostImportSourceInfo struct {
	ID string `json:"id"`

	Label string `json:"label"`

	DefaultPath string `json:"defaultPath"`

	PathKind string `json:"pathKind"`

	Detected bool `json:"detected"`
}

func (s *Service) ListHostImportSources() []HostImportSourceInfo {
	sshConfig, _ := hostimport.DefaultSSHConfigPath()
	secureCRT := hostimport.DefaultSecureCRTSessionsPath()
	putty := hostimport.DefaultPuttyRegPath()
	termius := hostimport.DefaultTermiusPath()
	sources := []HostImportSourceInfo{

		{ID: "xshell", Label: "Xshell", DefaultPath: hostimport.DefaultXshellPath(), PathKind: "file"},
		{ID: "mobaxterm", Label: "MobaXterm", DefaultPath: hostimport.DefaultMobaXtermPath(), PathKind: "file"},
		{ID: "windterm", Label: "WindTerm", DefaultPath: hostimport.DefaultWindTermPath(), PathKind: "file"},
		{ID: "securecrt", Label: "SecureCRT", DefaultPath: secureCRT, PathKind: "directory"},

		{ID: "securecrt-xml", Label: "SecureCRT", DefaultPath: hostimport.DefaultSecureCRTXMLPath(), PathKind: "file"},

		{ID: "finalshell", Label: "FinalShell", DefaultPath: "", PathKind: "directory"},

		{ID: "termius", Label: "Termius", DefaultPath: hostimport.DefaultTermiusDBPath(), PathKind: "directory"},
		{ID: "electerm", Label: "Electerm", DefaultPath: hostimport.DefaultElectermPath(), PathKind: "file"},
		{ID: "putty", Label: "PuTTY", DefaultPath: putty, PathKind: "file"},
		{ID: "json", Label: "JSON", DefaultPath: termius, PathKind: "file"},
		{ID: "sshconfig", Label: "SSH config", DefaultPath: sshConfig, PathKind: "file"},
	}
	for index := range sources {
		if sources[index].DefaultPath != "" {
			if _, err := os.Stat(sources[index].DefaultPath); err == nil {
				sources[index].Detected = true
			}
		}
	}
	return sources
}

func (s *Service) PickHostImportPath(source string) (string, error) {
	if s == nil || s.fileDialogs == nil {
		return "", publicError(errno.New(errno.CodeUnknown, "import.dialog_unavailable", ""))
	}
	kind := "file"
	label := source
	for _, item := range s.ListHostImportSources() {
		if item.ID == source {
			kind = item.PathKind
			label = item.Label
			break
		}
	}
	var (
		path string
		err  error
	)
	if kind == "directory" {
		path, err = s.fileDialogs.ChooseHostImportDirectory("选择 " + label + " 数据目录")
	} else {
		path, err = s.fileDialogs.ChooseHostImportFile("选择 " + label + " 配置文件")
	}
	if err != nil {
		return "", publicError(errno.New(errno.CodeStoreFailed, "import.dialog_failed", ""))
	}
	return path, nil
}

func dumpHostImportDebug(stage string, entries []hostimport.Entry) {
	if os.Getenv("QTERM_IMPORT_DEBUG") != "1" {
		return
	}
	path := filepath.Join(os.TempDir(), "qterm-import-debug.ndjson")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	for _, entry := range entries {
		_ = encoder.Encode(map[string]any{
			"stage":    stage,
			"alias":    entry.Alias,
			"hostName": entry.HostName,
			"user":     entry.User,
			"port":     entry.Port,
			"group":    entry.Group,
			"line":     entry.Line,
		})
	}
}

func (s *Service) PreviewHostImport(source string, path string) (hostimport.Result, error) {
	return s.PreviewHostImportWithSecret(source, path, "")
}

func (s *Service) PreviewHostImportWithSecret(source string, path string, secret string) (hostimport.Result, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		switch source {
		case "sshconfig":
			path, _ = hostimport.DefaultSSHConfigPath()
		case "putty":
			path = hostimport.DefaultPuttyRegPath()
		case "termius":
			path = hostimport.DefaultTermiusDBPath()
		case "json":
			path = hostimport.DefaultTermiusPath()
		case "securecrt":
			path = hostimport.DefaultSecureCRTSessionsPath()
		case "xshell":
			path = hostimport.DefaultXshellPath()
		case "mobaxterm":
			path = hostimport.DefaultMobaXtermPath()
		case "windterm":
			path = hostimport.DefaultWindTermPath()
		case "electerm":
			path = hostimport.DefaultElectermPath()
		case "securecrt-xml":
			path = hostimport.DefaultSecureCRTXMLPath()
		default:
			return hostimport.Result{}, publicError(errno.New(errno.CodeInvalidArg, "import.source_unsupported", ""))
		}
	}
	var (
		result hostimport.Result
		err    error
	)
	switch source {
	case "sshconfig":
		result, err = hostimport.ParseSSHConfigFile(path)
	case "putty":
		result, err = hostimport.ParsePuttyFile(path)
	case "windterm":
		result, err = hostimport.ParseWindTermWithMasterPassword(path, secret)
	case "termius":
		result, err = hostimport.ParseTermiusDB(path)
	case "json":

		result, err = hostimport.ParseNyaTermJSONFile(path)
	case "securecrt":
		result, err = hostimport.ParseSecureCRTSessions(path)
	case "xshell":
		result, err = hostimport.ParseXshell(path)
	case "mobaxterm":
		result, err = hostimport.ParseMobaXterm(path)
	case "finalshell":
		result, err = hostimport.ParseFinalShell(path)
	case "electerm":
		result, err = hostimport.ParseElecterm(path)
	case "securecrt-xml":
		result, err = hostimport.ParseSecureCRTXML(path)
	default:
		return hostimport.Result{}, publicError(errno.New(errno.CodeInvalidArg, "import.source_unsupported", ""))
	}
	if err != nil {
		if errors.Is(err, hostimport.ErrImportLocked) {
			return hostimport.Result{}, publicError(errno.New(errno.CodePermissionDenied, "import.source_locked", ""))
		}
		if errors.Is(err, hostimport.ErrTermiusDBMissing) || errors.Is(err, hostimport.ErrTermiusKeyMissing) {
			return hostimport.Result{}, publicError(errno.New(errno.CodePermissionDenied, "import.termius_key_unavailable", ""))
		}

		if errors.Is(err, hostimport.ErrImportLocked) {
			return hostimport.Result{}, publicError(errno.New(errno.CodePermissionDenied, "import.source_locked", ""))
		}
		if errors.Is(err, hostimport.ErrConfigNotFound) {
			return hostimport.Result{}, publicError(errno.New(errno.CodeNotFound, "import.source_not_found", ""))
		}
		return hostimport.Result{}, publicError(errno.New(errno.CodeStoreFailed, "import.source_parse_failed", ""))
	}
	existing, listErr := s.hosts.ListHosts()
	if listErr != nil {
		return hostimport.Result{}, publicError(listErr)
	}
	for index := range result.Entries {
		entry := &result.Entries[index]
		for _, host := range existing {
			if strings.EqualFold(host.Address, entry.HostName) && host.Port == entry.Port &&
				strings.EqualFold(host.Username, entry.User) {
				entry.Duplicate = true
				break
			}
		}
	}
	dumpHostImportDebug("preview:"+source, result.Entries)
	return result, nil
}

type ImportHostsInput struct {
	Entries []hostimport.Entry `json:"entries"`
}

type ImportHostsResult struct {
	Created int `json:"created"`
	Failed  int `json:"failed"`
}

func (s *Service) ImportHosts(input ImportHostsInput) (ImportHostsResult, error) {
	result := ImportHostsResult{}
	dumpHostImportDebug("import", input.Entries)

	resolver := newImportGroupResolver(s)
	for _, entry := range input.Entries {

		if looksLikeInternalGroupRef(entry.Alias) || strings.HasPrefix(strings.ToLower(strings.TrimSpace(entry.Alias)), "host_") {
			entry.Alias = strings.TrimSpace(entry.HostName)
		}
		if strings.TrimSpace(entry.Alias) == "" {
			result.Failed++
			continue
		}
		hostInput := entry.HostInput()
		if looksLikeInternalGroupRef(entry.Group) {

			entry.Group = ""
		}

		groupID := resolver.resolve(entry.Group)
		hostInput.Group = ""
		created, err := s.CreateHost(hostInput)
		if err != nil {
			result.Failed++
			continue
		}
		if groupID != "" {
			if err := s.hosts.MoveHost(created.ID, groupID); err != nil {

				result.Failed++
				continue
			}
		}
		result.Created++
	}
	if result.Created == 0 && result.Failed == 0 {
		return result, publicError(errno.New(errno.CodeInvalidArg, "import.nothing_selected", ""))
	}
	return result, nil
}

type importGroupResolver struct {
	service *Service
	loaded  bool
	byID    map[string]model.Group
	byKey   map[string]string
}

func newImportGroupResolver(service *Service) *importGroupResolver {
	return &importGroupResolver{
		service: service,
		byID:    make(map[string]model.Group, 16),
		byKey:   make(map[string]string, 16),
	}
}

func groupKey(parentID, name string) string {
	return parentID + "\x00" + strings.ToLower(strings.TrimSpace(name))
}

func (r *importGroupResolver) load() {
	if r.loaded {
		return
	}
	r.loaded = true
	groups, err := r.service.hosts.ListGroups()
	if err != nil {
		return
	}
	for _, group := range groups {
		r.remember(group)
	}
}

func (r *importGroupResolver) remember(group model.Group) {
	if group.ID == "" {
		return
	}
	r.byID[group.ID] = group
	r.byKey[groupKey(group.ParentID, group.Name)] = group.ID
}

func (r *importGroupResolver) resolve(path string) string {
	r.load()
	segments := splitGroupPath(path)
	if len(segments) == 0 {
		return ""
	}
	parentID := ""
	for _, segment := range segments {
		if looksLikeInternalGroupRef(segment) {

			continue
		}
		if id, ok := r.byKey[groupKey(parentID, segment)]; ok {
			parentID = id
			continue
		}
		created, err := r.service.CreateGroup(model.GroupInput{Name: segment, ParentID: parentID})
		if err != nil {

			fallback := strings.Join(segments, "/")
			if id, ok := r.byKey[groupKey("", fallback)]; ok {
				return id
			}
			created, err = r.service.CreateGroup(model.GroupInput{Name: fallback})
			if err != nil {
				return ""
			}
		}
		r.remember(created)
		parentID = created.ID
	}
	return parentID
}

func splitGroupPath(path string) []string {
	raw := strings.Split(path, "/")
	segments := make([]string, 0, len(raw))
	for _, segment := range raw {
		if trimmed := strings.TrimSpace(segment); trimmed != "" {
			segments = append(segments, trimmed)
		}
	}
	return segments
}

func (s *Service) CreateHost(input model.HostInput) (model.Host, error) {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicValue(s.hosts.CreateHost(input))
}

func (s *Service) UpdateHost(id string, input model.HostInput) (model.Host, error) {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicValue(s.hosts.UpdateHost(id, input))
}

func (s *Service) DeleteHost(id string) error {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicError(s.hosts.DeleteHost(id))
}

func (s *Service) ReorderHost(id, groupID, targetID, position string) error {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicError(s.hosts.ReorderHost(id, groupID, targetID, position))
}

func (s *Service) MoveHost(id, groupID string) error {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicError(s.hosts.MoveHost(id, groupID))
}

func (s *Service) ReorderGroup(id, parentID, targetID, position string) error {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicError(s.hosts.ReorderGroup(id, parentID, targetID, position))
}

func (s *Service) MoveGroup(id, parentID string) error {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicError(s.hosts.MoveGroup(id, parentID))
}

func (s *Service) CreateGroup(input model.GroupInput) (model.Group, error) {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicValue(s.hosts.CreateGroup(input))
}

func (s *Service) UpdateGroup(id string, input model.GroupInput) (model.Group, error) {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicValue(s.hosts.UpdateGroup(id, input))
}

func (s *Service) DeleteGroup(id string) error {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicError(s.hosts.DeleteGroup(id))
}

func (s *Service) CredentialStatus() (credential.CredentialVaultStatus, error) {
	return publicValue(s.credentials.CredentialStatus())
}

func (s *Service) SetupMasterPassword(
	ctx context.Context,
	input credential.SetupMasterPasswordInput,
) (credential.CredentialVaultStatus, error) {
	return publicValue(s.credentials.SetupMasterPassword(ctx, input))
}

func (s *Service) UnlockWithPassword(
	ctx context.Context,
	password string,
) (credential.CredentialVaultStatus, error) {
	return publicValue(s.credentials.UnlockWithPassword(ctx, password))
}

func (s *Service) UnlockAutomatically(ctx context.Context) (credential.CredentialVaultStatus, error) {
	status, err := s.credentials.UnlockAutomatically(ctx)
	if err == nil && status.State == credential.CredentialVaultStateUnlocked {
		if startErr := s.hosts.Start(ctx); startErr != nil {
			return status, publicError(startErr)
		}
	}
	return publicValue(status, err)
}

func (s *Service) ChangeEncryptionKey(
	ctx context.Context,
	input credential.ChangeEncryptionKeyInput,
) (credential.CredentialVaultStatus, error) {
	return publicValue(s.credentials.ChangeEncryptionKey(ctx, input))
}

func (s *Service) UnlockWithCustomKey(ctx context.Context, key string) (credential.CredentialVaultStatus, error) {
	status, err := s.credentials.UnlockWithCustomKey(ctx, key)
	if err == nil && status.State == credential.CredentialVaultStateUnlocked {
		if startErr := s.hosts.Start(ctx); startErr != nil {
			return status, publicError(startErr)
		}
	}
	return publicValue(status, err)
}

func (s *Service) ChangeMasterPassword(
	ctx context.Context,
	input credential.ChangeMasterPasswordInput,
) (credential.CredentialVaultStatus, error) {
	return publicValue(s.credentials.ChangeMasterPassword(ctx, input))
}

func (s *Service) SetAutoUnlock(
	ctx context.Context,
	input credential.SetAutoUnlockInput,
) (credential.CredentialVaultStatus, error) {
	return publicValue(s.credentials.SetAutoUnlock(ctx, input))
}

func (s *Service) LockCredentialVault() error {

	if s.hosts != nil {
		if err := s.hosts.Lock(); err != nil {
			return publicError(err)
		}
	}
	return publicError(s.credentials.LockCredentialVault())
}

func (s *Service) ListCredentials(ctx context.Context) ([]model.Credential, error) {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicValue(s.credentials.ListCredentials(ctx))
}

func (s *Service) CreateCredential(
	ctx context.Context,
	input credential.CreateCredentialInput,
) (model.Credential, error) {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicValue(s.credentials.CreateCredential(ctx, input))
}

func (s *Service) GeneratePrivateKey(ctx context.Context, input credential.GeneratePrivateKeyInput) (credential.GeneratedPrivateKey, error) {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicValue(s.credentials.GeneratePrivateKey(ctx, input))
}

func (s *Service) ListLocalPrivateKeys(ctx context.Context) ([]credential.LocalPrivateKey, error) {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicValue(s.credentials.ListLocalPrivateKeys(ctx))
}

func (s *Service) ImportLocalPrivateKey(ctx context.Context, input credential.ImportLocalPrivateKeyInput) (model.Credential, error) {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicValue(s.credentials.ImportLocalPrivateKey(ctx, input))
}

func (s *Service) RenameCredential(
	ctx context.Context,
	id string,
	input credential.RenameCredentialInput,
) (model.Credential, error) {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicValue(s.credentials.RenameCredential(ctx, id, input))
}

func (s *Service) ReplaceCredentialSecret(
	ctx context.Context,
	id string,
	input credential.ReplaceCredentialSecretInput,
) (model.Credential, error) {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicValue(s.credentials.ReplaceCredentialSecret(ctx, id, input))
}

func (s *Service) DeleteCredential(ctx context.Context, id string) error {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicError(s.credentials.DeleteCredential(ctx, id))
}

func (s *Service) ResetCredentialVault(
	ctx context.Context,
	input credential.ResetCredentialVaultInput,
) (credential.CredentialVaultStatus, error) {
	status, err := s.credentials.ResetCredentialVault(ctx, input)
	if err == nil && status.State == credential.CredentialVaultStateUnlocked {
		if startErr := s.hosts.Start(ctx); startErr != nil {
			return status, publicError(startErr)
		}
	}
	return publicValue(status, err)
}

func (s *Service) ExportCredentialVaultKey(
	ctx context.Context,
	input credential.ExportCredentialVaultKeyInput,
) error {
	return publicError(s.credentials.ExportCredentialVaultKey(ctx, input))
}

func (s *Service) ImportCredentialVaultKey(
	ctx context.Context,
	input credential.ImportCredentialVaultKeyInput,
) (credential.CredentialVaultStatus, error) {
	status, err := s.credentials.ImportCredentialVaultKey(ctx, input)
	if err == nil && status.State == credential.CredentialVaultStateUnlocked {
		if startErr := s.hosts.Start(ctx); startErr != nil {
			return status, publicError(startErr)
		}
	}
	return publicValue(status, err)
}

func (s *Service) OpenSession(ctx context.Context, hostID string) (session.OpenSessionResult, error) {
	return publicValue(s.sessions.OpenSession(ctx, hostID))
}

type AdHocSessionInput struct {
	Protocol string `json:"protocol"`

	Address  string `json:"address"`
	Username string `json:"username"`

	Port int `json:"port"`

	Password string `json:"password"`
}

func (s *Service) OpenAdHocSession(ctx context.Context, input AdHocSessionInput) (session.OpenSessionResult, error) {
	protocol := strings.ToLower(strings.TrimSpace(input.Protocol))
	if protocol != "ssh" && protocol != "telnet" {
		return session.OpenSessionResult{}, publicError(errno.New(errno.CodeInvalidArg, "session.adhoc_invalid_protocol", ""))
	}
	address := strings.TrimSpace(input.Address)
	if address == "" || strings.ContainsAny(address, " \t\n@") {
		return session.OpenSessionResult{}, publicError(errno.New(errno.CodeInvalidArg, "session.adhoc_invalid_address", ""))
	}
	port := input.Port
	if port == 0 {
		if protocol == "telnet" {
			port = 23
		} else {
			port = 22
		}
	}
	if port < 1 || port > 65535 {
		return session.OpenSessionResult{}, publicError(errno.New(errno.CodeInvalidArg, "session.adhoc_invalid_port", ""))
	}
	id, err := randomAdHocHostID()
	if err != nil {
		return session.OpenSessionResult{}, publicError(err)
	}
	username := strings.TrimSpace(input.Username)

	passwordSource := "prompt"
	passwordConfigured := strings.TrimSpace(input.Password) != ""
	if passwordConfigured {
		passwordSource = "saved"
	}
	if protocol == model.ProtocolTelnet {

		passwordSource = "none"
		passwordConfigured = false
	}
	host := model.Host{
		ID:       id,
		Name:     address,
		Protocol: protocol,
		Address:  address,
		Port:     port,
		Username: username,

		PasswordSource:     passwordSource,
		PasswordConfigured: passwordConfigured,

		SSHKeepAlive:     true,
		LatencyDetection: true,
	}

	storedPassword := input.Password
	if !passwordConfigured {
		storedPassword = ""
	}
	s.adhoc.store(host, storedPassword)
	result, openErr := s.sessions.OpenSession(ctx, id)
	if openErr != nil {
		s.adhoc.forget(id)
		return session.OpenSessionResult{}, publicError(openErr)
	}
	return publicValue(result, nil)
}

type SerialPortInfo struct {
	Name string `json:"name"`
}

func (s *Service) ListSerialPorts() []SerialPortInfo {
	seen := make(map[string]struct{}, 8)
	result := make([]SerialPortInfo, 0, 8)
	appendPort := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, exists := seen[name]; exists {
			return
		}
		seen[name] = struct{}{}
		result = append(result, SerialPortInfo{Name: name})
	}

	if ports, err := serial.GetPortsList(); err == nil {
		for _, name := range ports {
			appendPort(name)
		}
	}
	if details, err := enumerator.GetDetailedPortsList(); err == nil {
		for _, port := range details {
			appendPort(port.Name)
		}
	}
	for _, name := range scanSerialDevices() {
		appendPort(name)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func scanSerialDevices() []string {
	entries, err := os.ReadDir("/dev")
	if err != nil {
		return nil
	}
	names := make([]string, 0, 4)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		switch runtime.GOOS {
		case "darwin":
			if strings.HasPrefix(name, "cu.") || strings.HasPrefix(name, "tty.") {
				names = append(names, "/dev/"+name)
			}
		default:
			if strings.HasPrefix(name, "ttyUSB") ||
				strings.HasPrefix(name, "ttyACM") ||
				strings.HasPrefix(name, "ttyS") ||
				strings.HasPrefix(name, "serial") {
				names = append(names, "/dev/"+name)
			}
		}
	}
	return names
}

type OpenSerialSessionInput struct {
	Device string `json:"device"`

	Baud int `json:"baud"`
}

func (s *Service) OpenSerialSession(ctx context.Context, input OpenSerialSessionInput) (session.OpenSessionResult, error) {
	device := strings.TrimSpace(input.Device)
	if device == "" {
		return session.OpenSessionResult{}, publicError(errno.New(errno.CodeInvalidArg, "session.serial_device_missing", ""))
	}
	baud := input.Baud
	if baud == 0 {
		baud = session.DefaultSerialBaud
	}
	if baud < 50 || baud > 4_000_000 {
		return session.OpenSessionResult{}, publicError(errno.New(errno.CodeInvalidArg, "session.serial_baud_invalid", ""))
	}
	id, err := randomAdHocHostID()
	if err != nil {
		return session.OpenSessionResult{}, publicError(err)
	}
	host := model.Host{
		ID:         id,
		Name:       device,
		Protocol:   model.ProtocolSerial,
		Address:    device,
		SerialBaud: baud,
	}
	s.adhoc.store(host, "")
	result, openErr := s.sessions.OpenSession(ctx, id)
	if openErr != nil {
		s.adhoc.forget(id)
		return session.OpenSessionResult{}, publicError(openErr)
	}
	return publicValue(result, nil)
}

func (s *Service) ForgetAdHocSession(hostID string) error {
	s.adhoc.forget(hostID)
	return nil
}

func (s *Service) OpenSftpConnection(ctx context.Context, hostID string) (model.Session, error) {
	return publicValue(s.sessions.OpenSftpConnection(ctx, hostID))
}

func randomAdHocHostID() (string, error) {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return "", errno.New(errno.CodeUnknown, "session.adhoc_id_failed", "")
	}
	return "adhoc-" + hex.EncodeToString(buffer), nil
}

func (s *Service) OpenLocalSession(
	ctx context.Context,
	shellPath string,
	workingDirectory string,
) (session.OpenSessionResult, error) {
	return publicValue(s.sessions.OpenLocalSession(ctx, shellPath, workingDirectory))
}

func (s *Service) RebindSession(ctx context.Context, id string) (session.OpenSessionResult, error) {
	result, err := s.sessions.RebindSession(ctx, id)
	if err != nil {
		return publicValue(result, err)
	}
	s.releaseClaimedSessionWindowTransfer(strings.TrimSpace(id))
	return result, nil
}

func (s *Service) PrepareSessionHandover(ctx context.Context, id string) error {
	return publicError(s.sessions.PrepareSessionHandover(ctx, id))
}

func (s *Service) TestConnection(ctx context.Context, input session.TestConnectionInput) error {
	return publicError(s.sessions.TestConnection(ctx, input))
}

func (s *Service) TestHostRoute(ctx context.Context, hostID string) (model.RouteTestResult, error) {
	return publicValue(s.sessions.TestHostRoute(ctx, hostID))
}

func (s *Service) TestHostRouteDraft(input model.RouteDraftInput) (model.RouteTestResult, error) {
	return publicValue(s.sessions.TestRouteDraft(context.Background(), input))
}

func (s *Service) DetectHostIcon(ctx context.Context, input session.DetectIconInput) (string, error) {
	icon, err := s.sessions.DetectIcon(ctx, input)
	if err != nil {
		return "", publicError(err)
	}
	if strings.TrimSpace(icon) == "" {
		return "", publicError(errno.New(errno.CodeConnFailed, "session.remote_os_unsupported", ""))
	}
	if hostID := strings.TrimSpace(input.HostID); hostID != "" {
		if err := host.SetDetectedHostIcon(s.hosts, ctx, hostID, icon); err != nil {
			return "", publicError(err)
		}
	}
	return icon, nil
}

func (s *Service) DetectX11Server(ctx context.Context, settings x11.Settings) (x11.Result, error) {
	result, err := x11.Detect(ctx, settings)
	if err != nil {

		if errors.Is(err, x11.ErrUnavailable) {
			return result, nil
		}
		return x11.Result{}, publicError(err)
	}
	return result, nil
}

func (s *Service) RevealCredential(ctx context.Context, id string) (credential.RevealedCredential, error) {
	if s == nil || s.credentials == nil {
		return credential.RevealedCredential{}, publicError(errors.New("credential service is not ready"))
	}
	revealed, err := s.credentials.RevealCredential(ctx, id)
	if err != nil {
		return credential.RevealedCredential{}, publicError(err)
	}
	return revealed, nil
}

func (s *Service) GetSessionWarnings(ctx context.Context, id string) ([]string, error) {
	manager, err := s.sessions.Manager()
	if err != nil {
		return nil, publicError(err)
	}
	if warning := manager.X11Warning(ctx, id); warning != "" {
		return []string{warning}, nil
	}
	return []string{}, nil
}

func (s *Service) GetSession(ctx context.Context, id string) (model.Session, error) {
	return publicValue(s.sessions.GetSession(ctx, id))
}

type AIBackgroundCommandResult struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exitCode"`
}

func (s *Service) RunAIBackgroundCommand(ctx context.Context, sessionID, command string) (AIBackgroundCommandResult, error) {
	result, err := s.sessions.RunAIBackgroundCommand(ctx, sessionID, command)
	return publicValue(AIBackgroundCommandResult{Output: result.Output, ExitCode: result.ExitCode}, err)
}

func (s *Service) ReadResources(ctx context.Context, id string) (model.ResourceSnapshot, error) {
	return publicValue(s.sessions.ReadResources(ctx, id))
}

func (s *Service) ListSessions(ctx context.Context) ([]model.Session, error) {
	return publicValue(s.sessions.ListSessions(ctx))
}

func (s *Service) ListRemoteDirectory(ctx context.Context, sessionID, directory string) ([]model.RemoteFileEntry, error) {
	return publicValue(s.sftp.ListRemoteDirectory(ctx, sessionID, directory))
}

func (s *Service) CloseSession(ctx context.Context, id string) error {
	return publicError(s.sessions.CloseSession(ctx, id))
}

func (s *Service) GetSessionHostKeyPending(id string) (model.HostKeyPending, error) {
	return publicValue(s.sessions.GetSessionHostKeyPending(id))
}

func (s *Service) GetSessionHostKeyChallenge(id string) (model.HostKeyChallenge, error) {
	return publicValue(s.sessions.GetSessionHostKeyChallenge(id))
}

func (s *Service) GetSessionAuthenticationChallenge(id string) (session.AuthenticationChallenge, error) {
	return publicValue(s.sessions.GetSessionAuthenticationChallenge(id))
}

func (s *Service) SubmitSessionAuthentication(id string, input session.AuthenticationInput) error {
	return publicError(s.sessions.SubmitSessionAuthentication(id, input))
}

func (s *Service) GetSessionFailure(id string) (string, error) {
	return publicValue(s.sessions.GetSessionFailure(id))
}

func (s *Service) TrustSessionHostKey(
	ctx context.Context,
	id string,
) (session.OpenSessionResult, error) {
	return publicValue(s.sessions.TrustSessionHostKey(ctx, id))
}

func (s *Service) SelectSFTPUploadFile(ctx context.Context, sessionID, remoteDirectory string) (sftp.SFTPTransferSelectionResult, error) {
	return publicValue(s.sftp.SelectSFTPUploadFile(ctx, sessionID, remoteDirectory))
}

func (s *Service) SelectSFTPDownloadFile(ctx context.Context, sessionID, remotePath string) (sftp.SFTPTransferSelectionResult, error) {
	return publicValue(s.sftp.SelectSFTPDownloadFile(ctx, sessionID, remotePath))
}

func (s *Service) ListSFTPTransfers(ctx context.Context) ([]model.SFTPTransfer, error) {
	return publicValue(s.sftp.ListSFTPTransfers(ctx))
}

func (s *Service) ConfirmSFTPTransferOverwrite(ctx context.Context, id string) (model.SFTPTransfer, error) {
	return publicValue(s.sftp.ConfirmSFTPTransferOverwrite(ctx, id))
}

func (s *Service) RetrySFTPTransfer(ctx context.Context, id string) (model.SFTPTransfer, error) {
	return publicValue(s.sftp.RetrySFTPTransfer(ctx, id))
}

func (s *Service) KnownHostsPath() string {
	return s.sessions.KnownHostsPath()
}

func (s *Service) ListKnownHosts() ([]model.KnownHostEntry, error) {
	return publicValue(s.sessions.ListKnownHosts(context.Background()))
}

func (s *Service) DeleteKnownHost(line int) error {
	return publicError(s.sessions.DeleteKnownHost(context.Background(), line))
}

func (s *Service) PauseSFTPTransfer(ctx context.Context, id string) (model.SFTPTransfer, error) {
	return publicValue(s.sftp.PauseSFTPTransfer(ctx, id))
}

func (s *Service) ResumeSFTPTransfer(ctx context.Context, id string) (model.SFTPTransfer, error) {
	return publicValue(s.sftp.ResumeSFTPTransfer(ctx, id))
}

func (s *Service) CancelSFTPTransfer(ctx context.Context, id string) error {
	return publicError(s.sftp.CancelSFTPTransfer(ctx, id))
}

func (s *Service) ListSnippets() ([]model.Snippet, error) {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicValue(s.hosts.ListSnippets())
}

func (s *Service) CreateSnippet(input model.SnippetInput) (model.Snippet, error) {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicValue(s.hosts.CreateSnippet(input))
}

func (s *Service) UpdateSnippet(id string, input model.SnippetInput) (model.Snippet, error) {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicValue(s.hosts.UpdateSnippet(id, input))
}

func (s *Service) DeleteSnippet(id string) error {
	s.assetMu.RLock()
	defer s.assetMu.RUnlock()
	return publicError(s.hosts.DeleteSnippet(id))
}

func (s *Service) ResolveSFTPTransferConflict(ctx context.Context, id, action string) error {
	return publicError(s.sftp.ResolveSFTPTransferConflict(ctx, id, action))
}

func (s *Service) GetAppLockStatus() (applock.Status, error) {
	return s.appLock.Status(), nil
}

func (s *Service) EnableAppLock(password string, autoLockMinutes int, question, answer string) error {
	return s.appLock.Enable(password, autoLockMinutes, question, answer)
}

func (s *Service) ResetAppLockPassword(answer, next string) error {
	return s.appLock.ResetPasswordByRecovery(answer, next)
}

func (s *Service) NotifyAppLockState(locked bool) error {
	if locked && s.agents != nil {

		s.agents.CancelAll()
	}
	application.Get().Event.Emit(appLockEventName, locked)
	return nil
}

func (s *Service) ChangeAppLockPassword(current, next string) error {
	return s.appLock.ChangePassword(current, next)
}

func (s *Service) DisableAppLock(password string) error {
	return s.appLock.Disable(password)
}

func (s *Service) SetAppLockAutoLockMinutes(minutes int) error {
	return s.appLock.SetAutoLockMinutes(minutes)
}

func (s *Service) VerifyAppLockPassword(password string) (bool, error) {
	return s.appLock.Verify(password), nil
}

func looksLikeInternalGroupRef(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)

	if strings.HasPrefix(lower, "group_") || strings.HasPrefix(lower, "grp_") {
		suffix := lower[strings.Index(lower, "_")+1:]
		if suffix != "" && (isAllDigits(suffix) || isHex(suffix)) {
			return true
		}
	}
	return isAllDigits(trimmed) || isHex(trimmed)
}

func isHex(value string) bool {
	if len(value) < 16 {
		return false
	}
	for _, char := range strings.ToLower(value) {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

func isAllDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
