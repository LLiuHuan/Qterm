package session

import (
	"changeme/internal/errno"
	"changeme/internal/model"
	sshclient "changeme/internal/tunnel"
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	xssh "golang.org/x/crypto/ssh"
)

func (s *Service) TestConnection(ctx context.Context, input TestConnectionInput) error {
	_, connector, err := s.ready()
	if err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	testCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if strings.TrimSpace(input.Protocol) == "" {
		input.Protocol = "ssh"
	}
	switch strings.TrimSpace(input.Protocol) {
	case model.ProtocolRDP:
		return testRdpReachable(testCtx, input)
	case model.ProtocolTelnet:

		return testTelnetReachable(testCtx, input)
	case model.ProtocolVNC:

		return testTelnetReachable(testCtx, input)
	case model.ProtocolSSH:
	default:
		return errno.New(errno.CodeInvalidArg, "session.protocol_unsupported", "")
	}
	target := sshclient.Target{
		Address:  strings.TrimSpace(input.Address),
		Port:     input.Port,
		Username: strings.TrimSpace(input.Username),
	}
	if target.Address == "" || target.Username == "" || target.Port < 1 || target.Port > 65535 {
		return errno.New(errno.CodeInvalidArg, "session.target_invalid", "")
	}

	var host model.Host
	if hostID := strings.TrimSpace(input.HostID); hostID != "" {
		loaded, hostErr := s.hosts.SessionHost(testCtx, hostID)
		if hostErr != nil {
			return testConnectionHostError(hostErr)
		}
		host = loaded
	}

	var testErr error
	switch {
	case strings.TrimSpace(input.CredentialID) != "":

		testErr = connector.TestStoredConnectionForHost(
			testCtx, host, strings.TrimSpace(input.CredentialID), target,
		)
	case input.Kind == model.CredentialKindPassword && input.Secret == "":

		if host.ID == "" {
			return errno.New(errno.CodeInvalidArg, "session.credential_required", "")
		}
		if !host.PasswordConfigured {
			return errno.New(errno.CodeInvalidArg, "session.credential_required", "")
		}
		testErr = s.hosts.UseHostPassword(testCtx, host.ID, func(secret []byte) error {
			return connector.TestConnectionForHost(testCtx, host, target, model.CredentialKindPassword, secret)
		})
	case input.Kind == "" && input.Secret == "" && host.ID != "":

		if !host.PasswordConfigured {
			return errno.New(errno.CodeInvalidArg, "session.credential_required", "")
		}
		testErr = s.hosts.UseHostPassword(testCtx, host.ID, func(secret []byte) error {
			return connector.TestConnectionForHost(testCtx, host, target, model.CredentialKindPassword, secret)
		})
	case input.Kind == "" || input.Secret == "":
		return errno.New(errno.CodeInvalidArg, "session.credential_required", "")
	default:
		secret := []byte(input.Secret)
		defer clear(secret)
		testErr = connector.TestConnectionForHost(testCtx, host, target, input.Kind, secret)
	}
	return testConnectionError(testErr)
}

func testRdpReachable(ctx context.Context, input TestConnectionInput) error {
	address := strings.TrimSpace(input.Address)
	port := input.Port
	if address == "" || port < 1 || port > 65535 {
		return errno.New(errno.CodeInvalidArg, "session.target_invalid", "")
	}
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(address, strconv.Itoa(port)))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return errno.New(errno.CodeTimeout, "session.timeout", "")
		}
		if errors.Is(err, context.Canceled) {
			return err
		}
		return errno.New(errno.CodeConnFailed, "session.rdp_unreachable", "")
	}
	_ = conn.Close()
	return nil
}

func testTelnetReachable(ctx context.Context, input TestConnectionInput) error {
	address := strings.TrimSpace(input.Address)
	port := input.Port
	if address == "" || port < 1 || port > 65535 {
		return errno.New(errno.CodeInvalidArg, "session.target_invalid", "")
	}
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(address, strconv.Itoa(port)))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return errno.New(errno.CodeTimeout, "session.timeout", "")
		}
		if errors.Is(err, context.Canceled) {
			return err
		}

		return errno.New(errno.CodeConnFailed, "session.host_unreachable", "")
	}
	_ = conn.Close()
	return nil
}

func testConnectionHostError(err error) error {
	if err == nil {
		return nil
	}
	if errno.Extract(err) != nil {
		return err
	}
	if errors.Is(err, model.ErrCredentialVaultLocked) {
		return errno.New(errno.CodePermissionDenied, "credential.locked", "")
	}
	return errno.New(errno.CodeConnFailed, "session.host_unavailable", "")
}

