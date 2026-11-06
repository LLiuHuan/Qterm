package host

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"os"

	"changeme/internal/errno"
	"changeme/internal/model"
	"changeme/internal/recording"
	storage "changeme/internal/store"
)

func (s *HostService) ListTerminalRecordings() ([]model.TerminalRecording, error) {
	s.mu.RLock()
	store := s.store
	s.mu.RUnlock()
	if store == nil {
		return nil, ErrHostServiceNotReady
	}
	items, err := store.ListTerminalRecordings(context.Background())
	if err != nil {
		return nil, errno.Wrap(errno.CodeSessionClosed, "host.list_recordings_failed", err)
	}
	return items, nil
}

func (s *HostService) ListTerminalRecordingsPage(page, pageSize int, query string) (model.TerminalRecordingPage, error) {
	if page < 1 || pageSize < 1 || pageSize > 100 {
		return model.TerminalRecordingPage{}, errno.New(errno.CodeInvalidArg, "host.recording_page_invalid", "")
	}
	s.mu.RLock()
	store := s.store
	s.mu.RUnlock()
	if store == nil {
		return model.TerminalRecordingPage{}, ErrHostServiceNotReady
	}
	items, total, err := store.ListTerminalRecordingsPage(context.Background(), query, (page-1)*pageSize, pageSize)
	if err != nil {
		return model.TerminalRecordingPage{}, errno.Wrap(errno.CodeSessionClosed, "host.list_recordings_failed", err)
	}
	return model.TerminalRecordingPage{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *HostService) SaveTerminalRecording(item model.TerminalRecording) error {
	s.mu.RLock()
	store := s.store
	s.mu.RUnlock()
	if store == nil {
		return ErrHostServiceNotReady
	}
	if err := store.SaveTerminalRecording(context.Background(), item); err != nil {
		return errno.Wrap(errno.CodeSessionClosed, "host.save_recording_failed", err)
	}
	return nil
}

func (s *HostService) UpdateTerminalRecordingNote(id, note string) error {
	s.mu.RLock()
	store := s.store
	s.mu.RUnlock()
	if store == nil {
		return ErrHostServiceNotReady
	}
	if err := store.UpdateTerminalRecordingNote(context.Background(), id, note); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return errno.New(errno.CodeNotFound, "host.recording_not_found", "")
		}
		return errno.Wrap(errno.CodeStoreFailed, "host.recording_note_failed", err)
	}
	return nil
}

func (s *HostService) DeleteTerminalRecording(id string) error {
	s.mu.RLock()
	store := s.store
	recordingsRoot, rootErr := s.recordingRootHeld()
	s.mu.RUnlock()
	if store == nil {
		return ErrHostServiceNotReady
	}
	if rootErr != nil {
		return rootErr
	}
	item, err := store.GetTerminalRecording(context.Background(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return errno.New(errno.CodeNotFound, "host.recording_not_found", "")
		}
		return errno.Wrap(errno.CodeStoreFailed, "host.recording_lookup_failed", err)
	}

	if item.Status == "recording" || item.Status == "paused" {
		return errno.New(errno.CodeInvalidArg, "host.recording_active", "")
	}
	path, err := resolveRecordingPath(recordingsRoot, item.Path)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errno.Wrap(errno.CodeStoreFailed, "host.recording_file_delete_failed", err)
	}
	if err := store.DeleteTerminalRecording(context.Background(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return errno.New(errno.CodeSessionClosed, "host.recording_not_found", "")
		}
		return errno.Wrap(errno.CodeSessionClosed, "host.delete_recording_failed", err)
	}
	return nil
}

func (s *HostService) DeleteTerminalRecordings(ids []string) (model.TerminalRecordingClearResult, error) {
	idList := normalizeTerminalRecordingIDs(ids)
	if len(idList) == 0 {
		return model.TerminalRecordingClearResult{}, errno.New(errno.CodeInvalidArg, "host.recording_ids_required", "")
	}
	s.mu.RLock()
	store := s.store
	recordingsRoot, rootErr := s.recordingRootHeld()
	s.mu.RUnlock()
	if store == nil {
		return model.TerminalRecordingClearResult{}, ErrHostServiceNotReady
	}
	if rootErr != nil {
		return model.TerminalRecordingClearResult{}, rootErr
	}
	items, err := store.GetTerminalRecordings(context.Background(), idList)
	if err != nil {
		return model.TerminalRecordingClearResult{}, errno.Wrap(errno.CodeStoreFailed, "host.list_recordings_failed", err)
	}

	byID := make(map[string]model.TerminalRecording, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	result := model.TerminalRecordingClearResult{}
	for _, id := range idList {
		item, ok := byID[id]
		if !ok {
			result.Missing++
			continue
		}
		if item.Status == "recording" || item.Status == "paused" {
			result.Skipped++
			continue
		}
		path, pathErr := resolveRecordingPath(recordingsRoot, item.Path)
		if pathErr != nil {
			result.Failed++
			continue
		}
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			result.Failed++
			continue
		}
		if deleteErr := store.DeleteTerminalRecording(context.Background(), id); deleteErr != nil && !errors.Is(deleteErr, storage.ErrNotFound) {
			result.Failed++
			continue
		}
		result.Deleted++
	}
	return result, nil
}

func (s *HostService) ReadTerminalRecording(id string) ([]model.TerminalRecordingEvent, error) {
	s.mu.RLock()
	store := s.store
	recordingsRoot, rootErr := s.recordingRootHeld()
	s.mu.RUnlock()
	if store == nil {
		return nil, ErrHostServiceNotReady
	}
	if rootErr != nil {
		return nil, rootErr
	}
	item, err := store.GetTerminalRecording(context.Background(), id)
	if err != nil {
		return nil, errno.Wrap(errno.CodeNotFound, "host.recording_not_found", err)
	}

	path, err := resolveRecordingPath(recordingsRoot, item.Path)
	if err != nil {
		return nil, err
	}
	reader, err := recording.Open(path)
	if err != nil {
		return nil, errno.Wrap(errno.CodeStoreFailed, "host.recording_read_failed", err)
	}
	defer reader.Close()

	live := item.Status == "recording" || item.Status == "paused"
	events := make([]model.TerminalRecordingEvent, 0)
	var total int64
	for len(events) < 100000 && total < 64<<20 {
		event, nextErr := reader.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			if live {
				break
			}
			return nil, errno.Wrap(errno.CodeStoreFailed, "host.recording_corrupt", nextErr)
		}
		if event.Type == recording.EventEnd {
			break
		}
		total += int64(len(event.Data))
		events = append(events, model.TerminalRecordingEvent{AtMS: event.At.Milliseconds(), Type: event.Type, Columns: event.Columns, Rows: event.Rows, Data: base64.StdEncoding.EncodeToString(event.Data)})
	}
	return events, nil
}
