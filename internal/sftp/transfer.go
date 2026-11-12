package sftp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"changeme/internal/model"
)

var (
	ErrSFTPTransferManagerClosed      = errors.New("SFTP transfer manager is closed")
	ErrSFTPTransferNotFound           = errors.New("SFTP transfer not found")
	ErrSFTPTransferNotConfirmable     = errors.New("SFTP transfer is not awaiting overwrite")
	ErrSFTPTransferNotRetryable       = errors.New("SFTP transfer is not retryable")
	ErrSFTPTransferDestinationExists  = errors.New("SFTP transfer destination exists")
	ErrSFTPTransferExecutorNotReady   = errors.New("SFTP transfer executor is not ready")
	ErrSFTPTransferInvalidLocalSource = errors.New("invalid SFTP transfer local source")
	ErrSFTPTransferDuplicate          = errors.New("duplicate SFTP transfer ID")
)

type SFTPTransferRequest struct {
	SessionID        string
	BatchID          string
	Direction        model.SFTPTransferDirection
	LocalPath        string
	LocalName        string
	RemotePath       string
	SourceRemotePath string
	Total            int64
	SourceSessionID  string

	CopyDestination bool

	Archive *ArchiveUploadSpec

	ResumeKey string

	ArchiveDownload *ArchiveDownloadSpec

	ReportStage func(stage string)

	ReportTotal func(total int64)
}

type ArchiveDownloadSpec struct {
	SourceDirectory string

	TargetDirectory string
}

type SFTPTransferExecutor interface {
	Execute(
		ctx context.Context,
		request SFTPTransferRequest,
		overwrite bool,
		progress func(transferred int64) error,
	) error
}

type SFTPTransferManager struct {
	mu             sync.RWMutex
	executor       SFTPTransferExecutor
	now            func() time.Time
	newID          func() (string, error)
	rootCtx        context.Context
	cancel         context.CancelFunc
	limiter        *transferLimiter
	stoppedBatches map[string]bool
	tasks          map[string]*managedSFTPTransfer
	closed         bool
	wg             sync.WaitGroup
}

type transferLimiter struct {
	mu      sync.Mutex
	limit   int
	active  int
	changed chan struct{}
}

type managedSFTPTransfer struct {
	mu              sync.RWMutex
	snapshot        model.SFTPTransfer
	request         SFTPTransferRequest
	overwrite       bool
	copyDestination bool
	cancel          context.CancelFunc

	pauseRequested bool
}

func NewSFTPTransferManager(executor SFTPTransferExecutor, concurrency int) *SFTPTransferManager {
	return newSFTPTransferManager(executor, concurrency, time.Now, randomTransferID)
}

func newSFTPTransferManager(
	executor SFTPTransferExecutor,
	concurrency int,
	now func() time.Time,
	newID func() (string, error),
) *SFTPTransferManager {
	if concurrency < 1 {
		concurrency = 1
	}
	rootCtx, cancel := context.WithCancel(context.Background())
	manager := &SFTPTransferManager{
		executor:       executor,
		now:            now,
		newID:          newID,
		rootCtx:        rootCtx,
		cancel:         cancel,
		limiter:        newTransferLimiter(concurrency),
		tasks:          make(map[string]*managedSFTPTransfer),
		stoppedBatches: make(map[string]bool),
	}

	go manager.sweepPausedResumes()
	return manager
}

func (m *SFTPTransferManager) SetConcurrency(concurrency int) error {
	if concurrency < 1 || concurrency > 16 {
		return errors.New("SFTP transfer concurrency is out of range")
	}
	if m == nil || m.limiter == nil {
		return ErrSFTPTransferExecutorNotReady
	}
	m.mu.RLock()
	closed := m.closed
	m.mu.RUnlock()
	if closed {
		return ErrSFTPTransferManagerClosed
	}
	m.limiter.setLimit(concurrency)
	return nil
}

