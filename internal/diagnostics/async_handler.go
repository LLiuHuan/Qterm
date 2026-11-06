package diagnostics

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

type asyncSink struct {
	queue   chan slog.Record
	drops   atomic.Uint64
	pending atomic.Int64
}

type asyncHandler struct {
	sink *asyncSink
	next slog.Handler
}

const asyncQueueSize = 1024

var installedHandler *asyncHandler

func newAsyncHandler(next slog.Handler) *asyncHandler {
	if next == nil {
		return nil
	}
	sink := &asyncSink{queue: make(chan slog.Record, asyncQueueSize)}
	handler := &asyncHandler{sink: sink, next: next}
	go sink.run(next)
	return handler
}

func (s *asyncSink) run(handler slog.Handler) {
	ctx := context.Background()
	for record := range s.queue {
		_ = handler.Handle(ctx, record)
		s.pending.Add(-1)
	}
}

func Flush(timeout time.Duration) bool {
	if installedHandler == nil {
		return true
	}
	return installedHandler.Flush(timeout)
}

func (h *asyncHandler) Flush(timeout time.Duration) bool {
	if h == nil || h.sink == nil {
		return true
	}
	deadline := time.Now().Add(timeout)
	for {
		if h.sink.pending.Load() == 0 {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func (h *asyncHandler) Dropped() uint64 {
	if h == nil || h.sink == nil {
		return 0
	}
	return h.sink.drops.Load()
}

func (h *asyncHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *asyncHandler) Handle(_ context.Context, record slog.Record) error {

	select {
	case h.sink.queue <- record.Clone():
		h.sink.pending.Add(1)
	default:
		h.sink.drops.Add(1)
	}
	return nil
}

func (h *asyncHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &asyncHandler{sink: h.sink, next: h.next.WithAttrs(attrs)}
}

func (h *asyncHandler) WithGroup(name string) slog.Handler {
	return &asyncHandler{sink: h.sink, next: h.next.WithGroup(name)}
}
