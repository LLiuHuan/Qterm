package sftp

import (
	"context"
	"errors"
	"path"
	"strings"

	"changeme/internal/errno"
	"changeme/internal/model"
	sessionapp "changeme/internal/session"
)

const remoteDirectoryEntryLimit = 10000

type RemoteDirectoryTransferResult struct {
	Queued  int `json:"queued"`
	Skipped int `json:"skipped"`
}

type remoteDirectoryTransferEntry struct {
	sourcePath string
	destPath   string
	size       int64
	directory  bool
}

func (s *Service) QueueSFTPRemoteTransferDirectory(
	ctx context.Context,
	sourceSessionID, destSessionID, remotePath, destDirectory string,
) (RemoteDirectoryTransferResult, error) {
	result := RemoteDirectoryTransferResult{}
	manager, transfers, _, err := s.transferDependencies()
	if err != nil {
		return result, err
	}
	if err := localContextReady(ctx); err != nil {
		return result, err
	}
	sourceSessionID = strings.TrimSpace(sourceSessionID)
	destSessionID = strings.TrimSpace(destSessionID)
	if sourceSessionID == "" || destSessionID == "" {
		return result, errno.New(errno.CodeInvalidArg, "sftp.session_invalid", "")
	}
	if sourceSessionID == destSessionID {
		return result, errno.New(errno.CodeInvalidArg, "sftp.same_session", "源和目标不能是同一会话")
	}
	sourceSessionID, err = validateWorkspaceSession(ctx, manager, sourceSessionID)
	if err != nil {
		return result, err
	}
	destSessionID, err = validateWorkspaceSession(ctx, manager, destSessionID)
	if err != nil {
		return result, err
	}
	remotePath, err = model.NormalizeRemotePath(remotePath)
	if err != nil || remotePath == "/" {
		return result, errno.New(errno.CodeInvalidArg, "sftp.remote_path_invalid", "")
	}
	destDirectory, err = model.NormalizeRemoteDirectory(destDirectory)
	if err != nil {
		return result, errno.New(errno.CodeInvalidArg, "sftp.directory_invalid", "")
	}

	base := path.Base(remotePath)
	if !isSafeLocalBasename(base) {
		return result, errno.New(errno.CodeInvalidArg, "sftp.remote_path_invalid", "")
	}
	destRoot := path.Join(destDirectory, base)
	if normalized, normalizeErr := model.NormalizeRemotePath(destRoot); normalizeErr != nil || normalized != destRoot {
		return result, errno.New(errno.CodeInvalidArg, "sftp.remote_path_invalid", "")
	}

	sourceFS, err := manager.TransferFileSystem(ctx, sourceSessionID)
	if err != nil {
		return result, ServiceError(err)
	}
	destFS, err := manager.TransferFileSystem(ctx, destSessionID)
	if err != nil {
		return result, errors.Join(ServiceError(err), sourceFS.Close())
	}
	closeFileSystems := func() error { return errors.Join(sourceFS.Close(), destFS.Close()) }

	info, statErr := sourceFS.StatContext(ctx, remotePath)
	if statErr != nil {
		return result, errors.Join(errno.New(errno.CodeConnFailed, "sftp.stat_failed", ""), closeFileSystems())
	}
	if !info.IsDir() {
		return result, errors.Join(errno.New(errno.CodeInvalidArg, "sftp.transfer_directory_only", ""), closeFileSystems())
	}
	creator, ok := destFS.(directoryCreator)
	if !ok {
		return result, errors.Join(errno.New(errno.CodeInvalidArg, "sftp.operation_unsupported", ""), closeFileSystems())
	}

	plan, skipped, scanErr := inspectRemoteDirectory(ctx, sourceFS, remotePath, destRoot)
	if scanErr != nil {
		if ctx.Err() != nil {
			return result, errors.Join(ctx.Err(), closeFileSystems())
		}
		return result, errors.Join(errno.New(errno.CodeInvalidArg, "sftp.remote_directory_invalid", ""), closeFileSystems())
	}
	result.Skipped = skipped

	batchID, err := randomTransferID()
	if err != nil {
		return result, errors.Join(errno.Wrap(errno.CodeUnknown, "sftp.remote_directory_failed", err), closeFileSystems())
	}
	enqueueErr := enqueueRemoteDirectory(ctx, plan, creator, func(entry remoteDirectoryTransferEntry) error {

		_, enqueueErr := transfers.Enqueue(ctx, SFTPTransferRequest{
			SessionID:        destSessionID,
			BatchID:          batchID,
			Direction:        model.SFTPTransferUpload,
			LocalName:        path.Base(entry.destPath),
			RemotePath:       entry.destPath,
			SourceRemotePath: entry.sourcePath,
			Total:            entry.size * 2,
			SourceSessionID:  sourceSessionID,
		})
		if enqueueErr == nil {
			result.Queued++
		}
		return enqueueErr
	})

	_ = closeFileSystems()
	if enqueueErr != nil {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		return result, errno.Wrap(errno.CodeUnknown, "sftp.remote_directory_failed", enqueueErr)
	}
	return result, nil
}

func inspectRemoteDirectory(
	ctx context.Context,
	fileSystem sessionapp.SessionFileSystem,
	sourceRoot string,
	destRoot string,
) ([]remoteDirectoryTransferEntry, int, error) {
	root := remoteDirectoryTransferEntry{sourcePath: sourceRoot, destPath: destRoot, directory: true}
	plan := []remoteDirectoryTransferEntry{root}
	pending := []remoteDirectoryTransferEntry{root}
	skipped := 0
	for len(pending) > 0 {
		current := pending[0]
		pending = pending[1:]
		entries, err := fileSystem.ReadDirContext(ctx, current.sourcePath)
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
			source := path.Join(current.sourcePath, name)
			dest := path.Join(current.destPath, name)
			if normalized, normalizeErr := model.NormalizeRemotePath(dest); normalizeErr != nil || normalized != dest {
				return nil, skipped, errors.New("invalid destination path")
			}
			switch {
			case entry.IsDir():
				child := remoteDirectoryTransferEntry{sourcePath: source, destPath: dest, directory: true}
				plan = append(plan, child)
				pending = append(pending, child)
			case entry.Mode().IsRegular():
				plan = append(plan, remoteDirectoryTransferEntry{sourcePath: source, destPath: dest, size: entry.Size()})
			default:
				skipped++
			}
		}
	}
	return plan, skipped, nil
}

func enqueueRemoteDirectory(
	ctx context.Context,
	plan []remoteDirectoryTransferEntry,
	creator directoryCreator,
	enqueue func(remoteDirectoryTransferEntry) error,
) error {
	for _, entry := range plan {
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.directory {
			if err := creator.EnsureDirectoryContext(ctx, entry.destPath); err != nil {
				return err
			}
			continue
		}
		if err := enqueue(entry); err != nil {
			return err
		}
	}
	return nil
}
