package tunnel

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	socks5Version = 0x05
	socks5AuthVer = 0x01

	socks5MethodNoAuth   = 0x00
	socks5MethodUserPass = 0x02
	socks5MethodNone     = 0xff

	socks5CommandConnect = 0x01

	socks5ATYPIPv4   = 0x01
	socks5ATYPDomain = 0x03
	socks5ATYPIPv6   = 0x04

	maxProxyResponseBytes = 16 * 1024
)

var (
	ErrProxyInvalid = errors.New("invalid proxy configuration")

	ErrProxyUnreachable = errors.New("proxy is unreachable")

	ErrProxyAuthFailed = errors.New("proxy authentication failed")

	ErrProxyRejected = errors.New("proxy rejected the connection request")
)

type ProxySpec struct {
	Mode     string
	Host     string
	Port     int
	Username string
	Secret   []byte
}

func (s ProxySpec) AddressString() string {
	return net.JoinHostPort(strings.TrimSpace(s.Host), strconv.Itoa(s.Port))
}

func NewProxyDialer(spec ProxySpec) (DialerFunc, error) {
	mode := strings.ToLower(strings.TrimSpace(spec.Mode))
	host := strings.TrimSpace(spec.Host)
	if host == "" || spec.Port < 1 || spec.Port > 65535 {
		return nil, ErrProxyInvalid
	}
	switch mode {
	case "socks5":
		return newSOCKS5Dialer(spec, net.JoinHostPort(host, strconv.Itoa(spec.Port))), nil
	case "http":
		return newHTTPConnectDialer(spec, net.JoinHostPort(host, strconv.Itoa(spec.Port))), nil
	default:

		return nil, ErrProxyInvalid
	}
}

func dialProxy(ctx context.Context, address string) (net.Conn, error) {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("%w: %s: %v", ErrProxyUnreachable, address, err)
	}
	return conn, nil
}

func proxyHandshakeGuard(ctx context.Context, conn net.Conn) (stop func()) {
	stopCancel := context.AfterFunc(ctx, func() { _ = conn.Close() })
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	return func() {
		stopCancel()
		_ = conn.SetDeadline(time.Time{})
	}
}

func newSOCKS5Dialer(spec ProxySpec, proxyAddress string) DialerFunc {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		conn, err := dialProxy(ctx, proxyAddress)
		if err != nil {
			return nil, err
		}
		stop := proxyHandshakeGuard(ctx, conn)

		fail := func(err error) (net.Conn, error) {
			stop()
			_ = conn.Close()
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			return nil, err
		}
		if err := socks5Handshake(conn, spec); err != nil {
			return fail(err)
		}
		if err := socks5Connect(conn, address); err != nil {
			return fail(err)
		}
		stop()
		return conn, nil
	}
}

func socks5Handshake(conn net.Conn, spec ProxySpec) error {
	username := strings.TrimSpace(spec.Username)
	method := byte(socks5MethodNoAuth)
	if username != "" {
		method = socks5MethodUserPass
	}
	if _, err := conn.Write([]byte{socks5Version, 0x01, method}); err != nil {
		return fmt.Errorf("%w: %v", ErrProxyUnreachable, err)
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		return fmt.Errorf("%w: %v", ErrProxyUnreachable, err)
	}
	if reply[0] != socks5Version {
		return fmt.Errorf("%w: unexpected SOCKS version %d", ErrProxyRejected, reply[0])
	}
	switch reply[1] {
	case socks5MethodNoAuth:
		return nil
	case socks5MethodUserPass:
		if username == "" {
			return fmt.Errorf("%w: proxy requires authentication but no username is configured", ErrProxyAuthFailed)
		}
		return socks5UserPassAuth(conn, username, spec.Secret)
	case socks5MethodNone:
		return fmt.Errorf("%w: no acceptable authentication method", ErrProxyAuthFailed)
	default:
		return fmt.Errorf("%w: unsupported authentication method %d", ErrProxyRejected, reply[1])
	}
}

func socks5UserPassAuth(conn net.Conn, username string, secret []byte) error {
	if len(username) > 255 || len(secret) > 255 {

		return fmt.Errorf("%w: credentials are too long for SOCKS5", ErrProxyInvalid)
	}
	request := make([]byte, 0, 3+len(username)+len(secret))
	request = append(request, socks5AuthVer, byte(len(username)))
	request = append(request, username...)
	request = append(request, byte(len(secret)))
	request = append(request, secret...)
	if _, err := conn.Write(request); err != nil {
		return fmt.Errorf("%w: %v", ErrProxyUnreachable, err)
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		return fmt.Errorf("%w: %v", ErrProxyUnreachable, err)
	}
	if reply[1] != 0x00 {
		return fmt.Errorf("%w: username or password rejected", ErrProxyAuthFailed)
	}
	return nil
}

