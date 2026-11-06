package sftp

import (
	"bufio"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"changeme/internal/errno"
	"changeme/internal/model"
	sessionapp "changeme/internal/session"
)

type entryFileSystem interface {
	CreateDirectoryContext(context.Context, string) error
	CreateFileContext(context.Context, string) error
	DeleteEntryContext(context.Context, string) error
	RenameEntryContext(context.Context, string, string) error
	ChmodEntryContext(context.Context, string, os.FileMode) error

	StatContext(context.Context, string) (fs.FileInfo, error)
}

type remoteTreeRemover interface {
	RemoveRemoteTree(ctx context.Context, sessionID, remotePath string) error
}

var _ remoteTreeRemover = (*sessionapp.SessionManager)(nil)

type ownershipFileSystem interface {
	ChownEntryContext(context.Context, string, uint32, uint32) error
}

type principalFileSystem interface {
	OpenFile(context.Context, string) (io.ReadCloser, error)
}

func (s *Service) CreateRemoteDirectory(ctx context.Context, sessionID, directory, name string) error {
	directory, err := model.NormalizeRemoteDirectory(directory)

	if err != nil || !safeRemoteName(name) {
		return errno.New(errno.CodeInvalidArg, "sftp.folder_name_invalid", "")
	}
	return s.mutateEntry(ctx, sessionID, path.Join(directory, name), false)
}

func (s *Service) DeleteRemoteEntry(ctx context.Context, sessionID, remotePath string) error {
	normalized, err := model.NormalizeRemotePath(remotePath)
	if err != nil || normalized == "/" || normalized != remotePath {
		return errno.New(errno.CodeInvalidArg, "sftp.remote_path_invalid", "")
	}
	return s.mutateEntry(ctx, sessionID, normalized, true)
}

func (s *Service) CreateRemoteFile(ctx context.Context, sessionID, directory, name string) error {
	directory, err := model.NormalizeRemoteDirectory(directory)
	if err != nil || !safeRemoteName(name) {
		return errno.New(errno.CodeInvalidArg, "sftp.file_name_invalid", "")
	}
	return s.remoteOperation(ctx, sessionID, path.Join(directory, name), func(operationCtx context.Context, op entryFileSystem, p string) error {
		return op.CreateFileContext(operationCtx, p)
	})
}

func (s *Service) RenameRemoteEntry(ctx context.Context, sessionID, remotePath, newName string) error {
	remotePath, err := model.NormalizeRemotePath(remotePath)
	if err != nil || remotePath == "/" || !safeRemoteName(newName) {
		return errno.New(errno.CodeInvalidArg, "sftp.remote_path_invalid", "")
	}
	directory := path.Dir(remotePath)
	return s.remoteOperation(ctx, sessionID, remotePath, func(operationCtx context.Context, op entryFileSystem, p string) error {
		return op.RenameEntryContext(operationCtx, p, path.Join(directory, newName))
	})
}

func (s *Service) ChmodRemoteEntry(ctx context.Context, sessionID, remotePath, mode string) error {
	remotePath, err := model.NormalizeRemotePath(remotePath)
	if err != nil || remotePath == "/" {
		return errno.New(errno.CodeInvalidArg, "sftp.remote_path_invalid", "")
	}
	parsed, ok := parsePermissionMode(mode)
	if !ok {
		return errno.New(errno.CodeInvalidArg, "sftp.remote_mode_invalid", "")
	}
	return s.remoteOperation(ctx, sessionID, remotePath, func(operationCtx context.Context, op entryFileSystem, p string) error {
		return op.ChmodEntryContext(operationCtx, p, parsed)
	})
}

