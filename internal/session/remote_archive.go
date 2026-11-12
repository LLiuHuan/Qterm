package session

import (
	"context"
	"errors"

	"changeme/internal/model"
	sshclient "changeme/internal/tunnel"
)

var ErrRemoteTarUnavailable = sshclient.ErrRemoteTarUnavailable

var ErrRemoteArchiveUnsupported = errors.New("session connection does not support remote archive extraction")

type remoteArchiveExtractor interface {
	ExtractRemoteArchive(ctx context.Context, archivePath string, targetDirectory string) error
	ProbeRemoteTar(ctx context.Context) error
}

type remoteArchiveCreator interface {
	CreateRemoteArchive(ctx context.Context, archivePath string, sourceDirectory string) error
}

var _ remoteArchiveExtractor = (*productionSessionConnection)(nil)
var _ remoteArchiveCreator = (*productionSessionConnection)(nil)

func (c *productionSessionConnection) ExtractRemoteArchive(
	ctx context.Context,
	archivePath string,
	targetDirectory string,
) error {
	if c == nil || c.connection == nil {
		return ErrSessionNotConnected
	}
	return c.connection.ExtractRemoteArchive(ctx, archivePath, targetDirectory)
}

func (c *productionSessionConnection) CreateRemoteArchive(
	ctx context.Context,
	archivePath string,
	sourceDirectory string,
) error {
	if c == nil || c.connection == nil {
		return ErrSessionNotConnected
	}
	return c.connection.CreateRemoteArchive(ctx, archivePath, sourceDirectory)
}

func (c *productionSessionConnection) ProbeRemoteTar(ctx context.Context) error {
	if c == nil || c.connection == nil {
		return ErrSessionNotConnected
	}
	return c.connection.ProbeRemoteTar(ctx)
}

func (m *SessionManager) ExtractRemoteArchive(
	ctx context.Context,
	sessionID string,
	archivePath string,
	targetDirectory string,
) error {
	worker, err := m.lookup(ctx, sessionID)
	if err != nil {
		return err
	}
	return worker.extractRemoteArchive(ctx, archivePath, targetDirectory)
}

func (m *SessionManager) CreateRemoteArchive(
	ctx context.Context,
	sessionID string,
	archivePath string,
	sourceDirectory string,
) error {
	worker, err := m.lookup(ctx, sessionID)
	if err != nil {
		return err
	}
	return worker.createRemoteArchive(ctx, archivePath, sourceDirectory)
}

func (m *SessionManager) ProbeRemoteTar(ctx context.Context, sessionID string) error {
	worker, err := m.lookup(ctx, sessionID)
	if err != nil {
		return err
	}
	return worker.probeRemoteTar(ctx)
}

func (s *managedSession) extractRemoteArchive(
	ctx context.Context,
	archivePath string,
	targetDirectory string,
) error {
	extractor, err := s.remoteArchiveTarget(ctx)
	if err != nil {
		return err
	}
	return extractor.ExtractRemoteArchive(ctx, archivePath, targetDirectory)
}

func (s *managedSession) createRemoteArchive(
	ctx context.Context,
	archivePath string,
	sourceDirectory string,
) error {
	connection, err := s.connectedConnection(ctx)
	if err != nil {
		return err
	}
	creator, ok := connection.(remoteArchiveCreator)
	if !ok {
		return ErrRemoteArchiveUnsupported
	}
	return creator.CreateRemoteArchive(ctx, archivePath, sourceDirectory)
}

func (s *managedSession) probeRemoteTar(ctx context.Context) error {
	extractor, err := s.remoteArchiveTarget(ctx)
	if err != nil {
		return err
	}
	return extractor.ProbeRemoteTar(ctx)
}

func (s *managedSession) remoteArchiveTarget(ctx context.Context) (remoteArchiveExtractor, error) {
	connection, err := s.connectedConnection(ctx)
	if err != nil {
		return nil, err
	}
	extractor, ok := connection.(remoteArchiveExtractor)
	if !ok {
		return nil, ErrRemoteArchiveUnsupported
	}
	return extractor, nil
}

func (s *managedSession) connectedConnection(ctx context.Context) (SessionConnection, error) {
	if ctx == nil {
		return nil, errors.New("remote command context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	select {
	case <-s.ready:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	s.mu.RLock()
	connection := s.connection
	state := s.snapshot.State
	s.mu.RUnlock()
	if state != model.SessionStateConnected || connection == nil {
		return nil, ErrSessionNotConnected
	}
	return connection, nil
}
