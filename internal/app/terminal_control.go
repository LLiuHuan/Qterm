package app

import (
	"context"
	"errors"
	"strconv"

	"changeme/internal/errno"
	"changeme/internal/session"
	"changeme/internal/terminal"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type terminalSessionDirectory struct{ service *session.Service }

func (d terminalSessionDirectory) List(ctx context.Context) ([]terminal.SessionInfo, error) {
	items, err := d.service.ListSessions(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]terminal.SessionInfo, 0, len(items))
	for _, item := range items {
		result = append(result, terminal.SessionInfo{
			ID: item.ID, HostID: item.HostID, Kind: string(item.Kind), State: string(item.State), Purpose: string(item.Purpose),
		})
	}
	return result, nil
}

type terminalSessionWriter struct{ service *session.Service }

func (w terminalSessionWriter) Write(ctx context.Context, sessionID string, data []byte) error {
	manager, err := w.service.Manager()
	if err != nil {
		return err
	}
	return manager.WriteTerminalInput(ctx, sessionID, data)
}

func terminalWindowID(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", terminal.ErrWindowRequired
	}
	window, ok := ctx.Value(application.WindowKey).(application.Window)
	if !ok || window == nil {
		return "", terminal.ErrWindowRequired
	}
	return strconv.FormatUint(uint64(window.ID()), 10), nil
}

func (s *Service) terminalController() (*terminal.Manager, error) {
	if s == nil {
		return nil, terminal.ErrControlNotReady
	}
	s.mu.Lock()
	control := s.terminalControl
	s.mu.Unlock()
	if control == nil {
		return nil, terminal.ErrControlNotReady
	}
	return control, nil
}

func (s *Service) GetTerminalControl(ctx context.Context) (terminal.ControlSnapshot, error) {
	windowID, err := terminalWindowID(ctx)
	if err != nil {
		return terminal.ControlSnapshot{}, terminalControlError(err)
	}
	control, err := s.terminalController()
	if err != nil {
		return terminal.ControlSnapshot{}, terminalControlError(err)
	}
	value, err := control.Get(ctx, windowID)
	return value, terminalControlError(err)
}

func (s *Service) SetTerminalSync(ctx context.Context, request terminal.SyncRequest) (terminal.ControlSnapshot, error) {
	windowID, err := terminalWindowID(ctx)
	if err != nil {
		return terminal.ControlSnapshot{}, terminalControlError(err)
	}
	control, err := s.terminalController()
	if err != nil {
		return terminal.ControlSnapshot{}, terminalControlError(err)
	}
	value, err := control.SetSync(ctx, windowID, request)
	return value, terminalControlError(err)
}

func (s *Service) StartTerminalBatch(ctx context.Context, request terminal.BatchRequest) (terminal.BatchTaskState, error) {
	windowID, err := terminalWindowID(ctx)
	if err != nil {
		return terminal.BatchTaskState{}, terminalControlError(err)
	}
	control, err := s.terminalController()
	if err != nil {
		return terminal.BatchTaskState{}, terminalControlError(err)
	}
	value, err := control.StartBatch(ctx, windowID, request)
	return value, terminalControlError(err)
}

func (s *Service) CancelTerminalBatch(ctx context.Context, taskID string) (terminal.BatchTaskState, error) {
	windowID, err := terminalWindowID(ctx)
	if err != nil {
		return terminal.BatchTaskState{}, terminalControlError(err)
	}
	control, err := s.terminalController()
	if err != nil {
		return terminal.BatchTaskState{}, terminalControlError(err)
	}
	value, err := control.CancelBatch(ctx, windowID, taskID)
	return value, terminalControlError(err)
}

func terminalControlError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, terminal.ErrWindowRequired):
		return errno.Wrap(errno.CodeInvalidArg, "terminalControl.windowRequired", err)
	case errors.Is(err, terminal.ErrSessionRequired), errors.Is(err, terminal.ErrSessionNotEligible):
		return errno.Wrap(errno.CodeInvalidArg, "terminalControl.sessionNotEligible", err)
	case errors.Is(err, terminal.ErrSyncGroupNotFound):
		return errno.Wrap(errno.CodeNotFound, "terminalControl.syncNotFound", err)
	case errors.Is(err, terminal.ErrSyncRevisionConflict):
		return errno.Wrap(errno.CodeInvalidArg, "terminalControl.syncRevisionConflict", err)
	case errors.Is(err, terminal.ErrSyncMemberRequired):
		return errno.Wrap(errno.CodeInvalidArg, "terminalControl.syncMemberRequired", err)
	case errors.Is(err, terminal.ErrBatchSyncConflict):
		return errno.Wrap(errno.CodeInvalidArg, "terminalControl.batchSyncConflict", err)
	case errors.Is(err, terminal.ErrBatchAlreadyRunning):
		return errno.Wrap(errno.CodeInvalidArg, "terminalControl.batchAlreadyRunning", err)
	case errors.Is(err, terminal.ErrBatchTaskNotFound):
		return errno.Wrap(errno.CodeNotFound, "terminalControl.batchTaskNotFound", err)
	case errors.Is(err, terminal.ErrInvalidControlRequest), errors.Is(err, terminal.ErrBatchContentRequired), errors.Is(err, terminal.ErrBatchTargetRequired), errors.Is(err, terminal.ErrBatchRepeatOutOfRange), errors.Is(err, terminal.ErrBatchIntervalInvalid):
		return errno.Wrap(errno.CodeInvalidArg, "terminalControl.invalidBatchRequest", err)
	case errors.Is(err, terminal.ErrWriterQueueFull):
		return errno.Wrap(errno.CodeConnFailed, "terminalControl.error", err)
	case errors.Is(err, terminal.ErrControlNotReady):
		return errno.Wrap(errno.CodeSessionClosed, "terminalControl.notReady", err)
	default:
		return publicError(err)
	}
}

var _ terminal.SessionWriteFunc = (terminalSessionWriter{}).Write
