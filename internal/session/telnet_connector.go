package session

import (
	"context"
	"errors"
	"sync"
	"time"

	"changeme/internal/errno"
	"changeme/internal/model"
	sshclient "changeme/internal/tunnel"
)

type TelnetConnector struct {
	hosts SessionHostSource

	mu sync.RWMutex

	failures map[string]string
}

func NewTelnetConnector(hosts SessionHostSource) *TelnetConnector {
	return &TelnetConnector{hosts: hosts, failures: make(map[string]string)}
}

func (c *TelnetConnector) SessionKind() model.SessionKind {
	return model.SessionKindTelnet
}

func (c *TelnetConnector) Connect(
	ctx context.Context,
	sessionID string,
	hostID string,
	_ func(model.SessionState) error,
) (SessionConnection, error) {
	if c == nil || c.hosts == nil {
		return nil, errno.New(errno.CodeSessionClosed, "session.not_ready", "")
	}
	if ctx == nil {
		return nil, errors.New("telnet connect context is nil")
	}
	host, err := c.hosts.SessionHost(ctx, hostID)
	if err != nil {
		c.rememberFailure(sessionID, "session.host_unavailable")
		return nil, err
	}
	if host.Protocol != model.ProtocolTelnet {

		c.rememberFailure(sessionID, "session.protocol_unsupported")
		return nil, errno.New(errno.CodeInvalidArg, "session.protocol_unsupported", "")
	}

	target := sshclient.TelnetTarget{
		Address:      host.Address,
		Port:         host.Port,
		TerminalType: host.TerminalType,

		Timeout: time.Duration(host.ConnectTimeoutSeconds) * time.Second,
		Columns: telnetSessionDefaultColumns,
		Rows:    telnetSessionDefaultRows,
	}
	terminal, err := sshclient.DialTelnet(ctx, target)
	if err != nil {

		key := SessionConnectionFailureKey(err)
		var connectErr *sshclient.ConnectError
		if errors.As(err, &connectErr) && connectErr.Stage == sshclient.StageTCP {
			key = "session.telnet_unreachable"
		}
		c.rememberFailure(sessionID, key)
		return nil, err
	}
	return &telnetConnection{terminal: terminal, encoding: host.Encoding}, nil
}

func (c *TelnetConnector) Failure(sessionID string) (string, bool) {
	if c == nil {
		return "", false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	message, ok := c.failures[sessionID]
	return message, ok
}

func (c *TelnetConnector) Forget(sessionID string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.failures, sessionID)
}

func (c *TelnetConnector) rememberFailure(sessionID string, message string) {
	if c == nil || sessionID == "" || message == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures[sessionID] = message
}

const (
	telnetSessionDefaultColumns = 80
	telnetSessionDefaultRows    = 24
)

type telnetConnection struct {
	terminal *sshclient.TelnetTerminal

	encoding string
}

func (c *telnetConnection) Wait() error {
	if c == nil || c.terminal == nil {
		return nil
	}
	return c.terminal.Wait()
}

func (c *telnetConnection) Close() error {
	if c == nil || c.terminal == nil {
		return nil
	}
	return c.terminal.Close()
}

func (c *telnetConnection) OpenTerminal(
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

func (c *telnetConnection) terminalCharacterEncoding() string {
	if c == nil {
		return ""
	}
	return c.encoding
}

var (
	_ SessionConnector          = (*TelnetConnector)(nil)
	_ sessionKindProvider       = (*TelnetConnector)(nil)
	_ failureSource             = (*TelnetConnector)(nil)
	_ SessionConnection         = (*telnetConnection)(nil)
	_ terminalCapableConnection = (*telnetConnection)(nil)
	_ terminalEncodingProvider  = (*telnetConnection)(nil)
)
