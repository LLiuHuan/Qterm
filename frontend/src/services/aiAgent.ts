import { ApprovalMode, ProviderType, RunState, StepKind, StepState } from '../../bindings/changeme/internal/ai/models'
import type {
  AgentAuditEntry,
  AgentEvent,
  AgentRun,
  AgentSnapshot,
  AgentStep,
} from '../../bindings/changeme/internal/ai/models'
import { hasWailsRuntime } from './runtime'

export { ApprovalMode, ProviderType, RunState, StepKind, StepState }
export type { AgentAuditEntry, AgentEvent, AgentRun, AgentSnapshot, AgentStep }

export interface AgentSettings {
  maxSteps: number

  modelTimeoutSeconds: number

  stepTimeoutSeconds: number

  inlineLines: number

  executionChannel: 'background' | 'terminal'
}

const AGENT_SETTINGS_KEY = 'qterm.ai.agent-settings.v1'
export const DEFAULT_AGENT_SETTINGS: AgentSettings = {
  maxSteps: 10,
  modelTimeoutSeconds: 60,
  stepTimeoutSeconds: 30,
  inlineLines: 10,
  executionChannel: 'background',
}

function clamp(value: unknown, minimum: number, maximum: number, fallback: number): number {
  const number = Number(value)
  if (!Number.isFinite(number)) return fallback
  return Math.min(maximum, Math.max(minimum, Math.round(number)))
}

export function readAgentSettings(): AgentSettings {
  try {
    const parsed = JSON.parse(window.localStorage.getItem(AGENT_SETTINGS_KEY) ?? '{}') as Partial<AgentSettings>
    return {
      maxSteps: clamp(parsed.maxSteps, 1, 50, DEFAULT_AGENT_SETTINGS.maxSteps),
      modelTimeoutSeconds: clamp(parsed.modelTimeoutSeconds, 5, 600, DEFAULT_AGENT_SETTINGS.modelTimeoutSeconds),
      stepTimeoutSeconds: clamp(parsed.stepTimeoutSeconds, 5, 600, DEFAULT_AGENT_SETTINGS.stepTimeoutSeconds),
      inlineLines: clamp(parsed.inlineLines, 1, 200, DEFAULT_AGENT_SETTINGS.inlineLines),
      executionChannel: parsed.executionChannel === 'terminal' ? 'terminal' : 'background',
    }
  } catch {}
  return { ...DEFAULT_AGENT_SETTINGS }
}

export function writeAgentSettings(settings: AgentSettings): void {
  const next: AgentSettings = {
    maxSteps: clamp(settings.maxSteps, 1, 50, DEFAULT_AGENT_SETTINGS.maxSteps),
    modelTimeoutSeconds: clamp(settings.modelTimeoutSeconds, 5, 600, DEFAULT_AGENT_SETTINGS.modelTimeoutSeconds),
    stepTimeoutSeconds: clamp(settings.stepTimeoutSeconds, 5, 600, DEFAULT_AGENT_SETTINGS.stepTimeoutSeconds),
    inlineLines: clamp(settings.inlineLines, 1, 200, DEFAULT_AGENT_SETTINGS.inlineLines),
    executionChannel: settings.executionChannel === 'terminal' ? 'terminal' : 'background',
  }
  try {
    window.localStorage.setItem(AGENT_SETTINGS_KEY, JSON.stringify(next))
  } catch {}
}

export interface StartAgentInput {
  providerId: string
  providerType: ProviderType
  baseUrl: string
  model: string
  sessionId: string
  prompt: string
  policyVersion: string
  approvalMode: ApprovalMode
  maxSteps?: number
  modelTimeoutSeconds?: number
  stepTimeoutSeconds?: number
  inlineLines?: number
  executionChannel?: 'background' | 'terminal'

  contextText?: string
  history?: Array<{ role: string; content: string }>
}

export async function startAgentRun(input: StartAgentInput): Promise<AgentSnapshot> {
  if (!hasWailsRuntime()) return memoryAgent.start(input)
  const { AIAgentStart } = await import('../../bindings/changeme/internal/app/service')
  return (await AIAgentStart({
    ProviderID: input.providerId,
    ProviderType: input.providerType,
    BaseURL: input.baseUrl,
    Model: input.model,
    SessionID: input.sessionId,
    Prompt: input.prompt,
    PolicyVersion: input.policyVersion,
    ApprovalMode: input.approvalMode,
    MaxSteps: input.maxSteps ?? 0,
    ModelTimeoutSeconds: input.modelTimeoutSeconds ?? 0,
    StepTimeoutSeconds: input.stepTimeoutSeconds ?? 0,
    ExecutionChannel: input.executionChannel ?? '',
    ContextText: input.contextText ?? '',
    History: (input.history ?? []).map((message) => ({
      role: message.role,
      content: message.content,
    })),
  })) as AgentSnapshot
}

