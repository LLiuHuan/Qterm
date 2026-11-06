package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"changeme/internal/model"
	sshclient "changeme/internal/tunnel"
)

var (
	ErrSessionManagerNotReady       = errors.New("session manager is not ready")
	ErrSessionNotFound              = errors.New("session not found")
	ErrSessionManagerClosed         = errors.New("session manager is closed")
	ErrSessionNotRetryable          = errors.New("session is not retryable")
	ErrSessionNotConnected          = errors.New("session is not connected")
	ErrSessionTerminalAttached      = errors.New("session terminal is already attached")
	ErrSessionTerminalUnsupported   = errors.New("session connection does not support terminals")
	ErrSessionFileSystemUnsupported = errors.New("session connection does not support SFTP")
	ErrSessionLocalUnsupported      = errors.New("local sessions are not supported")

	ErrDockerUnsupported = errors.New("session connection does not support docker commands")
)

const (
	orphanGracePeriod = 10 * time.Second

	orphanSweepInterval = time.Second

	orphanCloseTimeout = 5 * time.Second
)

type PendingSessionError struct {
	State model.SessionState
	Err   error
}

func (e *PendingSessionError) Error() string {
	if e == nil {
		return "session requires user action"
	}
	return fmt.Sprintf("session requires user action: %s", e.State)
}

