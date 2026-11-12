package terminal

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
)

type syncGroup struct {
	state SyncGroupState
}

type batchTask struct {
	mu     sync.Mutex
	state  BatchTaskState
	cancel context.CancelFunc
}

type Manager struct {
	mu            sync.Mutex
	routeMu       sync.Mutex
	root          context.Context
	closed        bool
	directory     SessionDirectory
	writer        *sessionWriter
	groups        map[string]*syncGroup
	attached      map[string]map[string]struct{}
	sessionWindow map[string]string
	tasks         map[string]*batchTask
	leases        map[string]uint64
	wg            sync.WaitGroup
	errors        map[string]string
	nextID        atomic.Uint64
}

func NewManager(parent context.Context, directory SessionDirectory, write SessionWriteFunc) *Manager {
	if parent == nil {
		parent = context.Background()
	}
	control := &Manager{
		root: parent, directory: directory,
		groups: make(map[string]*syncGroup), attached: make(map[string]map[string]struct{}),
		sessionWindow: make(map[string]string), tasks: make(map[string]*batchTask), errors: make(map[string]string), leases: make(map[string]uint64),
	}
	control.writer = newSessionWriter(parent, write, control.recordWriteFailure)
	return control
}

func (c *Manager) Attach(windowID, sessionID string) error {
	_, err := c.bind(windowID, sessionID)
	return err
}

func (c *Manager) bind(windowID, sessionID string) (uint64, error) {
	windowID, sessionID = strings.TrimSpace(windowID), strings.TrimSpace(sessionID)
	if windowID == "" {
		return 0, ErrWindowRequired
	}
	if sessionID == "" {
		return 0, ErrSessionRequired
	}
	c.routeMu.Lock()
	defer c.routeMu.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, ErrControlNotReady
	}
	if oldWindow := c.sessionWindow[sessionID]; oldWindow != "" {

		if oldWindow != windowID {
			c.detachLocked(oldWindow, sessionID)
		}
	}
	if c.attached[windowID] == nil {
		c.attached[windowID] = make(map[string]struct{})
	}
	c.attached[windowID][sessionID] = struct{}{}
	c.sessionWindow[sessionID] = windowID
	lease := c.nextID.Add(1)
	c.leases[sessionID] = lease
	return lease, nil
}

type boundInputRouter struct {
	*Manager
	lease uint64
}

func (c *Manager) Bind(windowID, sessionID string) (InputRouter, func(), error) {
	lease, err := c.bind(windowID, sessionID)
	if err != nil {
		return nil, nil, err
	}
	return &boundInputRouter{Manager: c, lease: lease}, func() { c.detachLease(windowID, sessionID, lease) }, nil
}

func (r *boundInputRouter) LocalInput(ctx context.Context, w, s string, data []byte) error {
	return r.Manager.localInput(ctx, w, s, data, r.lease)
}
func (r *boundInputRouter) DirectInput(ctx context.Context, w, s string, data []byte) error {
	return r.Manager.directInput(ctx, w, s, data, r.lease)
}

func (c *Manager) Detach(windowID, sessionID string) { c.detachLease(windowID, sessionID, 0) }
func (c *Manager) detachLease(windowID, sessionID string, lease uint64) {
	c.routeMu.Lock()
	defer c.routeMu.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sessionWindow[sessionID] == windowID && (lease == 0 || c.leases[sessionID] == lease) {
		c.detachLocked(windowID, sessionID)
	}
}

func (c *Manager) detachLocked(windowID, sessionID string) {
	if members := c.attached[windowID]; members != nil {
		delete(members, sessionID)
		if len(members) == 0 {
			delete(c.attached, windowID)
		}
	}
	delete(c.sessionWindow, sessionID)
	delete(c.leases, sessionID)
	c.writer.retire(sessionID)
	if group := c.groups[windowID]; group != nil {
		if removeID(&group.state.SessionIDs, sessionID) {
			group.state.Revision++
			if len(group.state.SessionIDs) < 2 {
				delete(c.groups, windowID)
			}
		}
	}

	for _, task := range c.tasks {
		if task.state.WindowID == windowID && len(c.attached[windowID]) == 0 {
			task.cancel()
		}
	}
}

