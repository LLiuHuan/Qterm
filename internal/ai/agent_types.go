package ai

import "time"

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

	Since int
}

type StepSpec struct {
	CallID string
	Kind   StepKind

	Title string

	Command         string
	TargetSessionID string
	Risk            string
	RiskReason      string
}

type StepResult struct {
	Observation string

	ExitCode *int
}

type ApprovalRequest struct {
	Command         string
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

	Command         string `json:"command,omitempty"`
	TargetSessionID string `json:"targetSessionId,omitempty"`
	Risk            string `json:"risk,omitempty"`
	RiskReason      string `json:"riskReason,omitempty"`

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

	ModelCalls  int `json:"modelCalls,omitempty"`
	CommandRuns int `json:"commandRuns,omitempty"`

	Notes []string `json:"notes,omitempty"`

	NoteDurations []int64 `json:"noteDurations,omitempty"`

	Usage *Usage `json:"usage,omitempty"`

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
