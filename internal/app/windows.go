package app

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"changeme/internal/errno"
)

const sessionWindowClaimTimeout = 8 * time.Second
const cancelledSessionWindowRetention = 10 * time.Second

func (s *Service) OpenNewWindow(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.openWindow == nil {
		return errors.New("原生窗口服务尚未就绪")
	}
	s.openWindow(ctx, "/?newWindow=1", nil)
	return nil
}

const settingsWindowPrewarmDelay = 2 * time.Second

func (s *Service) OpenSettingsWindow(ctx context.Context, kind, title, category, search string, resizable bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.openWindow == nil {
		return errors.New("原生窗口服务尚未就绪")
	}
	category = strings.TrimSpace(category)
	search = strings.TrimSpace(search)

	if !s.settingsWindowEnabled.Load() {
		return errors.New("settings window is disabled")
	}

	s.settingsWindowMu.Lock()
	if s.settingsWindowID != 0 {
		if app := application.Get(); app != nil {
			if existing, ok := app.Window.GetByID(s.settingsWindowID); ok && existing != nil {
				s.settingsWindowMu.Unlock()

				applyDialogWindowSize(existing, kind, resizable)

				focusAuxiliaryWindow(existing)
				s.notifySettingsWindowOpened(kind, category, search)
				return nil
			}
		}

		s.settingsWindow = nil
		s.settingsWindowID = 0
	}
	s.settingsWindowMu.Unlock()

	params := url.Values{}
	params.Set("settingsWindow", "1")
	params.Set("kind", strings.TrimSpace(kind))
	if resizable {
		params.Set("resizable", "1")
	}
	if title = strings.TrimSpace(title); title != "" {
		params.Set("openTitle", title)
	}
	if category = strings.TrimSpace(category); category != "" {
		params.Set("settingsCategory", category)
	}
	if search != "" {
		params.Set("settingsSearch", search)
	}
	s.openWindow(ctx, "/?"+params.Encode(), nil)
	return nil
}

func (s *Service) prewarmSettingsWindow(ctx context.Context) error {
	if s == nil || s.openWindow == nil {
		return nil
	}

	if !s.settingsWindowEnabled.Load() {
		return nil
	}
	s.settingsWindowMu.Lock()
	alive := false
	if s.settingsWindowID != 0 {
		if app := application.Get(); app != nil {
			if existing, ok := app.Window.GetByID(s.settingsWindowID); ok && existing != nil {
				alive = true
			}
		}
	}
	s.settingsWindowMu.Unlock()
	if alive {
		return nil
	}
	params := url.Values{}
	params.Set("settingsWindow", "1")

	params.Set("prewarm", "1")
	s.openWindow(ctx, "/?"+params.Encode(), nil)
	return nil
}

const (
	maskWindowKindSettings   = "settings"
	maskWindowKindHostEditor = "hostEditor"
)

func (s *Service) emitMaskState(kind, category, search string) {
	if s == nil {
		return
	}
	open := s.settingsWindowShown.Load() || s.hostEditorShown.Load()
	if !open {
		kind = ""
		category = ""
		search = ""
	} else if kind == "" {
		if s.hostEditorShown.Load() {
			kind = maskWindowKindHostEditor
		} else {
			kind = maskWindowKindSettings
		}
	}
	if app := application.Get(); app != nil {
		app.Event.Emit(settingsWindowStateEvent, settingsWindowStatePayload{Open: open, Kind: kind, Category: category, Search: search})
	}
}

func (s *Service) markSettingsWindowOpen(open bool, kind, category, search string) {
	if s == nil {
		return
	}
	s.settingsWindowShown.Store(open)
	s.emitMaskState(kind, category, search)
}

func (s *Service) notifySettingsWindowOpened(kind, category, search string) {
	s.markSettingsWindowOpen(true, kind, category, search)
}

