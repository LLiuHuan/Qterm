package remoteedit

import "time"

const (
	watchDebounce = 800 * time.Millisecond

	watchStability = 200 * time.Millisecond
)

type watchAction int

const (
	watchActionNone watchAction = iota

	watchActionWait

	watchActionUpload
)

type watchMachine struct {
	targetBase string

	lastUploadedHash func() string

	debounce  time.Duration
	stability time.Duration

	suppressed bool

	suppressedDirty bool
	pending         bool
	sampled         bool
	firstHash       string
	deadline        time.Time
}

func newWatchMachine(targetBase string, lastUploadedHash func() string) *watchMachine {
	return &watchMachine{
		targetBase:       targetBase,
		lastUploadedHash: lastUploadedHash,
		debounce:         watchDebounce,
		stability:        watchStability,
	}
}

func (m *watchMachine) Observe(base string, at time.Time) (time.Duration, bool) {
	if m == nil || base != m.targetBase {
		return 0, false
	}
	if m.suppressed {

		m.suppressedDirty = true
		return 0, false
	}
	m.pending = true
	m.sampled = false
	m.firstHash = ""
	m.deadline = at.Add(m.debounce)
	return m.debounce, true
}

func (m *watchMachine) Sample(at time.Time, hash string) (watchAction, time.Duration) {
	if m == nil || !m.pending {
		return watchActionNone, 0
	}
	if m.suppressed {
		m.pending = false
		return watchActionNone, 0
	}
	if !m.sampled {
		m.sampled = true
		m.firstHash = hash
		m.deadline = at.Add(m.stability)
		return watchActionWait, m.stability
	}
	if hash != m.firstHash {

		m.firstHash = hash
		m.deadline = at.Add(m.stability)
		return watchActionWait, m.stability
	}
	m.pending = false
	if last := m.currentUploadedHash(); last != "" && last == hash {

		return watchActionNone, 0
	}
	return watchActionUpload, 0
}

func (m *watchMachine) Suppress() {
	if m == nil {
		return
	}
	m.suppressed = true
	m.pending = false
}

func (m *watchMachine) Resume() bool {
	if m == nil {
		return false
	}
	dirty := m.suppressedDirty
	m.suppressedDirty = false
	m.suppressed = false
	return dirty
}

func (m *watchMachine) Deadline() time.Time {
	if m == nil || !m.pending {
		return time.Time{}
	}
	return m.deadline
}

func (m *watchMachine) currentUploadedHash() string {
	if m == nil || m.lastUploadedHash == nil {
		return ""
	}
	return m.lastUploadedHash()
}
