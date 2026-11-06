package app

import (
	"changeme/internal/model"
	"changeme/internal/session"
)

func (s *Service) OpenForward(config model.ForwardConfig) error {
	if s == nil || s.sessions == nil {
		return publicError(session.ErrServiceNotReady)
	}
	return publicError(s.sessions.OpenForward(config))
}

func (s *Service) CloseForward(id string) error {
	if s == nil || s.sessions == nil {
		return publicError(session.ErrServiceNotReady)
	}
	return publicError(s.sessions.CloseForward(id))
}

func (s *Service) ForwardStatus(id string) model.ForwardStatus {
	if s == nil || s.sessions == nil {
		return model.ForwardStatus{ID: id, State: model.ForwardStopped}
	}
	return s.sessions.ForwardStatus(id)
}
