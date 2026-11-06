package session

import (
	"bytes"
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"changeme/internal/errno"
	"changeme/internal/model"
	sshclient "changeme/internal/tunnel"

	xssh "golang.org/x/crypto/ssh"
)

const maxRouteJumpDepth = 5

type resolvedRoute struct {
	proxy *sshclient.ProxySpec
	hops  []sshclient.Hop

	hopHosts []model.Host
	close    func()
}

func (r resolvedRoute) routed() bool {
	return r.proxy != nil || len(r.hops) > 0
}

func (r *resolvedRoute) release() {
	if r == nil || r.close == nil {
		return
	}
	r.close()
}

type routeAuthContext struct {
	sessionID   string
	reportState func(model.SessionState) error
}

func (c *Connector) resolveRoute(ctx context.Context, host model.Host) (resolvedRoute, error) {
	return c.resolveRouteWithProxyPassword(ctx, host, nil, routeAuthContext{})
}

func (c *Connector) resolveRouteWith(
	ctx context.Context,
	host model.Host,
	auth routeAuthContext,
) (resolvedRoute, error) {
	return c.resolveRouteWithProxyPassword(ctx, host, nil, auth)
}

func (c *Connector) resolveRouteWithProxyPassword(
	ctx context.Context,
	host model.Host,
	proxyPassword []byte,
	auth routeAuthContext,
) (resolvedRoute, error) {
	if c == nil || c.hosts == nil {
		return resolvedRoute{}, errno.New(errno.CodeSessionClosed, "session.connector_not_ready", "")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	chain, err := c.expandJumpChain(ctx, host)
	if err != nil {
		return resolvedRoute{}, err
	}
	proxy, releaseProxy, err := c.resolveProxy(ctx, host, proxyPassword)
	if err != nil {
		return resolvedRoute{}, err
	}
	route := resolvedRoute{proxy: proxy}
	if releaseProxy != nil {
		route.close = releaseProxy
	}
	for index, hopHost := range chain {
		hopAuth, reusablePassword, authErr := c.resolveHopAuth(ctx, hopHost, index, auth)
		if authErr != nil {
			route.release()
			return resolvedRoute{}, authErr
		}
		hopMethods := []xssh.AuthMethod{hopAuth}
		interactive := false
		if auth.sessionID != "" {
			hopMethods = append(hopMethods, c.keyboardInteractiveAuth(ctx, auth, hopHost, index+1, reusablePassword))
			interactive = true
		}
		route.hopHosts = append(route.hopHosts, hopHost)
		route.hops = append(route.hops, sshclient.Hop{
			Target:      hopTarget(hopHost),
			Auth:        hopAuth,
			AuthMethods: hopMethods,
			Interactive: interactive,

			Label: hopDisplayLabel(hopHost),
		})
	}
	return route, nil
}

func (c *Connector) expandJumpChain(ctx context.Context, host model.Host) ([]model.Host, error) {
	visited := make(map[string]struct{}, 2)
	if host.ID != "" {
		visited[host.ID] = struct{}{}
	}
	innerToOuter := make([]model.Host, 0, 2)
	current := host
	for depth := 0; ; depth++ {
		jumpID := jumpHostID(current)
		if jumpID == "" {
			break
		}
		if depth >= maxRouteJumpDepth {
			return nil, errno.New(errno.CodeInvalidArg, "host.jump_chain_too_deep", "")
		}
		if _, seen := visited[jumpID]; seen {

			return nil, errno.New(errno.CodeInvalidArg, "host.jump_cycle", "")
		}
		visited[jumpID] = struct{}{}
		jump, err := c.hosts.SessionHost(ctx, jumpID)
		if err != nil {

			return nil, errno.New(errno.CodeConnFailed, "host.jump_not_found", "")
		}
		if protocol := strings.TrimSpace(jump.Protocol); protocol != "" && protocol != model.ProtocolSSH {

			return nil, errno.New(errno.CodeInvalidArg, "host.jump_protocol_invalid", "")
		}
		innerToOuter = append(innerToOuter, jump)
		current = jump
	}

	hops := make([]model.Host, 0, len(innerToOuter))
	for index := len(innerToOuter) - 1; index >= 0; index-- {
		hops = append(hops, innerToOuter[index])
	}
	return hops, nil
}

func (c *Connector) resolveProxy(
	ctx context.Context,
	host model.Host,
	proxyPassword []byte,
) (*sshclient.ProxySpec, func(), error) {

	if proxyID := strings.TrimSpace(host.ProxyID); proxyID != "" {
		return c.resolveLibraryProxy(ctx, proxyID)
	}
	return c.resolveInlineProxy(ctx, host, proxyPassword)
}

func (c *Connector) resolveLibraryProxy(
	ctx context.Context,
	proxyID string,
) (*sshclient.ProxySpec, func(), error) {
	source, ok := c.hosts.(ProxySource)
	if !ok {

		return nil, nil, errno.New(errno.CodeConnFailed, "session.proxy_not_found", "")
	}
	record, err := source.ProxyByID(ctx, proxyID)
	if err != nil {

		if ctx != nil && ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, errno.New(errno.CodeConnFailed, "session.proxy_not_found", "")
	}
	mode := strings.TrimSpace(record.Mode)
	proxyHost := strings.TrimSpace(record.Host)

	if !model.ValidProxyMode(mode) || mode == model.ProxyModeNone ||
		proxyHost == "" || record.Port < 1 || record.Port > 65535 {
		return nil, nil, errno.New(errno.CodeInvalidArg, "session.proxy_invalid", "")
	}
	if record.PasswordConfigured && strings.TrimSpace(record.Username) == "" {
		return nil, nil, errno.New(errno.CodeInvalidArg, "session.proxy_invalid", "")
	}
	spec := &sshclient.ProxySpec{
		Mode:     mode,
		Host:     proxyHost,
		Port:     record.Port,
		Username: strings.TrimSpace(record.Username),
	}
	if record.PasswordConfigured {
		return c.resolveProxyPassword(ctx, spec, record.ID)
	}
	return spec, nil, nil
}

func (c *Connector) resolveInlineProxy(
	ctx context.Context,
	host model.Host,
	proxyPassword []byte,
) (*sshclient.ProxySpec, func(), error) {
	mode := ""
	if host.ProxyMode != nil {
		mode = strings.TrimSpace(*host.ProxyMode)
	}
	if mode == "" || mode == model.ProxyModeNone {
		return nil, nil, nil
	}
	if !model.ValidProxyMode(mode) {
		return nil, nil, errno.New(errno.CodeInvalidArg, "session.proxy_invalid", "")
	}
	proxyHost := ""
	if host.ProxyHost != nil {
		proxyHost = strings.TrimSpace(*host.ProxyHost)
	}
	proxyPort := 0
	if host.ProxyPort != nil {
		proxyPort = *host.ProxyPort
	}
	if proxyHost == "" || proxyPort < 1 || proxyPort > 65535 {
		return nil, nil, errno.New(errno.CodeInvalidArg, "session.proxy_invalid", "")
	}
	spec := &sshclient.ProxySpec{Mode: mode, Host: proxyHost, Port: proxyPort}
	if host.ProxyUsername != nil {
		spec.Username = strings.TrimSpace(*host.ProxyUsername)
	}
	if (host.ProxyPasswordConfigured || len(proxyPassword) > 0) && spec.Username == "" {
		return nil, nil, errno.New(errno.CodeInvalidArg, "session.proxy_invalid", "")
	}
	if len(proxyPassword) > 0 {
		return c.resolveProxySecret(ctx, spec, func(use func([]byte) error) error {
			return use(proxyPassword)
		})
	}
	if host.ProxyPasswordConfigured {
		source, ok := c.hosts.(HostProxyPasswordSource)
		if !ok || source == nil || strings.TrimSpace(host.ID) == "" {
			return nil, nil, errno.New(errno.CodePermissionDenied, "session.proxy_secret_unavailable", "")
		}
		return c.resolveProxySecret(ctx, spec, func(use func([]byte) error) error {
			return source.UseHostProxyPassword(ctx, host.ID, use)
		})
	}
	return spec, nil, nil
}

func (c *Connector) resolveProxyPassword(
	ctx context.Context,
	spec *sshclient.ProxySpec,
	proxyID string,
) (*sshclient.ProxySpec, func(), error) {
	source, ok := c.hosts.(ProxyPasswordSource)
	if !ok || source == nil {
		return nil, nil, errno.New(errno.CodePermissionDenied, "session.proxy_secret_unavailable", "")
	}
	return c.resolveProxySecret(ctx, spec, func(use func([]byte) error) error {
		return source.UseProxyPassword(ctx, proxyID, use)
	})
}

func (c *Connector) resolveProxySecret(ctx context.Context, spec *sshclient.ProxySpec, read func(func([]byte) error) error) (*sshclient.ProxySpec, func(), error) {
	var secret []byte
	err := read(func(value []byte) error {
		if len(value) == 0 {
			return errno.New(errno.CodePermissionDenied, "session.proxy_secret_unavailable", "")
		}
		secret = bytes.Clone(value)
		return nil
	})
	if err != nil {
		clear(secret)
		if ctx != nil && ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, errno.New(errno.CodePermissionDenied, "session.proxy_secret_unavailable", "")
	}
	if len(secret) == 0 {
		return nil, nil, errno.New(errno.CodePermissionDenied, "session.proxy_secret_unavailable", "")
	}
	spec.Secret = secret
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() {
			clear(secret)
			spec.Secret = nil
		})
	}
	return spec, release, nil
}

