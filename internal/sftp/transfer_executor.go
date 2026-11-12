package sftp

import (
	sessionapp "changeme/internal/session"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"time"

	"changeme/internal/model"
)

type sessionTransferFileSystemSource interface {
	TransferFileSystem(context.Context, string) (sessionapp.SessionTransferFileSystem, error)
}

type sessionTransferFileSystemSizer interface {
	TransferFileSystemForSize(
		context.Context,
		string,
		int64,
	) (sessionapp.SessionTransferFileSystem, error)
}

type remoteStreamableFileSystem interface {
	OpenFile(ctx context.Context, path string) (io.ReadCloser, error)
	CreateTransferFile(ctx context.Context, finalPath string) (string, io.WriteCloser, error)
	CommitTransferFile(ctx context.Context, temporaryPath, finalPath string, overwrite bool) error
	RemoveTransferFile(ctx context.Context, path string) error
	Close() error
}

type TransferConnectionReporter interface {
	ReportTransferConnections(connections int, desired int, blockedReason string)
}

type SessionSFTPTransferExecutor struct {
	sessions sessionTransferFileSystemSource
	reporter TransferConnectionReporter
}

var _ SFTPTransferExecutor = (*SessionSFTPTransferExecutor)(nil)

func NewSessionSFTPTransferExecutor(sessions *sessionapp.SessionManager) *SessionSFTPTransferExecutor {
	return &SessionSFTPTransferExecutor{sessions: sessions}
}

func (e *SessionSFTPTransferExecutor) SetConnectionReporter(reporter TransferConnectionReporter) {
	if e == nil {
		return
	}
	e.reporter = reporter
}

