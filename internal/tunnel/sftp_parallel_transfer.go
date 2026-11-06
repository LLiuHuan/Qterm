package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sync"

	"github.com/pkg/sftp"
)

const parallelTransferSubrangeBytes = 256 * 1024

type parallelUploadError struct {
	err       error
	temporary string
}

func (e *parallelUploadError) Error() string {
	if e.temporary == "" {
		return fmt.Sprintf("parallel upload: %v", e.err)
	}
	return fmt.Sprintf("parallel upload (temporary %s): %v", e.temporary, e.err)
}

func (e *parallelUploadError) Unwrap() error { return e.err }

func (e *parallelUploadError) TemporaryPath() string { return e.temporary }

func (f *FileSystem) parallelReady() bool {
	return f != nil && f.client != nil && f.group != nil
}

func (f *FileSystem) uploadReplacingParallel(
	ctx context.Context,
	source *os.File,
	remotePath string,
	expectedSize int64,
	plan sftpParallelPlan,
	progress func(int64) error,
	seed *subsystem,
	releaseSeed func(bool),
	desiredConnections int,
	resume *SFTPResumeManifest,
) (resultErr error) {
	stopCancellation := watchParallelUploadCancellation(ctx, seed)
	defer func() {
		stopCancellation()
		if releaseSeed != nil &&
			(ctx.Err() != nil || resultErr != nil && isBrokenSubsystemError(resultErr)) {
			releaseSeed(true)
		}
	}()
	temporaryPath, target, restarted, err := f.openUploadTemporary(remotePath, resume)
	if err != nil {
		return err
	}
	if restarted && resume != nil {

		resume.Reset(expectedSize, plan.chunkSize, resume.Fingerprint)
	}

	if err := target.Close(); err != nil {
		_ = f.client.Remove(temporaryPath)
		return fmt.Errorf("close remote upload temporary file: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}

		if resume != nil && errors.Is(ctx.Err(), context.Canceled) {
			return
		}
		_ = f.client.Remove(temporaryPath)
	}()

	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek local upload source: %w", err)
	}

	writeErr := runParallelChunks(
		ctx,
		f.group,
		plan,
		expectedSize,
		progress,
		seed,
		releaseSeed,
		desiredConnections,
		resume,
		func(system *FileSystem) (parallelChunkWorker, error) {
			remote, openErr := system.sftpClient().OpenFile(temporaryPath, os.O_WRONLY)
			if openErr != nil {
				return parallelChunkWorker{}, fmt.Errorf("open remote upload worker: %w", openErr)
			}
			return parallelChunkWorker{
				source: func(_ *FileSystem, buffer []byte, offset int64, _ func(int64) error) error {

					read, readErr := source.ReadAt(buffer, offset)
					if readErr != nil && !(errors.Is(readErr, io.EOF) && read == len(buffer)) {
						return fmt.Errorf("read local upload source at %d: %w", offset, readErr)
					}
					if read != len(buffer) {
						return fmt.Errorf("read local upload source at %d: got %d bytes, want %d", offset, read, len(buffer))
					}
					return nil
				},
				sink: func(_ *FileSystem, buffer []byte, offset int64, report func(int64) error) error {
					return writeRemoteChunkAt(remote, buffer, offset, report)
				},
				close: remote.Close,
			}, nil
		},
	)
	if writeErr != nil {
		return &parallelUploadError{err: writeErr, temporary: temporaryPath}
	}
	if err := ctx.Err(); err != nil {
		return &parallelUploadError{err: err, temporary: temporaryPath}
	}

	if err := truncateRemoteFile(f.client, temporaryPath, expectedSize); err != nil {
		return &parallelUploadError{err: err, temporary: temporaryPath}
	}
	if err := ctx.Err(); err != nil {
		return &parallelUploadError{err: err, temporary: temporaryPath}
	}
	if err := f.commitReplacing(temporaryPath, remotePath); err != nil {
		return &parallelUploadError{err: err, temporary: temporaryPath}
	}
	committed = true
	return nil
}

