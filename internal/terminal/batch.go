package terminal

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

func (c *Manager) StartBatch(ctx context.Context, windowID string, request BatchRequest) (BatchTaskState, error) {
	windowID = strings.TrimSpace(windowID)
	if windowID == "" {
		return BatchTaskState{}, ErrWindowRequired
	}
	if request.Content == "" {
		return BatchTaskState{}, ErrBatchContentRequired
	}
	if len(request.Content) > maxBatchContent {
		return BatchTaskState{}, ErrBatchContentRequired
	}
	if request.RepeatCount < 1 || request.RepeatCount > maxBatchRepeats {
		return BatchTaskState{}, ErrBatchRepeatOutOfRange
	}
	if request.IntervalMS < 0 || request.IntervalMS > int(maxBatchInterval/time.Millisecond) {
		return BatchTaskState{}, ErrBatchIntervalInvalid
	}
	data, err := batchBytes(request.Content, request.SendMode)
	if err != nil {
		return BatchTaskState{}, err
	}
	sessions, err := c.eligible(ctx, windowID)
	if err != nil {
		return BatchTaskState{}, err
	}
	eligible := make(map[string]struct{}, len(sessions))
	for _, item := range sessions {
		eligible[item.ID] = struct{}{}
	}
	ids := uniqueIDs(request.SessionIDs)
	if len(ids) == 0 {
		return BatchTaskState{}, ErrBatchTargetRequired
	}
	if len(ids) > maxBatchTargets {
		return BatchTaskState{}, ErrBatchTargetRequired
	}
	if err := validateMembers(ids, eligible); err != nil {
		return BatchTaskState{}, err
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return BatchTaskState{}, ErrControlNotReady
	}
	for _, id := range ids {
		if !c.isAttachedLocked(windowID, id) {
			c.mu.Unlock()
			return BatchTaskState{}, ErrSessionNotEligible
		}
	}
	for _, existing := range c.tasks {
		if existing.state.WindowID != windowID {
			continue
		}
		existing.mu.Lock()
		status := existing.state.Status
		existing.mu.Unlock()
		if status == BatchTaskQueued || status == BatchTaskRunning {
			c.mu.Unlock()
			return BatchTaskState{}, ErrBatchAlreadyRunning
		}
	}

	for id, existing := range c.tasks {
		if existing.state.WindowID != windowID {
			continue
		}
		existing.mu.Lock()
		status := existing.state.Status
		existing.mu.Unlock()
		if isBatchTaskTerminal(status) {
			delete(c.tasks, id)
		}
	}
	if group := c.groups[windowID]; group != nil && group.state.Status == SyncStatusActive && intersects(group.state.SessionIDs, ids) {
		if !request.PauseSync {
			c.mu.Unlock()
			return BatchTaskState{}, ErrBatchSyncConflict
		}

		group.state.Status = SyncStatusPaused
		group.state.Revision++
	}
	taskCtx, cancel := context.WithCancel(c.root)
	task := &batchTask{cancel: cancel, state: BatchTaskState{ID: c.newID("batch"), WindowID: windowID, SessionIDs: append([]string(nil), ids...), SendMode: request.SendMode, RepeatCount: request.RepeatCount, IntervalMS: request.IntervalMS, Status: BatchTaskQueued, Targets: make([]BatchTargetState, len(ids))}}
	for i, id := range ids {
		task.state.Targets[i] = BatchTargetState{SessionID: id, Status: "pending"}
	}
	c.tasks[task.state.ID] = task
	state := cloneBatchState(task.state)
	c.wg.Add(1)
	c.mu.Unlock()
	go c.runBatch(taskCtx, task, data)
	return state, nil
}

func isBatchTaskTerminal(status BatchTaskStatus) bool {
	return status == BatchTaskCompleted || status == BatchTaskPartiallyFailed || status == BatchTaskFailed || status == BatchTaskCancelled
}

func (c *Manager) runBatch(ctx context.Context, task *batchTask, data []byte) {
	defer c.wg.Done()
	defer task.cancel()
	task.mu.Lock()
	task.state.Status = BatchTaskRunning
	task.mu.Unlock()
	for round := 0; round < task.state.RepeatCount; round++ {
		if ctx.Err() != nil {
			task.mu.Lock()
			task.state.Status = BatchTaskCancelled
			task.mu.Unlock()
			return
		}
		type pendingWrite struct {
			done <-chan error
			err  error
		}
		pending := make([]pendingWrite, len(task.state.SessionIDs))

		c.routeMu.Lock()
		c.mu.Lock()
		for index, id := range task.state.SessionIDs {
			task.mu.Lock()
			failed := task.state.Targets[index].Status == "failed"
			task.mu.Unlock()
			if failed || c.closed || !c.isAttachedLocked(task.state.WindowID, id) {
				pending[index].err = ErrSessionNotEligible
				continue
			}
			pending[index].done, pending[index].err = c.writer.submit(ctx, id, data)
		}
		c.mu.Unlock()
		c.routeMu.Unlock()
		var wg sync.WaitGroup
		for index, item := range pending {
			wg.Add(1)
			go func(index int, item pendingWrite) {
				defer wg.Done()
				err := item.err
				if err == nil {
					select {
					case err = <-item.done:
					case <-ctx.Done():
						err = ctx.Err()
					}
				}
				task.mu.Lock()
				defer task.mu.Unlock()
				if err == nil {
					task.state.Targets[index].WrittenRounds++
					task.state.Targets[index].Status = "ok"
				} else if !errors.Is(err, context.Canceled) {
					task.state.Targets[index].Status = "failed"
					task.state.Targets[index].Error = "terminalControl.error"
				}
			}(index, item)
		}

		wg.Wait()
		if ctx.Err() != nil {
			task.mu.Lock()
			task.state.Status = BatchTaskCancelled
			task.mu.Unlock()
			return
		}
		task.mu.Lock()
		task.state.CompletedRounds = round + 1
		remaining := 0
		for _, target := range task.state.Targets {
			if target.Status != "failed" {
				remaining++
			}
		}
		if remaining == 0 {
			task.state.Status = BatchTaskFailed
			task.mu.Unlock()
			return
		}
		task.mu.Unlock()
		if round+1 < task.state.RepeatCount {
			timer := time.NewTimer(time.Duration(task.state.IntervalMS) * time.Millisecond)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				task.mu.Lock()
				task.state.Status = BatchTaskCancelled
				task.mu.Unlock()
				return
			}
		}
	}
	task.mu.Lock()
	failed := 0
	for _, target := range task.state.Targets {
		if target.Status == "failed" {
			failed++
		}
	}
	if failed == len(task.state.Targets) {
		task.state.Status = BatchTaskFailed
	} else if failed > 0 {
		task.state.Status = BatchTaskPartiallyFailed
	} else {
		task.state.Status = BatchTaskCompleted
	}
	task.mu.Unlock()
}

