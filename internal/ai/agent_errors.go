package ai

import "changeme/internal/errno"

const (
	msgAgentInvalidArg        = "ai.agent.invalid_arg"
	msgAgentRunNotFound       = "ai.agent.run_not_found"
	msgAgentApprovalNotFound  = "ai.agent.approval_not_found"
	msgAgentInvalidState      = "ai.agent.invalid_state"
	msgAgentTargetMismatch    = "ai.agent.approval_target_mismatch"
	msgAgentPolicyMismatch    = "ai.agent.approval_policy_mismatch"
	msgAgentTooManyRuns       = "ai.agent.too_many_runs"
	msgAgentClosed            = "ai.agent.manager_closed"
	msgAgentDuplicateCall     = "ai.agent.duplicate_call"
	msgAgentUnknownTool       = "ai.agent.unknown_tool"
	msgAgentInvalidAction     = "ai.agent.invalid_action"
	msgAgentMultipleActions   = "ai.agent.multiple_actions"
	msgAgentMaxSteps          = "ai.agent.max_steps"
	msgAgentExecutionFailed   = "ai.agent.execution_failed"
	msgAgentExecutorUnavail   = "ai.agent.executor_unavailable"
	msgAgentCapabilityUnavail = "ai.agent.capability_unavailable"
	msgAgentModelFailed       = "ai.agent.model_failed"

	msgAgentRejected        = "ai.agent.rejected"
	msgAgentCancelled       = "ai.agent.cancelled"
	msgAgentRunTimedOut     = "ai.agent.run_timed_out"
	msgAgentApprovalExpired = "ai.agent.approval_expired"
	msgAgentFailed          = "ai.agent.failed"
	msgAgentBlocked         = "ai.agent.blocked"
)

func agentInvalidArg(detail string) error {
	return errno.New(errno.CodeInvalidArg, msgAgentInvalidArg, detail)
}

func agentRunNotFound() error {
	return errno.New(errno.CodeNotFound, msgAgentRunNotFound, "")
}

func agentApprovalNotFound() error {

	return errno.New(errno.CodeNotFound, msgAgentApprovalNotFound, "")
}

func agentInvalidState(detail string) error {
	return errno.New(errno.CodeInvalidArg, msgAgentInvalidState, detail)
}

func agentTargetMismatch() error {
	return errno.New(errno.CodePermissionDenied, msgAgentTargetMismatch, "")
}

func agentPolicyMismatch() error {
	return errno.New(errno.CodePermissionDenied, msgAgentPolicyMismatch, "")
}

func agentDuplicateCall() error {
	return errno.New(errno.CodeInvalidArg, msgAgentDuplicateCall, "")
}

func agentUnknownTool() error {
	return errno.New(errno.CodeInvalidArg, msgAgentUnknownTool, "")
}

func agentInvalidAction(detail string) error {
	return errno.New(errno.CodeInvalidArg, msgAgentInvalidAction, detail)
}

func agentMultipleActions() error {
	return errno.New(errno.CodeInvalidArg, msgAgentMultipleActions, "")
}

func agentTooManyRuns() error {
	return errno.New(errno.CodeInvalidArg, msgAgentTooManyRuns, "")
}

func agentClosed() error {
	return errno.New(errno.CodeCancelled, msgAgentClosed, "")
}
