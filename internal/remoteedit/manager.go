package remoteedit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"changeme/internal/errno"
	"changeme/internal/model"
	sessionapp "changeme/internal/session"
)

const EventRemoteEditChanged = "remote-edit:changed"

type EventEmitter func(name string, payload any)

type sessionSource interface {
	FileSystem(ctx context.Context, id string) (sessionapp.SessionFileSystem, error)
	TransferFileSystem(ctx context.Context, id string) (sessionapp.SessionTransferFileSystem, error)
	TransferFileSystemForSize(ctx context.Context, id string, expectedSize int64) (sessionapp.SessionTransferFileSystem, error)
	Get(ctx context.Context, id string) (model.Session, error)
}

type localEdit struct {
	mu sync.Mutex

	syncMu sync.Mutex

	id         string
	sessionID  string
	remotePath string
	localPath  string

	state     model.LocalEditState
	lastError string

	base baseline

	uploadedHash string

	lastLocalHash string

	pendingConflictHash string
	hasPendingConflict  bool

	cancelWatch context.CancelFunc
}

func (e *localEdit) editID() string        { return e.id }
func (e *localEdit) session() string       { return e.sessionID }
func (e *localEdit) remote() string        { return e.remotePath }
func (e *localEdit) localFilePath() string { return e.localPath }

func (e *localEdit) current() model.LocalEdit {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.currentLocked()
}

func (e *localEdit) update(state model.LocalEditState, lastError string) model.LocalEdit {
	e.mu.Lock()
	e.state = state
	e.lastError = lastError
	snapshot := e.currentLocked()
	e.mu.Unlock()
	return snapshot
}

func (e *localEdit) baseline() baseline {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.base
}

func (e *localEdit) uploadedHashValue() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.uploadedHash
}

func (e *localEdit) noteLocalSample(hash string) {
	if e == nil || hash == "" {
		return
	}
	e.mu.Lock()
	e.lastLocalHash = hash
	e.mu.Unlock()
}

func (e *localEdit) hasUnsyncedChanges() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.unsyncedChangesLocked()
}

func (e *localEdit) unsyncedChangesLocked() bool {
	return e.lastLocalHash != "" && e.lastLocalHash != e.uploadedHash
}

func (e *localEdit) markSynced(base baseline, uploadedHash string) model.LocalEdit {
	e.mu.Lock()
	e.base = base
	e.uploadedHash = uploadedHash

	e.lastLocalHash = uploadedHash
	e.hasPendingConflict = false
	e.pendingConflictHash = ""
	e.state = model.LocalEditStateSynced
	e.lastError = ""
	snapshot := e.currentLocked()
	e.mu.Unlock()
	return snapshot
}

func (e *localEdit) markRedownloaded(base baseline) model.LocalEdit {
	e.mu.Lock()
	e.base = base
	e.uploadedHash = base.Hash
	e.lastLocalHash = base.Hash
	e.hasPendingConflict = false
	e.pendingConflictHash = ""
	e.state = model.LocalEditStateWatching
	e.lastError = ""
	snapshot := e.currentLocked()
	e.mu.Unlock()
	return snapshot
}

func (e *localEdit) noteExternalRemoteWrite(hash string, size int64) model.LocalEdit {
	e.mu.Lock()
	e.base = baseline{Hash: hash, Size: size}
	e.uploadedHash = hash
	e.hasPendingConflict = false
	e.pendingConflictHash = ""
	if e.state == model.LocalEditStateError {
		e.state = model.LocalEditStateSynced
		e.lastError = ""
	}
	snapshot := e.currentLocked()
	e.mu.Unlock()
	return snapshot
}

func (e *localEdit) currentLocked() model.LocalEdit {
	return model.LocalEdit{
		ID:           e.id,
		SessionID:    e.sessionID,
		RemotePath:   e.remotePath,
		LocalPath:    e.localPath,
		State:        e.state,
		BaselineHash: e.base.Hash,
		LastError:    e.lastError,

		HasUnsyncedChanges: e.unsyncedChangesLocked(),
	}
}

