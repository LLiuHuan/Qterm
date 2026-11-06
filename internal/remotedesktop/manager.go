package remotedesktop

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrHostUnreachable = errors.New("远程桌面主机不可达")

const hostProbeTimeout = 3 * time.Second

const (
	hostProbeTimeoutMin = 1 * time.Second
	hostProbeTimeoutMax = 30 * time.Second
)

func NormalizeHostProbeTimeout(value time.Duration) time.Duration {
	if value < hostProbeTimeoutMin || value > hostProbeTimeoutMax {
		return hostProbeTimeout
	}
	return value
}

func probeHostReachable(ctx context.Context, address string, port int, timeout time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		timeout = hostProbeTimeout
	}
	host := strings.TrimSpace(address)
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("%w：%s:%d（%v）", ErrHostUnreachable, host, port, err)
	}
	_ = conn.Close()
	return nil
}

const (
	defaultWidth  = 1280
	defaultHeight = 800

	renderSlice = 500 * time.Millisecond

	clipboardSequencePollInterval = 25 * time.Millisecond
)

type protocolSession interface {
	Render(ctx context.Context, framebuffer *Framebuffer, budget time.Duration) (EventStats, error)
	Close() error

	SendMouseMove(x, y int) error
	SendMouseButton(x, y int, button MouseButton, pressed bool) error
	SendMouseClick(x, y int, button MouseButton) error
	SendMouseWheel(x, y int, notches int, horizontal bool) error
	SendKey(scanCode uint16, pressed, extended bool) error
	SendText(text string) error

	SetLocalClipboardText(text string) error
	ClipboardStats() ClipboardStats

	RequestRemoteClipboard() error

	Resize(width, height int) error

	ResizeUnsupported() bool

	DynamicResolutionFallback() bool

	ChannelMap() map[string]uint16
	SecurityLabel() string
}

type connector func(ctx context.Context, options SessionOptions) (*Session, error)

type Options struct {
	Host     string
	Port     int
	Username string
	Password string
	Domain   string
	Width    int
	Height   int

	Engine string

	CertificateFingerprint string

	Clipboard ClipboardSettings
}

type Frame struct {
	Sequence      uint64
	Width         int
	Height        int
	PNG           []byte
	Changed       bool
	CursorVisible bool
	CursorX       int
	CursorY       int
	CursorWidth   int
	CursorHeight  int
	CursorPNG     []byte
}

type SessionStats struct {
	Engine         string
	Connected      bool
	Closed         bool
	Error          string
	Width          int
	Height         int
	Packets        int
	ChannelPackets int
	BitmapUpdates  int
	OrderUpdates   int
	PointerUpdates int
	OrdersApplied  int
	Glyphs         int
	Channels       map[string]uint16
	SecurityLabel  string
	ClipboardText  string

	ClipboardSequence uint64

	HelperStderr string

	ResizeUnsupported bool

	CertificateState CertificateState

	CertificateFingerprint string

	CertificateSubject string
	CertificateIssuer  string

	StreamClients   int
	ServerUpdates   uint64
	ServerFPS       float64
	BatchesSent     uint64
	KeyFramesSent   uint64
	RectsSent       uint64
	StreamBytesSent uint64
	RenderedBatches uint64
	RenderFPS       float64
	DecodeDrops     uint64
	EncodeFailures  uint64
	SlowClientDrops uint64

	PendingRects    int
	PendingArea     int
	PendingKeyFrame bool
	InFlight        bool
	InFlightBytes   int

	LastSnapshotMS float64
	LastEncodeMS   float64
	LastSendMS     float64
	LastRenderMS   float64
	LastBatchRects int
	LastBatchBytes int
}

type Session struct {
	id      string
	options Options

	manager *Manager
	engine  HelperEngine

	conn        protocolSession
	framebuffer *Framebuffer

	mu        sync.Mutex
	stats     EventStats
	lastError error
	closed    bool

	connClosed bool

	lastVersion uint64
	lastPNG     []byte
	sequence    uint64

	frameMu sync.Mutex

	ctx context.Context

	streams *streamMetrics
	hub     *updateHub

	cancel context.CancelFunc
	done   chan struct{}
}

