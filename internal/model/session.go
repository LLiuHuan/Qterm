package model

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidSession           = errors.New("invalid session")
	ErrInvalidSessionTransition = errors.New("invalid session transition")
)

const maxSessionIdentifierSize = 1024

type SessionState string

type SessionKind string

const (
	SessionKindSSH    SessionKind = "ssh"
	SessionKindTelnet SessionKind = "telnet"
	SessionKindLocal  SessionKind = "local"
)

type SessionPurpose string

const (
	SessionPurposeTerminal SessionPurpose = "terminal"

	SessionPurposeSftp SessionPurpose = "sftp"
)

const (
	SessionStateConnecting SessionState = "connecting"

	SessionStateAwaitingAuth   SessionState = "awaitingAuth"
	SessionStateHostKeyPending SessionState = "hostKeyPending"
	SessionStateAuthenticating SessionState = "authenticating"
	SessionStateConnected      SessionState = "connected"
	SessionStateClosing        SessionState = "closing"
	SessionStateClosed         SessionState = "closed"
	SessionStateFailed         SessionState = "failed"
)

type SessionCloseReason = string

const (
	SessionCloseReasonUserClosed SessionCloseReason = "user_closed"

	SessionCloseReasonRemoteEOF SessionCloseReason = "remote_eof"

	SessionCloseReasonOrphanTimeout SessionCloseReason = "orphan_timeout"

	SessionCloseReasonError SessionCloseReason = "error"
)

type Session struct {
	ID      string         `json:"id"`
	HostID  string         `json:"hostId"`
	Kind    SessionKind    `json:"kind"`
	State   SessionState   `json:"state"`
	Purpose SessionPurpose `json:"purpose"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	LatencyMS *int64    `json:"latencyMs"`

	AttachEpoch uint64 `json:"attachEpoch"`

	CloseReason string `json:"closeReason,omitempty"`

	ExitCode *int `json:"exitCode,omitempty"`
}

type ResourceSnapshot struct {
	MetricsSupported bool `json:"metricsSupported"`

	LoadAverage1 *float64 `json:"loadAverage1"`

	LoadAverage5  *float64 `json:"loadAverage5"`
	LoadAverage15 *float64 `json:"loadAverage15"`

	SampleWindowMS       int64  `json:"sampleWindowMs"`
	Hostname             string `json:"hostname"`
	OS                   string `json:"os"`
	Architecture         string `json:"architecture"`
	CPUCores             int    `json:"cpuCores"`
	MemoryTotalBytes     uint64 `json:"memoryTotalBytes"`
	MemoryAvailableBytes uint64 `json:"memoryAvailableBytes"`

	MemoryCachedBytes  uint64  `json:"memoryCachedBytes"`
	UptimeSeconds      uint64  `json:"uptimeSeconds"`
	CPUUsagePercent    float64 `json:"cpuUsagePercent"`
	MemoryUsagePercent float64 `json:"memoryUsagePercent"`
	MemoryUsedText     string  `json:"memoryUsedText"`

	MemoryAvailableText string            `json:"memoryAvailableText"`
	MemoryCachedText    string            `json:"memoryCachedText"`
	MemoryTotalText     string            `json:"memoryTotalText"`
	CPUPerCore          []float64         `json:"cpuPerCore"`
	Networks            []NetworkResource `json:"networks"`
	Disks               []DiskResource    `json:"disks"`
}

type NetworkResource struct {
	Name              string `json:"name"`
	ReceiveBytes      uint64 `json:"receiveBytes"`
	TransmitBytes     uint64 `json:"transmitBytes"`
	ReceiveText       string `json:"receiveText"`
	TransmitText      string `json:"transmitText"`
	ReceiveRateBytes  uint64 `json:"receiveRateBytes"`
	TransmitRateBytes uint64 `json:"transmitRateBytes"`
}

type DiskResource struct {
	Mount          string  `json:"mount"`
	FileSystemType string  `json:"fileSystemType"`
	UsedPercent    float64 `json:"usedPercent"`
	UsedBytes      uint64  `json:"usedBytes"`
	TotalBytes     uint64  `json:"totalBytes"`
	UsedText       string  `json:"usedText"`
	TotalText      string  `json:"totalText"`
}

func NewSession(id, hostID string, now time.Time) (Session, error) {
	id, err := normalizeSessionIdentifier(id)
	if err != nil {
		return Session{}, err
	}
	hostID, err = normalizeSessionIdentifier(hostID)
	if err != nil {
		return Session{}, err
	}
	now = now.UTC()
	return Session{
		ID:        id,
		HostID:    hostID,
		Kind:      SessionKindSSH,
		State:     SessionStateConnecting,
		Purpose:   SessionPurposeTerminal,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func NewTelnetSession(id, hostID string, now time.Time) (Session, error) {
	session, err := NewSession(id, hostID, now)
	if err != nil {
		return Session{}, err
	}
	session.Kind = SessionKindTelnet
	return session, nil
}

func NewSftpSession(id, hostID string, now time.Time) (Session, error) {
	session, err := NewSession(id, hostID, now)
	if err != nil {
		return Session{}, err
	}
	session.Purpose = SessionPurposeSftp
	return session, nil
}

func NewLocalSession(id string, now time.Time) (Session, error) {
	id, err := normalizeSessionIdentifier(id)
	if err != nil {
		return Session{}, err
	}
	now = now.UTC()
	return Session{
		ID:        id,
		Kind:      SessionKindLocal,
		State:     SessionStateConnecting,
		Purpose:   SessionPurposeTerminal,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (s Session) Transition(next SessionState, now time.Time) (Session, error) {
	if !s.State.CanTransitionTo(next) {
		return Session{}, ErrInvalidSessionTransition
	}
	s.State = next
	s.UpdatedAt = now.UTC()
	return s, nil
}

func (s SessionState) Terminal() bool {
	return s == SessionStateClosed || s == SessionStateFailed
}

func (s SessionState) CanTransitionTo(next SessionState) bool {
	switch s {
	case SessionStateConnecting:
		return next == SessionStateAwaitingAuth ||
			next == SessionStateHostKeyPending ||
			next == SessionStateAuthenticating ||
			next == SessionStateConnected ||
			next == SessionStateClosing ||
			next == SessionStateFailed
	case SessionStateAwaitingAuth:

		return next == SessionStateHostKeyPending ||
			next == SessionStateAuthenticating ||
			next == SessionStateClosing ||
			next == SessionStateFailed
	case SessionStateHostKeyPending:
		return next == SessionStateConnecting ||
			next == SessionStateClosing ||
			next == SessionStateFailed
	case SessionStateAuthenticating:

		return next == SessionStateAwaitingAuth ||
			next == SessionStateConnected ||
			next == SessionStateClosing ||
			next == SessionStateFailed
	case SessionStateConnected:
		return next == SessionStateClosing ||
			next == SessionStateClosed ||
			next == SessionStateFailed
	case SessionStateClosing:
		return next == SessionStateClosed
	default:
		return false
	}
}

func normalizeSessionIdentifier(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxSessionIdentifierSize ||
		!utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 {
		return "", ErrInvalidSession
	}
	return value, nil
}
