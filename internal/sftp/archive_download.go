package sftp

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"changeme/internal/errno"
	"changeme/internal/model"
	sessionapp "changeme/internal/session"
)

const (
	archiveDownloadRemotePrefix = ".qterm-download-"

	archiveDownloadLocalPrefix = "qterm-sftp-download-"

	archiveDownloadEntryLimit = 200_000

	archiveDownloadCleanupTimeout = 30 * time.Second
)

type remoteArchiveCreator interface {
	CreateRemoteArchive(ctx context.Context, sessionID, archivePath, sourceDirectory string) error
}

type archiveDownloadDeps struct {
	probe  func(ctx context.Context, sessionID string) error
	create func(ctx context.Context, sessionID, archivePath, sourceDirectory string) error
	remove func(ctx context.Context, sessionID, archivePath string) error

	stat func(ctx context.Context, sessionID, archivePath string) (int64, error)

	download func(
		ctx context.Context,
		sessionID, archivePath, localPath string,
		progress func(int64) error,
	) error
}

type archiveDownloadHooks struct {
	OnPack func()

	OnPacked func(size int64)

	OnExtract func()

	Progress func(transferred int64) error
}

func (s *Service) DownloadRemoteDirectoryArchive(
	ctx context.Context,
	sessionID string,
	remoteDirectory string,
	localDirectory string,
) error {
	manager, _, _, err := s.transferDependencies()
	if err != nil {
		return err
	}
	if err := localContextReady(ctx); err != nil {
		return err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errno.New(errno.CodeInvalidArg, "session.not_found", "")
	}
	remote, err := model.NormalizeRemoteDirectory(remoteDirectory)
	if err != nil {
		return errno.New(errno.CodeInvalidArg, "sftp.remote_path_invalid", "")
	}

	if remote == "/" {
		return errno.New(errno.CodeInvalidArg, "sftp.archive.source_invalid", "")
	}
	local, err := normalizeDialogPath(localDirectory)
	if err != nil {
		return err
	}
	info, statErr := os.Stat(local)
	if statErr != nil || !info.IsDir() {
		return errno.New(errno.CodeNotFound, "sftp.local_directory_not_found", "")
	}
	deps, err := archiveDownloadDepsFor(manager)
	if err != nil {
		return err
	}
	return downloadDirectoryArchive(ctx, deps, sessionID, remote, local)
}

func archiveDownloadDepsFor(source any) (archiveDownloadDeps, error) {
	prober, ok := source.(remoteTarProber)
	if !ok {
		return archiveDownloadDeps{}, errno.New(errno.CodeInvalidArg, "sftp.operation_unsupported", "")
	}
	creator, ok := source.(remoteArchiveCreator)
	if !ok {
		return archiveDownloadDeps{}, errno.New(errno.CodeInvalidArg, "sftp.operation_unsupported", "")
	}
	if _, ok := source.(archiveTransferFileSystemSource); !ok {
		return archiveDownloadDeps{}, errno.New(errno.CodeInvalidArg, "sftp.operation_unsupported", "")
	}
	remover, hasRemover := source.(remoteTreeRemover)
	return archiveDownloadDeps{
		probe:  prober.ProbeRemoteTar,
		create: creator.CreateRemoteArchive,
		remove: func(ctx context.Context, sessionID, archivePath string) error {
			if !hasRemover {
				return nil
			}
			return remover.RemoveRemoteTree(ctx, sessionID, archivePath)
		},
		stat: func(ctx context.Context, sessionID, archivePath string) (int64, error) {
			return statArchiveFile(source, ctx, sessionID, archivePath)
		},
		download: func(
			ctx context.Context,
			sessionID, archivePath, localPath string,
			progress func(int64) error,
		) error {
			return downloadArchiveFile(source, ctx, sessionID, archivePath, localPath, progress)
		},
	}, nil
}

type archiveTransferFileSystemSource interface {
	TransferFileSystem(ctx context.Context, sessionID string) (sessionapp.SessionTransferFileSystem, error)
}

