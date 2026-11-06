package session

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"changeme/internal/model"
	sshclient "changeme/internal/tunnel"
)

type managedSession struct {
	mu                sync.RWMutex
	snapshot          model.Session
	connector         SessionConnector
	ctx               context.Context
	cancel            context.CancelFunc
	connection        SessionConnection
	terminal          *terminalHub
	terminalCancel    context.CancelFunc
	terminalOpening   bool
	fileSystem        SessionFileSystem
	fileSystemCancel  context.CancelFunc
	fileSystemOpening bool
	fileSystemReady   chan struct{}
	ready             chan struct{}
	readyOnce         sync.Once
	done              chan struct{}
	closeOnce         sync.Once
	resourceOnce      sync.Once
	finishOnce        sync.Once

	now func() time.Time

	expectedRebindUntil time.Time
}

func (s *managedSession) current() model.Session {
	s.mu.RLock()
	snapshot := s.snapshot
	hub := s.terminal
	s.mu.RUnlock()
	if hub != nil {
		snapshot.AttachEpoch = hub.AttachmentState().AttachEpoch

		if snapshot.State == model.SessionStateClosed || snapshot.State == model.SessionStateFailed {
			if code, known := hub.ExitStatus(); known {
				value := code
				snapshot.ExitCode = &value
			}
		}
	}
	return snapshot
}

func (s *managedSession) sessionNow() time.Time {
	if s.now == nil {
		return time.Now()
	}
	return s.now()
}

func (s *managedSession) armExpectedRebind(now time.Time) {
	s.mu.Lock()
	s.expectedRebindUntil = now.Add(orphanGracePeriod)
	s.mu.Unlock()
}

func (s *managedSession) handoverPendingLocked(now time.Time) bool {
	return !s.expectedRebindUntil.IsZero() && now.Before(s.expectedRebindUntil)
}

func (s *managedSession) orphanExpired(now time.Time) bool {
	s.mu.RLock()
	state := s.snapshot.State
	hub := s.terminal
	handoverPending := s.handoverPendingLocked(now)
	s.mu.RUnlock()

	if state == model.SessionStateClosing || state == model.SessionStateClosed || state == model.SessionStateFailed {
		return false
	}
	if handoverPending {
		return false
	}
	if hub == nil {

		return false
	}
	attachment := hub.AttachmentState()
	if !attachment.AttachedAtLeastOnce || attachment.Attached {
		return false
	}

	return now.Sub(attachment.LastDetachAt) >= orphanGracePeriod
}

func (s *managedSession) transition(state model.SessionState, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ctx.Err(); err != nil {
		return err
	}
	next, err := s.snapshot.Transition(state, now)
	if err != nil {
		return err
	}
	s.snapshot = next
	return nil
}

func (s *managedSession) attach(connection SessionConnection, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil || s.snapshot.State == model.SessionStateClosing {
		return false
	}
	next, err := s.snapshot.Transition(model.SessionStateConnected, now)
	if err != nil {
		return false
	}
	s.connection = connection
	s.snapshot = next
	s.readyOnce.Do(func() { close(s.ready) })
	return true
}

