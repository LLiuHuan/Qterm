package app

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"

	"changeme/internal/credential"
	"changeme/internal/errno"
	"changeme/internal/host"
	"changeme/internal/model"
	sshclient "changeme/internal/tunnel"
	"changeme/internal/vnc"
)

func vncError(code errno.Code, messageKey string, err error) error {
	if err == nil {
		return errno.New(code, messageKey, "")
	}
	return errno.Wrap(code, messageKey, err)
}

func classifyVncOpenError(err error) error {
	switch {
	case errors.Is(err, vnc.ErrInvalidTarget):
		return vncError(errno.CodeInvalidArg, "vnc.invalidTarget", err)

	case errors.Is(err, sshclient.ErrHostKeyUnknown), errors.Is(err, sshclient.ErrHostKeyChanged):
		return vncError(errno.CodeConnFailed, "vnc.hostKeyRequired", err)
	case errors.Is(err, vnc.ErrTLSCertificate):

		return vncError(errno.CodeConnFailed, "vnc.tlsCertificate", err)
	case errors.Is(err, vnc.ErrBasicAuthRejected):

		return vncError(errno.CodePermissionDenied, "vnc.basicAuthRejected", err)
	case errors.Is(err, vnc.ErrHandshakeRejected):
		return vncError(errno.CodeConnFailed, "vnc.handshakeRejected", err)
	case errors.Is(err, vnc.ErrDialFailed):
		return vncError(errno.CodeConnFailed, "vnc.unreachable", err)
	default:
		return vncError(errno.CodeUnknown, "vnc.openFailed", err)
	}
}

func (s *Service) vncManager() (*vnc.Manager, error) {
	if s == nil || s.vncSessions == nil {
		return nil, vncError(errno.CodeConnFailed, "vnc.serviceNotReady", nil)
	}
	return s.vncSessions, nil
}

func (s *Service) resolveVncPassword(ctx context.Context, record model.Host, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if record.CredentialID != "" {
		var secret string
		err := credential.UseCredentialSecret(s.credentials, ctx, record.CredentialID,
			func(kind model.CredentialKind, value []byte) error {
				if kind != model.CredentialKindPassword {
					return vncError(errno.CodeInvalidArg, "vnc.credentialNotPassword", nil)
				}
				secret = string(value)
				return nil
			})
		if err != nil {
			return "", err
		}
		return secret, nil
	}
	var secret string
	if err := s.hosts.UseHostPassword(ctx, record.ID, func(value []byte) error {
		secret = string(value)
		return nil
	}); err != nil {

		return "", nil
	}
	return secret, nil
}

func (s *Service) vncHostProfile(ctx context.Context, hostID string) (model.VncHostProfile, error) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return model.VncHostProfile{}, vncError(errno.CodeInvalidArg, "vnc.hostRequired", nil)
	}
	record, err := host.SessionHost(s.hosts, ctx, hostID)
	if err != nil {
		return model.VncHostProfile{}, err
	}
	if record.Protocol != model.ProtocolVNC {
		return model.VncHostProfile{}, vncError(errno.CodeInvalidArg, "vnc.hostNotVnc", nil)
	}
	port := record.Port
	if port <= 0 {
		port = 5900
	}

	transport := normalizedVncTransport(record.VncTransport)

	rfbPasswordMode := normalizedVncRfbPasswordMode(record.VncRfbPasswordMode)

	wsPath, err := model.NormalizeVncWsPath(record.VncWsPath)
	if err != nil {
		return model.VncHostProfile{}, vncError(errno.CodeInvalidArg, "vnc.invalidWsPath", err)
	}
	return model.VncHostProfile{
		HostID:            record.ID,
		Username:          strings.TrimSpace(record.Username),
		Name:              record.Name,
		Address:           record.Address,
		Port:              port,
		PasswordAvailable: record.PasswordConfigured || record.CredentialID != "",
		PasswordSource:    record.PasswordSource,
		ViewOnly:          record.VncViewOnly,
		Shared:            record.VncShared,
		PixelDepth:        normalizedVncPixelDepth(record.VncPixelDepth),
		Transport:         transport,
		WsPath:            wsPath,
		AllowInsecureTLS:  record.VncAllowInsecureTLS,

		BasicAuthUsed:       transport == model.VncTransportWSS,
		RfbPasswordMode:     rfbPasswordMode,
		RfbPasswordRequired: transport != model.VncTransportWSS || rfbPasswordMode == model.VncRfbPasswordSame,
	}, nil
}