func testConnectionError(err error) error {
	if err == nil {
		return nil
	}

	if errno.Extract(err) != nil {
		return err
	}
	if errors.Is(err, context.Canceled) {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errno.New(errno.CodeTimeout, "session.timeout", "")
	}
	if errors.Is(err, sshclient.ErrHostKeyChanged) {
		return errno.New(errno.CodeConnFailed, "session.hostkey_changed", "")
	}
	if errors.Is(err, sshclient.ErrHostKeyUnknown) {

		return errno.New(errno.CodeConnFailed, "session.hostkey_unknown_reachable", "")
	}
	if errors.Is(err, model.ErrCredentialVaultLocked) {
		return errno.New(errno.CodePermissionDenied, "credential.locked", "")
	}

	messageKey := SessionConnectionFailureKey(err)
	code := errno.CodeConnFailed
	switch messageKey {
	case "session.connection_invalid":
		code = errno.CodeInvalidArg
	case "session.timeout":
		code = errno.CodeTimeout
	case "credential.locked":
		code = errno.CodePermissionDenied
	}
	return errno.New(code, messageKey, "")
}

const routeTestTimeout = 60 * time.Second

const routeSegmentTimeout = 10 * time.Second

var errTestHostKeyUnverified = errors.New("test route: host key not verified")

func (s *Service) TestHostRoute(ctx context.Context, hostID string) (model.RouteTestResult, error) {
	_, connector, err := s.ready()
	if err != nil {
		return model.RouteTestResult{}, err
	}
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return model.RouteTestResult{}, errno.New(errno.CodeInvalidArg, "session.target_invalid", "")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	testCtx, cancel := context.WithTimeout(ctx, routeTestTimeout)
	defer cancel()
	host, err := s.hosts.SessionHost(testCtx, hostID)
	if err != nil {
		return model.RouteTestResult{}, testConnectionHostError(err)
	}
	return s.testRouteForHost(testCtx, connector, host, nil, nil)
}

