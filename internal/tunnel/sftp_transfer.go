package tunnel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const transferBufferSize = 64 * 1024

func (f *FileSystem) parallelDesiredConnections() int {
	if f == nil || f.group == nil {
		return 1
	}
	return normalizedConnectionCount(f.group.DesiredConnections())
}

func (f *FileSystem) UploadFileContext(
	ctx context.Context,
	localPath string,
	remotePath string,
	expectedSize int64,
	overwrite bool,
	progress func(int64) error,
) error {
	return f.uploadFile(ctx, localPath, remotePath, expectedSize, overwrite, progress)
}

func (f *FileSystem) UploadFileParallel(
	ctx context.Context,
	localPath string,
	remotePath string,
	expectedSize int64,
	overwrite bool,
	preserveDestination bool,
	progress func(int64) error,
) error {
	return f.uploadFileParallelWithSeed(
		ctx, localPath, remotePath, expectedSize, overwrite, preserveDestination, progress,
		nil, nil, f.parallelDesiredConnections(),
	)
}

func (f *FileSystem) uploadFileParallelWithSeed(
	ctx context.Context,
	localPath string,
	remotePath string,
	expectedSize int64,
	overwrite bool,
	preserveDestination bool,
	progress func(int64) error,
	seed *subsystem,
	releaseSeed func(bool),
	desiredConnections int,
) error {
	_ = preserveDestination
	if plan, ok := planSFTPParallelTransfer(expectedSize); ok && f.parallelReady() {
		return f.uploadFileParallel(ctx, localPath, remotePath, expectedSize, overwrite, plan, progress, seed, releaseSeed, desiredConnections)
	}
	return f.uploadFile(ctx, localPath, remotePath, expectedSize, overwrite, progress)
}

func (f *FileSystem) uploadFileParallel(
	ctx context.Context,
	localPath string,
	remotePath string,
	expectedSize int64,
	overwrite bool,
	plan sftpParallelPlan,
	progress func(int64) error,
	seed *subsystem,
	releaseSeed func(bool),
	desiredConnections int,
) error {
	if err := transferReady(ctx, f, expectedSize, progress); err != nil {
		return err
	}
	source, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local upload source: %w", err)
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return fmt.Errorf("inspect local upload source: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() != expectedSize {
		return errors.New("local upload source changed or is not a regular file")
	}
	if overwrite {
		return f.uploadReplacingParallel(
			ctx, source, remotePath, expectedSize, plan, progress, seed, releaseSeed, desiredConnections, nil,
		)
	}
	return f.uploadCreateOnlyParallel(ctx, source, remotePath, expectedSize, plan, progress, seed, releaseSeed, desiredConnections, nil)
}

func (f *FileSystem) uploadFileParallelWithResume(
	ctx context.Context,
	localPath string,
	remotePath string,
	expectedSize int64,
	overwrite bool,
	_ bool,
	progress func(int64) error,
	manifest *SFTPResumeManifest,
	seed *subsystem,
	releaseSeed func(bool),
	desiredConnections int,
) error {
	plan, ok := planSFTPParallelTransfer(expectedSize)
	if !ok || !f.parallelReady() {
		return errors.New("resume is only supported for parallel uploads")
	}
	if !manifest.EnsurePlan(plan.chunkSize) {

		manifest.Reset(expectedSize, plan.chunkSize, "")
	}
	source, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local upload source: %w", err)
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return fmt.Errorf("inspect local upload source: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() != expectedSize {
		return errors.New("local upload source changed or is not a regular file")
	}
	fingerprint, err := resumeSourceFingerprint(localPath)
	if err != nil {
		return err
	}
	if manifest.Fingerprint == "" {
		manifest.Fingerprint = fingerprint
	} else if manifest.Fingerprint != fingerprint {

		manifest.Reset(expectedSize, plan.chunkSize, fingerprint)
	}
	if err := verifyResumeBeforeResume(f, manifest, expectedSize); err != nil {
		manifest.Reset(expectedSize, plan.chunkSize, fingerprint)
	}
	if overwrite {
		return f.uploadReplacingParallel(
			ctx, source, remotePath, expectedSize, plan, progress, seed, releaseSeed, desiredConnections, manifest,
		)
	}
	return f.uploadCreateOnlyParallel(
		ctx, source, remotePath, expectedSize, plan, progress, seed, releaseSeed, desiredConnections, manifest,
	)
}