func (m *SFTPTransferManager) Enqueue(
	ctx context.Context,
	request SFTPTransferRequest,
) (model.SFTPTransfer, error) {
	if err := transferManagerReady(ctx, m); err != nil {
		return model.SFTPTransfer{}, err
	}
	if request.SourceSessionID != "" {

		if request.SourceRemotePath == "" {
			return model.SFTPTransfer{}, ErrSFTPTransferInvalidLocalSource
		}
	} else if request.LocalPath == "" || len(request.LocalPath) > 4096 || !utf8.ValidString(request.LocalPath) ||
		strings.IndexByte(request.LocalPath, 0) >= 0 {
		return model.SFTPTransfer{}, ErrSFTPTransferInvalidLocalSource
	}
	id, err := m.newID()
	if err != nil {
		return model.SFTPTransfer{}, err
	}

	if request.Direction == model.SFTPTransferUpload && request.SourceSessionID == "" && request.Archive == nil {
		request.ResumeKey = id
	}
	snapshot, err := model.NewSFTPTransfer(
		id,
		request.SessionID,
		request.Direction,
		request.LocalName,
		request.RemotePath,
		request.Total,
		m.now(),
	)
	if err != nil {
		return model.SFTPTransfer{}, err
	}
	if request.BatchID == "" {
		request.BatchID = id
	}
	task := &managedSFTPTransfer{snapshot: snapshot, request: request}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return model.SFTPTransfer{}, ErrSFTPTransferManagerClosed
	}
	if m.stoppedBatches[request.BatchID] {
		m.mu.Unlock()
		return model.SFTPTransfer{}, context.Canceled
	}
	if _, exists := m.tasks[id]; exists {
		m.mu.Unlock()
		return model.SFTPTransfer{}, ErrSFTPTransferDuplicate
	}
	m.tasks[id] = task
	m.startLocked(task)
	m.mu.Unlock()
	return snapshot, nil
}

func (m *SFTPTransferManager) Get(ctx context.Context, id string) (model.SFTPTransfer, error) {
	task, err := m.lookup(ctx, id)
	if err != nil {
		return model.SFTPTransfer{}, err
	}
	return task.current(), nil
}

func (m *SFTPTransferManager) List(ctx context.Context) ([]model.SFTPTransfer, error) {
	if err := transferManagerReady(ctx, m); err != nil {
		return nil, err
	}
	m.mu.RLock()
	result := make([]model.SFTPTransfer, 0, len(m.tasks))
	for _, task := range m.tasks {
		result = append(result, task.current())
	}
	m.mu.RUnlock()
	sort.Slice(result, func(left, right int) bool {
		if result[left].CreatedAt.Equal(result[right].CreatedAt) {
			return result[left].ID < result[right].ID
		}
		return result[left].CreatedAt.Before(result[right].CreatedAt)
	})
	return result, nil
}

func (m *SFTPTransferManager) ConfirmOverwrite(ctx context.Context, id string) (model.SFTPTransfer, error) {
	return m.restart(ctx, id, model.SFTPTransferAwaitingOverwrite, true, false, ErrSFTPTransferNotConfirmable)
}

func (m *SFTPTransferManager) Retry(ctx context.Context, id string) (model.SFTPTransfer, error) {
	return m.restart(ctx, id, model.SFTPTransferFailed, false, false, ErrSFTPTransferNotRetryable)
}

const pausedResumeTTL = 24 * time.Hour

type resumeCleaner interface {
	CleanupResume(ctx context.Context, sessionID, resumeKey string) error
}

func (m *SFTPTransferManager) cleanupResume(task *managedSFTPTransfer) {
	cleaner, ok := m.executor.(resumeCleaner)
	if !ok || task == nil {
		return
	}
	task.mu.RLock()
	resumeKey := task.request.ResumeKey
	sessionID := task.request.SessionID
	task.mu.RUnlock()
	if resumeKey == "" || sessionID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = cleaner.CleanupResume(ctx, sessionID, resumeKey)
}

func (m *SFTPTransferManager) sweepPausedResumes() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-m.rootCtx.Done():
			return
		case <-ticker.C:
		}
		now := m.now()
		m.mu.RLock()
		tasks := make([]*managedSFTPTransfer, 0, len(m.tasks))
		for _, task := range m.tasks {
			tasks = append(tasks, task)
		}
		m.mu.RUnlock()
		for _, task := range tasks {
			task.mu.RLock()
			expired := task.snapshot.State == model.SFTPTransferPaused &&
				now.Sub(task.snapshot.UpdatedAt) >= pausedResumeTTL
			task.mu.RUnlock()
			if !expired {
				continue
			}
			task.mu.Lock()
			if next, transitionErr := task.snapshot.Transition(model.SFTPTransferCanceled, now); transitionErr == nil {
				task.snapshot = next
			}
			task.mu.Unlock()
			m.cleanupResume(task)
		}
	}
}