func (f *FileSystem) uploadCreateOnlyParallel(
	ctx context.Context,
	source *os.File,
	remotePath string,
	expectedSize int64,
	plan sftpParallelPlan,
	progress func(int64) error,
	seed *subsystem,
	releaseSeed func(bool),
	desiredConnections int,
	resume *SFTPResumeManifest,
) (resultErr error) {
	stopCancellation := watchParallelUploadCancellation(ctx, seed)
	defer func() {
		stopCancellation()
		if releaseSeed != nil &&
			(ctx.Err() != nil || resultErr != nil && isBrokenSubsystemError(resultErr)) {
			releaseSeed(true)
		}
	}()
	if exists, err := f.remotePathExists(remotePath); err != nil {
		return fmt.Errorf("inspect remote upload destination: %w", err)
	} else if exists {
		return fs.ErrExist
	}
	temporaryPath, target, restarted, err := f.openUploadTemporary(remotePath, resume)
	if err != nil {
		return err
	}
	if restarted && resume != nil {

		resume.Reset(expectedSize, plan.chunkSize, resume.Fingerprint)
	}
	if err := target.Close(); err != nil {
		_ = f.client.Remove(temporaryPath)
		return fmt.Errorf("close remote upload temporary file: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}

		if resume != nil && errors.Is(ctx.Err(), context.Canceled) {
			return
		}
		_ = f.client.Remove(temporaryPath)
	}()

	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek local upload source: %w", err)
	}
	writeErr := runParallelChunks(
		ctx,
		f.group,
		plan,
		expectedSize,
		progress,
		seed,
		releaseSeed,
		desiredConnections,
		resume,
		func(system *FileSystem) (parallelChunkWorker, error) {
			remote, openErr := system.sftpClient().OpenFile(temporaryPath, os.O_WRONLY)
			if openErr != nil {
				return parallelChunkWorker{}, fmt.Errorf("open remote upload worker: %w", openErr)
			}
			return parallelChunkWorker{
				source: func(_ *FileSystem, buffer []byte, offset int64, _ func(int64) error) error {
					read, readErr := source.ReadAt(buffer, offset)
					if readErr != nil && !(errors.Is(readErr, io.EOF) && read == len(buffer)) {
						return fmt.Errorf("read local upload source at %d: %w", offset, readErr)
					}
					if read != len(buffer) {
						return fmt.Errorf("read local upload source at %d: got %d bytes, want %d", offset, read, len(buffer))
					}
					return nil
				},
				sink: func(_ *FileSystem, buffer []byte, offset int64, report func(int64) error) error {
					return writeRemoteChunkAt(remote, buffer, offset, report)
				},
				close: remote.Close,
			}, nil
		},
	)
	if writeErr != nil {
		return &parallelUploadError{err: writeErr, temporary: temporaryPath}
	}
	if err := ctx.Err(); err != nil {
		return &parallelUploadError{err: err, temporary: temporaryPath}
	}
	if err := truncateRemoteFile(f.client, temporaryPath, expectedSize); err != nil {
		return &parallelUploadError{err: err, temporary: temporaryPath}
	}
	if err := ctx.Err(); err != nil {
		return &parallelUploadError{err: err, temporary: temporaryPath}
	}
	if err := f.commitCreateOnly(temporaryPath, remotePath); err != nil {
		return &parallelUploadError{err: err, temporary: temporaryPath}
	}
	committed = true
	return nil
}

func watchParallelUploadCancellation(ctx context.Context, seed *subsystem) func() {
	if ctx == nil || seed == nil || seed.fileSystem == nil {
		return func() {}
	}
	return watchSubsystemCancellation(ctx, seed)
}

func (f *FileSystem) parallelDownload(
	ctx context.Context,
	remotePath string,
	expectedSize int64,
	plan sftpParallelPlan,
	temporary *os.File,
	progress func(int64) error,
	seed *subsystem,
	releaseSeed func(bool),
	desiredConnections int,
) error {
	return runParallelChunks(
		ctx,
		f.group,
		plan,
		expectedSize,
		progress,
		seed,
		releaseSeed,
		desiredConnections,
		nil,
		func(system *FileSystem) (parallelChunkWorker, error) {
			remote, openErr := system.sftpClient().Open(remotePath)
			if openErr != nil {
				return parallelChunkWorker{}, fmt.Errorf("open remote download worker: %w", openErr)
			}
			return parallelChunkWorker{
				source: func(_ *FileSystem, buffer []byte, offset int64, report func(int64) error) error {
					return readRemoteChunkAt(remote, buffer, offset, report)
				},
				sink: func(_ *FileSystem, buffer []byte, offset int64, _ func(int64) error) error {
					return writeLocalChunkAt(temporary, buffer, offset, nil)
				},
				close: remote.Close,
			}, nil
		},
	)
}

