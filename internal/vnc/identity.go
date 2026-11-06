package vnc

import (
	"fmt"
	"net"
	"strings"
	"time"
)

type ServerIdentity struct {
	Software string

	Version string

	Banner string

	SecurityTypes string

	Source string
}

const bannerPeekTimeout = 3 * time.Second

const bannerPeekLimit = 24

func PeekServerBanner(conn net.Conn) (*identityConn, ServerIdentity) {

	wrapped := &identityConn{inner: conn}
	if conn == nil {
		return wrapped, ServerIdentity{}
	}

	if conn.SetReadDeadline(time.Now().Add(bannerPeekTimeout)) == nil {

		defer func() { _ = conn.SetReadDeadline(time.Time{}) }()
	}
	buffer := make([]byte, bannerPeekLimit)
	read, err := conn.Read(buffer)
	if read > 0 {

		wrapped.buffer = buffer[:read]
	}
	if !verifyReplayInvariant(read, wrapped) {

		return wrapped, ServerIdentity{}
	}
	identity := identifyFromBanner(wrapped.buffer)
	if err != nil && identity.Software == "" {

		return wrapped, ServerIdentity{}
	}
	return wrapped, identity
}

func verifyReplayInvariant(peeked int, wrapped *identityConn) bool {
	if wrapped == nil {
		return peeked == 0
	}
	return wrapped.PeekedBytes() == peeked
}

type identityConn struct {
	inner net.Conn

	buffer []byte
}

func (c *identityConn) PeekedBytes() int {
	if c == nil {
		return 0
	}
	return len(c.buffer)
}

func (c *identityConn) Read(target []byte) (int, error) {

	if len(c.buffer) > 0 {
		taken := copy(target, c.buffer)
		c.buffer = c.buffer[taken:]
		return taken, nil
	}
	return c.inner.Read(target)
}

func (c *identityConn) Write(payload []byte) (int, error) { return c.inner.Write(payload) }
func (c *identityConn) Close() error                      { return c.inner.Close() }
func (c *identityConn) LocalAddr() net.Addr               { return c.inner.LocalAddr() }
func (c *identityConn) RemoteAddr() net.Addr              { return c.inner.RemoteAddr() }
func (c *identityConn) SetDeadline(at time.Time) error    { return c.inner.SetDeadline(at) }
func (c *identityConn) SetReadDeadline(at time.Time) error {
	return c.inner.SetReadDeadline(at)
}

func (c *identityConn) SetWriteDeadline(at time.Time) error {
	return c.inner.SetWriteDeadline(at)
}

func identifyFromBanner(banner []byte) ServerIdentity {
	if len(banner) == 0 {
		return ServerIdentity{}
	}
	text := string(banner)
	identity := ServerIdentity{}
	if index := strings.Index(text, "RFB "); index >= 0 && index < 4 {
		line := text[index:]
		if end := strings.IndexAny(line, "\r\n"); end > 0 {
			line = line[:end]
		}
		identity.Banner = strings.TrimSpace(line)
	}
	if len(banner) > 12 {
		identity.SecurityTypes = strings.TrimSpace(text[12:])
	}

	if strings.HasPrefix(identity.Banner, "RFB ") {
		identity.Software, identity.Version = softwareFromServerHeader(identity.Banner)
	}
	if identity.Software == "" {

		identity.Software, identity.Version = softwareFromServerHeader(text)
	}
	if identity.Software != "" {
		identity.Source = "banner"
	}
	return identity
}

func softwareFromServerHeader(value string) (string, string) {
	lowered := strings.ToLower(value)
	for _, candidate := range knownServerSoftware {
		index := strings.Index(lowered, candidate)
		if index < 0 {
			continue
		}
		return candidate, versionAfter(lowered, index+len(candidate))
	}
	return "", ""
}

var knownServerSoftware = []string{"kasmvnc", "tigervnc", "tightvnc", "realvnc", "ultravnc", "libvnc", "websockify", "novnc", "nginx", "python"}

func versionAfter(lowered string, index int) string {

	if index < len(lowered) {
		switch lowered[index] {
		case '/', ' ', '-', '_', ':':
			index++
		}
	}
	start := index
	for index < len(lowered) {
		character := lowered[index]
		if (character >= '0' && character <= '9') || character == '.' {
			index++
			continue
		}
		break
	}
	version := strings.Trim(lowered[start:index], ".")
	return version
}

func softwareFromHandshake(handshake WebSocketHandshake) string {
	software, _ := softwareFromServerHeader(handshake.ServerHeader)
	if software != "" {
		return software
	}
	realmSoftware, _ := softwareFromServerHeader(handshake.Realm)
	return realmSoftware
}

func versionFromHandshake(handshake WebSocketHandshake) string {
	_, version := softwareFromServerHeader(handshake.ServerHeader)
	if version != "" {
		return version
	}
	_, version = softwareFromServerHeader(handshake.Realm)
	return version
}

func mergeServerIdentity(fromHeader, fromBanner ServerIdentity) ServerIdentity {
	merged := fromHeader
	if merged.Software == "" {
		merged.Software = fromBanner.Software
		merged.Source = fromBanner.Source
		if merged.Version == "" {
			merged.Version = fromBanner.Version
		}
	}
	if merged.Version == "" {
		merged.Version = fromBanner.Version
	}
	if merged.Banner == "" {
		merged.Banner = fromBanner.Banner
	}
	if merged.SecurityTypes == "" {
		merged.SecurityTypes = fromBanner.SecurityTypes
	}
	if merged.Software != "" && merged.Source == "" {
		merged.Source = "header"
	}
	return merged
}

func (i ServerIdentity) Describe() string {
	if i.Software == "" {
		return "未知（未拿到服务端身份）"
	}
	if i.Version == "" {
		return fmt.Sprintf("%s（版本未知）", i.Software)
	}
	return fmt.Sprintf("%s/%s", i.Software, i.Version)
}
