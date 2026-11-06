package session

import (
	"context"
	"errors"

	"changeme/internal/errno"
	"changeme/internal/model"
	sshclient "changeme/internal/tunnel"
)

type LocalConnector struct {
	plan sshclient.LocalShellPlan
}

func NewLocalConnector(plan sshclient.LocalShellPlan) *LocalConnector {
	return &LocalConnector{plan: plan}
}

func (c *LocalConnector) Connect(
	ctx context.Context,
	_ string,
	_ string,
	_ func(model.SessionState) error,
) (SessionConnection, error) {
	terminal, err := sshclient.OpenLocalTerminal(ctx, sshclient.LocalTerminalOptions{Plan: c.plan})
	if err != nil {
		if errors.Is(err, sshclient.ErrLocalShellUnavailable) {
			return nil, errno.New(errno.CodeInvalidArg, "session.local_shell_unavailable", "")
		}
		return nil, errno.Wrap(errno.CodeConnFailed, "session.local_shell_failed", err)
	}
	return &localConnection{terminal: terminal}, nil
}

type localConnection struct {
	terminal *sshclient.LocalTerminal
}

func (c *localConnection) Wait() error {
	if c == nil || c.terminal == nil {
		return nil
	}
	return c.terminal.Wait()
}

func (c *localConnection) Close() error {
	if c == nil || c.terminal == nil {
		return nil
	}
	return c.terminal.Close()
}

func (c *localConnection) OpenTerminal(
	ctx context.Context,
	columns int,
	rows int,
) (SessionTerminal, error) {
	if ctx == nil {
		return nil, errors.New("terminal context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c == nil || c.terminal == nil || c.terminal.Closed() {
		return nil, ErrSessionNotConnected
	}
	if err := c.terminal.Resize(columns, rows); err != nil {
		return nil, err
	}
	return c.terminal, nil
}
