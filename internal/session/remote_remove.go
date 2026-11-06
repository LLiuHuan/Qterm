package session

import (
	"context"
	"fmt"

	sshclient "changeme/internal/tunnel"
)

var ErrRemoteRemoveUnavailable = sshclient.ErrRemoteRemoveUnavailable

var ErrRemoteRemoveUnsupported = fmt.Errorf(
	"%w: session connection does not support remote tree removal",
	sshclient.ErrRemoteRemoveUnavailable,
)

type remoteTreeRemover interface {
	RemoveRemoteTree(ctx context.Context, remotePath string) error
}

var _ remoteTreeRemover = (*productionSessionConnection)(nil)

func (c *productionSessionConnection) RemoveRemoteTree(ctx context.Context, remotePath string) error {
	if c == nil || c.connection == nil {
		return ErrSessionNotConnected
	}
	return c.connection.RemoveRemoteTree(ctx, remotePath)
}

func (m *SessionManager) RemoveRemoteTree(ctx context.Context, sessionID, remotePath string) error {
	worker, err := m.lookup(ctx, sessionID)
	if err != nil {
		return err
	}
	return worker.removeRemoteTree(ctx, remotePath)
}

func (s *managedSession) removeRemoteTree(ctx context.Context, remotePath string) error {
	remover, err := s.remoteRemoveTarget(ctx)
	if err != nil {
		return err
	}
	return remover.RemoveRemoteTree(ctx, remotePath)
}

func (s *managedSession) remoteRemoveTarget(ctx context.Context) (remoteTreeRemover, error) {
	connection, err := s.connectedConnection(ctx)
	if err != nil {
		return nil, err
	}
	remover, ok := connection.(remoteTreeRemover)
	if !ok {
		return nil, ErrRemoteRemoveUnsupported
	}
	return remover, nil
}
