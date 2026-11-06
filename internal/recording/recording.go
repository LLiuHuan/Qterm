package recording

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	fileMagic       = "QTR1"
	fileVersion     = byte(1)
	headerSize      = 16
	maxFramePayload = 4 << 20

	EventOutput = byte(1)
	EventResize = byte(2)
	EventEnd    = byte(3)
	EventInput  = byte(4)
)

type Event struct {
	At      time.Duration
	Type    byte
	Columns uint16
	Rows    uint16
	Data    []byte
}

type Writer struct {
	mu        sync.Mutex
	file      *os.File
	startedAt time.Time
	closed    bool
	bytes     int64
	events    int64
	queue     []Event
	sequence  int64
	paused    bool
}

func Create(path string, startedAt time.Time) (*Writer, error) {
	if path == "" {
		return nil, errors.New("recording path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create recording directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create recording file: %w", err)
	}
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	var header [headerSize]byte
	copy(header[:4], fileMagic)
	header[4] = fileVersion
	binary.BigEndian.PutUint64(header[8:16], uint64(startedAt.UnixNano()))
	if _, err := file.Write(header[:]); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("write recording header: %w", err)
	}
	return &Writer{file: file, startedAt: startedAt}, nil
}

func (w *Writer) Append(event Event) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.append(event)
}

func (w *Writer) append(event Event) error {
	if w == nil || w.file == nil || w.closed {
		return errors.New("recording writer is closed")
	}
	if event.Type != EventOutput && event.Type != EventResize && event.Type != EventEnd && event.Type != EventInput {
		return errors.New("recording event type is invalid")
	}
	if len(event.Data) > maxFramePayload {
		return fmt.Errorf("recording event payload exceeds %d bytes", maxFramePayload)
	}

	frame := make([]byte, 8+1+2+2+4+len(event.Data))
	binary.BigEndian.PutUint64(frame[0:8], uint64(event.At))
	frame[8] = event.Type
	binary.BigEndian.PutUint16(frame[9:11], event.Columns)
	binary.BigEndian.PutUint16(frame[11:13], event.Rows)
	binary.BigEndian.PutUint32(frame[13:17], uint32(len(event.Data)))
	copy(frame[17:], event.Data)
	payload := make([]byte, 4+len(frame)+4)
	binary.BigEndian.PutUint32(payload[0:4], uint32(len(frame)+4))
	copy(payload[4:], frame)
	binary.BigEndian.PutUint32(payload[4+len(frame):], crc32.ChecksumIEEE(frame))
	if _, err := w.file.Write(payload); err != nil {
		return fmt.Errorf("write recording frame: %w", err)
	}
	w.bytes += int64(len(payload))
	w.events++
	if event.Type != EventEnd {
		w.sequence++
		w.queue = append(w.queue, Event{At: event.At, Type: event.Type, Columns: event.Columns, Rows: event.Rows, Data: append([]byte(nil), event.Data...)})
		if len(w.queue) > 2048 {
			w.queue = w.queue[len(w.queue)-2048:]
		}
	}
	return nil
}

func (w *Writer) EventsAfter(cursor int64) (int64, []Event, bool) {
	if w == nil {
		return cursor, nil, false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	start := w.sequence - int64(len(w.queue))
	truncated := cursor < start
	if truncated {
		cursor = start
	}
	offset := int(cursor - start)
	if offset < 0 {
		offset = 0
	}
	if offset > len(w.queue) {
		offset = len(w.queue)
	}
	items := append([]Event(nil), w.queue[offset:]...)
	return w.sequence, items, truncated
}

func (w *Writer) Bytes() int64 {
	if w == nil {
		return 0
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.bytes
}
func (w *Writer) Events() int64 {
	if w == nil {
		return 0
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.events
}

func (w *Writer) Output(data []byte) error {
	w.mu.Lock()
	paused := w.paused
	w.mu.Unlock()
	if paused {
		return nil
	}
	return w.Append(Event{At: time.Since(w.startedAt), Type: EventOutput, Data: data})
}

func (w *Writer) Input(data []byte) error {
	w.mu.Lock()
	paused := w.paused
	w.mu.Unlock()
	if paused {
		return nil
	}
	return w.Append(Event{At: time.Since(w.startedAt), Type: EventInput, Data: data})
}

func (w *Writer) Resize(columns, rows int) error {
	w.mu.Lock()
	paused := w.paused
	w.mu.Unlock()
	if paused {
		return nil
	}
	if columns < 0 || columns > 65535 || rows < 0 || rows > 65535 {
		return errors.New("recording terminal size is invalid")
	}
	return w.Append(Event{At: time.Since(w.startedAt), Type: EventResize, Columns: uint16(columns), Rows: uint16(rows)})
}

func (w *Writer) SetPaused(paused bool) {
	if w == nil {
		return
	}
	w.mu.Lock()
	w.paused = paused
	w.mu.Unlock()
}

func (w *Writer) Close() error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	var err error
	if appendErr := w.append(Event{At: time.Since(w.startedAt), Type: EventEnd}); appendErr != nil {
		err = appendErr
	}
	w.closed = true
	if syncErr := w.file.Sync(); err == nil && syncErr != nil {
		err = fmt.Errorf("sync recording file: %w", syncErr)
	}
	if closeErr := w.file.Close(); err == nil && closeErr != nil {
		err = fmt.Errorf("close recording file: %w", closeErr)
	}
	return err
}

type Reader struct {
	file      *os.File
	buf       *bufio.Reader
	startedAt time.Time
}

func Open(path string) (*Reader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open recording file: %w", err)
	}
	var header [headerSize]byte
	if _, err := io.ReadFull(file, header[:]); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("read recording header: %w", err)
	}
	if string(header[:4]) != fileMagic || header[4] != fileVersion {
		_ = file.Close()
		return nil, errors.New("recording header is invalid")
	}
	return &Reader{file: file, buf: bufio.NewReader(file), startedAt: time.Unix(0, int64(binary.BigEndian.Uint64(header[8:16]))).UTC()}, nil
}

func (r *Reader) StartedAt() time.Time { return r.startedAt }

func (r *Reader) Next() (Event, error) {
	var lengthBytes [4]byte
	if _, err := io.ReadFull(r.buf, lengthBytes[:]); err != nil {
		return Event{}, err
	}
	length := binary.BigEndian.Uint32(lengthBytes[:])
	if length < 8+1+2+2+4+4 || length > maxFramePayload+21 {
		return Event{}, errors.New("recording frame length is invalid")
	}
	frame := make([]byte, length)
	if _, err := io.ReadFull(r.buf, frame); err != nil {
		return Event{}, fmt.Errorf("read recording frame: %w", err)
	}
	if crc32.ChecksumIEEE(frame[:length-4]) != binary.BigEndian.Uint32(frame[length-4:]) {
		return Event{}, errors.New("recording frame checksum mismatch")
	}
	dataLen := binary.BigEndian.Uint32(frame[13:17])
	if uint64(dataLen)+21 != uint64(length) {
		return Event{}, errors.New("recording frame payload length is invalid")
	}
	event := Event{At: time.Duration(binary.BigEndian.Uint64(frame[:8])), Type: frame[8], Columns: binary.BigEndian.Uint16(frame[9:11]), Rows: binary.BigEndian.Uint16(frame[11:13])}
	event.Data = append([]byte(nil), frame[17:17+dataLen]...)
	return event, nil
}

func (r *Reader) Close() error {
	if r == nil || r.file == nil {
		return nil
	}
	return r.file.Close()
}
