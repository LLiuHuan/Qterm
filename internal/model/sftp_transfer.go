package model

import (
	"errors"
	"path"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidSFTPTransfer           = errors.New("invalid SFTP transfer")
	ErrInvalidSFTPTransferTransition = errors.New("invalid SFTP transfer transition")
)

type SFTPTransferDirection string

const (
	SFTPTransferUpload   SFTPTransferDirection = "upload"
	SFTPTransferDownload SFTPTransferDirection = "download"
)

type SFTPTransferState string

const (
	SFTPTransferQueued            SFTPTransferState = "queued"
	SFTPTransferAwaitingOverwrite SFTPTransferState = "awaitingOverwrite"
	SFTPTransferRunning           SFTPTransferState = "running"

	SFTPTransferPaused    SFTPTransferState = "paused"
	SFTPTransferCompleted SFTPTransferState = "completed"
	SFTPTransferFailed    SFTPTransferState = "failed"
	SFTPTransferCanceled  SFTPTransferState = "canceled"
)

const SFTPTransferStageExtracting = "extracting"

const SFTPTransferStagePacking = "packing"

type SFTPTransfer struct {
	ID          string                `json:"id"`
	SessionID   string                `json:"sessionId"`
	Direction   SFTPTransferDirection `json:"direction"`
	LocalName   string                `json:"localName"`
	RemotePath  string                `json:"remotePath"`
	State       SFTPTransferState     `json:"state"`
	Transferred int64                 `json:"transferred"`
	Total       int64                 `json:"total"`
	CreatedAt   time.Time             `json:"createdAt"`
	UpdatedAt   time.Time             `json:"updatedAt"`

	RateBytesPerSecond int64 `json:"rateBytesPerSecond"`

	Stage string `json:"stage,omitempty"`
}

func NewSFTPTransfer(
	id string,
	sessionID string,
	direction SFTPTransferDirection,
	localName string,
	remotePath string,
	total int64,
	now time.Time,
) (SFTPTransfer, error) {
	id = strings.TrimSpace(id)
	sessionID = strings.TrimSpace(sessionID)
	if id == "" || sessionID == "" || len(id) > 128 || len(sessionID) > 128 ||
		(direction != SFTPTransferUpload && direction != SFTPTransferDownload) ||
		!validTransferName(localName) || total < 0 {
		return SFTPTransfer{}, ErrInvalidSFTPTransfer
	}
	remotePath, err := NormalizeRemotePath(remotePath)
	if err != nil || remotePath == "/" || path.Base(remotePath) == "." {
		return SFTPTransfer{}, ErrInvalidSFTPTransfer
	}
	now = now.UTC()
	return SFTPTransfer{
		ID:         id,
		SessionID:  sessionID,
		Direction:  direction,
		LocalName:  localName,
		RemotePath: remotePath,
		State:      SFTPTransferQueued,
		Total:      total,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

func (t SFTPTransfer) Transition(next SFTPTransferState, now time.Time) (SFTPTransfer, error) {
	allowed := false
	switch t.State {
	case SFTPTransferQueued:
		allowed = next == SFTPTransferRunning || next == SFTPTransferAwaitingOverwrite ||
			next == SFTPTransferFailed || next == SFTPTransferCanceled
	case SFTPTransferAwaitingOverwrite:
		allowed = next == SFTPTransferQueued || next == SFTPTransferCanceled
	case SFTPTransferRunning:
		allowed = next == SFTPTransferAwaitingOverwrite || next == SFTPTransferCompleted ||
			next == SFTPTransferFailed || next == SFTPTransferCanceled || next == SFTPTransferPaused
	case SFTPTransferPaused:

		allowed = next == SFTPTransferQueued || next == SFTPTransferCanceled
	case SFTPTransferFailed:
		allowed = next == SFTPTransferQueued || next == SFTPTransferCanceled
	}
	if !allowed {
		return SFTPTransfer{}, ErrInvalidSFTPTransferTransition
	}
	t.State = next
	t.UpdatedAt = now.UTC()

	t.Stage = ""
	if next == SFTPTransferCompleted {
		t.Transferred = t.Total
	}
	return t, nil
}

func (t SFTPTransfer) WithStage(stage string, now time.Time) (SFTPTransfer, error) {
	if stage != "" && stage != SFTPTransferStageExtracting && stage != SFTPTransferStagePacking {
		return SFTPTransfer{}, ErrInvalidSFTPTransfer
	}
	if stage != "" && t.State != SFTPTransferRunning {
		return SFTPTransfer{}, ErrInvalidSFTPTransferTransition
	}
	if t.Stage == stage {
		return t, nil
	}
	t.Stage = stage
	t.UpdatedAt = now.UTC()
	return t, nil
}

func (t SFTPTransfer) WithTotal(total int64, now time.Time) (SFTPTransfer, error) {
	if total <= 0 {
		return SFTPTransfer{}, ErrInvalidSFTPTransfer
	}
	if t.State != SFTPTransferRunning || t.Total != 0 {
		return SFTPTransfer{}, ErrInvalidSFTPTransferTransition
	}
	t.Total = total
	t.UpdatedAt = now.UTC()
	return t, nil
}

func (t SFTPTransfer) WithProgressAndRate(transferred int64, rate int64, now time.Time) (SFTPTransfer, error) {
	if rate < 0 {
		rate = 0
	}
	next, err := t.WithProgress(transferred, now)
	if err != nil {
		return SFTPTransfer{}, err
	}
	next.RateBytesPerSecond = rate
	return next, nil
}

func (t SFTPTransfer) WithProgress(transferred int64, now time.Time) (SFTPTransfer, error) {
	if t.State != SFTPTransferRunning || transferred < t.Transferred || transferred > t.Total {
		return SFTPTransfer{}, ErrInvalidSFTPTransfer
	}
	t.Transferred = transferred
	t.UpdatedAt = now.UTC()
	return t, nil
}

func validTransferName(value string) bool {
	return value != "" && value != "." && value != ".." && len(value) <= maxRemoteNameSize &&
		utf8.ValidString(value) && strings.IndexByte(value, 0) < 0 &&
		!strings.ContainsAny(value, "/\\")
}
