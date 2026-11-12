package session

import (
	"context"
	"errors"
	"io"
	"sync"

	"changeme/internal/model"
)

const (
	minAuxiliaryTerminalSize = 1
	maxAuxiliaryTerminalSize = 1000
)

var ErrAuxiliaryTerminalSizeInvalid = errors.New("terminal size is invalid")

type AuxiliaryTerminal struct {
	terminal SessionTerminal
	cancel   context.CancelFunc

	closeOnce sync.Once
	closeErr  error
}

func (t *AuxiliaryTerminal) Read(target []byte) (int, error) {
	if t == nil || t.terminal == nil {
		return 0, io.EOF
	}
	return t.terminal.Read(target)
}

func (t *AuxiliaryTerminal) Write(value []byte) (int, error) {
	if t == nil || t.terminal == nil {
		return 0, io.ErrClosedPipe
	}
	return t.terminal.Write(value)
}

func (t *AuxiliaryTerminal) Resize(columns int, rows int) error {
	if t == nil || t.terminal == nil {
		return io.ErrClosedPipe
	}
	if err := validateAuxiliaryTerminalSize(columns, rows); err != nil {
		return err
	}
	return t.terminal.Resize(columns, rows)
}

func (t *AuxiliaryTerminal) Close() error {
	if t == nil {
		return nil
	}
	t.closeOnce.Do(func() {
		if t.terminal != nil {
			t.closeErr = t.terminal.Close()
		}
		if t.cancel != nil {
			t.cancel()
		}
	})
	return t.closeErr
}

func (t *AuxiliaryTerminal) ExitCode() int {
	if t == nil || t.terminal == nil {
		return 0
	}
	reporter, ok := t.terminal.(terminalExitStatus)
	if !ok {
		return 0
	}
	code, known := reporter.ExitStatus()
	if !known {
		return 0
	}
	return code
}

func (s *Service) OpenAuxiliaryTerminal(
	ctx context.Context,
	sessionID string,
	columns int,
	rows int,
) (*AuxiliaryTerminal, error) {
	manager, _, err := s.ready()
	if err != nil {
		return nil, err
	}
	return manager.OpenAuxiliaryTerminal(ctx, sessionID, columns, rows)
}

func (m *SessionManager) OpenAuxiliaryTerminal(
	ctx context.Context,
	sessionID string,
	columns int,
	rows int,
) (*AuxiliaryTerminal, error) {
	worker, err := m.lookup(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return worker.openAuxiliaryTerminal(ctx, columns, rows)
}

func (s *managedSession) openAuxiliaryTerminal(
	ctx context.Context,
	columns int,
	rows int,
) (*AuxiliaryTerminal, error) {
	if err := validateAuxiliaryTerminalSize(columns, rows); err != nil {
		return nil, err
	}
	connection, err := s.connectedConnection(ctx)
	if err != nil {
		return nil, err
	}
	capable, ok := connection.(terminalCapableConnection)
	if !ok {
		return nil, ErrSessionTerminalUnsupported
	}

	terminalCtx, terminalCancel := context.WithCancel(s.ctx)
	stopCallerCancellation := context.AfterFunc(ctx, terminalCancel)
	terminal, err := capable.OpenTerminal(terminalCtx, columns, rows)
	stopCallerCancellation()

	if err != nil || terminal == nil {
		terminalCancel()
		if terminal != nil {
			_ = terminal.Close()
		}
		if err != nil {
			return nil, err
		}
		return nil, ErrSessionTerminalUnsupported
	}

	if ctx.Err() != nil {
		_ = terminal.Close()
		terminalCancel()
		return nil, ctx.Err()
	}
	if state := s.currentState(); state != model.SessionStateConnected {
		_ = terminal.Close()
		terminalCancel()
		return nil, ErrSessionNotConnected
	}
	return &AuxiliaryTerminal{terminal: terminal, cancel: terminalCancel}, nil
}

func validateAuxiliaryTerminalSize(columns int, rows int) error {
	if columns < minAuxiliaryTerminalSize || columns > maxAuxiliaryTerminalSize {
		return ErrAuxiliaryTerminalSizeInvalid
	}
	if rows < minAuxiliaryTerminalSize || rows > maxAuxiliaryTerminalSize {
		return ErrAuxiliaryTerminalSizeInvalid
	}
	return nil
}

func (s *managedSession) currentState() model.SessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot.State
}