func (c *Connector) resolveHopAuth(
	ctx context.Context,
	hop model.Host,
	index int,
	auth routeAuthContext,
) (xssh.AuthMethod, *interactivePassword, error) {
	if hop.CredentialID != "" {
		if c.credentials == nil {
			return nil, nil, errno.New(errno.CodeSessionClosed, "session.connector_not_ready", "")
		}
		var method xssh.AuthMethod
		var reusablePassword *interactivePassword
		err := c.credentials.UseCredentialSecret(ctx, hop.CredentialID, func(kind model.CredentialKind, secret []byte) error {
			converted, convertErr := auxiliaryAuthMethod(kind, secret)
			if convertErr != nil {
				return convertErr
			}
			method = converted
			if kind == model.CredentialKindPassword {
				reusablePassword = newInteractivePassword(secret)
			}
			return nil
		})
		if err != nil {
			return nil, nil, decorateHopAuthError(err, index)
		}
		return method, reusablePassword, nil
	}
	if hop.PasswordConfigured && c.hosts != nil {
		var method xssh.AuthMethod
		var reusablePassword *interactivePassword
		err := c.hosts.UseHostPassword(ctx, hop.ID, func(secret []byte) error {
			if len(secret) == 0 {
				return errno.New(errno.CodeInvalidArg, "session.invalid_authentication", "")
			}

			method = xssh.Password(string(secret))
			reusablePassword = newInteractivePassword(secret)
			return nil
		})
		if err != nil {
			return nil, nil, decorateHopAuthError(err, index)
		}
		return method, reusablePassword, nil
	}
	if hop.PasswordSource == "prompt" && auth.sessionID != "" {
		reusablePassword := &interactivePassword{}
		method, err := c.hopPromptAuth(ctx, hop, index, auth, reusablePassword)
		return method, reusablePassword, err
	}

	return nil, nil, errno.New(errno.CodeInvalidArg, "session.credential_unlinked", "")
}