func (c *Manager) LocalInput(ctx context.Context, windowID, sourceID string, data []byte) error {
	return c.localInput(ctx, windowID, sourceID, data, 0)
}
func (c *Manager) localInput(ctx context.Context, windowID, sourceID string, data []byte, lease uint64) error {
	windowID, sourceID = strings.TrimSpace(windowID), strings.TrimSpace(sourceID)
	if windowID == "" {
		return ErrWindowRequired
	}
	if sourceID == "" || len(data) == 0 {
		return ErrSessionRequired
	}

	c.routeMu.Lock()
	defer c.routeMu.Unlock()
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrControlNotReady
	}
	if !c.isAttachedLocked(windowID, sourceID) || (lease != 0 && c.leases[sourceID] != lease) {
		c.mu.Unlock()
		return ErrSessionNotEligible
	}
	targets := []string{sourceID}
	if group := c.groups[windowID]; group != nil && group.state.Status == SyncStatusActive && contains(group.state.SessionIDs, sourceID) {
		targets = append([]string(nil), group.state.SessionIDs...)
		group.state.LastSourceSessionID = sourceID
	}

	failures := make(map[string]error)
	for _, target := range targets {
		if !c.isAttachedLocked(windowID, target) {
			continue
		}
		if err := c.writer.enqueue(ctx, target, data); err != nil {
			failures[target] = err
		}
	}
	c.mu.Unlock()
	var sourceErr error
	for target, err := range failures {
		c.recordWriteFailure(target, err)
		if target == sourceID {
			sourceErr = err
		}
	}

	return sourceErr
}

func (c *Manager) DirectInput(ctx context.Context, windowID, sourceID string, data []byte) error {
	return c.directInput(ctx, windowID, sourceID, data, 0)
}
func (c *Manager) directInput(ctx context.Context, windowID, sourceID string, data []byte, lease uint64) error {
	windowID, sourceID = strings.TrimSpace(windowID), strings.TrimSpace(sourceID)
	if windowID == "" {
		return ErrWindowRequired
	}
	c.routeMu.Lock()
	defer c.routeMu.Unlock()
	c.mu.Lock()
	ok := !c.closed && c.isAttachedLocked(windowID, sourceID) && (lease == 0 || c.leases[sourceID] == lease)
	if !ok {
		c.mu.Unlock()
		return ErrSessionNotEligible
	}
	err := c.writer.enqueue(ctx, sourceID, data)
	c.mu.Unlock()
	if err != nil {
		c.recordWriteFailure(sourceID, err)
		return err
	}

	return nil
}

func (c *Manager) recordWriteFailure(sessionID string, _ error) {
	c.writer.retire(sessionID)

	c.mu.Lock()
	defer c.mu.Unlock()
	if windowID := c.sessionWindow[sessionID]; windowID != "" {
		if members := c.attached[windowID]; members != nil {
			delete(members, sessionID)
		}
		delete(c.sessionWindow, sessionID)
		c.errors[windowID] = "terminalControl.error"
	}
	for windowID, group := range c.groups {
		if !contains(group.state.SessionIDs, sessionID) {
			continue
		}
		removeID(&group.state.SessionIDs, sessionID)
		group.state.Error = "terminalControl.error"
		group.state.Revision++
		if len(group.state.SessionIDs) < 2 {
			delete(c.groups, windowID)
		}
		c.errors[windowID] = "terminalControl.error"
	}
}

