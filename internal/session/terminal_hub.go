package session

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	sshclient "changeme/internal/tunnel"
)

var ErrSessionTerminalDetached = errors.New("session terminal attachment was replaced")

type terminalReadResult struct {
	data []byte
	err  error

	historyEnd uint64
}

type TerminalAttachmentState struct {
	Attached bool `json:"attached"`

	AttachedAtLeastOnce bool `json:"attachedAtLeastOnce"`

	AttachEpoch uint64 `json:"attachEpoch"`

	LastDetachAt time.Time `json:"lastDetachAt"`
}

type terminalHub struct {
	mu  sync.Mutex
	raw SessionTerminal

	reader    io.Reader
	writer    io.Writer
	recorders map[string]TerminalRecorder
	current   *terminalAttachment

	history []byte

	historyBytes uint64
	nextID       uint64
	wake         chan struct{}
	closed       chan struct{}
	readDone     chan struct{}
	waitDone     chan struct{}
	waitErr      error
	closeOnce    sync.Once
	closeErr     error

	now func() time.Time

	sessionID string

	attachEpoch         uint64
	attachedAtLeastOnce bool
	lastDetachAt        time.Time
}

func (h *terminalHub) AttachmentState() TerminalAttachmentState {
	if h == nil {
		return TerminalAttachmentState{}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return TerminalAttachmentState{
		Attached:            h.current != nil,
		AttachedAtLeastOnce: h.attachedAtLeastOnce,
		AttachEpoch:         h.attachEpoch,
		LastDetachAt:        h.lastDetachAt,
	}
}

func (h *terminalHub) attachmentNow() time.Time {
	if h.now == nil {
		return time.Now()
	}
	return h.now()
}

func (h *terminalHub) SetRecorder(recorder TerminalRecorder) error {
	return h.SetRecorderFor("segment", recorder)
}

func (h *terminalHub) SetRecorderFor(key string, recorder TerminalRecorder) error {
	if key == "" {
		return errors.New("terminal recorder key is empty")
	}
	h.mu.Lock()
	select {
	case <-h.closed:
		h.mu.Unlock()
		if recorder != nil {
			return recorder.Close()
		}
		return ErrSessionTerminalDetached
	default:
	}
	previous := h.recorders[key]
	var history []byte
	if recorder != nil {
		switch key {
		case recordingModeFull:

			history = append([]byte(nil), h.history...)
		case recordingModeSegment:

			history = terminalCurrentLine(h.history)
			if previous == recorder && len(history) > 0 {

				history = append([]byte("\r\n"), history...)
			}
		}
	}
	if recorder == nil {
		delete(h.recorders, key)
	} else {
		h.recorders[key] = recorder
	}
	h.mu.Unlock()
	if previous != nil && previous != recorder {
		return previous.Close()
	}
	if recorder != nil && len(history) > 0 {
		if err := recorder.Output(history); err != nil {
			_ = h.SetRecorderFor(key, nil)
			return err
		}
	}
	return nil
}

const terminalHistoryLimit = 256 << 10

func terminalCurrentLine(history []byte) []byte {
	for index := len(history) - 1; index >= 0; index-- {
		if history[index] == '\r' || history[index] == '\n' {
			return append([]byte(nil), history[index+1:]...)
		}
	}
	return append([]byte(nil), history...)
}

func newTerminalHub(raw SessionTerminal) *terminalHub {
	return newTerminalHubWithClock(raw, time.Now, "")
}

func newTerminalHubWithClock(raw SessionTerminal, now func() time.Time, encoding string) *terminalHub {
	reader, writer := io.Reader(raw), io.Writer(raw)

	if decoded, err := sshclient.NewTerminalOutputReader(raw, encoding); err == nil {
		reader = decoded
	}
	if encoded, err := sshclient.NewTerminalInputEncoder(raw, encoding); err == nil {
		writer = encoded
	}
	hub := &terminalHub{
		raw:       raw,
		reader:    reader,
		writer:    writer,
		recorders: make(map[string]TerminalRecorder),
		wake:      make(chan struct{}),
		closed:    make(chan struct{}),
		readDone:  make(chan struct{}),
		waitDone:  make(chan struct{}),
		now:       now,
	}
	go hub.readOutput()
	go hub.waitTerminal()
	return hub
}

func (h *terminalHub) ExitStatus() (int, bool) {
	if h == nil {
		return 0, false
	}
	h.mu.Lock()
	raw := h.raw
	h.mu.Unlock()
	provider, ok := raw.(terminalExitStatus)
	if !ok {
		return 0, false
	}
	return provider.ExitStatus()
}

func (h *terminalHub) attach() SessionTerminal {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nextID++
	history := append([]byte(nil), h.history...)
	return &terminalAttachment{
		hub:     h,
		id:      h.nextID,
		output:  make(chan terminalReadResult),
		closed:  make(chan struct{}),
		pending: history,

		historyStart: h.historyBytes,
	}
}

func (h *terminalHub) releaseHistory() {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.history = nil
	h.mu.Unlock()
}

func (h *terminalHub) readOutput() {
	defer close(h.readDone)

	buffer := make([]byte, 32<<10)
	firstOutput := true
	for {
		count, err := h.reader.Read(buffer)
		if count > 0 {
			if firstOutput {
				firstOutput = false
				logFirstTerminalOutput(h.sessionID, buffer[:count])
			}
			data := append([]byte(nil), buffer[:count]...)
			if !h.deliver(terminalReadResult{data: data}) {
				return
			}
		}
		if err != nil {
			_ = h.deliver(terminalReadResult{err: err})
			return
		}
	}
}

func logFirstTerminalOutput(sessionID string, data []byte) {
	digest := sha256.Sum256(data)
	slog.Debug("terminal first output",
		"event", "terminal.first_output",
		"session", sessionID,
		"bytes", len(data),
		"sha256_prefix", hex.EncodeToString(digest[:8]),
		"login_banner", bytes.Contains(data, []byte("Last login:")),
	)
}

func (h *terminalHub) deliver(result terminalReadResult) bool {

	if len(result.data) > 0 {
		h.mu.Lock()
		recorders := h.recorderSnapshotLocked()
		h.history = append(h.history, result.data...)
		h.historyBytes += uint64(len(result.data))
		result.historyEnd = h.historyBytes
		if len(h.history) > terminalHistoryLimit {
			h.history = append([]byte(nil), h.history[len(h.history)-terminalHistoryLimit:]...)
		}
		h.mu.Unlock()
		for key, recorder := range recorders {
			if err := recorder.Output(result.data); err != nil {
				h.removeRecorder(key, recorder)
			}
		}
	}
	for {
		h.mu.Lock()
		attachment := h.current
		wake := h.wake
		closed := h.closed
		h.mu.Unlock()
		if attachment == nil {
			select {
			case <-wake:
				continue
			case <-closed:
				return false
			}
		}

		if len(result.data) > 0 && result.historyEnd <= attachment.historyStart {
			return true
		}
		select {
		case attachment.output <- result:
			return true
		case <-attachment.closed:
			continue
		case <-closed:
			return false
		}
	}
}

func (h *terminalHub) recorderSnapshotLocked() map[string]TerminalRecorder {
	result := make(map[string]TerminalRecorder, len(h.recorders))
	for key, recorder := range h.recorders {
		result[key] = recorder
	}
	return result
}

func (h *terminalHub) removeRecorder(key string, recorder TerminalRecorder) {
	h.mu.Lock()
	if current, ok := h.recorders[key]; ok && current == recorder {
		delete(h.recorders, key)
	}
	h.mu.Unlock()
	if recorder != nil {
		_ = recorder.Close()
	}
}

func (h *terminalHub) recordInput(data []byte) {
	if len(data) == 0 {
		return
	}
	h.mu.Lock()
	recorders := h.recorderSnapshotLocked()
	h.mu.Unlock()
	for key, recorder := range recorders {
		if err := recorder.Input(data); err != nil {
			h.removeRecorder(key, recorder)
		}
	}
}

func (h *terminalHub) WriteSensitive(data []byte) (int, error) {
	if h == nil || len(data) == 0 {
		return 0, nil
	}
	select {
	case <-h.closed:

		return 0, ErrSessionTerminalDetached
	default:
	}
	return h.writer.Write(data)
}

func (h *terminalHub) WriteInput(data []byte) (int, error) {
	if h == nil || len(data) == 0 {
		return 0, nil
	}
	select {
	case <-h.closed:

		return 0, ErrSessionTerminalDetached
	default:
	}
	count, err := h.writer.Write(data)
	if count > 0 {
		h.recordInput(data[:count])
	}
	return count, err
}

func (h *terminalHub) waitTerminal() {
	err := h.raw.Wait()
	h.mu.Lock()
	h.waitErr = err

	for _, recorder := range h.recorders {
		if completer, ok := recorder.(interface{ MarkCompleted() }); ok {
			completer.MarkCompleted()
		}
	}
	h.mu.Unlock()
	close(h.waitDone)
}

func (h *terminalHub) detach(id uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.current == nil || h.current.id != id {
		return
	}
	h.current.detach()
	h.current = nil

	h.lastDetachAt = h.attachmentNow()
	h.signalLocked()
}

func (h *terminalHub) signalLocked() {
	close(h.wake)
	h.wake = make(chan struct{})
}

func (h *terminalHub) Close() error {
	if h == nil {
		return nil
	}
	h.closeOnce.Do(func() {
		h.mu.Lock()
		recorders := h.recorderSnapshotLocked()
		h.recorders = make(map[string]TerminalRecorder)
		if h.current != nil {
			h.current.detach()
			h.current = nil

			h.lastDetachAt = h.attachmentNow()
		}
		close(h.closed)
		h.signalLocked()
		h.mu.Unlock()
		h.closeErr = h.raw.Close()
		<-h.readDone
		<-h.waitDone
		for _, recorder := range recorders {
			_ = recorder.Close()
		}
	})
	return h.closeErr
}

type terminalAttachment struct {
	hub        *terminalHub
	id         uint64
	output     chan terminalReadResult
	closed     chan struct{}
	closeOnce  sync.Once
	pending    []byte
	pendingErr error

	historyStart uint64
}

func (a *terminalAttachment) Read(target []byte) (int, error) {
	if len(target) == 0 {
		return 0, nil
	}
	if len(a.pending) > 0 {
		count := copy(target, a.pending)
		a.pending = a.pending[count:]
		if len(a.pending) == 0 && a.pendingErr != nil {
			err := a.pendingErr
			a.pendingErr = nil
			return count, err
		}
		return count, nil
	}
	select {
	case result := <-a.output:
		count := copy(target, result.data)
		if count < len(result.data) {
			a.pending = append(a.pending[:0], result.data[count:]...)
			a.pendingErr = result.err
			return count, nil
		}
		return count, result.err
	case <-a.closed:
		return 0, ErrSessionTerminalDetached
	}
}

func (a *terminalAttachment) Write(value []byte) (int, error) {
	if !a.current() {
		return 0, ErrSessionTerminalDetached
	}
	count, err := a.hub.writer.Write(value)
	if count > 0 {
		a.hub.recordInput(value[:count])
	}
	return count, err
}

func (a *terminalAttachment) Resize(columns int, rows int) error {
	if !a.current() {
		return ErrSessionTerminalDetached
	}
	if err := a.hub.raw.Resize(columns, rows); err != nil {
		return err
	}
	a.hub.mu.Lock()
	recorders := a.hub.recorderSnapshotLocked()
	a.hub.mu.Unlock()
	for key, recorder := range recorders {
		if err := recorder.Resize(columns, rows); err != nil {
			a.hub.removeRecorder(key, recorder)
		}
	}
	return nil
}

func (a *terminalAttachment) Wait() error {
	select {
	case <-a.closed:
		return ErrSessionTerminalDetached
	case <-a.hub.waitDone:
		a.hub.mu.Lock()
		err := a.hub.waitErr
		a.hub.mu.Unlock()
		return err
	}
}

func (a *terminalAttachment) Close() error {
	if a == nil || a.hub == nil {
		return nil
	}
	a.hub.detach(a.id)
	a.detach()
	return nil
}

func (a *terminalAttachment) Activate() error {
	if a == nil || a.hub == nil {
		return ErrSessionTerminalDetached
	}
	a.hub.mu.Lock()
	defer a.hub.mu.Unlock()
	select {
	case <-a.closed:
		return ErrSessionTerminalDetached
	case <-a.hub.closed:
		return ErrSessionTerminalDetached
	default:
	}
	if a.hub.current != nil && a.hub.current != a {
		a.hub.current.detach()
	}
	a.hub.current = a

	a.hub.attachEpoch++
	a.hub.attachedAtLeastOnce = true
	a.hub.signalLocked()
	return nil
}

func (a *terminalAttachment) current() bool {
	if a == nil || a.hub == nil {
		return false
	}
	a.hub.mu.Lock()
	defer a.hub.mu.Unlock()
	return a.hub.current == a
}

func (a *terminalAttachment) detach() {
	a.closeOnce.Do(func() { close(a.closed) })
}

var _ SessionTerminal = (*terminalAttachment)(nil)
