package session

import (
	"context"
	"errors"

	"changeme/internal/errno"
	"changeme/internal/model"
	sshclient "changeme/internal/tunnel"
)

type serviceConnection interface {
	ReadServices(context.Context) (model.ServiceSnapshot, error)
	RunServiceAction(context.Context, string, sshclient.ServiceAction) error
	ReadServiceDetail(context.Context, string) (model.ServiceDetail, error)
	ReadServiceLogs(context.Context, string, int) (model.ServiceLog, error)
}

func (c *productionSessionConnection) ReadServices(ctx context.Context) (model.ServiceSnapshot, error) {
	if c == nil || c.connection == nil {
		return model.ServiceSnapshot{}, ErrSessionNotConnected
	}
	return c.connection.ReadServices(ctx)
}

func (c *productionSessionConnection) RunServiceAction(ctx context.Context, unit string, action sshclient.ServiceAction) error {
	if c == nil || c.connection == nil {
		return ErrSessionNotConnected
	}
	return c.connection.RunServiceAction(ctx, unit, action)
}

func (c *productionSessionConnection) ReadServiceDetail(ctx context.Context, unit string) (model.ServiceDetail, error) {
	if c == nil || c.connection == nil {
		return model.ServiceDetail{}, ErrSessionNotConnected
	}
	return c.connection.ReadServiceDetail(ctx, unit)
}

func (c *productionSessionConnection) ReadServiceLogs(ctx context.Context, unit string, lines int) (model.ServiceLog, error) {
	if c == nil || c.connection == nil {
		return model.ServiceLog{}, ErrSessionNotConnected
	}
	return c.connection.ReadServiceLogs(ctx, unit, lines)
}

func (m *SessionManager) serviceProbe(ctx context.Context, id string) (serviceConnection, error) {
	worker, err := m.lookup(ctx, id)
	if err != nil {
		return nil, err
	}
	worker.mu.RLock()
	raw := worker.connection
	worker.mu.RUnlock()
	probe, ok := raw.(serviceConnection)
	if !ok {
		return nil, errno.New(errno.CodeConnFailed, "services.unsupported", "")
	}
	return probe, nil
}

func (m *SessionManager) ReadServices(ctx context.Context, id string) (model.ServiceSnapshot, error) {
	probe, err := m.serviceProbe(ctx, id)
	if err != nil {
		return model.ServiceSnapshot{}, err
	}
	snapshot, err := probe.ReadServices(ctx)
	if err != nil {

		return model.ServiceSnapshot{}, classifyServiceReadError("list", err)
	}
	return snapshot, nil
}

func (m *SessionManager) RunServiceAction(ctx context.Context, id, unit, action string) error {
	probe, err := m.serviceProbe(ctx, id)
	if err != nil {
		return err
	}
	normalized, ok := serviceActionOf(action)
	if !ok {
		return errno.New(errno.CodeInvalidArg, "services.invalid_action", "")
	}
	if err := probe.RunServiceAction(ctx, unit, normalized); err != nil {
		return errno.Wrap(errno.CodeConnFailed, "services.action_failed", err)
	}
	return nil
}

func (m *SessionManager) ReadServiceDetail(ctx context.Context, id, unit string) (model.ServiceDetail, error) {
	probe, err := m.serviceProbe(ctx, id)
	if err != nil {
		return model.ServiceDetail{}, err
	}
	detail, err := probe.ReadServiceDetail(ctx, unit)
	if err != nil {
		return model.ServiceDetail{}, classifyServiceReadError("detail", err)
	}
	return detail, nil
}

func (m *SessionManager) ReadServiceLogs(ctx context.Context, id, unit string, lines int) (model.ServiceLog, error) {
	probe, err := m.serviceProbe(ctx, id)
	if err != nil {
		return model.ServiceLog{}, err
	}
	logs, err := probe.ReadServiceLogs(ctx, unit, lines)
	if err != nil {
		return model.ServiceLog{}, classifyServiceReadError("logs", err)
	}
	return logs, nil
}

func classifyServiceReadError(operation string, err error) error {
	if err == nil {
		return nil
	}
	var probeErr *sshclient.ServiceProbeError
	if errors.As(err, &probeErr) {
		switch probeErr.Kind {
		case sshclient.ServiceProbePermissionDenied:
			return errno.New(errno.CodePermissionDenied, "services.permission_denied", "")
		case sshclient.ServiceProbeManagerUnavailable:
			return errno.New(errno.CodeConnFailed, "services.manager_unavailable", "")
		case sshclient.ServiceProbeJournalUnavailable:
			return errno.New(errno.CodeConnFailed, "services.journal_unavailable", "")
		case sshclient.ServiceProbeLogsUnsupported:
			return errno.New(errno.CodeUnsupported, "services.logs_unsupported", "")
		case sshclient.ServiceProbeUnitNotFound:
			return errno.New(errno.CodeNotFound, "services.unit_not_found", "")
		}
	}
	if errors.Is(err, sshclient.ErrInvalidUnitName) {
		return errno.New(errno.CodeInvalidArg, "services.invalid_unit", "")
	}
	if operation == "logs" && errors.Is(err, sshclient.ErrInvalidServiceLogLines) {
		return errno.New(errno.CodeInvalidArg, "services.invalid_log_lines", "")
	}
	if operation == "logs" {
		return errno.Wrap(errno.CodeConnFailed, "services.logs_failed", err)
	}
	return errno.Wrap(errno.CodeConnFailed, "services.detail_failed", err)
}

func serviceActionOf(value string) (sshclient.ServiceAction, bool) {
	switch sshclient.ServiceAction(value) {
	case sshclient.ServiceStart, sshclient.ServiceStop, sshclient.ServiceRestart, sshclient.ServiceEnable, sshclient.ServiceDisable:
		return sshclient.ServiceAction(value), true
	default:
		return "", false
	}
}

func (s *Service) ReadServices(ctx context.Context, id string) (model.ServiceSnapshot, error) {
	manager, err := s.sessionManager()
	if err != nil {
		return model.ServiceSnapshot{}, err
	}
	return manager.ReadServices(ctx, id)
}

func (s *Service) RunServiceAction(ctx context.Context, id, unit, action string) error {
	manager, err := s.sessionManager()
	if err != nil {
		return err
	}
	return manager.RunServiceAction(ctx, id, unit, action)
}

func (s *Service) ReadServiceDetail(ctx context.Context, id, unit string) (model.ServiceDetail, error) {
	manager, err := s.sessionManager()
	if err != nil {
		return model.ServiceDetail{}, err
	}
	return manager.ReadServiceDetail(ctx, id, unit)
}

func (s *Service) ReadServiceLogs(ctx context.Context, id, unit string, lines int) (model.ServiceLog, error) {
	manager, err := s.sessionManager()
	if err != nil {
		return model.ServiceLog{}, err
	}
	return manager.ReadServiceLogs(ctx, id, unit, lines)
}

func (s *Service) sessionManager() (*SessionManager, error) {
	if s == nil {
		return nil, ErrServiceNotReady
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.manager == nil {
		return nil, ErrServiceNotReady
	}
	return s.manager, nil
}
