package app

import (
	"context"
	"strings"
	"time"

	"changeme/internal/errno"
)

type rdpWindowHandoverState uint8

const (
	rdpWindowHandoverPending rdpWindowHandoverState = iota

	rdpWindowHandoverClaimed

	rdpWindowHandoverReturning

	rdpWindowHandoverReturned

	rdpWindowHandoverCancelled

	rdpWindowHandoverClosed
)

const rdpWindowHandoverIndependent = rdpWindowHandoverClaimed

type rdpWindowHandover struct {
	ready  chan struct{}
	closed chan struct{}
	state  rdpWindowHandoverState

	windowClosed bool
}

func (s *Service) RdpOpenSessionWindow(ctx context.Context, sessionID, hostID, title string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return rdpError(errno.CodeInvalidArg, "rdp.windowSessionRequired", nil)
	}
	if s == nil || s.openWindow == nil {
		return rdpError(errno.CodeConnFailed, "rdp.serviceNotReady", nil)
	}

	if err := s.checkRdpHandoverSession(sessionID); err != nil {
		return publicError(err)
	}
	handover, err := s.beginRdpWindowHandover(sessionID)
	if err != nil {
		return publicError(err)
	}
	closeWindow := s.openWindow(ctx, rdpSessionWindowURL(sessionID, hostID, title), func() {
		s.rdpHandoverWindowClosed(sessionID, handover)
	})
	timer := time.NewTimer(sessionWindowClaimTimeout)
	defer timer.Stop()
	select {
	case <-handover.ready:
		return nil
	case <-handover.closed:

		return rdpError(errno.CodeSessionClosed, "rdp.windowHandoverClosed", nil)
	case <-ctx.Done():

		if state := s.cancelRdpWindowHandover(sessionID, handover); state != rdpWindowHandoverCancelled {
			return rdpHandoverResult(state)
		}
		if closeWindow != nil {
			closeWindow()
		}
		return ctx.Err()
	case <-timer.C:
		if state := s.cancelRdpWindowHandover(sessionID, handover); state != rdpWindowHandoverCancelled {
			return rdpHandoverResult(state)
		}
		if closeWindow != nil {
			closeWindow()
		}
		return rdpError(errno.CodeTimeout, "rdp.windowHandoverTimeout", nil)
	}
}

func (s *Service) RdpAdoptWindowSession(ctx context.Context, sessionID string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return rdpError(errno.CodeInvalidArg, "rdp.windowSessionRequired", nil)
	}
	if s == nil {
		return rdpError(errno.CodeConnFailed, "rdp.serviceNotReady", nil)
	}
	s.rdpWindowMu.Lock()
	handover := s.rdpWindowHandovers[sessionID]
	if handover == nil || handover.state != rdpWindowHandoverPending {
		s.rdpWindowMu.Unlock()
		return rdpError(errno.CodeNotFound, "rdp.windowHandoverExpired", nil)
	}

	handover.state = rdpWindowHandoverClaimed
	close(handover.ready)
	s.rdpWindowMu.Unlock()
	return nil
}

func (s *Service) RdpRegisterWindowSession(ctx context.Context, sessionID string) error {
	if err := validateRdpWindowContext(ctx); err != nil {
		return err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return rdpError(errno.CodeInvalidArg, "rdp.windowSessionRequired", nil)
	}
	if s == nil {
		return rdpError(errno.CodeConnFailed, "rdp.serviceNotReady", nil)
	}
	if err := s.checkRdpHandoverSession(sessionID); err != nil {
		return publicError(err)
	}

	s.rdpWindowMu.Lock()
	defer s.rdpWindowMu.Unlock()
	if s.rdpWindowHandovers == nil {
		s.rdpWindowHandovers = make(map[string]*rdpWindowHandover)
	}
	if existing := s.rdpWindowHandovers[sessionID]; existing != nil {
		if existing.state == rdpWindowHandoverIndependent {
			return nil
		}
		return rdpError(errno.CodeInvalidArg, "rdp.windowHandoverInProgress", nil)
	}
	s.rdpWindowHandovers[sessionID] = &rdpWindowHandover{
		ready:  make(chan struct{}),
		closed: make(chan struct{}),
		state:  rdpWindowHandoverIndependent,
	}
	return nil
}

func (s *Service) forgetRdpWindowSessionOwnership(sessionID string) {
	if s == nil {
		return
	}
	s.rdpWindowMu.Lock()
	delete(s.rdpWindowHandovers, strings.TrimSpace(sessionID))
	s.rdpWindowMu.Unlock()
}

func (s *Service) RdpPrepareReturnToMain(ctx context.Context, sessionID string) error {
	if err := validateRdpWindowContext(ctx); err != nil {
		return err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return rdpError(errno.CodeInvalidArg, "rdp.windowSessionRequired", nil)
	}
	if s == nil {
		return rdpError(errno.CodeConnFailed, "rdp.serviceNotReady", nil)
	}

	s.rdpWindowMu.Lock()
	defer s.rdpWindowMu.Unlock()
	handover := s.rdpWindowHandovers[sessionID]
	if handover == nil {
		return rdpError(errno.CodeNotFound, "rdp.windowHandoverExpired", nil)
	}
	switch handover.state {
	case rdpWindowHandoverClaimed:
		handover.state = rdpWindowHandoverReturning
		return nil
	case rdpWindowHandoverReturning:

		return nil
	case rdpWindowHandoverPending:
		return rdpError(errno.CodeInvalidArg, "rdp.windowHandoverInProgress", nil)
	default:
		return rdpError(errno.CodeNotFound, "rdp.windowHandoverExpired", nil)
	}
}

