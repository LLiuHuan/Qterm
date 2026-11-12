package sftp

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"changeme/internal/errno"
	"changeme/internal/model"
)

func (e *SessionSFTPTransferExecutor) transferArchiveDownload(
	ctx context.Context,
	request SFTPTransferRequest,
	progress func(transferred int64) error,
) error {
	spec := request.ArchiveDownload
	if spec == nil {
		return model.ErrInvalidSFTPTransfer
	}
	deps, err := archiveDownloadDepsFor(e.sessions)
	if err != nil {
		return err
	}
	hooks := archiveDownloadHooks{
		OnPack: func() {

			request.reportStage(model.SFTPTransferStagePacking)
		},
		OnPacked: func(size int64) {

			if size > 0 {
				request.reportTotal(size)
			}
			request.reportStage("")
		},
		OnExtract: func() {

			request.reportStage(model.SFTPTransferStageExtracting)
		},
		Progress: progress,
	}

	defer request.reportStage("")
	return downloadDirectoryArchiveWith(
		ctx,
		deps,
		request.SessionID,
		spec.SourceDirectory,
		spec.TargetDirectory,
		hooks,
	)
}

func (r SFTPTransferRequest) reportTotal(total int64) {
	if r.ReportTotal == nil {
		return
	}
	r.ReportTotal(total)
}

func (s *Service) EnqueueDirectoryArchiveDownload(
	ctx context.Context,
	sessionID string,
	remoteDirectory string,
	localDirectory string,
) (model.SFTPTransfer, error) {
	manager, transfers, _, err := s.transferDependencies()
	if err != nil {
		return model.SFTPTransfer{}, err
	}
	if err := localContextReady(ctx); err != nil {
		return model.SFTPTransfer{}, err
	}
	request, err := buildArchiveDownloadRequest(sessionID, remoteDirectory, localDirectory)
	if err != nil {
		return model.SFTPTransfer{}, err
	}

	if _, depErr := archiveDownloadDepsFor(manager); depErr != nil {
		return model.SFTPTransfer{}, depErr
	}
	return transfers.Enqueue(ctx, request)
}

func buildArchiveDownloadRequest(
	sessionID string,
	remoteDirectory string,
	localDirectory string,
) (SFTPTransferRequest, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return SFTPTransferRequest{}, errno.New(errno.CodeInvalidArg, "session.not_found", "")
	}
	remote, err := model.NormalizeRemoteDirectory(remoteDirectory)
	if err != nil {
		return SFTPTransferRequest{}, errno.New(errno.CodeInvalidArg, "sftp.remote_path_invalid", "")
	}

	if remote == "/" {
		return SFTPTransferRequest{}, errno.New(errno.CodeInvalidArg, "sftp.archive.source_invalid", "")
	}
	local, err := normalizeDialogPath(localDirectory)
	if err != nil {
		return SFTPTransferRequest{}, err
	}
	info, statErr := os.Stat(local)
	if statErr != nil || !info.IsDir() {
		return SFTPTransferRequest{}, errno.New(errno.CodeNotFound, "sftp.local_directory_not_found", "")
	}
	localName := filepath.Base(local)

	if localName == "" || localName == "." || localName == ".." || localName == "/" ||
		strings.ContainsAny(localName, "/\\") {
		return SFTPTransferRequest{}, errno.New(errno.CodeInvalidArg, "sftp.local_directory_invalid", "")
	}
	return SFTPTransferRequest{
		SessionID:  sessionID,
		Direction:  model.SFTPTransferDownload,
		LocalPath:  local,
		LocalName:  localName,
		RemotePath: remote,
		Total:      0,
		ArchiveDownload: &ArchiveDownloadSpec{
			SourceDirectory: remote,
			TargetDirectory: local,
		},
	}, nil
}
