package ai

import (
	"encoding/json"
	"time"
)

type RunState string

const (
	RunRunning RunState = "running"

	RunWaitingApproval RunState = "waiting_approval"

	RunCompleted RunState = "completed"

	RunRejected RunState = "rejected"

	RunFailed RunState = "failed"

	RunCancelled RunState = "cancelled"

	RunTimedOut RunState = "timed_out"

	RunBlocked RunState = "blocked"
)

type StepKind string

const (
	StepKindCommand StepKind = "execute_command"

	StepKindAnswer StepKind = "final_answer"

	StepKindListHosts StepKind = "list_hosts"

	StepKindGetHost StepKind = "get_host"

	StepKindSftpList StepKind = "sftp_list"

	StepKindSftpReadFile StepKind = "sftp_read_file"

	StepKindSftpWriteFile StepKind = "sftp_write_file"

	StepKindSftpUploadFile StepKind = "sftp_upload_file"

	StepKindSftpDownloadFile StepKind = "sftp_download_file"

	StepKindLocalListDir StepKind = "local_list_dir"

	StepKindLocalReadFile StepKind = "local_read_file"

	StepKindWebFetch StepKind = "web_fetch"

	StepKindAskUser StepKind = "ask_user"
)

type StepState string

const (
	StepRunning         StepState = "running"
	StepWaitingApproval StepState = "waiting_approval"
	StepCompleted       StepState = "completed"
	StepFailed          StepState = "failed"
	StepCancelled       StepState = "cancelled"
	StepTimedOut        StepState = "timed_out"
	StepRejected        StepState = "rejected"
)

const (
	RiskReadOnly = "readonly"
	RiskConfirm  = "confirm"
	RiskDanger   = "danger"
)

const (
	ExecutionBackground = "background"
	ExecutionTerminal   = "terminal"
)

type StartInput struct {
	ProviderID string

	ProviderType ProviderType
	BaseURL      string
	Model        string

	SessionID string

	HostIDs []string `json:"hostIds,omitempty"`

	SessionKind string `json:"sessionKind,omitempty"`

	TargetLabel string `json:"targetLabel,omitempty"`

	Prompt string

	PolicyVersion string

	ApprovalMode ApprovalMode

	History []AgentMessage

	ContextText string

	ExecutionChannel string

	MaxSteps int

	ModelTimeoutSeconds int
	StepTimeoutSeconds  int
}

type DecideInput struct {
	RunID         string
	StepID        string
	PendingID     string
	Approve       bool
	SessionID     string
	PolicyVersion string

	Choice string

	Remember bool

	Since int
}

type StepSpec struct {
	CallID string
	Kind   StepKind

	Title string

	Command string

	Args            json.RawMessage
	TargetSessionID string
	Risk            string
	RiskReason      string
}

type StepResult struct {
	Observation string

	ExitCode *int

	Failure string
}

type ToolPreview struct {
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data,omitempty"`
}

type ApprovalRequest struct {
	Command string

	Preview *ToolPreview

	Rememberable    bool
	TargetSessionID string
	Risk            string
	RiskReason      string
}

type AgentStep struct {
	ID     string    `json:"id"`
	Index  int       `json:"index"`
	CallID string    `json:"callId,omitempty"`
	Kind   StepKind  `json:"kind"`
	State  StepState `json:"state"`
	Title  string    `json:"title,omitempty"`

	Command string `json:"command,omitempty"`

	Args json.RawMessage `json:"args,omitempty"`

	Preview         *ToolPreview `json:"preview,omitempty"`
	TargetSessionID string       `json:"targetSessionId,omitempty"`
	Risk            string       `json:"risk,omitempty"`
	RiskReason      string       `json:"riskReason,omitempty"`

	Observation          string `json:"observation,omitempty"`
	ObservationTruncated bool   `json:"observationTruncated,omitempty"`

	ExitCode *int `json:"exitCode,omitempty"`

	PendingID string `json:"pendingId,omitempty"`

	ErrorKey    string `json:"errorKey,omitempty"`
	ErrorDetail string `json:"errorDetail,omitempty"`

	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
}

type AgentRun struct {
	ID            string      `json:"id"`
	ProviderID    string      `json:"providerId"`
	Model         string      `json:"model"`
	SessionID     string      `json:"sessionId"`
	Prompt        string      `json:"prompt"`
	PolicyVersion string      `json:"policyVersion"`
	State         RunState    `json:"state"`
	Steps         []AgentStep `json:"steps"`

	Draft string `json:"draft,omitempty"`

	ReasoningDraft string `json:"reasoningDraft,omitempty"`

	ModelCalls  int `json:"modelCalls,omitempty"`
	CommandRuns int `json:"commandRuns,omitempty"`

	Reasonings []string `json:"reasonings,omitempty"`

	Notes []string `json:"notes,omitempty"`

	NoteDurations []int64 `json:"noteDurations,omitempty"`

	Usage *Usage `json:"usage,omitempty"`

	AllowedTools []string `json:"allowedTools,omitempty"`

	Context *ContextBreakdown `json:"context,omitempty"`

	ErrorKey    string `json:"errorKey,omitempty"`
	ErrorDetail string `json:"errorDetail,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type AgentEvent struct {
	Cursor int    `json:"cursor"`
	RunID  string `json:"runId"`

	Kind   string    `json:"kind"`
	StepID string    `json:"stepId,omitempty"`
	State  string    `json:"state"`
	At     time.Time `json:"at"`
}

type AgentSnapshot struct {
	Run AgentRun `json:"run"`

	Events []AgentEvent `json:"events"`

	Cursor int `json:"cursor"`

	EarliestCursor int  `json:"earliestCursor"`
	Truncated      bool `json:"truncated"`
}
