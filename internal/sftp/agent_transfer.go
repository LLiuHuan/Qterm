package sftp

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"changeme/internal/errno"
	"changeme/internal/model"
)

const (
	agentTransferMaxBytes int64 = 64 << 20

	agentTransferTimeout = 10 * time.Minute
)

func (s *Service) AgentUploadLocalFile(ctx context.Context, sessionID, localPath, remotePath string) (int64, error) {
	manager, _, _, err := s.transferDependencies()
	if err != nil {
		return 0, err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return 0, errno.New(errno.CodeInvalidArg, "session.not_found", "")
	}
	localPath = strings.TrimSpace(localPath)
	if !filepath.IsAbs(localPath) {
		return 0, errno.New(errno.CodeInvalidArg, "ai.agent.tool.path_not_absolute", "")
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return 0, errno.New(errno.CodeNotFound, "ai.agent.tool.local_file_not_found", "")
	}
	if !info.Mode().IsRegular() {
		return 0, errno.New(errno.CodeInvalidArg, "ai.agent.tool.not_a_regular_file", "")
	}
	if info.Size() > agentTransferMaxBytes {
		return 0, errno.New(errno.CodeInvalidArg, "ai.agent.tool.transfer_too_large", "")
	}
	normalized, err := model.NormalizeRemotePath(remotePath)
	if err != nil {
		return 0, errno.New(errno.CodeInvalidArg, "sftp.remote_path_invalid", "")
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, agentTransferTimeout)
	defer cancel()

	fileSystem, err := manager.TransferFileSystem(ctx, sessionID)
	if err != nil {
		return 0, ServiceError(err)
	}
	defer func() { _ = fileSystem.Close() }()

	if _, statErr := fileSystem.StatContext(ctx, normalized); statErr == nil {
		return 0, errno.New(errno.CodeInvalidArg, "ai.agent.tool.transfer_target_exists", "")
	} else if !errors.Is(statErr, fs.ErrNotExist) {

		return 0, errno.New(errno.CodeConnFailed, "ai.agent.tool.transfer_stat_failed", "")
	}

	if err := fileSystem.UploadFileContext(ctx, localPath, normalized, info.Size(), false, nil); err != nil {
		return 0, ServiceError(err)
	}
	return info.Size(), nil
}

func (s *Service) AgentDownloadRemoteFile(ctx context.Context, sessionID, remotePath, localPath string) (int64, error) {
	manager, _, _, err := s.transferDependencies()
	if err != nil {
		return 0, err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return 0, errno.New(errno.CodeInvalidArg, "session.not_found", "")
	}
	localPath = strings.TrimSpace(localPath)
	if !filepath.IsAbs(localPath) {
		return 0, errno.New(errno.CodeInvalidArg, "ai.agent.tool.path_not_absolute", "")
	}
	normalized, err := model.NormalizeRemotePath(remotePath)
	if err != nil {
		return 0, errno.New(errno.CodeInvalidArg, "sftp.remote_path_invalid", "")
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, agentTransferTimeout)
	defer cancel()

	fileSystem, err := manager.TransferFileSystem(ctx, sessionID)
	if err != nil {
		return 0, ServiceError(err)
	}
	defer func() { _ = fileSystem.Close() }()

	info, err := fileSystem.StatContext(ctx, normalized)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, errno.New(errno.CodeNotFound, "sftp.remote_path_not_found", "")
		}
		return 0, ServiceError(err)
	}
	if info.IsDir() {
		return 0, errno.New(errno.CodeInvalidArg, "ai.agent.tool.not_a_regular_file", "")
	}
	if info.Size() > agentTransferMaxBytes {
		return 0, errno.New(errno.CodeInvalidArg, "ai.agent.tool.transfer_too_large", "")
	}

	if _, statErr := os.Stat(localPath); statErr == nil {
		return 0, errno.New(errno.CodeInvalidArg, "ai.agent.tool.transfer_target_exists", "")
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return 0, errno.New(errno.CodeInvalidArg, "ai.agent.tool.transfer_stat_failed", "")
	}

	if err := fileSystem.DownloadFileContext(ctx, normalized, localPath, info.Size(), false, nil); err != nil {
		return 0, ServiceError(err)
	}
	return info.Size(), nil
}
