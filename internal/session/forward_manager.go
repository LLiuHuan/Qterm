package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"changeme/internal/model"
	"changeme/internal/tunnel"
)

const ForwardStatusEvent = "qterm:forward-status"

var forwardRetryDelays = []time.Duration{time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second}

type ForwardManager struct {
	mu       sync.Mutex
	sessions *SessionManager
	entries  map[string]*forwardEntry
	emit     func(model.ForwardStatus)
}

type forwardEntry struct {
	config    model.ForwardConfig
	cancel    context.CancelFunc
	stopped   chan struct{}
	sessionID string
	forward   *tunnel.Forward
	retry     int

	state   model.ForwardState
	port    int
	lastErr string
}

func NewForwardManager(sessions *SessionManager, emit func(model.ForwardStatus)) *ForwardManager {
	return &ForwardManager{
		sessions: sessions,
		entries:  make(map[string]*forwardEntry),
		emit:     emit,
	}
}

func (m *ForwardManager) SetEmitter(emit func(model.ForwardStatus)) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.emit = emit
	m.mu.Unlock()
}

func (m *ForwardManager) Open(cfg model.ForwardConfig) error {
	if err := validateForwardConfig(cfg); err != nil {
		return err
	}
	m.mu.Lock()
	if _, exists := m.entries[cfg.ID]; exists {
		m.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	entry := &forwardEntry{config: cfg, cancel: cancel, stopped: make(chan struct{})}
	m.entries[cfg.ID] = entry
	m.mu.Unlock()

	go m.run(ctx, entry)
	return nil
}

func (m *ForwardManager) Close(id string) error {
	m.mu.Lock()
	entry, exists := m.entries[id]
	if exists {
		delete(m.entries, id)
	}
	m.mu.Unlock()
	if !exists {
		return nil
	}
	entry.cancel()
	<-entry.stopped
	m.publish(entry, model.ForwardStatus{ID: id, State: model.ForwardStopped})
	return nil
}

func (m *ForwardManager) CloseAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.entries))
	for id := range m.entries {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		_ = m.Close(id)
	}
}

func (m *ForwardManager) ActiveCount() int {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, entry := range m.entries {
		if entry.state == model.ForwardRunning || entry.state == model.ForwardReconnecting {
			count++
		}
	}
	return count
}

func (m *ForwardManager) Status(id string) model.ForwardStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, exists := m.entries[id]
	if !exists {
		return model.ForwardStatus{ID: id, State: model.ForwardStopped}
	}
	return model.ForwardStatus{
		ID:           id,
		State:        entry.state,
		Error:        entry.lastErr,
		ListenPort:   entry.port,
		RetryAttempt: entry.retry,
		UpdatedAt:    time.Now().UnixMilli(),
	}
}

func (m *ForwardManager) run(ctx context.Context, entry *forwardEntry) {
	defer close(entry.stopped)
	id := entry.config.ID
	retry := 0
	for {
		if ctx.Err() != nil {
			return
		}
		m.publish(entry, model.ForwardStatus{ID: id, State: model.ForwardStarting, RetryAttempt: retry})
		started, err := m.attempt(ctx, entry)
		if ctx.Err() != nil {
			return
		}
		if started {

			retry = 0
		}
		retry++
		slog.Debug("forward attempt ended",
			"event", "forward.attempt.ended",
			"forward", id,
			"attempt", retry,
			"started", started,
			"error", errText(err),
		)
		m.publish(entry, model.ForwardStatus{
			ID:           id,
			State:        model.ForwardReconnecting,
			Error:        errText(err),
			RetryAttempt: retry,
		})
		if !waitForwardRetry(ctx, retry) {
			return
		}
	}
}

