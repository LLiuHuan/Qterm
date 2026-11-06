package session

import (
	"context"
	"errors"
	"time"

	sshclient "changeme/internal/tunnel"
)

const aiBackgroundTimeout = 2 * time.Minute

var ErrAIBackgroundUnsupported = errors.New("session does not support AI background commands")

type aiCommandRunner interface {
	RunAICommand(context.Context, string) (sshclient.AICommandResult, error)
}

var _ aiCommandRunner = (*productionSessionConnection)(nil)

func (c *productionSessionConnection) RunAICommand(ctx context.Context, command string) (sshclient.AICommandResult, error) {
	if c == nil || c.connection == nil {
		return sshclient.AICommandResult{}, ErrSessionNotConnected
	}
	return c.connection.RunAICommand(ctx, command)
}

func (m *SessionManager) RunAIBackgroundCommand(ctx context.Context, sessionID, command string) (sshclient.AICommandResult, error) {
	worker, err := m.lookup(ctx, sessionID)
	if err != nil {
		return sshclient.AICommandResult{}, err
	}
	connection, err := worker.connectedConnection(ctx)
	if err != nil {
		return sshclient.AICommandResult{}, err
	}
	runner, ok := connection.(aiCommandRunner)
	if !ok {
		return sshclient.AICommandResult{}, ErrAIBackgroundUnsupported
	}
	return runner.RunAICommand(ctx, command)
}

func (s *Service) RunAIBackgroundCommand(ctx context.Context, sessionID, command string) (sshclient.AICommandResult, error) {
	manager, _, err := s.ready()
	if err != nil {
		return sshclient.AICommandResult{}, err
	}
	runCtx, cancel := context.WithTimeout(ctx, aiBackgroundTimeout)
	defer cancel()
	return manager.RunAIBackgroundCommand(runCtx, sessionID, command)
}
