package sftp

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"

	"changeme/internal/errno"
	"changeme/internal/model"
	sessionapp "changeme/internal/session"
)

const (
	archiveLocalDirectoryPrefix = "qterm-sftp-archive-"

	archiveRemoteTemporaryPrefix = ".qterm-upload-"

	archiveRemoteExtension = ".tar.gz"

	archiveFallbackReasonRemoteTar = "remote_tar_unavailable"
)

type remoteTarProber interface {
	ProbeRemoteTar(ctx context.Context, sessionID string) error
}

type remoteArchiveExtractor interface {
	ExtractRemoteArchive(ctx context.Context, sessionID, archivePath, targetDirectory string) error
}

type ArchiveUploadSpec struct {
	TargetDirectory string

	LocalTemporaryDirectory string
}

type localArchive struct {
	Path string

	Directory string

	Size int64

	Entries int
}

type archiveUploadOutcome struct {
	Transfer       model.SFTPTransfer
	FellBack       bool
	FallbackReason string
}

type archiveUploadPlan struct {
	Request        SFTPTransferRequest
	Archive        localArchive
	FellBack       bool
	FallbackReason string
}

func (s *Service) enqueueArchivedDirectoryUpload(
	ctx context.Context,
	sessionID string,
	localPath string,
	remoteDirectory string,
) (archiveUploadOutcome, error) {
	outcome := archiveUploadOutcome{}
	manager, transfers, _, err := s.transferDependencies()
	if err != nil {
		return outcome, err
	}

	prober, ok := any(manager).(remoteTarProber)
	if !ok {
		return outcome, errno.New(errno.CodeInvalidArg, "sftp.operation_unsupported", "")
	}
	plan, err := prepareArchiveUpload(ctx, prober, sessionID, localPath, remoteDirectory)
	if err != nil {
		return outcome, err
	}
	if plan.FellBack {
		outcome.FellBack = true
		outcome.FallbackReason = plan.FallbackReason
		return outcome, nil
	}
	transfer, enqueueErr := transfers.Enqueue(ctx, plan.Request)
	if enqueueErr != nil {

		cleanupLocalArchive(plan.Archive)
		return outcome, TransferServiceError(enqueueErr)
	}
	outcome.Transfer = transfer
	return outcome, nil
}

func prepareArchiveUpload(
	ctx context.Context,
	prober remoteTarProber,
	sessionID string,
	localPath string,
	remoteDirectory string,
) (archiveUploadPlan, error) {
	plan := archiveUploadPlan{}

	if err := prober.ProbeRemoteTar(ctx, sessionID); err != nil {
		if isRemoteTarUnavailable(err) {
			plan.FellBack = true
			plan.FallbackReason = archiveFallbackReasonRemoteTar
			return plan, nil
		}
		return archiveUploadPlan{}, err
	}
	entries, skipped, err := inspectArchiveUploadDirectory(ctx, localPath, remoteDirectory)
	if err != nil {
		return archiveUploadPlan{}, errno.Wrap(errno.CodeInvalidArg, "sftp.upload_directory_invalid", err)
	}
	if skipped > 0 {

		slog.Info("sftp archive upload skipped unsupported entries",
			"event", "sftp.archive.skipped",
			"session", sessionID,
			"directory", remoteDirectory,
			"skipped", skipped)
	}
	archive, err := createDirectoryArchive(ctx, localPath, remoteDirectory, entries)
	if err != nil {
		return archiveUploadPlan{}, err
	}
	remoteArchivePath, err := archiveRemoteTemporaryPath(remoteDirectory)
	if err != nil {
		cleanupLocalArchive(archive)
		return archiveUploadPlan{}, errno.Wrap(errno.CodeUnknown, "sftp.upload_directory_failed", err)
	}
	batchID, err := randomTransferID()
	if err != nil {
		cleanupLocalArchive(archive)
		return archiveUploadPlan{}, errno.Wrap(errno.CodeUnknown, "sftp.upload_directory_failed", err)
	}
	plan.Archive = archive
	plan.Request = SFTPTransferRequest{
		SessionID:  sessionID,
		BatchID:    batchID,
		Direction:  model.SFTPTransferUpload,
		LocalPath:  archive.Path,
		LocalName:  archiveDisplayName(localPath, archive.Path),
		RemotePath: remoteArchivePath,
		Total:      archive.Size,
		Archive: &ArchiveUploadSpec{
			TargetDirectory:         remoteDirectory,
			LocalTemporaryDirectory: archive.Directory,
		},
	}
	return plan, nil
}

func isRemoteTarUnavailable(err error) bool {
	return errors.Is(err, sessionapp.ErrRemoteTarUnavailable)
}

