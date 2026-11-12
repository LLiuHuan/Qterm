package terminal

import (
	"context"
	"errors"
	"time"
)

type SessionInfo struct {
	ID      string `json:"id"`
	HostID  string `json:"hostId"`
	Kind    string `json:"kind"`
	State   string `json:"state"`
	Purpose string `json:"purpose"`
}

type SessionDirectory interface {
	List(context.Context) ([]SessionInfo, error)
}

type SessionWriteFunc func(context.Context, string, []byte) error

var (
	ErrInvalidControlRequest = errors.New("invalid terminal control request")
	ErrControlNotReady       = errors.New("terminal control is not ready")
	ErrWindowRequired        = errors.New("terminal window is required")
	ErrSessionRequired       = errors.New("terminal session is required")
	ErrSessionNotEligible    = errors.New("terminal session is not eligible")
	ErrSyncGroupNotFound     = errors.New("terminal sync group not found")
	ErrSyncRevisionConflict  = errors.New("terminal sync revision conflict")
	ErrSyncMemberRequired    = errors.New("terminal sync requires at least two members")
	ErrBatchTaskNotFound     = errors.New("terminal batch task not found")
	ErrBatchTargetRequired   = errors.New("terminal batch target is required")
	ErrBatchContentRequired  = errors.New("terminal batch content is required")
	ErrBatchRepeatOutOfRange = errors.New("terminal batch repeat count is out of range")
	ErrBatchIntervalInvalid  = errors.New("terminal batch interval is invalid")
	ErrBatchSyncConflict     = errors.New("terminal batch conflicts with active sync")
	ErrBatchAlreadyRunning   = errors.New("terminal batch is already running")
	ErrWriterQueueFull       = errors.New("terminal session input queue is full")
)

const (
	maxWriterQueue   = 256
	maxBatchRepeats  = 1000
	maxBatchInterval = 24 * time.Hour
	maxBatchTargets  = 64
	maxBatchContent  = 1 << 20
	writeTimeout     = 10 * time.Second
)

type SyncStatus string

const (
	SyncStatusActive  SyncStatus = "active"
	SyncStatusPaused  SyncStatus = "paused"
	SyncStatusStopped SyncStatus = "stopped"
)

type SyncGroupState struct {
	ID                  string     `json:"id"`
	WindowID            string     `json:"windowId"`
	SessionIDs          []string   `json:"sessionIds"`
	Status              SyncStatus `json:"status"`
	Revision            uint64     `json:"revision"`
	LastSourceSessionID string     `json:"lastSourceSessionId,omitempty"`
	Error               string     `json:"error,omitempty"`
}

type SyncAction string

const (
	SyncActionCreate  SyncAction = "create"
	SyncActionMembers SyncAction = "members"
	SyncActionJoin    SyncAction = "join"
	SyncActionLeave   SyncAction = "leave"
	SyncActionPause   SyncAction = "pause"
	SyncActionResume  SyncAction = "resume"
	SyncActionStop    SyncAction = "stop"
)

type SyncRequest struct {
	Action     SyncAction `json:"action"`
	SessionIDs []string   `json:"sessionIds,omitempty"`
	Revision   uint64     `json:"revision,omitempty"`
}

type BatchSendMode string

const (
	BatchSendRaw      BatchSendMode = "raw"
	BatchSendAppendCR BatchSendMode = "append_cr"
	BatchSendEachLine BatchSendMode = "each_line_cr"
)

type BatchRequest struct {
	SessionIDs  []string      `json:"sessionIds"`
	Content     string        `json:"content"`
	SendMode    BatchSendMode `json:"sendMode"`
	RepeatCount int           `json:"repeatCount"`
	IntervalMS  int           `json:"intervalMs"`
	PauseSync   bool          `json:"pauseSync,omitempty"`
}

type BatchTaskStatus string

const (
	BatchTaskQueued          BatchTaskStatus = "queued"
	BatchTaskRunning         BatchTaskStatus = "running"
	BatchTaskCompleted       BatchTaskStatus = "completed"
	BatchTaskPartiallyFailed BatchTaskStatus = "partially_failed"
	BatchTaskFailed          BatchTaskStatus = "failed"
	BatchTaskCancelled       BatchTaskStatus = "cancelled"
)

type BatchTargetState struct {
	SessionID     string `json:"sessionId"`
	WrittenRounds int    `json:"writtenRounds"`
	Status        string `json:"status"`
	Error         string `json:"error,omitempty"`
}

type BatchTaskState struct {
	ID              string             `json:"id"`
	WindowID        string             `json:"windowId"`
	SessionIDs      []string           `json:"sessionIds"`
	SendMode        BatchSendMode      `json:"sendMode"`
	RepeatCount     int                `json:"repeatCount"`
	IntervalMS      int                `json:"intervalMs"`
	CompletedRounds int                `json:"completedRounds"`
	Status          BatchTaskStatus    `json:"status"`
	Targets         []BatchTargetState `json:"targets"`
}

type ControlSnapshot struct {
	Sessions []SessionInfo    `json:"sessions"`
	Sync     *SyncGroupState  `json:"sync,omitempty"`
	Tasks    []BatchTaskState `json:"tasks"`
	Error    string           `json:"error,omitempty"`
}

type InputRouter interface {
	Bind(windowID, sessionID string) (InputRouter, func(), error)
	LocalInput(context.Context, string, string, []byte) error
	DirectInput(context.Context, string, string, []byte) error
}