export async function getAgentSnapshot(runId: string, since: number): Promise<AgentSnapshot> {
  if (!hasWailsRuntime()) return memoryAgent.snapshot(runId, since)
  const { AIAgentSnapshot } = await import('../../bindings/changeme/internal/app/service')
  return (await AIAgentSnapshot(runId, since)) as AgentSnapshot
}

export async function decideAgentApproval(input: {
  runId: string
  stepId: string
  pendingId: string
  approve: boolean
  sessionId: string
  policyVersion: string
  since: number
}): Promise<AgentSnapshot> {
  if (!hasWailsRuntime()) return memoryAgent.decide(input)
  const { AIAgentDecide } = await import('../../bindings/changeme/internal/app/service')
  return (await AIAgentDecide({
    RunID: input.runId,
    StepID: input.stepId,
    PendingID: input.pendingId,
    Approve: input.approve,
    SessionID: input.sessionId,
    PolicyVersion: input.policyVersion,
    Since: input.since,
  })) as AgentSnapshot
}

export async function cancelAgentRun(runId: string): Promise<AgentSnapshot> {
  if (!hasWailsRuntime()) return memoryAgent.cancel(runId)
  const { AIAgentCancel } = await import('../../bindings/changeme/internal/app/service')
  return (await AIAgentCancel(runId)) as AgentSnapshot
}

export async function readAgentAudit(limit = 20): Promise<AgentAuditEntry[]> {
  if (!hasWailsRuntime()) return memoryAgent.audit()
  const { AIAgentAudit } = await import('../../bindings/changeme/internal/app/service')
  return ((await AIAgentAudit(limit)) ?? []) as AgentAuditEntry[]
}

export async function releaseAgentRun(runId: string): Promise<void> {
  if (!hasWailsRuntime()) {
    memoryAgent.release(runId)
    return
  }
  const { AIAgentRelease } = await import('../../bindings/changeme/internal/app/service')
  await AIAgentRelease(runId)
}

class MemoryAgent {
  private runs = new Map<string, AgentRun>()
  private events = new Map<string, AgentEvent[]>()
  private cursors = new Map<string, number>()
  private timers = new Map<string, number>()
  private sequence = 0

  start(input: StartAgentInput): AgentSnapshot {
    const id = `airun_preview_${++this.sequence}`
    const now = new Date().toISOString()
    const run: AgentRun = {
      id,
      providerId: input.providerId,
      model: input.model,
      sessionId: input.sessionId,
      prompt: input.prompt,
      policyVersion: input.policyVersion,
      state: RunState.RunRunning,
      steps: [],
      draft: '正在分析请求…',
      createdAt: now,
      updatedAt: now,
    }
    this.runs.set(id, run)
    this.events.set(id, [])
    this.cursors.set(id, 0)
    this.emit(id, 'run', '', RunState.RunRunning)
    this.schedule(id, 600, () => this.requestCommand(id, input.approvalMode))
    return this.snapshot(id, 0)
  }

  snapshot(runId: string, since: number): AgentSnapshot {
    const run = this.runs.get(runId)
    if (!run) throw new Error('ai.agent.run_not_found')
    const events = (this.events.get(runId) ?? []).filter((event) => event.cursor > since)
    const cursor = this.cursors.get(runId) ?? 0
    return {
      run: structuredClone(run),
      events,
      cursor,
      earliestCursor: events.length > 0 ? events[0].cursor : cursor + 1,
      truncated: false,
    }
  }

  decide(input: { runId: string; stepId: string; pendingId: string; approve: boolean }): AgentSnapshot {
    const run = this.runs.get(input.runId)
    if (!run) throw new Error('ai.agent.run_not_found')
    const step = run.steps?.find((item) => item.id === input.stepId)
    if (!step || step.pendingId !== input.pendingId) throw new Error('ai.agent.approval_not_found')
    step.pendingId = ''
    run.updatedAt = new Date().toISOString()
    if (!input.approve) {
      step.state = StepState.StepRejected
      step.finishedAt = run.updatedAt
      run.state = RunState.RunRejected
      run.errorKey = 'ai.agent.rejected'
      this.emit(input.runId, 'approval', step.id, 'rejected')
      this.emit(input.runId, 'run', '', RunState.RunRejected)
      return this.snapshot(input.runId, 0)
    }
    step.state = StepState.StepRunning
    run.state = RunState.RunRunning
    this.emit(input.runId, 'approval', step.id, 'approved')
    this.emit(input.runId, 'step', step.id, StepState.StepRunning)
    this.schedule(input.runId, 700, () => this.finishCommand(input.runId, step.id))
    return this.snapshot(input.runId, 0)
  }

  cancel(runId: string): AgentSnapshot {
    const run = this.runs.get(runId)
    if (!run) throw new Error('ai.agent.run_not_found')
    if (run.state !== RunState.RunRunning && run.state !== RunState.RunWaitingApproval) {
      return this.snapshot(runId, 0)
    }
    this.clearTimer(runId)
    for (const step of run.steps ?? []) {
      if (step.state === StepState.StepRunning || step.state === StepState.StepWaitingApproval) {
        step.state = StepState.StepCancelled
        step.pendingId = ''
        this.emit(runId, 'step', step.id, StepState.StepCancelled)
      }
    }
    run.state = RunState.RunCancelled
    run.errorKey = 'ai.agent.cancelled'
    run.updatedAt = new Date().toISOString()
    this.emit(runId, 'run', '', RunState.RunCancelled)
    return this.snapshot(runId, 0)
  }

