package app

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"image"
	_ "image/png"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"changeme/internal/applock"
	"changeme/internal/biometric"
	"changeme/internal/credential"
	"changeme/internal/diagnostics"
	"changeme/internal/fonts"
	"changeme/internal/host"
	"changeme/internal/preview"
	"changeme/internal/proxy"
	"changeme/internal/remoteedit"
	"changeme/internal/security"
	"changeme/internal/session"
	"changeme/internal/sftp"
	"changeme/internal/terminal"
	"changeme/internal/zmodem"
)

func reportWailsError(err error) {
	if err == nil {
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, "qterm: wails error: %v\n", err)
	slog.Error("wails error", "error", err)
}

func Run(assets embed.FS) error {

	iconPath := "frontend/dist/qterm-icon.png"
	switch runtime.GOOS {
	case "darwin", "windows", "linux":
		iconPath = "frontend/dist/app-icons/" + runtime.GOOS + ".png"
	}
	icon, err := fs.ReadFile(assets, iconPath)
	if err != nil {
		return err
	}

	if _, _, err := image.Decode(bytes.NewReader(icon)); err != nil {
		return fmt.Errorf("嵌入的应用图标无法解析（%s）：%w", iconPath, err)
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	securityManager, err := security.NewManager(filepath.Join(configDir, "Qterm"))
	if err != nil {
		return err
	}

	appLockManager, err := applock.New(
		filepath.Join(configDir, "Qterm"),
		applock.WithBiometricAvailability(biometric.Available),
	)
	if err != nil {
		return err
	}
	diagnosticsManager, err := diagnostics.NewManager(filepath.Join(configDir, "Qterm"))
	if err != nil {
		return err
	}
	diagnosticsManager.Install()
	hostService := host.NewHostServiceWithSecurity(securityManager)
	credentialService := credential.NewCredentialServiceWithSecurity(securityManager)

	proxyService := proxy.NewService(hostService.WithStore)
	tokens := terminal.NewTokenStore()

	rdpTokens := terminal.NewTokenStore()

	adhocRegistry := &adHocHostRegistry{}
	hostSource := &hostServiceSource{service: hostService, proxies: proxyService, adhoc: adhocRegistry}
	sessionService := session.NewService(
		hostSource,
		&credentialServiceSource{service: credentialService},
		tokens,
	)
	sftpService := sftp.NewService()

	remoteEditManager := remoteedit.NewManager(&sessionManagerSource{service: sessionService})
	remoteEditManager.SetEditorPathSource(func() string { return sftpService.Settings().EditorPath })

	previewService := preview.NewService(&sessionManagerSource{service: sessionService})
	service := NewService(
		hostService,
		credentialService,
		proxyService,
		securityManager,
		appLockManager,
		diagnosticsManager,
		fonts.NewService(),
		sessionService,
		sftpService,
		remoteEditManager,
		previewService,
		tokens,
		rdpTokens,
		zmodem.NewManager(),
	)

	service.adhoc = adhocRegistry

	app := application.New(application.Options{
		Name:         "Qterm",
		Description:  "A cross-platform SSH workspace",
		Icon:         icon,
		MarshalError: marshalError,

		ErrorHandler: reportWailsError,

		ShouldQuit: service.allowQuit,
		Services:   []application.Service{application.NewService(service)},

		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(assets)},

		Mac: application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: false},
	})
	ConfigureRuntime(service, app)
	var windowCreationMu sync.Mutex
	primaryWindowCreated := false
	service.openWindow = func(ctx context.Context, target string, onClose func()) func() {
		windowCreationMu.Lock()
		isPrimaryWindow := !primaryWindowCreated
		primaryWindowCreated = true
		windowCreationMu.Unlock()

		options := workspaceWindowOptions(icon)
		options.URL = target
		deferShowUntilContentReady := false
		isSettingsWindow := false
		isHostEditorWindow := false
		prewarmSettings := false
		settingsCategory := ""
		dialogKind := "settings"
		dialogResizable := false
		if parsed, err := url.Parse(target); err == nil {
			query := parsed.Query()

			deferShowUntilContentReady = query.Has("openSession") || query.Get("newWindow") == "1" || query.Get("settingsWindow") == "1" || query.Get("hostEditorWindow") == "1"
			isSettingsWindow = query.Get("settingsWindow") == "1"
			isHostEditorWindow = query.Get("hostEditorWindow") == "1"

			prewarmSettings = isSettingsWindow && query.Get("prewarm") == "1"
			settingsCategory = query.Get("settingsCategory")
			if kind := query.Get("kind"); kind != "" {
				dialogKind = kind
			}
			dialogResizable = query.Get("resizable") == "1"
			if title := parsed.Query().Get("openTitle"); title != "" {
				options.Title = title
			}
		}

		if isHostEditorWindow {
			options.Width, options.Height = hostEditorWindowWidth, hostEditorWindowHeight
			options.MinWidth, options.MinHeight = 640, 560
		}
		if isSettingsWindow {

			options.Width, options.Height = dialogWindowSize(dialogKind)
			options.DisableResize = !dialogResizable
			if dialogResizable {
				options.MinWidth, options.MinHeight = 860, 560
			} else {
				options.MinWidth, options.MinHeight = options.Width, options.Height
				options.MaxWidth, options.MaxHeight = options.Width, options.Height
			}
		}
		var positionX, positionY int
		positionReady := false
		if !isPrimaryWindow {
			options.Hidden = true
			sourceWindow, _ := ctx.Value(application.WindowKey).(application.Window)
			if sourceWindow == nil {
				sourceWindow = app.Window.Current()
			}
			if isHostEditorWindow {
				positionX, positionY, positionReady = centeredHostEditorWindowPosition(sourceWindow, options.Width, options.Height)
			} else {
				positionX, positionY, positionReady = offsetWorkspaceWindowPosition(sourceWindow, options.Width, options.Height)
			}
			if positionReady {

				options.InitialPosition = application.WindowXY
				options.X = positionX
				options.Y = positionY
			}
		}

		window := app.Window.NewWithOptions(options)
		if isHostEditorWindow && service != nil {

			service.registerHostEditorWindow(window)
		}
		if isSettingsWindow && service != nil {

			service.settingsWindowMu.Lock()
			service.settingsWindow = window
			service.settingsWindowID = window.ID()
			service.settingsWindowMu.Unlock()
			if prewarmSettings {

			} else {

				service.markSettingsWindowOpen(true, "", settingsCategory, "")
			}

			window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {

				if service.tray == nil || service.tray.quitting.Load() || !service.settingsWindowEnabled.Load() {
					service.settingsWindowMu.Lock()
					cleared := service.settingsWindowID == window.ID()
					if cleared {
						service.settingsWindow = nil
						service.settingsWindowID = 0
					}
					service.settingsWindowMu.Unlock()

					if cleared {
						service.notifySettingsWindowHidden()
					}
					return
				}
				event.Cancel()
				window.Hide()
				service.notifySettingsWindowHidden()
			})
		}
		if positionReady {
			window.SetPosition(positionX, positionY)
		}
		if isPrimaryWindow {

			state := newTrayState(window)
			service.tray = state
			installTray(app, state, service, icon)

			app.OnShutdown(func() { state.quitting.Store(true) })

			window.RegisterHook(events.Common.WindowClosing, state.handlePrimaryWindowClose())

			window.OnWindowEvent(events.Common.WindowFocus, func(_ *application.WindowEvent) {
				service.refocusAuxiliaryWindow()
			})
		}
		configureFileDrop(window, service)
		var showTimer *time.Timer
		var showOnce sync.Once
		showWindow := func() {
			showOnce.Do(func() {
				window.Show()
			})
		}
		if !isPrimaryWindow && !deferShowUntilContentReady {
			window.OnWindowEvent(events.Common.WindowRuntimeReady, func(_ *application.WindowEvent) { showWindow() })
		}
		if !isPrimaryWindow && !deferShowUntilContentReady {

			showTimer = time.AfterFunc(3*time.Second, showWindow)
		}
		if onClose != nil {
			window.OnWindowEvent(events.Common.WindowClosing, func(_ *application.WindowEvent) { onClose() })
		}
		window.OnWindowEvent(events.Common.WindowClosing, func(_ *application.WindowEvent) {
			if showTimer != nil {
				showTimer.Stop()
			}

		})
		return window.Close
	}
	service.openWindow(context.Background(), "/", nil)

	go func() {
		time.Sleep(settingsWindowPrewarmDelay)
		if err := service.prewarmSettingsWindow(context.Background()); err != nil {
			return
		}
	}()

	traceStartup := os.Getenv("QTERM_STARTUP_TRACE") != ""
	if traceStartup {
		_, _ = fmt.Fprintln(os.Stderr, "qterm: 进入 Wails 主循环")
	}

	runErr := app.Run()
	if traceStartup {
		_, _ = fmt.Fprintf(os.Stderr, "qterm: Wails 主循环退出：%v\n", runErr)
	}
	shutdownErr := service.ServiceShutdown()

	diagnostics.Flush(2 * time.Second)
	return errors.Join(runErr, shutdownErr)
}

