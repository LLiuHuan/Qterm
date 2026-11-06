package session

import (
	"changeme/internal/errno"
	"changeme/internal/model"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

func (s *Service) StartRecording(ctx context.Context, id string) (model.TerminalRecording, error) {
	return s.startRecording(ctx, id, recordingModeSegment)
}

func (s *Service) StartFullRecording(ctx context.Context, id string) (model.TerminalRecording, error) {
	return s.startRecording(ctx, id, recordingModeFull)
}

func (s *Service) startRecording(ctx context.Context, id, mode string) (model.TerminalRecording, error) {
	manager, _, err := s.ready()
	if err != nil {
		return model.TerminalRecording{}, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return model.TerminalRecording{}, errno.New(errno.CodeInvalidArg, "recording.session_required", "")
	}
	snapshot, err := manager.Get(ctx, id)
	if err != nil {
		return model.TerminalRecording{}, sessionServiceError(err)
	}
	recordingsRoot, configErr := s.RecordingDirectory()
	if configErr != nil {
		return model.TerminalRecording{}, configErr
	}
	active, created, err := s.recordings.ensure(id, snapshot.HostID, recordingsRoot, mode, time.Now())
	if err != nil {
		return model.TerminalRecording{}, err
	}
	if !created {

		return active.Snapshot(), nil
	}

	if host, hostErr := s.hosts.SessionHost(ctx, snapshot.HostID); hostErr == nil {
		active.mu.Lock()
		active.metadata.HostName = host.Name
		active.metadata.Address = host.Address
		active.mu.Unlock()
	}
	if err := manager.SetTerminalRecorderFor(ctx, id, mode, active); err != nil {

		_ = active.Close()
		return model.TerminalRecording{}, sessionServiceError(err)
	}

	if err := s.recordings.save(active.Snapshot()); err != nil {
		_ = manager.SetTerminalRecorderFor(ctx, id, mode, nil)
		return model.TerminalRecording{}, errno.Wrap(errno.CodeStoreFailed, "recording.index_failed", err)
	}
	return active.Snapshot(), nil
}

func (s *Service) StopRecording(ctx context.Context, id string) (model.TerminalRecording, error) {
	return s.stopRecording(ctx, id, recordingModeSegment, true)
}

func (s *Service) StopFullRecording(ctx context.Context, id string) (model.TerminalRecording, error) {
	return s.stopRecording(ctx, id, recordingModeFull, true)
}

func (s *Service) stopRecording(ctx context.Context, id, mode string, markCompleted bool) (model.TerminalRecording, error) {
	manager, _, err := s.ready()
	if err != nil {
		return model.TerminalRecording{}, err
	}
	id = strings.TrimSpace(id)

	s.recordings.flushPending()
	active, exists := s.recordings.peek(id, mode)
	if !exists {
		return model.TerminalRecording{}, errno.New(errno.CodeNotFound, "recording.not_found", "")
	}

	if markCompleted {
		active.MarkCompleted()
	}
	if err := manager.SetTerminalRecorderFor(ctx, id, mode, nil); err != nil {

		if errors.Is(err, ErrSessionTerminalDetached) {
			return active.Snapshot(), nil
		}

		s.recordings.drop(id, mode, active)
		_ = active.Close()
		return active.Snapshot(), sessionServiceError(err)
	}

	if current, ok := s.recordings.peek(id, mode); ok && current == active {
		s.recordings.drop(id, mode, active)
		_ = active.Close()
	}
	return active.Snapshot(), nil
}

func (s *Service) PauseRecording(ctx context.Context, id string) (model.TerminalRecording, error) {
	if _, _, err := s.ready(); err != nil {
		return model.TerminalRecording{}, err
	}
	if err := ctx.Err(); err != nil {
		return model.TerminalRecording{}, err
	}
	s.recordings.flushPending()
	active, exists := s.recordings.peek(strings.TrimSpace(id), recordingModeSegment)
	if !exists {
		return model.TerminalRecording{}, errno.New(errno.CodeNotFound, "recording.not_found", "")
	}
	active.SetPaused(true)
	if err := s.recordings.save(active.Snapshot()); err != nil {
		return active.Snapshot(), errno.Wrap(errno.CodeStoreFailed, "recording.index_failed", err)
	}
	return active.Snapshot(), nil
}

func (s *Service) ResumeRecording(ctx context.Context, id string) (model.TerminalRecording, error) {
	manager, _, err := s.ready()
	if err != nil {
		return model.TerminalRecording{}, err
	}
	if err := ctx.Err(); err != nil {
		return model.TerminalRecording{}, err
	}
	s.recordings.flushPending()
	id = strings.TrimSpace(id)
	active, exists := s.recordings.peek(id, recordingModeSegment)
	if !exists {
		return model.TerminalRecording{}, errno.New(errno.CodeNotFound, "recording.not_found", "")
	}
	active.SetPaused(false)

	if err := manager.SetTerminalRecorderFor(ctx, id, recordingModeSegment, active); err != nil {

		if current, ok := s.recordings.peek(id, recordingModeSegment); ok && current == active {
			active.SetPaused(true)
			_ = s.recordings.save(active.Snapshot())
		}
		return active.Snapshot(), sessionServiceError(err)
	}
	if err := s.recordings.save(active.Snapshot()); err != nil {
		return active.Snapshot(), errno.Wrap(errno.CodeStoreFailed, "recording.index_failed", err)
	}
	return active.Snapshot(), nil
}

func (s *Service) ReadActiveRecordingEvents(ctx context.Context, id string, cursor int64) (model.TerminalRecordingEventBatch, error) {
	if _, _, err := s.ready(); err != nil {
		return model.TerminalRecordingEventBatch{}, err
	}
	if err := ctx.Err(); err != nil {
		return model.TerminalRecordingEventBatch{}, err
	}
	active, exists := s.recordings.peek(strings.TrimSpace(id), recordingModeSegment)
	if !exists {
		return model.TerminalRecordingEventBatch{}, errno.New(errno.CodeNotFound, "recording.not_found", "")
	}
	next, events, truncated := active.EventsAfter(cursor)
	result := model.TerminalRecordingEventBatch{Cursor: next, Truncated: truncated, Events: make([]model.TerminalRecordingEvent, 0, len(events))}
	for _, event := range events {
		result.Events = append(result.Events, model.TerminalRecordingEvent{AtMS: event.At.Milliseconds(), Type: event.Type, Columns: event.Columns, Rows: event.Rows, Data: base64.StdEncoding.EncodeToString(event.Data)})
	}
	return result, nil
}