func (s *managedSession) openTerminal(
	ctx context.Context,
	columns int,
	rows int,
) (SessionTerminal, error) {
	if ctx == nil {
		return nil, errors.New("terminal context is nil")
	}
	select {
	case <-s.ready:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	s.mu.Lock()
	if s.snapshot.State != model.SessionStateConnected || s.connection == nil {
		s.mu.Unlock()
		return nil, ErrSessionNotConnected
	}
	if s.terminal != nil {
		terminal := s.terminal.attach()
		s.mu.Unlock()
		return terminal, nil
	}
	if s.terminalOpening {
		s.mu.Unlock()
		return nil, ErrSessionTerminalAttached
	}
	connection, ok := s.connection.(terminalCapableConnection)
	if !ok {
		s.mu.Unlock()
		return nil, ErrSessionTerminalUnsupported
	}
	s.terminalOpening = true
	terminalCtx, terminalCancel := context.WithCancel(s.ctx)
	s.mu.Unlock()

	stopCallerCancellation := context.AfterFunc(ctx, terminalCancel)
	terminal, err := connection.OpenTerminal(terminalCtx, columns, rows)
	stopCallerCancellation()
	if ctx.Err() != nil {
		if terminal != nil {
			_ = terminal.Close()
		}
		err = ctx.Err()
	}

	s.mu.Lock()
	s.terminalOpening = false
	if err != nil || terminal == nil || s.snapshot.State != model.SessionStateConnected || s.ctx.Err() != nil {
		s.mu.Unlock()
		terminalCancel()
		if terminal != nil {
			_ = terminal.Close()
		}
		if err != nil {
			return nil, err
		}
		return nil, ErrSessionNotConnected
	}

	encoding := "utf-8"
	if provider, ok := s.connection.(terminalEncodingProvider); ok {
		encoding = provider.terminalCharacterEncoding()
	}
	hub := newTerminalHubWithClock(terminal, s.sessionNow, encoding)

	hub.sessionID = s.snapshot.ID
	s.terminal = hub
	s.terminalCancel = terminalCancel
	s.mu.Unlock()
	return hub.attach(), nil
}

func (s *managedSession) setTerminalRecorder(recorder TerminalRecorder) error {
	return s.setTerminalRecorderFor("segment", recorder)
}

func (s *managedSession) setTerminalRecorderFor(key string, recorder TerminalRecorder) error {
	s.mu.RLock()
	hub := s.terminal
	s.mu.RUnlock()
	if hub == nil {
		return ErrSessionTerminalUnsupported
	}
	return hub.SetRecorderFor(key, recorder)
}

func (s *managedSession) requireTerminal() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.snapshot.State != model.SessionStateConnected || s.connection == nil {
		return ErrSessionNotConnected
	}
	if s.terminal == nil {
		return ErrSessionTerminalUnsupported
	}
	return nil
}

func (s *managedSession) writeSensitive(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if err := s.requireTerminal(); err != nil {
		return err
	}
	s.mu.RLock()
	hub := s.terminal
	s.mu.RUnlock()
	if hub == nil {
		return ErrSessionTerminalUnsupported
	}
	_, err := hub.WriteSensitive(data)
	return err
}

func (s *managedSession) writeInput(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if err := s.requireTerminal(); err != nil {
		return err
	}
	s.mu.RLock()
	hub := s.terminal
	s.mu.RUnlock()
	if hub == nil {
		return ErrSessionTerminalUnsupported
	}
	_, err := hub.WriteInput(data)
	return err
}

