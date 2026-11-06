package vnc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"changeme/internal/model"
	"changeme/internal/terminal"
)

const StreamName = "vnc"

const ProtocolVersion = int(vncProtocolVersion)

const defaultDialTimeout = 8 * time.Second

var (
	ErrSessionNotFound = errors.New("vnc session not found")

	ErrSessionBusy = errors.New("vnc session already claimed")

	ErrInvalidTarget = errors.New("invalid vnc target")

	ErrDialFailed = errors.New("vnc dial failed")

	ErrTokenUnavailable = errors.New("vnc stream token unavailable")

	ErrTLSCertificate = errors.New("vnc tls certificate rejected")

	ErrBasicAuthRejected = errors.New("vnc basic auth rejected")

	ErrHandshakeRejected = errors.New("vnc websocket handshake rejected")
)

type TokenVerifier interface {
	Consume(sessionID, token string) bool
}

type Stream interface {
	Context() context.Context
	Send([]byte) error
	Receive() ([]byte, error)
	Close() error
}

type SessionResolver interface {
	StreamSession(id string) (*Session, bool)
}

type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

type Target struct {
	HostID  string
	Name    string
	Address string
	Port    int

	ViewOnly   bool
	Shared     bool
	PixelDepth int

	Transport string

	WsPath string

	AllowInsecureTLS bool

	BasicUser     string
	BasicPassword string
}

type Session struct {
	id               string
	hostID           string
	name             string
	address          string
	port             int
	viewOnly         bool
	shared           bool
	pixelDepth       int
	transport        string
	allowInsecureTLS bool

	certFingerprint string

	identity  ServerIdentity
	createdAt time.Time

	conn net.Conn

	mu      sync.Mutex
	claimed bool
	closed  bool
	lastErr string
}

func (s *Session) ID() string { return s.id }

func (s *Session) Target() Target {
	return Target{
		HostID:           s.hostID,
		Name:             s.name,
		Address:          s.address,
		Port:             s.port,
		ViewOnly:         s.viewOnly,
		Shared:           s.shared,
		PixelDepth:       s.pixelDepth,
		Transport:        s.transport,
		AllowInsecureTLS: s.allowInsecureTLS,
	}
}

func (s *Session) Claim() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.claimed {
		return false
	}
	s.claimed = true
	return true
}

func (s *Session) Write(payload []byte) error {
	s.mu.Lock()
	conn := s.conn
	closed := s.closed
	s.mu.Unlock()
	if closed || conn == nil {
		return ErrSessionNotFound
	}
	_, err := conn.Write(payload)
	return err
}

func (s *Session) Read(buffer []byte) (int, error) {
	s.mu.Lock()
	conn := s.conn
	closed := s.closed
	s.mu.Unlock()
	if closed || conn == nil {
		return 0, ErrSessionNotFound
	}
	return conn.Read(buffer)
}

func (s *Session) Snapshot() model.VncSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	return model.VncSession{
		ID:         s.id,
		HostID:     s.hostID,
		Name:       s.name,
		Address:    s.address,
		Port:       s.port,
		ViewOnly:   s.viewOnly,
		Shared:     s.shared,
		PixelDepth: s.pixelDepth,

		Transport:             s.transport,
		AllowInsecureTLS:      s.allowInsecureTLS,
		ServerCertFingerprint: s.certFingerprint,

		ServerSoftware: s.identity.Software,
		ServerVersion:  s.identity.Version,
		ServerBanner:   s.identity.Banner,
		IdentitySource: s.identity.Source,
		Connected:      !s.closed && s.conn != nil,
		Error:          s.lastErr,
	}
}

func (s *Session) Close(reason string) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	if reason != "" && s.lastErr == "" {
		s.lastErr = reason
	}
	conn := s.conn
	s.conn = nil
	s.mu.Unlock()
	if conn == nil {
		return nil
	}
	return conn.Close()
}

type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	sequence uint64
	tokens   *terminal.TokenStore
	dialer   Dialer

	wsDialer WebSocketDialer

	now func() time.Time

	dialTimeout time.Duration
}

func NewManager() *Manager {
	return &Manager{
		sessions:    make(map[string]*Session),
		tokens:      terminal.NewTokenStore(),
		dialer:      &net.Dialer{},
		wsDialer:    coderWebSocketDialer{},
		now:         time.Now,
		dialTimeout: defaultDialTimeout,
	}
}

func (m *Manager) SetDialer(dialer Dialer) {
	if m == nil || dialer == nil {
		return
	}
	m.mu.Lock()
	m.dialer = dialer
	m.mu.Unlock()
}

func (m *Manager) SetWebSocketDialer(dialer WebSocketDialer) {
	if m == nil || dialer == nil {
		return
	}
	m.mu.Lock()
	m.wsDialer = dialer
	m.mu.Unlock()
}

func (m *Manager) Open(ctx context.Context, target Target) (*Session, error) {
	return m.OpenWithDialer(ctx, target, nil)
}

