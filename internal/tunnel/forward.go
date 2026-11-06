package tunnel

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

type ForwardKind string

const (
	ForwardLocal ForwardKind = "local"

	ForwardRemote ForwardKind = "remote"

	ForwardDynamic ForwardKind = "dynamic"
)

type ForwardSpec struct {
	Kind       ForwardKind
	ListenHost string
	ListenPort int
	TargetHost string
	TargetPort int
}

type Forward struct {
	spec     ForwardSpec
	listener net.Listener
	cancel   context.CancelFunc
	done     chan struct{}

	mu      sync.Mutex
	closing bool
	conns   map[net.Conn]struct{}
	err     error
}

func (f *Forward) Addr() net.Addr {
	if f == nil || f.listener == nil {
		return nil
	}
	return f.listener.Addr()
}

func (f *Forward) Done() <-chan struct{} {
	if f == nil {
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	return f.done
}

func (f *Forward) Err() error {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.err
}

func (f *Forward) Close() error {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	if f.closing {
		f.mu.Unlock()
		<-f.done
		return nil
	}
	f.closing = true
	connections := make([]net.Conn, 0, len(f.conns))
	for conn := range f.conns {
		connections = append(connections, conn)
	}
	f.mu.Unlock()

	f.cancel()
	if f.listener != nil {
		_ = f.listener.Close()
	}
	for _, conn := range connections {
		_ = conn.Close()
	}
	<-f.done
	return nil
}

func (c *Connection) OpenForward(ctx context.Context, spec ForwardSpec) (*Forward, error) {
	if ctx == nil {
		return nil, errors.New("forward context is nil")
	}
	if c == nil || c.client == nil {
		return nil, errSSHConnectionClosed
	}
	if err := spec.validate(); err != nil {
		return nil, err
	}

	forwardCtx, cancel := context.WithCancel(ctx)
	forward := &Forward{
		spec:   spec,
		cancel: cancel,
		done:   make(chan struct{}),
		conns:  make(map[net.Conn]struct{}),
	}

	var listener net.Listener
	var err error
	switch spec.Kind {
	case ForwardRemote:

		listener, err = c.client.Listen("tcp", spec.listenAddress())
	default:
		listener, err = net.Listen("tcp", spec.listenAddress())
	}
	if err != nil {
		cancel()
		return nil, fmt.Errorf("listen %s: %w", spec.listenAddress(), err)
	}
	forward.listener = listener

	go forward.serve(forwardCtx, c)
	return forward, nil
}

func (c *Connection) Listen(network, address string) (net.Listener, error) {
	if c == nil || c.client == nil {
		return nil, errSSHConnectionClosed
	}
	return c.client.Listen(network, address)
}

func (f *Forward) serve(ctx context.Context, connection *Connection) {
	defer close(f.done)
	go func() {
		<-ctx.Done()
		_ = f.listener.Close()
	}()

	for {
		accepted, err := f.listener.Accept()
		if err != nil {
			f.mu.Lock()
			if !f.closing && ctx.Err() == nil {
				f.err = err
			}
			f.mu.Unlock()
			return
		}
		if !f.track(accepted) {
			_ = accepted.Close()
			return
		}
		go func(client net.Conn) {
			defer func() {
				_ = client.Close()
				f.untrack(client)
			}()
			if err := f.handle(ctx, connection, client); err != nil {

				return
			}
		}(accepted)
	}
}

func (f *Forward) track(conn net.Conn) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closing {
		return false
	}
	f.conns[conn] = struct{}{}
	return true
}

func (f *Forward) untrack(conn net.Conn) {
	f.mu.Lock()
	delete(f.conns, conn)
	f.mu.Unlock()
}

func (f *Forward) handle(ctx context.Context, connection *Connection, client net.Conn) error {
	if f.spec.Kind == ForwardDynamic {
		return serveSOCKS5(ctx, connection, client)
	}
	target := net.JoinHostPort(f.spec.TargetHost, strconv.Itoa(f.spec.TargetPort))
	var remote net.Conn
	var err error
	if f.spec.Kind == ForwardRemote {

		remote, err = (&net.Dialer{}).DialContext(ctx, "tcp", target)
	} else {

		remote, err = connection.DialContext(ctx, "tcp", target)
	}
	if err != nil {
		return err
	}
	defer remote.Close()
	relay(client, remote)
	return nil
}