func (c *Connector) hopPromptAuth(
	ctx context.Context,
	hop model.Host,
	index int,
	auth routeAuthContext,
	reusablePassword *interactivePassword,
) (xssh.AuthMethod, error) {
	request := &authenticationRequest{
		challenge: AuthenticationChallenge{
			SessionID: auth.sessionID,
			Address:   hop.Address,
			Username:  hop.Username,
			RequestID: newAuthenticationRequestID(auth.sessionID),
			Kind:      authenticationKindCredential,
			HostID:    hop.ID,
			HopIndex:  index,
			HopHostID: hop.ID,
		},
		input: make(chan AuthenticationInput, 1),
	}
	c.mu.Lock()
	c.pendingAuth[auth.sessionID] = request
	c.mu.Unlock()
	defer func() {
		c.removePendingAuthentication(auth.sessionID, request)
	}()
	if auth.reportState != nil {

		if err := auth.reportState(model.SessionStateAwaitingAuth); err != nil &&
			!errors.Is(err, model.ErrInvalidSessionTransition) {
			return nil, err
		}
	}
	var input AuthenticationInput
	select {
	case input = <-request.input:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if input.CredentialID != "" {
		if c.credentials == nil {
			return nil, errno.New(errno.CodeSessionClosed, "session.connector_not_ready", "")
		}
		var method xssh.AuthMethod
		err := c.credentials.UseCredentialSecret(ctx, input.CredentialID, func(kind model.CredentialKind, secret []byte) error {
			converted, convertErr := auxiliaryAuthMethod(kind, secret)
			if convertErr != nil {
				return convertErr
			}
			method = converted
			if kind == model.CredentialKindPassword {
				reusablePassword.value = string(secret)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		if input.Remember {
			if rememberer, ok := c.hosts.(hostCredentialRememberer); ok {
				_ = rememberer.RememberHostCredential(ctx, hop.ID, input.CredentialID)
			}
		}
		return method, nil
	}
	secret := []byte(input.Secret)
	input.Secret = ""
	if len(secret) == 0 {
		return nil, errno.New(errno.CodeInvalidArg, "session.invalid_authentication", "")
	}
	method := xssh.Password(string(secret))
	if reusablePassword != nil {
		reusablePassword.value = string(secret)
	}
	if input.Remember {
		if rememberer, ok := c.hosts.(hostPasswordRememberer); ok {
			_ = rememberer.RememberHostPassword(ctx, hop.ID, secret)
		}
	}
	clear(secret)
	return method, nil
}

func sessionRouteFailureKey(err error) string {
	if appErr := errno.Extract(err); appErr != nil {
		return appErr.MessageKey
	}
	if err == nil {
		return ""
	}
	return SessionConnectionFailureKey(err)
}

func hopTarget(host model.Host) sshclient.Target {
	return sshclient.Target{
		Address:  host.Address,
		Port:     host.Port,
		Username: host.Username,

		Timeout: hostConnectTimeout(host),
	}
}

func hostConnectTimeout(host model.Host) time.Duration {
	return time.Duration(host.ConnectTimeoutSeconds) * time.Second
}

func jumpHostID(host model.Host) string {
	if host.JumpHostID == nil {
		return ""
	}
	return strings.TrimSpace(*host.JumpHostID)
}

func decorateHopAuthError(err error, index int) error {
	if err == nil || errno.Extract(err) != nil {
		return err
	}
	return &sshclient.ConnectError{Stage: sshclient.StageJump, Hop: index, Err: err}
}

func (c *Connector) routeTargetAuth(ctx context.Context, host model.Host) (xssh.AuthMethod, error) {
	if c == nil {
		return nil, errno.New(errno.CodeSessionClosed, "session.connector_not_ready", "")
	}
	if host.CredentialID != "" {
		if c.credentials == nil {
			return nil, errno.New(errno.CodeSessionClosed, "session.connector_not_ready", "")
		}
		var method xssh.AuthMethod
		err := c.credentials.UseCredentialSecret(ctx, host.CredentialID, func(kind model.CredentialKind, secret []byte) error {
			converted, convertErr := auxiliaryAuthMethod(kind, secret)
			if convertErr != nil {
				return convertErr
			}
			method = converted
			return nil
		})
		if err != nil {
			return nil, err
		}
		return method, nil
	}
	if host.PasswordConfigured && c.hosts != nil {
		var method xssh.AuthMethod
		err := c.hosts.UseHostPassword(ctx, host.ID, func(secret []byte) error {
			if len(secret) == 0 {
				return errno.New(errno.CodeInvalidArg, "session.invalid_authentication", "")
			}
			method = xssh.Password(string(secret))
			return nil
		})
		if err != nil {
			return nil, err
		}
		return method, nil
	}
	return nil, errno.New(errno.CodeInvalidArg, "session.credential_required", "")
}

func hopDisplayLabel(hop model.Host) string {
	if name := strings.TrimSpace(hop.Name); name != "" {
		return name
	}
	return net.JoinHostPort(strings.TrimSpace(hop.Address), strconv.Itoa(hop.Port))
}