func (f *FileSystem) UploadFileParallelResumable(
	ctx context.Context,
	localPath string,
	remotePath string,
	expectedSize int64,
	overwrite bool,
	progress func(int64) error,
	resumeKey string,
) error {
	manifest := ResumeManifestFor(resumeKey, remotePath, expectedSize)
	return f.uploadFileParallelWithResume(
		ctx, localPath, remotePath, expectedSize, overwrite, false, progress, manifest, nil, nil,
		f.parallelDesiredConnections(),
	)
}

func (f *FileSystem) DropResumeState(resumeKey string) string {
	return DropResumeManifest(resumeKey)
}

func (f *FileSystem) DownloadFileParallel(
	ctx context.Context,
	remotePath string,
	localPath string,
	expectedSize int64,
	overwrite bool,
	progress func(int64) error,
) error {
	return f.downloadFileParallelWithSeed(ctx, remotePath, localPath, expectedSize, overwrite, progress, nil, nil, f.parallelDesiredConnections())
}

func (f *FileSystem) downloadFileParallelWithSeed(
	ctx context.Context,
	remotePath string,
	localPath string,
	expectedSize int64,
	overwrite bool,
	progress func(int64) error,
	seed *subsystem,
	releaseSeed func(bool),
	desiredConnections int,
) error {
	if plan, ok := planSFTPParallelTransfer(expectedSize); ok && f.parallelReady() {

		return f.downloadWith(ctx, remotePath, localPath, expectedSize, overwrite, progress, plan, seed, releaseSeed, desiredConnections)
	}
	return f.downloadFile(ctx, remotePath, localPath, expectedSize, overwrite, progress)
}

func (f *FileSystem) uploadFile(
	ctx context.Context,
	localPath string,
	remotePath string,
	expectedSize int64,
	overwrite bool,
	progress func(int64) error,
) error {
	if err := transferReady(ctx, f, expectedSize, progress); err != nil {
		return err
	}
	source, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local upload source: %w", err)
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return fmt.Errorf("inspect local upload source: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() != expectedSize {
		return errors.New("local upload source changed or is not a regular file")
	}

	if !overwrite {
		return f.uploadCreateOnly(ctx, source, remotePath, expectedSize, progress)
	}
	return f.uploadReplacing(ctx, source, remotePath, expectedSize, progress)
}

func (f *FileSystem) DownloadFileContext(
	ctx context.Context,
	remotePath string,
	localPath string,
	expectedSize int64,
	overwrite bool,
	progress func(int64) error,
) error {
	return f.downloadFile(ctx, remotePath, localPath, expectedSize, overwrite, progress)
}

func (f *FileSystem) downloadFile(
	ctx context.Context,
	remotePath string,
	localPath string,
	expectedSize int64,
	overwrite bool,
	progress func(int64) error,
) error {
	return f.downloadWith(ctx, remotePath, localPath, expectedSize, overwrite, progress, sftpParallelPlan{}, nil, nil, 1)
}

func (f *FileSystem) downloadFileReplacingParallel(
	ctx context.Context,
	remotePath string,
	localPath string,
	expectedSize int64,
	plan sftpParallelPlan,
	progress func(int64) error,
) error {
	return f.downloadWith(ctx, remotePath, localPath, expectedSize, true, progress, plan, nil, nil, f.parallelDesiredConnections())
}