func (s *Service) RdpCompleteReturnToMain(ctx context.Context, sessionID string) error {
	if err := validateRdpWindowContext(ctx); err != nil {
		return err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return rdpError(errno.CodeInvalidArg, "rdp.windowSessionRequired", nil)
	}
	if s == nil {
		return rdpError(errno.CodeConnFailed, "rdp.serviceNotReady", nil)
	}

	s.rdpWindowMu.Lock()
	defer s.rdpWindowMu.Unlock()
	handover := s.rdpWindowHandovers[sessionID]
	if handover == nil || handover.state != rdpWindowHandoverReturning {
		return rdpError(errno.CodeNotFound, "rdp.windowHandoverExpired", nil)
	}

	handover.state = rdpWindowHandoverReturned
	delete(s.rdpWindowHandovers, sessionID)
	return nil
}

func (s *Service) RdpCancelReturnToMain(ctx context.Context, sessionID string) error {
	if err := validateRdpWindowContext(ctx); err != nil {
		return err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return rdpError(errno.CodeInvalidArg, "rdp.windowSessionRequired", nil)
	}
	if s == nil {
		return rdpError(errno.CodeConnFailed, "rdp.serviceNotReady", nil)
	}

	s.rdpWindowMu.Lock()
	defer s.rdpWindowMu.Unlock()
	handover := s.rdpWindowHandovers[sessionID]
	if handover == nil {
		return rdpError(errno.CodeNotFound, "rdp.windowHandoverExpired", nil)
	}
	switch handover.state {
	case rdpWindowHandoverReturning:
		handover.state = rdpWindowHandoverClaimed
		return nil
	case rdpWindowHandoverClaimed:

		return nil
	default:
		return rdpError(errno.CodeNotFound, "rdp.windowHandoverExpired", nil)
	}
}

func validateRdpWindowContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func (s *Service) beginRdpWindowHandover(sessionID string) (*rdpWindowHandover, error) {
	s.rdpWindowMu.Lock()
	defer s.rdpWindowMu.Unlock()
	if s.rdpWindowHandovers == nil {
		s.rdpWindowHandovers = make(map[string]*rdpWindowHandover)
	}
	if existing := s.rdpWindowHandovers[sessionID]; existing != nil && isRdpWindowHandoverActive(existing.state) {
		return nil, rdpError(errno.CodeInvalidArg, "rdp.windowHandoverInProgress", nil)
	}
	handover := &rdpWindowHandover{ready: make(chan struct{}), closed: make(chan struct{})}
	s.rdpWindowHandovers[sessionID] = handover
	return handover, nil
}

func (s *Service) cancelRdpWindowHandover(sessionID string, handover *rdpWindowHandover) rdpWindowHandoverState {
	s.rdpWindowMu.Lock()
	defer s.rdpWindowMu.Unlock()
	if handover.state == rdpWindowHandoverPending && s.rdpWindowHandovers[sessionID] == handover {
		handover.state = rdpWindowHandoverCancelled

		delete(s.rdpWindowHandovers, sessionID)
	}
	return handover.state
}

func (s *Service) rdpHandoverWindowClosed(sessionID string, handover *rdpWindowHandover) {
	closeSession := false
	s.rdpWindowMu.Lock()
	if s.rdpWindowHandovers[sessionID] != handover || handover.windowClosed {
		s.rdpWindowMu.Unlock()
		return
	}
	handover.windowClosed = true
	switch handover.state {
	case rdpWindowHandoverPending:
		handover.state = rdpWindowHandoverClosed
		delete(s.rdpWindowHandovers, sessionID)
	case rdpWindowHandoverClaimed, rdpWindowHandoverReturning:

		handover.state = rdpWindowHandoverClosed
		delete(s.rdpWindowHandovers, sessionID)
		closeSession = true
	default:

		delete(s.rdpWindowHandovers, sessionID)
	}
	close(handover.closed)
	s.rdpWindowMu.Unlock()
	if closeSession {

		_ = s.closeRdpWindowSession(sessionID)
	}
}

func rdpHandoverResult(state rdpWindowHandoverState) error {
	switch state {
	case rdpWindowHandoverClaimed:

		return nil
	case rdpWindowHandoverClosed:
		return rdpError(errno.CodeSessionClosed, "rdp.windowHandoverClosed", nil)
	case rdpWindowHandoverReturned:
		return nil
	default:
		return rdpError(errno.CodeTimeout, "rdp.windowHandoverTimeout", nil)
	}
}

func isRdpWindowHandoverActive(state rdpWindowHandoverState) bool {
	switch state {
	case rdpWindowHandoverPending, rdpWindowHandoverClaimed, rdpWindowHandoverReturning:
		return true
	default:
		return false
	}
}

func (s *Service) closeRdpWindowSession(sessionID string) error {
	if s == nil {
		return rdpError(errno.CodeConnFailed, "rdp.serviceNotReady", nil)
	}
	s.releaseRdpRouteBridge(sessionID)
	if s.rdpSessionCloser != nil {
		return s.rdpSessionCloser(sessionID)
	}
	manager, err := s.rdpManager()
	if err != nil {
		return err
	}
	return manager.Close(sessionID)
}

func (s *Service) checkRdpHandoverSession(sessionID string) error {
	if s.rdpSessionChecker != nil {
		return s.rdpSessionChecker(sessionID)
	}
	session, err := s.rdpSession(sessionID)
	if err != nil {
		return err
	}
	if session.Stats().Closed {
		return rdpError(errno.CodeSessionClosed, "rdp.sessionMissing", nil)
	}
	return nil
}
