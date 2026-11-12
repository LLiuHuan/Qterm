package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"changeme/internal/ai"
	"changeme/internal/errno"
	"changeme/internal/model"
	"changeme/internal/session"
)

const (
	aiTempSessionLimit = 3

	aiTempSessionIdleTTL = 90 * time.Second

	aiTempSessionSweepInterval = 30 * time.Second

	aiTempSessionConnectTimeout = 15 * time.Second

	aiTempSessionPollInterval = 50 * time.Millisecond

	aiTempSessionCloseTimeout = 5 * time.Second
)

const msgAIAgentConnectFailed = "ai.agent.connect_failed"

var aiTempSessionManualKeys = map[string]struct{}{
	"session.hostkey_changed":     {},
	"session.credential_unlinked": {},
	"session.credential_required": {},
	"session.auth_failed":         {},
	"session.trust_failed":        {},
}

type aiTempSessionBackend interface {
	List(ctx context.Context) ([]model.Session, error)
	Get(ctx context.Context, id string) (model.Session, error)
	OpenForAI(ctx context.Context, hostID string) (model.Session, error)
	Close(ctx context.Context, id string) error

	Failure(id string) string
}

type liveAITempSessionBackend struct {
	service *session.Service
	manager *session.SessionManager
}

func (backend liveAITempSessionBackend) List(ctx context.Context) ([]model.Session, error) {
	return backend.manager.List(ctx)
}

func (backend liveAITempSessionBackend) Get(ctx context.Context, id string) (model.Session, error) {
	return backend.manager.Get(ctx, id)
}

func (backend liveAITempSessionBackend) OpenForAI(ctx context.Context, hostID string) (model.Session, error) {
	return backend.manager.OpenForAI(ctx, hostID)
}

func (backend liveAITempSessionBackend) Close(ctx context.Context, id string) error {
	return backend.manager.Close(ctx, id)
}

func (backend liveAITempSessionBackend) Failure(id string) string {
	message, err := backend.service.GetSessionFailure(id)
	if err != nil {
		return ""
	}
	return message
}

type aiTempSessionEntry struct {
	hostID    string
	sessionID string

	lastUsed time.Time
}

type aiTempSessionCall struct {
	id   string
	err  error
	done chan struct{}
}

