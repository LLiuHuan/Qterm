package session

import (
	"context"
	"errors"
	"sync"

	"go.bug.st/serial"

	"changeme/internal/errno"
	"changeme/internal/model"
)

type SerialConnector struct {
	hosts SessionHostSource

	mu       sync.RWMutex
	failures map[string]string
}

func NewSerialConnector(hosts SessionHostSource) *SerialConnector {
	return &SerialConnector{hosts: hosts, failures: make(map[string]string)}
}

func (c *SerialConnector) SessionKind() model.SessionKind {
	return model.SessionKindTelnet
}

func (c *SerialConnector) Connect(
	ctx context.Context,
	sessionID string,
	hostID string,
	_ func(model.SessionState) error,
) (SessionConnection, error) {
	if c == nil || c.hosts == nil {
		return nil, errno.New(errno.CodeSessionClosed, "session.not_ready", "")
	}
	if ctx == nil {
		return nil, errors.New("serial connect context is nil")
	}
	host, err := c.hosts.SessionHost(ctx, hostID)
	if err != nil {
		c.rememberFailure(sessionID, "session.host_unavailable")
		return nil, err
	}
	if host.Protocol != model.ProtocolSerial {
		c.rememberFailure(sessionID, "session.protocol_unsupported")
		return nil, errno.New(errno.CodeInvalidArg, "session.protocol_unsupported", "")
	}
	if host.Address == "" {
		c.rememberFailure(sessionID, "session.serial_device_missing")
		return nil, errno.New(errno.CodeInvalidArg, "session.serial_device_missing", "")
	}

	port, openErr := serial.Open(host.Address, &serial.Mode{
		BaudRate: serialBaudOrDefault(host.SerialBaud),
		DataBits: serialDataBitsOrDefault(host.SerialDataBits),
		Parity:   serialParityOrDefault(host.SerialParity),
		StopBits: serialStopBitsOrDefault(host.SerialStopBits),
	})
	if openErr != nil {

		c.rememberFailure(sessionID, "session.serial_open_failed")
		return nil, errno.New(errno.CodeSessionClosed, "session.serial_open_failed", "")
	}
	return &serialConnection{port: port}, nil
}

func (c *SerialConnector) Failure(sessionID string) (string, bool) {
	if c == nil {
		return "", false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	message, ok := c.failures[sessionID]
	return message, ok
}

func (c *SerialConnector) Forget(sessionID string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.failures, sessionID)
}

func (c *SerialConnector) rememberFailure(sessionID, message string) {
	if c == nil || sessionID == "" || message == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures[sessionID] = message
}

const DefaultSerialBaud = 115200

type serialConnection struct {
	port serial.Port

	closeOnce sync.Once
	closeErr  error
	done      chan struct{}
	doneOnce  sync.Once
}

func (c *serialConnection) Wait() error {
	if c == nil {
		return nil
	}
	c.ensureDone()
	<-c.done
	if c.closeErr != nil {
		return c.closeErr
	}
	return nil
}

func (c *serialConnection) Close() error {
	if c == nil {
		return nil
	}
	c.closeOnce.Do(func() {
		if c.port != nil {
			c.closeErr = c.port.Close()
		}
		c.ensureDone()
		c.doneOnce.Do(func() { close(c.done) })
	})
	return c.closeErr
}

func (c *serialConnection) ensureDone() {
	if c.done == nil {
		c.done = make(chan struct{})
	}
}

func (c *serialConnection) OpenTerminal(
	ctx context.Context,
	_ int,
	_ int,
) (SessionTerminal, error) {
	if ctx == nil {
		return nil, errors.New("terminal context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c == nil || c.port == nil {
		return nil, ErrSessionNotConnected
	}
	return &serialTerminal{conn: c}, nil
}

type serialTerminal struct {
	conn *serialConnection
}

func (t *serialTerminal) Read(p []byte) (int, error) {
	if t == nil || t.conn == nil || t.conn.port == nil {
		return 0, ErrSessionNotConnected
	}
	return t.conn.port.Read(p)
}

func (t *serialTerminal) Write(p []byte) (int, error) {
	if t == nil || t.conn == nil || t.conn.port == nil {
		return 0, ErrSessionNotConnected
	}
	return t.conn.port.Write(p)
}

func (t *serialTerminal) Resize(int, int) error {
	return nil
}

func (t *serialTerminal) Wait() error {
	if t == nil || t.conn == nil {
		return nil
	}
	return t.conn.Wait()
}

func (t *serialTerminal) Close() error {
	if t == nil || t.conn == nil {
		return nil
	}
	return t.conn.Close()
}

var (
	_ SessionConnector          = (*SerialConnector)(nil)
	_ sessionKindProvider       = (*SerialConnector)(nil)
	_ failureSource             = (*SerialConnector)(nil)
	_ SessionConnection         = (*serialConnection)(nil)
	_ terminalCapableConnection = (*serialConnection)(nil)
	_ SessionTerminal           = (*serialTerminal)(nil)
)

func serialBaudOrDefault(value int) int {
	if value <= 0 {
		return DefaultSerialBaud
	}
	return value
}

func serialDataBitsOrDefault(value int) int {
	switch value {
	case 5, 6, 7, 8:
		return value
	default:
		return 8
	}
}

func serialParityOrDefault(value string) serial.Parity {
	switch value {
	case "odd":
		return serial.OddParity
	case "even":
		return serial.EvenParity
	case "mark":
		return serial.MarkParity
	case "space":
		return serial.SpaceParity
	default:
		return serial.NoParity
	}
}

func serialStopBitsOrDefault(value string) serial.StopBits {
	switch value {
	case "1.5":
		return serial.OnePointFiveStopBits
	case "2":
		return serial.TwoStopBits
	default:
		return serial.OneStopBit
	}
}
