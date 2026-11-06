package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/pkg/sftp"
)

const remoteDeleteEntryLimit = 10000

var errRemoteDeleteTooLarge = errors.New("recursive delete aborted: entry limit reached")

type entryRemover interface {
	Lstat(p string) (os.FileInfo, error)
	ReadDir(p string) ([]os.FileInfo, error)
	Remove(p string) error
	RemoveDirectory(p string) error
}

var _ entryRemover = (*sftp.Client)(nil)

func (f *FileSystem) CreateDirectoryContext(ctx context.Context, remotePath string) error {
	return f.mutatePath(ctx, func() error { return f.client.Mkdir(remotePath) })
}

func (f *FileSystem) DeleteEntryContext(ctx context.Context, remotePath string) error {
	return f.mutatePath(ctx, func() error {
		return removeEntryTree(ctx, f.client, remotePath)
	})
}

func removeEntryTree(ctx context.Context, client entryRemover, remotePath string) error {
	if ctx == nil {
		return errors.New("SFTP context is nil")
	}
	if client == nil {
		return ErrFileSystemNotReady
	}
	budget := remoteDeleteEntryLimit
	return removeEntry(ctx, client, remotePath, &budget)
}

func removeEntry(ctx context.Context, client entryRemover, remotePath string, budget *int) error {

	if err := ctx.Err(); err != nil {
		return err
	}
	if *budget <= 0 {
		return fmt.Errorf("%w at %q", errRemoteDeleteTooLarge, remotePath)
	}
	*budget = *budget - 1
	info, err := client.Lstat(remotePath)
	if err != nil {
		return fmt.Errorf("inspect SFTP entry %q: %w", remotePath, err)
	}

	if !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
		if err := client.Remove(remotePath); err != nil {
			return fmt.Errorf("remove SFTP entry %q: %w", remotePath, err)
		}
		return nil
	}
	entries, err := client.ReadDir(remotePath)
	if err != nil {
		return fmt.Errorf("list SFTP directory %q: %w", remotePath, err)
	}
	for _, entry := range entries {
		if entry == nil {
			continue
		}

		if err := removeEntry(ctx, client, path.Join(remotePath, entry.Name()), budget); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := client.RemoveDirectory(remotePath); err != nil {
		return fmt.Errorf("remove SFTP directory %q: %w", remotePath, err)
	}
	return nil
}

func (f *FileSystem) CreateFileContext(ctx context.Context, remotePath string) error {
	return f.mutatePath(ctx, func() error {
		file, err := f.client.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
		if err != nil {
			return err
		}
		return file.Close()
	})
}

func (f *FileSystem) RenameEntryContext(ctx context.Context, oldPath, newPath string) error {
	return f.mutatePath(ctx, func() error {
		if _, err := f.client.Lstat(oldPath); err != nil {
			return fmt.Errorf("inspect SFTP source: %w", err)
		}
		if _, err := f.client.Lstat(newPath); err == nil {
			return fs.ErrExist
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("inspect SFTP destination: %w", err)
		}

		return f.client.Rename(oldPath, newPath)
	})
}

func (f *FileSystem) ChmodEntryContext(ctx context.Context, remotePath string, mode os.FileMode) error {
	return f.mutatePath(ctx, func() error {
		info, err := f.client.Lstat(remotePath)
		if err != nil {
			return fmt.Errorf("inspect SFTP entry: %w", err)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return errors.New("cannot chmod symbolic link")
		}
		return f.client.Chmod(remotePath, mode.Perm())
	})
}

func (f *FileSystem) ChownEntryContext(ctx context.Context, remotePath string, uid, gid uint32) error {
	return f.mutatePath(ctx, func() error {
		info, err := f.client.Lstat(remotePath)
		if err != nil {
			return fmt.Errorf("inspect SFTP entry: %w", err)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return errors.New("cannot chown symbolic link")
		}
		return f.client.Chown(remotePath, int(uid), int(gid))
	})
}

func (f *FileSystem) mutatePath(ctx context.Context, operation func() error) error {
	if ctx == nil {
		return errors.New("SFTP context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if f == nil || f.client == nil {
		return ErrFileSystemNotReady
	}

	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		_ = f.Close()
		close(done)
	})
	err := operation()
	if !stop() {
		<-done
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("mutate SFTP entry: %w", err)
	}
	return nil
}

func (f *FileSystem) EnsureDirectoryContext(ctx context.Context, remotePath string) error {
	return f.mutatePath(ctx, func() error {
		info, err := f.client.Lstat(remotePath)
		if err == nil {
			if info.IsDir() {
				return nil
			}
			return errors.New("upload destination is not a directory")
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return f.client.Mkdir(remotePath)
	})
}