func (s *managedSession) dockerExecutor(ctx context.Context) (dockerCommandRunner, error) {
	if ctx == nil {
		return nil, errors.New("docker command context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case <-s.ready:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	s.mu.RLock()
	connection := s.connection
	state := s.snapshot.State
	s.mu.RUnlock()
	if state != model.SessionStateConnected || connection == nil {
		return nil, ErrSessionNotConnected
	}

	executor, ok := connection.(dockerCommandRunner)
	if !ok {
		return nil, ErrDockerUnsupported
	}
	return executor, nil
}

func (c *productionSessionConnection) RunDockerCommand(
	ctx context.Context,
	args []string,
) ([]byte, int, error) {
	if c == nil || c.connection == nil {
		return nil, -1, ErrSessionNotConnected
	}
	return c.connection.RunDockerCommand(ctx, args)
}

func (s *managedSession) pause(state model.SessionState, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil || s.snapshot.State.Terminal() || s.snapshot.State == model.SessionStateClosing {
		return false
	}
	next, err := s.snapshot.Transition(state, now)
	if err != nil {
		return false
	}
	s.snapshot = next
	return true
}

func (s *managedSession) openFileSystem(ctx context.Context) (SessionFileSystem, error) {
	if ctx == nil {
		return nil, errors.New("SFTP context is nil")
	}
	for {
		select {
		case <-s.ready:
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		s.mu.Lock()
		if s.snapshot.State != model.SessionStateConnected || s.connection == nil {
			s.mu.Unlock()
			return nil, ErrSessionNotConnected
		}
		if s.fileSystem != nil {
			fileSystem := s.fileSystem
			s.mu.Unlock()
			return fileSystem, nil
		}
		if s.fileSystemOpening {
			ready := s.fileSystemReady
			s.mu.Unlock()
			select {
			case <-ready:
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		connection, ok := s.connection.(fileSystemCapableConnection)
		if !ok {
			s.mu.Unlock()
			return nil, ErrSessionFileSystemUnsupported
		}
		s.fileSystemOpening = true
		s.fileSystemReady = make(chan struct{})
		ready := s.fileSystemReady
		fileSystemCtx, fileSystemCancel := context.WithCancel(s.ctx)
		s.mu.Unlock()

		stopCallerCancellation := context.AfterFunc(ctx, fileSystemCancel)
		fileSystem, err := connection.OpenFileSystem(fileSystemCtx)
		stopCallerCancellation()
		if ctx.Err() != nil {
			if fileSystem != nil {
				_ = fileSystem.Close()
			}
			err = ctx.Err()
		}

		s.mu.Lock()
		s.fileSystemOpening = false
		if err == nil && fileSystem != nil && s.snapshot.State == model.SessionStateConnected && s.ctx.Err() == nil {
			s.fileSystem = fileSystem
			s.fileSystemCancel = fileSystemCancel
		}
		close(ready)
		s.mu.Unlock()

		if err != nil || fileSystem == nil || s.ctx.Err() != nil {
			fileSystemCancel()
			if fileSystem != nil {
				_ = fileSystem.Close()
			}
			if err != nil {
				return nil, err
			}
			return nil, ErrSessionNotConnected
		}
		return fileSystem, nil
	}
}

func (s *managedSession) openTransferFileSystem(ctx context.Context, expectedSize int64) (SessionTransferFileSystem, error) {
	if ctx == nil {
		return nil, errors.New("SFTP transfer context is nil")
	}
	select {
	case <-s.ready:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	s.mu.RLock()
	if s.snapshot.State != model.SessionStateConnected || s.connection == nil {
		s.mu.RUnlock()
		return nil, ErrSessionNotConnected
	}
	connection, ok := s.connection.(fileSystemCapableConnection)
	s.mu.RUnlock()
	if !ok {
		return nil, ErrSessionFileSystemUnsupported
	}
	planned := 1
	if expectedSize > 0 {
		planned = s.planTransferConnections(ctx, connection, expectedSize)
	}

	fileSystemCtx, fileSystemCancel := context.WithCancel(s.ctx)
	stopCallerCancellation := context.AfterFunc(ctx, fileSystemCancel)
	var fileSystem SessionFileSystem
	var err error
	if opener, ok := connection.(transferFileSystemOpener); ok {
		fileSystem, err = opener.OpenFileSystemForConnections(fileSystemCtx, planned)
	} else {

		if planner, plannerOK := connection.(fileSystemConnectionPlanner); plannerOK {
			planner.SetFileSystemConnections(planned)
		}
		fileSystem, err = connection.OpenFileSystem(fileSystemCtx)
	}
	stopCallerCancellation()
	if contextErr := ctx.Err(); contextErr != nil {
		fileSystemCancel()
		if fileSystem != nil {
			_ = fileSystem.Close()
		}
		return nil, contextErr
	}
	fileSystemCancel()
	if err != nil {
		return nil, err
	}
	transferFileSystem, ok := fileSystem.(SessionTransferFileSystem)
	if !ok {
		_ = fileSystem.Close()
		return nil, ErrSessionFileSystemUnsupported
	}
	return transferFileSystem, nil
}

func (s *managedSession) planTransferConnections(
	ctx context.Context,
	connection fileSystemCapableConnection,
	expectedSize int64,
) int {
	var latency time.Duration

	if measured, measuredOK := connection.(latencyEnabledConnection); measuredOK && measured.LatencyEnabled() {
		if probe, probeOK := connection.(latencyCapableConnection); probeOK {
			probeCtx, cancel := context.WithTimeout(ctx, connectionPlanProbeTimeout)
			if value, err := probe.MeasureLatency(probeCtx); err == nil {
				latency = value
			}
			cancel()
		}
	}
	planned := sshclient.ParallelConnectionsFor(expectedSize, latency, sshclient.ParallelConnectionLimit)

	slog.Info("sftp transfer connection plan",
		"event", "sftp.plan",
		"size_bytes", expectedSize,
		"latency_ms", latency.Milliseconds(),
		"planned_connections", planned)
	return planned
}

const connectionPlanProbeTimeout = 2 * time.Second

func (s *managedSession) requestClose(now time.Time, reason model.SessionCloseReason) {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		if !s.snapshot.State.Terminal() && s.snapshot.State != model.SessionStateClosing {
			if next, err := s.snapshot.Transition(model.SessionStateClosing, now); err == nil {
				s.snapshot = next
			}
		}
		s.setCloseReasonLocked(reason)
		connection := s.connection
		s.cancel()
		select {
		case <-s.done:
			if s.snapshot.State == model.SessionStateClosing {
				if next, err := s.snapshot.Transition(model.SessionStateClosed, now); err == nil {
					s.snapshot = next
				}
			}
		default:
		}
		s.mu.Unlock()
		if connection != nil {
			s.closeConnection()
		}
	})
}

func (s *managedSession) closeConnection() {
	s.resourceOnce.Do(func() {
		s.mu.RLock()
		terminal := s.terminal
		terminalCancel := s.terminalCancel
		connection := s.connection
		fileSystem := s.fileSystem
		fileSystemCancel := s.fileSystemCancel
		s.mu.RUnlock()
		if terminalCancel != nil {
			terminalCancel()
		}
		if terminal != nil {
			_ = terminal.Close()

			terminal.releaseHistory()
		}
		if fileSystemCancel != nil {
			fileSystemCancel()
		}
		if fileSystem != nil {
			_ = fileSystem.Close()
		}
		if connection != nil {
			_ = connection.Close()
		}
	})
}

func (s *managedSession) finishConnectError(now time.Time) {
	if s.ctx.Err() != nil {

		s.closeNormally(now, model.SessionCloseReasonError)
		return
	}
	s.fail(now)
}

func (s *managedSession) setCloseReasonLocked(reason model.SessionCloseReason) {
	if s.snapshot.CloseReason != "" || reason == "" {
		return
	}
	s.snapshot.CloseReason = reason
}

func (s *managedSession) closeNormally(now time.Time, reason model.SessionCloseReason) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.snapshot.State.Terminal() {
		return
	}
	if s.snapshot.State != model.SessionStateClosing {
		next, err := s.snapshot.Transition(model.SessionStateClosing, now)
		if err != nil {
			return
		}
		s.snapshot = next
	}
	s.setCloseReasonLocked(reason)
	if next, err := s.snapshot.Transition(model.SessionStateClosed, now); err == nil {
		s.snapshot = next
	}
}

func (s *managedSession) fail(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.snapshot.State.Terminal() || s.snapshot.State == model.SessionStateClosing {
		return
	}

	s.setCloseReasonLocked(model.SessionCloseReasonError)
	if next, err := s.snapshot.Transition(model.SessionStateFailed, now); err == nil {
		s.snapshot = next
	}
}

func (s *managedSession) finish() {
	s.finishOnce.Do(func() {
		s.cancel()
		s.readyOnce.Do(func() { close(s.ready) })
		close(s.done)
	})
}