func (m *SFTPTransferManager) Pause(ctx context.Context, id string) (model.SFTPTransfer, error) {
	task, err := m.lookup(ctx, id)
	if err != nil {
		return model.SFTPTransfer{}, err
	}
	task.mu.Lock()
	if task.request.ResumeKey == "" || task.request.Direction != model.SFTPTransferUpload {
		task.mu.Unlock()
		return model.SFTPTransfer{}, model.ErrInvalidSFTPTransferTransition
	}

	if task.snapshot.State != model.SFTPTransferRunning {
		task.mu.Unlock()
		return model.SFTPTransfer{}, model.ErrInvalidSFTPTransferTransition
	}
	task.pauseRequested = true
	cancel := task.cancel
	task.mu.Unlock()
	if cancel != nil {
		cancel()
	}

	task.mu.RLock()
	defer task.mu.RUnlock()
	return task.snapshot, nil
}

func (m *SFTPTransferManager) Resume(ctx context.Context, id string) (model.SFTPTransfer, error) {
	task, err := m.lookup(ctx, id)
	if err != nil {
		return model.SFTPTransfer{}, err
	}
	task.mu.Lock()
	if task.snapshot.State != model.SFTPTransferPaused {
		task.mu.Unlock()
		return model.SFTPTransfer{}, model.ErrInvalidSFTPTransferTransition
	}
	next, transitionErr := task.snapshot.Transition(model.SFTPTransferQueued, m.now())
	if transitionErr != nil {
		task.mu.Unlock()
		return model.SFTPTransfer{}, transitionErr
	}
	task.snapshot = next

	task.snapshot.RateBytesPerSecond = 0
	task.pauseRequested = false
	task.mu.Unlock()
	m.mu.Lock()
	m.startLocked(task)
	m.mu.Unlock()
	task.mu.RLock()
	defer task.mu.RUnlock()
	return task.snapshot, nil
}

func (m *SFTPTransferManager) Cancel(ctx context.Context, id string) error {
	task, err := m.lookup(ctx, id)
	if err != nil {
		return err
	}
	task.mu.Lock()
	if task.snapshot.State == model.SFTPTransferCompleted || task.snapshot.State == model.SFTPTransferCanceled {
		task.mu.Unlock()
		return nil
	}
	next, transitionErr := task.snapshot.Transition(model.SFTPTransferCanceled, m.now())
	if transitionErr != nil {
		task.mu.Unlock()
		return transitionErr
	}
	task.snapshot = next
	cancel := task.cancel
	task.mu.Unlock()
	if cancel != nil {
		cancel()
	}

	go m.cleanupResume(task)
	return nil
}

func (m *SFTPTransferManager) Shutdown(ctx context.Context) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("SFTP transfer shutdown context is nil")
	}
	var cleanup []*managedSFTPTransfer
	m.mu.Lock()
	if !m.closed {
		m.closed = true
		m.cancel()
		for _, task := range m.tasks {
			task.mu.Lock()
			if task.cancel != nil {
				task.cancel()
			}
			wasPaused := task.snapshot.State == model.SFTPTransferPaused
			if task.snapshot.State != model.SFTPTransferCompleted && task.snapshot.State != model.SFTPTransferCanceled {
				if next, err := task.snapshot.Transition(model.SFTPTransferCanceled, m.now()); err == nil {
					task.snapshot = next
				}
			}

			if wasPaused && task.request.ResumeKey != "" {
				cleanup = append(cleanup, task)
			}
			task.mu.Unlock()
		}
	}
	m.mu.Unlock()
	for _, task := range cleanup {
		m.cleanupResume(task)
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

func (m *SFTPTransferManager) restart(
	ctx context.Context,
	id string,
	expected model.SFTPTransferState,
	confirmOverwrite bool,
	copyDestination bool,
	stateErr error,
) (model.SFTPTransfer, error) {
	if err := transferManagerReady(ctx, m); err != nil {
		return model.SFTPTransfer{}, err
	}
	id = strings.TrimSpace(id)
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return model.SFTPTransfer{}, ErrSFTPTransferManagerClosed
	}
	task := m.tasks[id]
	if task == nil {
		m.mu.Unlock()
		return model.SFTPTransfer{}, ErrSFTPTransferNotFound
	}
	task.mu.Lock()
	if task.snapshot.State != expected {
		task.mu.Unlock()
		m.mu.Unlock()
		return model.SFTPTransfer{}, stateErr
	}
	next, err := task.snapshot.Transition(model.SFTPTransferQueued, m.now())
	if err != nil {
		task.mu.Unlock()
		m.mu.Unlock()
		return model.SFTPTransfer{}, err
	}

	next.Transferred = 0
	task.snapshot = next
	task.overwrite = confirmOverwrite
	task.copyDestination = copyDestination
	snapshot := task.snapshot
	task.mu.Unlock()
	m.startLocked(task)
	m.mu.Unlock()
	return snapshot, nil
}