func (s *Service) FocusAuxiliaryWindow() error {
	if s == nil {
		return nil
	}
	if s.hostEditorShown.Load() {
		if window, ok := s.liveHostEditorWindow(); ok {
			focusAuxiliaryWindow(window)
			s.emitMaskState(maskWindowKindHostEditor, "", "")
			return nil
		}
	}
	if s.settingsWindowEnabled.Load() && s.settingsWindowShown.Load() {
		if window, ok := s.liveSettingsWindow(); ok {
			focusAuxiliaryWindow(window)
			s.emitMaskState(maskWindowKindSettings, "", "")
			return nil
		}
	}

	if s.hostEditorShown.Load() {
		if _, ok := s.liveHostEditorWindow(); !ok {
			s.hostEditorShown.Store(false)
		}
	}
	if s.settingsWindowShown.Load() {
		if _, ok := s.liveSettingsWindow(); !ok {
			s.settingsWindowShown.Store(false)
		}
	}
	s.emitMaskState("", "", "")
	return nil
}

func (s *Service) liveSettingsWindow() (application.Window, bool) {
	s.settingsWindowMu.Lock()
	id := s.settingsWindowID
	s.settingsWindowMu.Unlock()
	if id == 0 {
		return nil, false
	}
	app := application.Get()
	if app == nil {
		return nil, false
	}
	window, ok := app.Window.GetByID(id)
	if !ok || window == nil {
		return nil, false
	}
	return window, true
}

func (s *Service) SettingsWindowVisible() (bool, error) {
	if s == nil {
		return false, nil
	}
	s.settingsWindowMu.Lock()
	id := s.settingsWindowID
	s.settingsWindowMu.Unlock()
	if id == 0 {
		return false, nil
	}
	app := application.Get()
	if app == nil {
		return false, nil
	}
	window, ok := app.Window.GetByID(id)
	if !ok || window == nil {
		return false, nil
	}
	return window.IsVisible(), nil
}

func dialogWindowSize(kind string) (width, height int) {
	switch kind {
	case "import":
		return 900, 700
	case "export":
		return 520, 420
	default:
		return 1040, 720
	}
}

func applyDialogWindowSize(window application.Window, kind string, resizable bool) {
	width, height := dialogWindowSize(kind)
	window.SetResizable(resizable)
	if resizable {
		window.SetMinSize(860, 560)
		window.SetMaxSize(0, 0)
	} else {

		window.SetMinSize(width, height)
		window.SetMaxSize(width, height)
	}
	window.SetSize(width, height)
}

func (s *Service) HideSettingsWindow() error {
	if s == nil {
		return nil
	}
	s.settingsWindowMu.Lock()
	id := s.settingsWindowID
	s.settingsWindowMu.Unlock()
	if id == 0 {
		return nil
	}
	app := application.Get()
	if app == nil {
		return nil
	}
	window, ok := app.Window.GetByID(id)
	if !ok || window == nil {
		return nil
	}
	window.Hide()
	s.notifySettingsWindowHidden()
	return nil
}

func (s *Service) notifySettingsWindowHidden() {
	s.markSettingsWindowOpen(false, "", "", "")
}

func (s *Service) hideFocusedSettingsWindow() bool {
	if s == nil {
		return false
	}
	s.settingsWindowMu.Lock()
	window := s.settingsWindow
	id := s.settingsWindowID
	s.settingsWindowMu.Unlock()
	if window == nil || id == 0 {
		return false
	}

	app := application.Get()
	if app == nil {
		return false
	}
	existing, ok := app.Window.GetByID(id)
	if !ok || existing == nil {
		return false
	}
	if !existing.IsFocused() {
		return false
	}
	existing.Hide()
	s.notifySettingsWindowHidden()
	return true
}

func (s *Service) SetSettingsWindowEnabled(enabled bool) error {
	if s == nil {
		return nil
	}
	previous := s.settingsWindowEnabled.Swap(enabled)

	if enabled && !previous {
		go func() {

			_ = s.prewarmSettingsWindow(context.Background())
		}()
	}
	return nil
}

func focusAuxiliaryWindow(window application.Window) {
	if window.IsMinimised() {
		window.UnMinimise()
	}
	if !window.IsVisible() {
		window.Show()
	}
	window.Focus()
	go func() {
		for _, delay := range []time.Duration{150 * time.Millisecond, 400 * time.Millisecond} {
			time.Sleep(delay)
			window.Focus()
		}
	}()
}

