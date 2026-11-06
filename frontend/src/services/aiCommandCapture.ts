import { subscribeTerminalOutput } from '../features/sessions/terminalInput'

const IDLE_MS = 600

const MAX_BYTES = 120 * 1024

export interface CommandCapture {
  promise: Promise<string>
  cancel: () => void
}

export function captureCommandOutput(sessionId: string): CommandCapture {
  let buffer = ''
  let finished = false
  let idleTimer: ReturnType<typeof setTimeout> | null = null
  let hardTimer: ReturnType<typeof setTimeout> | null = null
  let unsubscribe: (() => void) | null = null
  let resolveFn: (text: string) => void = () => {}

  const finish = (): void => {
    if (finished) return
    finished = true
    if (idleTimer) clearTimeout(idleTimer)
    if (hardTimer) clearTimeout(hardTimer)
    unsubscribe?.()
    resolveFn(buffer)
  }

  unsubscribe = subscribeTerminalOutput(sessionId, (chunk: string) => {
    if (finished) return
    buffer += chunk
    if (buffer.length > MAX_BYTES) {
      finish()
      return
    }
    if (idleTimer) clearTimeout(idleTimer)
    idleTimer = setTimeout(finish, IDLE_MS)
  })

  idleTimer = setTimeout(finish, IDLE_MS)
  hardTimer = setTimeout(finish, 60_000)

  const promise = new Promise<string>((resolve) => {
    resolveFn = resolve
  })

  return {
    promise,
    cancel: () => {
      finish()
    },
  }
}
