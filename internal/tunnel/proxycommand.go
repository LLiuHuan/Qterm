package tunnel

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var (
	ErrProxyCommandEmpty = errors.New("proxy command is empty")

	ErrProxyCommandInvalid = errors.New("invalid proxy command")

	ErrProxyCommandStart = errors.New("proxy command failed to start")
)

const maxProxyCommandLength = 4096

type ProxyCommandSpec struct {
	Command string
}

func expandProxyCommand(command, host string, port int) string {
	if command == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(command) + len(host))
	for index := 0; index < len(command); index++ {
		char := command[index]
		if char != '%' || index+1 >= len(command) {
			builder.WriteByte(char)
			continue
		}
		switch command[index+1] {
		case '%':
			builder.WriteByte('%')
			index++
		case 'h':
			builder.WriteString(host)
			index++
		case 'p':
			builder.WriteString(fmt.Sprintf("%d", port))
			index++
		default:

			builder.WriteByte(char)
		}
	}
	return builder.String()
}

func NewProxyCommandDialer(spec ProxyCommandSpec) (DialerFunc, error) {
	if err := ValidateProxyCommand(spec.Command); err != nil {
		return nil, err
	}
	command := strings.TrimSpace(spec.Command)
	return func(ctx context.Context, _, address string) (net.Conn, error) {
		if ctx == nil {
			ctx = context.Background()
		}
		host, portText, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrProxyCommandInvalid, err)
		}
		port := 0
		if _, scanErr := fmt.Sscanf(portText, "%d", &port); scanErr != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("%w: bad port", ErrProxyCommandInvalid)
		}
		return startProxyCommand(ctx, expandProxyCommand(command, host, port))
	}, nil
}

func ValidateProxyCommand(command string) error {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return ErrProxyCommandEmpty
	}
	if len(trimmed) > maxProxyCommandLength || strings.IndexByte(trimmed, 0) >= 0 {
		return ErrProxyCommandInvalid
	}
	return nil
}

func startProxyCommand(ctx context.Context, command string) (net.Conn, error) {

	childStdin, parentStdin, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("%w: create stdin pipe", ErrProxyCommandStart)
	}
	parentStdout, childStdout, err := os.Pipe()
	if err != nil {
		_ = childStdin.Close()
		_ = parentStdin.Close()
		return nil, fmt.Errorf("%w: create stdout pipe", ErrProxyCommandStart)
	}
	process := shellFor(command)
	process.Stdin = childStdin
	process.Stdout = childStdout

	process.Stderr = nil
	if err := process.Start(); err != nil {
		_ = childStdin.Close()
		_ = parentStdin.Close()
		_ = parentStdout.Close()
		_ = childStdout.Close()

		return nil, fmt.Errorf("%w: %v", ErrProxyCommandStart, err)
	}

	_ = childStdin.Close()
	_ = childStdout.Close()
	conn := &proxyCommandConn{process: process, reader: parentStdout, writer: parentStdin}
	conn.armDialGuard(ctx)
	return conn, nil
}

func (c *proxyCommandConn) armDialGuard(ctx context.Context) {
	c.stopCancel = context.AfterFunc(ctx, func() {
		_ = c.reader.Close()
		_ = c.writer.Close()
	})
}

func (c *proxyCommandConn) ReleaseDialGuard() {
	if c.stopCancel != nil && c.stopCancel() {
		c.stopCancel = nil
	}
}

type proxyCommandConn struct {
	process    *exec.Cmd
	reader     *os.File
	writer     *os.File
	stopCancel func() bool
	closeOnce  sync.Once
	closeErr   error
}

func (c *proxyCommandConn) Read(p []byte) (int, error)  { return c.reader.Read(p) }
func (c *proxyCommandConn) Write(p []byte) (int, error) { return c.writer.Write(p) }

func (c *proxyCommandConn) Close() error {
	c.closeOnce.Do(func() {
		if c.stopCancel != nil {
			c.stopCancel()
		}
		_ = c.reader.Close()
		_ = c.writer.Close()

		killProxyCommand(c.process)
		_ = c.process.Wait()
	})
	return c.closeErr
}

func (c *proxyCommandConn) LocalAddr() net.Addr  { return proxyCommandAddr{} }
func (c *proxyCommandConn) RemoteAddr() net.Addr { return proxyCommandAddr{} }

func (c *proxyCommandConn) SetDeadline(_ time.Time) error      { return nil }
func (c *proxyCommandConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *proxyCommandConn) SetWriteDeadline(_ time.Time) error { return nil }

type proxyCommandAddr struct{}

func (proxyCommandAddr) Network() string { return "proxy-command" }
func (proxyCommandAddr) String() string  { return "proxy-command" }
