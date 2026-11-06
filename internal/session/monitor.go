package session

import (
	"context"
	"errors"

	"changeme/internal/errno"
	"changeme/internal/model"
	sshclient "changeme/internal/tunnel"
)

type monitorConnection interface {
	ReadProcesses(context.Context) (model.ProcessSnapshot, error)
	ReadAccelerators(context.Context) (model.AcceleratorSnapshot, error)
	RunProcessAction(context.Context, int, string, sshclient.ProcessAction) error
}

func (m *SessionManager) ReadProcesses(ctx context.Context, id string) (model.ProcessSnapshot, error) {
	worker, err := m.lookup(ctx, id)
	if err != nil {
		return model.ProcessSnapshot{}, err
	}
	worker.mu.RLock()
	raw := worker.connection
	worker.mu.RUnlock()
	probe, ok := raw.(monitorConnection)
	if !ok {

		if raw == nil {
			return model.ProcessSnapshot{}, errno.New(errno.CodeConnFailed, "monitor.not_ready", "")
		}
		return model.ProcessSnapshot{}, errno.New(errno.CodeConnFailed, "monitor.unsupported", "")
	}
	snapshot, err := probe.ReadProcesses(ctx)
	if err != nil {
		return model.ProcessSnapshot{}, errno.Wrap(errno.CodeConnFailed, "monitor.processes_failed", err)
	}
	return snapshot, nil
}

func (m *SessionManager) ReadAccelerators(ctx context.Context, id string) (model.AcceleratorSnapshot, error) {
	worker, err := m.lookup(ctx, id)
	if err != nil {
		return model.AcceleratorSnapshot{}, err
	}
	worker.mu.RLock()
	raw := worker.connection
	worker.mu.RUnlock()
	probe, ok := raw.(monitorConnection)
	if !ok {
		return model.AcceleratorSnapshot{}, errno.New(errno.CodeConnFailed, "monitor.unsupported", "")
	}
	snapshot, err := probe.ReadAccelerators(ctx)
	if err != nil {
		return model.AcceleratorSnapshot{}, errno.Wrap(errno.CodeConnFailed, "monitor.accelerators_failed", err)
	}
	return snapshot, nil
}

func (m *SessionManager) RunProcessAction(
	ctx context.Context,
	id string,
	pid int,
	expectedStartTime string,
	action string,
) error {
	worker, err := m.lookup(ctx, id)
	if err != nil {
		return err
	}
	worker.mu.RLock()
	raw := worker.connection
	worker.mu.RUnlock()
	probe, ok := raw.(monitorConnection)
	if !ok {
		return errno.New(errno.CodeConnFailed, "monitor.unsupported", "")
	}
	normalized, ok := processActionOf(action)
	if !ok {
		return errno.New(errno.CodeInvalidArg, "monitor.invalid_process_action", "")
	}
	if err := probe.RunProcessAction(ctx, pid, expectedStartTime, normalized); err != nil {
		switch {
		case errors.Is(err, sshclient.ErrInvalidProcessPID),
			errors.Is(err, sshclient.ErrMissingProcessStartTime),
			errors.Is(err, sshclient.ErrInvalidProcessStartTime),
			errors.Is(err, sshclient.ErrInvalidProcessAction):
			return errno.New(errno.CodeInvalidArg, "monitor.invalid_process_action", "")
		case errors.Is(err, sshclient.ErrProcessNotFound),
			errors.Is(err, sshclient.ErrProcessStartTimeMismatch):
			return errno.New(errno.CodeInvalidArg, "monitor.process_changed", "")
		default:
			return errno.Wrap(errno.CodeConnFailed, "monitor.process_action_failed", err)
		}
	}
	return nil
}

func (c *productionSessionConnection) ReadProcesses(ctx context.Context) (model.ProcessSnapshot, error) {
	if c == nil || c.connection == nil {
		return model.ProcessSnapshot{}, ErrSessionNotConnected
	}
	return c.connection.ReadProcesses(ctx)
}

func (c *productionSessionConnection) ReadAccelerators(ctx context.Context) (model.AcceleratorSnapshot, error) {
	if c == nil || c.connection == nil {
		return model.AcceleratorSnapshot{}, ErrSessionNotConnected
	}
	return c.connection.ReadAccelerators(ctx)
}

func (c *productionSessionConnection) RunProcessAction(
	ctx context.Context,
	pid int,
	expectedStartTime string,
	action sshclient.ProcessAction,
) error {
	if c == nil || c.connection == nil {
		return ErrSessionNotConnected
	}
	return c.connection.RunProcessAction(ctx, pid, expectedStartTime, action)
}

func processActionOf(value string) (sshclient.ProcessAction, bool) {
	switch sshclient.ProcessAction(value) {
	case sshclient.ProcessTerminate, sshclient.ProcessKill:
		return sshclient.ProcessAction(value), true
	default:
		return "", false
	}
}

func (s *Service) ReadProcesses(ctx context.Context, id string) (model.ProcessSnapshot, error) {
	if s == nil {
		return model.ProcessSnapshot{}, ErrServiceNotReady
	}
	s.mu.RLock()
	manager := s.manager
	s.mu.RUnlock()
	if manager == nil {
		return model.ProcessSnapshot{}, ErrServiceNotReady
	}
	return manager.ReadProcesses(ctx, id)
}

func (s *Service) ReadAccelerators(ctx context.Context, id string) (model.AcceleratorSnapshot, error) {
	if s == nil {
		return model.AcceleratorSnapshot{}, ErrServiceNotReady
	}
	s.mu.RLock()
	manager := s.manager
	s.mu.RUnlock()
	if manager == nil {
		return model.AcceleratorSnapshot{}, ErrServiceNotReady
	}
	return manager.ReadAccelerators(ctx, id)
}

func (s *Service) RunProcessAction(
	ctx context.Context,
	id string,
	pid int,
	expectedStartTime string,
	action string,
) error {
	manager, err := s.sessionManager()
	if err != nil {
		return err
	}
	return manager.RunProcessAction(ctx, id, pid, expectedStartTime, action)
}
