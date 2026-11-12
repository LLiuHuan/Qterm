package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"changeme/internal/errno"
)

var ErrExecutorUnavailable = errors.New("agent executor is not available")

var ErrCapabilityUnavailable = errors.New("target does not support agent execution")

type ExecRequest struct {
	SessionID string
	Command   string

	Channel string
}

type ToolRequest struct {
	Kind      StepKind
	Args      json.RawMessage
	SessionID string
}

type ToolExecutor interface {
	ExecuteTool(ctx context.Context, request ToolRequest) (StepResult, error)
}

type ToolPrep struct {
	Preview ToolPreview
	Args    json.RawMessage
}

type ToolPreparer interface {
	PrepareTool(ctx context.Context, request ToolRequest) (ToolPrep, error)
}

var ErrNoOpenSession = errors.New("host has no open session")

type HostSessionResolver interface {
	SessionForHost(ctx context.Context, hostID string) (string, error)
}

type Executor interface {
	Execute(ctx context.Context, request ExecRequest) (StepResult, error)
}

const (
	defaultAgentMaxSteps        = 10
	maxAgentMaxSteps            = 50
	defaultAgentModelTimeout    = 60 * time.Second
	defaultAgentStepTimeout     = 30 * time.Second
	maxAgentObservationForModel = 8 << 10
	maxAgentHistoryMessages     = 20
	maxAgentHistoryBytes        = 32 << 10
)

type RunnerOption func(*Runner)

func WithMaxSteps(steps int) RunnerOption {
	return func(runner *Runner) {
		if steps > 0 {
			runner.maxSteps = steps
		}
	}
}

func WithModelTimeout(timeout time.Duration) RunnerOption {
	return func(runner *Runner) {
		if timeout > 0 {
			runner.modelTimeout = timeout
		}
	}
}

func WithStepTimeout(timeout time.Duration) RunnerOption {
	return func(runner *Runner) {
		if timeout > 0 {
			runner.stepTimeout = timeout
		}
	}
}

func WithDebugLog(log AgentDebugFunc) RunnerOption {
	return func(runner *Runner) {
		if log != nil {
			runner.debugLog = log
		}
	}
}

func WithPromptBudget(bytes int) RunnerOption {
	return func(runner *Runner) {
		if bytes >= minAgentPromptBytes {
			runner.promptBudget = bytes
		}
	}
}

func WithObservationBudget(bytes int) RunnerOption {
	return func(runner *Runner) {
		if bytes > 0 {
			runner.observationBudget = bytes
		}
	}
}

type Runner struct {
	manager           *Manager
	service           *Service
	executor          Executor
	maxSteps          int
	modelTimeout      time.Duration
	stepTimeout       time.Duration
	observationBudget int
	promptBudget      int
	debugLog          AgentDebugFunc

	draftObserver func(runID string)
}

func NewRunner(manager *Manager, service *Service, executor Executor, options ...RunnerOption) *Runner {
	runner := &Runner{
		manager:           manager,
		service:           service,
		executor:          executor,
		maxSteps:          defaultAgentMaxSteps,
		modelTimeout:      defaultAgentModelTimeout,
		stepTimeout:       defaultAgentStepTimeout,
		observationBudget: maxAgentObservationForModel,
		promptBudget:      defaultAgentPromptBytes,
	}
	for _, option := range options {
		if option != nil {
			option(runner)
		}
	}
	if runner.maxSteps > maxAgentMaxSteps {
		runner.maxSteps = maxAgentMaxSteps
	}
	return runner
}

func WithDraftObserver(observer func(runID string)) RunnerOption {
	return func(runner *Runner) {
		runner.draftObserver = observer
	}
}

