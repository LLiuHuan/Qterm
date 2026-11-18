package diagnostics

import (
	"context"
	"log/slog"
)

func modeAllows(mode Mode, level slog.Level) bool {
	switch mode {
	case ModeOff:
		return false
	case ModeNormal:
		return level >= slog.LevelWarn
	case ModeDebug:
		return true
	default:
		return false
	}
}

type switchHandler struct {
	base    slog.Handler
	file    slog.Handler
	manager *Manager
}

func (h *switchHandler) Enabled(ctx context.Context, level slog.Level) bool {

	if h.base != nil && h.base.Enabled(ctx, level) {
		return true
	}
	if !modeAllows(h.manager.Mode(), level) {
		return false
	}
	return h.file.Enabled(ctx, level)
}

func (h *switchHandler) Handle(ctx context.Context, record slog.Record) error {
	var firstErr error
	if h.base != nil && h.base.Enabled(ctx, record.Level) {
		firstErr = h.base.Handle(ctx, record)
	}

	if modeAllows(h.manager.Mode(), record.Level) && h.file.Enabled(ctx, record.Level) {
		if err := h.file.Handle(ctx, record); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (h *switchHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := &switchHandler{file: h.file.WithAttrs(attrs), manager: h.manager}
	if h.base != nil {
		next.base = h.base.WithAttrs(attrs)
	}
	return next
}

func (h *switchHandler) WithGroup(name string) slog.Handler {
	next := &switchHandler{file: h.file.WithGroup(name), manager: h.manager}
	if h.base != nil {
		next.base = h.base.WithGroup(name)
	}
	return next
}
