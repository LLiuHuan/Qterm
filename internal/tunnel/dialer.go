package tunnel

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type DialerFunc func(ctx context.Context, network, address string) (net.Conn, error)

type interactiveWaitContextKey struct{}

type InteractiveWait struct {
	mu         sync.Mutex
	conn       net.Conn
	cancel     context.CancelFunc
	timer      *time.Timer
	deadline   time.Time
	remaining  time.Duration
	generation uint64
	configured bool
	paused     bool
}

func WithInteractiveWait(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, interactiveWaitContextKey{}, &InteractiveWait{})
}

func InteractiveWaitFromContext(ctx context.Context) *InteractiveWait {
	if ctx == nil {
		return nil
	}
	wait, _ := ctx.Value(interactiveWaitContextKey{}).(*InteractiveWait)
	return wait
}

func (w *InteractiveWait) configure(conn net.Conn, timeout time.Duration, cancel context.CancelFunc) {
	if w == nil || conn == nil || timeout <= 0 {
		return
	}
	w.mu.Lock()
	if w.timer != nil {
		w.timer.Stop()
	}
	w.generation++
	generation := w.generation
	w.conn = conn
	w.cancel = cancel
	w.remaining = timeout
	w.deadline = time.Now().Add(timeout)
	w.configured = true
	w.paused = false
	w.timer = time.AfterFunc(timeout, func() { w.expire(generation) })
	deadline := w.deadline
	w.mu.Unlock()
	_ = conn.SetDeadline(deadline)
}

