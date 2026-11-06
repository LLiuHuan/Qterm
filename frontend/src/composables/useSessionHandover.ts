import { ref, type Ref } from 'vue'
import type { SessionGateway, SessionKind } from '../services/sessions'

export const HANDOVER_POLL_INTERVAL_MS = 1000

export const HANDOVER_DEADLINE_MS = 12000

export interface PendingHandover {
  sessionId: string
  hostId: string
  kind: SessionKind
  title: string

  epochAtDetach: number

  deadline: number
}

export interface HandoverRecordInput {
  sessionId: string
  hostId: string
  kind: SessionKind
  title: string
  epochAtDetach: number
}

export interface HandoverNotice {
  sessionId: string
  hostId: string
  kind: SessionKind
  title: string
  reason: 'closed' | 'pending'
}

export interface SessionHandoverOptions {
  sessionGateway: SessionGateway

  onNotice: (notice: HandoverNotice) => void

  intervalMs?: number

  deadlineMs?: number

  now?: () => number
}

export interface SessionHandoverWatcher {
  watch(record: HandoverRecordInput): void

  forget(sessionId: string): void

  dispose(): void

  readonly pending: Ref<PendingHandover[]>
}

export function createSessionHandoverWatcher(options: SessionHandoverOptions): SessionHandoverWatcher {
  const pending = ref<PendingHandover[]>([])
  const intervalMs = options.intervalMs ?? HANDOVER_POLL_INTERVAL_MS
  const deadlineMs = options.deadlineMs ?? HANDOVER_DEADLINE_MS
  const now = options.now ?? (() => Date.now())
  let timer: number | undefined
  let disposed = false
  let polling = false

  function schedule(delay = intervalMs): void {
    if (disposed || timer !== undefined || pending.value.length === 0) return
    timer = window.setTimeout(() => {
      timer = undefined
      void poll()
    }, delay)
  }

  function forget(sessionId: string): void {
    pending.value = pending.value.filter((record) => record.sessionId !== sessionId)
    if (pending.value.length === 0 && timer !== undefined) {
      window.clearTimeout(timer)
      timer = undefined
    }
  }

  async function poll(): Promise<void> {
    if (disposed || polling) return
    polling = true
    try {
      for (const record of [...pending.value]) {
        if (disposed) return
        await check(record)
      }
    } finally {
      polling = false
      schedule()
    }
  }

  async function check(record: PendingHandover): Promise<void> {
    let snapshot: Awaited<ReturnType<SessionGateway['get']>>
    try {
      snapshot = await options.sessionGateway.get(record.sessionId)
    } catch {
      forget(record.sessionId)
      return
    }
    if (disposed) return

    if ((snapshot.attachEpoch ?? 0) > record.epochAtDetach) {
      forget(record.sessionId)
      return
    }
    if (snapshot.state === 'closed') {
      forget(record.sessionId)

      if (snapshot.closeReason === 'orphan_timeout') {
        options.onNotice({ ...noticeOf(record), reason: 'closed' })
      }
      return
    }
    if (snapshot.state === 'failed') {
      forget(record.sessionId)
      return
    }
    if (now() >= record.deadline) {
      forget(record.sessionId)
      options.onNotice({ ...noticeOf(record), reason: 'pending' })
    }
  }

  function noticeOf(record: PendingHandover): Omit<HandoverNotice, 'reason'> {
    return {
      sessionId: record.sessionId,
      hostId: record.hostId,
      kind: record.kind,
      title: record.title,
    }
  }

  return {
    pending,
    watch(record: HandoverRecordInput): void {
      if (disposed) return
      forget(record.sessionId)
      pending.value = [
        ...pending.value,
        {
          sessionId: record.sessionId,
          hostId: record.hostId,
          kind: record.kind,
          title: record.title,
          epochAtDetach: record.epochAtDetach,
          deadline: now() + deadlineMs,
        },
      ]

      schedule(0)
    },
    forget,
    dispose(): void {
      disposed = true
      if (timer !== undefined) window.clearTimeout(timer)
      timer = undefined
      pending.value = []
    },
  }
}
