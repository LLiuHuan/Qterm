package host

import (
	"path/filepath"
	"strings"

	"changeme/internal/errno"
	"changeme/internal/recordingsdir"
)

func (s *HostService) SetRecordingDirectory(directory string) error {
	if s == nil {
		return ErrHostServiceNotReady
	}
	normalized := ""
	if strings.TrimSpace(directory) != "" {
		value, err := recordingsdir.Validate(directory)
		if err != nil {
			return errno.New(errno.CodeInvalidArg, "host.recording_directory_invalid", "")
		}
		normalized = value
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recordingDirOverride = normalized
	return nil
}

func (s *HostService) RecordingsRoot() (string, error) {
	if s == nil {
		return "", ErrHostServiceNotReady
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.recordingRootHeld()
}

func (s *HostService) recordingRootHeld() (string, error) {
	if strings.TrimSpace(s.recordingDirOverride) != "" {
		root, err := recordingsdir.Validate(s.recordingDirOverride)
		if err != nil {
			return "", errno.New(errno.CodeStoreFailed, "host.recording_path_failed", "")
		}
		return root, nil
	}
	if s.configDir == nil {
		return "", errno.New(errno.CodeStoreFailed, "host.recording_path_failed", "")
	}
	base, err := s.configDir()
	if err != nil {
		return "", errno.New(errno.CodeStoreFailed, "host.recording_path_failed", "")
	}
	return filepath.Join(base, "Qterm", "recordings"), nil
}