func socks5Connect(conn net.Conn, address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%w: invalid target address", ErrProxyInvalid)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("%w: invalid target port", ErrProxyInvalid)
	}
	request := []byte{socks5Version, socks5CommandConnect, 0x00}

	if ip := net.ParseIP(host); ip != nil {
		if ipv4 := ip.To4(); ipv4 != nil {
			request = append(request, socks5ATYPIPv4)
			request = append(request, ipv4...)
		} else {
			request = append(request, socks5ATYPIPv6)
			request = append(request, ip.To16()...)
		}
	} else {
		if len(host) > 255 {
			return fmt.Errorf("%w: target hostname is too long", ErrProxyInvalid)
		}
		request = append(request, socks5ATYPDomain, byte(len(host)))
		request = append(request, host...)
	}
	request = append(request, byte(port>>8), byte(port))
	if _, err := conn.Write(request); err != nil {
		return fmt.Errorf("%w: %v", ErrProxyUnreachable, err)
	}

	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("%w: %v", ErrProxyUnreachable, err)
	}
	if header[0] != socks5Version {
		return fmt.Errorf("%w: unexpected SOCKS version %d", ErrProxyRejected, header[0])
	}
	switch header[1] {
	case 0x00:

	case 0x02:
		return fmt.Errorf("%w: connection not allowed by ruleset", ErrProxyRejected)
	case 0x03:
		return fmt.Errorf("%w: network unreachable", ErrProxyRejected)
	case 0x04:
		return fmt.Errorf("%w: host unreachable", ErrProxyRejected)
	case 0x05:
		return fmt.Errorf("%w: connection refused by destination", ErrProxyRejected)
	case 0x06:
		return fmt.Errorf("%w: TTL expired", ErrProxyRejected)
	case 0x07:
		return fmt.Errorf("%w: command not supported", ErrProxyRejected)
	case 0x08:
		return fmt.Errorf("%w: address type not supported", ErrProxyRejected)
	default:
		return fmt.Errorf("%w: reply code %d", ErrProxyRejected, header[1])
	}
	var addressLength int
	switch header[3] {
	case socks5ATYPIPv4:
		addressLength = 4
	case socks5ATYPIPv6:
		addressLength = 16
	case socks5ATYPDomain:
		length := make([]byte, 1)
		if _, err := io.ReadFull(conn, length); err != nil {
			return fmt.Errorf("%w: %v", ErrProxyUnreachable, err)
		}
		addressLength = int(length[0])
	default:
		return fmt.Errorf("%w: unexpected address type %d", ErrProxyRejected, header[3])
	}
	if _, err := io.CopyN(io.Discard, conn, int64(addressLength)+2); err != nil {
		return fmt.Errorf("%w: %v", ErrProxyUnreachable, err)
	}
	return nil
}

func newHTTPConnectDialer(spec ProxySpec, proxyAddress string) DialerFunc {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		conn, err := dialProxy(ctx, proxyAddress)
		if err != nil {
			return nil, err
		}
		stop := proxyHandshakeGuard(ctx, conn)
		fail := func(err error) (net.Conn, error) {
			stop()
			_ = conn.Close()
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			return nil, err
		}
		request, err := httpConnectRequest(spec, address)
		if err != nil {
			return fail(err)
		}
		if _, err := conn.Write(request); err != nil {
			return fail(fmt.Errorf("%w: %v", ErrProxyUnreachable, err))
		}

		reader := bufio.NewReaderSize(conn, maxProxyResponseBytes)
		if err := readHTTPConnectResponse(reader); err != nil {
			return fail(err)
		}
		stop()
		return &bufferedConn{Conn: conn, reader: reader}, nil
	}
}

func httpConnectRequest(spec ProxySpec, target string) ([]byte, error) {
	var builder strings.Builder
	builder.WriteString("CONNECT " + target + " HTTP/1.1\r\n")
	builder.WriteString("Host: " + target + "\r\n")
	if username := strings.TrimSpace(spec.Username); username != "" {
		credentials := base64.StdEncoding.EncodeToString([]byte(username + ":" + string(spec.Secret)))
		builder.WriteString("Proxy-Authorization: Basic " + credentials + "\r\n")
	}

	builder.WriteString("Proxy-Connection: Keep-Alive\r\n\r\n")
	request := builder.String()
	if len(request) > maxProxyResponseBytes {
		return nil, fmt.Errorf("%w: CONNECT request is too large", ErrProxyInvalid)
	}
	return []byte(request), nil
}

func readHTTPConnectResponse(reader *bufio.Reader) error {
	total := 0
	status := 0
	first := true
	for {
		line, err := reader.ReadString('\n')
		total += len(line)
		if total > maxProxyResponseBytes {
			return fmt.Errorf("%w: proxy response is too large", ErrProxyRejected)
		}
		if err != nil {
			if errors.Is(err, io.EOF) && status != 0 && !first {

				return fmt.Errorf("%w: proxy closed the response early", ErrProxyRejected)
			}
			return fmt.Errorf("%w: %v", ErrProxyUnreachable, err)
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			break
		}
		if !first {
			continue
		}
		first = false
		fields := strings.Fields(trimmed)
		if len(fields) < 2 || !strings.HasPrefix(strings.ToUpper(fields[0]), "HTTP/") {
			return fmt.Errorf("%w: malformed proxy response", ErrProxyRejected)
		}
		parsed, convErr := strconv.Atoi(fields[1])
		if convErr != nil {
			return fmt.Errorf("%w: malformed proxy status", ErrProxyRejected)
		}
		status = parsed
	}
	switch {
	case status == 200:
		return nil
	case status == 407, status == 401:

		return fmt.Errorf("%w: proxy returned status %d", ErrProxyAuthFailed, status)
	case status == 0:
		return fmt.Errorf("%w: proxy returned no status line", ErrProxyRejected)
	default:
		return fmt.Errorf("%w: proxy returned status %d", ErrProxyRejected, status)
	}
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}
