package app

import (
	"context"

	"changeme/internal/tmux"
)

func (s *Service) ListTmuxSessions(ctx context.Context, sessionID string) ([]tmux.Session, error) {
	if s == nil || s.tmux == nil {
		return nil, publicError(tmux.ErrUnavailable)
	}
	sessions, err := s.tmux.List(ctx, sessionID)
	if err != nil {
		return nil, publicError(err)
	}
	return sessions, nil
}

func (s *Service) CreateTmuxSession(ctx context.Context, sessionID, name string) error {
	if s == nil || s.tmux == nil {
		return publicError(tmux.ErrUnavailable)
	}
	return publicError(s.tmux.Create(ctx, sessionID, name))
}

func (s *Service) KillTmuxSession(ctx context.Context, sessionID, name string) error {
	if s == nil || s.tmux == nil {
		return publicError(tmux.ErrUnavailable)
	}
	return publicError(s.tmux.Kill(ctx, sessionID, name))
}

func (s *Service) TmuxAttachCommand(_ context.Context, name, trackingToken string) (string, error) {
	command, err := tmux.BuildTrackedAttachCommand(name, trackingToken)
	if err != nil {
		return "", publicError(err)
	}
	return command, nil
}

func (s *Service) TmuxVersion(ctx context.Context, sessionID string) (string, error) {
	if s == nil || s.tmux == nil {
		return "", publicError(tmux.ErrUnavailable)
	}
	version, err := s.tmux.Version(ctx, sessionID)
	if err != nil {
		return "", publicError(err)
	}
	return version, nil
}

func (s *Service) RenameTmuxSession(ctx context.Context, sessionID, name, newName string) error {
	if s == nil || s.tmux == nil {
		return publicError(tmux.ErrUnavailable)
	}
	return publicError(s.tmux.Rename(ctx, sessionID, name, newName))
}

func (s *Service) TmuxDetachKeys(ctx context.Context, sessionID string) (string, error) {
	if s == nil || s.tmux == nil {
		return "", publicError(tmux.ErrUnavailable)
	}
	keys, err := s.tmux.DetachKeys(ctx, sessionID)
	if err != nil {
		return "", publicError(err)
	}
	return keys, nil
}

func (s *Service) TmuxPrefixKeys(ctx context.Context, sessionID string) (string, error) {
	if s == nil || s.tmux == nil {
		return "", publicError(tmux.ErrUnavailable)
	}
	keys, err := s.tmux.PrefixKeys(ctx, sessionID)
	if err != nil {
		return "", publicError(err)
	}
	return keys, nil
}
