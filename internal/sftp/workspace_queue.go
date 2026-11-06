package sftp

import (
	"context"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"changeme/internal/errno"
	"changeme/internal/model"
	"changeme/internal/session"
)

func (s *Service) QueueSFTPWorkspaceUpload(
	ctx context.Context,
	sessionID string,
	directoryToken string,
	name string,
	remoteDirectory string,
) (model.SFTPTransfer, error) {
	manager, transfers, browser, err := s.localTransferDependencies()
	if err != nil {
		return model.SFTPTransfer{}, err
	}
	if err := localContextReady(ctx); err != nil {
		return model.SFTPTransfer{}, err
	}
	sessionID, err = validateWorkspaceSession(ctx, manager, sessionID)
	if err != nil {
		return model.SFTPTransfer{}, err
	}
	remoteDirectory, err = model.NormalizeRemoteDirectory(remoteDirectory)
	if err != nil {
		return model.SFTPTransfer{}, errno.New(errno.CodeInvalidArg, "sftp.directory_invalid", "")
	}
	localPath, localName, size, err := browser.resolveFile(ctx, directoryToken, name)
	if err != nil {
		return model.SFTPTransfer{}, err
	}
	remotePath, err := model.NormalizeRemotePath(path.Join(remoteDirectory, localName))
	if err != nil {
		return model.SFTPTransfer{}, errno.New(errno.CodeInvalidArg, "sftp.remote_path_invalid", "")
	}
	transfer, err := transfers.Enqueue(ctx, SFTPTransferRequest{
		SessionID:  sessionID,
		Direction:  model.SFTPTransferUpload,
		LocalPath:  localPath,
		LocalName:  localName,
		RemotePath: remotePath,
		Total:      size,
	})
	if err != nil {
		return model.SFTPTransfer{}, TransferServiceError(err)
	}
	return transfer, nil
}

func (s *Service) QueueSFTPWorkspaceUploadDirectory(
	ctx context.Context,
	sessionID string,
	directoryToken string,
	name string,
	remoteDirectory string,
) (DirectoryUploadResult, error) {
	manager, _, browser, err := s.localTransferDependencies()
	if err != nil {
		return DirectoryUploadResult{}, err
	}
	if err := localContextReady(ctx); err != nil {
		return DirectoryUploadResult{}, err
	}
	sessionID, err = validateWorkspaceSession(ctx, manager, sessionID)
	if err != nil {
		return DirectoryUploadResult{}, err
	}
	remoteDirectory, err = model.NormalizeRemoteDirectory(remoteDirectory)
	if err != nil {
		return DirectoryUploadResult{}, errno.New(errno.CodeInvalidArg, "sftp.directory_invalid", "")
	}
	if !isSafeLocalBasename(name) {
		return DirectoryUploadResult{}, errno.New(errno.CodeInvalidArg, "sftp.local_entry_invalid", "")
	}
	parent, err := browser.resolveDirectory(ctx, directoryToken)
	if err != nil {
		return DirectoryUploadResult{}, err
	}
	localPath := filepath.Join(parent, name)
	info, err := os.Lstat(localPath)
	if err != nil || !info.IsDir() {
		return DirectoryUploadResult{}, errno.New(errno.CodeInvalidArg, "sftp.upload_directory_invalid", "")
	}
	return s.enqueueDirectoryUpload(ctx, sessionID, localPath, remoteDirectory)
}

func (s *Service) QueueSFTPWorkspaceDownload(
	ctx context.Context,
	sessionID string,
	remotePath string,
	directoryToken string,
) (model.SFTPTransfer, error) {
	manager, transfers, browser, err := s.localTransferDependencies()
	if err != nil {
		return model.SFTPTransfer{}, err
	}
	if err := localContextReady(ctx); err != nil {
		return model.SFTPTransfer{}, err
	}
	sessionID, err = validateWorkspaceSession(ctx, manager, sessionID)
	if err != nil {
		return model.SFTPTransfer{}, err
	}
	remotePath, err = model.NormalizeRemotePath(remotePath)
	if err != nil || remotePath == "/" {
		return model.SFTPTransfer{}, errno.New(errno.CodeInvalidArg, "sftp.remote_path_invalid", "")
	}
	fileSystem, err := manager.FileSystem(ctx, sessionID)
	if err != nil {
		return model.SFTPTransfer{}, ServiceError(err)
	}
	info, err := fileSystem.StatContext(ctx, remotePath)
	if err != nil {
		return model.SFTPTransfer{}, RemotePathError(err, "sftp.stat_failed")
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return model.SFTPTransfer{}, errno.New(errno.CodeInvalidArg, "sftp.download_regular_only", "")
	}
	localDirectory, err := browser.resolveDirectory(ctx, directoryToken)
	if err != nil {
		return model.SFTPTransfer{}, err
	}
	localName := path.Base(remotePath)
	if !isSafeLocalBasename(localName) {
		return model.SFTPTransfer{}, errno.New(errno.CodeInvalidArg, "sftp.local_entry_invalid", "")
	}
	localPath, err := ValidateDownloadDestination(filepath.Join(localDirectory, localName))
	if err != nil {
		return model.SFTPTransfer{}, err
	}
	transfer, err := transfers.Enqueue(ctx, SFTPTransferRequest{
		SessionID:  sessionID,
		Direction:  model.SFTPTransferDownload,
		LocalPath:  localPath,
		LocalName:  localName,
		RemotePath: remotePath,
		Total:      info.Size(),
	})
	if err != nil {
		return model.SFTPTransfer{}, TransferServiceError(err)
	}
	return transfer, nil
}