func (m *SFTPTransferManager) startLocked(task *managedSFTPTransfer) {
	runCtx, cancel := context.WithCancel(m.rootCtx)
	task.mu.Lock()
	task.cancel = cancel
	task.mu.Unlock()
	m.wg.Add(1)
	go m.run(runCtx, cancel, task)
}

func (m *SFTPTransferManager) run(
	ctx context.Context,
	cancel context.CancelFunc,
	task *managedSFTPTransfer,
) {
	defer m.wg.Done()
	defer cancel()
	if !m.limiter.acquire(ctx) {
		m.finishCanceled(task)
		return
	}
	defer m.limiter.release()

	task.mu.Lock()
	if task.snapshot.State != model.SFTPTransferQueued {
		task.mu.Unlock()
		return
	}
	next, err := task.snapshot.Transition(model.SFTPTransferRunning, m.now())
	if err != nil {
		task.mu.Unlock()
		return
	}
	task.snapshot = next
	request := task.request
	overwrite := task.overwrite
	copyDestination := task.copyDestination
	task.mu.Unlock()

	var lastSampleAt time.Time
	var lastSampleBytes int64
	var lastRate int64
	var smoothedRate float64
	reportProgress := func(transferred int64) error {
		task.mu.Lock()
		defer task.mu.Unlock()
		now := m.now()
		if lastSampleAt.IsZero() {
			lastSampleAt, lastSampleBytes = now, transferred
		} else if elapsed := now.Sub(lastSampleAt); elapsed >= 200*time.Millisecond {
			delta := transferred - lastSampleBytes
			if delta < 0 {
				delta = 0
			}
			instant := float64(delta) / elapsed.Seconds()
			if smoothedRate <= 0 {
				smoothedRate = instant
			} else {
				smoothedRate = smoothedRate*0.65 + instant*0.35
			}
			lastRate = int64(smoothedRate)
			lastSampleAt, lastSampleBytes = now, transferred
		}

		if transferred < task.snapshot.Transferred {
			transferred = task.snapshot.Transferred
		}
		if transferred > task.snapshot.Total {
			transferred = task.snapshot.Total
		}
		next, progressErr := task.snapshot.WithProgressAndRate(transferred, lastRate, now)
		if progressErr == nil {
			task.snapshot = next
		}
		return progressErr
	}

	reportStage := func(stage string) {
		task.mu.Lock()
		defer task.mu.Unlock()
		next, stageErr := task.snapshot.WithStage(stage, m.now())
		if stageErr == nil {
			task.snapshot = next
		}
	}

	reportTotal := func(total int64) {
		task.mu.Lock()
		defer task.mu.Unlock()
		next, totalErr := task.snapshot.WithTotal(total, m.now())
		if totalErr == nil {
			task.snapshot = next
		}
	}
	for attempt := 0; attempt < 10000; attempt++ {
		if err = ctx.Err(); err != nil {
			break
		}
		candidate := request

		candidate.CopyDestination = false

		candidate.ReportStage = reportStage
		candidate.ReportTotal = reportTotal
		if copyDestination {
			candidate.CopyDestination = true
			if request.Direction == model.SFTPTransferUpload {
				candidate.RemotePath = path.Join(path.Dir(request.RemotePath), model.TransferCopyName(path.Base(request.RemotePath), attempt+1))
			} else {
				candidate.LocalName = model.TransferCopyName(filepath.Base(request.LocalPath), attempt+1)
				candidate.LocalPath = filepath.Join(filepath.Dir(request.LocalPath), candidate.LocalName)
			}
		}
		task.mu.Lock()
		task.snapshot.RemotePath = candidate.RemotePath
		task.snapshot.LocalName = candidate.LocalName
		task.mu.Unlock()
		err = m.executor.Execute(ctx, candidate, overwrite, reportProgress)
		if !copyDestination || !errors.Is(err, ErrSFTPTransferDestinationExists) {
			break
		}
	}

	task.mu.Lock()
	defer task.mu.Unlock()
	if task.snapshot.State == model.SFTPTransferCanceled {
		return
	}

	if task.pauseRequested && errors.Is(ctx.Err(), context.Canceled) {

		task.pauseRequested = false
		if next, transitionErr := task.snapshot.Transition(model.SFTPTransferPaused, m.now()); transitionErr == nil {
			task.snapshot = next
		}
		return
	}
	state := model.SFTPTransferCompleted
	if errors.Is(err, ErrSFTPTransferDestinationExists) && !overwrite {
		state = model.SFTPTransferAwaitingOverwrite
	} else if err != nil {
		if ctx.Err() != nil {
			state = model.SFTPTransferCanceled
		} else {
			state = model.SFTPTransferFailed
		}
	}
	if state == model.SFTPTransferFailed {

		slog.Warn("sftp transfer failed",
			"event", "sftp.transfer.failed",
			"id", task.snapshot.ID,
			"direction", string(task.snapshot.Direction),
			"transferred", task.snapshot.Transferred,
			"total", task.snapshot.Total,
			"err", err)
	}
	if next, transitionErr := task.snapshot.Transition(state, m.now()); transitionErr == nil {
		task.snapshot = next
	}
}