func (s *Service) VncHostProfile(ctx context.Context, hostID string) (model.VncHostProfile, error) {
	if s == nil || s.hosts == nil {
		return model.VncHostProfile{}, vncError(errno.CodeConnFailed, "vnc.serviceNotReady", nil)
	}
	return publicValue(s.vncHostProfile(ctx, hostID))
}

func (s *Service) VncOpenHost(ctx context.Context, hostID string) (model.VncOpenResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	manager, err := s.vncManager()
	if err != nil {
		return model.VncOpenResult{}, publicError(err)
	}
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return model.VncOpenResult{}, publicError(vncError(errno.CodeInvalidArg, "vnc.hostRequired", nil))
	}
	record, err := host.SessionHost(s.hosts, ctx, hostID)
	if err != nil {
		return model.VncOpenResult{}, publicError(err)
	}
	if record.Protocol != model.ProtocolVNC {
		return model.VncOpenResult{}, publicError(vncError(errno.CodeInvalidArg, "vnc.hostNotVnc", nil))
	}
	secret, err := s.resolveVncPassword(ctx, record, "")
	if err != nil {
		return model.VncOpenResult{}, publicError(err)
	}
	port := record.Port
	if port <= 0 {
		port = 5900
	}
	transport := normalizedVncTransport(record.VncTransport)

	rfbPasswordMode := normalizedVncRfbPasswordMode(record.VncRfbPasswordMode)
	wsPath, err := model.NormalizeVncWsPath(record.VncWsPath)
	if err != nil {
		return model.VncOpenResult{}, publicError(vncError(errno.CodeInvalidArg, "vnc.invalidWsPath", err))
	}
	target := vnc.Target{
		HostID:           record.ID,
		Name:             record.Name,
		Address:          record.Address,
		Port:             port,
		ViewOnly:         record.VncViewOnly,
		Shared:           record.VncShared,
		PixelDepth:       normalizedVncPixelDepth(record.VncPixelDepth),
		Transport:        transport,
		WsPath:           wsPath,
		AllowInsecureTLS: record.VncAllowInsecureTLS,
	}
	if transport == model.VncTransportWSS {

		target.BasicUser = strings.TrimSpace(record.Username)
		target.BasicPassword = secret
	}

	var routedConn *vncRoutedConn
	if transport == model.VncTransportTCP && vncHostHasRoute(record) {
		if s.sessions == nil && s.vncDialRoute == nil {
			secret = ""
			return model.VncOpenResult{}, publicError(vncError(errno.CodeConnFailed, "vnc.serviceNotReady", nil))
		}
		dialRoute := s.vncDialRoute
		if dialRoute == nil {
			dialRoute = s.sessions.DialRoute
		}
		endpoint := net.JoinHostPort(strings.TrimSpace(record.Address), strconv.Itoa(port))
		conn, closeChain, dialErr := dialRoute(ctx, record.ID, endpoint)
		if dialErr != nil {

			secret = ""
			return model.VncOpenResult{}, publicError(classifyVncOpenError(dialErr))
		}
		routedConn = &vncRoutedConn{Conn: conn, closeChain: closeChain}
	}

	var session *vnc.Session
	if routedConn != nil {

		session, err = manager.OpenWithDialer(ctx, target, &vncOneShotDialer{conn: routedConn})
	} else {

		session, err = manager.Open(ctx, target)
	}

	target.BasicPassword = ""
	if err != nil {
		secret = ""

		if routedConn != nil {
			_ = routedConn.Close()
		}
		return model.VncOpenResult{}, publicError(classifyVncOpenError(err))
	}

	s.applyDefaultVncHostIcon(ctx, record)
	ticket, err := manager.IssueTicket(session.ID())
	if err != nil {

		_ = manager.Close(session.ID())
		secret = ""
		return model.VncOpenResult{}, publicError(vncError(errno.CodeUnknown, "vnc.streamUnavailable", err))
	}
	snapshot := session.Snapshot()
	result := model.VncOpenResult{
		Session:               snapshot,
		Ticket:                ticket,
		Transport:             transport,
		AllowInsecureTLS:      record.VncAllowInsecureTLS,
		BasicAuthUsed:         transport == model.VncTransportWSS,
		RfbPasswordMode:       rfbPasswordMode,
		ServerCertFingerprint: snapshot.ServerCertFingerprint,

		ServerSoftware: snapshot.ServerSoftware,
		ServerVersion:  snapshot.ServerVersion,
		ServerBanner:   snapshot.ServerBanner,
		IdentitySource: snapshot.IdentitySource,
	}

	if transport != model.VncTransportWSS || rfbPasswordMode == model.VncRfbPasswordSame {
		result.Username = strings.TrimSpace(record.Username)
		result.Password = secret
	}

	secret = ""
	return result, nil
}