func (e *PendingSessionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type SessionManager struct {
	mu             sync.RWMutex
	connector      SessionConnector
	localConnector SessionConnector
	now            func() time.Time
	newID          func() (string, error)
	rootCtx        context.Context
	cancel         context.CancelFunc
	sessions       map[string]*managedSession
	closed         bool
	wg             sync.WaitGroup

	sweepOnce sync.Once

	orphanFinalizer func(sessionID string)
}

func NewSessionManager(connector SessionConnector, localConnector SessionConnector) *SessionManager {
	return newSessionManager(connector, localConnector, time.Now, randomSessionID)
}

func newSessionManager(
	connector SessionConnector,
	localConnector SessionConnector,
	now func() time.Time,
	newID func() (string, error),
) *SessionManager {
	rootCtx, cancel := context.WithCancel(context.Background())
	return &SessionManager{
		connector:      connector,
		localConnector: localConnector,
		now:            now,
		newID:          newID,
		rootCtx:        rootCtx,
		cancel:         cancel,
		sessions:       make(map[string]*managedSession),
	}
}

func (m *SessionManager) Open(ctx context.Context, hostID string) (model.Session, error) {
	if err := m.ready(ctx); err != nil {
		return model.Session{}, err
	}
	return m.OpenWith(ctx, hostID, m.connector)
}

func (m *SessionManager) OpenWith(
	ctx context.Context,
	hostID string,
	connector SessionConnector,
) (model.Session, error) {
	if err := m.ready(ctx); err != nil {
		return model.Session{}, err
	}
	if connector == nil {
		return model.Session{}, ErrSessionManagerNotReady
	}
	id, err := m.newID()
	if err != nil {
		return model.Session{}, fmt.Errorf("generate session ID: %w", err)
	}
	var snapshot model.Session
	if provider, ok := connector.(sessionKindProvider); ok &&
		provider.SessionKind() == model.SessionKindTelnet {
		snapshot, err = model.NewTelnetSession(id, hostID, m.now())
	} else {
		snapshot, err = model.NewSession(id, hostID, m.now())
	}
	if err != nil {
		return model.Session{}, err
	}
	return m.start(snapshot, connector)
}

func (m *SessionManager) OpenForSftp(ctx context.Context, hostID string) (model.Session, error) {
	if err := m.ready(ctx); err != nil {
		return model.Session{}, err
	}
	id, err := m.newID()
	if err != nil {
		return model.Session{}, fmt.Errorf("generate session ID: %w", err)
	}
	snapshot, err := model.NewSftpSession(id, hostID, m.now())
	if err != nil {
		return model.Session{}, err
	}
	return m.start(snapshot, m.connector)
}

func (m *SessionManager) OpenLocal(ctx context.Context, plan sshclient.LocalShellPlan) (model.Session, error) {
	if err := m.ready(ctx); err != nil {
		return model.Session{}, err
	}

	if m.localConnector == nil {
		return model.Session{}, ErrSessionLocalUnsupported
	}
	id, err := m.newID()
	if err != nil {
		return model.Session{}, fmt.Errorf("generate session ID: %w", err)
	}
	snapshot, err := model.NewLocalSession(id, m.now())
	if err != nil {
		return model.Session{}, err
	}
	return m.start(snapshot, NewLocalConnector(plan))
}

func (m *SessionManager) start(snapshot model.Session, connector SessionConnector) (model.Session, error) {
	if connector == nil {
		return model.Session{}, ErrSessionManagerNotReady
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return model.Session{}, ErrSessionManagerClosed
	}
	workerCtx, cancel := context.WithCancel(m.rootCtx)
	worker := &managedSession{
		snapshot:  snapshot,
		connector: connector,
		ctx:       workerCtx,
		cancel:    cancel,
		ready:     make(chan struct{}),
		done:      make(chan struct{}),
		now:       m.now,
	}
	m.sessions[snapshot.ID] = worker

	m.startOrphanSweepLocked()
	m.wg.Add(1)
	m.mu.Unlock()

	go m.run(worker)
	return snapshot, nil
}

func (m *SessionManager) OpenTerminal(
	ctx context.Context,
	id string,
	columns int,
	rows int,
) (SessionTerminal, error) {
	worker, err := m.lookup(ctx, id)
	if err != nil {
		return nil, err
	}
	return worker.openTerminal(ctx, columns, rows)
}

func (m *SessionManager) SetTerminalRecorder(ctx context.Context, id string, recorder TerminalRecorder) error {
	return m.SetTerminalRecorderFor(ctx, id, "segment", recorder)
}

func (m *SessionManager) SetTerminalRecorderFor(ctx context.Context, id, key string, recorder TerminalRecorder) error {
	worker, err := m.lookup(ctx, id)
	if err != nil {
		return err
	}
	return worker.setTerminalRecorderFor(key, recorder)
}

func (m *SessionManager) FileSystem(ctx context.Context, id string) (SessionFileSystem, error) {
	worker, err := m.lookup(ctx, id)
	if err != nil {
		return nil, err
	}
	return worker.openFileSystem(ctx)
}

func (m *SessionManager) RequireTerminal(ctx context.Context, id string) error {
	worker, err := m.lookup(ctx, id)
	if err != nil {
		return err
	}
	return worker.requireTerminal()
}

func (m *SessionManager) WriteSensitiveInput(ctx context.Context, id string, data []byte) error {
	worker, err := m.lookup(ctx, id)
	if err != nil {
		return err
	}
	return worker.writeSensitive(data)
}

func (m *SessionManager) WriteTerminalInput(ctx context.Context, id string, data []byte) error {
	worker, err := m.lookup(ctx, id)
	if err != nil {
		return err
	}
	return worker.writeInput(data)
}

func (m *SessionManager) DockerCommandRunner(
	ctx context.Context,
	id string,
) (DockerCommandRunner, error) {
	worker, err := m.lookup(ctx, id)
	if err != nil {
		return nil, err
	}
	return worker.dockerExecutor(ctx)
}

func (m *SessionManager) TransferFileSystem(
	ctx context.Context,
	id string,
) (SessionTransferFileSystem, error) {
	worker, err := m.lookup(ctx, id)
	if err != nil {
		return nil, err
	}
	return worker.openTransferFileSystem(ctx, 0)
}

func (m *SessionManager) TransferFileSystemForSize(
	ctx context.Context,
	id string,
	expectedSize int64,
) (SessionTransferFileSystem, error) {
	worker, err := m.lookup(ctx, id)
	if err != nil {
		return nil, err
	}
	return worker.openTransferFileSystem(ctx, expectedSize)
}

func (m *SessionManager) Get(ctx context.Context, id string) (model.Session, error) {
	worker, err := m.lookup(ctx, id)
	if err != nil {
		return model.Session{}, err
	}
	return worker.current(), nil
}

func (m *SessionManager) ReadResources(ctx context.Context, id string) (model.ResourceSnapshot, error) {
	worker, err := m.lookup(ctx, id)
	if err != nil {
		return model.ResourceSnapshot{}, err
	}
	worker.mu.RLock()
	connection := worker.connection
	worker.mu.RUnlock()
	probe, ok := connection.(resourceCapableConnection)
	if !ok {
		return model.ResourceSnapshot{}, errors.New("remote resource probe unavailable")
	}
	return probe.ReadResources(ctx)
}

func (m *SessionManager) DetectRemoteIcon(ctx context.Context, id string) (string, error) {
	worker, err := m.lookup(ctx, id)
	if err != nil {
		return "", err
	}
	worker.mu.RLock()
	connection := worker.connection
	worker.mu.RUnlock()
	probe, ok := connection.(remoteOSDetector)
	if !ok {
		return "", errors.New("remote OS detection unavailable")
	}
	return probe.DetectRemoteOS(ctx)
}

func (m *SessionManager) X11Warning(ctx context.Context, id string) string {
	worker, err := m.lookup(ctx, id)
	if err != nil {
		return ""
	}
	worker.mu.RLock()
	connection := worker.connection
	worker.mu.RUnlock()
	if provider, ok := connection.(interface{ X11Warning() string }); ok {
		return provider.X11Warning()
	}
	return ""
}

func (m *SessionManager) ActiveCount() int {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

func (m *SessionManager) List(ctx context.Context) ([]model.Session, error) {
	if err := m.ready(ctx); err != nil && !errors.Is(err, ErrSessionManagerClosed) {
		return nil, err
	}
	m.mu.RLock()
	workers := make([]*managedSession, 0, len(m.sessions))
	for _, worker := range m.sessions {
		workers = append(workers, worker)
	}
	m.mu.RUnlock()

	sessions := make([]model.Session, 0, len(workers))
	for _, worker := range workers {
		sessions = append(sessions, worker.current())
	}
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].CreatedAt.Equal(sessions[j].CreatedAt) {
			return sessions[i].ID < sessions[j].ID
		}
		return sessions[i].CreatedAt.Before(sessions[j].CreatedAt)
	})
	return sessions, nil
}