func (s *Service) refocusAuxiliaryWindow() {
	if s == nil {
		return
	}
	if s.tray != nil {
		if s.tray.locked.Load() {
			return
		}
		s.tray.quitMu.Lock()
		pending := s.tray.quitPending
		s.tray.quitMu.Unlock()
		if pending {
			return
		}
	}

	window, ok := s.liveHostEditorWindow()
	stillOpen := func() bool { return s.hostEditorShown.Load() }
	if !ok || !stillOpen() {
		if !s.settingsWindowEnabled.Load() || !s.settingsWindowShown.Load() {
			return
		}
		window, ok = s.liveSettingsWindow()
		stillOpen = func() bool { return s.settingsWindowShown.Load() }
		if !ok {
			return
		}
	}

	go func() {
		for _, delay := range []time.Duration{80 * time.Millisecond, 260 * time.Millisecond, 520 * time.Millisecond} {
			time.Sleep(delay)
			if !stillOpen() {
				return
			}
			window.Focus()
			if window.IsFocused() {
				return
			}
		}
	}()
}

type sessionWindowTransfer struct {
	ready        chan struct{}
	closed       chan struct{}
	state        sessionWindowTransferState
	windowClosed bool
}

type sessionWindowTransferState uint8

const (
	sessionWindowTransferPending sessionWindowTransferState = iota
	sessionWindowTransferClaimed
	sessionWindowTransferClosed
	sessionWindowTransferCancelled
)

func (s *Service) OpenSftpWindow(ctx context.Context, sessionID, title string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errno.New(errno.CodeInvalidArg, "sftp.windowSessionRequired", "")
	}
	if s == nil || s.openWindow == nil {
		return errors.New("原生窗口服务尚未就绪")
	}
	target := sftpWindowURL(sessionID, title)
	s.openWindow(ctx, target, nil)
	return nil
}

func (s *Service) OpenSessionWindow(ctx context.Context, kind, hostID, sessionID, title string) error {
	target, err := sessionWindowURL(kind, hostID, sessionID, title)
	if err != nil {
		return err
	}
	if s.openWindow == nil {
		return errors.New("原生窗口服务尚未就绪")
	}
	if sessionID == "" {
		return errno.New(errno.CodeInvalidArg, "session.windowSessionRequired", "")
	}

	transfer, err := s.beginSessionWindowTransfer(sessionID)
	if err != nil {
		return err
	}
	closeWindow := s.openWindow(ctx, target, func() { s.sessionWindowClosed(sessionID, transfer) })
	timer := time.NewTimer(sessionWindowClaimTimeout)
	defer timer.Stop()
	select {
	case <-transfer.ready:
		return s.sessionWindowTransferResult(transfer)
	case <-transfer.closed:
		return errno.New(errno.CodeSessionClosed, "session.windowClosed", "")
	case <-ctx.Done():
		if result := s.abortSessionWindowTransfer(sessionID, transfer); result != sessionWindowTransferCancelled {
			return s.sessionWindowTransferResult(transfer)
		}
		if closeWindow != nil {
			closeWindow()
		}
		return ctx.Err()
	case <-timer.C:
		if result := s.abortSessionWindowTransfer(sessionID, transfer); result != sessionWindowTransferCancelled {
			return s.sessionWindowTransferResult(transfer)
		}
		if closeWindow != nil {
			closeWindow()
		}
		return errno.New(errno.CodeTimeout, "error.timeout", "")
	}
}

func (s *Service) beginSessionWindowTransfer(sessionID string) (*sessionWindowTransfer, error) {
	s.windowMu.Lock()
	defer s.windowMu.Unlock()
	if s.windowTransfers == nil {
		s.windowTransfers = make(map[string]*sessionWindowTransfer)
	}
	if _, exists := s.windowTransfers[sessionID]; exists {
		return nil, errno.New(errno.CodeInvalidArg, "session.windowTransferInProgress", "")
	}
	transfer := &sessionWindowTransfer{ready: make(chan struct{}), closed: make(chan struct{})}
	s.windowTransfers[sessionID] = transfer
	return transfer, nil
}

