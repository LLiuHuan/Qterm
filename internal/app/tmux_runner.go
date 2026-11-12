package app

import (
	"context"

	"changeme/internal/session"
	"changeme/internal/tmux"
)

type tmuxCommandRunner struct {
	sessions *session.Service
}

func (runner tmuxCommandRunner) RunAIBackgroundCommand(
	ctx context.Context,
	sessionID string,
	command string,
) (tmux.CommandResult, error) {
	if runner.sessions == nil {
		return tmux.CommandResult{}, tmux.ErrUnavailable
	}
	result, err := runner.sessions.RunAIBackgroundCommand(ctx, sessionID, command)
	if err != nil {
		return tmux.CommandResult{}, err
	}
	return tmux.CommandResult{Output: result.Output, ExitCode: result.ExitCode}, nil
}