type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	sequence uint64

	dynamicResolutionBlocked map[string]bool

	hostGates map[string]*sync.Mutex

	firstFrameTimeout time.Duration
	probeTimeout      time.Duration
}

func NewManager() *Manager {
	return &Manager{
		sessions:                 map[string]*Session{},
		dynamicResolutionBlocked: map[string]bool{},
		hostGates:                map[string]*sync.Mutex{},
	}
}

func (m *Manager) hostGate(address string, port int) *sync.Mutex {
	if m == nil {
		return nil
	}
	key := sessionHostKey(address, port)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.hostGates == nil {
		m.hostGates = map[string]*sync.Mutex{}
	}
	gate, ok := m.hostGates[key]
	if !ok {
		gate = &sync.Mutex{}
		m.hostGates[key] = gate
	}
	return gate
}

func (m *Manager) ApplyConnectTimeouts(firstFrame, probe time.Duration) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.firstFrameTimeout = NormalizeFirstFrameTimeout(firstFrame)
	m.probeTimeout = NormalizeHostProbeTimeout(probe)
}

func (m *Manager) connectTimeouts() (firstFrame, probe time.Duration) {
	firstFrame, probe = helperFirstFrameTimeout, hostProbeTimeout
	if m == nil {
		return firstFrame, probe
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.firstFrameTimeout > 0 {
		firstFrame = m.firstFrameTimeout
	}
	if m.probeTimeout > 0 {
		probe = m.probeTimeout
	}
	return firstFrame, probe
}

func (m *Manager) Open(ctx context.Context, options Options) (*Session, error) {
	if options.Host == "" {
		return nil, errors.New("远程桌面会话缺少主机地址")
	}
	if options.Width <= 0 {
		options.Width = defaultWidth
	}
	if options.Height <= 0 {
		options.Height = defaultHeight
	}
	if options.Port <= 0 {
		options.Port = 3389
	}

	firstFrameTimeout, probeTimeout := m.connectTimeouts()

	engine, err := resolveHelperEngine(options.Engine)
	if err != nil {
		return nil, err
	}
	sessionOptions := SessionOptions{
		Host:     options.Host,
		Port:     options.Port,
		Username: options.Username,
		Password: options.Password,
		Domain:   options.Domain,
		Width:    uint16(options.Width),
		Height:   uint16(options.Height),

		CertificateFingerprint: options.CertificateFingerprint,

		DynamicResolutionDisabled: m.dynamicResolutionBlockedFor(options.Host, options.Port),

		FirstFrameTimeout: firstFrameTimeout,

		Clipboard: NormalizeClipboardSettings(options.Clipboard),
	}

	if err := probeHostReachable(ctx, options.Host, options.Port, probeTimeout); err != nil {
		return nil, err
	}

	if gate := m.hostGate(options.Host, options.Port); gate != nil {
		gate.Lock()
		defer gate.Unlock()
	}
	helper, err := openHelperSession(ctx, engine, sessionOptions)
	if err != nil {
		return nil, err
	}
	return m.register(options, helper, engine)
}

func (m *Manager) register(options Options, connection protocolSession, engine HelperEngine) (*Session, error) {
	sessionCtx, cancel := context.WithCancel(context.Background())
	session := &Session{
		manager:     m,
		options:     options,
		engine:      engine,
		conn:        connection,
		framebuffer: NewFramebuffer(options.Width, options.Height),
		ctx:         sessionCtx,
		streams:     newStreamMetrics(),
		cancel:      cancel,
		done:        make(chan struct{}),
	}

	session.hub = newUpdateHub(session, DefaultStreamOptions())

	m.mu.Lock()
	m.sequence++
	session.id = fmt.Sprintf("rdp-%d", m.sequence)
	m.sessions[session.id] = session
	m.mu.Unlock()

	go session.renderLoop(sessionCtx)
	return session, nil
}

func (s *Session) Engine() string { return string(s.engine) }

func (s *Session) Clipboard() ClipboardSettings {
	if s == nil {
		return NormalizeClipboardSettings(ClipboardSettings{})
	}
	return NormalizeClipboardSettings(s.options.Clipboard)
}

func (s *Session) CertificateStatus() CertificateStatus {
	if provider, ok := s.conn.(interface{ CertificateStatus() CertificateStatus }); ok {
		status := provider.CertificateStatus()
		status.State = normalizeCertificateState(status.State)
		return status
	}
	return CertificateStatus{State: CertificateStateUnknown}
}

func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.sessions[id]
	return session, ok
}

