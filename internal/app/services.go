package app

import (
	"context"

	"changeme/internal/errno"
	"changeme/internal/model"
)

var errServicesNotReady = errno.New(errno.CodeInvalidArg, "services.service_unavailable", "")

func (s *Service) ListServices(sessionID string) (model.ServiceSnapshot, error) {
	if s == nil || s.sessions == nil {
		return model.ServiceSnapshot{}, publicError(errServicesNotReady)
	}
	snapshot, err := s.sessions.ReadServices(context.Background(), sessionID)
	if err != nil {
		return model.ServiceSnapshot{}, publicError(err)
	}
	return snapshot, nil
}

func (s *Service) RunServiceAction(sessionID, unit, action string) error {
	if s == nil || s.sessions == nil {
		return publicError(errServicesNotReady)
	}
	return publicError(s.sessions.RunServiceAction(context.Background(), sessionID, unit, action))
}

func (s *Service) ServiceDetails(sessionID, unit string) (model.ServiceDetail, error) {
	if s == nil || s.sessions == nil {
		return model.ServiceDetail{}, publicError(errServicesNotReady)
	}
	detail, err := s.sessions.ReadServiceDetail(context.Background(), sessionID, unit)
	if err != nil {
		return model.ServiceDetail{}, publicError(err)
	}
	return detail, nil
}

func (s *Service) ServiceLogs(sessionID, unit string, lines int) (model.ServiceLog, error) {
	if s == nil || s.sessions == nil {
		return model.ServiceLog{}, publicError(errServicesNotReady)
	}
	logs, err := s.sessions.ReadServiceLogs(context.Background(), sessionID, unit, lines)
	if err != nil {
		return model.ServiceLog{}, publicError(err)
	}
	return logs, nil
}