type aiTempSessions struct {
	mu       sync.Mutex
	entries  map[string]*aiTempSessionEntry
	inflight map[string]*aiTempSessionCall

	backend func() (aiTempSessionBackend, error)

	now func() time.Time

	limit          int
	idleTTL        time.Duration
	sweepInterval  time.Duration
	connectTimeout time.Duration
	pollInterval   time.Duration

	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

func newAITempSessions(service *session.Service) *aiTempSessions {
	if service == nil {
		return nil
	}
	pool := &aiTempSessions{
		entries:        make(map[string]*aiTempSessionEntry),
		inflight:       make(map[string]*aiTempSessionCall),
		now:            time.Now,
		limit:          aiTempSessionLimit,
		idleTTL:        aiTempSessionIdleTTL,
		sweepInterval:  aiTempSessionSweepInterval,
		connectTimeout: aiTempSessionConnectTimeout,
		pollInterval:   aiTempSessionPollInterval,
		stopCh:         make(chan struct{}),
	}
	pool.backend = func() (aiTempSessionBackend, error) {
		manager, err := service.Manager()
		if err != nil {
			return nil, err
		}
		return liveAITempSessionBackend{service: service, manager: manager}, nil
	}
	return pool
}

func (pool *aiTempSessions) connect(ctx context.Context, hostID string) (string, error) {
	if pool == nil {
		return "", ai.ErrNoOpenSession
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return "", errno.New(errno.CodeInvalidArg, "session.not_found", "")
	}
	backend, err := pool.backend()
	if err != nil {

		return "", errno.New(errno.CodeConnFailed, msgAIAgentConnectFailed, "")
	}

	if sessionID, ok := pool.reuse(ctx, backend, hostID); ok {
		return sessionID, nil
	}

	return pool.connectOnce(ctx, backend, hostID)
}

func (pool *aiTempSessions) reuse(
	ctx context.Context,
	backend aiTempSessionBackend,
	hostID string,
) (string, bool) {
	pool.mu.Lock()
	entry, ok := pool.entries[hostID]
	pool.mu.Unlock()
	if !ok {
		return "", false
	}

	current, err := backend.Get(ctx, entry.sessionID)
	if err == nil && current.State == model.SessionStateConnected && current.Kind == model.SessionKindSSH {
		pool.touch(hostID, entry.sessionID)
		return entry.sessionID, true
	}

	if err != nil && ctx.Err() != nil {
		return "", false
	}

	pool.remove(hostID, entry.sessionID)
	pool.closeSession(ctx, backend, entry.sessionID)
	return "", false
}

func (pool *aiTempSessions) connectOnce(
	ctx context.Context,
	backend aiTempSessionBackend,
	hostID string,
) (string, error) {
	pool.mu.Lock()
	if call, running := pool.inflight[hostID]; running {
		pool.mu.Unlock()
		select {
		case <-call.done:
			return call.id, call.err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	call := &aiTempSessionCall{done: make(chan struct{})}
	pool.inflight[hostID] = call
	pool.mu.Unlock()

	sessionID, err := pool.open(ctx, backend, hostID)

	pool.mu.Lock()
	delete(pool.inflight, hostID)
	pool.mu.Unlock()

	call.id, call.err = sessionID, err
	close(call.done)
	return sessionID, err
}

func (pool *aiTempSessions) open(
	ctx context.Context,
	backend aiTempSessionBackend,
	hostID string,
) (string, error) {
	opened, err := backend.OpenForAI(ctx, hostID)
	if err != nil {
		return "", pool.mapConnectError(err)
	}
	sessionID := strings.TrimSpace(opened.ID)
	if sessionID == "" {
		return "", errno.New(errno.CodeConnFailed, msgAIAgentConnectFailed, "")
	}

	state, failureKey, waitErr := pool.waitForConnection(ctx, backend, sessionID)
	if waitErr != nil {
		pool.closeSession(ctx, backend, sessionID)
		return "", waitErr
	}

	switch state {
	case model.SessionStateConnected:

	case model.SessionStateAwaitingAuth, model.SessionStateHostKeyPending:

		pool.closeSession(ctx, backend, sessionID)
		slog.Info("ai temp session requires manual authentication",
			"hostId", hostID, "sessionId", sessionID, "state", string(state))
		return "", fmt.Errorf("%w: interactive authentication required", ai.ErrConnectNeedsManual)
	default:
		pool.closeSession(ctx, backend, sessionID)
		return "", pool.mapFailureKey(failureKey)
	}

	evicted := pool.register(hostID, sessionID)
	for _, stale := range evicted {

		pool.closeSession(ctx, backend, stale)
	}
	return sessionID, nil
}

func (pool *aiTempSessions) waitForConnection(
	ctx context.Context,
	backend aiTempSessionBackend,
	sessionID string,
) (model.SessionState, string, error) {
	timeout := time.NewTimer(pool.connectTimeout)
	defer timeout.Stop()
	ticker := time.NewTicker(pool.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-timeout.C:

			return "", "", errno.New(errno.CodeConnFailed, msgAIAgentConnectFailed, "")
		case <-ticker.C:
			current, err := backend.Get(ctx, sessionID)
			if err != nil {

				return "", "", errno.New(errno.CodeConnFailed, msgAIAgentConnectFailed, "")
			}
			switch current.State {
			case model.SessionStateConnected:
				if current.Kind != model.SessionKindSSH {

					return "", "", errno.New(errno.CodeConnFailed, msgAIAgentConnectFailed, "")
				}
				return model.SessionStateConnected, "", nil
			case model.SessionStateAwaitingAuth, model.SessionStateHostKeyPending:
				return current.State, "", nil
			case model.SessionStateFailed, model.SessionStateClosed, model.SessionStateClosing:
				return current.State, backend.Failure(sessionID), nil
			}

		}
	}
}

func (pool *aiTempSessions) register(hostID string, sessionID string) []string {
	now := pool.now()
	pool.mu.Lock()
	var evicted []string
	if existing, ok := pool.entries[hostID]; ok {
		delete(pool.entries, hostID)
		if existing.sessionID != sessionID {
			evicted = append(evicted, existing.sessionID)
		}
	}
	for len(pool.entries) >= pool.limit {
		oldestHostID, oldest, found := pool.oldestLocked()
		if !found {
			break
		}
		delete(pool.entries, oldestHostID)
		evicted = append(evicted, oldest.sessionID)
	}
	pool.entries[hostID] = &aiTempSessionEntry{
		hostID:    hostID,
		sessionID: sessionID,
		lastUsed:  now,
	}
	pool.mu.Unlock()

	pool.startSweeper()
	return evicted
}

func (pool *aiTempSessions) oldestLocked() (string, *aiTempSessionEntry, bool) {
	var (
		oldestHostID string
		oldest       *aiTempSessionEntry
	)
	for hostID, entry := range pool.entries {
		if oldest == nil || entry.lastUsed.Before(oldest.lastUsed) {
			oldestHostID, oldest = hostID, entry
		}
	}
	return oldestHostID, oldest, oldest != nil
}

func (pool *aiTempSessions) touch(hostID string, sessionID string) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	entry, ok := pool.entries[hostID]
	if !ok || entry.sessionID != sessionID {
		return
	}
	entry.lastUsed = pool.now()
}

func (pool *aiTempSessions) remove(hostID string, sessionID string) bool {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	entry, ok := pool.entries[hostID]
	if !ok || entry.sessionID != sessionID {
		return false
	}
	delete(pool.entries, hostID)
	return true
}

func (pool *aiTempSessions) startSweeper() {
	pool.startOnce.Do(func() {
		pool.wg.Add(1)
		go pool.sweepLoop()
	})
}

func (pool *aiTempSessions) sweepLoop() {
	defer pool.wg.Done()
	ticker := time.NewTicker(pool.sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-pool.stopCh:
			return
		case <-ticker.C:
			if !pool.sweepOnce() {

				return
			}
		}
	}
}

