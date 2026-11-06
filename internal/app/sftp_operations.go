package app

import (
	"context"

	"changeme/internal/model"
	"changeme/internal/sftp"
)

func (s *Service) CreateRemoteDirectory(ctx context.Context, sessionID, directory, name string) error {
	return publicError(s.sftp.CreateRemoteDirectory(ctx, sessionID, directory, name))
}

func (s *Service) DeleteRemoteEntry(ctx context.Context, sessionID, remotePath string) error {
	return publicError(s.sftp.DeleteRemoteEntry(ctx, sessionID, remotePath))
}

func (s *Service) CreateRemoteFile(ctx context.Context, sessionID, directory, name string) error {
	return publicError(s.sftp.CreateRemoteFile(ctx, sessionID, directory, name))
}
func (s *Service) RenameRemoteEntry(ctx context.Context, sessionID, remotePath, newName string) error {
	return publicError(s.sftp.RenameRemoteEntry(ctx, sessionID, remotePath, newName))
}
func (s *Service) ChmodRemoteEntry(ctx context.Context, sessionID, remotePath, mode string) error {
	return publicError(s.sftp.ChmodRemoteEntry(ctx, sessionID, remotePath, mode))
}
func (s *Service) ListRemotePrincipals(ctx context.Context, sessionID string) (model.RemotePrincipals, error) {
	return publicValue(s.sftp.ListRemotePrincipals(ctx, sessionID))
}
func (s *Service) SetRemoteEntryPermissions(
	ctx context.Context,
	sessionID, remotePath, mode string,
	uid, gid uint32,
) error {
	return publicError(s.sftp.SetRemoteEntryPermissions(ctx, sessionID, remotePath, mode, uid, gid))
}
func (s *Service) CreateLocalFile(ctx context.Context, token, name string) error {
	return publicError(s.sftp.CreateLocalFile(ctx, token, name))
}
func (s *Service) CreateLocalDirectory(ctx context.Context, token, name string) error {
	return publicError(s.sftp.CreateLocalDirectory(ctx, token, name))
}
func (s *Service) RenameLocalEntry(ctx context.Context, token, name, newName string) error {
	return publicError(s.sftp.RenameLocalEntry(ctx, token, name, newName))
}
func (s *Service) ChmodLocalEntry(ctx context.Context, token, name, mode string) error {
	return publicError(s.sftp.ChmodLocalEntry(ctx, token, name, mode))
}
func (s *Service) DeleteLocalEntry(ctx context.Context, token, name string) error {
	return publicError(s.sftp.DeleteLocalEntry(ctx, token, name))
}

func (s *Service) DirectoryFollow(ctx context.Context, sessionID string, configure bool) (string, error) {
	result, err := s.sftp.DirectoryFollow(ctx, sessionID, configure)
	return result, publicError(err)
}

func (s *Service) SelectSFTPUploadDirectory(ctx context.Context, sessionID, remoteDirectory string) (sftp.DirectoryUploadResult, error) {
	return publicValue(s.sftp.SelectSFTPUploadDirectory(ctx, sessionID, remoteDirectory))
}

func (s *Service) SelectSFTPLocalDirectory(ctx context.Context) (sftp.SFTPLocalDirectorySelectionResult, error) {
	return publicValue(s.sftp.SelectSFTPLocalDirectory(ctx))
}

func (s *Service) OpenSFTPLocalDirectory(ctx context.Context, token, child string) (sftp.LocalDirectory, error) {
	return publicValue(s.sftp.OpenSFTPLocalDirectory(ctx, token, child))
}

func (s *Service) QueueSFTPWorkspaceDownloadDirectory(
	ctx context.Context,
	sessionID, remotePath, directoryToken string,
) (sftp.DirectoryDownloadResult, error) {
	return publicValue(s.sftp.QueueSFTPWorkspaceDownloadDirectory(ctx, sessionID, remotePath, directoryToken))
}

func (s *Service) QueueSFTPWorkspaceUpload(
	ctx context.Context,
	sessionID, directoryToken, name, remoteDirectory string,
) (model.SFTPTransfer, error) {
	return publicValue(s.sftp.QueueSFTPWorkspaceUpload(ctx, sessionID, directoryToken, name, remoteDirectory))
}

func (s *Service) QueueSFTPWorkspaceUploadDirectory(
	ctx context.Context,
	sessionID, directoryToken, name, remoteDirectory string,
) (sftp.DirectoryUploadResult, error) {
	return publicValue(s.sftp.QueueSFTPWorkspaceUploadDirectory(ctx, sessionID, directoryToken, name, remoteDirectory))
}

func (s *Service) QueueSFTPWorkspaceDownload(
	ctx context.Context,
	sessionID, remotePath, directoryToken string,
) (model.SFTPTransfer, error) {
	return publicValue(s.sftp.QueueSFTPWorkspaceDownload(ctx, sessionID, remotePath, directoryToken))
}

func (s *Service) QueueSFTPRemoteTransfer(
	ctx context.Context,
	sourceSessionID, destSessionID, remotePath, destDirectory string,
) (model.SFTPTransfer, error) {
	return publicValue(s.sftp.QueueSFTPRemoteTransfer(ctx, sourceSessionID, destSessionID, remotePath, destDirectory))
}

func (s *Service) QueueSFTPRemoteTransferDirectory(
	ctx context.Context,
	sourceSessionID, destSessionID, remotePath, destDirectory string,
) (sftp.RemoteDirectoryTransferResult, error) {
	return publicValue(s.sftp.QueueSFTPRemoteTransferDirectory(ctx, sourceSessionID, destSessionID, remotePath, destDirectory))
}
