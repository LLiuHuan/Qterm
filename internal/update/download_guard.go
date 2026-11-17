package update

import (
	"errors"
	"net/http"
	"sync"
	"time"
)

const (
	downloadResponseHeaderTimeout = 30 * time.Second

	downloadStallTimeout = 60 * time.Second
)

func downloadTransport(base http.RoundTripper) http.RoundTripper {
	transport, ok := base.(*http.Transport)
	if !ok || transport == nil {
		return base
	}
	clone := transport.Clone()
	if clone.ResponseHeaderTimeout <= 0 {
		clone.ResponseHeaderTimeout = downloadResponseHeaderTimeout
	}
	if clone.TLSHandshakeTimeout <= 0 {
		clone.TLSHandshakeTimeout = downloadResponseHeaderTimeout
	}
	if clone.IdleConnTimeout <= 0 {
		clone.IdleConnTimeout = 90 * time.Second
	}
	return clone
}

var errDownloadStalled = errors.New("download stalled")

type stallGuard struct {
	body    interface{ Read([]byte) (int, error) }
	closer  interface{ Close() error }
	timeout time.Duration

	mu       sync.Mutex
	lastRead time.Time
	closed   bool
	stalled  bool

	stop chan struct{}
}

func newStallGuard(body interface {
	Read([]byte) (int, error)
	Close() error
}, timeout time.Duration) *stallGuard {
	guard := &stallGuard{
		body:     body,
		closer:   body,
		timeout:  timeout,
		lastRead: time.Now(),
		stop:     make(chan struct{}),
	}
	go guard.watch()
	return guard
}

func (g *stallGuard) Read(buffer []byte) (int, error) {
	count, err := g.body.Read(buffer)
	if count > 0 {
		g.mu.Lock()
		g.lastRead = time.Now()
		g.mu.Unlock()
	}
	return count, err
}

func (g *stallGuard) Close() error {
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return nil
	}
	g.closed = true
	g.mu.Unlock()
	close(g.stop)
	return g.closer.Close()
}

func (g *stallGuard) Stalled() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.stalled
}

func (g *stallGuard) watch() {
	interval := g.timeout / 4
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-g.stop:
			return
		case <-ticker.C:
			g.mu.Lock()
			stalled := !g.closed && time.Since(g.lastRead) > g.timeout
			g.mu.Unlock()
			if stalled {
				g.mu.Lock()
				g.stalled = true
				g.mu.Unlock()
				_ = g.closer.Close()
				return
			}
		}
	}
}