func (c *Manager) Get(ctx context.Context, windowID string) (ControlSnapshot, error) {
	windowID = strings.TrimSpace(windowID)
	if windowID == "" {
		return ControlSnapshot{}, ErrWindowRequired
	}
	sessions, err := c.eligible(ctx, windowID)
	if err != nil {
		return ControlSnapshot{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snapshotLocked(windowID, sessions), nil
}

func (c *Manager) SetSync(ctx context.Context, windowID string, request SyncRequest) (ControlSnapshot, error) {
	windowID = strings.TrimSpace(windowID)
	if windowID == "" {
		return ControlSnapshot{}, ErrWindowRequired
	}
	sessions, err := c.eligible(ctx, windowID)
	if err != nil {
		return ControlSnapshot{}, err
	}
	eligible := make(map[string]struct{}, len(sessions))
	for _, item := range sessions {
		eligible[item.ID] = struct{}{}
	}
	ids := uniqueIDs(request.SessionIDs)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ControlSnapshot{}, ErrControlNotReady
	}
	for _, id := range ids {
		if _, ok := eligible[id]; !ok || !c.isAttachedLocked(windowID, id) {
			c.mu.Unlock()
			return ControlSnapshot{}, ErrSessionNotEligible
		}
	}
	group := c.groups[windowID]
	switch request.Action {
	case SyncActionCreate:
		if group != nil {
			c.mu.Unlock()
			return ControlSnapshot{}, ErrSyncRevisionConflict
		}
		if len(ids) < 2 {
			c.mu.Unlock()
			return ControlSnapshot{}, ErrSyncMemberRequired
		}
		if err := validateMembers(ids, eligible); err != nil {
			c.mu.Unlock()
			return ControlSnapshot{}, err
		}
		if group == nil {
			group = &syncGroup{state: SyncGroupState{ID: c.newID("sync"), WindowID: windowID, Status: SyncStatusActive, Revision: 1}}
			c.groups[windowID] = group
		}
		group.state.SessionIDs = append([]string(nil), ids...)
		group.state.Status = SyncStatusActive
		group.state.Error = ""
		delete(c.errors, windowID)
		group.state.Revision++
	case SyncActionMembers:
		if group == nil {
			c.mu.Unlock()
			return ControlSnapshot{}, ErrSyncGroupNotFound
		}
		if request.Revision != group.state.Revision {
			c.mu.Unlock()
			return ControlSnapshot{}, ErrSyncRevisionConflict
		}
		if len(ids) < 2 {
			delete(c.groups, windowID)
			break
		}
		if err := validateMembers(ids, eligible); err != nil {
			c.mu.Unlock()
			return ControlSnapshot{}, err
		}
		group.state.SessionIDs = append([]string(nil), ids...)
		group.state.Revision++
	case SyncActionJoin:
		if group == nil {
			c.mu.Unlock()
			return ControlSnapshot{}, ErrSyncGroupNotFound
		}
		if err := validateMembers(ids, eligible); err != nil {
			c.mu.Unlock()
			return ControlSnapshot{}, err
		}
		for _, id := range ids {
			if !contains(group.state.SessionIDs, id) {
				group.state.SessionIDs = append(group.state.SessionIDs, id)
			}
		}
		if len(group.state.SessionIDs) < 2 {
			c.mu.Unlock()
			return ControlSnapshot{}, ErrSyncMemberRequired
		}
		group.state.Revision++
	case SyncActionLeave:
		if group == nil {
			c.mu.Unlock()
			return ControlSnapshot{}, ErrSyncGroupNotFound
		}
		if request.Revision != 0 && request.Revision != group.state.Revision {
			c.mu.Unlock()
			return ControlSnapshot{}, ErrSyncRevisionConflict
		}
		for _, id := range ids {
			removeID(&group.state.SessionIDs, id)
		}
		group.state.Revision++
		if len(group.state.SessionIDs) < 2 {
			delete(c.groups, windowID)
		}
	case SyncActionPause, SyncActionResume:
		if group == nil {
			c.mu.Unlock()
			return ControlSnapshot{}, ErrSyncGroupNotFound
		}
		if request.Revision != 0 && request.Revision != group.state.Revision {
			c.mu.Unlock()
			return ControlSnapshot{}, ErrSyncRevisionConflict
		}
		if request.Action == SyncActionPause {
			group.state.Status = SyncStatusPaused
		} else {
			group.state.Status = SyncStatusActive
		}
		group.state.Revision++
	case SyncActionStop:
		if group == nil {
			c.mu.Unlock()
			return ControlSnapshot{}, ErrSyncGroupNotFound
		}
		delete(c.groups, windowID)
	default:
		c.mu.Unlock()
		return ControlSnapshot{}, ErrInvalidControlRequest
	}
	snapshot := c.snapshotLocked(windowID, sessions)
	c.mu.Unlock()
	return snapshot, nil
}
