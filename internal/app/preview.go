package app

import (
	"context"

	"changeme/internal/errno"
	"changeme/internal/model"
)

func (s *Service) ReadRemoteImage(sessionID string, remotePath string) (model.RemoteImageContent, error) {
	if s == nil || s.previews == nil {
		return model.RemoteImageContent{}, publicError(errno.New(errno.CodeStoreFailed, "error.serviceNotReady", ""))
	}
	return publicValue(s.previews.ReadImage(context.Background(), sessionID, remotePath))
}