func (s *Service) TestRouteDraft(ctx context.Context, input model.RouteDraftInput) (model.RouteTestResult, error) {
	_, connector, err := s.ready()
	if err != nil {
		return model.RouteTestResult{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	testCtx, cancel := context.WithTimeout(ctx, routeTestTimeout)
	defer cancel()

	host := model.Host{}
	if hostID := strings.TrimSpace(input.HostID); hostID != "" {
		loaded, hostErr := s.hosts.SessionHost(testCtx, hostID)
		if hostErr != nil {
			return model.RouteTestResult{}, testConnectionHostError(hostErr)
		}
		host = loaded
	} else {

		host.Protocol = model.ProtocolSSH
	}

	if protocol := strings.TrimSpace(input.Protocol); protocol != "" {
		host.Protocol = protocol
	}

	if strings.TrimSpace(input.Kind) != "" || strings.TrimSpace(input.Secret) != "" {
		host.CredentialID = ""
		host.PasswordSource = ""
	}
	host.Address = strings.TrimSpace(input.Address)
	host.Port = input.Port
	host.Username = strings.TrimSpace(input.Username)
	if host.Address == "" || host.Port < 1 || host.Port > 65535 {
		return model.RouteTestResult{}, errno.New(errno.CodeInvalidArg, "session.target_invalid", "")
	}
	if jumpID := strings.TrimSpace(input.JumpHostID); jumpID != "" {
		host.JumpHostID = &jumpID
	} else {

		host.JumpHostID = nil
	}

	if proxyID := strings.TrimSpace(input.ProxyID); proxyID != "" {
		host.ProxyID = proxyID
		noneMode := model.ProxyModeNone
		host.ProxyMode = &noneMode
		host.ProxyHost, host.ProxyPort, host.ProxyUsername = nil, nil, nil
		host.ProxyPasswordConfigured = false
	} else {
		host.ProxyID = ""
		proxyMode := strings.TrimSpace(input.ProxyMode)
		if proxyMode == "" {
			proxyMode = model.ProxyModeNone
		}
		host.ProxyMode = &proxyMode
		if proxyMode == model.ProxyModeNone {
			host.ProxyHost, host.ProxyPort, host.ProxyUsername = nil, nil, nil
			host.ProxyPasswordConfigured = false
		} else {
			host.ProxyHost = optionalRouteText(input.ProxyHost)
			host.ProxyUsername = optionalRouteText(input.ProxyUsername)
			if input.ProxyPassword != "" {
				host.ProxyPasswordConfigured = true
			} else if input.ClearProxyPassword {
				host.ProxyPasswordConfigured = false
			}
			if input.ProxyPort > 0 {
				port := input.ProxyPort
				host.ProxyPort = &port
			} else {
				host.ProxyPort = nil
			}
		}
	}

	if credentialID := strings.TrimSpace(input.CredentialID); credentialID != "" {
		host.CredentialID = credentialID
	}

	var targetAuth xssh.AuthMethod
	if host.CredentialID == "" && input.Kind != "" && input.Secret != "" {
		secret := []byte(input.Secret)

		defer clear(secret)
		targetAuth, err = auxiliaryAuthMethod(model.CredentialKind(input.Kind), secret)
		if err != nil {
			return model.RouteTestResult{}, testConnectionError(err)
		}
	}
	proxyPassword := []byte(input.ProxyPassword)
	defer clear(proxyPassword)
	return s.testRouteForHost(testCtx, connector, host, targetAuth, proxyPassword)
}

func optionalRouteText(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func (s *Service) testRouteForHost(
	ctx context.Context,
	connector *Connector,
	host model.Host,
	targetAuthOverride xssh.AuthMethod,
	proxyPassword []byte,
) (model.RouteTestResult, error) {
	route, err := connector.resolveRouteWithProxyPassword(ctx, host, proxyPassword, routeAuthContext{})
	if err != nil {
		return model.RouteTestResult{}, testConnectionError(err)
	}
	defer route.release()

	result := model.RouteTestResult{
		Mode: routeMode(route),
		Hops: make([]model.RouteTestHop, 0, len(route.hops)+2),
	}

	fail := func(row model.RouteTestHop) (model.RouteTestResult, error) {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return model.RouteTestResult{}, ctxErr
		}
		result.OK = false
		result.Hops = append(result.Hops, row)
		return result, nil
	}

	targetAddress := joinTargetAddress(host.Address, host.Port)
	firstAddress := targetAddress
	if len(route.hops) > 0 {
		firstAddress = joinTargetAddress(route.hops[0].Target.Address, route.hops[0].Target.Port)
	}

	var proxyDial sshclient.DialerFunc
	if route.proxy != nil {
		proxyAddress := route.proxy.AddressString()
		row := model.RouteTestHop{Kind: "proxy", Label: proxyAddress, Address: proxyAddress}
		dial, dialErr := sshclient.NewProxyDialer(*route.proxy)
		if dialErr != nil {
			row.ErrorKey = "session.proxy_invalid"
			return fail(row)
		}

		started := time.Now()

		probeCtx, probeCancel := context.WithTimeout(ctx, routeSegmentTimeout)
		probe, probeErr := dial(probeCtx, "tcp", firstAddress)
		probeCancel()
		row.LatencyMs = time.Since(started).Milliseconds()
		if probeErr != nil {
			row.Stage, row.ErrorKey = routeTestFailure(&sshclient.ConnectError{
				Stage: sshclient.StageProxy,
				Err:   probeErr,
			})
			return fail(row)
		}
		_ = probe.Close()
		row.Reachable = true
		proxyDial = dial
		result.Hops = append(result.Hops, row)
	}

	unverified := make(map[string]bool)
	baseVerify := connector.verifier.Callback()
	testVerify := func(hostname string, remote net.Addr, key xssh.PublicKey) error {
		verifyErr := baseVerify(hostname, remote, key)
		if verifyErr == nil {
			return nil
		}
		if errors.Is(verifyErr, sshclient.ErrHostKeyUnknown) {
			unverified[strings.ToLower(strings.TrimSpace(hostname))] = true

			return errTestHostKeyUnverified
		}
		return verifyErr
	}
	directConnector := sshclient.NewConnector(testVerify)
	baseConnector := directConnector
	if proxyDial != nil {

		baseConnector = sshclient.NewConnectorWithDialer(testVerify, proxyDial)
	}
	builder := baseConnector.NewHopConnector()
	defer builder.Close()
	for index, hop := range route.hops {
		label := joinTargetAddress(hop.Target.Address, hop.Target.Port)
		if index < len(route.hopHosts) {
			if name := strings.TrimSpace(route.hopHosts[index].Name); name != "" {
				label = name
			}
		}
		hopAddress := joinTargetAddress(hop.Target.Address, hop.Target.Port)
		row := model.RouteTestHop{Kind: "jump", Label: label, Address: hopAddress}
		started := time.Now()
		_, hopErr := builder.Connect(ctx, hop)
		row.LatencyMs = time.Since(started).Milliseconds()
		if errors.Is(hopErr, errTestHostKeyUnverified) {

			row.Reachable = true
			row.UnverifiedHostKey = true
			row.Stage = "hostkey"
			result.Hops = append(result.Hops, row)
			result.Hops = append(result.Hops, model.RouteTestHop{
				Kind:     "target",
				Label:    joinTargetAddress(host.Address, host.Port),
				Address:  joinTargetAddress(host.Address, host.Port),
				ErrorKey: "session.route_test_unreached",
				Stage:    "hostkey",
			})
			result.OK = false
			return result, nil
		}
		if hopErr != nil {

			row.Stage, row.ErrorKey = routeTestFailure(sshclient.HopConnectError(hopErr, index))
			return fail(row)
		}
		row.Reachable = true
		row.UnverifiedHostKey = unverified[strings.ToLower(strings.TrimSpace(hopAddress))]
		result.Hops = append(result.Hops, row)
	}

	targetLabel := joinTargetAddress(host.Address, host.Port)
	if name := strings.TrimSpace(host.Name); name != "" {

		targetLabel = name
	}
	targetRow := model.RouteTestHop{
		Kind:    "target",
		Label:   targetLabel,
		Address: joinTargetAddress(host.Address, host.Port),
	}

	if !strings.EqualFold(strings.TrimSpace(host.Protocol), model.ProtocolSSH) &&
		strings.TrimSpace(host.Protocol) != "" {
		started := time.Now()
		conn, dialErr := builder.Dialer()(ctx, "tcp", targetRow.Address)
		targetRow.LatencyMs = time.Since(started).Milliseconds()
		if dialErr != nil {
			targetRow.Stage, targetRow.ErrorKey = routeTestFailure(dialErr)
			return fail(targetRow)
		}
		_ = conn.Close()
		targetRow.Reachable = true
		result.Hops = append(result.Hops, targetRow)
		result.OK = true
		return result, nil
	}
	targetAuth := targetAuthOverride
	if targetAuth == nil {
		var authErr error
		targetAuth, authErr = connector.routeTargetAuth(ctx, host)
		if authErr != nil {
			targetRow.Stage, targetRow.ErrorKey = routeTestFailure(authErr)
			if targetRow.ErrorKey == "" || targetRow.ErrorKey == "session.connection_failed" {
				targetRow.ErrorKey = "session.credential_required"
				targetRow.Stage = "auth"
			}
			return fail(targetRow)
		}
	}
	started := time.Now()
	_, targetErr := builder.Connect(ctx, sshclient.Hop{
		Target: sshclient.Target{
			Address:  host.Address,
			Port:     host.Port,
			Username: host.Username,
			Timeout:  hostConnectTimeout(host),
		},
		Auth: targetAuth,
	})
	targetRow.LatencyMs = time.Since(started).Milliseconds()
	if errors.Is(targetErr, errTestHostKeyUnverified) {
		targetRow.Reachable = true
		targetRow.UnverifiedHostKey = true
		targetRow.Stage = "hostkey"
		result.Hops = append(result.Hops, targetRow)
		result.OK = true
		return result, nil
	}
	if targetErr != nil {

		targetRow.Stage, targetRow.ErrorKey = routeTestFailure(targetErr)
		return fail(targetRow)
	}
	targetRow.Reachable = true
	targetRow.UnverifiedHostKey = unverified[strings.ToLower(strings.TrimSpace(targetRow.Address))]
	result.Hops = append(result.Hops, targetRow)
	result.OK = true
	return result, nil
}

func routeMode(route resolvedRoute) string {
	if len(route.hops) > 0 {
		return "jump"
	}
	if route.proxy != nil {
		return "proxy"
	}
	return "direct"
}

func routeTestFailure(err error) (string, string) {
	if err == nil {
		return "", ""
	}
	if errors.Is(err, sshclient.ErrHostKeyUnknown) {

		return "hostkey", "session.hostkey_unknown_reachable"
	}
	if errors.Is(err, sshclient.ErrHostKeyChanged) {
		return "hostkey", "session.hostkey_changed"
	}
	var connectErr *sshclient.ConnectError
	if errors.As(err, &connectErr) && connectErr.Stage == sshclient.StageValidation {
		return "handshake", "session.connection_invalid"
	}
	if isSSHAuthenticationError(err) {
		return "auth", SessionConnectionFailureKey(err)
	}
	if errors.As(err, &connectErr) && connectErr.Stage == sshclient.StageProxy {

		return "proxy", "session.proxy_unreachable"
	}
	if errors.As(err, &connectErr) && connectErr.Stage == sshclient.StageTCP {
		return "tcp", SessionConnectionFailureKey(err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "tcp", "session.timeout"
	}
	return "handshake", SessionConnectionFailureKey(err)
}

func joinTargetAddress(address string, port int) string {
	if port <= 0 {
		port = 22
	}
	return net.JoinHostPort(strings.TrimSpace(address), strconv.Itoa(port))
}
