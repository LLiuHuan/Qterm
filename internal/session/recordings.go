package session

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"changeme/internal/errno"
	"changeme/internal/model"
	"changeme/internal/recording"
)

const (
	recordingStatusRecording   = "recording"
	recordingStatusPaused      = "paused"
	recordingStatusCompleted   = "completed"
	recordingStatusInterrupted = "interrupted"
	recordingModeSegment       = "segment"
	recordingModeFull          = "full"
)

type RecordingIndex interface {
	SaveTerminalRecording(item model.TerminalRecording) error
}

type recordingRegistry struct {
	mu     sync.Mutex
	index  RecordingIndex
	active map[string]map[string]*activeRecording

	pending []model.TerminalRecording
}

func newRecordingRegistry() *recordingRegistry {
	return &recordingRegistry{active: make(map[string]map[string]*activeRecording)}
}

func (r *recordingRegistry) SetIndex(index RecordingIndex) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.index = index
	r.mu.Unlock()
}

func (r *recordingRegistry) save(item model.TerminalRecording) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	index := r.index
	r.mu.Unlock()
	if index == nil {
		return nil
	}
	return index.SaveTerminalRecording(item)
}

func (r *recordingRegistry) saveOrDefer(item model.TerminalRecording) error {
	if err := r.save(item); err != nil {
		r.mu.Lock()
		r.pending = append(r.pending, item)
		r.mu.Unlock()
		return err
	}
	return nil
}

func (r *recordingRegistry) flushPending() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	index := r.index
	if index == nil {

		r.mu.Unlock()
		return len(r.pending)
	}
	items := append([]model.TerminalRecording(nil), r.pending...)
	r.pending = nil
	r.mu.Unlock()
	if len(items) == 0 {
		return 0
	}
	remaining := 0
	for _, item := range items {
		if err := index.SaveTerminalRecording(item); err != nil {
			r.mu.Lock()
			r.pending = append(r.pending, item)
			r.mu.Unlock()
			remaining++
		}
	}
	return remaining
}

func (r *recordingRegistry) take(id, mode string) (*activeRecording, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	byMode := r.active[id]
	item, exists := byMode[mode]
	if !exists {
		return nil, false
	}
	delete(byMode, mode)
	if len(byMode) == 0 {
		delete(r.active, id)
	}
	return item, true
}

func (r *recordingRegistry) peek(id, mode string) (*activeRecording, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, exists := r.active[id][mode]
	return item, exists
}

func (r *recordingRegistry) drop(id, mode string, item *activeRecording) {
	r.mu.Lock()
	if byMode, exists := r.active[id]; exists {
		if current, exists := byMode[mode]; exists && current == item {
			delete(byMode, mode)
		}
		if len(byMode) == 0 {
			delete(r.active, id)
		}
	}
	r.mu.Unlock()
}

func (r *recordingRegistry) snapshot() []*activeRecording {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]*activeRecording, 0, len(r.active))
	for _, byMode := range r.active {
		for _, item := range byMode {
			items = append(items, item)
		}
	}
	return items
}

type activeRecording struct {
	sessionID string
	mode      string
	registry  *recordingRegistry
	writer    *recording.Writer

	closeOnce sync.Once
	closeErr  error
	mu        sync.Mutex
	metadata  model.TerminalRecording
}

func (a *activeRecording) Output(data []byte) error {
	if a == nil || a.writer == nil {
		return nil
	}
	return a.writer.Output(data)
}

func (a *activeRecording) Input(data []byte) error {
	if a == nil || a.writer == nil {
		return nil
	}
	return a.writer.Input(data)
}

func (a *activeRecording) Resize(columns, rows int) error {
	if a == nil || a.writer == nil {
		return nil
	}
	return a.writer.Resize(columns, rows)
}

func (a *activeRecording) EventsAfter(cursor int64) (int64, []recording.Event, bool) {
	if a == nil || a.writer == nil {
		return cursor, nil, false
	}
	return a.writer.EventsAfter(cursor)
}

