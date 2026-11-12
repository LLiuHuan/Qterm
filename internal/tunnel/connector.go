package tunnel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/ssh"

	"changeme/internal/model"
)

type ResourceSnapshot = model.ResourceSnapshot

func parseUint(value string) uint64 {
	n, _ := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	return n
}
func parseFloat(value string) float64 {
	n, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return n
}

func clampPercent(value float64) float64 {
	if value < 0 || value != value {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

const (
	defaultSSHPort        = 22
	defaultConnectTimeout = 15 * time.Second

	tcpProbeTimeout = 5 * time.Second
	maxSSHFieldSize = 1024
)

var (
	ErrInvalidTarget           = errors.New("invalid SSH target")
	ErrHostKeyVerifierRequired = errors.New("SSH host-key verifier is required")
	ErrInvalidAuthentication   = errors.New("invalid SSH authentication")
	ErrRemoteOSUnsupported     = errors.New("remote operating system is unavailable")
)

type Stage string

const (
	StageValidation Stage = "validation"
	StageTCP        Stage = "tcp"
	StageHandshake  Stage = "handshake"

	StageJump Stage = "jump"

	StageJumpDial Stage = "jump-dial"

	StageProxy Stage = "proxy"
)

type ConnectError struct {
	Stage Stage

	Hop int
	Err error

	Protocol string
}

func (e *ConnectError) Error() string {
	if e == nil {
		return "SSH connection failed"
	}
	protocol := e.Protocol
	if protocol == "" {
		protocol = "SSH"
	}
	return fmt.Sprintf("%s %s failed", protocol, e.Stage)
}

func (e *ConnectError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type Target struct {
	Address  string
	Port     int
	Username string

	Timeout time.Duration
}

type Connector struct {
	dialContext func(context.Context, string, string) (net.Conn, error)
	hostKey     ssh.HostKeyCallback
	timeout     time.Duration
}

type Connection struct {
	client          *ssh.Client
	closeOnce       sync.Once
	closeErr        error
	latencyPending  atomic.Bool
	keepaliveMu     sync.Mutex
	keepaliveCancel context.CancelFunc

	fsMu        sync.Mutex
	fileSystems *FileSystemGroup

	auxiliaryProvisioner ConnectionProvisioner

	chainMu sync.Mutex
	chain   []*Connection

	resources resourceSampler

	processCPU processCPUSampler

	closeTrace func()
}

func (c *Connection) detachAuxiliary() AuxiliaryConnection {
	if c == nil || c.client == nil {
		return AuxiliaryConnection{}
	}
	c.keepaliveMu.Lock()
	cancel := c.keepaliveCancel
	c.keepaliveCancel = nil
	c.keepaliveMu.Unlock()
	if cancel != nil {
		cancel()
	}
	client := c.client

	c.startKeepalive()
	closeFn := func() error {
		var err error
		c.closeOnce.Do(func() {
			c.keepaliveMu.Lock()
			cancel := c.keepaliveCancel
			c.keepaliveCancel = nil
			c.keepaliveMu.Unlock()
			if cancel != nil {
				cancel()
			}
			err = client.Close()
		})
		return err
	}
	return AuxiliaryConnection{Client: client, Close: closeFn}
}

func (c *Connection) SetAuxiliaryProvisioner(provisioner ConnectionProvisioner) {
	if c == nil || provisioner == nil {
		return
	}
	c.fsMu.Lock()
	c.auxiliaryProvisioner = provisioner
	c.fsMu.Unlock()
}

func (c *Connection) SetFileSystemConnections(count int) {
	if c == nil {
		return
	}
	c.fsMu.Lock()
	group := c.fileSystems
	c.fsMu.Unlock()
	if group == nil {

		group = c.fileSystemGroup()
	}
	group.setConnections(count)
}

func (c *Connection) DisableKeepalive() {
	if c == nil {
		return
	}
	c.keepaliveMu.Lock()
	cancel := c.keepaliveCancel
	c.keepaliveMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (c *Connection) startKeepalive() {
	if c == nil || c.client == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.keepaliveMu.Lock()
	c.keepaliveCancel = cancel
	c.keepaliveMu.Unlock()
	go runKeepalive(
		ctx,
		keepaliveInterval,
		keepaliveReplyTimeout,
		keepaliveMaxFailures,
		c.probeKeepalive,
		func() { _ = c.Close() },
	)
}

func (c *Connection) probeKeepalive() error {
	if c == nil || c.client == nil {
		return errKeepaliveNotReady
	}
	_, _, err := c.client.SendRequest(keepaliveRequestName, true, nil)
	return err
}

const (
	remoteOSProbeOutputLimit = 4096
	resourceProbeOutputLimit = 64 * 1024

	remoteOSProbeTimeout = 4 * time.Second
)

func (c *Connection) DetectRemoteOS(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", errors.New("remote OS context is nil")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if c == nil || c.client == nil {
		return "", ErrRemoteOSUnsupported
	}
	for _, command := range []string{remoteOSProbeUnixCommand, remoteOSProbeWindowsCIM, remoteOSProbeWindowsVer} {
		output, err := c.runOSProbe(ctx, command)
		if err != nil {

			if ctxErr := ctx.Err(); ctxErr != nil {
				return "", ctxErr
			}
			continue
		}
		icon, ok := remoteOSIcon(output)
		if !ok {
			continue
		}
		return c.refineRemoteOSIcon(ctx, icon, output), nil
	}
	return "", ErrRemoteOSUnsupported
}

func (c *Connection) runOSProbe(ctx context.Context, command string) (string, error) {
	probeCtx, cancel := context.WithTimeout(ctx, remoteOSProbeTimeout)
	defer cancel()
	return c.runBoundedProbe(probeCtx, command)
}

func (c *Connection) refineRemoteOSIcon(ctx context.Context, icon string, platformOutput string) string {
	switch icon {
	case "linux":

		if release, err := c.runOSProbeRelease(ctx); err == nil {
			return linuxDistributionIcon(release)
		}
		return icon
	case "macos":
		if output, err := c.runOSProbe(ctx, remoteOSProbeMacOSVer); err == nil {
			if versioned := macosVersionIcon(output); versioned != "" {
				return versioned
			}
		}
		return icon
	case "windows":

		if versioned := windowsIconForOutput(platformOutput); versioned != "" {
			return versioned
		}
		if output, err := c.runOSProbe(ctx, remoteOSProbeWindowsCIM); err == nil {
			if versioned := windowsIconForOutput(output); versioned != "" {
				return versioned
			}
		}
		return icon
	default:
		return icon
	}
}

func (c *Connection) runOSProbeRelease(ctx context.Context) (string, error) {
	return c.runOSProbe(ctx, osReleaseProbeCommand)
}

func (c *Connection) MeasureLatency(ctx context.Context) (time.Duration, error) {
	if ctx == nil {
		return 0, errors.New("latency context is nil")
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if c == nil || c.client == nil {
		return 0, errors.New("SSH connection is not ready")
	}

	if !c.latencyPending.CompareAndSwap(false, true) {
		return 0, errors.New("latency request is pending")
	}
	result := make(chan error, 1)
	started := time.Now()
	go func() {
		defer c.latencyPending.Store(false)

		_, _, err := c.client.SendRequest("keepalive@openssh.com", true, nil)
		result <- err
	}()
	select {
	case err := <-result:
		if err != nil {
			return 0, err
		}
		return time.Since(started), nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (c *Connection) runBoundedProbe(ctx context.Context, command string) (string, error) {
	return c.runProbe(ctx, command, remoteOSProbeOutputLimit)
}

func (c *Connection) runProbe(ctx context.Context, command string, outputLimit int) (string, error) {
	output, _, err := c.runProbeWithStderr(ctx, command, outputLimit)
	return output, err
}

func (c *Connection) runProbeWithStderr(ctx context.Context, command string, outputLimit int) (string, string, error) {
	if ctx == nil {
		return "", "", errors.New("remote probe context is nil")
	}
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	session, err := c.openProbeSession(ctx)
	if err != nil {
		return "", "", err
	}
	return c.runProbeSession(ctx, session, command, outputLimit)
}

func (c *Connection) openProbeSession(ctx context.Context) (*ssh.Session, error) {
	if c == nil || c.client == nil {
		return nil, ErrRemoteOSUnsupported
	}

	type sessionResult struct {
		session *ssh.Session
		err     error
	}

	result := make(chan sessionResult)
	openCtx, cancel := context.WithTimeout(ctx, remoteOSProbeTimeout)
	defer cancel()
	go func() {
		session, err := c.client.NewSession()
		select {
		case result <- sessionResult{session: session, err: err}:
		case <-openCtx.Done():
			if session != nil {
				_ = session.Close()
			}
		}
	}()
	select {
	case opened := <-result:
		if err := ctx.Err(); err != nil {
			if opened.session != nil {
				_ = opened.session.Close()
			}
			return nil, err
		}
		if opened.err != nil {
			return nil, opened.err
		}
		if opened.session == nil {
			return nil, ErrRemoteOSUnsupported
		}
		return opened.session, nil
	case <-openCtx.Done():
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, openCtx.Err()
	}
}

func (c *Connection) runProbeSession(ctx context.Context, session *ssh.Session, command string, outputLimit int) (string, string, error) {
	if ctx == nil {
		return "", "", errors.New("remote probe context is nil")
	}
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	if session == nil {
		return "", "", ErrRemoteOSUnsupported
	}
	defer func() { _ = session.Close() }()
	stdout := boundedProbeBuffer{limit: outputLimit}
	stderr := boundedProbeBuffer{limit: outputLimit}
	session.Stdout = &stdout
	session.Stderr = &stderr
	stop := context.AfterFunc(ctx, func() { _ = session.Close() })
	defer stop()
	if err := session.Run(command); err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return stdout.String(), stderr.String(), contextErr
		}
		return stdout.String(), stderr.String(), err
	}
	return stdout.String(), stderr.String(), nil
}

type boundedProbeBuffer struct {
	data  []byte
	limit int
}

func (b *boundedProbeBuffer) Write(value []byte) (int, error) {
	limit := b.limit
	if limit <= 0 {
		limit = remoteOSProbeOutputLimit
	}
	remaining := limit - len(b.data)
	if remaining > 0 {
		if remaining > len(value) {
			remaining = len(value)
		}
		b.data = append(b.data, value[:remaining]...)
	}
	return len(value), nil
}

func (b *boundedProbeBuffer) String() string { return string(b.data) }

func NewConnector(hostKey ssh.HostKeyCallback) *Connector {
	dialer := &net.Dialer{}
	return newConnector(dialer.DialContext, hostKey, defaultConnectTimeout)
}

func newConnector(
	dialContext func(context.Context, string, string) (net.Conn, error),
	hostKey ssh.HostKeyCallback,
	timeout time.Duration,
) *Connector {
	return &Connector{dialContext: dialContext, hostKey: hostKey, timeout: timeout}
}

func (c *Connector) ConnectPassword(
	ctx context.Context,
	target Target,
	password []byte,
) (*Connection, error) {
	secret := bytes.Clone(password)
	defer clear(secret)
	if len(secret) == 0 {
		return nil, &ConnectError{Stage: StageValidation, Err: ErrInvalidAuthentication}
	}

	auth := ssh.PasswordCallback(func() (string, error) {
		return string(secret), nil
	})
	return c.connect(ctx, target, auth)
}

func (c *Connector) ConnectPrivateKey(
	ctx context.Context,
	target Target,
	privateKey []byte,
	passphrase []byte,
) (*Connection, error) {
	keyCopy := bytes.Clone(privateKey)
	passphraseCopy := bytes.Clone(passphrase)
	defer clear(keyCopy)
	defer clear(passphraseCopy)
	if len(keyCopy) == 0 {
		return nil, &ConnectError{Stage: StageValidation, Err: ErrInvalidAuthentication}
	}

	var (
		signer ssh.Signer
		err    error
	)
	if len(passphraseCopy) == 0 {
		signer, err = ssh.ParsePrivateKey(keyCopy)
	} else {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyCopy, passphraseCopy)
	}
	if err != nil {
		return nil, &ConnectError{Stage: StageValidation, Err: fmt.Errorf("parse private key: %w", err)}
	}
	return c.connect(ctx, target, ssh.PublicKeys(signer))
}

type AuxiliaryAuthenticator func(ctx context.Context, auth AuxiliaryAuth) (AuxiliaryConnection, error)

type ConnectionProvisioner func(ctx context.Context) (AuxiliaryConnection, error)

type AuxiliaryAuth struct {
	Auth ssh.AuthMethod
}

type AuxiliaryConnection struct {
	Client *ssh.Client
	Close  func() error
}

func NewAuxiliaryAuthenticatorFor(connector *Connector, target Target) AuxiliaryAuthenticator {
	return connector.NewAuxiliaryAuthenticator(target)
}

func (c *Connector) NewAuxiliaryAuthenticator(target Target) AuxiliaryAuthenticator {
	return func(ctx context.Context, material AuxiliaryAuth) (AuxiliaryConnection, error) {
		if c == nil {
			return AuxiliaryConnection{}, &ConnectError{Stage: StageValidation, Err: ErrInvalidTarget}
		}
		if material.Auth == nil {
			return AuxiliaryConnection{}, &ConnectError{Stage: StageValidation, Err: ErrInvalidAuthentication}
		}
		connection, err := c.connect(ctx, target, material.Auth)
		if err != nil {
			return AuxiliaryConnection{}, err
		}
		return connection.detachAuxiliary(), nil
	}
}

func (c *Connection) Wait() error {
	if c == nil || c.client == nil {
		return errors.New("SSH connection is not ready")
	}
	return c.client.Wait()
}

func (c *Connection) InterruptTerminalWrite() {
	if c == nil {
		return
	}
	c.chainMu.Lock()
	hops := append([]*Connection(nil), c.chain...)
	c.chainMu.Unlock()

	for _, hop := range hops {
		if hop != nil && hop.client != nil {
			_ = hop.client.Close()
		}
	}
	if c.client != nil {
		_ = c.client.Close()
	}
}

func (c *Connection) Close() error {
	if c == nil {
		return nil
	}
	c.closeOnce.Do(func() {
		if c.closeTrace != nil {
			c.closeTrace()
		}

		c.keepaliveMu.Lock()
		cancel := c.keepaliveCancel
		c.keepaliveMu.Unlock()
		if cancel != nil {
			cancel()
		}
		c.fsMu.Lock()
		group := c.fileSystems
		c.fileSystems = nil
		c.fsMu.Unlock()
		if group != nil {
			group.abort(errSSHConnectionClosed)
			group.shutdown()
		}
		if c.client != nil {
			c.closeErr = c.client.Close()
		}

		c.closeChain()
	})
	return c.closeErr
}

func (c *Connection) closeChain() {
	c.chainMu.Lock()
	chain := c.chain
	c.chain = nil
	c.chainMu.Unlock()
	for index := len(chain) - 1; index >= 0; index-- {
		if chain[index] == nil {
			continue
		}
		if err := chain[index].Close(); err != nil && c.closeErr == nil {
			c.closeErr = err
		}
	}
}

func (c *Connection) adoptChain(chain []*Connection) {
	if c == nil {
		return
	}
	owned := make([]*Connection, 0, len(chain))
	for _, hop := range chain {

		if hop == nil || hop == c {
			continue
		}
		owned = append(owned, hop)
	}
	c.chainMu.Lock()
	c.chain = owned
	c.chainMu.Unlock()
}

func (c *Connection) ChainDialer() DialerFunc {
	if c == nil {
		return nil
	}
	c.chainMu.Lock()
	defer c.chainMu.Unlock()
	if len(c.chain) == 0 {
		return nil
	}
	last := c.chain[len(c.chain)-1]
	if last == nil {
		return nil
	}
	return last.DialContext
}

func (c *Connection) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if ctx == nil {
		return nil, errors.New("SSH dial context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c == nil || c.client == nil {
		return nil, errSSHConnectionClosed
	}
	return c.client.DialContext(ctx, network, address)
}

var errSSHConnectionClosed = errors.New("SSH connection is closed")

func normalizeTarget(target Target) (string, string, error) {
	address := strings.TrimSpace(target.Address)
	username := strings.TrimSpace(target.Username)
	if address == "" || username == "" ||
		len(address) > maxSSHFieldSize || len(username) > maxSSHFieldSize ||
		!utf8.ValidString(address) || !utf8.ValidString(username) ||
		strings.IndexByte(address, 0) >= 0 || strings.IndexByte(username, 0) >= 0 {
		return "", "", ErrInvalidTarget
	}
	port := target.Port
	if port == 0 {
		port = defaultSSHPort
	}
	if port < 1 || port > 65535 {
		return "", "", ErrInvalidTarget
	}
	return net.JoinHostPort(address, strconv.Itoa(port)), username, nil
}

func connectError(ctx context.Context, stage Stage, err error) error {
	if contextErr := ctx.Err(); contextErr != nil {
		err = contextErr
	}
	return &ConnectError{Stage: stage, Err: err}
}
