package ai

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	defaultRunTimeout     = 30 * time.Minute
	defaultApprovalTTL    = 5 * time.Minute
	defaultMaxRuns        = 32
	defaultMaxObservation = 64 << 10
	defaultMaxEvents      = 512
	defaultMaxAudit       = 500
	maxStartPromptBytes   = 64 << 10
	agentIDRandomBytes    = 12
	agentIDPrefixRun      = "airun"
	agentIDPrefixStep     = "aistep"
	agentIDPrefixPending  = "aipending"
)

type ManagerOption func(*Manager)

func WithRunTimeout(timeout time.Duration) ManagerOption {
	return func(manager *Manager) {
		if timeout > 0 {
			manager.runTimeout = timeout
		}
	}
}

func WithApprovalTTL(ttl time.Duration) ManagerOption {
	return func(manager *Manager) {
		if ttl > 0 {
			manager.approvalTTL = ttl
		}
	}
}

func WithMaxRuns(limit int) ManagerOption {
	return func(manager *Manager) {
		if limit > 0 {
			manager.maxRuns = limit
		}
	}
}

func WithMaxObservation(limit int) ManagerOption {
	return func(manager *Manager) {
		if limit > 0 {
			manager.maxObservation = limit
		}
	}
}

func WithMaxEvents(limit int) ManagerOption {
	return func(manager *Manager) {
		if limit > 0 {
			manager.maxEvents = limit
		}
	}
}

func WithMaxAudit(limit int) ManagerOption {
	return func(manager *Manager) {
		if limit > 0 {
			manager.maxAudit = limit
		}
	}
}

func WithClock(now func() time.Time) ManagerOption {
	return func(manager *Manager) {
		if now != nil {
			manager.now = now
		}
	}
}

type Manager struct {
	mu             sync.Mutex
	runs           map[string]*agentRunState
	runTimeout     time.Duration
	approvalTTL    time.Duration
	maxRuns        int
	maxObservation int
	maxEvents      int
	maxAudit       int
	audit          []AgentAuditEntry
	now            func() time.Time
	closed         bool
}

type agentRunState struct {
	run          AgentRun
	provider     Provider
	approval     ApprovalMode
	history      []AgentMessage
	maxSteps     int
	channel      string
	contextText  string
	modelTimeout time.Duration
	stepTimeout  time.Duration
	driving      bool
	lastNoteAt   time.Time
	runCtx       context.Context
	cancel       context.CancelFunc
	deadline     time.Time
	cursor       int
	earliest     int
	events       []AgentEvent
	pending      map[string]*agentPending
}

type agentPending struct {
	stepID        string
	sessionID     string
	policyVersion string
	expiresAt     time.Time

	decision chan bool
}

func NewManager(options ...ManagerOption) *Manager {
	manager := &Manager{
		runs:           make(map[string]*agentRunState),
		runTimeout:     defaultRunTimeout,
		approvalTTL:    defaultApprovalTTL,
		maxRuns:        defaultMaxRuns,
		maxObservation: defaultMaxObservation,
		maxEvents:      defaultMaxEvents,
		maxAudit:       defaultMaxAudit,
		now:            time.Now,
	}
	for _, option := range options {
		if option != nil {
			option(manager)
		}
	}
	return manager
}

