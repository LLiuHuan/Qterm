package session

import (
	"context"
	"time"

	"changeme/internal/errno"
)

type directoryFollowConnection interface {
	DirectoryFollow(context.Context, bool) (string, error)
}

func (m *SessionManager) DirectoryFollow(ctx context.Context, id string, configure bool) (string, error) {
	if ctx == nil {
		return "", ErrSessionNotConnected
	}
	worker, err := m.lookup(ctx, id)
	if err != nil {
		return "", err
	}
	worker.mu.RLock()
	connection := worker.connection
	worker.mu.RUnlock()
	probe, ok := connection.(directoryFollowConnection)
	if !ok {
		return "", ErrSessionNotConnected
	}
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	result, err := probe.DirectoryFollow(probeCtx, configure)
	return result, errno.Wrap(errno.CodeConnFailed, "sftp.follow_failed", err)
}

func (c *productionSessionConnection) DirectoryFollow(ctx context.Context, configure bool) (string, error) {
	if c == nil || c.connection == nil {
		return "", ErrSessionNotConnected
	}

	if c.directoryFollowMode == "session" || c.directoryFollowMode == "disabled" {
		return "unsupported", nil
	}
	return c.connection.DirectoryFollow(ctx, configure)
}