func (m *Manager) FindByHost(address string, port int) (*Session, bool) {
	if m == nil {
		return nil, false
	}
	normalizedAddress := strings.ToLower(strings.TrimSpace(address))
	if normalizedAddress == "" {
		return nil, false
	}
	if port <= 0 {
		port = 3389
	}
	m.mu.Lock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	m.mu.Unlock()

	for _, session := range sessions {
		if strings.ToLower(strings.TrimSpace(session.options.Host)) != normalizedAddress {
			continue
		}
		sessionPort := session.options.Port
		if sessionPort <= 0 {
			sessionPort = 3389
		}
		if sessionPort == port {
			return session, true
		}
	}
	return nil, false
}

func (m *Manager) List() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	return ids
}

func (m *Manager) Close(id string) error {
	m.mu.Lock()
	session, ok := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("会话 %s 不存在", id)
	}
	return session.Close()
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	m.sessions = map[string]*Session{}
	m.mu.Unlock()
	for _, session := range sessions {
		_ = session.Close()
	}
}

func (m *Manager) noteDynamicResolutionFallback(address string, port int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.dynamicResolutionBlocked == nil {
		m.dynamicResolutionBlocked = map[string]bool{}
	}
	m.dynamicResolutionBlocked[dynamicResolutionKey(address, port)] = true
}

func (m *Manager) dynamicResolutionBlockedFor(address string, port int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.dynamicResolutionBlocked[dynamicResolutionKey(address, port)]
}

func dynamicResolutionKey(address string, port int) string {
	if port <= 0 {
		port = 3389
	}
	return fmt.Sprintf("%s:%d", strings.ToLower(strings.TrimSpace(address)), port)
}

func sessionHostKey(address string, port int) string {
	return dynamicResolutionKey(address, port)
}

func (s *Session) renderLoop(ctx context.Context) {
	defer func() {

		if fallback, ok := s.conn.(interface{ DynamicResolutionFallback() bool }); ok && fallback.DynamicResolutionFallback() {
			if s.manager != nil {
				s.manager.noteDynamicResolutionFallback(s.options.Host, s.options.Port)
			}
		}

		_ = s.closeConnection()
		close(s.done)
	}()
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		stats, err := s.conn.Render(ctx, s.framebuffer, renderSlice)
		s.mu.Lock()
		s.stats = mergeStats(s.stats, stats)
		if err != nil {
			s.lastError = err
			s.closed = true
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()
	}
}

func mergeStats(a, b EventStats) EventStats {
	merged := b
	merged.Packets = a.Packets + b.Packets
	merged.SlowPathPDUs = a.SlowPathPDUs + b.SlowPathPDUs
	merged.BitmapUpdates = a.BitmapUpdates + b.BitmapUpdates
	merged.OrdersUpdates = a.OrdersUpdates + b.OrdersUpdates
	merged.PointerUpdates = a.PointerUpdates + b.PointerUpdates
	merged.OtherUpdates = a.OtherUpdates + b.OtherUpdates
	merged.ChannelPackets = a.ChannelPackets + b.ChannelPackets
	merged.BitmapsApplied = a.BitmapsApplied + b.BitmapsApplied
	merged.BitmapsFailed = a.BitmapsFailed + b.BitmapsFailed
	merged.OrdersApplied = a.OrdersApplied + b.OrdersApplied
	merged.OrdersFailed = a.OrdersFailed + b.OrdersFailed
	merged.PointerApplied = a.PointerApplied + b.PointerApplied
	merged.PointerFailed = a.PointerFailed + b.PointerFailed
	return merged
}

