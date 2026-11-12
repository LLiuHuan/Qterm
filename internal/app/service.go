package app

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"changeme/internal/ai"
	"changeme/internal/applock"
	"changeme/internal/cliapi"
	"changeme/internal/credential"
	"changeme/internal/diagnostics"
	"changeme/internal/fonts"
	"changeme/internal/host"
	"changeme/internal/otp"
	"changeme/internal/preferences"
	"changeme/internal/preview"
	"changeme/internal/proxy"
	"changeme/internal/remotedesktop"
	"changeme/internal/remoteedit"
	"changeme/internal/security"
	"changeme/internal/session"
	"changeme/internal/sftp"
	"changeme/internal/terminal"
	"changeme/internal/tmux"
	"changeme/internal/translate"
	"changeme/internal/update"
	"changeme/internal/vnc"
	"changeme/internal/zmodem"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type Service struct {
	mu       sync.Mutex
	windowMu sync.Mutex

	assetMu sync.RWMutex

	ai *ai.Service

	aiStreams *aiStreamStore

	agentAuditPath string
	agentAuditMu   sync.Mutex

	agentDebug *agentDebugLog

	agentDraft agentDraftThrottle

	tempSessions *aiTempSessions

	agents *ai.Manager

	adhoc *adHocHostRegistry

	tray        *trayState
	hosts       *host.HostService
	credentials *credential.CredentialService

	proxies  *proxy.Service
	security *security.Manager

	otp *otp.Service

	translate *translate.Service

	translationCredentialsMu sync.Mutex

	appLock     *applock.Manager
	diagnostics *diagnostics.Manager

	updates     *update.Service
	preferences *preferences.Store

	cliServer *cliapi.Server

	cliHandshake string
	fonts        *fonts.Service
	sessions     *session.Service

	remoteDesktops *remotedesktop.Manager

	vncSessions *vnc.Manager

	vncDialRoute func(ctx context.Context, hostID, address string) (net.Conn, func() error, error)

	rdpDialRoute rdpRouteDialer

	rdpSessionOpener rdpSessionOpener

	rdpBridgeStarter func(ctx context.Context, dial rdpRouteDialer, target string) (*rdpRouteBridge, net.Addr, error)

	rdpRouteConfirmTimeoutValue time.Duration

	rdpRouteMu      sync.Mutex
	rdpRouteBridges map[string]func() error

	rdpCertificates *remotedesktop.CertificateStore
	sftp            *sftp.Service

	tmux        *tmux.Service
	remoteEdits *remoteedit.Manager

	previews *preview.Service
	tokens   *terminal.TokenStore

	terminalControl *terminal.Manager

	rdpTokens   *terminal.TokenStore
	zmodem      *zmodem.Manager
	fileDialogs *appFileDialogs

	recordingDirOverride string
	openWindow           func(context.Context, string, func()) func()

	settingsWindowMu sync.Mutex
	settingsWindow   application.Window
	settingsWindowID uint

	settingsWindowEnabled atomic.Bool

	settingsWindowShown atomic.Bool

	hostEditorMu       sync.Mutex
	hostEditor         application.Window
	hostEditorID       uint
	hostEditorTargetID string
	hostEditorShown    atomic.Bool

	appLocked       atomic.Bool
	windowTransfers map[string]*sessionWindowTransfer

	rdpWindowHandovers map[string]*rdpWindowHandover
	rdpWindowMu        sync.Mutex

	rdpSessionChecker func(sessionID string) error

	rdpSessionCloser func(sessionID string) error

	dockerHooks *dockerTestHooks
	started     bool
}

func NewService(
	hostService *host.HostService,
	credentialService *credential.CredentialService,
	proxyService *proxy.Service,
	securityManager *security.Manager,
	appLockManager *applock.Manager,
	diagnosticsManager *diagnostics.Manager,
	fontService *fonts.Service,
	sessionService *session.Service,
	sftpService *sftp.Service,
	remoteEditManager *remoteedit.Manager,
	previewService *preview.Service,
	tokens *terminal.TokenStore,
	rdpTokens *terminal.TokenStore,
	files *zmodem.Manager,
	tmuxService *tmux.Service,
) *Service {
	linkServices(hostService, credentialService, sessionService)
	return &Service{
		ai:              ai.NewService(),
		aiStreams:       newAIStreamStore(),
		agents:          ai.NewManager(),
		tempSessions:    newAITempSessions(sessionService),
		hosts:           hostService,
		credentials:     credentialService,
		proxies:         proxyService,
		security:        securityManager,
		otp:             newOTPService(securityManager),
		appLock:         appLockManager,
		diagnostics:     diagnosticsManager,
		fonts:           fontService,
		sessions:        sessionService,
		sftp:            sftpService,
		tmux:            tmuxService,
		remoteEdits:     remoteEditManager,
		previews:        previewService,
		tokens:          tokens,
		rdpTokens:       rdpTokens,
		zmodem:          files,
		windowTransfers: make(map[string]*sessionWindowTransfer),
	}
}