func (f *FileSystem) downloadWith(
	ctx context.Context,
	remotePath string,
	localPath string,
	expectedSize int64,
	overwrite bool,
	progress func(int64) error,
	plan sftpParallelPlan,
	seed *subsystem,
	releaseSeed func(bool),
	desiredConnections int,
) error {
	if err := transferReady(ctx, f, expectedSize, progress); err != nil {
		return err
	}
	remote, err := f.client.Open(remotePath)
	if err != nil {
		return fmt.Errorf("open remote download source: %w", err)
	}
	defer remote.Close()
	info, err := remote.Stat()
	if err != nil {
		return fmt.Errorf("inspect remote download source: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() != expectedSize {
		return errors.New("remote download source changed or is not a regular file")
	}
	if !overwrite {
		if _, err := os.Lstat(localPath); err == nil {
			return fs.ErrExist
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("inspect local download destination: %w", err)
		}
	}

	temporary, err := os.CreateTemp(filepath.Dir(localPath), ".qterm-download-*.part")
	if err != nil {
		return fmt.Errorf("create local download temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()

	stopCancellation := context.AfterFunc(ctx, func() {
		_ = remote.Close()
		_ = temporary.Close()
	})
	var transferred int64
	var copyErr error
	if plan.width > 0 {

		transferred, copyErr = expectedSize, f.parallelDownload(ctx, remotePath, expectedSize, plan, temporary, progress, seed, releaseSeed, desiredConnections)
	} else {
		transferred, copyErr = copyTransfer(ctx, temporary, remote, progress)
	}
	stopCancellation()
	if copyErr != nil {
		return fmt.Errorf("download remote file: %w", copyErr)
	}
	if transferred != expectedSize {
		return fmt.Errorf("downloaded size = %d, want %d", transferred, expectedSize)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync local download temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close local download temporary file: %w", err)
	}

	if overwrite {
		if err := os.Rename(temporaryPath, localPath); err != nil {
			return fmt.Errorf("replace local download destination: %w", err)
		}
		committed = true
		return nil
	}
	if err := os.Link(temporaryPath, localPath); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fs.ErrExist
		}
		return fmt.Errorf("commit local download destination: %w", err)
	}
	if err := os.Remove(temporaryPath); err != nil {
		return fmt.Errorf("remove local download temporary file: %w", err)
	}
	committed = true
	return nil
}

func (f *FileSystem) uploadCreateOnly(
	ctx context.Context,
	source *os.File,
	remotePath string,
	expectedSize int64,
	progress func(int64) error,
) error {
	exists, err := f.remotePathExists(remotePath)
	if err != nil {
		return fmt.Errorf("inspect remote upload destination: %w", err)
	}
	if exists {
		return fs.ErrExist
	}
	target, err := f.client.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		exists, inspectErr := f.remotePathExists(remotePath)
		if inspectErr == nil && exists {
			return fs.ErrExist
		}
		return fmt.Errorf("create remote upload destination: %w", err)
	}
	committed := false
	defer func() {
		_ = target.Close()
		if !committed {
			_ = f.client.Remove(remotePath)
		}
	}()
	if err := copyUpload(ctx, target, source, expectedSize, progress); err != nil {
		return err
	}
	committed = true
	return nil
}

func (f *FileSystem) uploadReplacing(
	ctx context.Context,
	source *os.File,
	remotePath string,
	expectedSize int64,
	progress func(int64) error,
) error {
	if info, err := f.client.Lstat(remotePath); err == nil {
		if info.IsDir() {
			return errors.New("remote upload destination is a directory")
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect remote upload destination: %w", err)
	}
	temporaryPath, target, err := f.createRemoteTemporary(remotePath)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		_ = target.Close()
		if !committed {
			_ = f.client.Remove(temporaryPath)
		}
	}()
	if err := copyUpload(ctx, target, source, expectedSize, progress); err != nil {
		return err
	}

	if err := f.commitReplacing(temporaryPath, remotePath); err != nil {
		return err
	}
	committed = true
	return nil
}

