package session

import (
	"context"
	"errors"
	"time"

	"changeme/internal/model"
	sshclient "changeme/internal/tunnel"
)

const aiBackgroundTimeout = 2 * time.Minute

var ErrAIBackgroundUnsupported = errors.New("session does not support AI background commands")

type aiCommandRunner interface {
	RunAICommand(context.Context, string) (sshclient.AICommandResult, error)
}

type aiStdinCommandRunner interface {
	RunAICommandWithStdin(context.Context, string, []byte) (sshclient.AICommandResult, error)
}

var _ aiCommandRunner = (*productionSessionConnection)(nil)
var _ aiStdinCommandRunner = (*productionSessionConnection)(nil)

func (c *productionSessionConnection) RunAICommand(ctx context.Context, command string) (sshclient.AICommandResult, error) {
	if c == nil || c.connection == nil {
		return sshclient.AICommandResult{}, ErrSessionNotConnected
	}
	return c.connection.RunAICommand(ctx, command)
}

func (c *productionSessionConnection) RunAICommandWithStdin(
	ctx context.Context,
	command string,
	stdin []byte,
) (sshclient.AICommandResult, error) {
	if c == nil || c.connection == nil {
		return sshclient.AICommandResult{}, ErrSessionNotConnected
	}
	return c.connection.RunAICommandWithStdin(ctx, command, stdin)
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

func (m *SessionManager) RunAIBackgroundCommandWithStdin(
	ctx context.Context,
	sessionID, command string,
	stdin []byte,
) (sshclient.AICommandResult, error) {
	worker, err := m.lookup(ctx, sessionID)
	if err != nil {
		return sshclient.AICommandResult{}, err
	}
	connection, err := worker.connectedConnection(ctx)
	if err != nil {
		return sshclient.AICommandResult{}, err
	}
	runner, ok := connection.(aiStdinCommandRunner)
	if !ok {
		return sshclient.AICommandResult{}, ErrAIBackgroundUnsupported
	}
	return runner.RunAICommandWithStdin(ctx, command, stdin)
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

func (s *Service) RunAIBackgroundCommandWithStdin(
	ctx context.Context,
	sessionID, command string,
	stdin []byte,
) (sshclient.AICommandResult, error) {
	manager, _, err := s.ready()
	if err != nil {
		return sshclient.AICommandResult{}, err
	}
	runCtx, cancel := context.WithTimeout(ctx, aiBackgroundTimeout)
	defer cancel()
	return manager.RunAIBackgroundCommandWithStdin(runCtx, sessionID, command, stdin)
}

func (s *Service) SessionKind(ctx context.Context, sessionID string) (model.SessionKind, error) {
	manager, _, err := s.ready()
	if err != nil {
		return "", err
	}
	session, err := manager.Get(ctx, sessionID)
	if err != nil {
		return "", err
	}
	return session.Kind, nil
}