func (pool *aiTempSessions) sweepOnce() bool {
	backend, err := pool.backend()
	if err != nil {
		return false
	}
	now := pool.now()
	pool.mu.Lock()
	var expired []string
	for hostID, entry := range pool.entries {
		if now.Sub(entry.lastUsed) >= pool.idleTTL {
			delete(pool.entries, hostID)
			expired = append(expired, entry.sessionID)
		}
	}
	pool.mu.Unlock()

	for _, sessionID := range expired {
		slog.Info("ai temp session idle closed", "sessionId", sessionID)
		pool.closeSession(context.Background(), backend, sessionID)
	}
	return true
}

func (pool *aiTempSessions) Stop() {
	if pool == nil {
		return
	}
	pool.stopOnce.Do(func() { close(pool.stopCh) })
	pool.wg.Wait()

	pool.mu.Lock()
	sessionIDs := make([]string, 0, len(pool.entries))
	for _, entry := range pool.entries {
		sessionIDs = append(sessionIDs, entry.sessionID)
	}
	pool.entries = make(map[string]*aiTempSessionEntry)
	pool.mu.Unlock()

	backend, err := pool.backend()
	if err != nil {
		return
	}
	for _, sessionID := range sessionIDs {
		pool.closeSession(context.Background(), backend, sessionID)
	}
}

func (pool *aiTempSessions) closeSession(ctx context.Context, backend aiTempSessionBackend, sessionID string) {
	sessionID = strings.TrimSpace(sessionID)
	if backend == nil || sessionID == "" {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), aiTempSessionCloseTimeout)
	defer cancel()
	ctx = closeCtx
	if err := backend.Close(ctx, sessionID); err != nil {

		slog.Warn("ai temp session close failed", "sessionId", sessionID, "error", err)
	}
}

func (pool *aiTempSessions) mapConnectError(err error) error {
	if err == nil {
		return nil
	}
	if appError := errno.Extract(err); appError != nil {

		code := errno.CodeConnFailed
		if appError.Code == errno.CodeCancelled || appError.Code == errno.CodeTimeout {
			code = appError.Code
		}
		if cause := appError.Unwrap(); cause != nil {
			return errno.Wrap(code, msgAIAgentConnectFailed, cause)
		}
		return errno.New(code, msgAIAgentConnectFailed, "")
	}
	return errno.Wrap(errno.CodeConnFailed, msgAIAgentConnectFailed, err)
}

func (pool *aiTempSessions) mapFailureKey(failureKey string) error {
	key := strings.TrimSpace(failureKey)
	if key == "" {
		return errno.New(errno.CodeConnFailed, msgAIAgentConnectFailed, "")
	}
	if _, manual := aiTempSessionManualKeys[key]; manual {

		return fmt.Errorf("%w: connection rejected", ai.ErrConnectNeedsManual)
	}
	return errno.New(errno.CodeConnFailed, msgAIAgentConnectFailed, "")
}
