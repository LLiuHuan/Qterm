package sftp

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"

	"changeme/internal/errno"
	"changeme/internal/model"
	sessionapp "changeme/internal/session"
)

type DirectoryDownloadResult struct {
	Queued  int `json:"queued"`
	Skipped int `json:"skipped"`
}

type remoteDownloadEntry struct {
	remotePath string
	localPath  string
	size       int64
	directory  bool
}

func (s *Service) QueueSFTPWorkspaceDownloadDirectory(
	ctx context.Context,
	sessionID, remotePath, directoryToken string,
) (DirectoryDownloadResult, error) {
	result := DirectoryDownloadResult{}
	manager, transfers, browser, err := s.localTransferDependencies()
	if err != nil {
		return result, err
	}
	if err := localContextReady(ctx); err != nil {
		return result, err
	}
	sessionID, err = validateWorkspaceSession(ctx, manager, sessionID)
	if err != nil {
		return result, err
	}
	remotePath, err = model.NormalizeRemotePath(remotePath)
	if err != nil || remotePath == "/" {
		return result, errno.New(errno.CodeInvalidArg, "sftp.remote_path_invalid", "")
	}
	localDirectory, err := browser.resolveDirectory(ctx, directoryToken)
	if err != nil {
		return result, err
	}

	base := path.Base(remotePath)
	if !isSafeLocalBasename(base) {
		return result, errno.New(errno.CodeInvalidArg, "sftp.remote_path_invalid", "")
	}
	localRoot := filepath.Join(localDirectory, base)

	fileSystem, err := manager.TransferFileSystem(ctx, sessionID)
	if err != nil {
		return result, ServiceError(err)
	}
	info, statErr := fileSystem.StatContext(ctx, remotePath)
	if statErr != nil {
		_ = fileSystem.Close()
		return result, errno.New(errno.CodeConnFailed, "sftp.stat_failed", "")
	}
	if !info.IsDir() {
		_ = fileSystem.Close()
		return result, errno.New(errno.CodeInvalidArg, "sftp.transfer_directory_only", "")
	}

	plan, skipped, scanErr := inspectRemoteDownload(ctx, fileSystem, remotePath, localRoot, localDirectory)
	_ = fileSystem.Close()
	if scanErr != nil {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		return result, errno.New(errno.CodeInvalidArg, "sftp.remote_directory_invalid", "")
	}
	result.Skipped = skipped

	for _, entry := range plan {
		if !entry.directory {
			continue
		}
		if err := validateDirectoryDestination(entry.localPath); err != nil {
			return result, err
		}
	}

	batchID, err := randomTransferID()
	if err != nil {
		return result, errno.Wrap(errno.CodeUnknown, "sftp.remote_directory_failed", err)
	}
	for _, entry := range plan {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if entry.directory {
			if err := os.MkdirAll(entry.localPath, 0o755); err != nil {
				return result, errno.Wrap(errno.CodeStoreFailed, "sftp.local_directory_failed", err)
			}
			continue
		}

		localPath, validateErr := ValidateDownloadDestination(entry.localPath)
		if validateErr != nil {
			return result, validateErr
		}
		_, enqueueErr := transfers.Enqueue(ctx, SFTPTransferRequest{
			SessionID:  sessionID,
			BatchID:    batchID,
			Direction:  model.SFTPTransferDownload,
			LocalPath:  localPath,
			LocalName:  filepath.Base(localPath),
			RemotePath: entry.remotePath,
			Total:      entry.size,
		})
		if enqueueErr != nil {
			return result, TransferServiceError(enqueueErr)
		}
		result.Queued++
	}
	return result, nil
}

func validateDirectoryDestination(localPath string) error {
	info, err := os.Lstat(localPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return errno.Wrap(errno.CodeStoreFailed, "sftp.local_directory_failed", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errno.New(errno.CodeStoreFailed, "sftp.local_directory_failed", "")
	}
	return nil
}

func inspectRemoteDownload(
	ctx context.Context,
	fileSystem sessionapp.SessionFileSystem,
	remoteRoot string,
	localRoot string,
	localParent string,
) ([]remoteDownloadEntry, int, error) {
	root := remoteDownloadEntry{remotePath: remoteRoot, localPath: localRoot, directory: true}
	plan := []remoteDownloadEntry{root}
	pending := []remoteDownloadEntry{root}
	skipped := 0
	for len(pending) > 0 {
		current := pending[0]
		pending = pending[1:]
		entries, err := fileSystem.ReadDirContext(ctx, current.remotePath)
		if err != nil {
			return nil, skipped, err
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return nil, skipped, err
			}
			if len(plan) >= remoteDirectoryEntryLimit {
				return nil, skipped, errors.New("remote directory entry limit exceeded")
			}
			name := entry.Name()
			if !isSafeLocalBasename(name) {
				return nil, skipped, errors.New("unsupported remote entry name")
			}
			remoteChild := path.Join(current.remotePath, name)
			localChild := filepath.Join(current.localPath, name)

			if !localPathWithin(localParent, localChild) {
				return nil, skipped, errors.New("local destination escaped the target directory")
			}
			switch {
			case entry.IsDir():
				child := remoteDownloadEntry{remotePath: remoteChild, localPath: localChild, directory: true}
				plan = append(plan, child)
				pending = append(pending, child)
			case entry.Mode().IsRegular():
				plan = append(plan, remoteDownloadEntry{remotePath: remoteChild, localPath: localChild, size: entry.Size()})
			default:
				skipped++
			}
		}
	}
	return plan, skipped, nil
}

func localPathWithin(root, child string) bool {
	relative, err := filepath.Rel(root, child)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
