package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

const (
	logFilename   = "qterm-debug.log"
	stateFilename = "diagnostics.json"
	maxReadBytes  = 4 * 1024 * 1024
	maxLogBytes   = 8 * 1024 * 1024
)

type persistedState struct {
	Enabled bool `json:"enabled"`
}

type Manager struct {
	mu        sync.Mutex
	directory string
	logPath   string
	statePath string
	enabled   bool
	closed    bool
	file      *os.File
}

func NewManager(root string) (*Manager, error) {
	directory := filepath.Join(root, "logs")
	manager := &Manager{
		directory: directory,
		logPath:   filepath.Join(directory, logFilename),
		statePath: filepath.Join(directory, stateFilename),
	}
	data, err := os.ReadFile(manager.statePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if len(data) > 0 {
		var state persistedState
		if json.Unmarshal(data, &state) == nil {
			manager.enabled = state.Enabled
		}
	}
	if manager.enabled {
		if err := manager.openLocked(); err != nil {
			return nil, err
		}
	}
	return manager, nil
}

func (m *Manager) Install() {
	if m == nil {
		return
	}
	fileHandler := slog.NewTextHandler(m, &slog.HandlerOptions{
		Level:       slog.LevelDebug,
		ReplaceAttr: normalizeLogAttr,
	})

	async := newAsyncHandler(&schemaHandler{
		inner: &switchHandler{file: fileHandler, manager: m},
	})
	installedHandler = async
	slog.SetDefault(slog.New(async))
}

func (m *Manager) Enabled() bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled
}

func (m *Manager) SetEnabled(enabled bool) error {
	if m == nil {
		return errors.New("调试日志未配置")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("调试日志已经关闭")
	}
	if enabled == m.enabled {
		return nil
	}
	if enabled {
		if err := m.openLocked(); err != nil {
			return err
		}
	}
	if err := m.persistLocked(enabled); err != nil {
		if enabled && m.file != nil {
			_ = m.file.Close()
			m.file = nil
		}
		return err
	}
	m.enabled = enabled
	if !enabled && m.file != nil {
		err := m.file.Close()
		m.file = nil
		return err
	}
	return nil
}

func (m *Manager) Write(data []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.enabled || m.closed {
		return len(data), nil
	}
	if m.file == nil {
		if err := m.openLocked(); err != nil {
			return 0, err
		}
	}
	if info, err := m.file.Stat(); err == nil && info.Size()+int64(len(data)) > maxLogBytes {
		if err := m.file.Truncate(0); err != nil {
			return 0, err
		}
		if _, err := m.file.Seek(0, io.SeekStart); err != nil {
			return 0, err
		}
	}
	return m.file.Write(data)
}

func (m *Manager) Read() (string, error) {
	if m == nil {
		return "", errors.New("调试日志未配置")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	file, err := os.Open(m.logPath)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	start := info.Size() - maxReadBytes
	if start < 0 {
		start = 0
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return "", err
	}
	data, err := io.ReadAll(file)
	return string(data), err
}

func (m *Manager) Clear() error {
	if m == nil {
		return errors.New("调试日志未配置")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.file != nil {
		if err := m.file.Close(); err != nil {
			return err
		}
		m.file = nil
	}
	if err := os.MkdirAll(m.directory, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(m.logPath, nil, 0o600); err != nil {
		return err
	}
	if m.enabled {
		return m.openLocked()
	}
	return nil
}

func (m *Manager) Export(target string) error {
	data, err := m.Read()
	if err != nil {
		return err
	}
	return os.WriteFile(target, []byte(data), 0o600)
}

func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.file == nil {
		m.closed = true
		return nil
	}
	err := m.file.Close()
	m.file = nil
	m.closed = true
	return err
}

func (m *Manager) openLocked() error {
	if m.file != nil {
		return nil
	}
	if err := os.MkdirAll(m.directory, 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(m.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	m.file = file
	return nil
}

func (m *Manager) persistLocked(enabled bool) error {
	if err := os.MkdirAll(m.directory, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(persistedState{Enabled: enabled})
	if err != nil {
		return err
	}
	return os.WriteFile(m.statePath, data, 0o600)
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
	return h.manager.Enabled() && h.file.Enabled(ctx, level)
}

func (h *switchHandler) Handle(ctx context.Context, record slog.Record) error {
	var firstErr error
	if h.base != nil && h.base.Enabled(ctx, record.Level) {
		firstErr = h.base.Handle(ctx, record)
	}
	if h.manager.Enabled() && h.file.Enabled(ctx, record.Level) {
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
