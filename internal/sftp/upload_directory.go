package sftp

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"changeme/internal/errno"
	"changeme/internal/model"
)

type directoryPicker interface{ ChooseUploadDirectory() (string, error) }
type directoryCreator interface {
	EnsureDirectoryContext(context.Context, string) error
}

type DirectoryUploadResult struct {
	Canceled bool `json:"canceled"`
	Queued   int  `json:"queued"`
}

type uploadDirectoryEntry struct {
	localPath  string
	remotePath string
	size       int64
	directory  bool
}

func (s *Service) SelectSFTPUploadDirectory(ctx context.Context, sessionID, remoteDirectory string) (DirectoryUploadResult, error) {
	result := DirectoryUploadResult{}
	manager, _, dialogs, err := s.transferDependencies()
	if err != nil {
		return result, err
	}
	picker, ok := dialogs.(directoryPicker)
	if !ok {
		return result, ErrServiceNotReady
	}
	remoteDirectory, err = model.NormalizeRemoteDirectory(remoteDirectory)
	if err != nil {
		return result, errno.New(errno.CodeInvalidArg, "sftp.directory_invalid", "")
	}
	sessionID = strings.TrimSpace(sessionID)
	sessionInfo, err := manager.Get(ctx, sessionID)
	if err != nil || sessionInfo.State != model.SessionStateConnected {
		return result, errno.New(errno.CodeSessionClosed, "session.disconnected", "")
	}
	localPath, err := picker.ChooseUploadDirectory()
	if err != nil {
		return result, errno.Wrap(errno.CodeUnknown, "sftp.open_dialog_failed", err)
	}
	if localPath == "" {
		result.Canceled = true
		return result, nil
	}
	return s.enqueueDirectoryUpload(ctx, sessionID, localPath, remoteDirectory)
}

func (s *Service) enqueueDirectoryUpload(
	ctx context.Context,
	sessionID string,
	localPath string,
	remoteDirectory string,
) (DirectoryUploadResult, error) {
	result := DirectoryUploadResult{}
	manager, transfers, _, err := s.transferDependencies()
	if err != nil {
		return result, err
	}

	plan, err := inspectUploadDirectory(ctx, localPath, remoteDirectory)
	if err != nil {
		return result, errno.Wrap(errno.CodeInvalidArg, "sftp.upload_directory_invalid", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	fileSystem, err := manager.TransferFileSystem(ctx, sessionID)
	if err != nil {
		return result, ServiceError(err)
	}
	creator, ok := fileSystem.(directoryCreator)
	if !ok {
		return result, errors.Join(errno.New(errno.CodeInvalidArg, "sftp.operation_unsupported", ""), fileSystem.Close())
	}
	batchID, err := randomTransferID()
	if err != nil {
		return result, errors.Join(err, fileSystem.Close())
	}
	operationErr := enqueueUploadDirectory(ctx, plan, creator, func(entry uploadDirectoryEntry) error {
		_, enqueueErr := transfers.Enqueue(ctx, SFTPTransferRequest{SessionID: sessionID, BatchID: batchID, Direction: model.SFTPTransferUpload, LocalPath: entry.localPath, LocalName: filepath.Base(entry.localPath), RemotePath: entry.remotePath, Total: entry.size})
		if enqueueErr == nil {
			result.Queued++
		}
		return enqueueErr
	})

	_ = fileSystem.Close()
	return result, errno.Wrap(errno.CodeUnknown, "sftp.upload_directory_failed", operationErr)
}

func inspectUploadDirectory(ctx context.Context, localPath, remoteDirectory string) ([]uploadDirectoryEntry, error) {
	if !filepath.IsAbs(localPath) {
		return nil, errors.New("directory must be absolute")
	}
	localPath = filepath.Clean(localPath)
	rootInfo, err := os.Lstat(localPath)
	if err != nil {
		return nil, err
	}
	if !rootInfo.IsDir() || filepath.Dir(localPath) == localPath {
		return nil, errors.New("invalid upload directory")
	}
	plan := []uploadDirectoryEntry{}
	err = filepath.WalkDir(localPath, func(source string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(plan) >= 10000 {
			return errors.New("directory entry limit exceeded")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return errors.New("unsupported directory entry")
		}
		relative, err := filepath.Rel(filepath.Dir(localPath), source)
		if err != nil {
			return err
		}
		destination := path.Join(remoteDirectory, filepath.ToSlash(relative))
		normalized, err := model.NormalizeRemotePath(destination)
		if err != nil || normalized != destination {
			return errors.New("invalid destination path")
		}
		plan = append(plan, uploadDirectoryEntry{localPath: source, remotePath: destination, size: info.Size(), directory: info.IsDir()})
		return nil
	})
	return plan, err
}

func enqueueUploadDirectory(ctx context.Context, plan []uploadDirectoryEntry, creator directoryCreator, enqueue func(uploadDirectoryEntry) error) error {

	for _, entry := range plan {
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.directory {
			if err := creator.EnsureDirectoryContext(ctx, entry.remotePath); err != nil {
				return err
			}
		} else if err := enqueue(entry); err != nil {
			return err
		}
	}
	return nil
}