func workspaceWindowOptions(icon []byte) application.WebviewWindowOptions {
	return application.WebviewWindowOptions{
		Title:            "Qterm",
		EnableFileDrop:   true,
		Width:            1280,
		Height:           800,
		MinWidth:         900,
		MinHeight:        620,
		URL:              "/",
		BackgroundColour: application.NewRGB(247, 247, 248),

		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 42,
			Backdrop:                application.MacBackdropNormal,
			TitleBar: application.MacTitleBar{
				AppearsTransparent:   true,
				HideTitle:            true,
				FullSizeContent:      true,
				UseToolbar:           true,
				HideToolbarSeparator: true,
				ToolbarStyle:         application.MacToolbarStyleUnifiedCompact,
			},
		},

		Frameless: false,
		Linux:     application.LinuxWindow{Icon: icon},
	}
}

func offsetWorkspaceWindowPosition(source application.Window, width, height int) (int, int, bool) {
	if source == nil {
		return 0, 0, false
	}
	x, y := source.Position()
	x += 28
	y += 28
	screen, err := source.GetScreen()
	if err != nil || screen == nil {
		return x, y, true
	}
	workArea := screen.WorkArea
	maxX := workArea.X + max(workArea.Width-width, 0)
	maxY := workArea.Y + max(workArea.Height-height, 0)
	x = min(max(x, workArea.X), maxX)
	y = min(max(y, workArea.Y), maxY)
	return x, y, true
}
