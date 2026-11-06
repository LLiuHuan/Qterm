package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	defaultTerminalType = "xterm-256color"
	maxTerminalColumns  = 1000
	maxTerminalRows     = 1000
)

var (
	ErrInvalidTerminalSize = errors.New("invalid terminal size")
	ErrTerminalNotReady    = errors.New("SSH terminal is not ready")

	ErrTerminalHandshakeTimeout = errors.New("SSH terminal handshake timed out")
)

const terminalHandshakeTimeout = 15 * time.Second

type Terminal struct {
	session *ssh.Session
	stdin   io.WriteCloser
	reader  *io.PipeReader
	writer  *io.PipeWriter
	done    chan struct{}

	mu        sync.RWMutex
	waitErr   error
	closeOnce sync.Once
	closeErr  error
	stopCtx   func() bool

	x11Warning string
}

type TerminalEnvironmentVariable struct {
	Name  string
	Value string
}

type TerminalOptions struct {
	DirectoryFollow bool

	ShellIntegration bool

	EnvironmentVariables []TerminalEnvironmentVariable

	PostLoginCommand string

	PostLoginCommandDelay time.Duration

	TerminalType string

	NetworkDevice bool

	Encoding string

	X11 *X11ForwardingOptions
}

func (c *Connection) OpenTerminal(
	ctx context.Context,
	columns int,
	rows int,
) (*Terminal, error) {
	return c.OpenTerminalWithOptions(ctx, columns, rows, TerminalOptions{})
}

func (c *Connection) OpenTerminalWithOptions(
	ctx context.Context,
	columns int,
	rows int,
	options TerminalOptions,
) (*Terminal, error) {
	if ctx == nil {
		return nil, errors.New("terminal context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c == nil || c.client == nil {
		return nil, ErrTerminalNotReady
	}
	if err := validateTerminalSize(columns, rows); err != nil {
		return nil, err
	}
	startupCommand := ""
	if options.DirectoryFollow || options.ShellIntegration {

		var startupErr error
		startupCommand, startupErr = c.terminalStartupCommand(ctx, options.DirectoryFollow, options.ShellIntegration)
		if startupErr != nil && ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	slog.Debug("terminal open requested",
		"event", "terminal.open.requested",
		"columns", columns,
		"rows", rows,
	)
	session, err := c.newTerminalSession(terminalHandshakeTimeout)
	if err != nil {
		if errors.Is(err, ErrTerminalHandshakeTimeout) {
			return nil, ErrTerminalHandshakeTimeout
		}
		return nil, fmt.Errorf("create SSH session: %w", err)
	}
	opened := false
	defer func() {
		if !opened {
			_ = session.Close()
		}
	}()
	var handshakeTimedOut atomic.Bool
	handshakeTimer := time.AfterFunc(terminalHandshakeTimeout, func() {
		handshakeTimedOut.Store(true)
		_ = session.Close()
	})
	defer handshakeTimer.Stop()
	stdin, err := session.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open SSH terminal input: %w", err)
	}
	reader, writer := io.Pipe()

	session.Stdout = writer
	session.Stderr = writer
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	terminalType := options.TerminalType
	if terminalType == "" {
		terminalType = defaultTerminalType
	}
	if err := session.RequestPty(terminalType, rows, columns, modes); err != nil {
		_ = reader.Close()
		_ = writer.Close()
		if handshakeTimedOut.Load() {
			return nil, ErrTerminalHandshakeTimeout
		}
		return nil, fmt.Errorf("request SSH PTY: %w", err)
	}
	slog.Debug("terminal pty ready",
		"event", "terminal.open.pty_ready",
		"terminal_type", terminalType,
	)

	var x11Warning string
	if options.X11 != nil {
		if x11Err := requestX11Forwarding(c.client, session, *options.X11); x11Err != nil {
			x11Warning = "session.x11_unavailable"
			slog.Warn("terminal x11 forwarding unavailable",
				"event", "terminal.x11_unavailable",

				"stage", "request",
			)
		}
	}

	refused := applySessionEnvironment(session, options.EnvironmentVariables)

	if options.NetworkDevice {
		refused = nil
	}

	start := session.Shell
	if startupCommand != "" {

		start = func() error { return session.Start(startupCommand) }
	}
	if err := start(); err != nil {
		_ = reader.Close()
		_ = writer.Close()
		if handshakeTimedOut.Load() {
			return nil, ErrTerminalHandshakeTimeout
		}
		if startupCommand != "" {
			return nil, fmt.Errorf("start SSH shell with temporary integration: %w", err)
		}
		return nil, fmt.Errorf("start SSH shell: %w", err)
	}

	handshakeTimer.Stop()

	request := "shell"
	loginBanner := "openssh_delivered"
	if startupCommand != "" {
		request = "exec"
		loginBanner = "openssh_suppressed"
	}
	slog.Info("terminal shell start",
		"event", "terminal.start",
		"request", request,
		"directory_follow", options.DirectoryFollow,
		"terminal_type", terminalType,
		"startup_command_bytes", len(startupCommand),
		"login_banner", loginBanner,
	)

	terminal := &Terminal{
		session: session,
		stdin:   stdin,
		reader:  reader,
		writer:  writer,
		done:    make(chan struct{}),
	}
	terminal.x11Warning = x11Warning
	terminal.stopCtx = context.AfterFunc(ctx, func() { _ = terminal.Close() })
	opened = true
	go terminal.wait()

	if options.PostLoginCommandDelay > 0 {
		delay := options.PostLoginCommandDelay
		go func() {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-ctx.Done():
				return
			}
			if err := writeLoginSetup(terminal, options, refused); err != nil {
				slog.Warn("terminal login setup failed",
					"event", "terminal.login_setup_failed",
					"stage", "delayed",
				)
			}
		}()
		return terminal, nil
	}
	if err := writeLoginSetup(terminal, options, refused); err != nil {
		slog.Warn("terminal login setup failed",
			"event", "terminal.login_setup_failed",
			"error", err,
		)
	}
	return terminal, nil
}

