package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	gopty "github.com/aymanbagabas/go-pty"
)

const (
	localTerminalDefaultColumns = 80
	localTerminalDefaultRows    = 24
)

var (
	ErrLocalShellUnavailable = errors.New("local shell is unavailable")

	ErrLocalTerminalClosed = errors.New("local terminal is closed")
)

type LocalTerminalOptions struct {
	Columns int
	Rows    int

	Plan LocalShellPlan
}

type LocalTerminal struct {
	ptmx        gopty.Pty
	command     *gopty.Cmd
	integration localCommandIntegration

	closed atomic.Bool

	closeOnce sync.Once
	closeErr  error

	waitOnce sync.Once
	waitDone chan struct{}
	waitErr  error
}

func OpenLocalTerminal(ctx context.Context, options LocalTerminalOptions) (*LocalTerminal, error) {
	if ctx == nil {
		return nil, errors.New("local terminal context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	columns, rows := normalizeLocalTerminalSize(options.Columns, options.Rows)

	ptmx, err := gopty.New()
	if err != nil {
		return nil, fmt.Errorf("open local pty: %w", err)
	}
	if err := ptmx.Resize(columns, rows); err != nil {
		_ = ptmx.Close()
		return nil, fmt.Errorf("resize local pty: %w", err)
	}
	plan := options.Plan
	if plan.Path == "" {

		resolved, err := ResolveLocalShellPlan(LocalShellOptions{})
		if err != nil {
			_ = ptmx.Close()
			return nil, err
		}
		plan = resolved
	}
	integration := prepareLocalCommandIntegration(plan)

	command := ptmx.CommandContext(ctx, plan.Path, integration.commandArgs(plan.Args)...)
	command.Env = integration.applyEnvironment(localShellEnv())

	command.Dir = plan.WorkingDirectory
	if err := command.Start(); err != nil {
		_ = integration.cleanup()
		_ = ptmx.Close()
		return nil, fmt.Errorf("start local shell: %w", err)
	}
	return &LocalTerminal{
		ptmx:        ptmx,
		command:     command,
		integration: integration,
		waitDone:    make(chan struct{}),
	}, nil
}

func (t *LocalTerminal) Read(buffer []byte) (int, error) {
	if t == nil {
		return 0, ErrLocalTerminalClosed
	}
	count, err := t.ptmx.Read(buffer)
	if err != nil && count == 0 && (t.Closed() || t.exited() || isLocalPTYReadClosed(err)) {
		return 0, io.EOF
	}
	return count, err
}

func (t *LocalTerminal) Write(value []byte) (int, error) {
	if t == nil || t.Closed() {
		return 0, ErrLocalTerminalClosed
	}
	return t.ptmx.Write(value)
}

func (t *LocalTerminal) Resize(columns int, rows int) error {
	if err := validateTerminalSize(columns, rows); err != nil {
		return err
	}
	if t == nil || t.Closed() {
		return ErrLocalTerminalClosed
	}
	return t.ptmx.Resize(columns, rows)
}

func (t *LocalTerminal) Wait() error {
	if t == nil {
		return nil
	}
	t.waitOnce.Do(func() {
		err := t.command.Wait()
		if t.Closed() {
			err = nil
		}
		t.waitErr = err
		close(t.waitDone)
	})
	return t.waitErr
}

func (t *LocalTerminal) ExitStatus() (int, bool) {
	if t == nil || t.waitDone == nil {
		return 0, false
	}
	select {
	case <-t.waitDone:
	default:

		return 0, false
	}
	err := t.waitErr
	if err == nil {
		return 0, true
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), true
	}
	return 0, false
}

func (t *LocalTerminal) Close() error {
	if t == nil {
		return nil
	}
	t.closeOnce.Do(func() {

		t.closed.Store(true)
		firstErr := t.ptmx.Close()

		if t.command != nil && t.command.Process != nil {
			if err := t.command.Process.Kill(); err != nil &&
				!errors.Is(err, os.ErrProcessDone) && firstErr == nil {
				firstErr = err
			}
		}
		if err := t.integration.cleanup(); err != nil && firstErr == nil {
			firstErr = err
		}
		t.closeErr = firstErr
	})
	return t.closeErr
}

func (t *LocalTerminal) Closed() bool {
	return t == nil || t.closed.Load()
}

func (t *LocalTerminal) exited() bool {
	select {
	case <-t.waitDone:
		return true
	default:
		return false
	}
}

func normalizeLocalTerminalSize(columns int, rows int) (int, int) {
	if validateTerminalSize(columns, rows) != nil {
		return localTerminalDefaultColumns, localTerminalDefaultRows
	}
	return columns, rows
}