func (s *Session) ID() string { return s.id }

func (s *Session) Frame() (Frame, error) {
	s.frameMu.Lock()
	defer s.frameMu.Unlock()

	version := s.framebuffer.Version()
	width, height := s.framebuffer.Size()
	frame := Frame{Width: width, Height: height}

	if version == 0 {
		s.mu.Lock()
		frame.Sequence = s.sequence
		s.mu.Unlock()
		frame.Changed = false
		return frame, nil
	}

	s.mu.Lock()
	changed := version != s.lastVersion
	s.mu.Unlock()
	frame.Changed = changed

	if changed {

		png, err := s.framebuffer.PNG()
		if err != nil {
			return Frame{}, fmt.Errorf("编码画面失败: %w", err)
		}
		s.mu.Lock()
		s.lastPNG = png
		s.lastVersion = version
		s.sequence++
		frame.PNG = png
		frame.Sequence = s.sequence
		s.mu.Unlock()
	} else {
		s.mu.Lock()
		frame.Sequence = s.sequence
		s.mu.Unlock()
	}

	if png, x, y, width, height, ok := s.framebuffer.CursorFrame(); ok {
		frame.CursorVisible = true
		frame.CursorX = x
		frame.CursorY = y
		frame.CursorWidth = width
		frame.CursorHeight = height
		frame.CursorPNG = png
	}
	return frame, nil
}

func (s *Session) Stats() SessionStats {
	s.mu.Lock()
	stats := s.stats
	lastError := s.lastError
	closed := s.closed
	s.mu.Unlock()

	result := SessionStats{
		Engine:         string(s.engine),
		Connected:      !closed,
		Closed:         closed,
		Packets:        stats.Packets,
		ChannelPackets: stats.ChannelPackets,
		BitmapUpdates:  stats.BitmapUpdates,
		OrderUpdates:   stats.OrdersUpdates,
		PointerUpdates: stats.PointerUpdates,
		OrdersApplied:  stats.OrdersApplied,
		Channels:       s.conn.ChannelMap(),
		SecurityLabel:  s.conn.SecurityLabel(),

		CertificateState: CertificateStateUnknown,
	}
	result.Width, result.Height = s.framebuffer.Size()
	if lastError != nil {
		result.Error = lastError.Error()
	}

	clipboard := s.conn.ClipboardStats()
	result.ClipboardText = clipboard.Text
	result.ClipboardSequence = clipboard.Sequence
	result.ResizeUnsupported = s.conn.ResizeUnsupported()
	if diagnostics, ok := s.conn.(interface{ HelperStderr() string }); ok {
		result.HelperStderr = diagnostics.HelperStderr()
	}

	if provider, ok := s.conn.(interface{ CertificateStatus() CertificateStatus }); ok {
		certificate := provider.CertificateStatus()
		result.CertificateState = normalizeCertificateState(certificate.State)
		result.CertificateFingerprint = certificate.Fingerprint
		result.CertificateSubject = certificate.Subject
		result.CertificateIssuer = certificate.Issuer
		if certificateErr := certificateStateError(result.CertificateState); certificateErr != nil {

			result.Closed = true
			result.Connected = false
			result.Error = certificateErr.Error()
		}
	}

	result.ServerUpdates = s.framebuffer.Updates()
	pendingRects, pendingArea, pendingKeyFrame := s.framebuffer.PendingDirty()
	result.PendingRects = pendingRects
	result.PendingArea = pendingArea
	result.PendingKeyFrame = pendingKeyFrame
	streams := s.streams.snapshot(result.ServerUpdates)
	result.StreamClients = streams.clients
	result.ServerFPS = streams.serverFPS
	result.BatchesSent = streams.batchesSent
	result.KeyFramesSent = streams.keyFramesSent
	result.RectsSent = streams.rectsSent
	result.StreamBytesSent = streams.bytesSent
	result.RenderedBatches = streams.renderedBatches
	result.RenderFPS = streams.renderFPS
	result.DecodeDrops = streams.decodeDrops
	result.EncodeFailures = streams.encodeFailures
	result.SlowClientDrops = streams.slowClient
	result.InFlight = streams.inFlight
	result.InFlightBytes = streams.inFlightBytes
	result.LastSnapshotMS = streams.lastSnapshotMS
	result.LastEncodeMS = streams.lastEncodeMS
	result.LastSendMS = streams.lastSendMS
	result.LastRenderMS = streams.lastRenderMS
	result.LastBatchRects = streams.lastBatchRects
	result.LastBatchBytes = streams.lastBatchBytes
	return result
}

