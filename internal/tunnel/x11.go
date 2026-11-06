package tunnel

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type X11ForwardingOptions struct {
	Display      string
	Screen       int
	AuthProtocol string
	AuthCookie   string

	Trusted bool
}

var ErrX11Unavailable = errors.New("local X server is unavailable")

type x11Request struct {
	SingleRequest bool
	AuthProtocol  string
	AuthCookie    string
	ScreenNumber  uint32
}

const x11DialTimeout = 3 * time.Second

func requestX11Forwarding(
	client *ssh.Client,
	session *ssh.Session,
	options X11ForwardingOptions,
) error {
	if client == nil || session == nil {
		return ErrTerminalNotReady
	}
	if strings.TrimSpace(options.AuthCookie) == "" {
		return ErrX11Unavailable
	}
	protocol := strings.TrimSpace(options.AuthProtocol)
	if protocol == "" {
		protocol = "MIT-MAGIC-COOKIE-1"
	}

	if _, err := dialLocalXServer(options); err != nil {
		return ErrX11Unavailable
	}
	serveX11Channels(client, options)
	payload := ssh.Marshal(x11Request{

		SingleRequest: !options.Trusted,
		AuthProtocol:  protocol,
		AuthCookie:    options.AuthCookie,
		ScreenNumber:  uint32(max(options.Screen, 0)),
	})
	ok, err := session.SendRequest("x11-req", true, payload)
	if err != nil {
		return fmt.Errorf("request X11 forwarding: %w", err)
	}
	if !ok {
		return fmt.Errorf("request X11 forwarding: %w", ErrX11Unavailable)
	}
	return nil
}

func serveX11Channels(client *ssh.Client, options X11ForwardingOptions) {
	channels := client.HandleChannelOpen("x11")
	go func() {
		for newChannel := range channels {
			go bridgeX11Channel(newChannel, options)
		}
	}()
}

func bridgeX11Channel(newChannel ssh.NewChannel, options X11ForwardingOptions) {
	local, err := dialLocalXServer(options)
	if err != nil {

		_ = newChannel.Reject(ssh.ConnectionFailed, "local X server unavailable")
		return
	}
	defer local.Close()
	channel, requests, err := newChannel.Accept()
	if err != nil {
		return
	}
	go ssh.DiscardRequests(requests)
	done := make(chan struct{}, 1)
	go func() {
		_, _ = io.Copy(channel, local)
		_ = channel.Close()
		done <- struct{}{}
	}()
	_, _ = io.Copy(local, channel)
	select {
	case <-done:
	case <-time.After(50 * time.Millisecond):

	}
}

func dialLocalXServer(options X11ForwardingOptions) (net.Conn, error) {
	screen := max(options.Screen, 0)
	candidates := []struct {
		network string
		address string
	}{
		{"unix", filepath.Join(os.TempDir(), fmt.Sprintf(".X11-unix/X%d", screen))},
		{"unix", fmt.Sprintf("/tmp/.X11-unix/X%d", screen)},
		{"tcp", fmt.Sprintf("127.0.0.1:%d", 6000+screen)},
	}

	if strings.HasPrefix(options.Display, "/") {
		if conn, err := net.DialTimeout("unix", options.Display, x11DialTimeout); err == nil {
			return conn, nil
		}
	}
	var lastErr error
	for _, candidate := range candidates {
		if candidate.network == "unix" {
			if _, err := os.Stat(candidate.address); err != nil {
				lastErr = err
				continue
			}
		}
		conn, err := net.DialTimeout(candidate.network, candidate.address, x11DialTimeout)
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = ErrX11Unavailable
	}
	return nil, lastErr
}
