package cliapi

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"changeme/internal/model"
)

type (
	Hosts interface {
		ListHosts() ([]model.Host, error)
	}

	Sessions interface {
		OpenSession(ctx context.Context, hostID string) (model.Session, error)
		GetSession(ctx context.Context, id string) (model.Session, error)
		ListSessions(ctx context.Context) ([]model.Session, error)
		CloseSession(ctx context.Context, id string) error
	}

	ExecResult struct {
		Output   string
		ExitCode int
	}

	Exec interface {
		RunBackgroundCommand(ctx context.Context, sessionID, command string) (ExecResult, error)
	}

	Files interface {
		ListRemoteDirectory(ctx context.Context, sessionID, path string) ([]model.RemoteFileEntry, error)
		ReadRemoteText(ctx context.Context, sessionID, path string) (model.RemoteFileContent, error)
		WriteRemoteText(ctx context.Context, sessionID, path, contentBase64, expectedHash string) (model.RemoteFileWriteResult, error)
	}
)

type Deps struct {
	Hosts    Hosts
	Sessions Sessions
	Exec     Exec
	Files    Files

	Version string

	PID int

	Shell func(ctx context.Context, sessionID string, columns int, rows int) (ShellChannel, error)

	Locked func() bool
}

type Server struct {
	deps  Deps
	token string

	mu         sync.Mutex
	listener   net.Listener
	httpServer *http.Server
	port       int
	closed     bool

	closeOnce sync.Once
	closeErr  error
	serveDone chan struct{}
}

var ErrServerClosed = errors.New("cliapi: server is closed")

func NewServer(deps Deps) (*Server, error) {
	if deps.Hosts == nil || deps.Sessions == nil || deps.Exec == nil || deps.Files == nil {
		return nil, errors.New("cliapi: hosts, sessions, exec and files are required")
	}
	token, err := newToken()
	if err != nil {
		return nil, err
	}
	if deps.PID == 0 {
		deps.PID = os.Getpid()
	}
	return &Server{deps: deps, token: token, serveDone: make(chan struct{})}, nil
}

func newToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func (s *Server) Token() string { return s.token }

func (s *Server) Start() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, ErrServerClosed
	}
	if s.listener != nil {
		return s.port, nil
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		_ = listener.Close()
		return 0, errors.New("cliapi: unexpected listener address")
	}
	s.listener = listener
	s.port = address.Port
	s.httpServer = &http.Server{
		Handler:           http.HandlerFunc(s.serveHTTP),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		defer close(s.serveDone)

		_ = s.httpServer.Serve(listener)
	}()
	return s.port, nil
}

func (s *Server) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port
}

func (s *Server) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		server := s.httpServer
		listener := s.listener
		s.mu.Unlock()

		if server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

			_ = server.Shutdown(ctx)
			cancel()
			_ = server.Close()
		} else if listener != nil {
			_ = listener.Close()
		}

		if server != nil {
			<-s.serveDone
		}
	})
	return s.closeErr
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "missing or invalid bearer token")
		return
	}
	if s.deps.Locked != nil && s.deps.Locked() {
		writeError(w, http.StatusLocked, CodeAppLocked, "application is locked")
		return
	}
	s.route(w, r)
}

func (s *Server) authorized(r *http.Request) bool {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if len(header) <= len(prefix) || header[:len(prefix)] != prefix {
		return false
	}
	provided := header[len(prefix):]
	return subtle.ConstantTimeCompare([]byte(provided), []byte(s.token)) == 1
}
