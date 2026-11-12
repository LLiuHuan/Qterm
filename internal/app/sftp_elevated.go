package app

import (
	"context"
	"strings"

	"changeme/internal/errno"
	"changeme/internal/session"
	"changeme/internal/sftp"
)

type sftpCommandRunner struct {
	sessions *session.Service
}

func (runner sftpCommandRunner) RunAIBackgroundCommand(ctx context.Context, sessionID, command string) (sftp.CommandResult, error) {
	result, err := runner.sessions.RunAIBackgroundCommand(ctx, sessionID, command)
	if err != nil {
		return sftp.CommandResult{}, err
	}
	return sftp.CommandResult{Output: result.Output, ExitCode: result.ExitCode}, nil
}

func (runner sftpCommandRunner) RunCommandWithStdin(
	ctx context.Context,
	sessionID, command string,
	stdin []byte,
) (sftp.CommandResult, error) {
	result, err := runner.sessions.RunAIBackgroundCommandWithStdin(ctx, sessionID, command, stdin)
	if err != nil {
		return sftp.CommandResult{}, err
	}
	return sftp.CommandResult{Output: result.Output, ExitCode: result.ExitCode}, nil
}

func (s *Service) RemoveSFTPEntryElevated(sessionID string, path string, password string) error {
	if s == nil || s.sftp == nil {
		return publicError(errno.New(errno.CodeStoreFailed, "error.serviceNotReady", ""))
	}
	if strings.TrimSpace(password) != "" {
		return publicError(s.sftp.RemoveEntryElevatedWithPassword(context.Background(), sessionID, path, password))
	}
	return publicError(s.sftp.RemoveEntryElevated(context.Background(), sessionID, path))
}