func (m *Manager) OpenWithDialer(ctx context.Context, target Target, dialer Dialer) (*Session, error) {
	if m == nil {
		return nil, ErrSessionNotFound
	}
	address := strings.TrimSpace(target.Address)
	if address == "" || target.Port <= 0 || target.Port > 65535 {
		return nil, fmt.Errorf("%w: %q:%d", ErrInvalidTarget, target.Address, target.Port)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	m.mu.Lock()

	if dialer == nil {
		dialer = m.dialer
	}
	wsDialer := m.wsDialer
	timeout := m.dialTimeout
	m.mu.Unlock()
	if dialer == nil {
		dialer = &net.Dialer{}
	}
	if wsDialer == nil {
		wsDialer = coderWebSocketDialer{}
	}

	dialCtx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		dialCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	endpoint := net.JoinHostPort(address, fmt.Sprintf("%d", target.Port))

	transport := normalizedTransport(target.Transport)
	var (
		conn            net.Conn
		certFingerprint string
		headerIdentity  ServerIdentity
		err             error
	)
	if transport == model.VncTransportWSS {

		var handshake WebSocketHandshake
		conn, certFingerprint, handshake, err = wsDialer.DialWebSocket(dialCtx, WebSocketDialRequest{
			Endpoint:         endpoint,
			Path:             target.WsPath,
			AllowInsecureTLS: target.AllowInsecureTLS,
			BasicUser:        target.BasicUser,
			BasicPassword:    target.BasicPassword,
		})
		if err != nil {

			return nil, err
		}

		headerIdentity = ServerIdentity{
			Software: softwareFromHandshake(handshake),
			Version:  versionFromHandshake(handshake),
			Source:   "header",
		}
		if headerIdentity.Software == "" {
			headerIdentity = ServerIdentity{}
		}
	} else {
		conn, err = dialer.DialContext(dialCtx, "tcp", endpoint)
		if err != nil {

			return nil, fmt.Errorf("%w: %w", ErrDialFailed, err)
		}

		var bannerIdentity ServerIdentity
		conn, bannerIdentity = PeekServerBanner(conn)
		headerIdentity = bannerIdentity

	}

	session := &Session{
		hostID:           target.HostID,
		name:             target.Name,
		address:          address,
		port:             target.Port,
		viewOnly:         target.ViewOnly,
		shared:           target.Shared,
		pixelDepth:       normalizedPixelDepth(target.PixelDepth),
		transport:        transport,
		allowInsecureTLS: target.AllowInsecureTLS,
		certFingerprint:  certFingerprint,
		identity:         headerIdentity,
		conn:             conn,
	}

	m.mu.Lock()
	m.sequence++
	session.id = fmt.Sprintf("vnc-%d", m.sequence)
	session.createdAt = m.now()
	m.sessions[session.id] = session
	m.mu.Unlock()
	return session, nil
}

func (m *Manager) StreamSession(id string) (*Session, bool) {
	if m == nil {
		return nil, false
	}
	m.mu.Lock()
	session, found := m.sessions[id]
	m.mu.Unlock()
	if !found {
		return nil, false
	}
	return session, true
}

func (m *Manager) Get(id string) (model.VncSession, bool) {
	session, found := m.StreamSession(id)
	if !found {
		return model.VncSession{}, false
	}
	return session.Snapshot(), true
}

func (m *Manager) List() []model.VncSession {
	if m == nil {
		return []model.VncSession{}
	}
	m.mu.Lock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	m.mu.Unlock()

	result := make([]model.VncSession, 0, len(sessions))
	for _, session := range sessions {
		result = append(result, session.Snapshot())
	}
	return result
}

func (m *Manager) IssueTicket(id string) (model.VncStreamTicket, error) {
	if m == nil || m.tokens == nil {
		return model.VncStreamTicket{}, ErrTokenUnavailable
	}
	if _, found := m.StreamSession(id); !found {
		return model.VncStreamTicket{}, ErrSessionNotFound
	}
	token, err := m.tokens.Issue(id)
	if err != nil {
		return model.VncStreamTicket{}, fmt.Errorf("%w: %v", ErrTokenUnavailable, err)
	}
	return model.VncStreamTicket{
		StreamName:      StreamName,
		Token:           token,
		ProtocolVersion: ProtocolVersion,
	}, nil
}

func (m *Manager) ConsumeToken(sessionID, token string) bool {
	if m == nil || m.tokens == nil {
		return false
	}
	return m.tokens.Consume(sessionID, token)
}

func (m *Manager) Close(id string) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	session, found := m.sessions[id]
	if found {
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	if !found {
		return nil
	}
	if m.tokens != nil {
		m.tokens.Revoke(id)
	}
	return session.Close("会话已关闭")
}

func (m *Manager) CloseAll() {
	if m == nil {
		return
	}
	m.mu.Lock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	m.sessions = make(map[string]*Session)
	m.mu.Unlock()
	for _, session := range sessions {
		_ = session.Close("应用正在退出")
	}
	if m.tokens != nil {
		m.tokens.Clear()
	}
}

func normalizedPixelDepth(value int) int {
	if value == 16 {
		return 16
	}
	return 32
}

func normalizedTransport(value string) string {
	if value == model.VncTransportWSS {
		return model.VncTransportWSS
	}
	return model.VncTransportTCP
}