func (e *SessionSFTPTransferExecutor) Execute(
	ctx context.Context,
	request SFTPTransferRequest,
	overwrite bool,
	progress func(transferred int64) error,
) (transferErr error) {

	if request.Archive != nil && request.LocalPath != "" {
		defer cleanupLocalArchive(localArchive{
			Path:      request.LocalPath,
			Directory: request.Archive.LocalTemporaryDirectory,
		})
	}
	if ctx == nil {
		return errors.New("SFTP transfer context is nil")
	}
	if e == nil || e.sessions == nil {
		return ErrSFTPTransferExecutorNotReady
	}

	if request.Archive != nil {
		if _, statErr := os.Stat(request.LocalPath); statErr != nil {
			if errors.Is(statErr, fs.ErrNotExist) {

				return fmt.Errorf("archived upload local archive is gone: %w", fs.ErrNotExist)
			}
			return fmt.Errorf("archived upload local archive is unreadable: %w", statErr)
		}
	}

	if request.ArchiveDownload != nil {
		return e.transferArchiveDownload(ctx, request, progress)
	}

	if request.Direction == model.SFTPTransferUpload && request.SourceSessionID != "" {
		fileSize := request.Total / 2
		sourceFS, sErr := e.sessions.TransferFileSystem(ctx, request.SourceSessionID)
		if sErr != nil {
			return fmt.Errorf("open source transfer file system: %w", sErr)
		}
		destFS, dErr := e.sessions.TransferFileSystem(ctx, request.SessionID)
		if dErr != nil {
			_ = sourceFS.Close()
			return fmt.Errorf("open dest transfer file system: %w", dErr)
		}

		stopCancel := context.AfterFunc(ctx, func() {
			_ = sourceFS.Close()
			_ = destFS.Close()
		})
		defer stopCancel()

		srcStream, srcOK := sourceFS.(remoteStreamableFileSystem)
		dstStream, dstOK := destFS.(remoteStreamableFileSystem)
		if srcOK && dstOK {
			sourcePath := request.SourceRemotePath
			if sourcePath == "" {
				_ = sourceFS.Close()
				_ = destFS.Close()
				return fmt.Errorf("remote source path is missing")
			}

			if _, statErr := destFS.StatContext(ctx, request.RemotePath); statErr == nil && !overwrite {
				_ = sourceFS.Close()
				_ = destFS.Close()
				return ErrSFTPTransferDestinationExists
			}

			streamScale := int64(1)
			if fileSize > 0 {
				streamScale = request.Total / fileSize
			}
			if streamScale < 1 {
				streamScale = 1
			}
			streamProgress := func(transferred int64) error {
				return progress(transferred * streamScale)
			}
			temporaryPath, streamErr := copyRemoteStream(
				ctx,
				dstStream,
				srcStream,
				sourcePath,
				request.RemotePath,
				fileSize,
				overwrite,
				streamProgress,
			)

			_ = destFS.Close()
			_ = sourceFS.Close()
			if streamErr != nil {

				e.removeRemoteTemporary(request.SessionID, temporaryPath)
				return streamErr
			}
			return nil
		}

		tmpFile, tmpErr := os.CreateTemp("", "qterm-remote-transfer-*")
		if tmpErr != nil {
			_ = sourceFS.Close()
			_ = destFS.Close()
			return fmt.Errorf("create temp file: %w", tmpErr)
		}
		tmpPath := tmpFile.Name()
		_ = tmpFile.Close()
		defer os.Remove(tmpPath)

		dlProgress := func(transferred int64) error { return progress(transferred) }
		transferErr = sourceFS.DownloadFileContext(ctx, request.SourceRemotePath, tmpPath, fileSize, true, dlProgress)
		_ = sourceFS.Close()
		if transferErr != nil {
			_ = destFS.Close()
			return transferErr
		}

		ulProgress := func(transferred int64) error { return progress(fileSize + transferred) }
		transferErr = destFS.UploadFileContext(ctx, tmpPath, request.RemotePath, fileSize, overwrite, ulProgress)
		_ = destFS.Close()
		if errors.Is(transferErr, fs.ErrExist) {
			return ErrSFTPTransferDestinationExists
		}
		if transferErr != nil {
			return transferErr
		}
		return nil
	}

	fileSystem, err := e.openTransferFileSystem(ctx, request)
	if err != nil {
		return fmt.Errorf("open transfer file system: %w", err)
	}

	defer func() { _ = fileSystem.Close() }()

	parallel, parallelOK := fileSystem.(sessionapp.ParallelCapableFileSystem)
	if !parallelOK {
		return e.finishArchiveUpload(ctx, request, finishTransferDirection(ctx, fileSystem, request, overwrite, progress))
	}

	if status, ok := fileSystem.(sessionapp.ConnectionStatusFileSystem); ok {
		expandedAtStart, degradedAtStart := status.TransferExpansionCounters()
		transferStartedAt := time.Now()
		defer func() {
			active, desired, reason := status.TransferConnectionStatus()
			expanded, degraded := status.TransferExpansionCounters()
			expanded -= expandedAtStart
			degraded -= degradedAtStart
			if expanded < 0 {
				expanded = 0
			}
			if degraded < 0 {
				degraded = 0
			}
			if e.reporter != nil {
				e.reporter.ReportTransferConnections(active, desired, reason)
			}
			duration := time.Since(transferStartedAt)
			averageBytesPerSecond := int64(0)
			result := "failed"
			switch {
			case transferErr == nil:
				result = "completed"
			case errors.Is(transferErr, ErrSFTPTransferDestinationExists):
				result = "destination_exists"
			case errors.Is(transferErr, context.Canceled), errors.Is(transferErr, context.DeadlineExceeded):
				result = "canceled"
			}
			if transferErr == nil && duration > 0 && request.Total > 0 {
				averageBytesPerSecond = int64(float64(request.Total) / duration.Seconds())
			}
			slog.Info("sftp transfer connections finished",
				"event", "sftp.connections.done",
				"direction", string(request.Direction),
				"active", active,
				"desired", desired,
				"blocked_reason", reason,
				"expanded", expanded,
				"degraded", degraded,
				"size_bytes", request.Total,
				"duration_ms", duration.Milliseconds(),
				"average_bytes_per_sec", averageBytesPerSecond,
				"result", result,
				"success", transferErr == nil)
		}()
	}

	switch request.Direction {
	case model.SFTPTransferUpload:

		if request.ResumeKey != "" {
			if resumable, ok := fileSystem.(sessionapp.ResumableUploadFileSystem); ok {
				transferErr = resumable.UploadFileParallelResumable(
					ctx,
					request.LocalPath,
					request.RemotePath,
					request.Total,
					overwrite,
					progress,
					request.ResumeKey,
				)

				if !errors.Is(ctx.Err(), context.Canceled) {
					resumable.DropResumeState(request.ResumeKey)
				}
				break
			}
		}
		transferErr = parallel.UploadFileParallel(
			ctx,
			request.LocalPath,
			request.RemotePath,
			request.Total,
			overwrite,
			request.CopyDestination,
			progress,
		)
	case model.SFTPTransferDownload:
		transferErr = parallel.DownloadFileParallel(
			ctx,
			request.RemotePath,
			request.LocalPath,
			request.Total,
			overwrite,
			progress,
		)
	default:
		transferErr = model.ErrInvalidSFTPTransfer
	}
	if errors.Is(transferErr, fs.ErrExist) {
		return ErrSFTPTransferDestinationExists
	}
	if transferErr == nil {
		return e.finishArchiveUpload(ctx, request, nil)
	}

	pausing := request.ResumeKey != "" && errors.Is(ctx.Err(), context.Canceled)
	if !pausing {
		if temporary := parallel.TemporaryUploadPath(transferErr); temporary != "" {
			e.removeRemoteTemporary(request.SessionID, temporary)
		}
	}
	return transferErr
}

