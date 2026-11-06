package tunnel

import (
	"context"
	"errors"
	"strings"

	"golang.org/x/crypto/ssh"
)

type Hop struct {
	Target Target
	Auth   ssh.AuthMethod

	AuthMethods []ssh.AuthMethod
	Interactive bool

	Label string
}

type HopConnector struct {
	connector *Connector
	previous  *Connection
	chain     []*Connection
	closed    bool
}

func (c *Connector) NewHopConnector() *HopConnector {
	return &HopConnector{connector: c}
}

func (h *HopConnector) Connect(ctx context.Context, hop Hop) (*Connection, error) {
	if h == nil || h.connector == nil {
		return nil, &ConnectError{Stage: StageValidation, Err: ErrInvalidTarget}
	}
	if h.closed {
		return nil, &ConnectError{Stage: StageValidation, Err: errors.New("SSH hop chain is closed")}
	}
	dial := h.connector.dialContext
	if h.previous != nil {
		dial = h.previous.DialContext
	}
	authMethods := hop.AuthMethods
	if len(authMethods) == 0 && hop.Auth != nil {
		authMethods = []ssh.AuthMethod{hop.Auth}
	}
	connection, err := h.connector.connectOverMethods(ctx, hop.Target, authMethods, hop.Interactive, dial)
	if err != nil {
		return nil, err
	}
	h.previous = connection
	h.chain = append(h.chain, connection)
	return connection, nil
}

func (h *HopConnector) Dialer() DialerFunc {
	if h == nil || h.connector == nil {
		return nil
	}
	if h.previous == nil {
		return h.connector.dialContext
	}
	return h.previous.DialContext
}

func (h *HopConnector) Chain() []*Connection {
	if h == nil {
		return nil
	}
	return append([]*Connection(nil), h.chain...)
}

func (h *HopConnector) Close() {
	if h == nil || h.closed {
		return
	}
	h.closed = true
	for index := len(h.chain) - 1; index >= 0; index-- {
		if h.chain[index] != nil {
			_ = h.chain[index].Close()
		}
	}
	h.chain = nil
	h.previous = nil
}

func (c *Connector) ConnectChain(
	ctx context.Context,
	hops []Hop,
	target Target,
	targetAuth ssh.AuthMethod,
) (*Connection, error) {
	return c.ConnectChainObserved(ctx, hops, target, targetAuth, nil)
}

type HopObserver func(index int, label string)

func (c *Connector) ConnectChainObserved(
	ctx context.Context,
	hops []Hop,
	target Target,
	targetAuth ssh.AuthMethod,
	observe HopObserver,
) (*Connection, error) {
	return c.ConnectChainAuthMethodsObserved(ctx, hops, target, []ssh.AuthMethod{targetAuth}, false, observe)
}

func (c *Connector) ConnectChainAuthMethodsObserved(
	ctx context.Context,
	hops []Hop,
	target Target,
	targetAuth []ssh.AuthMethod,
	interactive bool,
	observe HopObserver,
) (*Connection, error) {
	if ctx == nil {
		return nil, &ConnectError{Stage: StageValidation, Err: errors.New("SSH context is nil")}
	}
	if c == nil || c.dialContext == nil || c.timeout <= 0 {
		return nil, &ConnectError{Stage: StageValidation, Err: ErrInvalidTarget}
	}
	builder := c.NewHopConnector()
	report := func(index int, label string) {
		if observe != nil {
			observe(index, label)
		}
	}
	for index, hop := range hops {
		report(index, hop.Label)
		if _, err := builder.Connect(ctx, hop); err != nil {
			builder.Close()
			return nil, HopConnectError(err, index)
		}
	}

	hopChain := builder.Chain()
	dial := builder.Dialer()
	report(len(hops), "")
	final, err := c.connectOverMethods(ctx, target, targetAuth, interactive, dial)
	if err != nil {
		builder.Close()

		if len(hops) == 0 && IsProxyError(err) {
			return nil, &ConnectError{Stage: StageProxy, Err: err}
		}

		if len(hops) > 0 && IsForwardDenied(err) {
			return nil, &ConnectError{Stage: StageJumpDial, Hop: len(hops) - 1, Err: err}
		}
		return nil, err
	}
	final.adoptChain(hopChain)
	return final, nil
}

func HopConnectError(err error, hop int) error {
	var connectErr *ConnectError
	stage := StageJump
	owner := hop
	if errors.As(err, &connectErr) && connectErr.Stage == StageTCP {
		switch {
		case IsProxyError(err):
			stage = StageProxy
		case hop > 0:
			stage = StageJumpDial
			owner = hop - 1
		}
	}
	return &ConnectError{Stage: stage, Hop: owner, Err: err}
}

func IsForwardDenied(err error) bool {
	if err == nil {
		return false
	}
	var openErr *ssh.OpenChannelError
	if errors.As(err, &openErr) {
		return openErr.Reason == ssh.Prohibited
	}
	return chainContainsText(err, "administratively prohibited")
}

func chainContainsText(err error, needle string) bool {
	for current := err; current != nil; current = errors.Unwrap(current) {
		if strings.Contains(strings.ToLower(current.Error()), needle) {
			return true
		}
	}
	return false
}

func IsProxyError(err error) bool {
	return errors.Is(err, ErrProxyUnreachable) ||
		errors.Is(err, ErrProxyAuthFailed) ||
		errors.Is(err, ErrProxyRejected)
}