func (m *ForwardManager) attempt(ctx context.Context, entry *forwardEntry) (bool, error) {
	session, err := m.sessions.OpenForSftp(ctx, entry.config.HostID)
	if err != nil {
		return false, fmt.Errorf("连接主机失败: %w", err)
	}
	entry.sessionID = session.ID
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = m.sessions.Close(closeCtx, session.ID)
		entry.sessionID = ""
	}()

	connection, err := m.connectionOf(ctx, session.ID)
	if err != nil {
		return false, err
	}
	spec := tunnel.ForwardSpec{
		Kind:       tunnel.ForwardKind(entry.config.Kind),
		ListenHost: entry.config.ListenHost,
		ListenPort: entry.config.ListenPort,
		TargetHost: entry.config.TargetHost,
		TargetPort: entry.config.TargetPort,
	}
	forward, err := connection.OpenForward(ctx, spec)
	if err != nil {
		return false, err
	}
	entry.forward = forward
	m.publish(entry, model.ForwardStatus{
		ID:         entry.config.ID,
		State:      model.ForwardRunning,
		ListenPort: listenPortOf(forward, entry.config.ListenPort),
	})
	select {
	case <-ctx.Done():
		_ = forward.Close()
		return true, nil
	case <-forward.Done():
		err := forward.Err()
		_ = forward.Close()
		if err != nil {
			return true, err
		}
		return true, errors.New("转发已停止")
	}
}

func (m *ForwardManager) connectionOf(ctx context.Context, id string) (*tunnel.Connection, error) {
	raw, err := m.readyConnection(ctx, id)
	if err != nil {
		return nil, err
	}
	connection, ok := raw.(*productionSessionConnection)
	if !ok || connection.connection == nil {
		return nil, errors.New("该会话不支持端口转发")
	}
	return connection.connection, nil
}

func (m *ForwardManager) readyConnection(ctx context.Context, id string) (SessionConnection, error) {
	worker, err := m.sessions.lookup(ctx, id)
	if err != nil {
		return nil, err
	}
	select {
	case <-worker.ready:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	worker.mu.RLock()
	raw := worker.connection
	worker.mu.RUnlock()
	if raw == nil {
		return nil, errors.New("会话未连接成功")
	}
	return raw, nil
}

func (m *ForwardManager) publish(entry *forwardEntry, status model.ForwardStatus) {
	if status.UpdatedAt == 0 {
		status.UpdatedAt = time.Now().UnixMilli()
	}
	m.mu.Lock()
	if entry != nil {
		entry.state = status.State
		entry.lastErr = status.Error
		entry.retry = status.RetryAttempt
		if status.ListenPort > 0 {
			entry.port = status.ListenPort
		}
	}
	emit := m.emit
	m.mu.Unlock()
	if emit != nil {
		emit(status)
	}
}

func waitForwardRetry(ctx context.Context, attempt int) bool {
	index := attempt - 1
	if index < 0 {
		index = 0
	}
	if index >= len(forwardRetryDelays) {
		index = len(forwardRetryDelays) - 1
	}
	timer := time.NewTimer(forwardRetryDelays[index])
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func listenPortOf(forward *tunnel.Forward, fallback int) int {
	if forward == nil {
		return fallback
	}
	addr := forward.Addr()
	if addr == nil {
		return fallback
	}
	if tcp, ok := addr.(interface{ Port() int }); ok {
		return tcp.Port()
	}
	return fallback
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func validateForwardConfig(cfg model.ForwardConfig) error {
	if cfg.ID == "" {
		return errors.New("转发缺少 ID")
	}
	if cfg.HostID == "" {
		return errors.New("请选择使用哪台主机")
	}
	switch cfg.Kind {
	case model.ForwardLocal, model.ForwardRemote, model.ForwardDynamic:
	default:
		return fmt.Errorf("不支持的转发类型 %q", cfg.Kind)
	}
	if cfg.ListenPort < 0 || cfg.ListenPort > 65535 {
		return errors.New("监听端口超出范围")
	}
	if cfg.Kind != model.ForwardDynamic {
		if cfg.TargetHost == "" {
			return errors.New("请填写目标地址")
		}
		if cfg.TargetPort <= 0 || cfg.TargetPort > 65535 {
			return errors.New("目标端口超出范围")
		}
	}
	return nil
}
