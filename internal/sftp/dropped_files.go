package sftp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"changeme/internal/errno"
	"changeme/internal/model"
)

type droppedFiles struct {
	sessionID string
	paths     []string
	expires   time.Time
}

type DropUploadResult struct {
	Transfers      []model.SFTPTransfer `json:"transfers"`
	Rejected       []string             `json:"rejected"`
	Queued         int                  `json:"queued"`
	FellBack       bool                 `json:"fellBack"`
	FallbackReason string               `json:"fallbackReason,omitempty"`
}

func (s *Service) RegisterDroppedFiles(sessionID string, paths []string) (string, error) {
	if len(paths) == 0 || len(paths) > 100 {
		return "", errno.New(errno.CodeInvalidArg, "sftp.drop_limit", "")
	}
	var token [24]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", errno.New(errno.CodeUnknown, "sftp.failed", "")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.transfers == nil {
		return "", ErrServiceNotReady
	}
	for id, pending := range s.drops {
		if time.Now().After(pending.expires) {
			delete(s.drops, id)
		}
	}
	if len(s.drops) >= 20 {
		return "", errno.New(errno.CodeInvalidArg, "sftp.drop_limit", "")
	}
	if s.drops == nil {
		s.drops = make(map[string]droppedFiles)
	}
	id := hex.EncodeToString(token[:])
	s.drops[id] = droppedFiles{sessionID: sessionID, paths: append([]string(nil), paths...), expires: time.Now().Add(10 * time.Minute)}
	return id, nil
}

func isDirectorySource(localPath string) bool {
	info, err := os.Lstat(strings.TrimSpace(localPath))
	return err == nil && info.IsDir()
}

func (s *Service) CancelDroppedFiles(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.drops, id)
}

func (s *Service) UploadDroppedFiles(ctx context.Context, id, directory string) (DropUploadResult, error) {
	result := DropUploadResult{Transfers: []model.SFTPTransfer{}, Rejected: []string{}}
	directory, err := model.NormalizeRemoteDirectory(directory)
	if err != nil {
		return result, errno.New(errno.CodeInvalidArg, "sftp.directory_invalid", "")
	}
	manager, transfers, _, err := s.transferDependencies()
	if err != nil {
		return result, err
	}
	s.mu.Lock()
	pending, ok := s.drops[id]
	s.mu.Unlock()
	if !ok || time.Now().After(pending.expires) {
		s.CancelDroppedFiles(id)
		return result, errno.New(errno.CodeNotFound, "sftp.drop_expired", "")
	}
	info, err := manager.Get(ctx, pending.sessionID)
	if err != nil || info.State != model.SessionStateConnected || info.Kind == model.SessionKindLocal {
		s.CancelDroppedFiles(id)
		return result, errno.New(errno.CodeSessionClosed, "session.disconnected", "")
	}

	filesystem, err := manager.FileSystem(ctx, pending.sessionID)
	if err != nil {
		return result, ServiceError(err)
	}
	target, err := filesystem.StatContext(ctx, directory)
	if err != nil || !target.IsDir() {
		return result, errno.New(errno.CodeInvalidArg, "sftp.directory_invalid", "")
	}
	s.mu.Lock()
	_, ok = s.drops[id]
	delete(s.drops, id)
	s.mu.Unlock()
	if !ok {
		return result, errno.New(errno.CodeNotFound, "sftp.drop_expired", "")
	}
	seen := make(map[string]bool)
	for _, source := range pending.paths {
		if seen[source] {
			continue
		}
		seen[source] = true
		name := filepath.Base(source)
		if ctx.Err() != nil {
			result.Rejected = append(result.Rejected, name)
			continue
		}

		if isDirectorySource(source) {
			uploadDroppedDirectorySource(
				ctx,
				name,
				s.dragFolderUploadMode() == DragFolderUploadModeArchive,
				func(ctx context.Context) (archiveUploadOutcome, error) {
					return s.enqueueArchivedDirectoryUpload(ctx, pending.sessionID, source, directory)
				},
				func(ctx context.Context) (DirectoryUploadResult, error) {
					return s.enqueueDirectoryUpload(ctx, pending.sessionID, source, directory)
				},
				&result,
			)
			continue
		}
		localPath, localName, size, inspectErr := InspectUploadSource(source)
		if inspectErr != nil {
			result.Rejected = append(result.Rejected, name)
			continue
		}
		remotePath, normalizeErr := model.NormalizeRemotePath(path.Join(directory, localName))
		if normalizeErr != nil || strings.ContainsAny(localName, "\r\n") {
			result.Rejected = append(result.Rejected, name)
			continue
		}
		transfer, enqueueErr := transfers.Enqueue(ctx, SFTPTransferRequest{SessionID: pending.sessionID, BatchID: id, Direction: model.SFTPTransferUpload, LocalPath: localPath, LocalName: localName, RemotePath: remotePath, Total: size})
		if enqueueErr != nil {
			result.Rejected = append(result.Rejected, name)
			continue
		}
		result.Transfers = append(result.Transfers, transfer)
	}
	return result, nil
}

func uploadDroppedDirectorySource(
	ctx context.Context,
	name string,
	useArchive bool,
	archive func(context.Context) (archiveUploadOutcome, error),
	normal func(context.Context) (DirectoryUploadResult, error),
	result *DropUploadResult,
) {
	if useArchive {
		outcome, archiveErr := archive(ctx)
		switch {
		case archiveErr != nil:
			result.Rejected = append(result.Rejected, name)
			return
		case !outcome.FellBack:

			result.Queued++
			result.Transfers = append(result.Transfers, outcome.Transfer)
			return
		default:

			result.FellBack = true
			result.FallbackReason = outcome.FallbackReason
		}
	}
	directoryResult, directoryErr := normal(ctx)
	if directoryErr != nil {
		result.Rejected = append(result.Rejected, name)
		return
	}
	result.Queued += directoryResult.Queued
}
