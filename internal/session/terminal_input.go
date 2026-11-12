package session

import (
	"context"
	"io"
)

type terminalInputInterrupter interface{ InterruptTerminalWrite() }

func (s *managedSession) writeInputContext(ctx context.Context, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	if err := s.requireTerminal(); err != nil {
		return err
	}
	s.mu.RLock()
	hub, connection := s.terminal, s.connection
	s.mu.RUnlock()
	if hub == nil {
		return ErrSessionTerminalUnsupported
	}
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(interrupted)
		if transport, ok := connection.(terminalInputInterrupter); ok {
			transport.InterruptTerminalWrite()
		} else {

			_ = hub.raw.Close()
		}
	})
	defer func() {
		if !stop() {
			<-interrupted
		}
	}()
	for len(data) > 0 {
		n, err := hub.WriteInput(data)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return err
		}
		if n <= 0 || n > len(data) {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}