func (m *Manager) Start(ctx context.Context, input StartInput) (AgentSnapshot, error) {
	providerID := strings.TrimSpace(input.ProviderID)
	providerType := input.ProviderType
	baseURL := strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	model := strings.TrimSpace(input.Model)
	sessionID := strings.TrimSpace(input.SessionID)
	prompt := strings.TrimSpace(input.Prompt)
	if providerID == "" || model == "" || sessionID == "" || prompt == "" {
		return AgentSnapshot{}, agentInvalidArg("provider, model, session and prompt are required")
	}
	if providerType != ProviderTypeOpenAI && providerType != ProviderTypeOpenAIResponse && providerType != ProviderTypeAnthropic {
		return AgentSnapshot{}, agentInvalidArg("provider type is invalid")
	}
	if baseURL == "" {
		return AgentSnapshot{}, agentInvalidArg("provider base url is required")
	}
	if len(prompt) > maxStartPromptBytes {
		return AgentSnapshot{}, agentInvalidArg("prompt is too large")
	}
	contextText := strings.TrimSpace(input.ContextText)
	if len(contextText) > 8<<10 {
		contextText = contextText[:8<<10]
	}
	if input.MaxSteps < 0 || input.MaxSteps > maxAgentMaxSteps {
		return AgentSnapshot{}, agentInvalidArg("max steps is out of range")
	}
	channel := strings.TrimSpace(input.ExecutionChannel)
	if channel == "" {
		channel = ExecutionBackground
	}
	if channel != ExecutionBackground && channel != ExecutionTerminal {
		return AgentSnapshot{}, agentInvalidArg("unknown execution channel")
	}
	if input.ModelTimeoutSeconds < 0 || input.ModelTimeoutSeconds > 600 {
		return AgentSnapshot{}, agentInvalidArg("model timeout is out of range")
	}
	if input.StepTimeoutSeconds < 0 || input.StepTimeoutSeconds > 600 {
		return AgentSnapshot{}, agentInvalidArg("step timeout is out of range")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return AgentSnapshot{}, agentClosed()
	}
	now := m.now()
	m.expireLocked(now)
	if len(m.runs) >= m.maxRuns {
		return AgentSnapshot{}, agentTooManyRuns()
	}
	runID, err := newAgentID(agentIDPrefixRun)
	if err != nil {
		return AgentSnapshot{}, err
	}

	baseCtx := context.WithoutCancel(ctx)
	runCtx, cancel := context.WithCancel(baseCtx)
	state := &agentRunState{
		run: AgentRun{
			ID:            runID,
			ProviderID:    providerID,
			Model:         model,
			SessionID:     sessionID,
			Prompt:        prompt,
			PolicyVersion: strings.TrimSpace(input.PolicyVersion),
			State:         RunRunning,
			Steps:         []AgentStep{},
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		provider: Provider{
			ID:      providerID,
			Type:    providerType,
			BaseURL: baseURL,
			Model:   model,
		},
		approval:     ParseApprovalMode(string(input.ApprovalMode)),
		history:      capHistory(input.History),
		maxSteps:     input.MaxSteps,
		channel:      channel,
		contextText:  contextText,
		modelTimeout: clampSeconds(input.ModelTimeoutSeconds, 5, 600),
		stepTimeout:  clampSeconds(input.StepTimeoutSeconds, 5, 600),
		runCtx:       runCtx,
		cancel:       cancel,
		deadline:     now.Add(m.runTimeout),
		earliest:     1,
		pending:      make(map[string]*agentPending),
	}
	m.runs[runID] = state
	m.appendEventLocked(state, "run", "", string(RunRunning))
	return m.snapshotLocked(state, 0), nil
}

func (m *Manager) Snapshot(runID string, since int) (AgentSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked(m.now())
	state, ok := m.runs[strings.TrimSpace(runID)]
	if !ok {
		return AgentSnapshot{}, agentRunNotFound()
	}
	return m.snapshotLocked(state, since), nil
}

func (m *Manager) Decide(input DecideInput) (AgentSnapshot, error) {
	runID := strings.TrimSpace(input.RunID)
	pendingID := strings.TrimSpace(input.PendingID)
	if runID == "" || pendingID == "" {
		return AgentSnapshot{}, agentInvalidArg("runId and pendingId are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.expireLocked(now)
	state, ok := m.runs[runID]
	if !ok {
		return AgentSnapshot{}, agentRunNotFound()
	}
	pending, ok := state.pending[pendingID]
	if !ok {
		return AgentSnapshot{}, agentApprovalNotFound()
	}
	if stepID := strings.TrimSpace(input.StepID); stepID != "" && stepID != pending.stepID {

		return AgentSnapshot{}, agentApprovalNotFound()
	}
	if strings.TrimSpace(input.SessionID) != pending.sessionID {
		return AgentSnapshot{}, agentTargetMismatch()
	}
	if strings.TrimSpace(input.PolicyVersion) != pending.policyVersion {
		return AgentSnapshot{}, agentPolicyMismatch()
	}
	if state.run.State != RunWaitingApproval {
		return AgentSnapshot{}, agentInvalidState("run is not waiting for approval")
	}
	step := findStep(state, pending.stepID)
	if step == nil || step.State != StepWaitingApproval {

		delete(state.pending, pendingID)
		return AgentSnapshot{}, agentApprovalNotFound()
	}

	delete(state.pending, pendingID)
	step.PendingID = ""
	state.run.UpdatedAt = now

	if input.Approve {
		step.State = StepRunning
		state.run.State = RunRunning
		m.appendEventLocked(state, "approval", step.ID, "approved")
		m.appendEventLocked(state, "step", step.ID, string(StepRunning))
		m.appendEventLocked(state, "run", "", string(RunRunning))
		if entry := m.auditFor(runID, step.ID); entry != nil {
			entry.Decision = AuditDecisionApproved
			entry.Risk = step.Risk
		}
		notifyDecision(pending, true)
		return m.snapshotLocked(state, input.Since), nil
	}
	step.State = StepRejected
	step.FinishedAt = &now
	if entry := m.auditFor(runID, step.ID); entry != nil {
		entry.Decision = AuditDecisionRejected
		entry.Risk = step.Risk
	}
	m.finishAuditLocked(runID, step.ID, StepRejected, nil, now)
	m.appendEventLocked(state, "approval", step.ID, "rejected")
	notifyDecision(pending, false)

	m.finishLocked(state, RunRejected, msgAgentRejected, "", now)
	return m.snapshotLocked(state, input.Since), nil
}

func (m *Manager) Cancel(runID string) (AgentSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.expireLocked(now)
	state, ok := m.runs[strings.TrimSpace(runID)]
	if !ok {
		return AgentSnapshot{}, agentRunNotFound()
	}
	m.finishLocked(state, RunCancelled, msgAgentCancelled, "", now)
	return m.snapshotLocked(state, 0), nil
}

func (m *Manager) Release(runID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := strings.TrimSpace(runID)
	state, ok := m.runs[id]
	if !ok {
		return nil
	}
	m.finishLocked(state, RunCancelled, msgAgentCancelled, "", m.now())
	delete(m.runs, id)
	return nil
}

func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	for id, state := range m.runs {
		m.finishLocked(state, RunCancelled, msgAgentCancelled, "", m.now())
		delete(m.runs, id)
	}
	return nil
}

func (m *Manager) BeginStep(runID string, spec StepSpec) (AgentStep, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return AgentStep{}, agentInvalidArg("runId is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.expireLocked(now)
	state, ok := m.runs[runID]
	if !ok {
		return AgentStep{}, agentRunNotFound()
	}
	if state.run.State != RunRunning {
		return AgentStep{}, agentInvalidState("run is not running")
	}
	for _, step := range state.run.Steps {
		if !isTerminalStep(step.State) {
			return AgentStep{}, agentInvalidState("a previous step is still active")
		}
	}
	if spec.Kind != StepKindCommand && spec.Kind != StepKindAnswer {
		return AgentStep{}, agentInvalidArg("unknown step kind")
	}
	command := strings.TrimSpace(spec.Command)
	if spec.Kind == StepKindCommand && command == "" {
		return AgentStep{}, agentInvalidArg("command is required")
	}
	callID := strings.TrimSpace(spec.CallID)
	if callID != "" {
		for _, existing := range state.run.Steps {
			if existing.CallID == callID {
				return AgentStep{}, agentDuplicateCall()
			}
		}
	}
	targetSessionID := strings.TrimSpace(spec.TargetSessionID)
	if targetSessionID != "" && targetSessionID != state.run.SessionID {

		return AgentStep{}, agentTargetMismatch()
	}
	stepID, err := newAgentID(agentIDPrefixStep)
	if err != nil {
		return AgentStep{}, err
	}
	step := AgentStep{
		ID:              stepID,
		Index:           len(state.run.Steps) + 1,
		CallID:          callID,
		Kind:            spec.Kind,
		State:           StepRunning,
		Title:           strings.TrimSpace(spec.Title),
		Command:         command,
		TargetSessionID: targetSessionID,
		Risk:            normalizeRisk(spec.Risk),
		RiskReason:      strings.TrimSpace(spec.RiskReason),
		StartedAt:       now,
	}
	state.run.Steps = append(state.run.Steps, step)
	state.run.UpdatedAt = now
	if step.Kind == StepKindCommand {
		m.appendAuditLocked(AgentAuditEntry{
			RunID:     runID,
			StepID:    stepID,
			CallID:    callID,
			SessionID: state.run.SessionID,
			Command:   redactAgentCommand(command),
			Risk:      step.Risk,
			State:     string(StepRunning),
			StartedAt: now,
		})
	}
	m.appendEventLocked(state, "step", stepID, string(StepRunning))
	return cloneStep(step), nil
}

func (m *Manager) RequestApproval(runID, stepID string, request ApprovalRequest) (string, error) {
	runID = strings.TrimSpace(runID)
	stepID = strings.TrimSpace(stepID)
	if runID == "" || stepID == "" {
		return "", agentInvalidArg("runId and stepId are required")
	}
	command := strings.TrimSpace(request.Command)
	if command == "" {
		return "", agentInvalidArg("command is required")
	}
	target := strings.TrimSpace(request.TargetSessionID)

	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.expireLocked(now)
	state, ok := m.runs[runID]
	if !ok {
		return "", agentRunNotFound()
	}
	if state.run.State != RunRunning {
		return "", agentInvalidState("run is not running")
	}
	if target != state.run.SessionID {
		return "", agentTargetMismatch()
	}
	step := findStep(state, stepID)
	if step == nil {
		return "", agentInvalidArg("step not found")
	}
	if step.Kind != StepKindCommand {
		return "", agentInvalidState("only command steps need approval")
	}
	if step.State != StepRunning {
		return "", agentInvalidState("step is not running")
	}
	pendingID, err := newAgentID(agentIDPrefixPending)
	if err != nil {
		return "", err
	}
	state.pending[pendingID] = &agentPending{
		stepID:        stepID,
		sessionID:     state.run.SessionID,
		policyVersion: state.run.PolicyVersion,
		expiresAt:     now.Add(m.approvalTTL),
		decision:      make(chan bool, 1),
	}
	step.State = StepWaitingApproval
	step.PendingID = pendingID
	if entry := m.auditFor(runID, stepID); entry != nil {
		entry.Decision = AuditDecisionNeedsApproval
		entry.Risk = step.Risk
	}
	step.Command = command
	step.TargetSessionID = target
	step.Risk = normalizeRisk(request.Risk)
	step.RiskReason = strings.TrimSpace(request.RiskReason)
	state.run.State = RunWaitingApproval
	state.run.UpdatedAt = now
	m.appendEventLocked(state, "step", stepID, string(StepWaitingApproval))
	m.appendEventLocked(state, "approval", stepID, "requested")
	m.appendEventLocked(state, "run", "", string(RunWaitingApproval))
	return pendingID, nil
}

func (m *Manager) CompleteStep(runID, stepID string, result StepResult) (AgentStep, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.expireLocked(now)
	state, step, err := m.mutableStepLocked(runID, stepID)
	if err != nil {
		return AgentStep{}, err
	}
	observation, truncated := truncateUTF8(result.Observation, m.maxObservation)
	step.State = StepCompleted
	step.Observation = observation
	step.ObservationTruncated = truncated
	step.ExitCode = cloneInt(result.ExitCode)
	step.FinishedAt = &now
	state.run.UpdatedAt = now
	m.finishAuditLocked(runID, step.ID, StepCompleted, result.ExitCode, now)
	m.appendEventLocked(state, "step", step.ID, string(StepCompleted))
	return cloneStep(*step), nil
}

func (m *Manager) FailStep(runID, stepID string, result StepResult, errorKey, detail string) (AgentStep, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.expireLocked(now)
	state, step, err := m.mutableStepLocked(runID, stepID)
	if err != nil {
		return AgentStep{}, err
	}
	observation, truncated := truncateUTF8(result.Observation, m.maxObservation)
	step.State = StepFailed
	step.Observation = observation
	step.ObservationTruncated = truncated
	step.ExitCode = cloneInt(result.ExitCode)
	step.FinishedAt = &now
	step.ErrorKey = strings.TrimSpace(errorKey)
	step.ErrorDetail = strings.TrimSpace(detail)
	state.run.UpdatedAt = now
	m.finishAuditLocked(runID, step.ID, StepFailed, result.ExitCode, now)

	m.appendEventLocked(state, "step", step.ID, string(StepFailed))
	return cloneStep(*step), nil
}

func (m *Manager) Finish(runID string, next RunState, errorKey, detail string) (AgentSnapshot, error) {
	if !isTerminalRun(next) || next == RunCancelled {
		return AgentSnapshot{}, agentInvalidArg("finish requires a terminal state")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.expireLocked(now)
	state, ok := m.runs[strings.TrimSpace(runID)]
	if !ok {
		return AgentSnapshot{}, agentRunNotFound()
	}
	if isTerminalRun(state.run.State) {
		return AgentSnapshot{}, agentInvalidState("run is already finished")
	}
	if !m.finishLocked(state, next, strings.TrimSpace(errorKey), strings.TrimSpace(detail), now) {
		return AgentSnapshot{}, agentInvalidState("active steps prevent completion")
	}
	return m.snapshotLocked(state, 0), nil
}

func (m *Manager) mutableStepLocked(runID, stepID string) (*agentRunState, *AgentStep, error) {
	runID = strings.TrimSpace(runID)
	stepID = strings.TrimSpace(stepID)
	if runID == "" || stepID == "" {
		return nil, nil, agentInvalidArg("runId and stepId are required")
	}
	state, ok := m.runs[runID]
	if !ok {
		return nil, nil, agentRunNotFound()
	}
	if state.run.State != RunRunning {
		return nil, nil, agentInvalidState("run is not running")
	}
	step := findStep(state, stepID)
	if step == nil {
		return nil, nil, agentInvalidArg("step not found")
	}
	if step.State != StepRunning {
		return nil, nil, agentInvalidState("step is not running")
	}
	return state, step, nil
}

func (m *Manager) expireLocked(now time.Time) {
	for _, state := range m.runs {
		if isTerminalRun(state.run.State) {
			continue
		}
		if !state.deadline.IsZero() && !now.Before(state.deadline) {
			m.finishLocked(state, RunTimedOut, msgAgentRunTimedOut, "", now)
			continue
		}
		if state.run.State != RunWaitingApproval {
			continue
		}
		for _, pending := range state.pending {
			if !now.Before(pending.expiresAt) {
				m.finishLocked(state, RunTimedOut, msgAgentApprovalExpired, "", now)
				break
			}
		}
	}
}

func (m *Manager) finishLocked(state *agentRunState, next RunState, errorKey, detail string, now time.Time) bool {
	if isTerminalRun(state.run.State) {
		return false
	}
	if next == RunCompleted {
		for _, step := range state.run.Steps {
			if !isTerminalStep(step.State) {
				return false
			}
		}
	}
	stepState := StepFailed
	switch next {
	case RunCancelled:
		stepState = StepCancelled
	case RunTimedOut:
		stepState = StepTimedOut
	case RunRejected:
		stepState = StepRejected
	case RunBlocked, RunFailed:
		stepState = StepFailed
	}
	for index := range state.run.Steps {
		step := &state.run.Steps[index]
		if isTerminalStep(step.State) {
			continue
		}
		step.State = stepState
		step.PendingID = ""
		step.FinishedAt = &now
		m.finishAuditLocked(state.run.ID, step.ID, stepState, nil, now)
		m.appendEventLocked(state, "step", step.ID, string(stepState))
	}
	for pendingID, pending := range state.pending {
		notifyDecision(pending, false)
		delete(state.pending, pendingID)
	}
	state.run.State = next

	state.run.Draft = ""
	state.run.ErrorKey = errorKey
	state.run.ErrorDetail = detail
	state.run.UpdatedAt = now

	state.cancel()
	m.appendEventLocked(state, "run", "", string(next))
	return true
}

func findStep(state *agentRunState, stepID string) *AgentStep {
	for index := range state.run.Steps {
		if state.run.Steps[index].ID == stepID {
			return &state.run.Steps[index]
		}
	}
	return nil
}

func isTerminalRun(state RunState) bool {
	switch state {
	case RunCompleted, RunRejected, RunFailed, RunCancelled, RunTimedOut, RunBlocked:
		return true
	}
	return false
}

func isTerminalStep(state StepState) bool {
	switch state {
	case StepCompleted, StepFailed, StepCancelled, StepTimedOut, StepRejected:
		return true
	}
	return false
}

func normalizeRisk(risk string) string {
	switch strings.TrimSpace(risk) {
	case RiskReadOnly:
		return RiskReadOnly
	case RiskDanger:
		return RiskDanger
	default:

		return RiskConfirm
	}
}

func newAgentID(prefix string) (string, error) {
	buffer := make([]byte, agentIDRandomBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate agent id: %w", err)
	}
	return prefix + "_" + hex.EncodeToString(buffer), nil
}

func truncateUTF8(value string, limit int) (string, bool) {
	if limit <= 0 || len(value) <= limit {
		return value, false
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(value[cut]) {
		cut--
	}
	return value[:cut], true
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func cloneStep(step AgentStep) AgentStep {
	step.ExitCode = cloneInt(step.ExitCode)
	step.FinishedAt = cloneTime(step.FinishedAt)
	return step
}

func cloneSteps(steps []AgentStep) []AgentStep {
	if len(steps) == 0 {
		return []AgentStep{}
	}
	cloned := make([]AgentStep, len(steps))
	for index, step := range steps {
		cloned[index] = cloneStep(step)
	}
	return cloned
}

func notifyDecision(pending *agentPending, approved bool) {
	if pending == nil || pending.decision == nil {
		return
	}
	select {
	case pending.decision <- approved:
	default:
	}
}

func (m *Manager) beginDrive(runID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.runs[strings.TrimSpace(runID)]
	if !ok {
		return agentRunNotFound()
	}
	if isTerminalRun(state.run.State) {
		return agentInvalidState("run is already finished")
	}
	if state.driving {
		return agentInvalidState("run is already being driven")
	}
	state.driving = true
	return nil
}

func (m *Manager) endDrive(runID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if state, ok := m.runs[strings.TrimSpace(runID)]; ok {
		state.driving = false
	}
}

func (m *Manager) driveContext(runID string) (AgentRun, Provider, ApprovalMode, []AgentMessage, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.runs[strings.TrimSpace(runID)]
	if !ok {
		return AgentRun{}, Provider{}, "", nil, "", agentRunNotFound()
	}
	run := state.run
	run.Steps = cloneSteps(state.run.Steps)
	history := make([]AgentMessage, len(state.history))
	copy(history, state.history)
	return run, state.provider, state.approval, history, state.run.Prompt, nil
}

func (m *Manager) appendNote(runID, note string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.runs[strings.TrimSpace(runID)]
	if !ok {
		return
	}
	note = strings.TrimSpace(note)
	if note == "" {
		return
	}
	m.runNotes(state, note)
}

func (m *Manager) addRunUsage(runID string, usage *Usage) {
	if usage == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.runs[runID]
	if !ok {
		return
	}
	mergeUsage(&state.run.Usage, usage)
}

func (m *Manager) RecordContext(runID string, breakdown ContextBreakdown) {
	if breakdown.IsZero() {

		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.runs[strings.TrimSpace(runID)]
	if !ok {
		return
	}
	copied := breakdown
	state.run.Context = &copied
}

func (m *Manager) runNotes(state *agentRunState, note string) {
	now := m.now()
	start := state.run.CreatedAt
	if !state.lastNoteAt.IsZero() {
		start = state.lastNoteAt
	}
	duration := now.Sub(start).Milliseconds()
	if duration < 0 {
		duration = 0
	}
	state.lastNoteAt = now
	state.run.Notes = append(state.run.Notes, note)
	state.run.NoteDurations = append(state.run.NoteDurations, duration)
	if len(state.run.Notes) > 20 {
		state.run.Notes = state.run.Notes[len(state.run.Notes)-20:]
		state.run.NoteDurations = state.run.NoteDurations[len(state.run.NoteDurations)-20:]
	}
}

func (m *Manager) markCommandRun(runID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if state, ok := m.runs[strings.TrimSpace(runID)]; ok {
		state.run.CommandRuns++
	}
}

func (m *Manager) resetDraft(runID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if state, ok := m.runs[strings.TrimSpace(runID)]; ok {
		state.run.Draft = ""
		state.run.ModelCalls++
	}
}

func (m *Manager) appendDraft(runID, text string) {
	if text == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.runs[strings.TrimSpace(runID)]
	if !ok {
		return
	}
	state.run.Draft += text
	if len(state.run.Draft) > 32<<10 {
		trimmed, _ := truncateUTF8(state.run.Draft, 32<<10)
		state.run.Draft = trimmed
	}
	state.run.UpdatedAt = m.now()
}

func (m *Manager) WaitDecision(ctx context.Context, runID, stepID, pendingID string) (bool, error) {
	m.mu.Lock()
	state, ok := m.runs[strings.TrimSpace(runID)]
	if !ok {
		m.mu.Unlock()
		return false, agentRunNotFound()
	}
	pending, hasPending := state.pending[strings.TrimSpace(pendingID)]
	if !hasPending {
		step := findStep(state, strings.TrimSpace(stepID))
		approved := step != nil && step.State == StepRunning
		m.mu.Unlock()
		return approved, nil
	}
	decision := pending.decision
	m.mu.Unlock()

	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case approved := <-decision:
		return approved, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

func (m *Manager) runContext(runID string) (context.Context, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.runs[strings.TrimSpace(runID)]
	if !ok {
		return nil, agentRunNotFound()
	}
	if state.runCtx == nil {
		return context.Background(), nil
	}
	return state.runCtx, nil
}

func (m *Manager) CancelAll() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	cancelled := 0
	for _, state := range m.runs {
		if m.finishLocked(state, RunCancelled, msgAgentCancelled, "", now) {
			cancelled++
		}
	}
	return cancelled
}

func (m *Manager) maxStepsFor(runID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.runs[strings.TrimSpace(runID)]
	if !ok {
		return 0
	}
	return state.maxSteps
}

func clampSeconds(seconds, minimum, maximum int) time.Duration {
	if seconds <= 0 {
		return 0
	}
	if seconds < minimum {
		seconds = minimum
	}
	if seconds > maximum {
		seconds = maximum
	}
	return time.Duration(seconds) * time.Second
}

func (m *Manager) limitsFor(runID string) (time.Duration, time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.runs[strings.TrimSpace(runID)]
	if !ok {
		return 0, 0
	}
	return state.modelTimeout, state.stepTimeout
}

func (m *Manager) contextFor(runID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.runs[strings.TrimSpace(runID)]
	if !ok {
		return ""
	}
	return state.contextText
}

func (m *Manager) executionChannelFor(runID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.runs[strings.TrimSpace(runID)]
	if !ok || state.channel == "" {
		return ExecutionBackground
	}
	return state.channel
}
