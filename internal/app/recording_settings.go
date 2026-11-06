package app

import (
	"context"
	"errors"
	"strings"

	"changeme/internal/errno"
	"changeme/internal/recordingsdir"
)

type RecordingSettings struct {
	Configured string `json:"configured"`

	Effective string `json:"effective"`

	Default string `json:"default"`
}

func (s *Service) GetRecordingSettings(ctx context.Context) (RecordingSettings, error) {
	if err := contextReady(ctx); err != nil {
		return RecordingSettings{}, err
	}
	if s == nil || s.sessions == nil {
		return RecordingSettings{}, publicError(errors.New("session service is not ready"))
	}
	defaultDir, err := recordingsdir.Default()
	if err != nil {
		return RecordingSettings{}, publicError(errno.New(errno.CodeStoreFailed, "recording.config_failed", ""))
	}
	effective, err := s.sessions.RecordingDirectory()
	if err != nil {
		return RecordingSettings{}, publicError(err)
	}
	return RecordingSettings{
		Configured: s.recordingDirectoryOverride(),
		Effective:  effective,
		Default:    defaultDir,
	}, nil
}

func (s *Service) SelectRecordingDirectory(ctx context.Context) (string, error) {
	if err := contextReady(ctx); err != nil {
		return "", err
	}
	if s == nil || s.fileDialogs == nil {
		return "", publicError(errno.New(errno.CodeStoreFailed, "host.recording_dialog_unavailable", ""))
	}
	value, err := s.fileDialogs.ChooseLocalDirectory()
	if err != nil {
		return "", publicError(err)
	}
	return strings.TrimSpace(value), nil
}

func (s *Service) OpenRecordingDirectory(ctx context.Context) error {
	if err := contextReady(ctx); err != nil {
		return err
	}
	if s == nil || s.sessions == nil {
		return publicError(errors.New("session service is not ready"))
	}
	if s.fileDialogs == nil {
		return publicError(errno.New(errno.CodeStoreFailed, "host.recording_dialog_unavailable", ""))
	}
	target, err := s.sessions.RecordingDirectory()
	if err != nil {
		return publicError(err)
	}
	if err := s.fileDialogs.OpenPath(target); err != nil {
		return publicError(errno.New(errno.CodeStoreFailed, "host.recording_directory_open_failed", ""))
	}
	return nil
}

func (s *Service) recordingDirectoryOverride() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.recordingDirOverride
}

func (s *Service) applyRecordingDirectory(directory string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.applyRecordingDirectoryLocked(directory)
}

func (s *Service) applyRecordingDirectoryLocked(directory string) error {
	trimmed := strings.TrimSpace(directory)
	if s.sessions != nil {
		if err := s.sessions.SetRecordingDirectory(trimmed); err != nil {
			return err
		}
	}
	if s.hosts != nil {
		if err := s.hosts.SetRecordingDirectory(trimmed); err != nil {
			return err
		}
	}
	s.recordingDirOverride = trimmed
	return nil
}