func (r *Runner) Run(ctx context.Context, runID string) (AgentSnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := r.manager.beginDrive(runID); err != nil {
		return AgentSnapshot{}, err
	}
	defer r.manager.endDrive(runID)

	runCtx, err := r.manager.runContext(runID)
	if err != nil {
		return AgentSnapshot{}, err
	}

	if ctx.Err() != nil {
		if _, cancelErr := r.manager.Cancel(runID); cancelErr != nil {
			return AgentSnapshot{}, cancelErr
		}
		return r.manager.Snapshot(runID, 0)
	}
	ctx = runCtx

	run, provider, approval, history, prompt, err := r.manager.driveContext(runID)
	if err != nil {
		return AgentSnapshot{}, err
	}
	r.debug(runID, "run.start", fmt.Sprintf(
		"provider=%s model=%s mode=%s channel=%s promptBytes=%d history=%d",
		provider.Type, provider.Model, approval, r.manager.executionChannelFor(runID), len(prompt), len(history),
	))
	if r.service == nil || r.executor == nil {
		snapshot, finishErr := r.converge(runID, RunBlocked, msgAgentExecutorUnavail, "")
		if finishErr != nil {
			return AgentSnapshot{}, finishErr
		}
		return snapshot, ErrExecutorUnavailable
	}

	native := SupportsNativeTools(provider.Type)
	sessionKind := r.manager.sessionKindFor(runID)
	messages := make([]AgentMessage, 0, len(history)+2)
	messages = append(messages, AgentMessage{
		Role:    "system",
		Content: AgentSystemPrompt(!native, sessionKind, r.manager.targetLabelFor(runID)),
	})
	if scope := r.manager.hostScopeFor(runID); len(scope) > 0 {

		messages = append(messages, AgentMessage{
			Role:    "system",
			Content: "The user selected these hosts for this task: " + strings.Join(scope, ", ") + ". Use only these host_id values.",
		})
	}
	if contextText := r.manager.contextFor(runID); contextText != "" {

		messages = append(messages, AgentMessage{
			Role:    "system",
			Content: AgentContextPrefix + redactAgentContext(contextText),
		})
	}
	messages = append(messages, history...)
	messages = append(messages, AgentMessage{Role: "user", Content: prompt})

	maxSteps := r.maxSteps
	if override := r.manager.maxStepsFor(runID); override > 0 {
		maxSteps = override
	}
	modelTimeout, stepTimeout := r.modelTimeout, r.stepTimeout
	if modelOverride, stepOverride := r.manager.limitsFor(runID); modelOverride > 0 || stepOverride > 0 {
		if modelOverride > 0 {
			modelTimeout = modelOverride
		}
		if stepOverride > 0 {
			stepTimeout = stepOverride
		}
	}

	toolFailures := make(map[string]int)
	stopOnRepeatedFailure := func(action AgentAction, reason string) (AgentSnapshot, bool, error) {
		key := toolFailureKey(action)
		toolFailures[key]++
		r.debug(runID, "tool.failure", fmt.Sprintf("kind=%s count=%d detail=%s", action.Kind, toolFailures[key], reason))
		if toolFailures[key] < maxIdenticalToolFailures {
			return AgentSnapshot{}, false, nil
		}
		snapshot, err := r.converge(runID, RunBlocked, msgAgentToolRepeated, reason)
		if err != nil {
			return AgentSnapshot{}, false, err
		}
		return snapshot, true, nil
	}

	for stepIndex := 1; stepIndex <= maxSteps; stepIndex++ {
		r.debug(runID, "turn.request", fmt.Sprintf("step=%d mode=%s promptBytes=%d", stepIndex, boolMode(native), len(messages)))
		turn, err := r.requestTurn(ctx, runID, provider, messages, native, modelTimeout)
		if err != nil {
			r.debug(runID, "turn.error", fmt.Sprintf("mode=%s error=%v", boolMode(native), err))
			if !native || !toolsRequestUnsupported(err) {
				return r.finishRunError(ctx, runID, err)
			}

			native = false
			turn, err = r.requestFallbackTurn(ctx, runID, provider, messages, modelTimeout)
			if err != nil {
				return r.finishRunError(ctx, runID, err)
			}
		}
		r.debugTurn(runID, boolMode(native), turn)
		r.manager.addRunUsage(runID, turn.Usage)
		var action AgentAction
		var prose bool
		if native {
			action, err = parseTurnAction(turn, true)
		} else {
			action, prose, err = r.parseFallbackTurn(runID, turn)
		}
		if prose {

			r.manager.appendNote(runID, turn.Content)
			messages = append(messages,
				AgentMessage{Role: "assistant", Content: strings.TrimSpace(turn.Content)},
				AgentMessage{Role: "user", Content: "继续：如果任务已完成，请调用 final_answer；否则调用下一个需要的工具。不要只说明。"},
			)
			continue
		}
		if err != nil && native {
			r.debug(runID, "turn.parse_error", fmt.Sprintf("mode=native error=%v", err))

			native = false
			turn, err = r.requestFallbackTurn(ctx, runID, provider, messages, modelTimeout)
			if err != nil {
				return r.finishRunError(ctx, runID, err)
			}
			r.debugTurn(runID, "json", turn)
			action, prose, err = r.parseFallbackTurn(runID, turn)
			if prose {
				r.manager.appendNote(runID, turn.Content)
				messages = append(messages,
					AgentMessage{Role: "assistant", Content: strings.TrimSpace(turn.Content)},
					AgentMessage{Role: "user", Content: "继续：如果任务已完成，请调用 final_answer；否则调用下一个需要的工具。不要只说明。"},
				)
				continue
			}
		}
		if err != nil {
			r.debug(runID, "turn.parse_error", fmt.Sprintf("error=%v", err))
			return r.finishRunError(ctx, runID, err)
		}

		if native && action.Kind != StepKindAnswer {
			r.manager.appendNote(runID, turn.Content)
		}
		if err := ValidateActionTarget(action, run.SessionID); err != nil {
			snapshot, finishErr := r.converge(runID, RunBlocked, msgAgentTargetMismatch, "")
			if finishErr != nil {
				return AgentSnapshot{}, finishErr
			}
			return snapshot, err
		}
		callID := strings.TrimSpace(action.CallID)
		if callID == "" {
			callID = fmt.Sprintf("agentcall_step_%d", stepIndex)
		}

		if action.Kind == StepKindAnswer {
			step, beginErr := r.manager.BeginStep(runID, StepSpec{
				CallID: callID,
				Kind:   StepKindAnswer,
				Title:  "",
			})
			if beginErr != nil {
				return r.finishRunError(ctx, runID, beginErr)
			}
			observation, _ := truncateUTF8(action.Answer, r.observationBudget)
			if _, completeErr := r.manager.CompleteStep(runID, step.ID, StepResult{Observation: observation}); completeErr != nil {
				return r.finishRunError(ctx, runID, completeErr)
			}
			snapshot, finishErr := r.manager.Finish(runID, RunCompleted, "", "")
			if finishErr != nil {
				return AgentSnapshot{}, finishErr
			}
			r.debug(runID, "run.finish", fmt.Sprintf("state=%s", snapshot.Run.State))
			return snapshot, nil
		}

		if action.Kind != StepKindAnswer && !StepKindNeedsApproval(action.Kind) {
			step, beginErr := r.manager.BeginStep(runID, StepSpec{
				CallID: callID,
				Kind:   action.Kind,
				Args:   publicToolArgs(action),
			})
			if beginErr != nil {
				return r.finishRunError(ctx, runID, beginErr)
			}
			toolExecutor, supported := r.executor.(ToolExecutor)
			if !supported {
				if _, failErr := r.manager.FailStep(runID, step.ID, StepResult{}, msgAgentCapabilityUnavail, ""); failErr != nil {
					return AgentSnapshot{}, failErr
				}
				messages = append(messages, r.observationMessages(native, turn, callID, action.Kind, "tool is not available in this environment", nil)...)
				continue
			}
			result, toolErr := toolExecutor.ExecuteTool(ctx, ToolRequest{
				Kind:      action.Kind,
				Args:      action.Args,
				SessionID: run.SessionID,
			})
			if toolErr != nil {
				if errors.Is(toolErr, ErrCapabilityUnavailable) {

					if _, failErr := r.manager.FailStep(runID, step.ID, StepResult{}, msgAgentCapabilityUnavail, ""); failErr != nil {
						return AgentSnapshot{}, failErr
					}
					snapshot, finishErr := r.converge(runID, RunBlocked, msgAgentCapabilityUnavail, "")
					if finishErr != nil {
						return AgentSnapshot{}, finishErr
					}
					return snapshot, toolErr
				}
				safeDetail := ""
				if appError := errno.Extract(toolErr); appError != nil {
					safeDetail = appError.SafeDetail
				}
				if _, failErr := r.manager.FailStep(runID, step.ID, StepResult{}, msgAgentExecutionFailed, safeDetail); failErr != nil {
					return r.finishRunError(ctx, runID, failErr)
				}
				reason := toolFailureReason(toolErr)
				if snapshot, stop, failErr := stopOnRepeatedFailure(action, reason); failErr != nil {
					return AgentSnapshot{}, failErr
				} else if stop {
					return snapshot, nil
				}
				messages = append(messages, r.observationMessages(native, turn, callID, action.Kind, reason, nil)...)
				continue
			}
			if result.Failure != "" {

				if _, failErr := r.manager.FailStep(runID, step.ID, result, msgAgentExecutionFailed, result.Failure); failErr != nil {
					return r.finishRunError(ctx, runID, failErr)
				}
				if snapshot, stop, failErr := stopOnRepeatedFailure(action, result.Failure); failErr != nil {
					return AgentSnapshot{}, failErr
				} else if stop {
					return snapshot, nil
				}
				messages = append(messages, r.observationMessages(native, turn, callID, action.Kind, formatObservation(result), result.ExitCode)...)
				continue
			}
			if _, completeErr := r.manager.CompleteStep(runID, step.ID, result); completeErr != nil {
				return r.finishRunError(ctx, runID, completeErr)
			}
			messages = append(messages, r.observationMessages(native, turn, callID, action.Kind, formatObservation(result), result.ExitCode)...)
			continue
		}

		if action.Kind == StepKindAskUser {
			step, beginErr := r.manager.BeginStep(runID, StepSpec{
				CallID: callID,
				Kind:   action.Kind,
				Args:   publicToolArgs(action),
			})
			if beginErr != nil {
				return r.finishRunError(ctx, runID, beginErr)
			}
			preview := ToolPreview{Kind: "ask_user", Data: action.Args}
			pendingID, approvalErr := r.manager.RequestApproval(runID, step.ID, ApprovalRequest{
				Preview:         &preview,
				TargetSessionID: run.SessionID,
				Risk:            RiskReadOnly,
			})
			if approvalErr != nil {
				return r.finishRunError(ctx, runID, approvalErr)
			}
			r.debug(runID, "question.request", fmt.Sprintf("step=%d", step.Index))
			decision, waitErr := r.manager.WaitDecision(ctx, runID, step.ID, pendingID)
			if waitErr != nil {
				return r.finishRunError(ctx, runID, waitErr)
			}
			if !decision.Approved {

				return r.manager.Snapshot(runID, 0)
			}
			observation, marshalErr := json.Marshal(struct {
				Choice string `json:"choice"`
			}{Choice: decision.Choice})
			if marshalErr != nil {
				return r.finishRunError(ctx, runID, agentInvalidAction("choice could not be encoded"))
			}
			if _, completeErr := r.manager.CompleteStep(runID, step.ID, StepResult{Observation: string(observation)}); completeErr != nil {
				return r.finishRunError(ctx, runID, completeErr)
			}
			messages = append(messages, r.observationMessages(native, turn, callID, action.Kind, string(observation), nil)...)
			continue
		}

		if StepKindNeedsApproval(action.Kind) && action.Kind != StepKindCommand {
			effect, knownEffect := ToolSideEffectFor(action.Kind)
			if !knownEffect {
				return r.finishRunError(ctx, runID, agentUnknownTool())
			}
			risk := RiskConfirm
			if effect == SideEffectDestructive {
				risk = RiskDanger
			}
			requirement := ToolApproval(effect, action.Kind, r.manager.toolApproved(runID, action.Kind))
			step, beginErr := r.manager.BeginStep(runID, StepSpec{
				CallID: callID,
				Kind:   action.Kind,
				Args:   publicToolArgs(action),
				Risk:   risk,
			})
			if beginErr != nil {
				return r.finishRunError(ctx, runID, beginErr)
			}
			preparer, supported := r.executor.(ToolPreparer)
			if !supported {
				if _, failErr := r.manager.FailStep(runID, step.ID, StepResult{}, msgAgentCapabilityUnavail, ""); failErr != nil {
					return AgentSnapshot{}, failErr
				}
				snapshot, finishErr := r.converge(runID, RunBlocked, msgAgentCapabilityUnavail, "")
				if finishErr != nil {
					return AgentSnapshot{}, finishErr
				}
				return snapshot, ErrCapabilityUnavailable
			}
			prep, prepErr := preparer.PrepareTool(ctx, ToolRequest{
				Kind:      action.Kind,
				Args:      action.Args,
				SessionID: run.SessionID,
			})
			if prepErr != nil {
				if errors.Is(prepErr, ErrCapabilityUnavailable) {
					if _, failErr := r.manager.FailStep(runID, step.ID, StepResult{}, msgAgentCapabilityUnavail, ""); failErr != nil {
						return AgentSnapshot{}, failErr
					}
					snapshot, finishErr := r.converge(runID, RunBlocked, msgAgentCapabilityUnavail, "")
					if finishErr != nil {
						return AgentSnapshot{}, finishErr
					}
					return snapshot, prepErr
				}
				safeDetail := ""
				if appError := errno.Extract(prepErr); appError != nil {
					safeDetail = appError.SafeDetail
				}
				if _, failErr := r.manager.FailStep(runID, step.ID, StepResult{}, msgAgentExecutionFailed, safeDetail); failErr != nil {
					return r.finishRunError(ctx, runID, failErr)
				}
				reason := toolFailureReason(prepErr)
				if snapshot, stop, failErr := stopOnRepeatedFailure(action, reason); failErr != nil {
					return AgentSnapshot{}, failErr
				} else if stop {
					return snapshot, nil
				}
				messages = append(messages, r.observationMessages(native, turn, callID, action.Kind, reason, nil)...)
				continue
			}
			if requirement.Required {
				preview := prep.Preview
				pendingID, approvalErr := r.manager.RequestApproval(runID, step.ID, ApprovalRequest{
					Preview:         &preview,
					Rememberable:    requirement.Rememberable,
					TargetSessionID: run.SessionID,
					Risk:            risk,
				})
				if approvalErr != nil {
					return r.finishRunError(ctx, runID, approvalErr)
				}
				r.debug(runID, "approval.request", fmt.Sprintf("step=%d kind=%s", step.Index, action.Kind))
				decision, waitErr := r.manager.WaitDecision(ctx, runID, step.ID, pendingID)
				if waitErr != nil {
					return r.finishRunError(ctx, runID, waitErr)
				}
				if !decision.Approved {
					return r.manager.Snapshot(runID, 0)
				}
			} else {

				r.manager.noteAuditDecision(runID, step.ID, AuditDecisionAuto)
			}
			toolExecutor, execSupported := r.executor.(ToolExecutor)
			if !execSupported {
				if _, failErr := r.manager.FailStep(runID, step.ID, StepResult{}, msgAgentCapabilityUnavail, ""); failErr != nil {
					return AgentSnapshot{}, failErr
				}
				snapshot, finishErr := r.converge(runID, RunBlocked, msgAgentCapabilityUnavail, "")
				if finishErr != nil {
					return AgentSnapshot{}, finishErr
				}
				return snapshot, ErrCapabilityUnavailable
			}
			result, toolErr := toolExecutor.ExecuteTool(ctx, ToolRequest{
				Kind:      action.Kind,
				Args:      prep.Args,
				SessionID: run.SessionID,
			})
			if toolErr != nil {
				if errors.Is(toolErr, ErrCapabilityUnavailable) {

					if _, failErr := r.manager.FailStep(runID, step.ID, StepResult{}, msgAgentCapabilityUnavail, ""); failErr != nil {
						return AgentSnapshot{}, failErr
					}
					snapshot, finishErr := r.converge(runID, RunBlocked, msgAgentCapabilityUnavail, "")
					if finishErr != nil {
						return AgentSnapshot{}, finishErr
					}
					return snapshot, toolErr
				}
				safeDetail := ""
				if appError := errno.Extract(toolErr); appError != nil {
					safeDetail = appError.SafeDetail
				}
				if _, failErr := r.manager.FailStep(runID, step.ID, StepResult{}, msgAgentExecutionFailed, safeDetail); failErr != nil {
					return r.finishRunError(ctx, runID, failErr)
				}
				reason := toolFailureReason(toolErr)
				if snapshot, stop, failErr := stopOnRepeatedFailure(action, reason); failErr != nil {
					return AgentSnapshot{}, failErr
				} else if stop {
					return snapshot, nil
				}
				messages = append(messages, r.observationMessages(native, turn, callID, action.Kind, reason, nil)...)
				continue
			}
			if result.Failure != "" {
				if _, failErr := r.manager.FailStep(runID, step.ID, result, msgAgentExecutionFailed, result.Failure); failErr != nil {
					return r.finishRunError(ctx, runID, failErr)
				}
				if snapshot, stop, failErr := stopOnRepeatedFailure(action, result.Failure); failErr != nil {
					return AgentSnapshot{}, failErr
				} else if stop {
					return snapshot, nil
				}
				messages = append(messages, r.observationMessages(native, turn, callID, action.Kind, formatObservation(result), result.ExitCode)...)
				continue
			}
			if _, completeErr := r.manager.CompleteStep(runID, step.ID, result); completeErr != nil {
				return r.finishRunError(ctx, runID, completeErr)
			}
			messages = append(messages, r.observationMessages(native, turn, callID, action.Kind, formatObservation(result), result.ExitCode)...)
			continue
		}

		if action.Kind != StepKindCommand {
			return r.finishRunError(ctx, runID, agentUnknownTool())
		}

		targetSessionID := run.SessionID
		var pendingConnector HostSessionConnector
		var pendingHostID string
		autoConnected := false
		if targetSessionID == "" {

			if scope := r.manager.hostScopeFor(runID); len(scope) > 0 && !hostInScope(scope, action.HostID) {
				step, beginErr := r.manager.BeginStep(runID, StepSpec{
					CallID:  callID,
					Kind:    StepKindCommand,
					Title:   action.Description,
					Command: action.Command,
				})
				if beginErr != nil {
					return r.finishRunError(ctx, runID, beginErr)
				}
				reason := "code=OUT_OF_SCOPE key=ai.agent.host_out_of_scope"
				if _, failErr := r.manager.FailStep(runID, step.ID, StepResult{}, msgAgentExecutionFailed, reason); failErr != nil {
					return r.finishRunError(ctx, runID, failErr)
				}
				messages = append(messages, r.observationMessages(native, turn, callID, action.Kind, reason, nil)...)
				continue
			}
			resolver, supported := r.executor.(HostSessionResolver)
			if !supported {

				return r.finishRunError(ctx, runID, ErrCapabilityUnavailable)
			}
			resolved, resolveErr := resolver.SessionForHost(ctx, action.HostID)
			if resolveErr != nil {

				if connector, canConnect := r.executor.(HostSessionConnector); canConnect && errors.Is(resolveErr, ErrNoOpenSession) {
					pendingConnector = connector
					pendingHostID = action.HostID
				} else {
					reason := "code=NO_OPEN_SESSION key=ai.agent.no_open_session"
					if !errors.Is(resolveErr, ErrNoOpenSession) {
						reason = toolFailureReason(resolveErr)
					}
					step, beginErr := r.manager.BeginStep(runID, StepSpec{
						CallID:  callID,
						Kind:    StepKindCommand,
						Title:   action.Description,
						Command: action.Command,
					})
					if beginErr != nil {
						return r.finishRunError(ctx, runID, beginErr)
					}
					if _, failErr := r.manager.FailStep(runID, step.ID, StepResult{}, msgAgentExecutionFailed, reason); failErr != nil {
						return r.finishRunError(ctx, runID, failErr)
					}
					messages = append(messages, r.observationMessages(native, turn, callID, action.Kind, reason, nil)...)
					continue
				}
			} else {
				targetSessionID = resolved
			}
		}

		needsApproval, risk, riskReason := RequiresApproval(approval, action.Command, action.Risk, action.RiskReason)
		step, beginErr := r.manager.BeginStep(runID, StepSpec{
			CallID:  callID,
			Kind:    StepKindCommand,
			Title:   action.Description,
			Command: action.Command,

			TargetSessionID: run.SessionID,
			Risk:            risk,
			RiskReason:      riskReason,
		})
		if beginErr != nil {
			return r.finishRunError(ctx, runID, beginErr)
		}
		if needsApproval {
			r.debug(runID, "approval.request", fmt.Sprintf("step=%d risk=%s", step.Index, risk))
			pendingID, approvalErr := r.manager.RequestApproval(runID, step.ID, ApprovalRequest{
				Command:         action.Command,
				TargetSessionID: run.SessionID,
				Risk:            risk,
				RiskReason:      riskReason,
			})
			if approvalErr != nil {
				return r.finishRunError(ctx, runID, approvalErr)
			}
			decision, waitErr := r.manager.WaitDecision(ctx, runID, step.ID, pendingID)
			if waitErr != nil {
				return r.finishRunError(ctx, runID, waitErr)
			}
			r.debug(runID, "approval.decision", fmt.Sprintf("step=%d approved=%v", step.Index, decision.Approved))
			if !decision.Approved {

				return r.manager.Snapshot(runID, 0)
			}
		} else {
			r.manager.noteAuditDecision(runID, step.ID, AuditDecisionAuto)
		}

		if pendingConnector != nil {
			connectedSessionID, connectErr := pendingConnector.ConnectForHost(ctx, pendingHostID)
			if connectErr != nil {
				reason := toolFailureReason(connectErr)
				if errors.Is(connectErr, ErrConnectNeedsManual) {

					reason = "code=CONNECT_NEEDS_MANUAL key=ai.agent.connect_needs_manual"
				}
				if _, failErr := r.manager.FailStep(runID, step.ID, StepResult{}, msgAgentExecutionFailed, reason); failErr != nil {
					return r.finishRunError(ctx, runID, failErr)
				}
				messages = append(messages, r.observationMessages(native, turn, callID, action.Kind, reason, nil)...)
				continue
			}

			if strings.TrimSpace(connectedSessionID) == "" {
				reason := toolFailureReason(ErrCapabilityUnavailable)
				if _, failErr := r.manager.FailStep(runID, step.ID, StepResult{}, msgAgentExecutionFailed, reason); failErr != nil {
					return r.finishRunError(ctx, runID, failErr)
				}
				messages = append(messages, r.observationMessages(native, turn, callID, action.Kind, reason, nil)...)
				continue
			}
			targetSessionID = connectedSessionID
			autoConnected = true
		}

		r.manager.markCommandRun(runID)
		channel := r.manager.executionChannelFor(runID)
		r.debug(runID, "exec.request", fmt.Sprintf("step=%d channel=%s command=%s", step.Index, channel, debugCommand(action.Command)))
		result, execErr := r.executeStep(ctx, targetSessionID, action.Command, stepTimeout, channel)
		r.debug(runID, "exec.result", fmt.Sprintf("step=%d exit=%v observationBytes=%d error=%v", step.Index, result.ExitCode, len(result.Observation), execErr))
		if execErr != nil && errors.Is(execErr, ErrCapabilityUnavailable) {
			if _, failErr := r.manager.FailStep(runID, step.ID, StepResult{}, msgAgentCapabilityUnavail, ""); failErr != nil {
				return AgentSnapshot{}, failErr
			}
			snapshot, finishErr := r.converge(runID, RunBlocked, msgAgentCapabilityUnavail, "")
			if finishErr != nil {
				return AgentSnapshot{}, finishErr
			}
			return snapshot, execErr
		}
		if execErr != nil {
			safeDetail := ""
			if appError := errno.Extract(execErr); appError != nil {
				safeDetail = appError.SafeDetail
			}
			if _, failErr := r.manager.FailStep(runID, step.ID, StepResult{}, msgAgentExecutionFailed, safeDetail); failErr != nil {
				return r.finishRunError(ctx, runID, failErr)
			}
			messages = append(messages, r.observationMessages(native, turn, callID, action.Kind, "command execution failed", nil)...)
			continue
		}
		observation := formatObservation(result)
		if autoConnected {

			observation = temporarySessionNote(pendingHostID) + "\n" + observation
			result.Observation = observation
		}
		if _, completeErr := r.manager.CompleteStep(runID, step.ID, result); completeErr != nil {
			return r.finishRunError(ctx, runID, completeErr)
		}
		messages = append(messages, r.observationMessages(native, turn, callID, action.Kind, observation, result.ExitCode)...)
	}

	return r.finishFailure(runID, RunBlocked, msgAgentMaxSteps, "")
}