func (m *SessionManager) SetOrphanSessionFinalizer(finalizer func(sessionID string)) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.orphanFinalizer = finalizer
	m.mu.Unlock()
}

func (m *SessionManager) PrepareSessionHandover(ctx context.Context, id string) error {
	worker, err := m.lookup(ctx, id)
	if err != nil {
		return err
	}
	worker.armExpectedRebind(m.now())
	return nil
}

func (m *SessionManager) HandoverPending(ctx context.Context, id string) bool {
	if m == nil {
		return false
	}
	worker, err := m.lookup(ctx, id)
	if err != nil || worker == nil {
		return false
	}
	worker.mu.RLock()
	defer worker.mu.RUnlock()
	return worker.handoverPendingLocked(worker.sessionNow())
}

func (m *SessionManager) Close(ctx context.Context, id string) error {
	return m.closeWithReason(ctx, id, model.SessionCloseReasonUserClosed)
}

func (m *SessionManager) CloseRemoteEnded(ctx context.Context, id string) error {
	return m.closeWithReason(ctx, id, model.SessionCloseReasonRemoteEOF)
}

func (m *SessionManager) closeWithReason(
	ctx context.Context,
	id string,
	reason model.SessionCloseReason,
) error {
	if ctx == nil {
		return errors.New("session context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return model.ErrInvalidSession
	}
	m.mu.RLock()
	worker := m.sessions[id]
	if worker == nil {
		m.mu.RUnlock()
		return ErrSessionNotFound
	}
	worker.requestClose(m.now(), reason)
	m.mu.RUnlock()
	select {
	case <-worker.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *SessionManager) Retry(ctx context.Context, id string) (model.Session, error) {
	if err := m.ready(ctx); err != nil {
		return model.Session{}, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return model.Session{}, model.ErrInvalidSession
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return model.Session{}, ErrSessionManagerClosed
	}
	previous := m.sessions[id]
	if previous == nil {
		m.mu.Unlock()
		return model.Session{}, ErrSessionNotFound
	}
	select {
	case <-previous.done:
	default:
		m.mu.Unlock()
		return model.Session{}, ErrSessionNotRetryable
	}
	previous.mu.Lock()
	if previous.snapshot.State != model.SessionStateHostKeyPending {
		previous.mu.Unlock()
		m.mu.Unlock()
		return model.Session{}, ErrSessionNotRetryable
	}
	snapshot, err := previous.snapshot.Transition(model.SessionStateConnecting, m.now())
	previous.mu.Unlock()
	if err != nil {
		m.mu.Unlock()
		return model.Session{}, err
	}
	workerCtx, cancel := context.WithCancel(m.rootCtx)
	worker := &managedSession{
		snapshot:  snapshot,
		connector: m.connector,
		ctx:       workerCtx,
		cancel:    cancel,
		ready:     make(chan struct{}),
		done:      make(chan struct{}),
		now:       m.now,
	}
	m.sessions[id] = worker
	m.startOrphanSweepLocked()
	m.wg.Add(1)
	m.mu.Unlock()

	go m.run(worker)
	return snapshot, nil
}

func (m *SessionManager) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return errors.New("session shutdown context is nil")
	}
	m.mu.Lock()
	if !m.closed {
		m.closed = true
		m.cancel()
	}
	workers := make([]*managedSession, 0, len(m.sessions))
	for _, worker := range m.sessions {
		workers = append(workers, worker)
	}
	m.mu.Unlock()

	for _, worker := range workers {
		worker.requestClose(m.now(), model.SessionCloseReasonUserClosed)
	}
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *SessionManager) startOrphanSweepLocked() {
	m.sweepOnce.Do(func() {
		m.wg.Add(1)
		go m.orphanSweepLoop()
	})
}