func (s *Service) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	if s == nil {
		return errors.New("应用服务未配置")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hosts == nil || s.credentials == nil || s.proxies == nil || s.security == nil || s.diagnostics == nil || s.fonts == nil || s.sessions == nil || s.sftp == nil || s.tokens == nil || s.rdpTokens == nil || s.zmodem == nil || s.remoteEdits == nil || s.previews == nil {
		return errors.New("应用服务依赖未配置")
	}
	if s.started {
		return errors.New("应用服务已经启动")
	}

	s.settingsWindowEnabled.Store(true)
	if err := s.security.Start(ctx); err != nil {
		return err
	}
	preferenceStore, err := preferences.Open(ctx, filepath.Join(s.security.Root(), "preferences.db"))
	if err != nil {
		_ = s.security.Close()
		return err
	}
	s.preferences = preferenceStore

	s.updates = newUpdateService()

	s.setupAgentAudit()
	s.agentDebug = newAgentDebugLog(filepath.Join(s.security.Root(), "logs", agentDebugFileName))

	s.remoteDesktops = remotedesktop.NewManager()

	s.vncSessions = vnc.NewManager()

	s.rdpCertificates = remotedesktop.NewCertificateStore(s.security.Root())
	if err := s.fonts.Start(ctx); err != nil {
		_ = s.preferences.Close()
		s.preferences = nil
		_ = s.security.Close()
		return err
	}
	if err := s.credentials.Start(ctx); err != nil {
		_ = s.preferences.Close()
		s.preferences = nil
		_ = s.fonts.Close()
		_ = s.security.Close()
		return err
	}
	if err := s.hosts.Start(ctx); err != nil {
		_ = s.preferences.Close()
		s.preferences = nil

		_ = s.credentials.Close()
		_ = s.hosts.Close()
		_ = s.security.Close()
		return err
	}
	sftpConcurrency := sftp.DefaultTransferConcurrency
	sftpSettings := sftp.DefaultSettings()

	if raw, found, prefErr := s.preferences.Get(ctx, "qterm.settings.v1"); prefErr == nil && found {
		var settings struct {
			SessionHistoryLimit int    `json:"sessionHistoryLimit"`
			SFTPMaxTransfers    int    `json:"sftpMaxTransfers"`
			SFTPAutoFollow      *bool  `json:"sftpAutoDirectoryFollow"`
			SFTPShowHidden      *bool  `json:"sftpShowHiddenFiles"`
			SFTPLocalDirectory  string `json:"sftpLocalDirectory"`
			SFTPEditorPath      string `json:"sftpEditorPath"`

			SFTPDragFolderUploadMode string `json:"sftpDragFolderUploadMode"`
			RecordingDirectory       string `json:"recordingDirectory"`

			RdpFirstFrameTimeoutSeconds *int `json:"rdpFirstFrameTimeoutSeconds"`
			RdpHostProbeTimeoutSeconds  *int `json:"rdpHostProbeTimeoutSeconds"`
		}
		if json.Unmarshal([]byte(raw), &settings) == nil {

			if err := s.applyRecordingDirectoryLocked(settings.RecordingDirectory); err != nil {
				_ = s.applyRecordingDirectoryLocked("")
			}
			if settings.SessionHistoryLimit > 0 {
				_ = s.hosts.SetSessionHistoryLimit(settings.SessionHistoryLimit)
			}

			if s.remoteDesktops != nil {
				firstFrame, probe := rdpConnectTimeouts(
					settings.RdpFirstFrameTimeoutSeconds,
					settings.RdpHostProbeTimeoutSeconds,
				)
				s.remoteDesktops.ApplyConnectTimeouts(firstFrame, probe)
			}
		}
		if settings.SFTPMaxTransfers >= 1 && settings.SFTPMaxTransfers <= 16 {
			sftpConcurrency = settings.SFTPMaxTransfers
			sftpSettings.MaxTransfers = settings.SFTPMaxTransfers
		}
		if settings.SFTPAutoFollow != nil {
			sftpSettings.AutoDirectoryFollow = *settings.SFTPAutoFollow
		}
		if settings.SFTPShowHidden != nil {
			sftpSettings.ShowHiddenFiles = *settings.SFTPShowHidden
		}
		sftpSettings.LocalDirectory = settings.SFTPLocalDirectory
		sftpSettings.EditorPath = settings.SFTPEditorPath

		if settings.SFTPDragFolderUploadMode != "" {
			sftpSettings.DragFolderUploadMode = settings.SFTPDragFolderUploadMode
		}
	}
	if err := s.sessions.Start(ctx); err != nil {
		_ = s.preferences.Close()
		s.preferences = nil
		_ = s.credentials.Close()
		_ = s.hosts.Close()
		_ = s.security.Close()
		return err
	}
	if raw, found, prefErr := s.preferences.Get(ctx, "qterm.settings.v1"); prefErr == nil && found {
		var settings struct {
			Mode string `json:"sftpDirectoryFollowMode"`
		}
		if json.Unmarshal([]byte(raw), &settings) == nil {
			s.sessions.SetDirectoryFollowMode(settings.Mode)
		}
	}
	manager, err := s.sessions.Manager()
	if err != nil {
		_ = s.sessions.Close()
		_ = s.credentials.Close()
		_ = s.hosts.Close()
		_ = s.preferences.Close()
		s.preferences = nil
		_ = s.security.Close()
		return err
	}

	s.terminalControl = terminal.NewManager(ctx, terminalSessionDirectory{service: s.sessions}, (terminalSessionWriter{service: s.sessions}).Write)

	s.sftp.SetCommandRunner(sftpCommandRunner{sessions: s.sessions})

	if s.tmux != nil {
		s.tmux.SetCommandRunner(tmuxCommandRunner{sessions: s.sessions})
	}
	if err := s.sftp.StartWithConcurrency(manager, sftpConcurrency); err != nil {
		_ = s.sessions.Close()
		_ = s.credentials.Close()
		_ = s.hosts.Close()
		_ = s.preferences.Close()
		s.preferences = nil
		_ = s.security.Close()
		return err
	}
	if err := s.sftp.ApplySettings(sftpSettings); err != nil {
		_ = s.sftp.Close()
		_ = s.sessions.Close()
		_ = s.credentials.Close()
		_ = s.hosts.Close()
		_ = s.preferences.Close()
		s.preferences = nil
		_ = s.security.Close()
		return err
	}

	if err := s.remoteEdits.Start(ctx); err != nil {
		_ = s.remoteEdits.Close()
		_ = s.sftp.Close()
		_ = s.sessions.Close()
		_ = s.credentials.Close()
		_ = s.hosts.Close()
		_ = s.preferences.Close()
		s.preferences = nil
		_ = s.security.Close()
		return err
	}

	s.appLocked.Store(s.appLock != nil && s.appLock.Status().Enabled)

	if err := s.startCLIServer(ctx); err != nil {
		logCLIError(err)
	}
	s.started = true
	return nil
}