func (e *SessionSFTPTransferExecutor) CleanupResume(ctx context.Context, sessionID, resumeKey string) error {
	if e == nil || e.sessions == nil || resumeKey == "" {
		return ErrSFTPTransferExecutorNotReady
	}
	fileSystem, err := e.sessions.TransferFileSystem(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("open transfer file system for resume cleanup: %w", err)
	}
	defer func() { _ = fileSystem.Close() }()
	resumable, ok := fileSystem.(sessionapp.ResumableUploadFileSystem)
	if !ok {
		return nil
	}
	temporaryPath := resumable.DropResumeState(resumeKey)
	if temporaryPath == "" {
		return nil
	}

	if remover, ok := fileSystem.(interface {
		RemoveTransferFile(context.Context, string) error
	}); ok {
		_ = remover.RemoveTransferFile(ctx, temporaryPath)
	}
	return nil
}

func (e *SessionSFTPTransferExecutor) finishArchiveUpload(
	ctx context.Context,
	request SFTPTransferRequest,
	transferErr error,
) error {
	if transferErr != nil {
		return transferErr
	}
	if request.Archive == nil {
		return nil
	}
	extractor, ok := e.sessions.(remoteArchiveExtractor)
	if !ok {

		e.removeRemoteTemporary(request.SessionID, request.RemotePath)
		return errors.New("remote archive extraction is unavailable")
	}

	request.reportStage(model.SFTPTransferStageExtracting)

	defer request.reportStage("")

	extractErr := extractor.ExtractRemoteArchive(
		ctx,
		request.SessionID,
		request.RemotePath,
		request.Archive.TargetDirectory,
	)
	e.removeRemoteTemporary(request.SessionID, request.RemotePath)
	if extractErr != nil {
		if isRemoteTarUnavailable(extractErr) {

			return fmt.Errorf("extract remote archive: remote tar is unavailable: %w", extractErr)
		}

		return fmt.Errorf("extract remote archive: %w", extractErr)
	}
	return nil
}

func (r SFTPTransferRequest) reportStage(stage string) {
	if r.ReportStage == nil {
		return
	}
	r.ReportStage(stage)
}

func (e *SessionSFTPTransferExecutor) openTransferFileSystem(
	ctx context.Context,
	request SFTPTransferRequest,
) (sessionapp.SessionTransferFileSystem, error) {
	var (
		fileSystem sessionapp.SessionTransferFileSystem
		err        error
	)
	if sizer, ok := e.sessions.(sessionTransferFileSystemSizer); ok {
		fileSystem, err = sizer.TransferFileSystemForSize(ctx, request.SessionID, request.Total)
	} else {
		fileSystem, err = e.sessions.TransferFileSystem(ctx, request.SessionID)
	}
	if err == nil {

		connections, desired, reason := -1, -1, "unsupported"
		if status, ok := fileSystem.(sessionapp.ConnectionStatusFileSystem); ok {
			connections, desired, reason = status.TransferConnectionStatus()
		}
		if e.reporter != nil {
			e.reporter.ReportTransferConnections(connections, desired, reason)
		}

		slog.Info("sftp transfer connections",
			"event", "sftp.connections",
			"direction", string(request.Direction),
			"active", connections,
			"desired", desired,
			"blocked_reason", reason,
			"size_bytes", request.Total)
	}
	return fileSystem, err
}

