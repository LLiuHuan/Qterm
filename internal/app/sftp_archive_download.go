package app

import (
	"context"

	"changeme/internal/errno"
	"changeme/internal/model"
)

func (s *Service) DownloadSFTPDirectoryArchive(
	sessionID string,
	remoteDirectory string,
	localDirectory string,
) error {
	if s == nil || s.sftp == nil {
		return publicError(errno.New(errno.CodeStoreFailed, "error.serviceNotReady", ""))
	}
	return publicError(
		s.sftp.DownloadRemoteDirectoryArchive(context.Background(), sessionID, remoteDirectory, localDirectory),
	)
}

func (s *Service) EnqueueSFTPDirectoryArchiveDownload(
	ctx context.Context,
	sessionID string,
	remoteDirectory string,
	localDirectory string,
) (model.SFTPTransfer, error) {
	if s == nil || s.sftp == nil {
		return model.SFTPTransfer{}, publicError(errno.New(errno.CodeStoreFailed, "error.serviceNotReady", ""))
	}
	return publicValue(s.sftp.EnqueueDirectoryArchiveDownload(ctx, sessionID, remoteDirectory, localDirectory))
}