func (m *SessionManager) orphanSweepLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(orphanSweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.rootCtx.Done():
			return
		case <-ticker.C:
			m.sweepOrphanSessions()
		}
	}
}

func (m *SessionManager) sweepOrphanSessions() {
	if m == nil || m.now == nil {
		return
	}
	now := m.now()
	m.mu.RLock()
	workers := make([]*managedSession, 0, len(m.sessions))
	for _, worker := range m.sessions {
		workers = append(workers, worker)
	}
	finalizer := m.orphanFinalizer
	m.mu.RUnlock()

	for _, worker := range workers {
		if !worker.orphanExpired(now) {
			continue
		}
		sessionID := worker.current().ID

		if finalizer != nil {
			finalizer(sessionID)
		}
		ctx, cancel := context.WithTimeout(context.Background(), orphanCloseTimeout)

		_ = m.closeWithReason(ctx, sessionID, model.SessionCloseReasonOrphanTimeout)
		cancel()
	}
}

func (m *SessionManager) run(worker *managedSession) {
	defer m.wg.Done()
	defer worker.finish()

	snapshot := worker.current()
	connector := worker.connector
	if connector == nil {
		worker.fail(m.now())
		return
	}
	connection, err := connector.Connect(
		worker.ctx,
		snapshot.ID,
		snapshot.HostID,
		func(state model.SessionState) error {
			return worker.transition(state, m.now())
		},
	)
	if err != nil {
		var pending *PendingSessionError
		if errors.As(err, &pending) &&
			pending.State == model.SessionStateHostKeyPending &&
			worker.pause(pending.State, m.now()) {
			return
		}
		worker.finishConnectError(m.now())
		return
	}
	if connection == nil {
		worker.fail(m.now())
		return
	}
	if !worker.attach(connection, m.now()) {
		worker.closeConnection()

		worker.closeNormally(m.now(), model.SessionCloseReasonError)
		return
	}
	defer worker.closeConnection()
	if measurable, ok := connection.(latencyCapableConnection); ok {
		latencyEnabled := true
		if configured, hasSetting := connection.(latencyEnabledConnection); hasSetting {
			latencyEnabled = configured.LatencyEnabled()
		}
		if latencyEnabled {
			monitorCtx, cancelMonitor := context.WithCancel(worker.ctx)
			monitorDone := make(chan struct{})
			go func() {
				defer close(monitorDone)
				worker.monitorLatency(monitorCtx, measurable)
			}()
			defer func() {
				cancelMonitor()
				<-monitorDone
			}()
		}
	}

	err = connection.Wait()
	if worker.ctx.Err() != nil {

		worker.closeNormally(m.now(), model.SessionCloseReasonError)
		return
	}
	if err == nil {

		worker.closeNormally(m.now(), model.SessionCloseReasonRemoteEOF)
		return
	}
	worker.fail(m.now())
}

func (s *managedSession) monitorLatency(ctx context.Context, connection latencyCapableConnection) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		latency, err := connection.MeasureLatency(probeCtx)
		cancel()
		s.mu.Lock()

		s.snapshot.LatencyMS = nil
		if err == nil && s.snapshot.State == model.SessionStateConnected && ctx.Err() == nil {
			milliseconds := max(int64(1), latency.Milliseconds())
			s.snapshot.LatencyMS = &milliseconds
		}
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (m *SessionManager) lookup(ctx context.Context, id string) (*managedSession, error) {
	if ctx == nil {
		return nil, errors.New("session context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, model.ErrInvalidSession
	}
	m.mu.RLock()
	worker := m.sessions[id]
	m.mu.RUnlock()
	if worker == nil {
		return nil, ErrSessionNotFound
	}
	return worker, nil
}

func (m *SessionManager) ready(ctx context.Context) error {
	if ctx == nil {
		return errors.New("session context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if m == nil || m.connector == nil || m.now == nil || m.newID == nil || m.rootCtx == nil || m.cancel == nil {
		return ErrSessionManagerNotReady
	}
	m.mu.RLock()
	closed := m.closed
	m.mu.RUnlock()
	if closed {
		return ErrSessionManagerClosed
	}
	return nil
}

func randomSessionID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return "session_" + hex.EncodeToString(value[:]), nil
}