func (c *Connection) newTerminalSession(timeout time.Duration) (*ssh.Session, error) {
	type sessionResult struct {
		session *ssh.Session
		err     error
	}

	done := make(chan sessionResult, 1)
	go func() {
		session, err := c.client.NewSession()
		done <- sessionResult{session: session, err: err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case result := <-done:
		return result.session, result.err
	case <-timer.C:

		go func() {
			if result := <-done; result.session != nil {
				_ = result.session.Close()
			}
		}()
		return nil, ErrTerminalHandshakeTimeout
	}
}

func applySessionEnvironment(session *ssh.Session, values []TerminalEnvironmentVariable) []TerminalEnvironmentVariable {
	if session == nil || len(values) == 0 {
		return nil
	}
	var refused []TerminalEnvironmentVariable
	for _, item := range values {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		if err := session.Setenv(item.Name, item.Value); err != nil {
			refused = append(refused, item)
		}
	}
	if len(refused) > 0 {
		slog.Debug("terminal environment fallback",
			"event", "terminal.environment_fallback",
			"refused", len(refused),
		)
	}
	return refused
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func writeLoginSetup(
	terminal *Terminal,
	options TerminalOptions,
	refused []TerminalEnvironmentVariable,
) error {
	if terminal == nil {
		return ErrTerminalNotReady
	}
	var script strings.Builder
	if len(refused) > 0 {

		assignments := make([]string, 0, len(refused))
		for _, item := range refused {
			assignments = append(assignments, item.Name+"="+shellSingleQuote(item.Value))
		}
		script.WriteString("export ")
		script.WriteString(strings.Join(assignments, " "))
		script.WriteString("\r")
	}

	if command := strings.TrimSpace(options.PostLoginCommand); command != "" {
		for _, line := range strings.Split(command, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			script.WriteString(trimmed)
			script.WriteString("\r")
		}
	}
	if script.Len() == 0 {
		return nil
	}
	payload, err := encodeTerminalInput(script.String(), options.Encoding)
	if err != nil {
		return err
	}
	defer clear(payload)
	_, err = terminal.Write(payload)
	return err
}

func (t *Terminal) X11Warning() string {
	if t == nil {
		return ""
	}
	return t.x11Warning
}

func (t *Terminal) Read(target []byte) (int, error) {
	if t == nil || t.reader == nil {
		return 0, ErrTerminalNotReady
	}
	return t.reader.Read(target)
}

func (t *Terminal) Write(value []byte) (int, error) {
	if t == nil || t.stdin == nil {
		return 0, ErrTerminalNotReady
	}
	return t.stdin.Write(value)
}

func (t *Terminal) Resize(columns int, rows int) error {
	if t == nil || t.session == nil {
		return ErrTerminalNotReady
	}
	if err := validateTerminalSize(columns, rows); err != nil {
		return err
	}
	if err := t.session.WindowChange(rows, columns); err != nil {
		return fmt.Errorf("resize SSH terminal: %w", err)
	}
	return nil
}

func (t *Terminal) Wait() error {
	if t == nil || t.done == nil {
		return ErrTerminalNotReady
	}
	<-t.done
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.waitErr
}

func (t *Terminal) ExitStatus() (int, bool) {
	if t == nil || t.done == nil {
		return 0, false
	}
	select {
	case <-t.done:
	default:

		return 0, false
	}
	t.mu.RLock()
	err := t.waitErr
	t.mu.RUnlock()
	if err == nil {

		return 0, true
	}
	var exit *ssh.ExitError
	if errors.As(err, &exit) {
		return exit.ExitStatus(), true
	}
	return 0, false
}

func (t *Terminal) Close() error {
	if t == nil {
		return nil
	}
	t.closeOnce.Do(func() {
		if t.stopCtx != nil {
			t.stopCtx()
		}

		t.closeErr = errors.Join(
			normalizeTerminalCloseError(t.reader.Close()),
			normalizeTerminalCloseError(t.writer.Close()),
			normalizeTerminalCloseError(t.stdin.Close()),
			normalizeTerminalCloseError(t.session.Close()),
		)
	})
	return t.closeErr
}

func normalizeTerminalCloseError(err error) error {

	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func (t *Terminal) wait() {
	err := t.session.Wait()
	t.mu.Lock()
	t.waitErr = err
	t.mu.Unlock()
	_ = t.writer.CloseWithError(err)
	close(t.done)
}

func validateTerminalSize(columns int, rows int) error {
	if columns < 1 || columns > maxTerminalColumns || rows < 1 || rows > maxTerminalRows {
		return ErrInvalidTerminalSize
	}
	return nil
}