  audit(): AgentAuditEntry[] {
    return []
  }

  release(runId: string): void {
    this.clearTimer(runId)
    this.runs.delete(runId)
    this.events.delete(runId)
    this.cursors.delete(runId)
  }

  private requestCommand(runId: string, mode: ApprovalMode): void {
    const run = this.runs.get(runId)
    if (!run || run.state !== RunState.RunRunning) return
    run.steps = run.steps ?? []

    run.context = {
      system: 1_850,
      tools: 1_640,
      messages: 320 + Math.min(2_000, (run.prompt?.length ?? 0) * 3),
      context: 0,
      limit: 96 << 10,
    }
    const needsApproval = mode === ApprovalMode.ApprovalStrict
    const step: AgentStep = {
      id: `aistep_preview_${run.steps.length + 1}`,
      index: run.steps.length + 1,
      callId: `call_${run.steps.length + 1}`,
      kind: StepKind.StepKindCommand,
      state: needsApproval ? StepState.StepWaitingApproval : StepState.StepRunning,
      title: '查看磁盘占用',
      command: 'df -h',
      targetSessionId: run.sessionId,
      risk: 'readonly',
      pendingId: needsApproval ? `aipending_preview_${++this.sequence}` : '',
      startedAt: new Date().toISOString(),
    }
    run.steps.push(step)
    run.draft = ''
    run.modelCalls = (run.modelCalls ?? 0) + 1
    run.updatedAt = new Date().toISOString()
    if (needsApproval) {
      run.state = RunState.RunWaitingApproval
      this.emit(runId, 'step', step.id, StepState.StepWaitingApproval)
      this.emit(runId, 'approval', step.id, 'requested')
      this.emit(runId, 'run', '', RunState.RunWaitingApproval)
      return
    }
    this.emit(runId, 'step', step.id, StepState.StepRunning)
    this.schedule(runId, 700, () => this.finishCommand(runId, step.id))
  }

  private finishCommand(runId: string, stepId: string): void {
    const run = this.runs.get(runId)
    if (!run || run.state !== RunState.RunRunning) return
    const step = run.steps?.find((item) => item.id === stepId)
    if (!step || step.state !== StepState.StepRunning) return
    step.state = StepState.StepCompleted
    step.observation = 'Filesystem Size Used Avail Use% Mounted on\n/dev/vda1 40G 34G 4.8G 88% /'
    step.exitCode = 0
    step.finishedAt = new Date().toISOString()
    run.commandRuns = (run.commandRuns ?? 0) + 1
    run.updatedAt = new Date().toISOString()
    this.emit(runId, 'step', step.id, StepState.StepCompleted)
    this.schedule(runId, 600, () => this.finishWithAnswer(runId))
  }

  private finishWithAnswer(runId: string): void {
    const run = this.runs.get(runId)
    if (!run || run.state !== RunState.RunRunning) return
    run.steps = run.steps ?? []
    const step: AgentStep = {
      id: `aistep_preview_${run.steps.length + 1}`,
      index: run.steps.length + 1,
      callId: `call_${run.steps.length + 1}`,
      kind: StepKind.StepKindAnswer,
      state: StepState.StepCompleted,
      title: '',
      observation: '根分区已用 88%，建议清理 `/var/log` 或扩容。',
      startedAt: new Date().toISOString(),
      finishedAt: new Date().toISOString(),
    }
    run.steps.push(step)
    run.state = RunState.RunCompleted
    run.draft = ''
    run.modelCalls = (run.modelCalls ?? 0) + 1
    run.updatedAt = new Date().toISOString()
    this.emit(runId, 'step', step.id, StepState.StepCompleted)
    this.emit(runId, 'run', '', RunState.RunCompleted)
  }

  private emit(runId: string, kind: string, stepId: string, state: string): void {
    const cursor = (this.cursors.get(runId) ?? 0) + 1
    this.cursors.set(runId, cursor)
    const list = this.events.get(runId) ?? []
    list.push({
      cursor,
      runId,
      kind,
      stepId,
      state,
      at: new Date().toISOString(),
    })
    this.events.set(runId, list)
  }

  private schedule(runId: string, delay: number, action: () => void): void {
    this.clearTimer(runId)
    const timer = window.setTimeout(() => {
      this.timers.delete(runId)
      action()
    }, delay)
    this.timers.set(runId, timer)
  }

  private clearTimer(runId: string): void {
    const timer = this.timers.get(runId)
    if (timer !== undefined) {
      window.clearTimeout(timer)
      this.timers.delete(runId)
    }
  }
}

const memoryAgent = new MemoryAgent()