func writeRemoteChunkAt(
	file *sftp.File,
	buffer []byte,
	offset int64,
	report func(int64) error,
) error {
	if file == nil {
		return ErrFileSystemNotReady
	}
	return forEachParallelSubrange(buffer, offset, report, func(part []byte, partOffset int64) error {
		written := 0
		for written < len(part) {
			count, err := file.WriteAt(part[written:], partOffset+int64(written))
			if count < 0 || count > len(part)-written {
				return fmt.Errorf("write remote chunk at %d: invalid byte count %d", partOffset+int64(written), count)
			}
			written += count
			if err != nil {
				return fmt.Errorf("write remote chunk at %d: %w", partOffset+int64(written-count), err)
			}
			if count == 0 {
				return io.ErrShortWrite
			}
		}
		return nil
	})
}

func readRemoteChunkAt(
	file *sftp.File,
	buffer []byte,
	offset int64,
	report func(int64) error,
) error {
	if file == nil {
		return ErrFileSystemNotReady
	}
	return forEachParallelSubrange(buffer, offset, report, func(part []byte, partOffset int64) error {
		read, readErr := file.ReadAt(part, partOffset)
		if readErr != nil && !(errors.Is(readErr, io.EOF) && read == len(part)) {
			return fmt.Errorf("read remote chunk at %d: %w", partOffset, readErr)
		}
		if read != len(part) {
			return fmt.Errorf("read remote chunk at %d: got %d bytes, want %d", partOffset, read, len(part))
		}
		return nil
	})
}

func forEachParallelSubrange(
	buffer []byte,
	offset int64,
	report func(int64) error,
	operation func([]byte, int64) error,
) error {
	if operation == nil {
		return errors.New("parallel SFTP subrange operation is nil")
	}
	count := (len(buffer) + parallelTransferSubrangeBytes - 1) / parallelTransferSubrangeBytes
	errs := make([]error, count)
	var workers sync.WaitGroup
	workers.Add(count)
	for index := 0; index < count; index++ {
		start := index * parallelTransferSubrangeBytes
		end := start + parallelTransferSubrangeBytes
		if end > len(buffer) {
			end = len(buffer)
		}
		go func(index int, part []byte, partOffset int64) {
			defer workers.Done()
			if err := operation(part, partOffset); err != nil {
				errs[index] = err
				return
			}
			if report != nil {
				errs[index] = report(int64(len(part)))
			}
		}(index, buffer[start:end], offset+int64(start))
	}
	workers.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func writeLocalChunkAt(file *os.File, buffer []byte, offset int64, report func(int64) error) error {
	written := 0
	for written < len(buffer) {
		count, err := file.WriteAt(buffer[written:], offset+int64(written))
		if err != nil {
			return fmt.Errorf("write local download temporary file at %d: %w", offset+int64(written), err)
		}
		if count == 0 {
			return io.ErrShortWrite
		}
		if count < 0 || count > len(buffer)-written {
			return fmt.Errorf("write local download temporary file at %d: invalid byte count %d", offset+int64(written), count)
		}
		written += count
	}
	if report != nil {
		return report(int64(written))
	}
	return nil
}

func truncateRemoteFile(client *sftp.Client, remotePath string, size int64) error {
	file, err := client.OpenFile(remotePath, os.O_WRONLY)
	if err != nil {
		return fmt.Errorf("open remote transfer file for truncate: %w", err)
	}
	if err := file.Truncate(size); err != nil {
		_ = file.Close()
		return fmt.Errorf("truncate remote transfer file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close remote transfer file after truncate: %w", err)
	}
	return nil
}
