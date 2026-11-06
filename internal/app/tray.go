package app

import (
	_ "embed"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed tray_icon.png
var trayIconPNG []byte

const (
	quitRequestEvent = "qterm:quit-request"

	quitConfirmTimeout = 2 * time.Second

	quitConfirmClaimTimeout = 10 * time.Minute

	trayActionEvent    = "qterm:tray-action"
	trayActionNewLocal = "newLocalTerminal"
	trayActionSettings = "openSettings"
)

type quitPromptLabels struct {
	Title   string
	Message string
	Confirm string
	Cancel  string
}

func defaultQuitPromptLabels() quitPromptLabels {
	return quitPromptLabels{
		Title:   "Qterm",
		Message: "Quit Qterm? Running sessions and transfers will be disconnected.",
		Confirm: "Quit",
		Cancel:  "Cancel",
	}
}

type quitRequest struct {
	RequestID int64 `json:"requestID"`

	ActiveSessions int `json:"activeSessions"`
	ActiveForwards int `json:"activeForwards"`
}

type trayState struct {
	closeToTray atomic.Bool

	quitting atomic.Bool

	quitConfirm atomic.Bool

	locked atomic.Bool
	window application.Window
	app    *application.App

	quitMu      sync.Mutex
	quitSeq     int64
	quitPending bool
	quitWaiters map[int64]chan bool

	quitClaims map[int64]chan struct{}

	quitLabels quitPromptLabels

	trayMenuAllowed atomic.Bool
	trayMenuMu      sync.Mutex
	showItem        *application.MenuItem
	newLocalItem    *application.MenuItem
	settingsItem    *application.MenuItem
	quitItem        *application.MenuItem

	emitQuitRequest func(quitRequest)
	showNativeQuit  func()
	activeForwards  func() int
	activeSessions  func() int

	quitTimeout time.Duration

	quitClaimTimeout time.Duration
}

type TrayMenuState struct {
	Ready         bool   `json:"ready"`
	Locked        bool   `json:"locked"`
	NewLocalLabel string `json:"newLocalLabel"`
	ShowLabel     string `json:"showLabel"`
	SettingsLabel string `json:"settingsLabel"`
	QuitLabel     string `json:"quitLabel"`
}

func newTrayState(window application.Window) *trayState {
	state := &trayState{
		window:      window,
		quitWaiters: make(map[int64]chan bool),
		quitClaims:  make(map[int64]chan struct{}),

		emitQuitRequest:  func(quitRequest) {},
		showNativeQuit:   func() {},
		activeForwards:   func() int { return 0 },
		activeSessions:   func() int { return 0 },
		quitTimeout:      quitConfirmTimeout,
		quitClaimTimeout: quitConfirmClaimTimeout,
		quitLabels:       defaultQuitPromptLabels(),
	}

	state.closeToTray.Store(true)
	state.quitConfirm.Store(true)
	return state
}

func (t *trayState) trayMenuEnabled() bool {
	return t.trayMenuAllowed.Load()
}

func (t *trayState) applyTrayMenuState(next TrayMenuState) {
	t.trayMenuMu.Lock()
	defer t.trayMenuMu.Unlock()

	t.locked.Store(next.Locked)
	t.trayMenuAllowed.Store(next.Ready && !next.Locked)
	if t.showItem == nil && t.newLocalItem == nil && t.settingsItem == nil && t.quitItem == nil {
		return
	}
	application.InvokeSync(func() {
		enabled := t.trayMenuEnabled()
		if t.showItem != nil && strings.TrimSpace(next.ShowLabel) != "" {
			t.showItem.SetLabel(next.ShowLabel)
		}
		if t.newLocalItem != nil {
			if strings.TrimSpace(next.NewLocalLabel) != "" {
				t.newLocalItem.SetLabel(next.NewLocalLabel)
			}
			t.newLocalItem.SetEnabled(enabled)
		}
		if t.settingsItem != nil {
			if strings.TrimSpace(next.SettingsLabel) != "" {
				t.settingsItem.SetLabel(next.SettingsLabel)
			}
			t.settingsItem.SetEnabled(enabled)
		}
		if t.quitItem != nil && strings.TrimSpace(next.QuitLabel) != "" {
			t.quitItem.SetLabel(next.QuitLabel)
		}
	})
}

func installTray(app *application.App, state *trayState, service *Service, icon []byte) *application.SystemTray {
	state.app = app
	state.emitQuitRequest = func(payload quitRequest) {
		app.Event.Emit(quitRequestEvent, payload)
	}
	state.showNativeQuit = func() { state.confirmQuitNative(app) }
	state.activeForwards = func() int {
		if service == nil || service.sessions == nil {
			return 0
		}
		return service.sessions.ActiveForwardCount()
	}
	tray := app.SystemTray.New()
	if runtime.GOOS == "darwin" {

		tray.SetTemplateIcon(trayIconPNG)
	} else {

		tray.SetIcon(icon)
	}
	tray.SetTooltip("Qterm")

	showWindow := func() {
		if state.window == nil {
			return
		}
		if state.window.IsMinimised() {
			state.window.UnMinimise()
		}
		state.window.Show()
		state.window.Focus()
	}

	menu := application.NewMenu()
	showItem := menu.Add("Show Qterm")
	state.showItem = showItem
	showItem.OnClick(func(*application.Context) { showWindow() })
	newLocalItem := menu.Add("New local terminal").SetEnabled(false)
	state.newLocalItem = newLocalItem
	newLocalItem.OnClick(func(*application.Context) {
		if !state.trayMenuEnabled() {
			return
		}
		showWindow()
		app.Event.Emit(trayActionEvent, trayActionNewLocal)
	})
	settingsItem := menu.Add("Settings").SetEnabled(false)
	state.settingsItem = settingsItem
	settingsItem.OnClick(func(*application.Context) {
		if !state.trayMenuEnabled() {
			return
		}
		showWindow()
		app.Event.Emit(trayActionEvent, trayActionSettings)
	})
	menu.AddSeparator()
	quitItem := menu.Add("Quit Qterm")
	state.quitItem = quitItem
	quitItem.OnClick(func(*application.Context) {

		state.beginQuit()
	})
	tray.SetMenu(menu)

	tray.OnClick(func() {

		if state.window == nil {
			return
		}
		if state.window.IsVisible() && !state.window.IsMinimised() {
			state.window.Hide()
			return
		}
		showWindow()
	})
	tray.OnRightClick(func() { tray.ShowMenu() })
	return tray
}

func (t *trayState) handlePrimaryWindowClose() func(*application.WindowEvent) {
	return func(event *application.WindowEvent) {
		if t.quitting.Load() {

			return
		}

		event.Cancel()
		if t.closeToTray.Load() {
			if t.window != nil {
				t.window.Hide()
			}
			return
		}
		go t.beginQuit()
	}
}

func (t *trayState) beginQuit() {
	if t == nil || t.quitting.Load() {
		return
	}
	if !t.quitConfirm.Load() {
		t.quitNow()
		return
	}
	t.requestQuitConfirm()
}

func (t *trayState) focusForQuitPrompt() {
	if t == nil || t.window == nil {

		return
	}
	if t.quitting.Load() {
		return
	}
	if t.window.IsMinimised() {
		t.window.UnMinimise()
	}
	if !t.window.IsVisible() {
		t.window.Show()
	}
	t.window.Focus()
	go func() {
		for _, delay := range []time.Duration{150 * time.Millisecond, 400 * time.Millisecond} {
			time.Sleep(delay)
			if t.quitting.Load() {
				return
			}
			t.window.Focus()
		}
	}()
}

func (t *trayState) quitNow() {
	if !t.quitting.CompareAndSwap(false, true) {
		return
	}
	if t.app != nil {
		t.app.Quit()
	}
}

func (t *trayState) requestQuitConfirm() {
	t.quitMu.Lock()
	if t.quitPending {

		t.quitMu.Unlock()
		return
	}
	t.quitPending = true
	t.quitSeq++
	requestID := t.quitSeq
	waiter := make(chan bool, 1)
	t.quitWaiters[requestID] = waiter
	claim := make(chan struct{}, 1)
	if t.quitClaims == nil {

		t.quitClaims = make(map[int64]chan struct{})
	}
	t.quitClaims[requestID] = claim
	emit := t.emitQuitRequest
	forwards, activeSessions := 0, 0
	if t.activeForwards != nil {
		forwards = t.activeForwards()
	}
	if t.activeSessions != nil {
		activeSessions = t.activeSessions()
	}
	t.quitMu.Unlock()

	defer func() {
		t.quitMu.Lock()
		t.quitPending = false
		delete(t.quitWaiters, requestID)
		delete(t.quitClaims, requestID)
		t.quitMu.Unlock()
	}()

	t.focusForQuitPrompt()

	if emit != nil {
		emit(quitRequest{RequestID: requestID, ActiveSessions: activeSessions, ActiveForwards: forwards})
	}

	select {
	case confirmed := <-waiter:
		if confirmed {
			t.quitNow()
		}
		return
	case <-claim:

		select {
		case confirmed := <-waiter:
			if confirmed {
				t.quitNow()
			}
			return
		case <-time.After(t.quitClaimTimeout):

		}
	case <-time.After(t.quitTimeout):

	}

	if t.showNativeQuit != nil {
		t.showNativeQuit()
	}
}

func (t *trayState) claimQuit(requestID int64) {
	t.quitMu.Lock()
	claim := t.quitClaims[requestID]
	t.quitMu.Unlock()
	if claim == nil {
		return
	}
	select {
	case claim <- struct{}{}:
	default:
	}
}

func (t *trayState) answerQuit(requestID int64, confirmed bool) {
	t.quitMu.Lock()
	waiter := t.quitWaiters[requestID]
	t.quitMu.Unlock()
	if waiter == nil {

		return
	}
	select {
	case waiter <- confirmed:
	default:
	}
}

func (t *trayState) allowQuit() bool {
	if t == nil || t.quitting.Load() || !t.quitConfirm.Load() {
		return true
	}
	go t.requestQuitConfirm()
	return false
}

func (t *trayState) confirmQuitNative(app *application.App) {
	t.quitMu.Lock()
	labels := t.quitLabels
	t.quitMu.Unlock()
	dialog := app.Dialog.Question().
		SetTitle(labels.Title).
		SetMessage(labels.Message)
	dialog.AddButton(labels.Confirm).OnClick(func() {
		t.quitNow()
	})
	dialog.AddButton(labels.Cancel).OnClick(func() {

		t.quitting.Store(false)
	})
	dialog.Show()
}

func (s *Service) SetQuitConfirmLabels(title, message, confirmLabel, cancelLabel string) error {
	if s == nil || s.tray == nil {
		return nil
	}
	s.tray.setQuitLabels(quitPromptLabels{
		Title:   title,
		Message: message,
		Confirm: confirmLabel,
		Cancel:  cancelLabel,
	})
	return nil
}

func (s *Service) SetTrayMenuState(state TrayMenuState) error {
	if s == nil || s.tray == nil {
		return nil
	}
	s.tray.applyTrayMenuState(state)
	return nil
}

func (t *trayState) setQuitLabels(next quitPromptLabels) {
	t.quitMu.Lock()
	defer t.quitMu.Unlock()
	merged := t.quitLabels
	if strings.TrimSpace(next.Title) != "" {
		merged.Title = next.Title
	}
	if strings.TrimSpace(next.Message) != "" {
		merged.Message = next.Message
	}
	if strings.TrimSpace(next.Confirm) != "" {
		merged.Confirm = next.Confirm
	}
	if strings.TrimSpace(next.Cancel) != "" {
		merged.Cancel = next.Cancel
	}
	t.quitLabels = merged
}

func (s *Service) ResolveQuitConfirm(requestID int64, confirmed bool) error {
	if s == nil || s.tray == nil {
		return nil
	}
	s.tray.answerQuit(requestID, confirmed)
	return nil
}

func (s *Service) ClaimQuitConfirm(requestID int64) error {
	if s == nil || s.tray == nil {
		return nil
	}
	s.tray.claimQuit(requestID)
	return nil
}

func (s *Service) allowQuit() bool {
	if s == nil || s.tray == nil {
		return true
	}

	if s.tray.quitting.Load() {
		return true
	}

	if s.hideFocusedSettingsWindow() {
		return false
	}
	return s.tray.allowQuit()
}

func (s *Service) SetQuitConfirm(enabled bool) error {
	if s == nil || s.tray == nil {
		return nil
	}
	s.tray.quitConfirm.Store(enabled)
	return nil
}

func (s *Service) QuitConfirm() bool {
	if s == nil || s.tray == nil {
		return true
	}
	return s.tray.quitConfirm.Load()
}

func (s *Service) SetCloseToTray(enabled bool) error {
	if s == nil || s.tray == nil {
		return nil
	}
	s.tray.closeToTray.Store(enabled)
	return nil
}

func (s *Service) CloseToTray() bool {
	if s == nil || s.tray == nil {
		return true
	}
	return s.tray.closeToTray.Load()
}
