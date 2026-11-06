import { computed, onBeforeUnmount, ref } from 'vue'
import {
  ApprovalMode,
  RunState,
  StepState,
  cancelAgentRun,
  decideAgentApproval,
  getAgentSnapshot,
  readAgentSettings,
  startAgentRun,
  type AgentRun,
  type AgentSnapshot,
} from '../services/aiAgent'
import type { AIApprovalMode } from '../services/aiApproval'
import type { AIProvider } from '../services/aiProviders'

export interface AiAgentTurnIdentity {
  conversationId: string
  sessionId: string
  providerId: string
}

export interface AiAgentTurnOptions {
  getIdentity: () => AiAgentTurnIdentity
  getProvider: () => AIProvider | null
  getApprovalMode: () => AIApprovalMode
  getContextText?: () => string
  onRunUpdated?: (run: AgentRun, identity: AiAgentTurnIdentity) => void
}

interface StoredAgentTurn {
  conversationId: string
  sessionId: string
  providerId: string
  run: AgentRun
}

const STORE_KEY = 'qterm.ai.agent.turns.v2'
const RUN_LIMIT = 100
const POLL_INTERVAL = 600

function identityKey(identity: AiAgentTurnIdentity): string {
  return `${identity.conversationId}\u0000${identity.sessionId}\u0000${identity.providerId}`
}

function readStoredTurns(): StoredAgentTurn[] {
  try {
    const parsed = JSON.parse(window.localStorage.getItem(STORE_KEY) ?? '[]') as unknown
    if (!Array.isArray(parsed)) return []
    return parsed.filter((item): item is StoredAgentTurn => {
      if (!item || typeof item !== 'object') return false
      const candidate = item as Partial<StoredAgentTurn>
      return (
        typeof candidate.conversationId === 'string' &&
        typeof candidate.sessionId === 'string' &&
        typeof candidate.providerId === 'string' &&
        Boolean(candidate.run && typeof candidate.run === 'object' && typeof candidate.run.id === 'string')
      )
    })
  } catch {
    return []
  }
}

function trimRun(run: AgentRun): AgentRun {
  return {
    ...run,
    draft: '',
    steps: (run.steps ?? []).map((step) => ({
      ...step,
      observation: step.observation ? step.observation.slice(0, 4096) : step.observation,
    })),
  }
}

export function persistAgentTurn(identity: AiAgentTurnIdentity, run: AgentRun): boolean {
  try {
    const current = readStoredTurns()
    const previous = current.find((entry) => identityKey(entry) === identityKey(identity) && entry.run.id === run.id)

    if (previous && !isLiveRun(previous.run) && isLiveRun(run)) return true
    const item: StoredAgentTurn = { ...identity, run: trimRun(run) }
    const next = [
      item,
      ...current.filter((entry) => identityKey(entry) !== identityKey(identity) || entry.run.id !== run.id),
    ].slice(0, RUN_LIMIT)
    window.localStorage.setItem(STORE_KEY, JSON.stringify(next))
    return true
  } catch {
    return false
  }
}

function isLiveRun(run: AgentRun): boolean {
  return run.state === RunState.RunRunning || run.state === RunState.RunWaitingApproval
}

export function readAgentTurn(identity: AiAgentTurnIdentity, runId?: string): AgentRun | null {
  const key = identityKey(identity)
  const entry = readStoredTurns().find((item) => identityKey(item) === key && (!runId || item.run.id === runId))
  return entry?.run ?? null
}

export function listAgentTurns(conversationId: string): StoredAgentTurn[] {
  return readStoredTurns().filter((item) => item.conversationId === conversationId)
}