const maxIdenticalToolFailures = 2

func toolFailureKey(action AgentAction) string {
	sum := sha256.Sum256(action.Args)
	return string(action.Kind) + ":" + hex.EncodeToString(sum[:8])
}

func temporarySessionNote(hostID string) string {
	return "[temporary session] 已按主机配置建立临时连接（" + hostID + "）"
}

func toolFailureReason(err error) string {
	if errors.Is(err, ErrCapabilityUnavailable) {

		return "code=CAPABILITY_UNAVAILABLE"
	}
	if appError := errno.Extract(err); appError != nil {
		reason := "code=" + string(appError.Code)
		if appError.MessageKey != "" {
			reason += " key=" + appError.MessageKey
		}
		return reason
	}
	return "tool failed"
}

func publicToolArgs(action AgentAction) json.RawMessage {
	if len(action.PublicArgs) > 0 {
		return action.PublicArgs
	}
	return action.Args
}

func (r *Runner) requestTurn(ctx context.Context, runID string, provider Provider, messages []AgentMessage, native bool, timeout time.Duration) (AgentTurn, error) {
	r.manager.resetDraft(runID)

	messages = compressAgentMessages(messages, r.promptBudget)
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if !native {

		return r.requestFallbackTurn(ctx, runID, provider, messages, timeout)
	}

	tools := AgentToolsFor(r.manager.sessionKindFor(runID))
	r.manager.RecordContext(runID, MeasureAgentContext(messages, tools, r.promptBudget))

	var answerArgs strings.Builder
	return r.service.StreamAgentTurn(requestCtx, provider, messages, tools, func(delta AgentDelta) error {
		if delta.Content != "" {
			r.manager.appendDraft(runID, delta.Content)
		}

		if delta.ToolArgs != "" {
			answerArgs.WriteString(delta.ToolArgs)
			raw := answerArgs.String()
			if isFinalAnswerToolName(delta.ToolName) || strings.Contains(raw, `"answer"`) {
				if text := partialJSONStringField(raw, "answer"); text != "" {
					r.manager.setDraft(runID, text)
				}
			}
		}

		if delta.Reasoning != "" {
			r.manager.appendReasoningDraft(runID, delta.Reasoning)
		}

		if (delta.Content != "" || delta.Reasoning != "" || delta.ToolArgs != "") && r.draftObserver != nil {
			r.draftObserver(runID)
		}
		return nil
	})
}

