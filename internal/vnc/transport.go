package vnc

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"changeme/internal/model"
)

const (
	webSocketSubprotocol = "binary"

	webSocketHandshakeTimeout = 10 * time.Second

	webSocketReadLimit = -1
)

type WebSocketDialRequest struct {
	Endpoint string

	Path string

	AllowInsecureTLS bool

	BasicUser     string
	BasicPassword string
}

type WebSocketHandshake struct {
	ServerHeader string

	Subprotocol string

	Realm string
}

type WebSocketDialer interface {
	DialWebSocket(ctx context.Context, request WebSocketDialRequest) (net.Conn, string, WebSocketHandshake, error)
}

type coderWebSocketDialer struct{}

func (coderWebSocketDialer) DialWebSocket(
	ctx context.Context,
	request WebSocketDialRequest,
) (net.Conn, string, WebSocketHandshake, error) {
	path := request.Path

	if path == "" {
		path = "/"
	}

	validated, err := model.NormalizeVncWsPath(path)
	if err != nil {
		return nil, "", WebSocketHandshake{}, fmt.Errorf("%w: %v", ErrInvalidTarget, err)
	}
	path = validated
	endpoint := strings.TrimSpace(request.Endpoint)
	if endpoint == "" {
		return nil, "", WebSocketHandshake{}, fmt.Errorf("%w: empty endpoint", ErrInvalidTarget)
	}
	target := "wss://" + endpoint + path

	header := http.Header{}

	header.Set("Origin", "https://"+endpoint)

	if request.BasicUser != "" || request.BasicPassword != "" {
		header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString(
			[]byte(request.BasicUser+":"+request.BasicPassword)))
	}

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if request.AllowInsecureTLS {

		tlsConfig.InsecureSkipVerify = true
	}
	transport := &http.Transport{TLSClientConfig: tlsConfig, Proxy: nil}

	capture := &handshakeHeaderCapture{inner: transport}
	client := &http.Client{Transport: capture}

	dialCtx, cancel := context.WithTimeout(ctx, webSocketHandshakeTimeout)
	defer cancel()

	conn, response, err := websocket.Dial(dialCtx, target, &websocket.DialOptions{
		HTTPClient:   client,
		HTTPHeader:   header,
		Subprotocols: []string{webSocketSubprotocol},
	})
	if err != nil {
		transport.CloseIdleConnections()
		return nil, "", capture.identity(), classifyWebSocketDialError(err, response, endpoint)
	}
	conn.SetReadLimit(webSocketReadLimit)

	fingerprint := ""
	if response != nil && response.TLS != nil && len(response.TLS.PeerCertificates) > 0 {
		fingerprint = certFingerprint(response.TLS.PeerCertificates[0])
	}

	return websocket.NetConn(context.Background(), conn, websocket.MessageBinary), fingerprint, capture.identity(), nil
}

type handshakeHeaderCapture struct {
	inner http.RoundTripper
	mu    sync.Mutex

	status int
	header http.Header
}

func (c *handshakeHeaderCapture) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := c.inner.RoundTrip(request)
	if response != nil {
		c.mu.Lock()
		c.status = response.StatusCode

		c.header = response.Header.Clone()
		c.mu.Unlock()
	}
	return response, err
}

func (c *handshakeHeaderCapture) identity() WebSocketHandshake {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.header == nil {
		return WebSocketHandshake{}
	}
	return WebSocketHandshake{
		ServerHeader: strings.TrimSpace(c.header.Get("Server")),
		Subprotocol:  strings.TrimSpace(c.header.Get("Sec-WebSocket-Protocol")),
		Realm:        strings.TrimSpace(c.header.Get("WWW-Authenticate")),
	}
}

func classifyWebSocketDialError(err error, response *http.Response, endpoint string) error {

	if isTLSCertificateError(err) {
		return fmt.Errorf("%w: %s: %v", ErrTLSCertificate, endpoint, err)
	}
	if response != nil && response.StatusCode == http.StatusUnauthorized {

		return fmt.Errorf("%w: %s: HTTP 401", ErrBasicAuthRejected, endpoint)
	}
	if response != nil {

		return fmt.Errorf("%w: %s: HTTP %d", ErrHandshakeRejected, endpoint, response.StatusCode)
	}
	return fmt.Errorf("%w: %s: %v", ErrHandshakeRejected, endpoint, err)
}

func isTLSCertificateError(err error) bool {
	var certVerifyErr *tls.CertificateVerificationError
	if errors.As(err, &certVerifyErr) {
		return true
	}
	var unknownAuthorityErr x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthorityErr) {
		return true
	}
	var hostnameErr x509.HostnameError
	if errors.As(err, &hostnameErr) {
		return true
	}
	var invalidErr x509.CertificateInvalidError
	if errors.As(err, &invalidErr) {
		return true
	}
	return false
}

func certFingerprint(cert *x509.Certificate) string {
	if cert == nil {
		return ""
	}
	sum := sha256.Sum256(cert.Raw)
	encoded := strings.ToUpper(hex.EncodeToString(sum[:]))

	var builder strings.Builder
	builder.Grow(len(encoded) + len(encoded)/2 + 7)
	builder.WriteString("SHA256:")
	for i := 0; i < len(encoded); i += 2 {
		if i > 0 {
			builder.WriteByte(':')
		}
		builder.WriteString(encoded[i : i+2])
	}
	return builder.String()
}