const vncDefaultHostIcon = "no-vnc"

func (s *Service) applyDefaultVncHostIcon(ctx context.Context, record model.Host) {
	if s == nil || s.hosts == nil {
		return
	}
	if strings.TrimSpace(record.Icon) != "" {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	_ = host.SetDetectedHostIcon(s.hosts, ctx, record.ID, vncDefaultHostIcon)
}

func (s *Service) VncCloseSession(sessionID string) error {
	manager, err := s.vncManager()
	if err != nil {
		return publicError(err)
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return publicError(vncError(errno.CodeInvalidArg, "vnc.sessionRequired", nil))
	}
	if err := manager.Close(sessionID); err != nil {
		return publicError(vncError(errno.CodeUnknown, "vnc.closeFailed", err))
	}
	return nil
}

func (s *Service) VncListSessions() ([]model.VncSession, error) {
	manager, err := s.vncManager()
	if err != nil {
		return []model.VncSession{}, publicError(err)
	}
	return manager.List(), nil
}

func normalizedVncPixelDepth(value int) int {
	if value == 16 {
		return 16
	}
	return 32
}

func normalizedVncTransport(value string) string {
	if value == model.VncTransportWSS {
		return model.VncTransportWSS
	}
	return model.VncTransportTCP
}

func normalizedVncRfbPasswordMode(value string) string {
	if value == model.VncRfbPasswordSame {
		return model.VncRfbPasswordSame
	}
	return model.VncRfbPasswordAuto
}

func vncHostHasRoute(record model.Host) bool {
	if record.JumpHostID != nil && strings.TrimSpace(*record.JumpHostID) != "" {
		return true
	}
	if record.ProxyMode != nil {
		mode := strings.TrimSpace(*record.ProxyMode)
		return mode != "" && mode != model.ProxyModeNone
	}
	return false
}

type vncOneShotDialer struct {
	mu   sync.Mutex
	conn *vncRoutedConn
}

func (d *vncOneShotDialer) DialContext(context.Context, string, string) (net.Conn, error) {
	if d == nil {
		return nil, errors.New("vnc routed dialer is not configured")
	}
	d.mu.Lock()
	conn := d.conn
	d.conn = nil
	d.mu.Unlock()
	if conn == nil {
		return nil, errors.New("vnc routed dialer has already been used")
	}
	return conn, nil
}

type vncRoutedConn struct {
	net.Conn
	closeChain func() error
	once       sync.Once
	releaseErr error
}

func (c *vncRoutedConn) Close() error {
	if c == nil {
		return nil
	}
	var closeErr error
	if c.Conn != nil {
		closeErr = c.Conn.Close()
	}
	c.once.Do(func() {
		if c.closeChain != nil {
			c.releaseErr = c.closeChain()
		}
	})
	if closeErr != nil {
		return closeErr
	}
	return c.releaseErr
}
