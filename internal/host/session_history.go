package host

import (
	"context"
	"errors"
	"os"

	"changeme/internal/errno"
	"changeme/internal/model"
	storage "changeme/internal/store"
)

func (s *HostService) ListSessionHistory() ([]model.SessionHistory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return nil, ErrHostServiceNotReady
	}
	items, err := s.store.ListSessionHistory(context.Background())
	if err != nil {
		return nil, errno.New(errno.CodeStoreFailed, "history.list_failed", "")
	}
	return items, nil
}

func (s *HostService) SaveSessionHistory(item model.SessionHistory) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return ErrHostServiceNotReady
	}
	if err := s.store.SaveSessionHistory(context.Background(), item); err != nil {
		return errno.New(errno.CodeStoreFailed, "history.save_failed", "")
	}
	return nil
}

func (s *HostService) ArchiveSessionHistory(id string, archived bool) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return ErrHostServiceNotReady
	}
	if err := s.store.ArchiveSessionHistory(context.Background(), id, archived); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return errno.New(errno.CodeNotFound, "history.not_found", "")
		}
		return errno.New(errno.CodeStoreFailed, "history.archive_failed", "")
	}
	return nil
}

func (s *HostService) TrimSessionHistory(limit int) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return ErrHostServiceNotReady
	}
	if err := s.store.TrimSessionHistory(context.Background(), limit); err != nil {
		if errors.Is(err, storage.ErrInvalidSessionHistoryLimit) {
			return errno.New(errno.CodeInvalidArg, "history.limit_invalid", "")
		}
		return errno.New(errno.CodeStoreFailed, "history.trim_failed", "")
	}
	return nil
}

func (s *HostService) SetSessionHistoryLimit(limit int) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return ErrHostServiceNotReady
	}
	if err := s.store.SetSessionHistoryLimit(context.Background(), limit); err != nil {
		if errors.Is(err, storage.ErrInvalidSessionHistoryLimit) {
			return errno.New(errno.CodeInvalidArg, "history.limit_invalid", "")
		}
		return errno.New(errno.CodeStoreFailed, "history.trim_failed", "")
	}
	return nil
}

func (s *HostService) DeleteSessionHistory(id string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return ErrHostServiceNotReady
	}
	if err := s.store.DeleteSessionHistory(context.Background(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return errno.New(errno.CodeNotFound, "history.not_found", "")
		}
		return errno.New(errno.CodeStoreFailed, "history.delete_failed", "")
	}
	return nil
}

func (s *HostService) SetSessionHistoryArchived(ids []string, archived bool) (model.SessionHistoryBatchArchiveResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.store == nil {
		return model.SessionHistoryBatchArchiveResult{}, ErrHostServiceNotReady
	}
	changed, missing, err := s.store.SetSessionHistoryArchived(context.Background(), ids, archived)
	if err != nil {
		return model.SessionHistoryBatchArchiveResult{}, errno.Wrap(errno.CodeStoreFailed, "history.archive_failed", err)
	}
	return model.SessionHistoryBatchArchiveResult{Updated: changed, Missing: missing}, nil
}

func (s *HostService) ClearUnarchivedSessionHistory() (model.SessionHistoryClearResult, error) {
	s.mu.RLock()
	store := s.store
	recordingsRoot, rootErr := s.recordingRootHeld()
	s.mu.RUnlock()
	if store == nil {
		return model.SessionHistoryClearResult{}, ErrHostServiceNotReady
	}
	if rootErr != nil {
		return model.SessionHistoryClearResult{}, rootErr
	}
	ctx := context.Background()
	sessionIDs, err := store.ClearUnarchivedSessionHistory(ctx)
	if err != nil {
		return model.SessionHistoryClearResult{}, errno.Wrap(errno.CodeStoreFailed, "history.clear_failed", err)
	}
	result := model.SessionHistoryClearResult{Sessions: len(sessionIDs)}
	if len(sessionIDs) == 0 {
		return result, nil
	}
	recordings, err := store.ListTerminalRecordingsBySessionIDs(ctx, sessionIDs)
	if err != nil {

		return result, errno.Wrap(errno.CodeStoreFailed, "history.clear_recordings_failed", err)
	}
	for _, item := range recordings {
		if item.Status == "recording" || item.Status == "paused" {
			result.Skipped++
			continue
		}
		path, err := resolveRecordingPath(recordingsRoot, item.Path)
		if err != nil {
			result.Failed++
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			result.Failed++
			continue
		}
		if err := store.DeleteTerminalRecording(ctx, item.ID); err != nil && !errors.Is(err, storage.ErrNotFound) {
			result.Failed++
			continue
		}
		result.Recordings++
	}
	return result, nil
}