func (e *localEdit) recordConflict(currentHash string) {
	e.mu.Lock()
	if !e.hasPendingConflict {
		e.pendingConflictHash = currentHash
	}
	e.hasPendingConflict = true
	e.mu.Unlock()
}

func (e *localEdit) conflictAllowsOverwrite(currentHash string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.hasPendingConflict {
		return false
	}
	return e.pendingConflictHash == currentHash
}

func (e *localEdit) clearConflict() {
	e.mu.Lock()
	e.hasPendingConflict = false
	e.pendingConflictHash = ""
	e.mu.Unlock()
}

func (e *localEdit) attachWatcher(cancel context.CancelFunc) {
	e.mu.Lock()
	e.cancelWatch = cancel
	e.mu.Unlock()
}

func (e *localEdit) detachWatcher() {
	e.mu.Lock()
	cancel := e.cancelWatch
	e.cancelWatch = nil
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

type Manager struct {
	sessions sessionSource

	mu      sync.RWMutex
	edits   map[string]*localEdit
	root    string
	started bool

	runCtx    context.Context
	cancelRun context.CancelFunc
	wg        sync.WaitGroup

	emitter    EventEmitter
	editorPath func() string
	newWatcher newWatcherFunc
	now        func() time.Time
	openEditor func(editorPath string, localPath string) error

	debounce  time.Duration
	stability time.Duration
}

func NewManager(sessions sessionSource) *Manager {
	return &Manager{
		sessions:   sessions,
		edits:      make(map[string]*localEdit),
		newWatcher: newFsnotifyWatcher,
		now:        time.Now,
		openEditor: startLocalEditor,
	}
}

func (m *Manager) SetEventEmitter(emitter EventEmitter) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.emitter = emitter
	m.mu.Unlock()
}

func (m *Manager) SetEditorPathSource(source func() string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.editorPath = source
	m.mu.Unlock()
}

func (m *Manager) Start(ctx context.Context) error {
	if m == nil {
		return errors.New("remote edit manager is nil")
	}
	if ctx == nil {
		return errors.New("remote edit start context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.sessions == nil {
		return errno.New(errno.CodeStoreFailed, messageKeyServiceNotReady, "")
	}
	root, err := RemoteEditRoot()
	if err != nil {
		return errno.Wrap(errno.CodeStoreFailed, messageKeyCacheUnavailable, err)
	}
	return m.startWithRoot(ctx, root)
}

func (m *Manager) startWithRoot(ctx context.Context, root string) error {
	if err := ensurePrivateDirectory(root); err != nil {
		return errno.Wrap(errno.CodeStoreFailed, messageKeyCacheUnavailable, err)
	}

	if err := cleanOrphanDirectories(root, nil); err != nil {
		slog.Warn("remote edit orphan cleanup failed",
			"event", "remote_edit.orphan_cleanup_failed", "error", err)
	}
	runCtx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		cancel()
		return nil
	}
	m.root = root
	m.runCtx = runCtx
	m.cancelRun = cancel
	m.started = true
	m.mu.Unlock()
	return nil
}

func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return nil
	}
	m.started = false
	cancel := m.cancelRun
	m.cancelRun = nil
	edits := make([]*localEdit, 0, len(m.edits))
	for _, edit := range m.edits {
		edits = append(edits, edit)
	}
	m.edits = make(map[string]*localEdit)
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	for _, edit := range edits {
		edit.detachWatcher()
	}
	m.wg.Wait()
	for _, edit := range edits {
		if edit.current().State != model.LocalEditStateSynced {
			continue
		}
		if err := removeLocalEditFile(edit.localFilePath()); err != nil {
			slog.Warn("remote edit local cleanup failed",
				"event", "remote_edit.local_cleanup_failed", "error", err)
		}
	}
	return nil
}