func inspectArchiveUploadDirectory(
	ctx context.Context,
	localPath string,
	remoteDirectory string,
) ([]uploadDirectoryEntry, int, error) {
	if !filepath.IsAbs(localPath) {
		return nil, 0, errors.New("directory must be absolute")
	}
	localPath = filepath.Clean(localPath)
	rootInfo, err := os.Lstat(localPath)
	if err != nil {
		return nil, 0, err
	}
	if !rootInfo.IsDir() || filepath.Dir(localPath) == localPath {
		return nil, 0, errors.New("invalid upload directory")
	}
	entries := make([]uploadDirectoryEntry, 0, 32)
	skipped := 0
	err = filepath.WalkDir(localPath, func(source string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(entries) >= remoteDirectoryEntryLimit {
			return errors.New("directory entry limit exceeded")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {

			skipped++
			return nil
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
		entries = append(entries, uploadDirectoryEntry{
			localPath:  source,
			remotePath: destination,
			size:       info.Size(),
			directory:  info.IsDir(),
		})
		return nil
	})
	if err != nil {
		return nil, skipped, err
	}
	return entries, skipped, nil
}

func createDirectoryArchive(
	ctx context.Context,
	localPath string,
	remoteDirectory string,
	entries []uploadDirectoryEntry,
) (localArchive, error) {
	directory, err := os.MkdirTemp("", archiveLocalDirectoryPrefix)
	if err != nil {
		return localArchive{}, errno.Wrap(errno.CodeStoreFailed, "sftp.upload_directory_failed", err)
	}
	filename, err := randomArchiveFilename()
	if err != nil {

		_ = os.Remove(directory)
		return localArchive{}, errno.Wrap(errno.CodeStoreFailed, "sftp.upload_directory_failed", err)
	}
	archive := localArchive{Path: filepath.Join(directory, filename), Directory: directory}

	file, err := os.OpenFile(archive.Path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		cleanupLocalArchive(archive)
		return localArchive{}, errno.Wrap(errno.CodeStoreFailed, "sftp.upload_directory_failed", err)
	}
	writeErr := writeDirectoryArchive(ctx, file, remoteDirectory, entries)
	closeErr := file.Close()
	if writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {

		cleanupLocalArchive(archive)
		if errors.Is(writeErr, context.Canceled) || errors.Is(writeErr, context.DeadlineExceeded) {
			return localArchive{}, writeErr
		}
		return localArchive{}, errno.Wrap(errno.CodeStoreFailed, "sftp.upload_directory_failed", writeErr)
	}
	info, err := os.Stat(archive.Path)
	if err != nil {
		cleanupLocalArchive(archive)
		return localArchive{}, errno.Wrap(errno.CodeStoreFailed, "sftp.upload_directory_failed", err)
	}
	archive.Size = info.Size()
	archive.Entries = len(entries)
	return archive, nil
}

func writeDirectoryArchive(
	ctx context.Context,
	writer io.Writer,
	remoteDirectory string,
	entries []uploadDirectoryEntry,
) error {
	gzipWriter := gzip.NewWriter(writer)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := writeDirectoryArchiveEntry(tarWriter, remoteDirectory, entry); err != nil {
			return err
		}
	}

	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		return err
	}
	return gzipWriter.Close()
}

func writeDirectoryArchiveEntry(tarWriter *tar.Writer, remoteDirectory string, entry uploadDirectoryEntry) error {
	name := archiveEntryName(remoteDirectory, entry.remotePath)
	if name == "" || name == "." {
		return errors.New("invalid archive entry name")
	}

	info, err := os.Lstat(entry.localPath)
	if err != nil {
		return err
	}
	header := &tar.Header{
		Name:    name,
		Mode:    int64(info.Mode().Perm()),
		ModTime: info.ModTime(),

		Format: tar.FormatPAX,
	}
	if entry.directory {
		if !info.IsDir() {
			return fmt.Errorf("archive entry changed type during packing")
		}

		header.Name = name + "/"
		header.Typeflag = tar.TypeDir
		header.Size = 0
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("archive entry changed type during packing")
	}
	header.Typeflag = tar.TypeReg
	header.Size = info.Size()
	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}
	file, err := os.Open(entry.localPath)
	if err != nil {
		return err
	}

	copied, copyErr := io.CopyN(tarWriter, file, info.Size())
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if copied != info.Size() {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func archiveEntryName(remoteDirectory string, remotePath string) string {
	name := strings.TrimPrefix(remotePath, strings.TrimSuffix(remoteDirectory, "/"))
	return strings.TrimPrefix(name, "/")
}

func archiveRemoteTemporaryPath(remoteDirectory string) (string, error) {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	name := archiveRemoteTemporaryPrefix + hex.EncodeToString(value[:]) + archiveRemoteExtension
	return model.NormalizeRemotePath(path.Join(remoteDirectory, name))
}

func randomArchiveFilename() (string, error) {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return "upload-" + hex.EncodeToString(value[:]) + archiveRemoteExtension, nil
}

func archiveDisplayName(localPath string, archivePath string) string {
	name := filepath.Base(filepath.Clean(localPath)) + archiveRemoteExtension
	if name == "" || strings.ContainsAny(name, "/\\\x00\r\n") || name == "." || name == ".." {
		return filepath.Base(archivePath)
	}
	return name
}

func cleanupLocalArchive(archive localArchive) {
	if archive.Path == "" && archive.Directory == "" {
		return
	}
	if archive.Path != "" {
		if err := os.Remove(archive.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			slog.Warn("sftp archive upload could not remove the local archive",
				"event", "sftp.archive.cleanup", "error", err)
		}
	}
	if archive.Directory != "" {

		if err := os.Remove(archive.Directory); err != nil && !errors.Is(err, fs.ErrNotExist) {
			slog.Warn("sftp archive upload could not remove the local temporary directory",
				"event", "sftp.archive.cleanup", "error", err)
		}
	}
}