func (s *Service) commitSessionWindowTerminal(sessionID string, activate func() error) error {
	s.windowMu.Lock()
	defer s.windowMu.Unlock()
	transfer := s.windowTransfers[sessionID]
	if transfer == nil {
		return activate()
	}
	if transfer.state != sessionWindowTransferPending {
		return errno.New(errno.CodeCancelled, "error.cancelled", "")
	}
	if err := activate(); err != nil {
		return err
	}
	transfer.state = sessionWindowTransferClaimed
	close(transfer.ready)
	return nil
}

func (s *Service) releaseClaimedSessionWindowTransfer(sessionID string) bool {
	s.windowMu.Lock()
	defer s.windowMu.Unlock()
	transfer := s.windowTransfers[sessionID]
	if transfer == nil || transfer.state != sessionWindowTransferClaimed {
		return false
	}
	delete(s.windowTransfers, sessionID)
	return true
}

func (s *Service) abortSessionWindowTransfer(
	sessionID string,
	transfer *sessionWindowTransfer,
) sessionWindowTransferState {
	s.windowMu.Lock()
	defer s.windowMu.Unlock()
	if transfer.state == sessionWindowTransferPending && s.windowTransfers[sessionID] == transfer {
		transfer.state = sessionWindowTransferCancelled
	}
	return transfer.state
}

func (s *Service) sessionWindowTransferResult(transfer *sessionWindowTransfer) error {
	s.windowMu.Lock()
	state := transfer.state
	s.windowMu.Unlock()
	if state == sessionWindowTransferClaimed {
		return nil
	}
	if state == sessionWindowTransferClosed {
		return errno.New(errno.CodeSessionClosed, "session.windowClosed", "")
	}
	return errno.New(errno.CodeTimeout, "error.timeout", "")
}

func (s *Service) sessionWindowClosed(sessionID string, transfer *sessionWindowTransfer) {
	s.windowMu.Lock()
	if s.windowTransfers[sessionID] != transfer || transfer.windowClosed {
		s.windowMu.Unlock()
		return
	}
	transfer.windowClosed = true
	claimed := transfer.state == sessionWindowTransferClaimed
	if claimed {
		delete(s.windowTransfers, sessionID)
		transfer.state = sessionWindowTransferClosed
	} else {

		if transfer.state == sessionWindowTransferPending {
			transfer.state = sessionWindowTransferClosed
		}
		go s.expireInactiveSessionWindow(sessionID, transfer)
	}
	close(transfer.closed)
	s.windowMu.Unlock()
	if !claimed || s.sessions == nil {
		return
	}
	closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = s.sessions.CloseSessionFromWindow(closeCtx, sessionID)
}

func (s *Service) expireInactiveSessionWindow(sessionID string, transfer *sessionWindowTransfer) {
	timer := time.NewTimer(cancelledSessionWindowRetention)
	defer timer.Stop()
	<-timer.C
	s.windowMu.Lock()
	defer s.windowMu.Unlock()
	if s.windowTransfers[sessionID] == transfer &&
		(transfer.state == sessionWindowTransferCancelled || transfer.state == sessionWindowTransferClosed) {
		delete(s.windowTransfers, sessionID)
	}
}

func sessionWindowURL(kind, hostID, sessionID, title string) (string, error) {
	params := url.Values{}

	if sessionID != "" {
		params.Set("openSession", sessionID)
		if title != "" {
			params.Set("openTitle", title)
		}
		return "/?" + params.Encode(), nil
	}
	switch kind {
	case "local":
		params.Set("openLocal", "1")
	case "ssh", "telnet":

		if hostID == "" {
			return "", errors.New("新窗口缺少主机 ID")
		}
		params.Set("openHost", hostID)
	default:
		return "", errors.New("不支持的会话类型")
	}
	return "/?" + params.Encode(), nil
}

func sftpWindowURL(sessionID, title string) string {
	params := url.Values{}
	params.Set("sftpSession", strings.TrimSpace(sessionID))
	if title = strings.TrimSpace(title); title != "" {
		params.Set("sftpTitle", title)
	}
	return "/?" + params.Encode()
}
