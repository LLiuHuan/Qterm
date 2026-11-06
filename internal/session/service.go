package session

import (
	"changeme/internal/errno"
	"changeme/internal/model"
	"changeme/internal/recordingsdir"
	sshclient "changeme/internal/tunnel"
	"changeme/internal/x11"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type StreamTokens interface {
	Issue(string) (string, error)
	Revoke(string)
	Clear()
}

var (
	ErrServiceNotReady = errno.New(errno.CodeSessionClosed, "session.not_ready", "")
)

const resourceReadTimeout = 5 * time.Second

type Service struct {
	mu          sync.RWMutex
	configDir   func() (string, error)
	userHomeDir func() (string, error)

	recordingDirOverride string
	hosts                SessionHostSource
	credentials          SessionCredentialSource
	manager              *SessionManager

	forwards  *ForwardManager
	connector *Connector

	telnetConnector *TelnetConnector

	serialConnector *SerialConnector

	verifier *sshclient.HostKeyVerifier
	tokens   StreamTokens
	started  bool

	recordings *recordingRegistry
}

type OpenSessionResult struct {
	Session     model.Session `json:"session"`
	StreamToken string        `json:"streamToken"`

	Warnings []string `json:"warnings,omitempty"`
}

type TestConnectionInput struct {
	HostID       string               `json:"hostId,omitempty"`
	Protocol     string               `json:"protocol"`
	Address      string               `json:"address"`
	Port         int                  `json:"port"`
	Username     string               `json:"username"`
	CredentialID string               `json:"credentialId"`
	Kind         model.CredentialKind `json:"kind"`
	Secret       string               `json:"secret"`
}

func NewService(hosts SessionHostSource, credentials SessionCredentialSource, tokens StreamTokens) *Service {
	return &Service{
		configDir:   os.UserConfigDir,
		userHomeDir: os.UserHomeDir,
		hosts:       hosts,
		credentials: credentials,
		tokens:      tokens,
		recordings:  newRecordingRegistry(),
	}
}

func AttachRecordingIndex(s *Service, index RecordingIndex) {
	if s == nil || index == nil {
		return
	}
	s.mu.Lock()
	registry := s.recordings
	s.mu.Unlock()
	registry.SetIndex(index)
}

func (s *Service) SetX11Settings(settings x11.Settings) {
	if s == nil || s.connector == nil {
		return
	}
	s.connector.SetX11Settings(settings)
}

func (s *Service) KnownHostsPath() string {
	if s == nil {
		return ""
	}
	s.mu.RLock()
	verifier := s.verifier
	s.mu.RUnlock()
	if verifier == nil {
		return ""
	}
	return verifier.ManagedPath()
}

func (s *Service) ListKnownHosts(ctx context.Context) ([]model.KnownHostEntry, error) {
	if s == nil {
		return nil, ErrServiceNotReady
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	verifier := s.verifier
	s.mu.RUnlock()
	if verifier == nil {
		return nil, errno.New(errno.CodeInvalidArg, "session.not_started", "")
	}
	entries, err := verifier.List()
	if err != nil {
		return nil, errno.New(errno.CodeStoreFailed, "session.known_hosts_read_failed", "")
	}
	return entries, nil
}

func (s *Service) DeleteKnownHost(ctx context.Context, line int) error {
	if s == nil {
		return ErrServiceNotReady
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.RLock()
	verifier := s.verifier
	s.mu.RUnlock()
	if verifier == nil {
		return errno.New(errno.CodeInvalidArg, "session.not_started", "")
	}
	if err := verifier.DeleteManagedLine(line); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return errno.New(errno.CodeNotFound, "session.known_hosts_missing", "")
		}
		return errno.New(errno.CodeStoreFailed, "session.known_hosts_write_failed", "")
	}
	return nil
}

func (s *Service) Start(ctx context.Context) error {
	if s == nil || s.configDir == nil || s.userHomeDir == nil || s.hosts == nil || s.credentials == nil || s.tokens == nil {
		return ErrServiceNotReady
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return errno.New(errno.CodeInvalidArg, "session.already_started", "")
	}
	homeDir, err := s.userHomeDir()
	if err != nil {
		return errno.New(errno.CodeStoreFailed, "session.home_failed", "")
	}

	userKnownHosts := filepath.Join(homeDir, ".ssh", "known_hosts")
	readOnly := []string{
		filepath.Join(homeDir, ".ssh", "known_hosts2"),
	}
	verifier, err := sshclient.NewHostKeyVerifier(userKnownHosts, readOnly...)
	if err != nil {
		return errno.New(errno.CodeStoreFailed, "session.verifier_failed", "")
	}
	s.verifier = verifier
	connector := NewConnector(s.hosts, s.credentials, verifier)

	manager := NewSessionManager(connector, NewLocalConnector(sshclient.LocalShellPlan{}))

	telnetConnector := NewTelnetConnector(s.hosts)
	serialConnector := NewSerialConnector(s.hosts)
	s.serialConnector = serialConnector

	manager.SetOrphanSessionFinalizer(s.finalizeOrphanSession)

	s.connector = connector
	s.telnetConnector = telnetConnector
	s.manager = manager
	s.forwards = NewForwardManager(manager, nil)
	s.started = true
	return nil
}

func (s *Service) finalizeOrphanSession(sessionID string) {
	if s == nil {
		return
	}
	s.mu.RLock()
	connector := s.connector
	tokens := s.tokens
	recordings := s.recordings
	s.mu.RUnlock()
	if tokens != nil {
		tokens.Revoke(sessionID)
	}
	if recordings != nil {

		recordings.flushPending()
	}
	if connector != nil {
		connector.Forget(sessionID)
	}
}

func (s *Service) SetRecordingDirectory(directory string) error {
	if s == nil {
		return ErrServiceNotReady
	}
	normalized := ""
	if strings.TrimSpace(directory) != "" {
		value, err := recordingsdir.Validate(directory)
		if err != nil {
			return errno.New(errno.CodeInvalidArg, "recording.directory_invalid", "")
		}
		normalized = value
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recordingDirOverride = normalized
	return nil
}

func (s *Service) RecordingDirectory() (string, error) {
	if s == nil {
		return "", ErrServiceNotReady
	}
	s.mu.RLock()
	override := s.recordingDirOverride
	configDir := s.configDir
	s.mu.RUnlock()
	if strings.TrimSpace(override) != "" {
		root, err := recordingsdir.Validate(override)
		if err != nil {
			return "", errno.New(errno.CodeInvalidArg, "recording.directory_invalid", "")
		}
		return root, nil
	}
	if configDir == nil {
		return "", errno.New(errno.CodeStoreFailed, "recording.config_failed", "")
	}
	base, err := configDir()
	if err != nil {
		return "", errno.New(errno.CodeStoreFailed, "recording.config_failed", "")
	}
	return filepath.Join(base, "Qterm", "recordings"), nil
}

func (s *Service) SetDirectoryFollowMode(mode string) {
	if s == nil {
		return
	}
	s.mu.RLock()
	connector := s.connector
	s.mu.RUnlock()
	if connector != nil {
		connector.SetDirectoryFollowMode(mode)
	}
}

func (s *Service) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	manager := s.manager
	forwards := s.forwards
	recordings := s.recordings
	s.forwards = nil
	s.started = false
	s.manager = nil
	s.connector = nil
	s.telnetConnector = nil
	s.tokens.Clear()
	s.mu.Unlock()

	if forwards != nil {
		forwards.CloseAll()
	}
	if manager != nil {

		var recordingErr error
		for _, failure := range recordings.finishAll() {
			if failure != nil && recordingErr == nil {
				recordingErr = failure
			}
		}
		sessionCtx, cancelSessions := context.WithTimeout(context.Background(), 5*time.Second)
		err := manager.Shutdown(sessionCtx)
		cancelSessions()
		if err != nil {
			return errno.New(errno.CodeSessionClosed, "session.close_timeout", "")
		}
		if recordingErr != nil {
			return errno.Wrap(errno.CodeStoreFailed, "recording.close_failed", recordingErr)
		}
	}
	return nil
}

func (s *Service) ready() (
	*SessionManager,
	*Connector,
	error,
) {
	if s == nil {
		return nil, nil, ErrServiceNotReady
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.started || s.manager == nil || s.connector == nil {
		return nil, nil, ErrServiceNotReady
	}
	return s.manager, s.connector, nil
}

func (s *Service) issueStreamToken(
	manager *SessionManager,
	sessionID string,
) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.started || s.manager != manager {
		return "", ErrServiceNotReady
	}
	return s.tokens.Issue(sessionID)
}

func sessionServiceError(err error) error {
	switch {

	case errno.Extract(err) != nil:
		return err
	case errors.Is(err, model.ErrInvalidSession):
		return errno.New(errno.CodeInvalidArg, "session.invalid", "")
	case errors.Is(err, ErrSessionNotFound):
		return errno.New(errno.CodeNotFound, "session.not_found", "")
	case errors.Is(err, ErrSessionNotRetryable):
		return errno.New(errno.CodeInvalidArg, "session.not_retryable", "")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	default:
		return errno.New(errno.CodeConnFailed, "session.failed", "")
	}
}

func (s *Service) Manager() (*SessionManager, error) {
	manager, _, err := s.ready()
	return manager, err
}

func closeSessionAfterToken(manager *SessionManager, sessionID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = manager.Close(ctx, sessionID)
}