func (r *Runner) executeStep(ctx context.Context, sessionID, command string, timeout time.Duration, channel string) (StepResult, error) {
	stepCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result, err := r.executor.Execute(stepCtx, ExecRequest{SessionID: sessionID, Command: command, Channel: channel})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return StepResult{}, errno.New(errno.CodeTimeout, "ai.agent.command_timeout", "")
		}
		return StepResult{}, err
	}
	return result, nil
}

func parseTurnAction(turn AgentTurn, native bool) (AgentAction, error) {
	if native {
		return ParseToolCalls(turn.ToolCalls)
	}
	action, err := ParseFallbackAction(turn.Content)
	if err != nil {
		return AgentAction{}, err
	}
	return action, nil
}

func (r *Runner) observationMessages(
	native bool,
	turn AgentTurn,
	callID string,
	toolName StepKind,
	observation string,
	exitCode *int,
) []AgentMessage {
	content, _ := truncateUTF8(observation, r.observationBudget)
	if exitCode != nil {
		content = fmt.Sprintf("%s\n[exit code: %d]", content, *exitCode)
	} else {
		content = content + "\n[exit code: unknown]"
	}
	if !native {
		return []AgentMessage{
			{Role: "assistant", Content: turn.Content},
			{Role: "user", Content: "OBSERVATION:\n" + content},
		}
	}
	call := ToolCall{ID: callID, Name: string(toolName), Arguments: rawToolArguments(turn, callID)}
	return []AgentMessage{
		{Role: "assistant", ToolCalls: []ToolCall{call}},
		{Role: "tool", ToolCallID: callID, Name: string(toolName), Content: content},
	}
}