func statArchiveFile(
	source any,
	ctx context.Context,
	sessionID string,
	remotePath string,
) (int64, error) {
	provider := source.(archiveTransferFileSystemSource)
	fileSystem, err := provider.TransferFileSystem(ctx, sessionID)
	if err != nil {
		return 0, err
	}
	defer func() { _ = fileSystem.Close() }()
	info, err := fileSystem.StatContext(ctx, remotePath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func downloadArchiveFile(
	source any,
	ctx context.Context,
	sessionID string,
	remotePath string,
	localPath string,
	progress func(int64) error,
) error {
	provider := source.(archiveTransferFileSystemSource)
	fileSystem, err := provider.TransferFileSystem(ctx, sessionID)
	if err != nil {
		return err
	}
	defer func() { _ = fileSystem.Close() }()
	info, err := fileSystem.StatContext(ctx, remotePath)
	if err != nil {
		return err
	}
	return fileSystem.DownloadFileContext(ctx, remotePath, localPath, info.Size(), true, progress)
}

func downloadDirectoryArchive(
	ctx context.Context,
	deps archiveDownloadDeps,
	sessionID string,
	remoteDirectory string,
	localDirectory string,
) error {
	return downloadDirectoryArchiveWith(ctx, deps, sessionID, remoteDirectory, localDirectory, archiveDownloadHooks{})
}

func downloadDirectoryArchiveWith(
	ctx context.Context,
	deps archiveDownloadDeps,
	sessionID string,
	remoteDirectory string,
	localDirectory string,
	hooks archiveDownloadHooks,
) error {
	if err := deps.probe(ctx, sessionID); err != nil {
		if isRemoteTarUnavailable(err) {
			return errno.New(errno.CodeInvalidArg, "sftp.archive.tar_unavailable", "")
		}
		return ServiceError(err)
	}
	archivePath, err := archiveDownloadRemotePath()
	if err != nil {
		return errno.Wrap(errno.CodeUnknown, "sftp.archive.download_failed", err)
	}
	if hooks.OnPack != nil {
		hooks.OnPack()
	}
	if err := deps.create(ctx, sessionID, archivePath, remoteDirectory); err != nil {
		if isRemoteTarUnavailable(err) {
			return errno.New(errno.CodeInvalidArg, "sftp.archive.tar_unavailable", "")
		}
		return ServiceError(err)
	}

	defer func() {
		cleanupCtx, cancel := context.WithTimeout(
			context.WithoutCancel(context.Background()),
			archiveDownloadCleanupTimeout,
		)
		defer cancel()
		if removeErr := deps.remove(cleanupCtx, sessionID, archivePath); removeErr != nil {
			slog.Warn("sftp archive download cleanup failed",
				"event", "sftp.archive.download_cleanup_failed",
				"session", sessionID)
		}
	}()

	archive, err := os.CreateTemp("", archiveDownloadLocalPrefix+".tar.gz")
	if err != nil {
		return errno.Wrap(errno.CodeStoreFailed, "sftp.archive.download_failed", err)
	}
	localArchivePath := archive.Name()
	_ = archive.Close()

	defer func() { _ = os.Remove(localArchivePath) }()

	if hooks.OnPacked != nil {
		size := int64(0)
		if deps.stat != nil {
			if measured, statErr := deps.stat(ctx, sessionID, archivePath); statErr == nil {
				size = measured
			}
		}
		hooks.OnPacked(size)
	}
	if err := deps.download(ctx, sessionID, archivePath, localArchivePath, hooks.Progress); err != nil {
		return ServiceError(err)
	}
	if hooks.OnExtract != nil {
		hooks.OnExtract()
	}
	return extractTarGzToDirectoryContext(ctx, localArchivePath, localDirectory)
}

func archiveDownloadRemotePath() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}

	name := archiveDownloadRemotePrefix + hex.EncodeToString(value[:]) + ".tar.gz"
	return model.NormalizeRemotePath(path.Join("/tmp", name))
}

func extractTarGzToDirectory(archivePath, targetDirectory string) error {
	return extractTarGzToDirectoryContext(context.Background(), archivePath, targetDirectory)
}

func extractTarGzToDirectoryContext(ctx context.Context, archivePath, targetDirectory string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := os.Open(archivePath)
	if err != nil {
		return errno.Wrap(errno.CodeStoreFailed, "sftp.archive.download_failed", err)
	}
	defer func() { _ = file.Close() }()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return errno.New(errno.CodeInvalidArg, "sftp.archive.invalid", "")
	}
	defer func() { _ = gzipReader.Close() }()
	reader := tar.NewReader(gzipReader)
	entries := 0
	for {
		header, nextErr := reader.Next()
		if errors.Is(nextErr, io.EOF) {
			return nil
		}
		if nextErr != nil {
			return errno.New(errno.CodeInvalidArg, "sftp.archive.invalid", "")
		}
		entries++

		if entries%256 == 0 {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
		}
		if entries > archiveDownloadEntryLimit {
			return errno.New(errno.CodeInvalidArg, "sftp.archive.too_large", "")
		}
		if err := extractTarEntry(reader, header, targetDirectory); err != nil {
			return err
		}
	}
}

func extractTarEntry(reader io.Reader, header *tar.Header, targetDirectory string) error {
	name := strings.TrimSpace(header.Name)
	if name == "" {
		return nil
	}
	cleaned := filepath.Clean(filepath.FromSlash(name))

	if cleaned == "." {
		return nil
	}

	if filepath.IsAbs(cleaned) || cleaned == ".." ||
		strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return errno.New(errno.CodeInvalidArg, "sftp.archive.unsafe_entry", "")
	}
	destination := filepath.Join(targetDirectory, cleaned)

	relative, err := filepath.Rel(targetDirectory, destination)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return errno.New(errno.CodeInvalidArg, "sftp.archive.unsafe_entry", "")
	}
	switch header.Typeflag {
	case tar.TypeDir:
		if mkdirErr := os.MkdirAll(destination, 0o755); mkdirErr != nil {
			return errno.Wrap(errno.CodeStoreFailed, "sftp.archive.extract_failed", mkdirErr)
		}
	case tar.TypeReg, tar.TypeRegA:
		if mkdirErr := os.MkdirAll(filepath.Dir(destination), 0o755); mkdirErr != nil {
			return errno.Wrap(errno.CodeStoreFailed, "sftp.archive.extract_failed", mkdirErr)
		}
		file, createErr := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if createErr != nil {
			return errno.Wrap(errno.CodeStoreFailed, "sftp.archive.extract_failed", createErr)
		}
		_, copyErr := io.Copy(file, reader)
		closeErr := file.Close()
		if copyErr != nil {
			return errno.Wrap(errno.CodeStoreFailed, "sftp.archive.extract_failed", copyErr)
		}
		if closeErr != nil {
			return errno.Wrap(errno.CodeStoreFailed, "sftp.archive.extract_failed", closeErr)
		}
	case tar.TypeSymlink, tar.TypeLink:

		slog.Info("sftp archive download skipped link entry", "event", "sftp.archive.download_link_skipped")
	default:

	}
	return nil
}
