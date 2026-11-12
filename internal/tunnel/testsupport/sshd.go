package testsupport

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type Options struct {
	Username string
	Password string

	HostKey ssh.Signer

	DenyForwarding bool

	EnableSFTP bool

	ExecCommands map[string]string
}

type Server struct {
	options  Options
	signer   ssh.Signer
	listener net.Listener
	config   *ssh.ServerConfig

	mu       sync.Mutex
	conns    map[net.Conn]struct{}
	closed   bool
	forwards int
	execs    int

	auths *atomic.Int64
	wg    sync.WaitGroup
}

func Start(t *testing.T, options Options) *Server {
	t.Helper()
	signer := options.HostKey
	if signer == nil {
		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatalf("testsupport: generate host key: %v", err)
		}
		generated, err := ssh.NewSignerFromKey(privateKey)
		if err != nil {
			t.Fatalf("testsupport: build host key signer: %v", err)
		}
		signer = generated
	}
	if options.Username == "" {
		options.Username = "tester"
	}
	auths := &atomic.Int64{}
	config := &ssh.ServerConfig{
		PasswordCallback: func(metadata ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			auths.Add(1)
			if metadata.User() != options.Username || string(password) != options.Password {
				return nil, fmt.Errorf("testsupport: password rejected for %q", metadata.User())
			}
			return nil, nil
		},
	}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("testsupport: listen: %v", err)
	}
	server := &Server{
		options:  options,
		signer:   signer,
		listener: listener,
		config:   config,
		conns:    make(map[net.Conn]struct{}),
		auths:    auths,
	}
	server.wg.Add(1)
	go server.acceptLoop()
	t.Cleanup(server.Close)
	return server
}

func (s *Server) Addr() string {
	if s == nil || s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func (s *Server) Host() string {
	host, _, err := net.SplitHostPort(s.Addr())
	if err != nil {
		return ""
	}
	return host
}

func (s *Server) Port() int {
	_, portText, err := net.SplitHostPort(s.Addr())
	if err != nil {
		return 0
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return 0
	}
	return port
}

func (s *Server) HostKey() ssh.PublicKey {
	if s == nil || s.signer == nil {
		return nil
	}
	return s.signer.PublicKey()
}

func (s *Server) Forwards() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.forwards
}

func (s *Server) PasswordAttempts() int64 {
	if s == nil || s.auths == nil {
		return 0
	}
	return s.auths.Load()
}

func (s *Server) Execs() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.execs
}

func (s *Server) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	conns := make([]net.Conn, 0, len(s.conns))
	for conn := range s.conns {
		conns = append(conns, conn)
	}
	s.mu.Unlock()
	if s.listener != nil {
		_ = s.listener.Close()
	}
	for _, conn := range conns {
		_ = conn.Close()
	}
	s.wg.Wait()
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		if !s.track(conn) {
			_ = conn.Close()
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer s.untrack(conn)
			s.handleConn(conn)
		}()
	}
}

func (s *Server) track(conn net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	s.conns[conn] = struct{}{}
	return true
}

func (s *Server) untrack(conn net.Conn) {
	s.mu.Lock()
	delete(s.conns, conn)
	s.mu.Unlock()
	_ = conn.Close()
}

func (s *Server) handleConn(conn net.Conn) {
	serverConn, channels, requests, err := ssh.NewServerConn(conn, s.config)
	if err != nil {
		return
	}
	defer serverConn.Close()
	go ssh.DiscardRequests(requests)
	for newChannel := range channels {

		s.wg.Add(1)
		go func(channel ssh.NewChannel) {
			defer s.wg.Done()
			switch channel.ChannelType() {
			case "direct-tcpip":
				s.handleDirectTCPIP(channel)
			case "session":
				s.handleSession(channel)
			default:
				_ = channel.Reject(ssh.UnknownChannelType, "testsupport: unsupported channel type")
			}
		}(newChannel)
	}
}

type directTCPIPPayload struct {
	DestinationHost string
	DestinationPort uint32
	OriginHost      string
	OriginPort      uint32
}

func (s *Server) handleDirectTCPIP(newChannel ssh.NewChannel) {
	if s.options.DenyForwarding {

		_ = newChannel.Reject(ssh.Prohibited, "administratively prohibited: forwarding disabled")
		return
	}
	var payload directTCPIPPayload
	if err := ssh.Unmarshal(newChannel.ExtraData(), &payload); err != nil {
		_ = newChannel.Reject(ssh.ConnectionFailed, "testsupport: malformed direct-tcpip payload")
		return
	}
	channel, requests, err := newChannel.Accept()
	if err != nil {
		return
	}
	go ssh.DiscardRequests(requests)
	s.mu.Lock()
	s.forwards++
	s.mu.Unlock()
	destination := net.JoinHostPort(payload.DestinationHost, strconv.Itoa(int(payload.DestinationPort)))
	upstream, err := net.DialTimeout("tcp", destination, 5*time.Second)
	if err != nil {

		_ = channel.Close()
		return
	}
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(upstream, channel)
		if tcp, ok := upstream.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(channel, upstream)
		_ = channel.CloseWrite()
		done <- struct{}{}
	}()
	<-done
	<-done
	_ = channel.Close()
	_ = upstream.Close()
}

func (s *Server) handleSession(newChannel ssh.NewChannel) {
	channel, requests, err := newChannel.Accept()
	if err != nil {
		return
	}
	defer channel.Close()
	for request := range requests {
		switch request.Type {
		case "subsystem":

			var payload struct{ Name string }
			_ = ssh.Unmarshal(request.Payload, &payload)
			if payload.Name != "sftp" || !s.options.EnableSFTP {
				_ = request.Reply(false, nil)
				return
			}
			if err := request.Reply(true, nil); err != nil {
				return
			}
			server, err := sftp.NewServer(channel)
			if err != nil {
				return
			}
			_ = server.Serve()
			_ = server.Close()
			return
		case "exec":
			var payload struct{ Command string }
			if err := ssh.Unmarshal(request.Payload, &payload); err != nil {
				_ = request.Reply(false, nil)
				continue
			}
			if err := request.Reply(true, nil); err != nil {
				return
			}
			s.mu.Lock()
			s.execs++
			s.mu.Unlock()
			s.runExec(channel, payload.Command)
			return
		case "pty-req", "env", "shell":

			_ = request.Reply(false, nil)
			if request.Type == "shell" {
				return
			}
		default:
			_ = request.Reply(false, nil)
		}
	}
}

func (s *Server) runExec(channel ssh.Channel, command string) {
	status := uint32(0)
	switch {
	case s.hasExecOverride(command):
		_, _ = io.WriteString(channel, s.options.ExecCommands[command])
	case strings.HasPrefix(command, "echo "):
		_, _ = io.WriteString(channel, strings.TrimPrefix(command, "echo ")+"\n")
	case command == "uname -s":

		_, _ = io.WriteString(channel, "Linux\n")
	default:
		status = 127
	}
	_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
	_ = channel.CloseWrite()
}

func (s *Server) hasExecOverride(command string) bool {
	_, ok := s.options.ExecCommands[command]
	return ok
}