func rawToolArguments(turn AgentTurn, callID string) string {
	for _, call := range turn.ToolCalls {
		if strings.TrimSpace(call.Arguments) != "" {
			return call.Arguments
		}
	}
	_ = callID
	return "{}"
}

func (r *Runner) finishRunError(ctx context.Context, runID string, err error) (AgentSnapshot, error) {
	if ctx.Err() != nil {

		snapshot, cancelErr := r.manager.Cancel(runID)
		if cancelErr != nil {
			return AgentSnapshot{}, cancelErr
		}
		return snapshot, nil
	}
	appError := errno.Extract(err)
	state := RunFailed
	errorKey := msgAgentModelFailed
	detail := ""
	if appError != nil {
		errorKey = appError.MessageKey
		detail = appError.SafeDetail
		if appError.Code == errno.CodeTimeout {
			state = RunTimedOut
		}
	}
	snapshot, finishErr := r.converge(runID, state, errorKey, detail)
	if finishErr != nil {
		return AgentSnapshot{}, finishErr
	}
	r.debug(runID, "run.finish", fmt.Sprintf("state=%s key=%s detail=%s", snapshot.Run.State, errorKey, detail))

	return snapshot, err
}

func (r *Runner) finishFailure(runID string, state RunState, errorKey, detail string) (AgentSnapshot, error) {
	return r.converge(runID, state, errorKey, detail)
}

