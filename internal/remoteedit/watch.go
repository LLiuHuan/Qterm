package remoteedit

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"changeme/internal/model"
	"github.com/fsnotify/fsnotify"
)

const watchEventBuffer = 256

type watchEvent struct {
	base string
	at   time.Time
}

type watcher interface {
	Events() <-chan watchEvent
	Errors() <-chan error
	Close() error
}

type newWatcherFunc func(directory string) (watcher, error)

type fsnotifyWatcher struct {
	handle    *fsnotify.Watcher
	events    chan watchEvent
	errs      chan error
	done      chan struct{}
	closeOnce sync.Once
}

func newFsnotifyWatcher(directory string) (watcher, error) {
	handle, err := fsnotify.NewBufferedWatcher(watchEventBuffer)
	if err != nil {
		return nil, err
	}
	if err := handle.Add(directory); err != nil {
		_ = handle.Close()
		return nil, err
	}
	w := &fsnotifyWatcher{
		handle: handle,
		events: make(chan watchEvent, watchEventBuffer),
		errs:   make(chan error, 8),
		done:   make(chan struct{}),
	}
	go w.pump()
	return w, nil
}

func (w *fsnotifyWatcher) pump() {
	defer close(w.events)
	for {
		select {
		case <-w.done:
			return
		case event, ok := <-w.handle.Events:
			if !ok {
				return
			}
			forward := watchEvent{base: filepath.Base(event.Name), at: time.Now()}
			select {
			case w.events <- forward:
			case <-w.done:
				return
			}
		case err, ok := <-w.handle.Errors:
			if !ok {
				return
			}
			select {
			case w.errs <- err:
			default:
			}
		}
	}
}

func (w *fsnotifyWatcher) Events() <-chan watchEvent { return w.events }

func (w *fsnotifyWatcher) Errors() <-chan error { return w.errs }

func (w *fsnotifyWatcher) Close() error {
	if w == nil {
		return nil
	}
	var err error
	w.closeOnce.Do(func() {
		close(w.done)
		err = w.handle.Close()
	})
	return err
}

func (m *Manager) startWatcher(edit *localEdit) {
	if m == nil || edit == nil {
		return
	}
	directory := filepath.Dir(edit.localFilePath())
	watch, err := m.newWatcher(directory)
	if err != nil {
		m.setState(edit, model.LocalEditStateError, messageKeyWatchFailed)
		return
	}
	runCtx := m.runContext()
	if runCtx.Err() != nil {

		_ = watch.Close()
		return
	}
	ctx, cancel := context.WithCancel(runCtx)
	edit.attachWatcher(cancel)
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer cancel()
		defer watch.Close()
		m.watchLoop(ctx, edit, watch)
	}()
}

func (m *Manager) watchLoop(ctx context.Context, edit *localEdit, watch watcher) {
	machine := m.buildWatchMachine(edit)
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	defer timer.Stop()
	stopTimer := func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}
	armTimer := func(wait time.Duration) {
		stopTimer()
		if wait > 0 {
			timer.Reset(wait)
		}
	}
	trigger := func(at time.Time, hash string) {
		action, wait := machine.Sample(at, hash)
		switch action {
		case watchActionWait:
			armTimer(wait)
		case watchActionUpload:

			machine.Suppress()
			_, _ = m.sync(ctx, edit.editID(), syncOptions{})
			if machine.Resume() {

				armTimer(machine.debounce)
			}
		default:
			stopTimer()
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watch.Events():
			if !ok {
				return
			}
			wait, pending := machine.Observe(event.base, event.at)
			if !pending {
				continue
			}
			armTimer(wait)
		case <-watch.Errors():

		case <-timer.C:
			hash, err := hashFile(edit.localFilePath())
			if err != nil {

				continue
			}

			edit.noteLocalSample(hash)
			trigger(m.now(), hash)
		}
	}
}

func (m *Manager) buildWatchMachine(edit *localEdit) *watchMachine {
	machine := newWatchMachine(filepath.Base(edit.localFilePath()), edit.uploadedHashValue)
	m.mu.RLock()
	debounce, stability := m.debounce, m.stability
	m.mu.RUnlock()
	if debounce > 0 {
		machine.debounce = debounce
	}
	if stability > 0 {
		machine.stability = stability
	}
	return machine
}
