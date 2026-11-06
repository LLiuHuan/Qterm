package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sync"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

var (
	ErrFileSystemNotReady = errors.New("SSH file system is not ready")
)

type FileSystem struct {
	client  *sftp.Client
	session *ssh.Session

	group *FileSystemGroup

	capabilities *serverCapabilities

	closeOnce sync.Once
	closeErr  error

	renameOnce  sync.Once
	posixRename bool
}

const posixRenameExtension = "posix-rename@openssh.com"

type serverCapabilities struct {
	renameOnce  sync.Once
	posixRename bool
}

func (f *FileSystem) supportsPosixRename() bool {
	if capabilities := f.capabilities; capabilities != nil {
		capabilities.renameOnce.Do(func() {
			_, capabilities.posixRename = f.client.HasExtension(posixRenameExtension)
		})
		return capabilities.posixRename
	}
	f.renameOnce.Do(func() {
		_, f.posixRename = f.client.HasExtension(posixRenameExtension)
	})
	return f.posixRename
}

func (f *FileSystem) WorkingDirectory(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", errors.New("SFTP context is nil")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if f == nil || f.client == nil {
		return "", ErrFileSystemNotReady
	}
	return f.client.Getwd()
}

func (c *Connection) OpenFileSystem(ctx context.Context) (*PooledFileSystem, error) {
	if ctx == nil {
		return nil, errors.New("SFTP context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c == nil || c.client == nil {
		return nil, ErrFileSystemNotReady
	}
	return c.fileSystemGroup().acquire(ctx)
}

func (c *Connection) OpenFileSystemForConnections(
	ctx context.Context,
	planned int,
) (*PooledFileSystem, error) {
	if ctx == nil {
		return nil, errors.New("SFTP context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c == nil || c.client == nil {
		return nil, ErrFileSystemNotReady
	}
	return c.fileSystemGroup().acquireForConnections(ctx, planned)
}

func (c *Connection) fileSystemGroup() *FileSystemGroup {
	c.fsMu.Lock()
	defer c.fsMu.Unlock()
	if c.fileSystems == nil {

		c.fileSystems = newFileSystemGroup(c.client, c.auxiliaryProvisioner, sftpParallelMaxWidth)
	}
	return c.fileSystems
}

func (f *FileSystem) ReadDirContext(ctx context.Context, directory string) ([]fs.FileInfo, error) {
	if ctx == nil {
		return nil, errors.New("SFTP context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f == nil || f.client == nil {
		return nil, ErrFileSystemNotReady
	}
	entries, err := f.client.ReadDirContext(ctx, directory)
	if err != nil {
		return nil, fmt.Errorf("read SFTP directory: %w", err)
	}
	return entries, nil
}

func (f *FileSystem) StatContext(ctx context.Context, remotePath string) (fs.FileInfo, error) {
	if ctx == nil {
		return nil, errors.New("SFTP context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f == nil || f.client == nil {
		return nil, ErrFileSystemNotReady
	}
	info, err := f.client.Stat(remotePath)
	if err != nil {
		return nil, fmt.Errorf("stat SFTP path: %w", err)
	}
	return info, nil
}

func (f *FileSystem) OpenFile(ctx context.Context, path string) (io.ReadCloser, error) {
	if ctx == nil {
		return nil, errors.New("SFTP context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f == nil || f.client == nil {
		return nil, ErrFileSystemNotReady
	}
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		_ = f.Close()
		close(done)
	})
	file, err := f.client.Open(path)
	if !stop() {
		<-done
	}
	if ctx.Err() != nil {
		if file != nil {
			_ = file.Close()
		}
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, fmt.Errorf("open remote file: %w", err)
	}
	return file, nil
}

func (f *FileSystem) CreateTransferFile(ctx context.Context, finalPath string) (string, io.WriteCloser, error) {
	if ctx == nil {
		return "", nil, errors.New("SFTP context is nil")
	}
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	if f == nil || f.client == nil {
		return "", nil, ErrFileSystemNotReady
	}
	return f.createRemoteTemporary(finalPath)
}

func (f *FileSystem) CommitTransferFile(
	ctx context.Context,
	temporaryPath string,
	finalPath string,
	overwrite bool,
) error {
	if ctx == nil {
		return errors.New("SFTP context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if f == nil || f.client == nil {
		return ErrFileSystemNotReady
	}
	if overwrite {
		return f.commitReplacing(temporaryPath, finalPath)
	}

	if exists, inspectErr := f.remotePathExists(finalPath); inspectErr != nil {
		return fmt.Errorf("inspect remote transfer destination: %w", inspectErr)
	} else if exists {
		return fs.ErrExist
	}
	if err := f.client.Rename(temporaryPath, finalPath); err != nil {
		return fmt.Errorf("commit remote transfer destination: %w", err)
	}
	return nil
}

func (f *FileSystem) commitReplacing(temporaryPath string, finalPath string) error {
	if f.supportsPosixRename() {

		f.preserveDestinationMode(temporaryPath, finalPath)
		if err := f.client.PosixRename(temporaryPath, finalPath); err != nil {
			return fmt.Errorf("replace remote transfer destination: %w", err)
		}
		return nil
	}
	return f.replaceWithBackup(temporaryPath, finalPath)
}

func (f *FileSystem) preserveDestinationMode(temporaryPath string, finalPath string) {
	if info, err := f.client.Lstat(finalPath); err == nil && info.Mode().IsRegular() {
		_ = f.client.Chmod(temporaryPath, info.Mode().Perm())
	}
}

func (f *FileSystem) replaceWithBackup(temporaryPath string, finalPath string) error {
	backupPath, err := f.backupName(finalPath)
	if err != nil {
		return err
	}
	hadTarget := false
	if info, statErr := f.client.Lstat(finalPath); statErr == nil {
		if info.IsDir() {
			return errors.New("remote transfer destination is a directory")
		}
		f.preserveDestinationMode(temporaryPath, finalPath)
		if err := f.client.Rename(finalPath, backupPath); err != nil {
			return fmt.Errorf("backup remote transfer destination: %w", err)
		}
		hadTarget = true
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return fmt.Errorf("inspect remote transfer destination: %w", statErr)
	}
	if err := f.client.Rename(temporaryPath, finalPath); err != nil {
		if hadTarget {

			_ = f.client.Rename(backupPath, finalPath)
		}
		return fmt.Errorf("commit remote transfer destination: %w", err)
	}
	if hadTarget {

		_ = f.client.Remove(backupPath)
	}
	return nil
}

func (f *FileSystem) backupName(finalPath string) (string, error) {
	directory := path.Dir(finalPath)
	base := path.Base(finalPath)
	for attempt := 0; attempt < 4; attempt++ {
		suffix, err := randomTransferSuffix()
		if err != nil {
			return "", fmt.Errorf("generate remote backup name: %w", err)
		}
		candidate := path.Join(directory, ".qterm-backup-"+base+"-"+suffix)
		exists, err := f.remotePathExists(candidate)
		if err != nil {
			return "", fmt.Errorf("inspect remote backup name: %w", err)
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", errors.New("create remote backup name: name collision")
}

func (f *FileSystem) RemoveTransferFile(ctx context.Context, remotePath string) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if f == nil || f.client == nil {
		return ErrFileSystemNotReady
	}
	return f.client.Remove(remotePath)
}

func (f *FileSystem) Close() error {
	if f == nil {
		return nil
	}
	f.closeOnce.Do(func() {
		var sessionErr error
		if f.session != nil {
			sessionErr = normalizeFileSystemCloseError(f.session.Close())
		}
		var clientErr error
		if f.client != nil {
			clientErr = normalizeFileSystemCloseError(f.client.Close())
		}
		f.closeErr = errors.Join(sessionErr, clientErr)
	})
	return f.closeErr
}

func normalizeFileSystemCloseError(err error) error {

	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func fileSystemOpenError(ctx context.Context, operation string, err error) error {
	if contextErr := ctx.Err(); contextErr != nil {
		return contextErr
	}
	return fmt.Errorf("%s: %w", operation, err)
}