func (f *FileSystem) openUploadTemporary(
	remotePath string,
	resume *SFTPResumeManifest,
) (temporaryPath string, target io.WriteCloser, restarted bool, err error) {
	if resume == nil || resume.TempPath == "" {
		path, writer, createErr := f.createRemoteTemporary(remotePath)
		return path, writer, false, createErr
	}
	if verifyErr := verifyResumeTempFile(f, resume.TempPath, resume.MaxWrittenOffset()); verifyErr == nil {
		writer, openErr := f.client.OpenFile(resume.TempPath, os.O_WRONLY)
		if openErr != nil {
			return "", nil, false, fmt.Errorf("open resume remote upload temporary file: %w", openErr)
		}
		return resume.TempPath, writer, false, nil
	}

	writer, createErr := f.client.OpenFile(resume.TempPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if createErr != nil {
		return "", nil, false, fmt.Errorf("create resume remote upload temporary file: %w", createErr)
	}
	return resume.TempPath, writer, true, nil
}

func (f *FileSystem) createRemoteTemporary(remotePath string) (string, io.WriteCloser, error) {
	directory := path.Dir(remotePath)
	base := path.Base(remotePath)
	for attempt := 0; attempt < 4; attempt++ {
		suffix, err := randomTransferSuffix()
		if err != nil {
			return "", nil, fmt.Errorf("generate remote upload temporary name: %w", err)
		}
		temporaryPath := path.Join(directory, ".qterm-"+base+"-"+suffix+".part")
		target, err := f.client.OpenFile(temporaryPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
		if err == nil {
			return temporaryPath, target, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", nil, fmt.Errorf("create remote upload temporary file: %w", err)
		}
	}
	return "", nil, errors.New("create remote upload temporary file: name collision")
}

func (f *FileSystem) readRemoteChunkDigest(remotePath string, offset, length int64) (string, error) {
	if length <= 0 {
		return "", nil
	}
	file, err := f.client.Open(remotePath)
	if err != nil {
		return "", fmt.Errorf("open resume temporary for verification: %w", err)
	}
	defer file.Close()
	buffer := make([]byte, length)
	read, readErr := file.ReadAt(buffer, offset)
	if readErr != nil && read != len(buffer) {
		return "", fmt.Errorf("read resume temporary chunk: %w", readErr)
	}
	return resumeChunkDigest(buffer[:read]), nil
}

func (f *FileSystem) remotePathExists(remotePath string) (bool, error) {
	_, err := f.client.Lstat(remotePath)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func (f *FileSystem) commitCreateOnly(temporaryPath string, finalPath string) error {
	if exists, err := f.remotePathExists(finalPath); err != nil {
		return fmt.Errorf("inspect remote transfer destination: %w", err)
	} else if exists {
		return fs.ErrExist
	}
	if err := f.client.Rename(temporaryPath, finalPath); err != nil {
		if exists, inspectErr := f.remotePathExists(finalPath); inspectErr == nil && exists {
			return fs.ErrExist
		}
		return fmt.Errorf("commit remote transfer destination: %w", err)
	}
	return nil
}

func copyUpload(
	ctx context.Context,
	target io.WriteCloser,
	source io.Reader,
	expectedSize int64,
	progress func(int64) error,
) error {
	stopCancellation := context.AfterFunc(ctx, func() { _ = target.Close() })
	transferred, err := copyTransfer(ctx, target, source, progress)
	stopCancellation()
	if err != nil {
		return fmt.Errorf("upload local file: %w", err)
	}
	if transferred != expectedSize {
		return fmt.Errorf("uploaded size = %d, want %d", transferred, expectedSize)
	}
	if err := target.Close(); err != nil {
		return fmt.Errorf("close remote upload file: %w", err)
	}
	return nil
}

func copyTransfer(
	ctx context.Context,
	destination io.Writer,
	source io.Reader,
	progress func(int64) error,
) (int64, error) {
	buffer := make([]byte, transferBufferSize)
	var transferred int64
	for {
		if err := ctx.Err(); err != nil {
			return transferred, err
		}
		read, readErr := source.Read(buffer)
		if read > 0 {
			written, writeErr := destination.Write(buffer[:read])
			transferred += int64(written)
			if writeErr != nil {
				return transferred, writeErr
			}
			if written != read {
				return transferred, io.ErrShortWrite
			}
			if err := progress(transferred); err != nil {
				return transferred, err
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return transferred, nil
			}
			return transferred, readErr
		}
	}
}

func transferReady(
	ctx context.Context,
	fileSystem *FileSystem,
	expectedSize int64,
	progress func(int64) error,
) error {
	if ctx == nil {
		return errors.New("SFTP transfer context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if fileSystem == nil || fileSystem.client == nil {
		return ErrFileSystemNotReady
	}
	if expectedSize < 0 || progress == nil {
		return errors.New("invalid SFTP transfer parameters")
	}
	return nil
}

func randomTransferSuffix() (string, error) {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return strings.ToLower(hex.EncodeToString(value[:])), nil
}