func (s *Session) FrameContent() (nonBlack int, distinctColors int) {
	return s.framebuffer.Stats()
}

func (s *Session) FrameChecksum() uint64 { return s.framebuffer.Checksum() }

func (s *Session) FramePNG() ([]byte, error) { return s.framebuffer.PNG() }

func (s *Session) SendPointerMove(x, y int) error { return s.conn.SendMouseMove(x, y) }

func (s *Session) SendPointerButton(x, y int, button MouseButton, pressed bool) error {
	return s.conn.SendMouseButton(x, y, button, pressed)
}

func (s *Session) SendPointerClick(x, y int, button MouseButton) error {
	return s.conn.SendMouseClick(x, y, button)
}

func (s *Session) SendWheel(x, y, notches int, horizontal bool) error {
	return s.conn.SendMouseWheel(x, y, notches, horizontal)
}

func (s *Session) SendKey(scanCode uint16, pressed, extended bool) error {
	return s.conn.SendKey(scanCode, pressed, extended)
}

func (s *Session) SendText(text string) error { return s.conn.SendText(text) }

func (s *Session) SetClipboardText(text string) error { return s.conn.SetLocalClipboardText(text) }

func (s *Session) RequestRemoteClipboard() error { return s.conn.RequestRemoteClipboard() }

func (s *Session) ClipboardText() string { return s.conn.ClipboardStats().Text }

func (s *Session) ClipboardSequence() uint64 { return s.conn.ClipboardStats().Sequence }

func (s *Session) WaitClipboardSequenceChange(after uint64, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if s.ClipboardSequence() != after {
			return true
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false
		}
		wait := clipboardSequencePollInterval
		if remaining < wait {
			wait = remaining
		}
		time.Sleep(wait)
	}
}

func (s *Session) Wait() {
	<-s.done
}

func (s *Session) ResizeUnsupported() bool { return s.conn.ResizeUnsupported() }

func (s *Session) Resize(width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("远程桌面尺寸非法: %dx%d", width, height)
	}
	return s.conn.Resize(width, height)
}

func (s *Session) closeConnection() error {
	s.mu.Lock()
	if s.connClosed {
		s.mu.Unlock()
		return nil
	}
	s.connClosed = true
	s.closed = true
	cancel := s.cancel
	conn := s.conn
	s.mu.Unlock()

	releaseHostGate := s.lockHostGate()
	defer releaseHostGate()

	if cancel != nil {
		cancel()
	}
	if conn == nil {
		return nil
	}
	if err := conn.Close(); err != nil {
		return fmt.Errorf("关闭远程桌面会话失败: %w", err)
	}
	return nil
}

func (s *Session) lockHostGate() func() {
	if s.manager == nil {
		return func() {}
	}
	gate := s.manager.hostGate(s.options.Host, s.options.Port)
	if gate == nil {
		return func() {}
	}
	gate.Lock()
	return gate.Unlock
}

func (s *Session) Close() error {
	err := s.closeConnection()
	if s.manager != nil {
		s.manager.remove(s.id, s)
	}
	return err
}

func (m *Manager) remove(id string, session *Session) {
	if m == nil || id == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if current, ok := m.sessions[id]; ok && current == session {
		delete(m.sessions, id)
	}
}