func (m *SFTPTransferManager) finishCanceled(task *managedSFTPTransfer) {
	task.mu.Lock()
	defer task.mu.Unlock()
	if task.snapshot.State == model.SFTPTransferCanceled || task.snapshot.State == model.SFTPTransferCompleted {
		return
	}
	if next, err := task.snapshot.Transition(model.SFTPTransferCanceled, m.now()); err == nil {
		task.snapshot = next
	}
}

func (m *SFTPTransferManager) lookup(ctx context.Context, id string) (*managedSFTPTransfer, error) {
	if err := transferManagerReady(ctx, m); err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	m.mu.RLock()
	task := m.tasks[id]
	m.mu.RUnlock()
	if task == nil {
		return nil, ErrSFTPTransferNotFound
	}
	return task, nil
}

func (t *managedSFTPTransfer) current() model.SFTPTransfer {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.snapshot
}

func transferManagerReady(ctx context.Context, manager *SFTPTransferManager) error {
	if ctx == nil {
		return errors.New("SFTP transfer context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if manager == nil || manager.executor == nil || manager.now == nil || manager.newID == nil {
		return ErrSFTPTransferExecutorNotReady
	}
	return nil
}

func randomTransferID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func newTransferLimiter(limit int) *transferLimiter {
	return &transferLimiter{limit: limit, changed: make(chan struct{})}
}

func (l *transferLimiter) acquire(ctx context.Context) bool {
	if l == nil || ctx == nil {
		return false
	}
	for {
		l.mu.Lock()
		if l.active < l.limit {
			l.active++
			l.mu.Unlock()
			return true
		}
		changed := l.changed
		l.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			return false
		}
	}
}

func (l *transferLimiter) release() {
	if l == nil {
		return
	}
	l.mu.Lock()
	if l.active > 0 {
		l.active--
	}
	close(l.changed)
	l.changed = make(chan struct{})
	l.mu.Unlock()
}

func (l *transferLimiter) setLimit(limit int) {
	l.mu.Lock()
	l.limit = limit
	close(l.changed)
	l.changed = make(chan struct{})
	l.mu.Unlock()
}