func (s *Service) ListRemotePrincipals(ctx context.Context, sessionID string) (model.RemotePrincipals, error) {
	manager, _, _, err := s.transferDependencies()
	if err != nil {
		return model.RemotePrincipals{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	fileSystem, err := manager.TransferFileSystem(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return model.RemotePrincipals{}, ServiceError(err)
	}
	defer func() { _ = fileSystem.Close() }()
	reader, ok := fileSystem.(principalFileSystem)
	if !ok {
		return model.RemotePrincipals{}, errno.New(errno.CodeInvalidArg, "sftp.principals_unsupported", "")
	}
	users, err := readRemotePrincipals(ctx, reader, "/etc/passwd")
	if err != nil {
		return model.RemotePrincipals{}, entryOperationError(err)
	}
	groups, err := readRemotePrincipals(ctx, reader, "/etc/group")
	if err != nil {
		return model.RemotePrincipals{}, entryOperationError(err)
	}
	return model.RemotePrincipals{Users: users, Groups: groups}, nil
}

func (s *Service) SetRemoteEntryPermissions(
	ctx context.Context,
	sessionID, remotePath, mode string,
	uid, gid uint32,
) error {
	remotePath, err := model.NormalizeRemotePath(remotePath)
	if err != nil || remotePath == "/" {
		return errno.New(errno.CodeInvalidArg, "sftp.remote_path_invalid", "")
	}
	parsed, ok := parsePermissionMode(mode)
	if !ok {
		return errno.New(errno.CodeInvalidArg, "sftp.remote_mode_invalid", "")
	}
	return s.remoteOperation(ctx, sessionID, remotePath, func(operationCtx context.Context, op entryFileSystem, p string) error {
		ownership, supported := op.(ownershipFileSystem)
		if !supported {
			return errno.New(errno.CodeInvalidArg, "sftp.ownership_unsupported", "")
		}
		if err := ownership.ChownEntryContext(operationCtx, p, uid, gid); err != nil {
			return err
		}
		return op.ChmodEntryContext(operationCtx, p, parsed)
	})
}

const (
	maxPrincipalFileSize = 2 << 20

	maxRemotePrincipals = 1024
)

func readRemotePrincipals(ctx context.Context, fileSystem principalFileSystem, remotePath string) ([]model.RemotePrincipal, error) {
	file, err := fileSystem.OpenFile(ctx, remotePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(io.LimitReader(file, maxPrincipalFileSize))
	principals := make([]model.RemotePrincipal, 0, 32)
	seen := make(map[uint32]struct{})
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(principals) >= maxRemotePrincipals {
			break
		}
		fields := strings.Split(scanner.Text(), ":")
		if len(fields) < 3 {
			continue
		}

		name := strings.TrimSpace(fields[0])

		if name == "" || name[0] == '+' || name[0] == '-' || name[0] == '#' {
			continue
		}
		value, parseErr := strconv.ParseUint(strings.TrimSpace(fields[2]), 10, 32)
		if parseErr != nil {
			continue
		}
		id := uint32(value)
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		principals = append(principals, model.RemotePrincipal{ID: id, Name: name})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Slice(principals, func(i, j int) bool { return principals[i].Name < principals[j].Name })
	return principals, nil
}

func safeRemoteName(name string) bool {
	return name != "" && name != "." && name != ".." && len(name) <= 255 && !strings.ContainsAny(name, "/\\\x00\r\n")
}

func (s *Service) remoteOperation(ctx context.Context, sessionID, remotePath string, operation func(context.Context, entryFileSystem, string) error) error {
	manager, _, _, err := s.transferDependencies()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	fileSystem, err := manager.TransferFileSystem(ctx, sessionID)
	if err != nil {
		return ServiceError(err)
	}
	operations, ok := fileSystem.(entryFileSystem)
	if !ok {
		_ = fileSystem.Close()
		return errno.New(errno.CodeInvalidArg, "sftp.operation_unsupported", "")
	}

	operationErr := operation(ctx, operations, remotePath)

	_ = fileSystem.Close()
	return entryOperationError(operationErr)
}

func parsePermissionMode(value string) (os.FileMode, bool) {
	value = strings.TrimSpace(value)

	if len(value) != 3 && len(value) != 4 {
		return 0, false
	}
	for _, digit := range value {
		if digit < '0' || digit > '7' {
			return 0, false
		}
	}
	parsed, err := strconv.ParseUint(value, 8, 9)
	if err != nil || parsed > 0o777 {
		return 0, false
	}
	return os.FileMode(parsed), true
}

const (
	entryMutationTimeout = 10 * time.Second
	entryDeleteTimeout   = 5 * time.Minute
)

func (s *Service) mutateEntry(ctx context.Context, sessionID, remotePath string, remove bool) error {
	manager, _, _, err := s.transferDependencies()
	if err != nil {
		return err
	}
	timeout := entryMutationTimeout
	if remove {
		timeout = entryDeleteTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	fileSystem, err := manager.TransferFileSystem(ctx, sessionID)
	if err != nil {
		return ServiceError(err)
	}
	operations, ok := fileSystem.(entryFileSystem)
	if !ok {
		return errors.Join(errno.New(errno.CodeInvalidArg, "sftp.operation_unsupported", ""), fileSystem.Close())
	}
	var operationErr error
	if remove {

		operationErr = removeEntry(ctx, sessionID, remotePath, operations, remoteTreeRemoverOf(manager))
	} else {
		operationErr = operations.CreateDirectoryContext(ctx, remotePath)
	}
	_ = fileSystem.Close()

	if remove && errors.Is(operationErr, context.DeadlineExceeded) {
		return errno.New(errno.CodeTimeout, "sftp.entry_delete_incomplete", "")
	}
	return entryOperationError(operationErr)
}

func remoteTreeRemoverOf(manager any) remoteTreeRemover {
	remover, _ := manager.(remoteTreeRemover)
	return remover
}

func removeEntry(
	ctx context.Context,
	sessionID string,
	remotePath string,
	operations entryFileSystem,
	remover remoteTreeRemover,
) error {

	info, statErr := operations.StatContext(ctx, remotePath)
	if statErr != nil || info == nil || !info.IsDir() || remover == nil {
		return operations.DeleteEntryContext(ctx, remotePath)
	}
	switch err := remover.RemoveRemoteTree(ctx, sessionID, remotePath); {
	case err == nil:
		return nil
	case errors.Is(err, sessionapp.ErrRemoteRemoveUnavailable):

		return operations.DeleteEntryContext(ctx, remotePath)
	default:
		return err
	}
}

func entryOperationError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, fs.ErrExist):
		return errno.New(errno.CodeInvalidArg, "sftp.destination_exists", "")
	case errors.Is(err, fs.ErrNotExist):
		return errno.New(errno.CodeNotFound, "sftp.not_found", "")
	case errors.Is(err, fs.ErrPermission):
		return errno.New(errno.CodePermissionDenied, "sftp.permission_denied", "")
	default:
		message := strings.ToLower(err.Error())

		if strings.Contains(message, "already exists") {
			return errno.New(errno.CodeInvalidArg, "sftp.destination_exists", "")
		}
		if strings.Contains(message, "symbolic link") {
			return errno.New(errno.CodeInvalidArg, "sftp.remote_symlink_unsupported", "")
		}

		if strings.Contains(message, "entry limit reached") {
			return errno.New(errno.CodeInvalidArg, "sftp.entry_delete_too_large", "")
		}
		return errno.Wrap(errno.CodeUnknown, "sftp.entry_operation_failed", err)
	}
}
