package session

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"

	"changeme/internal/errno"
	"changeme/internal/model"
	sshclient "changeme/internal/tunnel"
)

const defaultRouteDialTimeout = 15 * time.Second

func (s *Service) DialRoute(
	ctx context.Context,
	hostID string,
	address string,
) (net.Conn, func() error, error) {
	_, connector, err := s.ready()
	if err != nil {
		return nil, nil, err
	}
	if connector == nil || connector.verifier == nil || connector.hosts == nil {
		return nil, nil, errno.New(errno.CodeSessionClosed, "session.connector_not_ready", "")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	hostID = strings.TrimSpace(hostID)
	address = strings.TrimSpace(address)
	if hostID == "" || address == "" {
		return nil, nil, errno.New(errno.CodeInvalidArg, "session.invalid", "")
	}

	host, err := connector.hosts.SessionHost(ctx, hostID)
	if err != nil {

		if errno.Extract(err) != nil {
			return nil, nil, err
		}
		return nil, nil, errno.New(errno.CodeConnFailed, "session.host_unavailable", "")
	}

	route, err := connector.resolveRoute(ctx, host)
	if err != nil {

		return nil, nil, err
	}
	if !route.routed() {
		route.release()
		return nil, nil, errno.New(errno.CodeInvalidArg, "session.route_not_configured", "")
	}

	verify := connector.verifier.Verify
	tunnelConnector := sshclient.NewConnector(verify)
	if route.proxy != nil {
		dial, dialErr := sshclient.NewProxyDialer(*route.proxy)
		if dialErr != nil {
			route.release()
			return nil, nil, errno.New(errno.CodeInvalidArg, "session.proxy_invalid", "")
		}

		tunnelConnector = sshclient.NewConnectorWithDialer(verify, dial)
	}

	builder := tunnelConnector.NewHopConnector()

	var closeOnce sync.Once
	closeChain := func() error {
		closeOnce.Do(func() {
			builder.Close()
			route.release()
		})
		return nil
	}

	adopted := false
	defer func() {
		if !adopted {
			_ = closeChain()
		}
	}()

	for index, hop := range route.hops {
		if _, connectErr := builder.Connect(ctx, hop); connectErr != nil {

			return nil, nil, sshclient.HopConnectError(connectErr, index)
		}
	}

	dialCtx, cancel := context.WithTimeout(ctx, routeDialTimeout(host))
	conn, dialErr := builder.Dialer()(dialCtx, "tcp", address)
	cancel()
	if dialErr != nil {
		return nil, nil, routeDialFailure(dialErr, len(route.hops))
	}
	adopted = true
	return conn, closeChain, nil
}

func routeDialTimeout(host model.Host) time.Duration {
	if timeout := hostConnectTimeout(host); timeout > 0 {
		return timeout
	}
	return defaultRouteDialTimeout
}

func routeDialFailure(err error, hops int) error {
	if hops == 0 && sshclient.IsProxyError(err) {
		return &sshclient.ConnectError{Stage: sshclient.StageProxy, Err: err}
	}
	if hops > 0 && sshclient.IsForwardDenied(err) {
		return &sshclient.ConnectError{Stage: sshclient.StageJumpDial, Hop: hops - 1, Err: err}
	}
	return err
}