export function useAiAgentTurn(options: AiAgentTurnOptions) {
  const run = ref<AgentRun | null>(null)
  const cursor = ref(0)
  const starting = ref(false)
  const deciding = ref(false)
  const error = ref('')
  const live = computed(
    () => run.value?.state === RunState.RunRunning || run.value?.state === RunState.RunWaitingApproval,
  )
  let timer: number | undefined
  let disposed = false
  let generation = 0
  let activeIdentity: AiAgentTurnIdentity | null = null

  function stopPolling(): void {
    if (timer !== undefined) {
      window.clearInterval(timer)
      timer = undefined
    }
  }

  function isCurrent(identity: AiAgentTurnIdentity): boolean {
    return !disposed && identityKey(options.getIdentity()) === identityKey(identity)
  }

  function applySnapshot(snapshot: AgentSnapshot, identity: AiAgentTurnIdentity): void {
    if (isCurrent(identity) && run.value?.id === snapshot.run.id) {
      if (snapshot.cursor < cursor.value || (!isLiveRun(run.value) && isLiveRun(snapshot.run))) return
    }

    persistAgentTurn(identity, snapshot.run)
    if (!isCurrent(identity) || run.value?.id !== snapshot.run.id) return
    run.value = snapshot.run
    cursor.value = snapshot.cursor
    options.onRunUpdated?.(snapshot.run, identity)
    if (!live.value) stopPolling()
  }

  async function poll(identity: AiAgentTurnIdentity, runId: string): Promise<void> {
    if (!isCurrent(identity) || run.value?.id !== runId || !live.value) {
      return
    }
    try {
      applySnapshot(await getAgentSnapshot(runId, cursor.value), identity)
    } catch (cause) {
      if (!isCurrent(identity) || run.value?.id !== runId || !live.value) return
      error.value = cause instanceof Error ? cause.message : 'ai.agent.resume_failed'
      if (error.value.includes('ai.agent.run_not_found')) {
        applySnapshot(
          {
            run: {
              ...run.value,
              state: RunState.RunFailed,
              errorKey: 'ai.agent.run_not_found',
              steps: (run.value.steps ?? []).map((step) =>
                step.pendingId ? { ...step, state: StepState.StepFailed, pendingId: '' } : step,
              ),
            },
            cursor: cursor.value,
            events: [],
            earliestCursor: cursor.value,
            truncated: false,
          },
          identity,
        )
      }
      stopPolling()
    }
  }

  function startPolling(identity: AiAgentTurnIdentity, runId: string): void {
    stopPolling()
    timer = window.setInterval(() => void poll(identity, runId), POLL_INTERVAL)
  }

  function restore(identity: AiAgentTurnIdentity, runId?: string): AgentRun | null {
    const stored = readAgentTurn(identity, runId)
    if (!stored) return null
    generation += 1
    starting.value = false
    run.value = stored
    activeIdentity = identity
    cursor.value = 0
    error.value = ''
    options.onRunUpdated?.(stored, identity)
    if (stored.state === RunState.RunRunning || stored.state === RunState.RunWaitingApproval) {
      startPolling(identity, stored.id)
    }
    return stored
  }

  async function start(
    prompt: string,
    history: Array<{ role: string; content: string }>,
  ): Promise<AgentSnapshot | null> {
    const provider = options.getProvider()
    const identity = options.getIdentity()
    if (!provider || !identity.sessionId || !prompt.trim() || starting.value || live.value) return null
    const attempt = ++generation
    starting.value = true
    error.value = ''
    try {
      const snapshot = await startAgentRun({
        providerId: provider.id,
        providerType: provider.type,
        baseUrl: provider.baseUrl,
        model: provider.model,
        sessionId: identity.sessionId,
        prompt: prompt.trim(),
        policyVersion: `qterm.agent.v1.${options.getApprovalMode()}`,
        approvalMode: options.getApprovalMode() as unknown as ApprovalMode,
        ...readAgentSettings(),
        contextText: options.getContextText?.() ?? '',
        history,
      })

      persistAgentTurn(identity, snapshot.run)
      if (!isCurrent(identity) || attempt !== generation) {
        try {
          const cancelled = await cancelAgentRun(snapshot.run.id)
          persistAgentTurn(identity, cancelled.run)
        } catch {}
        return snapshot
      }
      activeIdentity = identity
      run.value = snapshot.run
      cursor.value = snapshot.cursor
      options.onRunUpdated?.(snapshot.run, identity)
      if (live.value) startPolling(identity, snapshot.run.id)
      return snapshot
    } catch (cause) {
      if (isCurrent(identity)) error.value = cause instanceof Error ? cause.message : 'ai.agent.failed'
      throw cause
    } finally {
      if (attempt === generation) starting.value = false
    }
  }

  async function decide(stepId: string, pendingId: string, approve: boolean): Promise<void> {
    const active = run.value
    const identity = options.getIdentity()
    if (!active || !identity.sessionId || !pendingId || deciding.value) return
    deciding.value = true
    try {
      applySnapshot(
        await decideAgentApproval({
          runId: active.id,
          stepId,
          pendingId,
          approve,
          sessionId: identity.sessionId,
          policyVersion: active.policyVersion,
          since: cursor.value,
        }),
        identity,
      )
      if (isCurrent(identity) && run.value?.id === active.id && live.value) startPolling(identity, active.id)
    } finally {
      deciding.value = false
    }
  }

  async function cancel(): Promise<void> {
    const active = run.value
    const identity = activeIdentity ?? options.getIdentity()
    if (starting.value) {
      generation += 1
      starting.value = false
    }
    if (!active) return
    stopPolling()
    applySnapshot(await cancelAgentRun(active.id), identity)
  }

  async function cancelAndClear(): Promise<void> {
    const active = run.value
    const identity = activeIdentity
    generation += 1
    starting.value = false
    stopPolling()

    run.value = null
    cursor.value = 0
    activeIdentity = null
    error.value = ''
    if (active && identity && (active.state === RunState.RunRunning || active.state === RunState.RunWaitingApproval)) {
      try {
        const snapshot = await cancelAgentRun(active.id)
        persistAgentTurn(identity, snapshot.run)
      } catch {}
    }
  }

  function clear(): void {
    generation += 1
    starting.value = false
    stopPolling()
    run.value = null
    cursor.value = 0
    activeIdentity = null
    error.value = ''
  }

  onBeforeUnmount(() => {
    disposed = true
    generation += 1
    stopPolling()
  })

  return {
    run,
    cursor,
    starting,
    deciding,
    error,
    live,
    start,
    restore,
    poll,
    decide,
    cancel,
    cancelAndClear,
    clear,
    stopPolling,
  }
}
