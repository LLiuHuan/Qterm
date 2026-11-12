package session

import (
	"changeme/internal/credential"
	"changeme/internal/errno"
	"changeme/internal/model"
	sshclient "changeme/internal/tunnel"
	"changeme/internal/x11"
	"context"
	"errors"
	xssh "golang.org/x/crypto/ssh"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type SessionHostSource interface {
	SessionHost(ctx context.Context, id string) (model.Host, error)
	UseHostPassword(ctx context.Context, id string, use func([]byte) error) error
}

type ProxySource interface {
	ProxyByID(ctx context.Context, id string) (model.ProxyRecord, error)
}

type ProxyPasswordSource interface {
	UseProxyPassword(ctx context.Context, id string, use func([]byte) error) error
}

type HostProxyPasswordSource interface {
	UseHostProxyPassword(ctx context.Context, id string, use func([]byte) error) error
}

type hostPasswordRememberer interface {
	RememberHostPassword(ctx context.Context, id string, password []byte) error
}

type hostCredentialRememberer interface {
	RememberHostCredential(ctx context.Context, id, credentialID string) error
}

type AuthenticationQuestion struct {
	Prompt string `json:"prompt"`
	Echo   bool   `json:"echo"`
}

type AuthenticationChallenge struct {
	SessionID   string                   `json:"sessionId"`
	Address     string                   `json:"address"`
	Username    string                   `json:"username"`
	RequestID   string                   `json:"requestId"`
	Kind        string                   `json:"kind"`
	HostID      string                   `json:"hostId,omitempty"`
	Instruction string                   `json:"instruction,omitempty"`
	Questions   []AuthenticationQuestion `json:"questions,omitempty"`

	HopIndex  int    `json:"hopIndex,omitempty"`
	HopHostID string `json:"hopHostId,omitempty"`
}

type AuthenticationInput struct {
	Secret       string   `json:"secret"`
	CredentialID string   `json:"credentialId"`
	Remember     bool     `json:"remember"`
	RequestID    string   `json:"requestId,omitempty"`
	Answers      []string `json:"answers,omitempty"`
}

type authenticationRequest struct {
	challenge AuthenticationChallenge
	input     chan AuthenticationInput
	submitted bool

	prefilled []string

	visibleIndexes []int
}

type interactivePassword struct {
	mu    sync.Mutex
	value string
}

func newInteractivePassword(secret []byte) *interactivePassword {
	if len(secret) == 0 {
		return nil
	}
	return &interactivePassword{value: string(secret)}
}

func (p *interactivePassword) take() string {
	if p == nil {
		return ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	value := p.value
	p.value = ""
	return value
}

func singlePasswordQuestionIndex(questions []string, echos []bool) int {
	index := -1
	for position, question := range questions {

		if position >= len(echos) || echos[position] {
			continue
		}
		prompt := strings.ToLower(strings.TrimSpace(question))
		if prompt != "password" && prompt != "password:" {
			continue
		}
		if index >= 0 {
			return -1
		}
		index = position
	}
	return index
}

var authenticationRequestSequence atomic.Uint64

func newAuthenticationRequestID(sessionID string) string {
	sequence := authenticationRequestSequence.Add(1)
	return sessionID + "-auth-" + strconv.FormatUint(sequence, 10)
}

type SessionHostIconUpdater interface {
	SetDetectedHostIcon(ctx context.Context, id string, icon string) error
}

type remoteOSDetector interface {
	DetectRemoteOS(ctx context.Context) (string, error)
}

type SessionCredentialSource interface {
	UseCredentialSecret(
		ctx context.Context,
		id string,
		use func(model.CredentialKind, []byte) error,
	) error
}

type sessionDialer interface {
	Connect(
		ctx context.Context,
		target sshclient.Target,
		kind model.CredentialKind,
		secret []byte,
	) (SessionConnection, error)
}

type interactiveSessionDialer interface {
	ConnectWithAuth(
		ctx context.Context,
		target sshclient.Target,
		kind model.CredentialKind,
		secret []byte,
		extra []xssh.AuthMethod,
		interactive bool,
	) (SessionConnection, error)
}

type sessionDialerFactory func(xssh.HostKeyCallback) sessionDialer

func (c *Connector) SetX11Settings(settings x11.Settings) {
	c.mu.Lock()
	c.x11Settings = x11.Normalize(settings)
	c.mu.Unlock()
}

func (c *Connector) currentX11Settings() x11.Settings {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.x11Settings
}

type Connector struct {
	hosts       SessionHostSource
	credentials SessionCredentialSource
	verifier    *sshclient.HostKeyVerifier
	newDialer   sessionDialerFactory

	x11Settings x11.Settings

	mu                  sync.RWMutex
	pending             map[string][]*sshclient.HostKeyError
	pendingAuth         map[string]*authenticationRequest
	failures            map[string]string
	directoryFollowMode string
}

func (c *Connector) SetDirectoryFollowMode(mode string) {
	if mode != "session" && mode != "remote" && mode != "disabled" {
		mode = "remote"
	}
	c.mu.Lock()
	c.directoryFollowMode = mode
	c.mu.Unlock()
}

type hopState struct {
	mu    sync.Mutex
	index int
	label string
	isHop bool
}

func (s *hopState) set(index int, label string, isHop bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.index, s.label, s.isHop = index, label, isHop
	s.mu.Unlock()
}

func (s *hopState) snapshot() (int, string, bool) {
	if s == nil {
		return 0, "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.index, s.label, s.isHop
}

type productionSessionDialer struct {
	connector *sshclient.Connector

	hostKey xssh.HostKeyCallback

	sessionConnector *Connector

	hopState *hopState
}

var (
	_ auxiliaryProvisionerInstaller = (*productionSessionConnection)(nil)
	_ fileSystemConnectionPlanner   = (*productionSessionConnection)(nil)
	_ transferFileSystemOpener      = (*productionSessionConnection)(nil)
	_ latencyCapableConnection      = (*productionSessionConnection)(nil)
	_ latencyEnabledConnection      = (*productionSessionConnection)(nil)
	_ fileSystemCapableConnection   = (*productionSessionConnection)(nil)
)

type productionSessionConnection struct {
	connection          *sshclient.Connection
	directoryFollowMode string
	latencyEnabled      bool

	terminalEncoding     string
	postLoginCommand     string
	postLoginDelay       time.Duration
	terminalType         string
	networkDevice        bool
	environmentVariables []sshclient.TerminalEnvironmentVariable

	x11        *sshclient.X11ForwardingOptions
	x11Warning string

	route *resolvedRoute
}

func resolveX11Forwarding(
	ctx context.Context,
	host model.Host,
	settings x11.Settings,
) (*sshclient.X11ForwardingOptions, string) {
	if !host.X11Forwarding || host.ConnectionType == model.ConnectionTypeNetwork {

		return nil, ""
	}
	detected, err := x11.Detect(ctx, settings)
	if err != nil || !detected.Available {
		return nil, "session.x11_unavailable"
	}
	return &sshclient.X11ForwardingOptions{
		Display:      detected.Display,
		Screen:       detected.Screen,
		AuthProtocol: detected.AuthProtocol,
		AuthCookie:   detected.Cookie,
		Trusted:      host.X11Trusted,
	}, ""
}

func terminalEnvironmentVariables(host model.Host) []sshclient.TerminalEnvironmentVariable {
	if len(host.EnvironmentVariables) == 0 {
		return nil
	}
	values := make([]sshclient.TerminalEnvironmentVariable, 0, len(host.EnvironmentVariables))
	for _, item := range host.EnvironmentVariables {
		values = append(values, sshclient.TerminalEnvironmentVariable{Name: item.Name, Value: item.Value})
	}
	return values
}

func (c *productionSessionConnection) setTerminalBehavior(
	encoding string,
	postLoginCommand string,
	postLoginDelay time.Duration,
	terminalType string,
	networkDevice bool,
	environment []sshclient.TerminalEnvironmentVariable,
) {
	c.terminalEncoding = encoding
	c.postLoginCommand = postLoginCommand
	c.postLoginDelay = postLoginDelay
	c.terminalType = terminalType
	c.networkDevice = networkDevice
	c.environmentVariables = environment
}

func (c *productionSessionConnection) setX11Forwarding(options *sshclient.X11ForwardingOptions, warning string) {
	c.x11 = options
	c.x11Warning = warning
}

func (c *productionSessionConnection) X11Warning() string {
	if c == nil {
		return ""
	}
	return c.x11Warning
}

func (c *productionSessionConnection) terminalCharacterEncoding() string {
	if c == nil || c.terminalEncoding == "" {
		return "utf-8"
	}
	return c.terminalEncoding
}

func (c *productionSessionConnection) SetAuxiliaryProvisioner(provisioner sshclient.ConnectionProvisioner) {
	if c == nil || c.connection == nil {
		return
	}
	c.connection.SetAuxiliaryProvisioner(provisioner)
}

type directoryFollowSetter interface{ setDirectoryFollow(string) }

func (c *productionSessionConnection) SetFileSystemConnections(count int) {
	if c == nil || c.connection == nil {
		return
	}
	c.connection.SetFileSystemConnections(count)
}

func (c *productionSessionConnection) setDirectoryFollow(value string) { c.directoryFollowMode = value }
func (c *productionSessionConnection) LatencyEnabled() bool            { return c.latencyEnabled }
func (c *productionSessionConnection) setConnectionOptions(keepAlive, latencyEnabled bool) {
	c.latencyEnabled = latencyEnabled
	if !keepAlive && c.connection != nil {
		c.connection.DisableKeepalive()
	}
}

func (c *productionSessionConnection) MeasureLatency(ctx context.Context) (time.Duration, error) {
	return c.connection.MeasureLatency(ctx)
}

func (c *productionSessionConnection) ReadResources(ctx context.Context) (model.ResourceSnapshot, error) {
	return c.connection.ReadResources(ctx)
}

func (c *productionSessionConnection) DetectRemoteOS(ctx context.Context) (string, error) {
	if c == nil || c.connection == nil {
		return "", sshclient.ErrRemoteOSUnsupported
	}
	return c.connection.DetectRemoteOS(ctx)
}

func NewConnector(
	hosts SessionHostSource,
	credentials SessionCredentialSource,
	verifier *sshclient.HostKeyVerifier,
) *Connector {
	return newConnectorWithFactory(
		hosts,
		credentials,
		verifier,
		func(callback xssh.HostKeyCallback) sessionDialer {
			return &productionSessionDialer{
				connector: sshclient.NewConnector(callback),
				hostKey:   callback,
			}
		},
	)
}

func newConnectorWithFactory(
	hosts SessionHostSource,
	credentials SessionCredentialSource,
	verifier *sshclient.HostKeyVerifier,
	newDialer sessionDialerFactory,
) *Connector {
	connector := &Connector{
		hosts:       hosts,
		credentials: credentials,
		verifier:    verifier,
		newDialer:   newDialer,
		pending:     make(map[string][]*sshclient.HostKeyError),
		pendingAuth: make(map[string]*authenticationRequest),
		failures:    make(map[string]string),
	}
	return connector
}

func (c *Connector) Connect(
	ctx context.Context,
	sessionID string,
	hostID string,
	reportState func(model.SessionState) error,
) (SessionConnection, error) {
	if c == nil || c.hosts == nil || c.credentials == nil || c.verifier == nil || c.newDialer == nil {
		return nil, errno.New(errno.CodeSessionClosed, "session.connector_not_ready", "")
	}

	if sessionID != "" {
		ctx = sshclient.WithInteractiveWait(ctx)
	}
	c.Forget(sessionID)
	host, err := c.hosts.SessionHost(ctx, hostID)
	if err != nil {
		c.rememberFailure(sessionID, "session.host_unavailable")
		return nil, err
	}
	if host.CredentialID == "" && !host.PasswordConfigured && host.PasswordSource != "prompt" {
		c.rememberFailure(sessionID, "session.credential_unlinked")
		return nil, errno.New(errno.CodeInvalidArg, "session.credential_unlinked", "")
	}

	target := sshclient.Target{
		Address:  host.Address,
		Port:     host.Port,
		Username: host.Username,

		Timeout: hostConnectTimeout(host),
	}
	hopInfo := &hopState{}
	callback := func(hostname string, remote net.Addr, key xssh.PublicKey) error {
		err := c.verifier.Verify(hostname, remote, key)
		if err != nil {
			var hostKeyErr *sshclient.HostKeyError
			if errors.As(err, &hostKeyErr) {

				if index, label, isHop := hopInfo.snapshot(); isHop {
					hostKeyErr.Challenge.HopIndex = index + 1
					hostKeyErr.Challenge.HopLabel = label
				}
				c.remember(sessionID, hostKeyErr)
			}
			return err
		}

		if !sameHostPort(hostname, target.Address, target.Port) {
			return nil
		}

		if reportErr := reportState(model.SessionStateAuthenticating); reportErr != nil &&
			!errors.Is(reportErr, model.ErrInvalidSessionTransition) {
			return reportErr
		}
		return nil
	}
	dialer := c.newDialer(callback)

	if production, ok := dialer.(*productionSessionDialer); ok {
		production.sessionConnector = c
		production.hopState = hopInfo
	}
	if dialer == nil {
		c.rememberFailure(sessionID, "session.connector_not_ready")
		return nil, errno.New(errno.CodeSessionClosed, "session.connector_not_ready", "")
	}

	route, routeErr := c.resolveRouteWith(ctx, host, routeAuthContext{
		sessionID:   sessionID,
		reportState: reportState,
	})
	if routeErr != nil {
		c.rememberFailure(sessionID, sessionRouteFailureKey(routeErr))
		return nil, routeErr
	}

	makeTargetInteractive := func(kind model.CredentialKind, secret []byte) []xssh.AuthMethod {
		if sessionID == "" {
			return nil
		}
		var reusablePassword *interactivePassword
		if kind == model.CredentialKindPassword {
			reusablePassword = newInteractivePassword(secret)
		}
		return []xssh.AuthMethod{
			c.keyboardInteractiveAuth(
				ctx,
				routeAuthContext{sessionID: sessionID, reportState: reportState},
				host,
				0,
				reusablePassword,
			),
		}
	}

	if len(route.hops) > 0 {
		challenges, probeErr := c.probeRouteHostKeys(ctx, route, target)
		if probeErr != nil {
			var hostKeyErr *sshclient.HostKeyError
			if errors.As(probeErr, &hostKeyErr) {

				c.remember(sessionID, hostKeyErr)
				return nil, &PendingSessionError{
					State: model.SessionStateHostKeyPending,
					Err:   hostKeyErr,
				}
			}
			c.rememberFailure(sessionID, SessionConnectionFailureKey(probeErr))
			return nil, probeErr
		}
		if len(challenges) > 0 {

			c.rememberAll(sessionID, challenges)
			return nil, &PendingSessionError{
				State: model.SessionStateHostKeyPending,
				Err:   challenges[0],
			}
		}
	}

	routeAdopted := false
	defer func() {
		if !routeAdopted {
			route.release()
		}
	}()
	var connection SessionConnection
	if host.PasswordSource == "prompt" && host.CredentialID == "" {
		request := &authenticationRequest{
			challenge: AuthenticationChallenge{
				SessionID: sessionID,
				Address:   host.Address,
				Username:  host.Username,
				RequestID: newAuthenticationRequestID(sessionID),
				Kind:      authenticationKindCredential,
				HostID:    host.ID,
			},
			input: make(chan AuthenticationInput, 1),
		}
		c.mu.Lock()
		c.pendingAuth[sessionID] = request
		c.mu.Unlock()
		defer func() {
			c.removePendingAuthentication(sessionID, request)
		}()

		if err = reportState(model.SessionStateAwaitingAuth); err != nil {
			return nil, err
		}
		var input AuthenticationInput
		select {
		case input = <-request.input:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		if input.CredentialID != "" {
			credentialID := input.CredentialID
			err = c.credentials.UseCredentialSecret(ctx, input.CredentialID, func(kind model.CredentialKind, secret []byte) error {

				if kind != model.CredentialKindPassword &&
					kind != model.CredentialKindPrivateKey &&
					kind != model.CredentialKindPrivateKeyPassphrase &&
					kind != model.CredentialKindSSHAgent {
					return errno.New(errno.CodeInvalidArg, "session.invalid_authentication", "")
				}
				var connectErr error
				targetInteractive := makeTargetInteractive(kind, secret)
				connection, connectErr = c.connectThroughRouteWithAuth(ctx, dialer, route, target, kind, secret, targetInteractive, len(targetInteractive) > 0)
				return connectErr
			})
			if err == nil && input.Remember {

				if rememberer, ok := c.hosts.(hostCredentialRememberer); ok {
					_ = rememberer.RememberHostCredential(ctx, host.ID, credentialID)
				}
			}
		} else {
			secret := []byte(input.Secret)
			input.Secret = ""
			if len(secret) == 0 {
				return nil, errno.New(errno.CodeInvalidArg, "session.invalid_authentication", "")
			}
			doRemember := input.Remember
			targetInteractive := makeTargetInteractive(model.CredentialKindPassword, secret)
			connection, err = c.connectThroughRouteWithAuth(ctx, dialer, route, target, model.CredentialKindPassword, secret, targetInteractive, len(targetInteractive) > 0)
			if err == nil && doRemember {

				if rememberer, ok := c.hosts.(hostPasswordRememberer); ok {
					_ = rememberer.RememberHostPassword(ctx, host.ID, secret)
				}
			}
			clear(secret)
		}
	} else if host.CredentialID != "" {
		err = c.credentials.UseCredentialSecret(
			ctx,
			host.CredentialID,
			func(kind model.CredentialKind, secret []byte) error {
				var connectErr error
				targetInteractive := makeTargetInteractive(kind, secret)
				connection, connectErr = c.connectThroughRouteWithAuth(ctx, dialer, route, target, kind, secret, targetInteractive, len(targetInteractive) > 0)
				return connectErr
			},
		)
	} else {
		err = c.hosts.UseHostPassword(ctx, host.ID, func(secret []byte) error {
			var connectErr error
			targetInteractive := makeTargetInteractive(model.CredentialKindPassword, secret)
			connection, connectErr = c.connectThroughRouteWithAuth(ctx, dialer, route, target, model.CredentialKindPassword, secret, targetInteractive, len(targetInteractive) > 0)
			return connectErr
		})
	}
	if err != nil {
		var hostKeyErr *sshclient.HostKeyError

		if errors.As(err, &hostKeyErr) {
			return nil, &PendingSessionError{
				State: model.SessionStateHostKeyPending,
				Err:   hostKeyErr,
			}
		}
		c.rememberFailure(sessionID, SessionConnectionFailureKey(err))
		return nil, err
	}
	if connection == nil {
		c.rememberFailure(sessionID, "session.connection_missing")
		return nil, errno.New(errno.CodeConnFailed, "session.connection_missing", "")
	}

	routeAdopted = true
	if setter, ok := connection.(directoryFollowSetter); ok {
		c.mu.RLock()
		mode := c.directoryFollowMode
		c.mu.RUnlock()
		if mode != "session" && mode != "disabled" {
			mode = "remote"
		}
		hostMode := host.DirectoryFollow
		if hostMode == "" {
			hostMode = "inherit"
		}

		switch hostMode {
		case "disabled":
			mode = "disabled"
		case "enabled":
			if mode == "disabled" {
				mode = "session"
			}
		}
		setter.setDirectoryFollow(mode)
	}
	if configured, ok := connection.(interface{ setConnectionOptions(bool, bool) }); ok {

		networkDevice := host.ConnectionType == model.ConnectionTypeNetwork
		configured.setConnectionOptions(host.SSHKeepAlive && !networkDevice, host.LatencyDetection && !networkDevice)
	}
	if configured, ok := connection.(interface {
		setTerminalBehavior(string, string, time.Duration, string, bool, []sshclient.TerminalEnvironmentVariable)
	}); ok {
		configured.setTerminalBehavior(
			host.Encoding,
			host.PostLoginCommand,
			time.Duration(host.PostLoginCommandDelaySeconds)*time.Second,
			host.TerminalType,
			host.ConnectionType == model.ConnectionTypeNetwork,
			terminalEnvironmentVariables(host),
		)
	}
	if configured, ok := connection.(interface {
		setX11Forwarding(*sshclient.X11ForwardingOptions, string)
	}); ok {
		options, warning := resolveX11Forwarding(ctx, host, c.currentX11Settings())
		configured.setX11Forwarding(options, warning)
	}

	if host.Icon == "" && host.AutoDetectOSIcon && host.ConnectionType != model.ConnectionTypeNetwork && ctx != nil {
		if detector, ok := connection.(remoteOSDetector); ok {

			probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			icon, probeErr := detector.DetectRemoteOS(probeCtx)
			cancel()

			if probeErr == nil && icon != "" {
				if updater, updateOK := c.hosts.(SessionHostIconUpdater); updateOK {

					_ = updater.SetDetectedHostIcon(ctx, host.ID, icon)
				}
			}
		}
	}
	if host.ConnectionType != model.ConnectionTypeNetwork {

		c.installAuxiliaryProvisioner(connection, host)
	}
	c.Forget(sessionID)
	return connection, nil
}

func (c *Connector) installAuxiliaryProvisioner(connection SessionConnection, host model.Host) {
	if c == nil || connection == nil || c.credentials == nil || c.hosts == nil {
		return
	}
	installer, ok := connection.(interface {
		SetAuxiliaryProvisioner(sshclient.ConnectionProvisioner)
	})
	if !ok {
		return
	}
	if host.CredentialID == "" && !host.PasswordConfigured {

		return
	}

	tunnelConnector := c.auxiliaryTunnelConnector(connection)
	if tunnelConnector == nil {
		return
	}
	authenticator := sshclient.NewAuxiliaryAuthenticatorFor(tunnelConnector, sshclient.Target{
		Address:  host.Address,
		Port:     host.Port,
		Username: host.Username,

		Timeout: hostConnectTimeout(host),
	})
	hostID := host.ID
	credentialID := host.CredentialID
	installer.SetAuxiliaryProvisioner(func(dialCtx context.Context) (sshclient.AuxiliaryConnection, error) {
		if dialCtx == nil {
			return sshclient.AuxiliaryConnection{}, errno.New(errno.CodeInvalidArg, "session.dialer_not_ready", "")
		}
		var connection sshclient.AuxiliaryConnection
		useSecret := func(kind model.CredentialKind, secret []byte) error {
			auth, authErr := auxiliaryAuthMethod(kind, secret)
			if authErr != nil {
				return authErr
			}
			var connectErr error
			connection, connectErr = authenticator(dialCtx, sshclient.AuxiliaryAuth{Auth: auth})
			return connectErr
		}
		var err error
		if credentialID != "" {
			err = c.credentials.UseCredentialSecret(dialCtx, credentialID, useSecret)
		} else {
			err = c.hosts.UseHostPassword(dialCtx, hostID, func(secret []byte) error {
				return useSecret(model.CredentialKindPassword, secret)
			})
		}
		if err != nil {
			return sshclient.AuxiliaryConnection{}, err
		}
		return connection, nil
	})
}

func (c *Connector) auxiliaryTunnelConnector(connection SessionConnection) *sshclient.Connector {
	if c == nil || c.verifier == nil {
		return nil
	}
	dial := sshclient.DialerFunc(nil)
	if provider, ok := connection.(interface {
		AuxiliaryDialer() sshclient.DialerFunc
	}); ok {
		dial = provider.AuxiliaryDialer()
	}
	if dial == nil {
		return c.tunnelConnectorForSession()
	}
	return sshclient.NewConnectorWithDialer(c.verifier.Callback(), dial)
}

func (c *Connector) tunnelConnectorForSession() *sshclient.Connector {
	if c == nil || c.newDialer == nil || c.verifier == nil {
		return nil
	}
	production, ok := c.newDialer(c.verifier.Callback()).(*productionSessionDialer)
	if !ok {
		return nil
	}
	production.sessionConnector = c
	return production.connector
}

func auxiliaryAuthMethod(kind model.CredentialKind, secret []byte) (xssh.AuthMethod, error) {
	switch kind {
	case model.CredentialKindPassword:
		if len(secret) == 0 {
			return nil, errno.New(errno.CodeInvalidArg, "session.invalid_authentication", "")
		}
		return xssh.Password(string(secret)), nil
	case model.CredentialKindPrivateKey, model.CredentialKindPrivateKeyPassphrase:
		privateKey, passphrase, decodeErr := credential.DecodePrivateKeySecret(secret)
		if decodeErr != nil {
			return nil, errno.New(errno.CodeInvalidArg, "session.credential_unsupported", "")
		}
		defer clear(privateKey)
		defer clear(passphrase)
		var signer xssh.Signer
		var err error
		if len(passphrase) == 0 {
			signer, err = xssh.ParsePrivateKey(privateKey)
		} else {
			signer, err = xssh.ParsePrivateKeyWithPassphrase(privateKey, passphrase)
		}
		if err != nil {
			return nil, errno.New(errno.CodeInvalidArg, "session.credential_unsupported", "")
		}
		return xssh.PublicKeys(signer), nil
	default:
		return nil, errno.New(errno.CodeInvalidArg, "session.credential_unsupported", "")
	}
}

const (
	authenticationKindCredential          = "credential"
	authenticationKindKeyboardInteractive = "keyboard-interactive"
)

func (c *Connector) removePendingAuthentication(sessionID string, request *authenticationRequest) {
	if c == nil || request == nil {
		return
	}
	c.mu.Lock()
	if current, ok := c.pendingAuth[sessionID]; ok && current == request {
		delete(c.pendingAuth, sessionID)
	}
	c.mu.Unlock()
}

func (c *Connector) keyboardInteractiveAuth(
	ctx context.Context,
	auth routeAuthContext,
	host model.Host,
	hopIndex int,
	reusablePassword *interactivePassword,
) xssh.AuthMethod {
	return xssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {

		if len(questions) == 0 && strings.TrimSpace(name) == "" && strings.TrimSpace(instruction) == "" {
			return []string{}, nil
		}

		answers := make([]string, len(questions))
		if reusablePassword != nil {
			if index := singlePasswordQuestionIndex(questions, echos); index >= 0 {
				if password := reusablePassword.take(); password != "" {
					answers[index] = password
				}
			}
		}
		visibleIndexes := make([]int, 0, len(questions))
		for index := range questions {
			if answers[index] == "" {
				visibleIndexes = append(visibleIndexes, index)
			}
		}

		if len(visibleIndexes) == 0 {
			return answers, nil
		}
		waitCtx := ctx
		if waitCtx == nil {
			waitCtx = context.Background()
		}

		interactiveWait := sshclient.InteractiveWaitFromContext(waitCtx)
		if interactiveWait != nil {
			if !interactiveWait.Begin() {
				return nil, context.DeadlineExceeded
			}
			defer interactiveWait.End()
		}
		promptInstruction := strings.TrimSpace(instruction)
		if trimmedName := strings.TrimSpace(name); trimmedName != "" {
			if promptInstruction == "" {
				promptInstruction = trimmedName
			} else {
				promptInstruction = trimmedName + "\n" + promptInstruction
			}
		}
		challengeQuestions := make([]AuthenticationQuestion, len(visibleIndexes))
		for slot, index := range visibleIndexes {
			challengeQuestions[slot] = AuthenticationQuestion{
				Prompt: questions[index],
				Echo:   index < len(echos) && echos[index],
			}
		}
		request := &authenticationRequest{
			challenge: AuthenticationChallenge{
				SessionID:   auth.sessionID,
				Address:     host.Address,
				Username:    host.Username,
				RequestID:   newAuthenticationRequestID(auth.sessionID),
				Kind:        authenticationKindKeyboardInteractive,
				HostID:      host.ID,
				Instruction: promptInstruction,
				Questions:   challengeQuestions,
				HopIndex:    hopIndex,
				HopHostID: func() string {
					if hopIndex > 0 {
						return host.ID
					}
					return ""
				}(),
			},
			prefilled:      answers,
			visibleIndexes: visibleIndexes,
			input:          make(chan AuthenticationInput, 1),
		}
		c.mu.Lock()
		c.pendingAuth[auth.sessionID] = request
		c.mu.Unlock()
		defer c.removePendingAuthentication(auth.sessionID, request)
		if auth.reportState != nil {
			if err := auth.reportState(model.SessionStateAwaitingAuth); err != nil &&
				!errors.Is(err, model.ErrInvalidSessionTransition) {
				return nil, err
			}
		}
		var input AuthenticationInput
		select {
		case input = <-request.input:
		case <-waitCtx.Done():
			return nil, waitCtx.Err()
		}

		merged := append([]string(nil), request.prefilled...)
		for slot, index := range request.visibleIndexes {
			if slot < len(input.Answers) {
				merged[index] = input.Answers[slot]
			}
		}
		for index := range input.Answers {
			input.Answers[index] = ""
		}
		if auth.reportState != nil {

			if err := auth.reportState(model.SessionStateAuthenticating); err != nil &&
				!errors.Is(err, model.ErrInvalidSessionTransition) {
				return nil, err
			}
		}
		return merged, nil
	})
}

func (c *Connector) AuthenticationChallenge(sessionID string) (AuthenticationChallenge, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	request, ok := c.pendingAuth[sessionID]
	if !ok || request.submitted {
		return AuthenticationChallenge{}, false
	}
	return cloneAuthenticationChallenge(request.challenge), true
}

func cloneAuthenticationChallenge(challenge AuthenticationChallenge) AuthenticationChallenge {
	challenge.Questions = append([]AuthenticationQuestion(nil), challenge.Questions...)
	return challenge
}

func (c *Connector) SubmitAuthentication(sessionID string, input AuthenticationInput) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	request, ok := c.pendingAuth[sessionID]
	if !ok || request.submitted {
		return errno.New(errno.CodeInvalidArg, "session.invalid_authentication", "")
	}
	if request.challenge.Kind == authenticationKindKeyboardInteractive {
		if input.RequestID == "" || input.RequestID != request.challenge.RequestID || input.CredentialID != "" || input.Secret != "" || input.Remember || len(input.Answers) != len(request.challenge.Questions) {
			return errno.New(errno.CodeInvalidArg, "session.invalid_authentication", "")
		}
		input.Answers = append([]string(nil), input.Answers...)
	} else {
		if input.RequestID != "" && input.RequestID != request.challenge.RequestID {
			return errno.New(errno.CodeInvalidArg, "session.invalid_authentication", "")
		}
		if (input.CredentialID == "" && input.Secret == "") || (input.CredentialID != "" && input.Secret != "") || len(input.Answers) > 0 {
			return errno.New(errno.CodeInvalidArg, "session.invalid_authentication", "")
		}
	}
	request.submitted = true
	request.input <- input
	return nil
}

func (c *Connector) TestConnection(
	ctx context.Context,
	target sshclient.Target,
	kind model.CredentialKind,
	secret []byte,
) error {
	return c.testConnectionOverRoute(ctx, model.Host{}, target, kind, secret)
}

func (c *Connector) TestConnectionForHost(
	ctx context.Context,
	host model.Host,
	target sshclient.Target,
	kind model.CredentialKind,
	secret []byte,
) error {
	return c.testConnectionOverRoute(ctx, host, target, kind, secret)
}

func (c *Connector) testConnectionOverRoute(
	ctx context.Context,
	host model.Host,
	target sshclient.Target,
	kind model.CredentialKind,
	secret []byte,
) error {
	if c == nil || c.verifier == nil || c.newDialer == nil {
		return errno.New(errno.CodeSessionClosed, "session.connection_not_ready", "")
	}
	dialer := c.newDialer(c.verifier.Callback())
	if dialer == nil {
		return errno.New(errno.CodeSessionClosed, "session.connection_not_ready", "")
	}
	route, err := c.resolveRoute(ctx, host)
	if err != nil {
		return err
	}

	defer route.release()
	connection, err := c.connectThroughRoute(ctx, dialer, route, target, kind, secret)
	if connection != nil {
		closeErr := connection.Close()
		if err == nil {
			err = closeErr
		}
	}
	return err
}

func (c *Connector) DetectRemoteIcon(
	ctx context.Context,
	target sshclient.Target,
	kind model.CredentialKind,
	secret []byte,
) (string, error) {
	return c.detectRemoteIconOverRoute(ctx, model.Host{}, target, kind, secret)
}

func (c *Connector) DetectRemoteIconForHost(
	ctx context.Context,
	host model.Host,
	target sshclient.Target,
	kind model.CredentialKind,
	secret []byte,
) (string, error) {
	return c.detectRemoteIconOverRoute(ctx, host, target, kind, secret)
}

func (c *Connector) detectRemoteIconOverRoute(
	ctx context.Context,
	host model.Host,
	target sshclient.Target,
	kind model.CredentialKind,
	secret []byte,
) (string, error) {
	if c == nil || c.verifier == nil || c.newDialer == nil {
		return "", errno.New(errno.CodeSessionClosed, "session.connection_not_ready", "")
	}
	dialer := c.newDialer(c.verifier.Callback())
	if dialer == nil {
		return "", errno.New(errno.CodeSessionClosed, "session.connection_not_ready", "")
	}
	route, err := c.resolveRoute(ctx, host)
	if err != nil {
		return "", err
	}
	defer route.release()
	connection, err := c.connectThroughRoute(ctx, dialer, route, target, kind, secret)
	if connection == nil {
		return "", err
	}

	detector, ok := connection.(remoteOSDetector)
	if !ok {
		_ = connection.Close()
		return "", errno.New(errno.CodeConnFailed, "session.remote_os_unsupported", "")
	}

	icon, probeErr := detector.DetectRemoteOS(ctx)
	closeErr := connection.Close()
	if probeErr != nil {
		return "", probeErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return icon, nil
}

func (c *Connector) DetectStoredRemoteIcon(
	ctx context.Context,
	credentialID string,
	target sshclient.Target,
) (string, error) {
	return c.detectStoredRemoteIconOverRoute(ctx, model.Host{}, credentialID, target)
}

func (c *Connector) DetectStoredRemoteIconForHost(
	ctx context.Context,
	host model.Host,
	credentialID string,
	target sshclient.Target,
) (string, error) {
	return c.detectStoredRemoteIconOverRoute(ctx, host, credentialID, target)
}

func (c *Connector) detectStoredRemoteIconOverRoute(
	ctx context.Context,
	host model.Host,
	credentialID string,
	target sshclient.Target,
) (string, error) {
	if c == nil || c.credentials == nil {
		return "", errno.New(errno.CodeSessionClosed, "session.connector_not_ready", "")
	}
	var icon string
	err := c.credentials.UseCredentialSecret(ctx, credentialID, func(kind model.CredentialKind, secret []byte) error {
		detected, detectErr := c.detectRemoteIconOverRoute(ctx, host, target, kind, secret)
		if detectErr != nil {
			return detectErr
		}
		icon = detected
		return nil
	})
	if err != nil {
		return "", err
	}
	return icon, nil
}

func (c *Connector) TestStoredConnection(
	ctx context.Context,
	credentialID string,
	target sshclient.Target,
) error {
	return c.testStoredConnectionOverRoute(ctx, model.Host{}, credentialID, target)
}

func (c *Connector) TestStoredConnectionForHost(
	ctx context.Context,
	host model.Host,
	credentialID string,
	target sshclient.Target,
) error {
	return c.testStoredConnectionOverRoute(ctx, host, credentialID, target)
}

func (c *Connector) testStoredConnectionOverRoute(
	ctx context.Context,
	host model.Host,
	credentialID string,
	target sshclient.Target,
) error {
	if c == nil || c.credentials == nil {
		return errno.New(errno.CodeSessionClosed, "session.connector_not_ready", "")
	}
	return c.credentials.UseCredentialSecret(ctx, credentialID, func(kind model.CredentialKind, secret []byte) error {
		return c.testConnectionOverRoute(ctx, host, target, kind, secret)
	})
}

type routeSessionDialer interface {
	ConnectRoute(
		ctx context.Context,
		route resolvedRoute,
		target sshclient.Target,
		kind model.CredentialKind,
		secret []byte,
	) (SessionConnection, error)
}

type routeInteractiveSessionDialer interface {
	ConnectRouteWithAuth(
		ctx context.Context,
		route resolvedRoute,
		target sshclient.Target,
		kind model.CredentialKind,
		secret []byte,
		extra []xssh.AuthMethod,
		interactive bool,
	) (SessionConnection, error)
}

func (c *Connector) probeRouteHostKeys(
	ctx context.Context,
	route resolvedRoute,
	target sshclient.Target,
) ([]*sshclient.HostKeyError, error) {
	verify := c.verifier.Verify
	probe := sshclient.NewConnector(verify)
	if route.proxy != nil {
		dial, dialErr := sshclient.NewProxyDialer(*route.proxy)
		if dialErr != nil {
			return nil, errno.New(errno.CodeInvalidArg, "session.proxy_invalid", "")
		}
		probe = sshclient.NewConnectorWithDialer(verify, dial)
	}
	challenges := make([]*sshclient.HostKeyError, 0, len(route.hops)+1)
	err := probe.ProbeHostKeys(ctx, route.hops, target, verify,
		func(hopIndex int, label, address string, key xssh.PublicKey, known bool) error {
			if known {
				return nil
			}
			challenge := model.HostKeyChallenge{
				Address:           address,
				HopIndex:          hopIndex,
				HopLabel:          label,
				Algorithm:         key.Type(),
				SHA256Fingerprint: xssh.FingerprintSHA256(key),
				Issue:             model.HostKeyIssueUnknown,
			}
			entry, entryErr := sshclient.NewHostKeyError(challenge, key)
			if entryErr != nil {
				return entryErr
			}
			challenges = append(challenges, entry)
			return nil
		})
	if err != nil {
		return nil, err
	}
	return challenges, nil
}

func (c *Connector) connectThroughRoute(
	ctx context.Context,
	dialer sessionDialer,
	route resolvedRoute,
	target sshclient.Target,
	kind model.CredentialKind,
	secret []byte,
) (SessionConnection, error) {
	return c.connectThroughRouteWithAuth(ctx, dialer, route, target, kind, secret, nil, false)
}

func (c *Connector) connectThroughRouteWithAuth(
	ctx context.Context,
	dialer sessionDialer,
	route resolvedRoute,
	target sshclient.Target,
	kind model.CredentialKind,
	secret []byte,
	extra []xssh.AuthMethod,
	interactive bool,
) (SessionConnection, error) {
	if len(extra) > 0 {
		if !route.routed() {
			interactiveDialer, ok := dialer.(interactiveSessionDialer)
			if !ok {

				return dialer.Connect(ctx, target, kind, secret)
			}
			return interactiveDialer.ConnectWithAuth(ctx, target, kind, secret, extra, interactive)
		}
		routed, ok := dialer.(routeInteractiveSessionDialer)
		if !ok {
			return c.connectThroughRoute(ctx, dialer, route, target, kind, secret)
		}
		return routed.ConnectRouteWithAuth(ctx, route, target, kind, secret, extra, interactive)
	}
	if !route.routed() {
		return dialer.Connect(ctx, target, kind, secret)
	}
	routed, ok := dialer.(routeSessionDialer)
	if !ok {

		return nil, errno.New(errno.CodeSessionClosed, "session.connector_not_ready", "")
	}
	return routed.ConnectRoute(ctx, route, target, kind, secret)
}

func (d *productionSessionDialer) ConnectRoute(
	ctx context.Context,
	route resolvedRoute,
	target sshclient.Target,
	kind model.CredentialKind,
	secret []byte,
) (SessionConnection, error) {
	return d.ConnectRouteWithAuth(ctx, route, target, kind, secret, nil, false)
}

func (d *productionSessionDialer) ConnectRouteWithAuth(
	ctx context.Context,
	route resolvedRoute,
	target sshclient.Target,
	kind model.CredentialKind,
	secret []byte,
	extra []xssh.AuthMethod,
	interactive bool,
) (SessionConnection, error) {
	if d == nil || d.connector == nil {
		return nil, errno.New(errno.CodeSessionClosed, "session.dialer_not_ready", "")
	}
	if ctx == nil {
		return nil, errno.New(errno.CodeInvalidArg, "session.dialer_not_ready", "")
	}
	auth, err := auxiliaryAuthMethod(kind, secret)
	if err != nil {
		return nil, err
	}
	connector := d.connector
	if route.proxy != nil {
		dial, dialErr := sshclient.NewProxyDialer(*route.proxy)
		if dialErr != nil {
			return nil, errno.New(errno.CodeInvalidArg, "session.proxy_invalid", "")
		}

		connector = sshclient.NewConnectorWithDialer(d.hostKey, dial)
	}

	state := d.hopState
	total := len(route.hops)
	authMethods := []xssh.AuthMethod{auth}
	authMethods = append(authMethods, extra...)
	connection, err := connector.ConnectChainAuthMethodsObserved(ctx, route.hops, target, authMethods, interactive, func(index int, label string) {
		state.set(index, label, index < total)
	})
	if err != nil {
		return nil, err
	}
	return &productionSessionConnection{connection: connection, route: &route}, nil
}

func (d *productionSessionDialer) Connect(
	ctx context.Context,
	target sshclient.Target,
	kind model.CredentialKind,
	secret []byte,
) (SessionConnection, error) {
	return d.ConnectWithAuth(ctx, target, kind, secret, nil, false)
}

func (d *productionSessionDialer) ConnectWithAuth(
	ctx context.Context,
	target sshclient.Target,
	kind model.CredentialKind,
	secret []byte,
	extra []xssh.AuthMethod,
	interactive bool,
) (SessionConnection, error) {
	if d == nil || d.connector == nil {
		return nil, errno.New(errno.CodeSessionClosed, "session.dialer_not_ready", "")
	}
	if kind == model.CredentialKindSSHAgent {
		connection, err := d.connector.ConnectSSHAgentAuthMethods(ctx, target, secret, extra, interactive)
		if err != nil {
			return nil, err
		}
		return &productionSessionConnection{connection: connection}, nil
	}
	auth, err := auxiliaryAuthMethod(kind, secret)
	if err != nil {
		return nil, err
	}
	authMethods := append([]xssh.AuthMethod{auth}, extra...)
	connection, err := d.connector.ConnectAuthMethods(ctx, target, authMethods, interactive)
	if err != nil {
		return nil, err
	}
	return &productionSessionConnection{connection: connection}, nil
}

func (c *productionSessionConnection) Wait() error {
	return c.connection.Wait()
}

func (c *productionSessionConnection) InterruptTerminalWrite() {
	if c != nil && c.connection != nil {
		c.connection.InterruptTerminalWrite()
	}
}

func (c *productionSessionConnection) Close() error {
	if c == nil {
		return nil
	}
	var err error
	if c.connection != nil {
		err = c.connection.Close()
	}

	if c.route != nil {
		c.route.release()
	}
	return err
}

func (c *productionSessionConnection) AuxiliaryDialer() sshclient.DialerFunc {
	if c == nil {
		return nil
	}
	if c.connection != nil {
		if dial := c.connection.ChainDialer(); dial != nil {
			return dial
		}
	}
	if c.route == nil || c.route.proxy == nil {
		return nil
	}
	dial, err := sshclient.NewProxyDialer(*c.route.proxy)
	if err != nil {

		return nil
	}
	return dial
}

func (c *productionSessionConnection) OpenTerminal(
	ctx context.Context,
	columns int,
	rows int,
) (SessionTerminal, error) {
	if c == nil || c.connection == nil {
		return nil, ErrSessionNotConnected
	}
	terminal, err := c.connection.OpenTerminalWithOptions(ctx, columns, rows, sshclient.TerminalOptions{

		DirectoryFollow: c.directoryFollowMode == "session" && !c.networkDevice,

		ShellIntegration:      !c.networkDevice,
		EnvironmentVariables:  c.environmentVariables,
		PostLoginCommand:      c.postLoginCommand,
		PostLoginCommandDelay: c.postLoginDelay,
		TerminalType:          c.terminalType,
		NetworkDevice:         c.networkDevice,
		Encoding:              c.terminalCharacterEncoding(),
		X11:                   c.x11,
	})
	if err != nil {
		return nil, err
	}

	if warning := terminal.X11Warning(); warning != "" {
		c.x11Warning = warning
	}
	return terminal, nil
}

func (c *productionSessionConnection) OpenFileSystem(
	ctx context.Context,
) (SessionFileSystem, error) {
	return c.connection.OpenFileSystem(ctx)
}

func (c *productionSessionConnection) OpenFileSystemForConnections(
	ctx context.Context,
	planned int,
) (SessionFileSystem, error) {
	return c.connection.OpenFileSystemForConnections(ctx, planned)
}

func (c *Connector) PendingHostKey(sessionID string) (model.HostKeyPending, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entries := c.pending[sessionID]
	challenges := make([]model.HostKeyChallenge, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		challenge := entry.Challenge
		challenge.TrustedFingerprints = append([]string(nil), challenge.TrustedFingerprints...)
		challenges = append(challenges, challenge)
	}
	if len(challenges) == 0 {
		return model.HostKeyPending{}, false
	}
	return model.HostKeyPending{
		SessionID:  sessionID,
		Challenge:  challenges[0],
		Challenges: challenges,
	}, true
}

func (c *Connector) Challenge(sessionID string) (model.HostKeyChallenge, bool) {
	pending, ok := c.PendingHostKey(sessionID)
	if !ok {
		return model.HostKeyChallenge{}, false
	}
	return pending.Challenge, true
}

func (c *Connector) Failure(sessionID string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	message, ok := c.failures[sessionID]
	return message, ok
}

func (c *Connector) TrustUnknown(sessionID string) error {
	c.mu.RLock()
	pending := append([]*sshclient.HostKeyError(nil), c.pending[sessionID]...)
	c.mu.RUnlock()
	if len(pending) == 0 {
		return errno.New(errno.CodeNotFound, "session.no_pending_key", "")
	}
	if err := c.verifier.TrustUnknownBatch(pending); err != nil {
		return err
	}
	c.Forget(sessionID)
	return nil
}

func (c *Connector) remember(sessionID string, pending *sshclient.HostKeyError) {
	if pending == nil {
		return
	}
	c.rememberAll(sessionID, []*sshclient.HostKeyError{pending})
}

func (c *Connector) rememberAll(sessionID string, pending []*sshclient.HostKeyError) {
	if len(pending) == 0 {
		return
	}
	copied := append([]*sshclient.HostKeyError(nil), pending...)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending[sessionID] = copied
}

func (c *Connector) rememberFailure(sessionID string, message string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures[sessionID] = message
}

func (c *Connector) Forget(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.pending, sessionID)
	delete(c.pendingAuth, sessionID)
	delete(c.failures, sessionID)
}

func SessionConnectionFailureKey(err error) string {

	var connectErr *sshclient.ConnectError
	if errors.As(err, &connectErr) && connectErr.Stage == sshclient.StageTCP {
		return "session.host_unreachable"
	}

	switch {
	case errors.Is(err, credential.ErrCredentialVaultLocked):
		return "credential.locked"
	case errors.Is(err, context.DeadlineExceeded):
		return "session.timeout"
	case errors.Is(err, sshclient.ErrHostKeyChanged):
		return "session.hostkey_changed"
	}

	if errors.As(err, &connectErr) {
		switch connectErr.Stage {
		case sshclient.StageValidation:
			return "session.connection_invalid"
		case sshclient.StageTCP:
			return "session.network_failed"
		case sshclient.StageProxy:

			switch {
			case errors.Is(err, sshclient.ErrProxyAuthFailed):
				return "session.proxy_auth_failed"
			case errors.Is(err, sshclient.ErrProxyInvalid):
				return "session.proxy_invalid"
			default:

				return "session.proxy_unreachable"
			}
		case sshclient.StageJump:
			if isSSHAuthenticationError(err) {
				return "session.jump_auth_failed"
			}
			return "session.jump_unreachable"
		case sshclient.StageJumpDial:

			if sshclient.IsForwardDenied(err) {
				return "session.jump_forward_denied"
			}
			return "session.jump_unreachable"
		case sshclient.StageHandshake:

			if isSSHAuthenticationError(err) {
				return "session.auth_failed"
			}
			return "session.handshake_failed"
		}
	}
	return "session.connection_failed"
}

func isSSHAuthenticationError(err error) bool {
	if err == nil {
		return false
	}

	for current := err; current != nil; current = errors.Unwrap(current) {
		message := strings.ToLower(current.Error())
		if strings.Contains(message, "unable to authenticate") ||
			strings.Contains(message, "no supported methods remain") {
			return true
		}
	}
	return false
}

func sameHostPort(hostname, host string, port int) bool {
	normalized := strings.ToLower(strings.TrimSpace(hostname))
	want := strings.ToLower(strings.TrimSpace(host))
	if normalized == "" || want == "" {
		return false
	}

	if _, _, err := net.SplitHostPort(want); err == nil {
		return normalized == want
	}
	if port <= 0 {
		return stripPort(normalized) == want
	}
	return normalized == net.JoinHostPort(want, strconv.Itoa(port))
}

func stripPort(address string) string {
	if host, _, err := net.SplitHostPort(address); err == nil {
		return host
	}
	return address
}