func (r *Runner) converge(runID string, state RunState, errorKey, detail string) (AgentSnapshot, error) {
	snapshot, err := r.manager.Finish(runID, state, errorKey, detail)
	if err != nil {
		return AgentSnapshot{}, err
	}
	return snapshot, nil
}

func formatObservation(result StepResult) string {
	observation := strings.TrimRight(result.Observation, "\n")
	if strings.TrimSpace(observation) == "" {

		return "(command produced no output)"
	}
	return observation
}

func capHistory(history []AgentMessage) []AgentMessage {
	if len(history) == 0 {
		return nil
	}
	if len(history) > maxAgentHistoryMessages {
		history = history[len(history)-maxAgentHistoryMessages:]
	}
	total := 0
	start := len(history)
	for index := len(history) - 1; index >= 0; index-- {
		total += len(history[index].Content) + len(history[index].ToolCallID) + len(history[index].Name)
		if total > maxAgentHistoryBytes {
			break
		}
		start = index
	}
	if start >= len(history) {
		return nil
	}
	capped := make([]AgentMessage, len(history)-start)
	copy(capped, history[start:])
	return capped
}

func toolsRequestUnsupported(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrToolsUnsupported) {
		return true
	}
	var status *StatusError
	if errors.As(err, &status) {
		switch status.Status {
		case 400, 404, 422, 501:
			return true
		}
	}
	return false
}

