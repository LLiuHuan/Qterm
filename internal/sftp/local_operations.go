package sftp

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"changeme/internal/errno"
)

func (s *Service) CreateLocalFile(ctx context.Context, token, name string) error {
	return s.localMutate(ctx, token, name, func(path string) error {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return err
		}
		return file.Close()
	})
}

func (s *Service) CreateLocalDirectory(ctx context.Context, token, name string) error {
	return s.localMutate(ctx, token, name, func(path string) error { return os.Mkdir(path, 0o755) })
}

func (s *Service) RenameLocalEntry(ctx context.Context, token, name, newName string) error {
	if !isSafeLocalBasename(name) || !isSafeLocalBasename(newName) {
		return errno.New(errno.CodeInvalidArg, "sftp.local_entry_invalid", "")
	}
	browser, err := s.localDirectoryDependencies()
	if err != nil {
		return err
	}
	directory, err := browser.resolveDirectory(ctx, token)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	oldPath, newPath := filepath.Join(directory, name), filepath.Join(directory, newName)
	oldInfo, err := os.Lstat(oldPath)
	if err != nil {
		return localEntryError(err, "sftp.local_source_missing")
	}
	newInfo, err := os.Lstat(newPath)
	if err == nil && !os.SameFile(oldInfo, newInfo) {
		return errno.New(errno.CodeInvalidArg, "sftp.destination_exists", "")
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return localEntryError(err, "sftp.local_entry_failed")
	}

	if name == newName {
		return nil
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return localEntryError(err, "sftp.local_entry_failed")
	}
	return nil
}

func (s *Service) ChmodLocalEntry(ctx context.Context, token, name, mode string) error {
	if !isSafeLocalBasename(name) {
		return errno.New(errno.CodeInvalidArg, "sftp.local_entry_invalid", "")
	}
	if strings.TrimSpace(mode) == "" {
		return errno.New(errno.CodeInvalidArg, "sftp.local_mode_invalid", "")
	}
	browser, err := s.localDirectoryDependencies()
	if err != nil {
		return err
	}
	directory, err := browser.resolveDirectory(ctx, token)
	if err != nil {
		return err
	}
	path := filepath.Join(directory, name)
	info, err := os.Lstat(path)
	if err != nil {
		return localEntryError(err, "sftp.local_source_missing")
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return errno.New(errno.CodeInvalidArg, "sftp.local_symlink_unsupported", "")
	}
	parsed, ok := parsePermissionMode(mode)
	if !ok {
		return errno.New(errno.CodeInvalidArg, "sftp.local_mode_invalid", "")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Chmod(path, parsed); err != nil {
		return localEntryError(err, "sftp.local_entry_failed")
	}
	return nil
}

func (s *Service) DeleteLocalEntry(ctx context.Context, token, name string) error {
	if !isSafeLocalBasename(name) {
		return errno.New(errno.CodeInvalidArg, "sftp.local_entry_invalid", "")
	}
	browser, err := s.localDirectoryDependencies()
	if err != nil {
		return err
	}
	directory, err := browser.resolveDirectory(ctx, token)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(directory, name)); err != nil {
		return localEntryError(err, "sftp.local_delete_failed")
	}
	return nil
}

func (s *Service) localMutate(ctx context.Context, token, name string, op func(string) error) error {
	if !isSafeLocalBasename(name) {
		return errno.New(errno.CodeInvalidArg, "sftp.local_entry_invalid", "")
	}
	browser, err := s.localDirectoryDependencies()
	if err != nil {
		return err
	}
	directory, err := browser.resolveDirectory(ctx, token)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := op(filepath.Join(directory, name)); err != nil {
		return localEntryError(err, "sftp.local_entry_failed")
	}
	return nil
}

func localEntryError(err error, fallbackKey string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, fs.ErrExist):
		return errno.New(errno.CodeInvalidArg, "sftp.destination_exists", "")
	case errors.Is(err, fs.ErrNotExist):
		return errno.New(errno.CodeNotFound, fallbackKey, "")
	case errors.Is(err, fs.ErrPermission):
		return errno.New(errno.CodePermissionDenied, "sftp.permission_denied", "")
	default:
		return errno.Wrap(errno.CodeUnknown, fallbackKey, err)
	}
}
