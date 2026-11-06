package app

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"changeme/internal/errno"
	"changeme/internal/model"
	"changeme/internal/remotedesktop"
)

type rdpOpenedSession interface {
	ID() string
	Stats() remotedesktop.SessionStats
}

type rdpSessionOpener func(ctx context.Context, options remotedesktop.Options) (rdpOpenedSession, error)

func rdpHostHasRoute(record model.Host) bool {
	if record.JumpHostID != nil && strings.TrimSpace(*record.JumpHostID) != "" {
		return true
	}
	if record.ProxyMode != nil {
		mode := strings.TrimSpace(*record.ProxyMode)
		return mode != "" && mode != model.ProxyModeNone
	}
	return false
}

type rdpRouteConn struct {
	net.Conn
	closeChain func() error
	once       sync.Once
	closeErr   error
}

func (c *rdpRouteConn) Close() error {
	if c == nil {
		return nil
	}
	var connErr error
	if c.Conn != nil {
		connErr = c.Conn.Close()
	}
	c.once.Do(func() {
		if c.closeChain != nil {
			c.closeErr = c.closeChain()
		}
	})
	if connErr != nil {
		return connErr
	}
	return c.closeErr
}

func rdpBridgeTCPAddr(local net.Addr) (string, int, bool) {
	addr, ok := local.(*net.TCPAddr)
	if !ok || addr == nil || addr.IP == nil {
		return "", 0, false
	}
	if !addr.IP.Equal(net.IPv4(127, 0, 0, 1)) || addr.Port <= 0 || addr.Port > 65535 {
		return "", 0, false
	}
	return "127.0.0.1", addr.Port, true
}

func (s *Service) openRdpOptions(ctx context.Context, options remotedesktop.Options, routeHost *model.Host) (rdpOpenedSession, error) {
	if routeHost != nil && rdpHostHasRoute(*routeHost) {
		return s.openRoutedRdpOptions(ctx, options, *routeHost)
	}
	return s.openDirectRdpOptions(ctx, options)
}

func (s *Service) openDirectRdpOptions(ctx context.Context, options remotedesktop.Options) (rdpOpenedSession, error) {
	if s != nil && s.rdpSessionOpener != nil {
		return s.rdpSessionOpener(ctx, options)
	}
	manager, err := s.rdpManager()
	if err != nil {
		return nil, err
	}
	session, err := manager.Open(ctx, options)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (s *Service) openRoutedRdpOptions(ctx context.Context, options remotedesktop.Options, record model.Host) (rdpOpenedSession, error) {
	dial := s.rdpDialRoute
	if dial == nil {
		if s == nil || s.sessions == nil {
			return nil, rdpError(errno.CodeConnFailed, "rdp.serviceNotReady", nil)
		}

		dial = func(dialCtx context.Context, _ string, address string) (net.Conn, error) {
			conn, closeChain, err := s.sessions.DialRoute(dialCtx, record.ID, address)
			if err != nil {
				return nil, err
			}
			return &rdpRouteConn{Conn: conn, closeChain: closeChain}, nil
		}
	}
	target := net.JoinHostPort(strings.TrimSpace(record.Address), strconv.Itoa(options.Port))

	bridgeCtx := context.Background()
	bridge, local, err := s.startRdpRouteBridge(bridgeCtx, dial, target)
	if err != nil {

		return nil, rdpError(errno.CodeConnFailed, "rdp.route_bridge_failed", err)
	}
	host, port, ok := rdpBridgeTCPAddr(local)
	if !ok {
		_ = bridge.close()
		return nil, rdpError(errno.CodeConnFailed, "rdp.route_bridge_failed",
			errors.New("rdp route bridge: unexpected listener address"))
	}
	options.Host = host
	options.Port = port

	session, err := s.openDirectRdpOptions(ctx, options)
	if err != nil {

		_ = bridge.close()
		return nil, err
	}

	if dialErr, known := bridge.waitFirstDial(ctx, s.rdpRouteConfirmTimeout()); known && dialErr != nil {
		_ = s.closeRdpOpenedSession(session.ID())
		_ = bridge.close()
		return nil, dialErr
	}

	s.registerRdpRouteBridge(session.ID(), bridge.close)
	return session, nil
}

func (s *Service) rdpRouteConfirmTimeout() time.Duration {
	if s != nil && s.rdpRouteConfirmTimeoutValue > 0 {
		return s.rdpRouteConfirmTimeoutValue
	}
	return rdpRouteBridgeConfirmTimeout
}

func (s *Service) closeRdpOpenedSession(sessionID string) error {
	if s == nil || sessionID == "" || s.remoteDesktops == nil {
		return nil
	}
	return s.remoteDesktops.Close(sessionID)
}

func (s *Service) startRdpRouteBridge(ctx context.Context, dial rdpRouteDialer, target string) (*rdpRouteBridge, net.Addr, error) {
	if s != nil && s.rdpBridgeStarter != nil {
		return s.rdpBridgeStarter(ctx, dial, target)
	}
	return startRdpRouteBridgeObserved(ctx, dial, target)
}

func (s *Service) registerRdpRouteBridge(sessionID string, closeFn func() error) {
	if s == nil || strings.TrimSpace(sessionID) == "" || closeFn == nil {
		return
	}
	s.rdpRouteMu.Lock()
	if s.rdpRouteBridges == nil {
		s.rdpRouteBridges = make(map[string]func() error)
	}
	previous := s.rdpRouteBridges[sessionID]
	s.rdpRouteBridges[sessionID] = closeFn
	s.rdpRouteMu.Unlock()

	if previous != nil {
		_ = previous()
	}
}

func (s *Service) releaseRdpRouteBridge(sessionID string) {
	if s == nil {
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	s.rdpRouteMu.Lock()
	closeFn := s.rdpRouteBridges[sessionID]
	delete(s.rdpRouteBridges, sessionID)
	s.rdpRouteMu.Unlock()
	if closeFn != nil {
		_ = closeFn()
	}
}

func (s *Service) releaseAllRdpRouteBridges() {
	if s == nil {
		return
	}
	s.rdpRouteMu.Lock()
	bridges := s.rdpRouteBridges
	s.rdpRouteBridges = nil
	s.rdpRouteMu.Unlock()
	for _, closeFn := range bridges {
		if closeFn != nil {
			_ = closeFn()
		}
	}
}

func (s *Service) rdpRouteBridgeCount() int {
	if s == nil {
		return 0
	}
	s.rdpRouteMu.Lock()
	defer s.rdpRouteMu.Unlock()
	return len(s.rdpRouteBridges)
}