func (s *Service) ServiceShutdown() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return nil
	}
	var firstErr error

	s.stopCLIServer()

	if s.updates != nil {
		_ = s.updates.Close()
	}

	if s.agents != nil {

		s.persistAgentAudit()

		_ = s.agents.Close()
	}
	if s.agentDebug != nil {
		s.agentDebug.Close()
	}
	if s.remoteDesktops != nil {
		s.remoteDesktops.CloseAll()
	}

	s.releaseAllRdpRouteBridges()
	if s.vncSessions != nil {
		s.vncSessions.CloseAll()
	}
	if err := s.remoteEdits.Close(); err != nil {
		firstErr = err
	}
	if err := s.zmodem.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := s.sftp.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if s.terminalControl != nil {
		if err := s.terminalControl.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		s.terminalControl = nil
	}
	if err := s.sessions.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := s.credentials.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := s.hosts.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := s.preferences.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := s.security.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := s.fonts.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := s.diagnostics.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	s.preferences = nil
	s.started = false
	return firstErr
}

func rdpConnectTimeouts(firstFrameSeconds, probeSeconds *int) (firstFrame, probe time.Duration) {
	toDuration := func(seconds *int) time.Duration {
		if seconds == nil {
			return 0
		}
		return time.Duration(*seconds) * time.Second
	}
	return toDuration(firstFrameSeconds), toDuration(probeSeconds)
}

func linkServices(hostService *host.HostService, credentialService *credential.CredentialService, sessionService *session.Service) {
	guard := &sync.RWMutex{}
	host.AttachCredentialLookup(hostService, guard, func(ctx context.Context, id string) (bool, error) {
		return credential.CredentialExists(credentialService, ctx, id)
	})
	credential.AttachReferenceCounter(credentialService, guard, func(ctx context.Context, id string) (int, error) {
		return host.CredentialReferenceCount(hostService, ctx, id)
	})
	session.AttachRecordingIndex(sessionService, hostService)
}
