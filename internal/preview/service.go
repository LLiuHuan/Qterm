package preview

import (
	"context"
	"strings"

	"changeme/internal/errno"
	"changeme/internal/model"
	sessionapp "changeme/internal/session"
)

type sessionSource interface {
	FileSystem(ctx context.Context, id string) (sessionapp.SessionFileSystem, error)
	Get(ctx context.Context, id string) (model.Session, error)
}

type Service struct {
	sessions sessionSource
}

func NewService(sessions sessionSource) *Service {
	return &Service{sessions: sessions}
}

func (s *Service) ready() error {
	if s == nil || s.sessions == nil {
		return errno.New(errno.CodeStoreFailed, messageKeyServiceNotReady, "")
	}
	return nil
}

func (s *Service) ReadImage(ctx context.Context, sessionID string, remotePath string) (model.RemoteImageContent, error) {
	if err := s.ready(); err != nil {
		return model.RemoteImageContent{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return model.RemoteImageContent{}, errno.New(errno.CodeInvalidArg, messageKeySessionNotFound, "")
	}
	normalized, err := normalizeRemoteFile(remotePath)
	if err != nil {
		return model.RemoteImageContent{}, err
	}

	mimeType, limit, supported := imageTypeFor(normalized)
	if !supported {
		return model.RemoteImageContent{}, errno.New(errno.CodeInvalidArg, MessageKeyUnsupported, "")
	}
	fileSystem, err := s.sessions.FileSystem(ctx, sessionID)
	if err != nil {
		return model.RemoteImageContent{}, sessionError(err)
	}
	defer fileSystem.Close()
	access, ok := fileSystem.(remoteFileAccess)
	if !ok {

		return model.RemoteImageContent{}, errno.New(errno.CodeInvalidArg, MessageKeyUnsupported, "")
	}
	info, statErr := access.StatContext(ctx, normalized)
	if statErr != nil {
		return model.RemoteImageContent{}, s.remoteError(ctx, sessionID, statErr, messageKeyStatFailed)
	}
	if !info.Mode().IsRegular() {
		return model.RemoteImageContent{}, errno.New(errno.CodeInvalidArg, messageKeyRegularOnly, "")
	}
	size := info.Size()
	modTime := info.ModTime().UTC()
	if size > limit {
		return model.RemoteImageContent{
			Size:      size,
			MimeType:  mimeType,
			Truncated: true,
			ModTime:   modTime,
		}, nil
	}
	reader, openErr := access.OpenFile(ctx, normalized)
	if openErr != nil {
		return model.RemoteImageContent{}, s.remoteError(ctx, sessionID, openErr, messageKeyReadFailed)
	}
	defer reader.Close()
	data, truncated, readErr := readUpTo(reader, limit)
	if readErr != nil {
		return model.RemoteImageContent{}, s.remoteError(ctx, sessionID, readErr, messageKeyReadFailed)
	}
	if truncated {

		return model.RemoteImageContent{
			Size:      size,
			MimeType:  mimeType,
			Truncated: true,
			ModTime:   modTime,
		}, nil
	}
	return model.RemoteImageContent{
		ContentBase64: encodeBase64(data),
		Size:          size,
		MimeType:      mimeType,
		Truncated:     false,
		ModTime:       modTime,
	}, nil
}