func (a *activeRecording) SetPaused(paused bool) {
	if a == nil || a.writer == nil {
		return
	}
	a.writer.SetPaused(paused)
	a.mu.Lock()
	if paused {
		a.metadata.Status = recordingStatusPaused
	} else {
		a.metadata.Status = recordingStatusRecording
	}
	a.mu.Unlock()
}

func (a *activeRecording) MarkCompleted() {
	if a == nil {
		return
	}
	a.mu.Lock()
	a.metadata.Status = recordingStatusCompleted
	a.mu.Unlock()
}

func (a *activeRecording) Snapshot() model.TerminalRecording {
	if a == nil {
		return model.TerminalRecording{}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.metadata
}

func (a *activeRecording) Close() error {
	if a == nil {
		return nil
	}
	a.closeOnce.Do(func() {
		a.mu.Lock()
		if a.metadata.Status == recordingStatusRecording || a.metadata.Status == recordingStatusPaused {
			a.metadata.Status = recordingStatusInterrupted
		}
		a.mu.Unlock()

		bytes := a.writer.Bytes()
		events := a.writer.Events()
		writerErr := a.writer.Close()

		a.mu.Lock()
		a.metadata.EndedAt = time.Now().UTC()
		a.metadata.Bytes = bytes
		a.metadata.EventCount = events
		item := a.metadata
		a.mu.Unlock()

		if a.registry != nil {
			a.registry.drop(a.sessionID, a.mode, a)
		}

		a.closeErr = errors.Join(writerErr, a.registry.saveOrDefer(item))
	})
	return a.closeErr
}

func (r *recordingRegistry) ensure(sessionID, hostID, recordingsRoot, mode string, now time.Time) (*activeRecording, bool, error) {

	r.flushPending()
	startedAt := now.UTC()
	recordingID := "rec-" + sessionID + "-" + mode + "-" + startedAt.Format("20060102T150405.000000000Z")

	path := filepath.Join(recordingsRoot, recordingID+".qtr")

	if err := os.MkdirAll(recordingsRoot, 0o700); err != nil {
		return nil, false, errno.New(errno.CodeStoreFailed, "recording.create_failed", "")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	byMode := r.active[sessionID]
	if existing, ok := byMode[mode]; ok {
		return existing, false, nil
	}
	writer, err := recording.Create(path, startedAt)
	if err != nil {
		return nil, false, errno.New(errno.CodeStoreFailed, "recording.create_failed", "")
	}
	item := &activeRecording{
		sessionID: sessionID,
		mode:      mode,
		registry:  r,
		writer:    writer,
		metadata: model.TerminalRecording{
			ID:        recordingID,
			SessionID: sessionID,
			Mode:      mode,
			HostID:    hostID,
			Path:      path,
			StartedAt: startedAt,
			Status:    recordingStatusRecording,
		},
	}
	if byMode == nil {
		byMode = make(map[string]*activeRecording)
		r.active[sessionID] = byMode
	}
	byMode[mode] = item
	return item, true, nil
}

func (r *recordingRegistry) finishRecording(sessionID, mode string, markCompleted bool) (model.TerminalRecording, error) {
	item, exists := r.take(sessionID, mode)
	if !exists {
		return model.TerminalRecording{}, errno.New(errno.CodeNotFound, "recording.not_found", "")
	}
	if markCompleted {
		item.MarkCompleted()
	}

	if err := item.Close(); err != nil {
		return item.Snapshot(), err
	}
	return item.Snapshot(), nil
}

func (r *recordingRegistry) finishAll() []error {
	items := r.snapshot()
	failures := make([]error, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		r.drop(item.sessionID, item.mode, item)
		if err := item.Close(); err != nil {
			failures = append(failures, err)
		}
	}

	if remaining := r.flushPending(); remaining > 0 {
		failures = append(failures, errno.New(errno.CodeStoreFailed, "recording.index_failed", ""))
	}
	return failures
}

func (r *recordingRegistry) pendingCount() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.pending)
}
