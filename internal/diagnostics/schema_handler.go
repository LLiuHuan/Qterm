package diagnostics

import (
	"context"
	"log/slog"
	"strings"
)

type schemaHandler struct {
	inner slog.Handler
}

func (h *schemaHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *schemaHandler) Handle(ctx context.Context, record slog.Record) error {
	var event, component string
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key != "event" {
			return true
		}
		event = strings.TrimSpace(attr.Value.String())
		if index := strings.IndexByte(event, '.'); index > 0 {
			component = event[:index]
		}
		return false
	})
	if component == "" {

		component = "app"
	}
	enriched := record.Clone()
	enriched.AddAttrs(slog.String("component", component))
	return h.inner.Handle(ctx, enriched)
}

func (h *schemaHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &schemaHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *schemaHandler) WithGroup(name string) slog.Handler {
	return &schemaHandler{inner: h.inner.WithGroup(name)}
}

func normalizeLogAttr(groups []string, attr slog.Attr) slog.Attr {
	if len(groups) != 0 {
		return attr
	}
	switch attr.Key {
	case slog.TimeKey:

		return slog.String(slog.TimeKey, attr.Value.Time().Format("2006-01-02T15:04:05.000Z07:00"))
	case slog.LevelKey:
		if level, ok := attr.Value.Any().(slog.Level); ok {
			return slog.String(slog.LevelKey, strings.ToUpper(level.String()))
		}
	}
	return attr
}
