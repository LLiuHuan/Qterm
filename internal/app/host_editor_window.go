package app

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const (
	hostEditorWindowWidth  = 760
	hostEditorWindowHeight = 840

	hostEditorTargetEvent = "qterm:host-editor-target"
)

type hostEditorTargetPayload struct {
	HostID string `json:"hostID"`
}

func (s *Service) OpenHostEditorWindow(ctx context.Context, hostID string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.openWindow == nil {
		return errors.New("原生窗口服务尚未就绪")
	}
	hostID = strings.TrimSpace(hostID)
	if window, ok := s.liveHostEditorWindow(); ok {

		s.hostEditorMu.Lock()
		s.hostEditorTargetID = hostID
		shown := s.hostEditorShown.Load()
		s.hostEditorMu.Unlock()
		if !shown {
			return nil
		}
		focusAuxiliaryWindow(window)
		s.emitMaskState(maskWindowKindHostEditor, "", "")

		if app := application.Get(); app != nil {
			app.Event.Emit(hostEditorTargetEvent, hostEditorTargetPayload{HostID: hostID})
		}
		return nil
	}
	s.hostEditorMu.Lock()
	s.hostEditorTargetID = hostID
	s.hostEditorMu.Unlock()
	params := url.Values{}
	params.Set("hostEditorWindow", "1")
	if hostID != "" {
		params.Set("hostID", hostID)
	}

	s.openWindow(ctx, "/?"+params.Encode(), nil)
	return nil
}

func (s *Service) CloseHostEditorWindow() error {

	application.InvokeSync(func() {
		window, ok := s.liveHostEditorWindow()
		if !ok {
			return
		}

		s.hostEditorMu.Lock()
		if s.hostEditorID == window.ID() {
			s.hostEditor = nil
			s.hostEditorID = 0
			s.hostEditorShown.Store(false)
		}
		s.hostEditorMu.Unlock()
		s.emitMaskState("", "", "")
		window.Close()
	})
	return nil
}

func (s *Service) liveHostEditorWindow() (application.Window, bool) {
	s.hostEditorMu.Lock()
	id := s.hostEditorID
	s.hostEditorMu.Unlock()
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

func (s *Service) ShowHostEditorWindow(ctx context.Context, hostID string) (string, error) {
	if ctx == nil {
		return "", errors.New("missing window context")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	caller, ok := ctx.Value(application.WindowKey).(application.Window)
	if s == nil || !ok || caller == nil {
		return "", errors.New("host editor window is unavailable")
	}
	return application.InvokeSyncWithResultAndError(func() (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		window, alive := s.liveHostEditorWindow()
		if !alive || caller.ID() != window.ID() {
			return "", errors.New("host editor window is unavailable")
		}
		s.hostEditorMu.Lock()
		target := s.hostEditorTargetID
		s.hostEditorMu.Unlock()
		if target != hostID {
			return target, nil
		}

		window.Show()
		s.hostEditorMu.Lock()
		s.hostEditorShown.Store(true)

		target = s.hostEditorTargetID
		s.hostEditorMu.Unlock()
		s.emitMaskState(maskWindowKindHostEditor, "", "")
		return target, nil
	})
}

func (s *Service) registerHostEditorWindow(window application.Window) {
	if s == nil || window == nil {
		return
	}
	s.hostEditorMu.Lock()
	s.hostEditor = window
	s.hostEditorID = window.ID()
	s.hostEditorMu.Unlock()

	window.RegisterHook(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		s.hostEditorMu.Lock()
		if s.hostEditorID == window.ID() {
			s.hostEditor = nil
			s.hostEditorID = 0
			s.hostEditorShown.Store(false)
		}
		s.hostEditorMu.Unlock()
		s.emitMaskState("", "", "")
	})
}

func centeredHostEditorWindowPosition(source application.Window, width, height int) (int, int, bool) {
	if source == nil {
		return 0, 0, false
	}
	x, y := source.Position()
	sourceWidth, sourceHeight := source.Size()
	x += (sourceWidth - width) / 2
	y += (sourceHeight - height) / 2
	screen, err := source.GetScreen()
	if err != nil || screen == nil {
		return x, y, true
	}
	area := screen.WorkArea
	x = min(max(x, area.X), area.X+max(area.Width-width, 0))
	y = min(max(y, area.Y), area.Y+max(area.Height-height, 0))
	return x, y, true
}
