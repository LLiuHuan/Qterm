import { nextTick } from 'vue'

export interface TimeoutOutcome<T> {
  timedOut: boolean
  value?: T
}

export function withTimeout<T>(task: Promise<T>, timeoutMs: number): Promise<TimeoutOutcome<T>> {
  if (!(timeoutMs > 0)) {
    return task.then(
      (value) => ({ timedOut: false, value }),
      () => ({ timedOut: false, value: undefined }),
    )
  }
  return new Promise<TimeoutOutcome<T>>((resolve) => {
    let settled = false

    const timer = { handle: undefined as number | undefined }
    const finish = (outcome: TimeoutOutcome<T>): void => {
      if (settled) return
      settled = true
      if (timer.handle !== undefined) window.clearTimeout(timer.handle)
      resolve(outcome)
    }

    void task.then(
      (value) => finish({ timedOut: false, value }),
      () => finish({ timedOut: false, value: undefined }),
    )
    timer.handle = window.setTimeout(() => finish({ timedOut: true }), timeoutMs)
  })
}

export async function waitForAvailable<T>(
  read: () => T | null | undefined,
  options: { timeoutMs?: number; intervalMs?: number } = {},
): Promise<T | undefined> {
  const timeoutMs = options.timeoutMs ?? 1000
  const intervalMs = options.intervalMs ?? 16
  const deadline = Date.now() + Math.max(0, timeoutMs)
  for (;;) {
    const value = read()
    if (value !== null && value !== undefined) return value
    if (Date.now() >= deadline) return undefined
    await nextTick()
    await wait(Math.min(intervalMs, Math.max(0, deadline - Date.now())))
  }
}

function wait(ms: number): Promise<void> {
  if (ms <= 0) return Promise.resolve()
  return new Promise<void>((resolve) => window.setTimeout(resolve, ms))
}
