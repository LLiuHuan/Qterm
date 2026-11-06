package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type rdpRouteDialer func(ctx context.Context, network, address string) (net.Conn, error)

const rdpRouteBridgeConfirmTimeout = 3 * time.Second

type rdpRouteBridge struct {
	listener net.Listener
	dial     rdpRouteDialer
	target   string

	ctx    context.Context
	cancel context.CancelFunc

	mu     sync.Mutex
	conns  map[net.Conn]struct{}
	closed bool

	wg sync.WaitGroup

	closedCh chan struct{}

	closeOnce sync.Once
	closeErr  error

	dialFailures atomic.Int64

	firstDialOnce sync.Once
	firstDialDone chan struct{}
	firstDialErr  error
}

func startRdpRouteBridge(ctx context.Context, dial rdpRouteDialer, target string) (local net.Addr, closeFn func() error, err error) {
	bridge, local, err := startRdpRouteBridgeObserved(ctx, dial, target)
	if err != nil {
		return nil, nil, err
	}
	return local, bridge.close, nil
}

func startRdpRouteBridgeObserved(ctx context.Context, dial rdpRouteDialer, target string) (*rdpRouteBridge, net.Addr, error) {
	if dial == nil {
		return nil, nil, errors.New("rdp route bridge: dialer is nil")
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, nil, errors.New("rdp route bridge: target is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, nil, fmt.Errorf("rdp route bridge listen: %w", err)
	}

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok || tcpAddr.IP == nil || !tcpAddr.IP.IsLoopback() || !tcpAddr.IP.Equal(net.IPv4(127, 0, 0, 1)) {
		_ = listener.Close()
		return nil, nil, fmt.Errorf("rdp route bridge: listener address %s is not loopback", listener.Addr())
	}

	bridgeCtx, cancel := context.WithCancel(ctx)
	bridge := &rdpRouteBridge{
		listener:      listener,
		dial:          dial,
		target:        target,
		ctx:           bridgeCtx,
		cancel:        cancel,
		conns:         make(map[net.Conn]struct{}),
		closedCh:      make(chan struct{}),
		firstDialDone: make(chan struct{}),
	}
	bridge.wg.Add(1)
	go bridge.acceptLoop()

	bridge.wg.Add(1)
	go func() {
		defer bridge.wg.Done()
		select {
		case <-bridge.ctx.Done():
			bridge.shutdown()
		case <-bridge.closedCh:
		}
	}()
	return bridge, listener.Addr(), nil
}

func (b *rdpRouteBridge) acceptLoop() {
	defer b.wg.Done()
	for {
		conn, err := b.listener.Accept()
		if err != nil {
			return
		}
		if !b.track(conn) {

			_ = conn.Close()
			return
		}
		b.wg.Add(1)
		go b.serve(conn)
	}
}

func (b *rdpRouteBridge) serve(client net.Conn) {
	defer b.wg.Done()
	defer b.untrack(client)
	defer func() { _ = client.Close() }()

	remote, err := b.dial(b.ctx, "tcp", b.target)
	if err != nil {
		b.dialFailures.Add(1)
		b.recordFirstDial(err)
		return
	}
	if !b.track(remote) {
		_ = remote.Close()
		return
	}
	defer b.untrack(remote)
	defer func() { _ = remote.Close() }()
	b.recordFirstDial(nil)

	var copies sync.WaitGroup
	copies.Add(2)
	go func() {
		defer copies.Done()
		_, _ = io.Copy(remote, client)
	}()
	go func() {
		defer copies.Done()
		_, _ = io.Copy(client, remote)
	}()
	copies.Wait()
	_ = client.Close()
	_ = remote.Close()
}

func (b *rdpRouteBridge) track(conn net.Conn) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return false
	}
	b.conns[conn] = struct{}{}
	return true
}

func (b *rdpRouteBridge) untrack(conn net.Conn) {
	b.mu.Lock()
	delete(b.conns, conn)
	b.mu.Unlock()
}

func (b *rdpRouteBridge) recordFirstDial(err error) {
	b.firstDialOnce.Do(func() {
		b.firstDialErr = err
		close(b.firstDialDone)
	})
}

func (b *rdpRouteBridge) waitFirstDial(ctx context.Context, budget time.Duration) (dialErr error, known bool) {
	if b == nil {
		return nil, false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if budget <= 0 {
		budget = rdpRouteBridgeConfirmTimeout
	}
	timer := time.NewTimer(budget)
	defer timer.Stop()
	select {
	case <-b.firstDialDone:
		return b.firstDialErr, true
	case <-ctx.Done():
		return ctx.Err(), true
	case <-timer.C:
		return nil, false
	}
}

func (b *rdpRouteBridge) shutdown() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	listener := b.listener
	conns := make([]net.Conn, 0, len(b.conns))
	for conn := range b.conns {
		conns = append(conns, conn)
	}
	b.mu.Unlock()

	if listener != nil {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			b.mu.Lock()
			if b.closeErr == nil {
				b.closeErr = err
			}
			b.mu.Unlock()
		}
	}
	for _, conn := range conns {
		_ = conn.Close()
	}
}

func (b *rdpRouteBridge) close() error {
	if b == nil {
		return nil
	}
	b.closeOnce.Do(func() {
		b.cancel()
		b.shutdown()
		close(b.closedCh)
		b.wg.Wait()
	})
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closeErr
}

func (b *rdpRouteBridge) dialFailureCount() int64 {
	if b == nil {
		return 0
	}
	return b.dialFailures.Load()
}