func (s *Service) QueueSFTPRemoteTransfer(
	ctx context.Context,
	sourceSessionID, destSessionID, remotePath, destDirectory string,
) (model.SFTPTransfer, error) {
	manager, transfers, _, err := s.transferDependencies()
	if err != nil {
		return model.SFTPTransfer{}, err
	}
	if err := localContextReady(ctx); err != nil {
		return model.SFTPTransfer{}, err
	}
	sourceSessionID = strings.TrimSpace(sourceSessionID)
	destSessionID = strings.TrimSpace(destSessionID)
	if sourceSessionID == "" || destSessionID == "" {
		return model.SFTPTransfer{}, errno.New(errno.CodeInvalidArg, "sftp.session_invalid", "")
	}
	if sourceSessionID == destSessionID {
		return model.SFTPTransfer{}, errno.New(errno.CodeInvalidArg, "sftp.same_session", "源和目标不能是同一会话")
	}

	sourceSessionID, err = validateWorkspaceSession(ctx, manager, sourceSessionID)
	if err != nil {
		return model.SFTPTransfer{}, err
	}
	destSessionID, err = validateWorkspaceSession(ctx, manager, destSessionID)
	if err != nil {
		return model.SFTPTransfer{}, err
	}
	remotePath, err = model.NormalizeRemotePath(remotePath)
	if err != nil || remotePath == "/" {
		return model.SFTPTransfer{}, errno.New(errno.CodeInvalidArg, "sftp.remote_path_invalid", "")
	}
	destDirectory, err = model.NormalizeRemoteDirectory(destDirectory)
	if err != nil {
		return model.SFTPTransfer{}, errno.New(errno.CodeInvalidArg, "sftp.directory_invalid", "")
	}

	statFS, statErr := manager.TransferFileSystem(ctx, sourceSessionID)
	if statErr != nil {
		return model.SFTPTransfer{}, ServiceError(statErr)
	}
	info, stErr := statFS.StatContext(ctx, remotePath)
	_ = statFS.Close()
	if stErr != nil {
		return model.SFTPTransfer{}, RemotePathError(stErr, "sftp.stat_failed")
	}
	if !info.Mode().IsRegular() {
		return model.SFTPTransfer{}, errno.New(errno.CodeInvalidArg, "sftp.transfer_regular_only", "")
	}

	totalBytes := info.Size() * 2

	localName := path.Base(remotePath)
	remoteDestPath, normErr := model.NormalizeRemotePath(path.Join(destDirectory, localName))
	if normErr != nil || remoteDestPath != path.Join(destDirectory, localName) {
		return model.SFTPTransfer{}, errno.New(errno.CodeInvalidArg, "sftp.remote_path_invalid", "")
	}

	transfer, enqErr := transfers.Enqueue(ctx, SFTPTransferRequest{
		SessionID:        destSessionID,
		Direction:        model.SFTPTransferUpload,
		LocalName:        localName,
		RemotePath:       remoteDestPath,
		SourceRemotePath: remotePath,
		Total:            totalBytes,
		SourceSessionID:  sourceSessionID,
	})
	if enqErr != nil {
		return model.SFTPTransfer{}, TransferServiceError(enqErr)
	}
	return transfer, nil
}

func (s *Service) localDirectoryDependencies() (*localDirectoryBrowser, error) {
	if s == nil {
		return nil, ErrServiceNotReady
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.transfers == nil || s.local == nil {
		return nil, ErrServiceNotReady
	}
	return s.local, nil
}

func (s *Service) localTransferDependencies() (
	*session.SessionManager,
	*SFTPTransferManager,
	*localDirectoryBrowser,
	error,
) {
	if s == nil {
		return nil, nil, nil, ErrServiceNotReady
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.manager == nil || s.transfers == nil || s.local == nil {
		return nil, nil, nil, ErrServiceNotReady
	}
	return s.manager, s.transfers, s.local, nil
}
