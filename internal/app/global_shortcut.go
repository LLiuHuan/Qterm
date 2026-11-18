package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	globalShortcutSettingField = "globalShortcut"
	singleInstanceSettingField = "singleInstance"
)

const (
	globalShortcutChangedEvent = "qterm:global-shortcut-changed"
	globalShortcutFailedEvent  = "qterm:global-shortcut-failed"
)

type globalShortcutState struct {
	mu          sync.Mutex
	accelerator string
}

func (s *Service) applyGlobalShortcut(accelerator string) error {
	if s == nil {
		return nil
	}
	app := application.Get()
	if app == nil || app.GlobalShortcut == nil {

		return nil
	}
	accelerator = strings.TrimSpace(accelerator)

	translated, err := toWailsAccelerator(accelerator)
	if err != nil {

		s.emitGlobalShortcutFailure(accelerator, err)
		return err
	}
	accelerator = translated

	s.globalShortcut.mu.Lock()
	previous := s.globalShortcut.accelerator
	s.globalShortcut.mu.Unlock()

	if previous == accelerator {
		return nil
	}
	if previous != "" {
		if err := app.GlobalShortcut.Unregister(previous); err != nil {

			slog.Warn("global shortcut unregister failed", "accelerator", previous, "error", err)
		}
	}
	if accelerator == "" {
		s.setActiveGlobalShortcut("")
		s.emitGlobalShortcutChanged("")
		return nil
	}

	handler := func() {

		slog.Warn("global hotkey triggered", "event", "global_hotkey.triggered", "accelerator", accelerator)

		s.focusWorkspaceWindow()
	}
	registerErr := app.GlobalShortcut.Register(accelerator, handler)
	if registerErr != nil {

		restored := ""
		if previous != "" {
			if rebound := app.GlobalShortcut.Register(previous, handler); rebound != nil {
				slog.Warn("global shortcut rollback failed", "accelerator", previous, "error", rebound)
			} else {
				restored = previous
			}
		}
		if restored != previous {

			s.setActiveGlobalShortcut(restored)
			s.emitGlobalShortcutChanged(restored)
		}
		s.emitGlobalShortcutFailure(accelerator, registerErr)
		return registerErr
	}
	s.setActiveGlobalShortcut(accelerator)
	s.emitGlobalShortcutChanged(accelerator)
	return nil
}

func (s *Service) focusWorkspaceWindow() {
	if s == nil {
		return
	}
	var window application.Window
	if s.tray != nil {
		window = s.tray.window
	}
	if window == nil {
		app := application.Get()
		if app == nil || app.Window == nil {
			return
		}
		windows := app.Window.GetAll()
		if len(windows) == 0 {
			return
		}
		window = windows[0]
	}

	if window.IsMinimised() {
		window.UnMinimise()
	}
	window.Show()
	window.Focus()
}

func (s *Service) setActiveGlobalShortcut(accelerator string) {
	s.globalShortcut.mu.Lock()
	s.globalShortcut.accelerator = accelerator
	s.globalShortcut.mu.Unlock()
}

func (s *Service) emitGlobalShortcutChanged(accelerator string) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(globalShortcutChangedEvent, map[string]string{"accelerator": accelerator})
}

func (s *Service) emitGlobalShortcutFailure(accelerator string, cause error) {
	app := application.Get()
	if app == nil {
		return
	}

	reason := "occupied"
	if cause != nil && strings.Contains(cause.Error(), "invalid global shortcut") {
		reason = "invalid"
	}
	app.Event.Emit(globalShortcutFailedEvent, map[string]string{
		"accelerator": accelerator,
		"reason":      reason,
	})
}

func (s *Service) GlobalShortcut() string {
	if s == nil {
		return ""
	}
	s.globalShortcut.mu.Lock()
	defer s.globalShortcut.mu.Unlock()
	return s.globalShortcut.accelerator
}

func (s *Service) SetGlobalShortcut(_ context.Context, accelerator string) error {
	if s == nil {
		return publicError(errors.New("应用服务未配置"))
	}
	if err := s.applyGlobalShortcut(accelerator); err != nil {
		return publicError(err)
	}
	return nil
}

func (s *Service) loadSettingsField(ctx context.Context, field string) string {
	if s == nil || s.preferences == nil {
		return ""
	}
	raw, found, err := s.preferences.Get(ctx, settingsPreferenceKey)
	if err != nil || !found || raw == "" {
		return ""
	}
	var settings map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &settings) != nil {
		return ""
	}
	value, ok := settings[field]
	if !ok {
		return ""
	}
	var text string
	if json.Unmarshal(value, &text) != nil {
		return ""
	}
	return strings.TrimSpace(text)
}

func (s *Service) registerGlobalShortcutFromPreferences(ctx context.Context) {
	if s == nil {
		return
	}
	accelerator := s.loadSettingsField(ctx, globalShortcutSettingField)
	if accelerator == "" {
		return
	}
	if err := s.applyGlobalShortcut(accelerator); err != nil {
		slog.Warn("global shortcut startup registration failed", "accelerator", accelerator, "error", err)
	}
}

func (s *Service) applySettingsFromPreference(raw string) {
	if s == nil || raw == "" {
		return
	}
	var settings struct {
		GlobalShortcut string `json:"globalShortcut"`
		SingleInstance *bool  `json:"singleInstance"`
	}
	if json.Unmarshal([]byte(raw), &settings) != nil {

		return
	}
	if err := s.applyGlobalShortcut(settings.GlobalShortcut); err != nil {
		slog.Warn("apply global shortcut from settings failed", "error", err)
	}
	if settings.SingleInstance != nil {
		applySingleInstancePreference(*settings.SingleInstance)
	}
}
