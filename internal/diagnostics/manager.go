package diagnostics

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	logFilename   = "qterm-debug.log"
	stateFilename = "diagnostics.json"
	maxReadBytes  = 4 * 1024 * 1024
	maxLogBytes   = 8 * 1024 * 1024
)

type Mode string

const (
	ModeOff    Mode = "off"
	ModeNormal Mode = "normal"
	ModeDebug  Mode = "debug"
)

func ParseMode(value string) (Mode, bool) {
	mode := Mode(strings.ToLower(strings.TrimSpace(value)))
	switch mode {
	case ModeOff, ModeNormal, ModeDebug:
		return mode, true
	default:
		return "", false
	}
}

func validMode(mode Mode) bool {
	return mode == ModeOff || mode == ModeNormal || mode == ModeDebug
}

type persistedState struct {
	Mode    string `json:"mode,omitempty"`
	Enabled *bool  `json:"enabled,omitempty"`
}

type Status struct {
	Mode       Mode
	Path       string
	SizeBytes  int64
	LimitBytes int64
	Dropped    uint64
	Present    bool
}

type Manager struct {
	mu        sync.Mutex
	directory string
	logPath   string
	statePath string
	mode      Mode
	closed    bool
	file      *os.File
}

func NewManager(root string) (*Manager, error) {
	directory := filepath.Join(root, "logs")
	logPath := filepath.Join(directory, logFilename)
	if abs, err := filepath.Abs(logPath); err == nil {
		logPath = abs
	}
	manager := &Manager{
		directory: directory,
		logPath:   logPath,
		statePath: filepath.Join(directory, stateFilename),
		mode:      ModeNormal,
	}

	if mode, err := manager.loadMode(); err == nil {
		manager.mode = mode
	}
	if manager.mode != ModeOff {

		_ = manager.openLocked()
	}
	return manager, nil
}

func (m *Manager) loadMode() (Mode, error) {
	data, err := os.ReadFile(m.statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ModeNormal, nil
		}
		return "", err
	}
	var state persistedState
	if json.Unmarshal(data, &state) != nil {
		return ModeNormal, nil
	}
	if mode, ok := ParseMode(state.Mode); ok {
		return mode, nil
	}
	if state.Enabled != nil && *state.Enabled {
		return ModeDebug, nil
	}
	return ModeNormal, nil
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

func (m *Manager) Mode() Mode {
	if m == nil {
		return ModeOff
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.mode
}

func (m *Manager) Enabled() bool {
	return m.Mode() != ModeOff
}

func (m *Manager) SetMode(mode Mode) error {
	if m == nil {
		return errors.New("调试日志未配置")
	}
	if !validMode(mode) {
		return errors.New("未知的调试日志档位")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("调试日志已经关闭")
	}
	if mode == m.mode {

		if mode != ModeOff && m.file == nil {
			return m.openLocked()
		}
		return nil
	}
	openedHere := false
	if mode != ModeOff && m.file == nil {
		if err := m.openLocked(); err != nil {
			return err
		}
		openedHere = true
	}
	if err := m.persistLocked(mode); err != nil {
		if openedHere && m.file != nil {
			_ = m.file.Close()
			m.file = nil
		}
		return err
	}
	m.mode = mode
	if mode == ModeOff && m.file != nil {
		err := m.file.Close()
		m.file = nil
		return err
	}
	return nil
}

func (m *Manager) Write(data []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.mode == ModeOff || m.closed {
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
	if m.mode != ModeOff {
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

func (m *Manager) Status() Status {
	if m == nil {
		return Status{Mode: ModeOff, LimitBytes: maxLogBytes}
	}
	m.mu.Lock()
	mode := m.mode
	path := m.logPath
	m.mu.Unlock()
	status := Status{Mode: mode, Path: path, LimitBytes: maxLogBytes}
	if info, err := os.Stat(path); err == nil {
		status.Present = true
		status.SizeBytes = info.Size()
	}
	if installedHandler != nil {
		status.Dropped = installedHandler.Dropped()
	}
	return status
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

func (m *Manager) persistLocked(mode Mode) error {
	if err := os.MkdirAll(m.directory, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(persistedState{Mode: string(mode)})
	if err != nil {
		return err
	}
	return os.WriteFile(m.statePath, data, 0o600)
}