func relay(a, b net.Conn) {
	var once sync.Once
	closeBoth := func() {
		once.Do(func() {
			_ = a.Close()
			_ = b.Close()
		})
	}
	go func() {
		_, _ = io.Copy(b, a)
		closeBoth()
	}()
	_, _ = io.Copy(a, b)
	closeBoth()
}

func (s ForwardSpec) listenAddress() string {
	host := s.ListenHost
	if host == "" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, strconv.Itoa(s.ListenPort))
}

func (s ForwardSpec) validate() error {
	switch s.Kind {
	case ForwardLocal, ForwardRemote, ForwardDynamic:
	default:
		return fmt.Errorf("unsupported forward kind %q", s.Kind)
	}
	if s.ListenPort < 0 || s.ListenPort > 65535 {
		return errors.New("listen port out of range")
	}
	if s.Kind == ForwardDynamic {
		return nil
	}
	if s.TargetHost == "" {
		return errors.New("forward target host is required")
	}
	if s.TargetPort <= 0 || s.TargetPort > 65535 {
		return errors.New("forward target port out of range")
	}
	return nil
}

const (
	socksVersion5         = 0x05
	socksAuthNone         = 0x00
	socksAuthNoAccept     = 0xff
	socksCmdConnect       = 0x01
	socksAddrIPv4         = 0x01
	socksAddrDomain       = 0x03
	socksAddrIPv6         = 0x04
	socksReplySucceeded   = 0x00
	socksReplyHostUnreach = 0x04
	socksReplyRefused     = 0x05
	socksHandshakeLimit   = 512
)

var errSOCKSRejected = errors.New("SOCKS5 request rejected")

func serveSOCKS5(ctx context.Context, connection *Connection, client net.Conn) error {
	if err := socksNegotiate(client); err != nil {
		return err
	}
	target, err := socksReadConnect(client)
	if err != nil {
		_ = socksWriteReply(client, socksReplyRefused)
		return err
	}
	remote, err := connection.DialContext(ctx, "tcp", target)
	if err != nil {
		_ = socksWriteReply(client, socksReplyHostUnreach)
		return err
	}
	defer remote.Close()
	if err := socksWriteReply(client, socksReplySucceeded); err != nil {
		return err
	}
	relay(client, remote)
	return nil
}

func socksNegotiate(conn net.Conn) error {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return err
	}
	if header[0] != socksVersion5 {
		return fmt.Errorf("unsupported SOCKS version %d", header[0])
	}
	count := int(header[1])
	if count == 0 || count > socksHandshakeLimit {
		return errSOCKSRejected
	}
	methods := make([]byte, count)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return err
	}
	for _, method := range methods {
		if method == socksAuthNone {
			_, err := conn.Write([]byte{socksVersion5, socksAuthNone})
			return err
		}
	}
	_, _ = conn.Write([]byte{socksVersion5, socksAuthNoAccept})
	return errSOCKSRejected
}

func socksReadConnect(conn net.Conn) (string, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", err
	}
	if header[0] != socksVersion5 || header[1] != socksCmdConnect {
		return "", errSOCKSRejected
	}
	var host string
	switch header[3] {
	case socksAddrIPv4:
		raw := make([]byte, 4)
		if _, err := io.ReadFull(conn, raw); err != nil {
			return "", err
		}
		host = net.IP(raw).String()
	case socksAddrIPv6:
		raw := make([]byte, 16)
		if _, err := io.ReadFull(conn, raw); err != nil {
			return "", err
		}
		host = net.IP(raw).String()
	case socksAddrDomain:
		length := make([]byte, 1)
		if _, err := io.ReadFull(conn, length); err != nil {
			return "", err
		}
		if length[0] == 0 {
			return "", errSOCKSRejected
		}
		raw := make([]byte, int(length[0]))
		if _, err := io.ReadFull(conn, raw); err != nil {
			return "", err
		}
		host = string(raw)
	default:
		return "", errSOCKSRejected
	}
	portRaw := make([]byte, 2)
	if _, err := io.ReadFull(conn, portRaw); err != nil {
		return "", err
	}
	port := binary.BigEndian.Uint16(portRaw)
	if port == 0 {
		return "", errSOCKSRejected
	}
	return net.JoinHostPort(host, strconv.Itoa(int(port))), nil
}

func socksWriteReply(conn net.Conn, code byte) error {
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetWriteDeadline(time.Time{})

	reply := []byte{socksVersion5, code, 0x00, socksAddrIPv4, 0, 0, 0, 0, 0, 0}
	_, err := conn.Write(reply)
	return err
}
