package app

import (
	"context"

	"changeme/internal/errno"
	"changeme/internal/model"
)

var errMonitorNotReady = errno.New(errno.CodeInvalidArg, "monitor.service_unavailable", "")

func (s *Service) ReadProcesses(sessionID string) (model.ProcessSnapshot, error) {
	if s == nil || s.sessions == nil {
		return model.ProcessSnapshot{}, publicError(errMonitorNotReady)
	}
	snapshot, err := s.sessions.ReadProcesses(context.Background(), sessionID)
	if err != nil {
		return model.ProcessSnapshot{}, publicError(err)
	}
	return snapshot, nil
}

func (s *Service) ReadAccelerators(sessionID string) (model.AcceleratorSnapshot, error) {
	if s == nil || s.sessions == nil {
		return model.AcceleratorSnapshot{}, publicError(errMonitorNotReady)
	}
	snapshot, err := s.sessions.ReadAccelerators(context.Background(), sessionID)
	if err != nil {
		return model.AcceleratorSnapshot{}, publicError(err)
	}
	return snapshot, nil
}

func (s *Service) RunProcessAction(sessionID string, pid int, expectedStartTime, action string) error {
	if s == nil || s.sessions == nil {
		return publicError(errMonitorNotReady)
	}
	return publicError(s.sessions.RunProcessAction(context.Background(), sessionID, pid, expectedStartTime, action))
}