func (m *Manager) List() []model.LocalEdit {
	if m == nil {
		return []model.LocalEdit{}
	}
	m.mu.RLock()
	result := make([]model.LocalEdit, 0, len(m.edits))
	for _, edit := range m.edits {
		result = append(result, edit.current())
	}
	m.mu.RUnlock()
	sort.Slice(result, func(left, right int) bool { return result[left].ID < result[right].ID })
	return result
}

func (m *Manager) Open(ctx context.Context, sessionID string, remotePath string) (model.LocalEdit, error) {
	if err := m.ready(); err != nil {
		return model.LocalEdit{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return model.LocalEdit{}, errno.New(errno.CodeInvalidArg, messageKeySessionNotFound, "")
	}
	normalized, err := normalizeRemoteFile(remotePath)
	if err != nil {
		return model.LocalEdit{}, err
	}

	if existing := m.findByTarget(sessionID, normalized); existing != nil {
		return m.reopen(ctx, existing)
	}

	localPath := m.localPathFor(sessionID, normalized)
	base, err := m.downloadRemoteFile(ctx, sessionID, normalized, localPath)
	if err != nil {
		return model.LocalEdit{}, err
	}
	editID, err := randomEditID()
	if err != nil {
		return model.LocalEdit{}, errno.Wrap(errno.CodeUnknown, messageKeyUnknown, err)
	}
	edit := &localEdit{
		id:            editID,
		sessionID:     sessionID,
		remotePath:    normalized,
		localPath:     localPath,
		state:         model.LocalEditStateWatching,
		base:          base,
		uploadedHash:  base.Hash,
		lastLocalHash: base.Hash,
	}
	m.register(edit)
	m.emitChanged(edit.current())

	_ = m.launchEditor(edit)
	m.startWatcher(edit)
	return edit.current(), nil
}

func (m *Manager) downloadRemoteFile(
	ctx context.Context,
	sessionID string,
	remotePath string,
	localPath string,
) (baseline, error) {
	browsing, err := m.sessions.FileSystem(ctx, sessionID)
	if err != nil {
		return baseline{}, sessionError(err)
	}
	info, statErr := browsing.StatContext(ctx, remotePath)
	_ = browsing.Close()
	if statErr != nil {
		return baseline{}, m.remoteError(ctx, sessionID, statErr, messageKeyStatFailed)
	}
	if !info.Mode().IsRegular() {
		return baseline{}, errno.New(errno.CodeInvalidArg, messageKeyRegularOnly, "")
	}
	size := info.Size()
	modTime := info.ModTime().UTC()

	if err := ensurePrivateDirectory(filepath.Dir(localPath)); err != nil {
		return baseline{}, errno.Wrap(errno.CodeStoreFailed, messageKeyLocalWriteFailed, err)
	}

	transfer, err := m.sessions.TransferFileSystemForSize(ctx, sessionID, size)
	if err != nil {
		return baseline{}, sessionError(err)
	}
	downloadErr := transfer.DownloadFileContext(ctx, remotePath, localPath, size, true, noTransferProgress)
	_ = transfer.Close()
	if downloadErr != nil {
		return baseline{}, m.remoteError(ctx, sessionID, downloadErr, messageKeyReadFailed)
	}

	hash, err := hashFile(localPath)
	if err != nil {
		return baseline{}, errno.Wrap(errno.CodeStoreFailed, messageKeyLocalWriteFailed, err)
	}
	return baseline{Hash: hash, Size: size, ModTime: modTime}, nil
}

func (m *Manager) launchEditor(edit *localEdit) error {
	if edit == nil {
		return nil
	}
	if err := m.openEditor(m.editorPathValue(), edit.localFilePath()); err != nil {
		logLocalFailure(messageKeyEditorFailed, err)
		m.setState(edit, model.LocalEditStateError, messageKeyEditorFailed)
		return err
	}

	if edit.current().LastError == messageKeyEditorFailed {
		m.setState(edit, model.LocalEditStateWatching, "")
	}
	return nil
}

func (m *Manager) reopen(ctx context.Context, edit *localEdit) (model.LocalEdit, error) {
	localPath := edit.localFilePath()
	info, statErr := os.Stat(localPath)
	filePresent := statErr == nil && info.Mode().IsRegular()

	if filePresent {

		_ = m.launchEditor(edit)
		return edit.current(), nil
	}

	if edit.hasUnsyncedChanges() {

		cause := statErr
		if cause == nil {
			cause = os.ErrNotExist
		}

		logCause := cause
		if errors.Is(cause, os.ErrNotExist) {
			logCause = os.ErrNotExist
		}
		logLocalFailure(messageKeyLocalMissing, logCause)
		m.setState(edit, model.LocalEditStateError, messageKeyLocalMissing)
		return model.LocalEdit{}, errno.Wrap(errno.CodeNotFound, messageKeyLocalMissing, cause)
	}

	base, err := m.downloadRemoteFile(ctx, edit.session(), edit.remote(), localPath)
	if err != nil {

		failureKey := messageKeyReadFailed
		if appErr := errno.Extract(err); appErr != nil {
			failureKey = appErr.MessageKey
		}
		m.setState(edit, model.LocalEditStateError, failureKey)
		return model.LocalEdit{}, err
	}
	m.emitChanged(edit.markRedownloaded(base))
	_ = m.launchEditor(edit)
	return edit.current(), nil
}

func (m *Manager) Sync(ctx context.Context, editID string) (model.LocalEdit, error) {
	if err := m.ready(); err != nil {
		return model.LocalEdit{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return m.sync(ctx, strings.TrimSpace(editID), syncOptions{explicit: true})
}

type syncOptions struct {
	explicit bool
}

func (m *Manager) Discard(editID string) error {
	if err := m.ready(); err != nil {
		return err
	}
	id := strings.TrimSpace(editID)
	if id == "" {
		return errno.New(errno.CodeInvalidArg, messageKeyNotFound, "")
	}
	m.mu.Lock()
	edit := m.edits[id]
	if edit != nil {
		delete(m.edits, id)
	}
	m.mu.Unlock()
	if edit == nil {
		return errno.New(errno.CodeNotFound, messageKeyNotFound, "")
	}
	edit.detachWatcher()

	edit.syncMu.Lock()
	edit.syncMu.Unlock()

	m.setState(edit, model.LocalEditStateStopped, "")
	if err := removeLocalEditFile(edit.localFilePath()); err != nil {
		return errno.Wrap(errno.CodeStoreFailed, messageKeyLocalWriteFailed, err)
	}
	return nil
}

func (m *Manager) ReadText(ctx context.Context, sessionID string, remotePath string) (model.RemoteFileContent, error) {
	if err := m.ready(); err != nil {
		return model.RemoteFileContent{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return model.RemoteFileContent{}, errno.New(errno.CodeInvalidArg, messageKeySessionNotFound, "")
	}
	normalized, err := normalizeRemoteFile(remotePath)
	if err != nil {
		return model.RemoteFileContent{}, err
	}
	fileSystem, err := m.sessions.FileSystem(ctx, sessionID)
	if err != nil {
		return model.RemoteFileContent{}, sessionError(err)
	}
	defer fileSystem.Close()
	access, ok := fileSystem.(remoteFileAccess)
	if !ok {
		return model.RemoteFileContent{}, errno.New(errno.CodeInvalidArg, messageKeyReadUnsupported, "")
	}
	info, statErr := access.StatContext(ctx, normalized)
	if statErr != nil {
		return model.RemoteFileContent{}, m.remoteError(ctx, sessionID, statErr, messageKeyStatFailed)
	}
	if !info.Mode().IsRegular() {
		return model.RemoteFileContent{}, errno.New(errno.CodeInvalidArg, messageKeyRegularOnly, "")
	}
	size := info.Size()
	modTime := info.ModTime().UTC()
	if size > maxRemoteTextBytes {
		return model.RemoteFileContent{Size: size, Truncated: true, ModTime: modTime}, nil
	}
	reader, openErr := access.OpenFile(ctx, normalized)
	if openErr != nil {
		return model.RemoteFileContent{}, m.remoteError(ctx, sessionID, openErr, messageKeyReadFailed)
	}
	defer reader.Close()
	data, truncated, readErr := readUpTo(reader, maxRemoteTextBytes)
	if readErr != nil {
		return model.RemoteFileContent{}, m.remoteError(ctx, sessionID, readErr, messageKeyReadFailed)
	}
	if truncated {

		return model.RemoteFileContent{Size: size, Truncated: true, ModTime: modTime}, nil
	}
	return model.RemoteFileContent{
		ContentBase64: encodeBase64(data),
		Size:          size,
		Binary:        looksBinary(data),
		Truncated:     false,
		Hash:          hashBytes(data),
		ModTime:       modTime,
	}, nil
}

func (m *Manager) WriteText(
	ctx context.Context,
	sessionID string,
	remotePath string,
	contentBase64 string,
	expectedHash string,
) (model.RemoteFileWriteResult, error) {
	if err := m.ready(); err != nil {
		return model.RemoteFileWriteResult{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return model.RemoteFileWriteResult{}, errno.New(errno.CodeInvalidArg, messageKeySessionNotFound, "")
	}
	normalized, err := normalizeRemoteFile(remotePath)
	if err != nil {
		return model.RemoteFileWriteResult{}, err
	}
	data, err := decodeBase64(contentBase64)
	if err != nil {
		return model.RemoteFileWriteResult{}, err
	}
	if len(data) > maxRemoteTextBytes {
		return model.RemoteFileWriteResult{}, errno.New(errno.CodeInvalidArg, messageKeyTooLarge, "")
	}
	transfer, err := m.sessions.TransferFileSystem(ctx, sessionID)
	if err != nil {
		return model.RemoteFileWriteResult{}, sessionError(err)
	}
	defer transfer.Close()
	access, ok := transfer.(remoteFileAccess)
	if !ok {
		return model.RemoteFileWriteResult{}, errno.New(errno.CodeInvalidArg, messageKeyWriteUnsupported, "")
	}
	writer, ok := transfer.(remoteAtomicWriter)
	if !ok {
		return model.RemoteFileWriteResult{}, errno.New(errno.CodeInvalidArg, messageKeyWriteUnsupported, "")
	}
	observed, truncated, observeErr := observeRemote(ctx, access, normalized, maxRemoteTextBytes)
	if observeErr != nil {
		return model.RemoteFileWriteResult{}, m.remoteError(ctx, sessionID, observeErr, messageKeyReadFailed)
	}
	if truncated {

		return model.RemoteFileWriteResult{Conflict: true}, nil
	}
	allowed, currentHash := decideExpectedWrite(strings.TrimSpace(expectedHash), observed)
	if !allowed {
		return model.RemoteFileWriteResult{Conflict: true, CurrentHash: currentHash}, nil
	}
	if err := writeRemoteBytes(ctx, writer, normalized, data); err != nil {
		return model.RemoteFileWriteResult{}, m.remoteError(ctx, sessionID, err, messageKeyWriteFailed)
	}

	m.noteExternalRemoteWrite(sessionID, normalized, hashBytes(data), int64(len(data)))
	return model.RemoteFileWriteResult{Saved: true, CurrentHash: hashBytes(data)}, nil
}

func (m *Manager) sync(ctx context.Context, editID string, options syncOptions) (model.LocalEdit, error) {
	edit := m.lookup(editID)
	if edit == nil {
		return model.LocalEdit{}, errno.New(errno.CodeNotFound, messageKeyNotFound, "")
	}

	edit.syncMu.Lock()
	defer edit.syncMu.Unlock()

	m.setState(edit, model.LocalEditStateSyncing, "")
	sessionID := edit.session()

	localInfo, err := os.Stat(edit.localFilePath())
	if err != nil || !localInfo.Mode().IsRegular() {

		m.setState(edit, model.LocalEditStateError, messageKeyLocalMissing)
		return edit.current(), nil
	}
	localHash, err := hashFile(edit.localFilePath())
	if err != nil {
		m.setState(edit, model.LocalEditStateError, messageKeyLocalMissing)
		return edit.current(), nil
	}

	edit.noteLocalSample(localHash)

	transfer, err := m.sessions.TransferFileSystemForSize(ctx, sessionID, localInfo.Size())
	if err != nil {
		m.setState(edit, model.LocalEditStateError, translateFailureCode(sessionError(err)))
		return edit.current(), nil
	}
	defer transfer.Close()
	access, ok := transfer.(remoteFileAccess)
	if !ok {
		m.setState(edit, model.LocalEditStateError, messageKeyReadUnsupported)
		return edit.current(), nil
	}
	writer, ok := transfer.(remoteAtomicWriter)
	if !ok {
		m.setState(edit, model.LocalEditStateError, messageKeyWriteUnsupported)
		return edit.current(), nil
	}

	base := edit.baseline()
	observed, truncated, observeErr := observeRemote(ctx, access, edit.remote(), maxRemoteTextBytes)
	if observeErr != nil {
		m.setState(edit, model.LocalEditStateError, translateFailureCode(m.remoteError(ctx, sessionID, observeErr, messageKeyStatFailed)))
		return edit.current(), nil
	}
	if truncated {

		edit.recordConflict("")
		m.setState(edit, model.LocalEditStateError, messageKeyConflict)
		return edit.current(), nil
	}
	decision, currentHash := decideSave(base, observed)
	if decision == saveNeedContentHash {
		observed.Hash, err = hashRemote(ctx, access, edit.remote(), maxRemoteTextBytes)
		if err != nil {
			m.setState(edit, model.LocalEditStateError, translateFailureCode(m.remoteError(ctx, sessionID, err, messageKeyReadFailed)))
			return edit.current(), nil
		}
		decision, currentHash = decideSave(base, observed)
	}
	if decision == saveConflict {
		if options.explicit && edit.conflictAllowsOverwrite(currentHash) {

			edit.clearConflict()
		} else {
			edit.recordConflict(currentHash)
			m.setState(edit, model.LocalEditStateError, messageKeyConflict)
			return edit.current(), nil
		}
	}

	if localHash == base.Hash {
		m.markSynced(edit, base, localHash)
		return edit.current(), nil
	}

	if err := writeLocalFileAtomic(ctx, writer, edit.localFilePath(), edit.remote()); err != nil {
		m.setState(edit, model.LocalEditStateError, translateFailureCode(m.remoteError(ctx, sessionID, err, messageKeyWriteFailed)))
		return edit.current(), nil
	}

	newBase := baseline{Hash: localHash, Size: localInfo.Size()}
	if info, statErr := access.StatContext(ctx, edit.remote()); statErr == nil {
		newBase.Size = info.Size()
		newBase.ModTime = info.ModTime().UTC()
	}
	m.markSynced(edit, newBase, localHash)
	return edit.current(), nil
}

func hashRemote(ctx context.Context, access remoteFileAccess, remotePath string, limit int64) (string, error) {
	observed, truncated, err := observeRemote(ctx, access, remotePath, limit)
	if err != nil {
		return "", err
	}
	if truncated || !observed.Exists {
		return "", nil
	}
	return observed.Hash, nil
}

func writeRemoteBytes(ctx context.Context, writer remoteAtomicWriter, remotePath string, data []byte) (err error) {
	temporaryPath, sink, err := writer.CreateTransferFile(ctx, remotePath)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			_ = sink.Close()
		}
		if err != nil {

			_ = writer.RemoveTransferFile(context.WithoutCancel(ctx), temporaryPath)
		}
	}()
	written := 0
	for written < len(data) {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		count, writeErr := sink.Write(data[written:])
		written += count
		if writeErr != nil {
			return writeErr
		}
		if count == 0 {
			return errno.New(errno.CodeConnFailed, messageKeyWriteFailed, "")
		}
	}
	if closeErr := sink.Close(); closeErr != nil {
		closed = true
		return closeErr
	}
	closed = true

	if err = writer.CommitTransferFile(ctx, temporaryPath, remotePath, true); err != nil {
		return err
	}
	return nil
}

func writeLocalFileAtomic(ctx context.Context, writer remoteAtomicWriter, localPath string, remotePath string) (err error) {
	temporaryPath, sink, err := writer.CreateTransferFile(ctx, remotePath)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			_ = sink.Close()
		}
		if err != nil {
			_ = writer.RemoveTransferFile(context.WithoutCancel(ctx), temporaryPath)
		}
	}()
	if _, err = copyLocalFileToWriter(ctx, localPath, sink); err != nil {
		return err
	}
	if closeErr := sink.Close(); closeErr != nil {
		closed = true
		return closeErr
	}
	closed = true
	if err = writer.CommitTransferFile(ctx, temporaryPath, remotePath, true); err != nil {
		return err
	}
	return nil
}

func (m *Manager) noteExternalRemoteWrite(sessionID string, remotePath string, hash string, size int64) {
	m.mu.RLock()
	candidates := make([]*localEdit, 0, len(m.edits))
	for _, edit := range m.edits {
		candidates = append(candidates, edit)
	}
	m.mu.RUnlock()
	for _, edit := range candidates {
		if edit.session() != sessionID || edit.remote() != remotePath {
			continue
		}
		m.emitChanged(edit.noteExternalRemoteWrite(hash, size))
	}
}

func (m *Manager) register(edit *localEdit) {
	if m == nil || edit == nil {
		return
	}
	m.mu.Lock()
	if m.edits == nil {
		m.edits = make(map[string]*localEdit)
	}
	if _, exists := m.edits[edit.id]; !exists {
		m.edits[edit.id] = edit
	}
	m.mu.Unlock()
}

func (m *Manager) findByTarget(sessionID string, remotePath string) *localEdit {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, edit := range m.edits {
		if edit.session() == sessionID && edit.remote() == remotePath {
			return edit
		}
	}
	return nil
}

func (m *Manager) lookup(editID string) *localEdit {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.edits[editID]
}

func (m *Manager) ready() error {
	if m == nil {
		return errno.New(errno.CodeStoreFailed, messageKeyServiceNotReady, "")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.started || m.sessions == nil {
		return errno.New(errno.CodeStoreFailed, messageKeyServiceNotReady, "")
	}
	return nil
}

func (m *Manager) cacheRoot() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.root
}

func (m *Manager) runContext() context.Context {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.runCtx == nil {
		return context.Background()
	}
	return m.runCtx
}

func (m *Manager) editorPathValue() string {
	m.mu.RLock()
	source := m.editorPath
	m.mu.RUnlock()
	if source == nil {
		return ""
	}
	return strings.TrimSpace(source())
}

func (m *Manager) setState(edit *localEdit, state model.LocalEditState, lastError string) {
	if edit == nil {
		return
	}
	m.emitChanged(edit.update(state, lastError))
}

func (m *Manager) emitChanged(snapshot model.LocalEdit) {
	m.mu.RLock()
	emitter := m.emitter
	m.mu.RUnlock()
	if emitter == nil {
		return
	}
	emitter(EventRemoteEditChanged, model.RemoteEditChanged{
		EditID:    snapshot.ID,
		State:     snapshot.State,
		LastError: snapshot.LastError,

		HasUnsyncedChanges: snapshot.HasUnsyncedChanges,
	})
}

func (m *Manager) markSynced(edit *localEdit, base baseline, uploadedHash string) {
	m.emitChanged(edit.markSynced(base, uploadedHash))
}

func randomEditID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func normalizeRemoteFile(remotePath string) (string, error) {
	normalized, err := model.NormalizeRemotePath(strings.TrimSpace(remotePath))
	if err != nil || normalized == "/" {
		return "", errno.New(errno.CodeInvalidArg, messageKeyPathInvalid, "")
	}
	return normalized, nil
}

func noTransferProgress(int64) error { return nil }

func translateFailureCode(err error) string {
	if appErr := errno.Extract(err); appErr != nil {
		return appErr.MessageKey
	}
	return messageKeyWriteFailed
}