func finishTransferDirection(
	ctx context.Context,
	fileSystem sessionapp.SessionTransferFileSystem,
	request SFTPTransferRequest,
	overwrite bool,
	progress func(transferred int64) error,
) error {
	var transferErr error
	switch request.Direction {
	case model.SFTPTransferUpload:
		transferErr = fileSystem.UploadFileContext(
			ctx,
			request.LocalPath,
			request.RemotePath,
			request.Total,
			overwrite,
			progress,
		)
	case model.SFTPTransferDownload:
		transferErr = fileSystem.DownloadFileContext(
			ctx,
			request.RemotePath,
			request.LocalPath,
			request.Total,
			overwrite,
			progress,
		)
	default:
		transferErr = model.ErrInvalidSFTPTransfer
	}
	if errors.Is(transferErr, fs.ErrExist) {
		return ErrSFTPTransferDestinationExists
	}
	return transferErr
}

func (e *SessionSFTPTransferExecutor) removeRemoteTemporary(sessionID, remotePath string) {
	if remotePath == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	fileSystem, err := e.sessions.TransferFileSystem(ctx, sessionID)
	if err != nil {
		return
	}
	if remover, ok := fileSystem.(remoteStreamableFileSystem); ok {
		_ = remover.RemoveTransferFile(ctx, remotePath)
	}
	_ = fileSystem.Close()
}

func copyRemoteStream(
	ctx context.Context,
	dest remoteStreamableFileSystem,
	source remoteStreamableFileSystem,
	sourcePath string,
	destPath string,
	expectedSize int64,
	overwrite bool,
	progress func(transferred int64) error,
) (string, error) {
	reader, err := source.OpenFile(ctx, sourcePath)
	if err != nil {
		return "", fmt.Errorf("open source remote file for streaming: %w", err)
	}
	defer func() { _ = reader.Close() }()

	temporaryPath, writer, err := dest.CreateTransferFile(ctx, destPath)
	if err != nil {
		return "", fmt.Errorf("create dest remote temporary file for streaming: %w", err)
	}
	committed := false
	defer func() {
		_ = writer.Close()
		if committed {
			return
		}

		_ = dest.RemoveTransferFile(context.WithoutCancel(ctx), temporaryPath)
	}()

	buffer := make([]byte, 64*1024)
	var total int64
	var copyErr error
	for {
		if err := ctx.Err(); err != nil {
			copyErr = err
			break
		}
		read, readErr := reader.Read(buffer)
		if read > 0 {
			written, writeErr := writer.Write(buffer[:read])
			total += int64(written)
			if writeErr != nil {
				copyErr = writeErr
				break
			}
			if written != read {
				copyErr = io.ErrShortWrite
				break
			}
			if err := progress(total); err != nil {
				copyErr = err
				break
			}
		}
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				copyErr = readErr
			}
			break
		}
	}

	if copyErr != nil {
		return temporaryPath, fmt.Errorf("stream remote transfer: %w", copyErr)
	}
	if total != expectedSize {
		return temporaryPath, fmt.Errorf("streamed size = %d, want %d", total, expectedSize)
	}
	if err := writer.Close(); err != nil {
		return temporaryPath, fmt.Errorf("close dest remote temporary file: %w", err)
	}
	if err := dest.CommitTransferFile(ctx, temporaryPath, destPath, overwrite); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return temporaryPath, ErrSFTPTransferDestinationExists
		}
		return temporaryPath, fmt.Errorf("commit dest remote transfer file: %w", err)
	}
	committed = true
	return temporaryPath, nil
}