func (r *Runner) requestFallbackTurn(ctx context.Context, runID string, provider Provider, messages []AgentMessage, timeout time.Duration) (AgentTurn, error) {
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	r.manager.resetDraft(runID)

	breakdown := MeasureAgentContext(messages, nil, r.promptBudget)
	r.manager.RecordContext(runID, breakdown)
	request := ChatRequest{
		Provider: provider,
		Messages: toFallbackMessages(messages, r.manager.sessionKindFor(runID), r.manager.targetLabelFor(runID)),
		JSONMode: true,
		Context:  breakdown,
	}
	content, err := r.streamFallback(requestCtx, request)
	if err != nil {

		if !toolsRequestUnsupported(err) {
			return AgentTurn{}, err
		}
		request.JSONMode = false
		content, err = r.streamFallback(requestCtx, request)
		if err != nil {
			return AgentTurn{}, err
		}
	}
	return AgentTurn{Content: content}, nil
}

func (r *Runner) streamFallback(ctx context.Context, request ChatRequest) (string, error) {
	var content strings.Builder
	err := r.service.StreamChat(ctx, request, func(delta string, _ string, _ *Usage) error {
		content.WriteString(delta)
		return nil
	})
	if err != nil {
		return "", err
	}
	return content.String(), nil
}

func toFallbackMessages(messages []AgentMessage, sessionKind, targetLabel string) []Message {
	converted := make([]Message, 0, len(messages)+2)
	appendMessage := func(role, content string) {
		if content == "" {
			return
		}

		if len(converted) > 0 && converted[len(converted)-1].Role == role {
			converted[len(converted)-1].Content += "\n\n" + content
			return
		}
		converted = append(converted, Message{Role: role, Content: content})
	}
	appendMessage("system", AgentSystemPrompt(true, sessionKind, targetLabel))
	for index, message := range messages {
		if index == 0 && message.Role == "system" {

			continue
		}
		switch {
		case message.Role == "system":
			appendMessage("system", message.Content)
		case len(message.ToolCalls) > 0:
			call := message.ToolCalls[0]
			appendMessage("assistant", fmt.Sprintf(`{"action":%q,"arguments":%s}`, call.Name, call.Arguments))
		case message.Role == "tool":
			appendMessage("user", "OBSERVATION:\n"+message.Content)
		default:
			appendMessage(message.Role, message.Content)
		}
	}

	appendMessage("user", "Reply now with exactly ONE JSON object: either "+`{"action":"execute_command","arguments":{...}}`+" or "+`{"action":"final_answer","arguments":{"answer":"..."}}`+". No prose or code fences.")
	return converted
}

func (r *Runner) parseFallbackTurn(runID string, turn AgentTurn) (AgentAction, bool, error) {
	if action, err := ParseFallbackAction(turn.Content); err == nil {
		return action, false, nil
	}
	if action, ok := ExtractJSONAction(turn.Content); ok {
		r.debug(runID, "turn.extracted_action", fmt.Sprintf("kind=%s", action.Kind))
		return action, false, nil
	}

	if answer, ok := salvageFinalAnswer(turn.Content); ok {
		r.debug(runID, "turn.salvaged_answer", "final_answer")
		return AgentAction{Kind: StepKindAnswer, Answer: answer}, false, nil
	}
	prose := strings.TrimSpace(turn.Content)
	if prose == "" {
		return AgentAction{}, false, agentInvalidAction("model returned empty response")
	}
	r.debug(runID, "turn.prose_note", debugExcerpt(prose))
	return AgentAction{}, true, nil
}

func hostInScope(scope []string, hostID string) bool {
	trimmed := strings.TrimSpace(hostID)
	for _, candidate := range scope {
		if candidate == trimmed {
			return true
		}
	}
	return false
}
