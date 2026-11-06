package sftp

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"changeme/internal/errno"
)

const (
	DefaultTransferConcurrency = 3
	MinTransferConcurrency     = 1
	MaxTransferConcurrency     = 16
)

const (
	DragFolderUploadModeNormal = "normal"

	DragFolderUploadModeArchive = "archive"
)

type Settings struct {
	AutoDirectoryFollow bool   `json:"autoDirectoryFollow"`
	ShowHiddenFiles     bool   `json:"showHiddenFiles"`
	MaxTransfers        int    `json:"maxTransfers"`
	LocalDirectory      string `json:"localDirectory"`
	EditorPath          string `json:"editorPath"`

	DragFolderUploadMode string `json:"dragFolderUploadMode"`

	TransferDiagnostics *TransferDiagnostics `json:"transferDiagnostics,omitempty"`
}

type TransferDiagnostics struct {
	ActiveConnections int `json:"activeConnections"`

	DesiredConnections int `json:"desiredConnections"`

	BlockedReason string `json:"blockedReason"`

	Recorded bool `json:"recorded"`
}

func DefaultSettings() Settings {
	return Settings{
		AutoDirectoryFollow:  true,
		ShowHiddenFiles:      false,
		MaxTransfers:         DefaultTransferConcurrency,
		DragFolderUploadMode: DragFolderUploadModeNormal,
	}
}

func normalizeSettings(value Settings) (Settings, error) {
	if value.MaxTransfers == 0 {
		value.MaxTransfers = DefaultTransferConcurrency
	}
	if value.MaxTransfers < MinTransferConcurrency || value.MaxTransfers > MaxTransferConcurrency {
		return Settings{}, errno.New(errno.CodeInvalidArg, "sftp.concurrency_invalid", "")
	}

	switch mode := strings.TrimSpace(value.DragFolderUploadMode); mode {
	case "":
		value.DragFolderUploadMode = DragFolderUploadModeNormal
	case DragFolderUploadModeNormal, DragFolderUploadModeArchive:
		value.DragFolderUploadMode = mode
	default:
		return Settings{}, errno.New(errno.CodeInvalidArg, "sftp.drag_folder_upload_mode_invalid", "")
	}
	localDirectory, err := normalizeConfiguredDirectory(value.LocalDirectory)
	if err != nil {
		return Settings{}, err
	}
	editorPath, err := normalizeEditorPath(value.EditorPath)
	if err != nil {
		return Settings{}, err
	}
	value.LocalDirectory = localDirectory
	value.EditorPath = editorPath
	return value, nil
}

func normalizeConfiguredDirectory(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !filepath.IsAbs(value) || strings.ContainsAny(value, "\x00\r\n") {
		return "", errno.New(errno.CodeInvalidArg, "sftp.local_directory_invalid", "")
	}
	value = filepath.Clean(value)
	info, err := os.Stat(value)
	if err != nil {
		return "", errno.New(errno.CodeNotFound, "sftp.local_directory_not_found", "")
	}
	if !info.IsDir() {
		return "", errno.New(errno.CodeInvalidArg, "sftp.local_directory_invalid", "")
	}
	return value, nil
}

func normalizeEditorPath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !filepath.IsAbs(value) || strings.ContainsAny(value, "\x00\r\n") {
		return "", errno.New(errno.CodeInvalidArg, "sftp.editor_invalid", "")
	}
	value = filepath.Clean(value)
	if _, err := os.Stat(value); err != nil {
		return "", errno.New(errno.CodeNotFound, "sftp.editor_not_found", "")
	}
	return value, nil
}

func (s *Service) dragFolderUploadMode() string {
	if s == nil {
		return DragFolderUploadModeNormal
	}
	s.mu.RLock()
	mode := s.settings.DragFolderUploadMode
	s.mu.RUnlock()
	if mode == DragFolderUploadModeArchive {
		return DragFolderUploadModeArchive
	}
	return DragFolderUploadModeNormal
}

func (s *Service) ApplySettings(value Settings) error {
	normalized, err := normalizeSettings(value)
	if err != nil {
		return err
	}
	s.mu.RLock()
	transfers := s.transfers
	local := s.local
	s.mu.RUnlock()
	if transfers == nil || local == nil {
		return ErrServiceNotReady
	}
	if err := transfers.SetConcurrency(normalized.MaxTransfers); err != nil {
		return err
	}
	if err := local.setStartDirectory(normalized.LocalDirectory); err != nil {
		return err
	}
	s.mu.Lock()
	s.settings = normalized
	s.mu.Unlock()
	return nil
}

func (s *Service) Settings() Settings {
	if s == nil {
		return DefaultSettings()
	}
	s.mu.RLock()
	value := s.settings
	s.mu.RUnlock()
	if value.MaxTransfers == 0 {
		return DefaultSettings()
	}
	if diagnostics := s.transferDiagnosticsLocked(); diagnostics != nil {
		value.TransferDiagnostics = diagnostics
	}
	return value
}

func (s *Service) TransferDiagnostics() TransferDiagnostics {
	if s == nil {
		return TransferDiagnostics{ActiveConnections: -1, DesiredConnections: -1, BlockedReason: "unsupported"}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return *s.transferDiagnosticsLocked()
}

func (s *Service) transferDiagnosticsLocked() *TransferDiagnostics {
	if !s.hasConnectionReport {
		return &TransferDiagnostics{ActiveConnections: -1, DesiredConnections: -1, BlockedReason: "expansion_pending"}
	}
	return &TransferDiagnostics{
		ActiveConnections:  s.lastConnections,
		DesiredConnections: s.lastDesired,
		BlockedReason:      s.lastBlocked,
		Recorded:           true,
	}
}

func configuredStartDirectory(value string) (string, error) {
	if value != "" {
		return value, nil
	}
	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) {
		return "", errors.New("user home directory is unavailable")
	}
	return filepath.Clean(home), nil
}