func (c *Manager) batchTargetsAttached(windowID string, ids []string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return false
	}
	for _, id := range ids {
		if !c.isAttachedLocked(windowID, id) {
			return false
		}
	}
	return true
}

func (c *Manager) CancelBatch(_ context.Context, windowID, taskID string) (BatchTaskState, error) {
	windowID, taskID = strings.TrimSpace(windowID), strings.TrimSpace(taskID)
	c.mu.Lock()
	task := c.tasks[taskID]
	c.mu.Unlock()
	if task == nil {
		return BatchTaskState{}, ErrBatchTaskNotFound
	}
	if task.state.WindowID != windowID {
		return BatchTaskState{}, ErrBatchTaskNotFound
	}
	task.cancel()
	task.mu.Lock()
	state := cloneBatchState(task.state)
	if state.Status == BatchTaskQueued {
		state.Status = BatchTaskCancelled
		task.state.Status = state.Status
	}
	task.mu.Unlock()
	return state, nil
}

func (c *Manager) snapshotLocked(windowID string, sessions []SessionInfo) ControlSnapshot {
	snapshot := ControlSnapshot{Sessions: append([]SessionInfo{}, sessions...), Tasks: make([]BatchTaskState, 0)}
	snapshot.Error = c.errors[windowID]
	if group := c.groups[windowID]; group != nil {
		copyState := group.state
		copyState.SessionIDs = append([]string(nil), group.state.SessionIDs...)
		snapshot.Sync = &copyState
	}
	for _, task := range c.tasks {
		if task.state.WindowID != windowID {
			continue
		}
		task.mu.Lock()
		snapshot.Tasks = append(snapshot.Tasks, cloneBatchState(task.state))
		task.mu.Unlock()
	}
	sort.Slice(snapshot.Tasks, func(i, j int) bool { return snapshot.Tasks[i].ID < snapshot.Tasks[j].ID })
	return snapshot
}

func (c *Manager) eligible(ctx context.Context, windowID string) ([]SessionInfo, error) {
	if c.directory == nil {
		return nil, ErrControlNotReady
	}
	items, err := c.directory.List(ctx)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	attached := make(map[string]struct{}, len(c.attached[windowID]))
	for id := range c.attached[windowID] {
		attached[id] = struct{}{}
	}
	c.mu.Unlock()
	result := make([]SessionInfo, 0, len(items))
	for _, item := range items {
		if _, ok := attached[item.ID]; !ok || item.State != "connected" || item.Purpose != "terminal" || item.Kind != "ssh" {
			continue
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (c *Manager) isAttachedLocked(windowID, sessionID string) bool {
	_, ok := c.attached[windowID][sessionID]
	return ok
}

func (c *Manager) newID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, c.nextID.Add(1))
}

func (c *Manager) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		c.writer.close()
		c.wg.Wait()
		return nil
	}
	c.closed = true
	tasks := make([]*batchTask, 0, len(c.tasks))
	for _, task := range c.tasks {
		tasks = append(tasks, task)
	}
	c.groups = make(map[string]*syncGroup)
	c.mu.Unlock()
	for _, task := range tasks {
		task.cancel()
	}
	c.writer.close()
	c.wg.Wait()
	return nil
}

func batchBytes(content string, mode BatchSendMode) ([]byte, error) {
	switch mode {
	case BatchSendRaw:
		return []byte(content), nil
	case BatchSendAppendCR:
		return append([]byte(content), '\r'), nil
	case BatchSendEachLine:
		content = strings.ReplaceAll(content, "\r\n", "\n")
		content = strings.ReplaceAll(content, "\r", "\n")
		lines := strings.Split(content, "\n")
		if len(lines) > 1 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		return []byte(strings.Join(lines, "\r") + "\r"), nil
	default:
		return nil, ErrInvalidControlRequest
	}
}

func validateMembers(ids []string, eligible map[string]struct{}) error {
	for _, id := range ids {
		if _, ok := eligible[id]; !ok {
			return ErrSessionNotEligible
		}
	}
	return nil
}

func uniqueIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func removeID(values *[]string, value string) bool {
	if values == nil {
		return false
	}
	for index, item := range *values {
		if item == value {
			*values = append((*values)[:index], (*values)[index+1:]...)
			return true
		}
	}
	return false
}

func intersects(left, right []string) bool {
	for _, item := range left {
		if contains(right, item) {
			return true
		}
	}
	return false
}

func cloneBatchState(state BatchTaskState) BatchTaskState {
	state.SessionIDs = append([]string(nil), state.SessionIDs...)
	state.Targets = append([]BatchTargetState(nil), state.Targets...)
	return state
}