func (w *InteractiveWait) expire(generation uint64) {
	if w == nil {
		return
	}
	w.mu.Lock()
	if !w.configured || w.paused || w.generation != generation {
		w.mu.Unlock()
		return
	}
	w.configured = false
	w.timer = nil
	cancel := w.cancel
	w.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (w *InteractiveWait) Begin() bool {
	if w == nil {
		return false
	}
	w.mu.Lock()
	if !w.configured {
		w.mu.Unlock()
		return false
	}
	if w.paused {
		w.mu.Unlock()
		return true
	}
	w.remaining = time.Until(w.deadline)
	if w.remaining <= 0 {
		w.configured = false
		w.generation++
		cancel := w.cancel
		w.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		return false
	}
	w.paused = true
	if w.timer != nil {
		w.timer.Stop()
	}

	w.generation++
	conn := w.conn
	w.mu.Unlock()
	if conn != nil {
		_ = conn.SetDeadline(time.Time{})
	}
	return true
}

func (w *InteractiveWait) End() {
	if w == nil {
		return
	}
	w.mu.Lock()
	if !w.configured || !w.paused {
		w.mu.Unlock()
		return
	}
	if w.remaining <= 0 {
		w.configured = false
		w.generation++
		cancel := w.cancel
		w.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		return
	}
	w.paused = false
	w.deadline = time.Now().Add(w.remaining)
	deadline := w.deadline
	if w.timer != nil {
		w.timer.Stop()
	}
	w.generation++
	generation := w.generation
	w.timer = time.AfterFunc(w.remaining, func() { w.expire(generation) })
	conn := w.conn
	w.mu.Unlock()
	if conn != nil {
		_ = conn.SetDeadline(deadline)
	}
}

func (w *InteractiveWait) clear() {
	if w == nil {
		return
	}
	w.mu.Lock()
	if w.timer != nil {
		w.timer.Stop()
	}
	w.generation++
	conn := w.conn
	w.conn = nil
	w.cancel = nil
	w.timer = nil
	w.configured = false
	w.paused = false
	w.mu.Unlock()
	if conn != nil {
		_ = conn.SetDeadline(time.Time{})
	}
}

func NewConnectorWithDialer(hostKey ssh.HostKeyCallback, dial DialerFunc) *Connector {
	return newConnector(dial, hostKey, defaultConnectTimeout)
}

var hostKeyAlgorithms = []string{
	ssh.KeyAlgoED25519,
	ssh.KeyAlgoECDSA256,
	ssh.KeyAlgoECDSA384,
	ssh.KeyAlgoECDSA521,
	ssh.KeyAlgoRSASHA512,
	ssh.KeyAlgoRSASHA256,
	ssh.KeyAlgoRSA,
}

func (c *Connector) connectOver(
	ctx context.Context,
	target Target,
	auth ssh.AuthMethod,
	dial DialerFunc,
) (*Connection, error) {
	return c.connectOverMethods(ctx, target, []ssh.AuthMethod{auth}, false, dial)
}

func (c *Connector) connectOverMethods(
	ctx context.Context,
	target Target,
	authMethods []ssh.AuthMethod,
	interactive bool,
	dial DialerFunc,
) (*Connection, error) {
	if ctx == nil {
		return nil, &ConnectError{Stage: StageValidation, Err: errors.New("SSH context is nil")}
	}
	address, username, err := normalizeTarget(target)
	if err != nil {
		return nil, &ConnectError{Stage: StageValidation, Err: err}
	}
	if c == nil || dial == nil || c.timeout <= 0 {
		return nil, &ConnectError{Stage: StageValidation, Err: ErrInvalidTarget}
	}
	if c.hostKey == nil {
		return nil, &ConnectError{Stage: StageValidation, Err: ErrHostKeyVerifierRequired}
	}
	filteredAuth := make([]ssh.AuthMethod, 0, len(authMethods))
	for _, auth := range authMethods {
		if auth != nil {
			filteredAuth = append(filteredAuth, auth)
		}
	}
	if len(filteredAuth) == 0 {
		return nil, &ConnectError{Stage: StageValidation, Err: ErrInvalidAuthentication}
	}

	timeout := c.timeout
	if target.Timeout > 0 {
		timeout = target.Timeout
	}
	connectCtx := ctx
	var cancel context.CancelFunc
	if interactive {
		connectCtx, cancel = context.WithCancel(ctx)
	} else {
		connectCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	dialCtx := connectCtx
	if interactive || timeout > tcpProbeTimeout {
		var cancelDial context.CancelFunc
		dialTimeout := tcpProbeTimeout
		if timeout < dialTimeout {
			dialTimeout = timeout
		}
		dialCtx, cancelDial = context.WithTimeout(ctx, dialTimeout)
		defer cancelDial()
	}
	netConn, err := dial(dialCtx, "tcp", address)
	if err != nil {
		return nil, connectError(dialCtx, StageTCP, err)
	}
	owned := false
	defer func() {
		if !owned {
			_ = netConn.Close()
		}
	}()
	interactiveWait := InteractiveWaitFromContext(ctx)
	if interactive {
		if interactiveWait == nil {
			interactiveWait = &InteractiveWait{}
		}
		interactiveWait.configure(netConn, timeout, cancel)
		defer interactiveWait.clear()
	}

	if deadline, ok := connectCtx.Deadline(); ok && !interactive {

		_ = netConn.SetDeadline(deadline)
	}

	stopCancellation := context.AfterFunc(connectCtx, func() { _ = netConn.Close() })
	config := &ssh.ClientConfig{
		User:              username,
		Auth:              filteredAuth,
		HostKeyCallback:   c.hostKey,
		HostKeyAlgorithms: hostKeyAlgorithms,
	}
	clientConn, channels, requests, err := ssh.NewClientConn(netConn, address, config)
	if !stopCancellation() {
		if contextErr := connectCtx.Err(); contextErr != nil {
			return nil, &ConnectError{Stage: StageHandshake, Err: contextErr}
		}
	}
	if err != nil {
		return nil, connectError(connectCtx, StageHandshake, err)
	}

	_ = netConn.SetDeadline(time.Time{})
	if err := connectCtx.Err(); err != nil {
		_ = clientConn.Close()
		return nil, &ConnectError{Stage: StageHandshake, Err: err}
	}

	owned = true
	connection := &Connection{client: ssh.NewClient(clientConn, channels, requests)}
	connection.startKeepalive()
	return connection, nil
}

func (c *Connector) connect(
	ctx context.Context,
	target Target,
	auth ssh.AuthMethod,
) (*Connection, error) {
	return c.connectOver(ctx, target, auth, c.dialContext)
}

func (c *Connector) ConnectAuthMethods(
	ctx context.Context,
	target Target,
	authMethods []ssh.AuthMethod,
	interactive bool,
) (*Connection, error) {
	return c.connectOverMethods(ctx, target, authMethods, interactive, c.dialContext)
}
