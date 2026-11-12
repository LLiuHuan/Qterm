package sftp

import (
	"context"
	"errors"
	"io/fs"
	"strconv"
	"strings"
	"time"

	"changeme/internal/errno"
	"changeme/internal/model"
	sessionapp "changeme/internal/session"
)

const (
	elevatedCommandTimeout = 90 * time.Second

	elevatedSudoPasswordMarker = "password is required"
	elevatedSudoTerminalMarker = "terminal is required"
	elevatedMissingMarker      = "No such file or directory"
)

type CommandResult struct {
	Output   string
	ExitCode int
}

type CommandRunner interface {
	RunAIBackgroundCommand(ctx context.Context, sessionID, command string) (CommandResult, error)
}

type StdinCommandRunner interface {
	RunCommandWithStdin(ctx context.Context, sessionID, command string, stdin []byte) (CommandResult, error)
}

type commandRunner = CommandRunner

func (s *Service) SetCommandRunner(runner CommandRunner) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commandRunner = runner
}

func (s *Service) RemoveEntryElevated(ctx context.Context, sessionID, path string) error {
	s.mu.RLock()
	manager := s.manager
	runner := s.commandRunner
	s.mu.RUnlock()

	trimmedSession := strings.TrimSpace(sessionID)
	if trimmedSession == "" {
		return errno.New(errno.CodeInvalidArg, "session.not_found", "")
	}
	if runner == nil {
		return errno.New(errno.CodeInvalidArg, "sftp.elevated_unsupported", "")
	}
	normalized, err := model.NormalizeRemotePath(path)
	if err != nil {
		return errno.New(errno.CodeInvalidArg, "sftp.remote_path_invalid", "")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if manager != nil {
		if fileSystem, fsErr := manager.FileSystem(ctx, trimmedSession); fsErr == nil {
			_, statErr := fileSystem.StatContext(ctx, normalized)
			_ = fileSystem.Close()
			if errors.Is(statErr, fs.ErrNotExist) {
				return errno.New(errno.CodeNotFound, "sftp.remote_path_not_found", "")
			}
		}
	}

	command := BuildElevatedRemoveCommand(normalized)
	runCtx, cancel := context.WithTimeout(ctx, elevatedCommandTimeout)
	defer cancel()
	result, runErr := runner.RunAIBackgroundCommand(runCtx, trimmedSession, command)
	if runErr != nil {
		if errors.Is(runErr, sessionapp.ErrAIBackgroundUnsupported) {

			return errno.New(errno.CodeInvalidArg, "sftp.elevated_unsupported", "")
		}
		if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
			return runErr
		}
		return errno.Wrap(errno.CodeConnFailed, "sftp.elevated_failed", runErr)
	}
	if result.ExitCode == 0 {
		return nil
	}
	return elevatedFailure(result)
}

func (s *Service) RemoveEntryElevatedWithPassword(
	ctx context.Context,
	sessionID, path, password string,
) error {
	s.mu.RLock()
	manager := s.manager
	runner := s.commandRunner
	s.mu.RUnlock()

	trimmedSession := strings.TrimSpace(sessionID)
	if trimmedSession == "" {
		return errno.New(errno.CodeInvalidArg, "session.not_found", "")
	}

	if strings.TrimSpace(password) == "" {
		return errno.New(errno.CodeInvalidArg, "sftp.sudo_password_required", "")
	}
	stdinRunner, ok := runner.(StdinCommandRunner)
	if runner == nil || !ok {
		return errno.New(errno.CodeInvalidArg, "sftp.elevated_unsupported", "")
	}
	normalized, err := model.NormalizeRemotePath(path)
	if err != nil {
		return errno.New(errno.CodeInvalidArg, "sftp.remote_path_invalid", "")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if manager != nil {
		if fileSystem, fsErr := manager.FileSystem(ctx, trimmedSession); fsErr == nil {
			_, statErr := fileSystem.StatContext(ctx, normalized)
			_ = fileSystem.Close()
			if errors.Is(statErr, fs.ErrNotExist) {
				return errno.New(errno.CodeNotFound, "sftp.remote_path_not_found", "")
			}
		}
	}

	if strings.ContainsAny(password, "\r\n") {
		return errno.New(errno.CodeInvalidArg, "sftp.sudo_password_invalid", "")
	}

	stdin := append([]byte(password), '\n')
	defer func() {

		for index := range stdin {
			stdin[index] = 0
		}
	}()

	runCtx, cancel := context.WithTimeout(ctx, elevatedCommandTimeout)
	defer cancel()
	result, runErr := stdinRunner.RunCommandWithStdin(runCtx, trimmedSession, BuildElevatedRemoveCommandWithPassword(normalized), stdin)
	if runErr != nil {
		if errors.Is(runErr, sessionapp.ErrAIBackgroundUnsupported) {
			return errno.New(errno.CodeInvalidArg, "sftp.elevated_unsupported", "")
		}
		if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
			return runErr
		}
		return errno.Wrap(errno.CodeConnFailed, "sftp.elevated_failed", runErr)
	}
	if result.ExitCode == 0 {
		return nil
	}

	if lowered := strings.ToLower(result.Output); strings.Contains(lowered, "incorrect password") ||
		strings.Contains(lowered, "sorry, try again") {
		return errno.New(errno.CodePermissionDenied, "sftp.sudo_password_incorrect", "")
	}
	if result.ExitCode != 0 && strings.TrimSpace(result.Output) == "" {
		return errno.New(errno.CodePermissionDenied, "sftp.sudo_password_rejected", "")
	}
	return elevatedFailure(result)
}

func elevatedFailure(result CommandResult) error {
	lowered := strings.ToLower(result.Output)
	if strings.Contains(lowered, elevatedSudoPasswordMarker) || strings.Contains(lowered, elevatedSudoTerminalMarker) {
		return errno.New(errno.CodePermissionDenied, "sftp.sudo_requires_password", "")
	}
	if strings.Contains(result.Output, elevatedMissingMarker) {
		return errno.New(errno.CodeNotFound, "sftp.remote_path_not_found", "")
	}
	return errno.New(errno.CodeConnFailed, "sftp.elevated_failed", "exit "+strconv.Itoa(result.ExitCode))
}

func BuildElevatedRemoveCommand(path string) string {
	return "sudo -n rm -f -- " + shellSingleQuote(path)
}

func BuildElevatedRemoveCommandWithPassword(path string) string {
	return "sudo -S -p '' rm -f -- " + shellSingleQuote(path)
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
