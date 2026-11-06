export type VncPasteResultCode = 'sent' | 'empty' | 'read-failed' | 'view-only' | 'send-failed' | 'not-connected'

const PASTE_RESULT_CODES: readonly VncPasteResultCode[] = [
  'sent',
  'empty',
  'read-failed',
  'view-only',
  'send-failed',
  'not-connected',
]

export function isVncPasteResultCode(value: unknown): value is VncPasteResultCode {
  return typeof value === 'string' && (PASTE_RESULT_CODES as readonly string[]).includes(value)
}

export function normalizePasteResult(value: unknown): VncPasteResultCode {
  return isVncPasteResultCode(value) ? value : 'send-failed'
}

export function isPasteShortcut(event: {
  ctrlKey?: boolean
  metaKey?: boolean
  shiftKey?: boolean
  altKey?: boolean
  key?: string
  code?: string
}): boolean {
  if (!(event.metaKey === true || event.ctrlKey === true)) return false
  if (event.shiftKey === true || event.altKey === true) return false
  const key = typeof event.key === 'string' ? event.key.toLowerCase() : ''
  return key === 'v' || (key === '' && event.code === 'KeyV')
}

export function isNativePasteTarget(target: unknown): boolean {
  if (!target || typeof target !== 'object') return false
  const element = target as { tagName?: unknown; isContentEditable?: unknown }
  if (element.isContentEditable === true) return true
  const tag = typeof element.tagName === 'string' ? element.tagName.toUpperCase() : ''
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT'
}

export interface PasteKeyEventLike {
  ctrlKey?: boolean
  metaKey?: boolean
  shiftKey?: boolean
  altKey?: boolean
  key?: string
  code?: string

  target?: unknown

  preventDefault?: () => void
}

export interface PasteKeyHandlerOptions {
  send: (text: string) => Promise<VncPasteResultCode> | VncPasteResultCode

  readClipboard: () => Promise<string>
}

export function createPasteKeyHandler(
  options: PasteKeyHandlerOptions,
): (event: PasteKeyEventLike) => Promise<VncPasteResultCode> | null {
  return (event: PasteKeyEventLike): Promise<VncPasteResultCode> | null => {
    if (isNativePasteTarget(event.target)) return null
    if (!isPasteShortcut(event)) return null

    try {
      event.preventDefault?.()
    } catch {}
    return (async (): Promise<VncPasteResultCode> => {
      let text: string
      try {
        text = await options.readClipboard()
      } catch {
        text = ''
      }
      try {
        return normalizePasteResult(await options.send(text))
      } catch {
        return 'send-failed'
      }
    })()
  }
}

export interface AutoSyncVisibilitySource {
  visibilityState?: string
  hasFocus?: () => boolean
}

export interface ClipboardAutoSyncOptions {
  readClipboard: () => Promise<string>

  send: (text: string, source: 'auto' | 'button') => Promise<VncPasteResultCode> | VncPasteResultCode

  isVisible: () => boolean

  intervalMs?: number

  settleMs?: number

  wait?: (ms: number) => Promise<void>

  visibility?: AutoSyncVisibilitySource | null
}

export interface ClipboardAutoSyncController {
  start(): void

  stop(): void

  isRunning(): boolean

  tick(): Promise<void>
}

export const VNC_CLIPBOARD_AUTO_SYNC_INTERVAL_MS = 750

export const VNC_CLIPBOARD_SETTLE_MS = 150

const READ_RETRY_LIMIT = 2

const FAILURE_RETRY_COOLDOWN_MS = 5000

function defaultVisibilitySource(): AutoSyncVisibilitySource | null {
  const candidate = (globalThis as { document?: AutoSyncVisibilitySource }).document
  return candidate ?? null
}

function defaultWait(ms: number): Promise<void> {
  return new Promise((resolve) => {
    setTimeout(resolve, ms)
  })
}

export function createClipboardAutoSync(options: ClipboardAutoSyncOptions): ClipboardAutoSyncController {
  const intervalMs = options.intervalMs ?? VNC_CLIPBOARD_AUTO_SYNC_INTERVAL_MS
  const settleMs = options.settleMs ?? VNC_CLIPBOARD_SETTLE_MS
  const wait = options.wait ?? defaultWait
  const visibility = options.visibility === undefined ? defaultVisibilitySource() : options.visibility

  let lastSentAuto = ''

  let failedAuto = ''
  let failedAt = 0
  let timer: ReturnType<typeof setInterval> | null = null
  let running = false

  let inFlight = false

  function documentHidden(): boolean {
    return visibility?.visibilityState === 'hidden'
  }

  function windowUnfocused(): boolean {
    if (!visibility || typeof visibility.hasFocus !== 'function') return false
    try {
      return visibility.hasFocus() === false
    } catch {
      return false
    }
  }

  async function readSettled(): Promise<string> {
    let previous = await options.readClipboard()
    for (let attempt = 0; attempt < READ_RETRY_LIMIT; attempt++) {
      await wait(settleMs)
      const next = await options.readClipboard()
      if (next === previous) return next
      previous = next
    }
    return previous
  }

  async function tick(): Promise<void> {
    if (inFlight) return
    inFlight = true
    try {
      if (!running) return
      if (!options.isVisible()) return
      if (documentHidden() || windowUnfocused()) return
      const text = await readSettled()
      if (!running) return
      if (!text) return

      if (text === lastSentAuto) return

      if (text === failedAuto && Date.now() - failedAt < FAILURE_RETRY_COOLDOWN_MS) return
      const result = await options.send(text, 'auto')
      if (result === 'sent') {
        lastSentAuto = text
        failedAuto = ''
        return
      }

      failedAuto = text
      failedAt = Date.now()
    } catch {
    } finally {
      inFlight = false
    }
  }

  return {
    start(): void {
      if (running) return
      running = true

      void tick()
      timer = setInterval(() => {
        void tick()
      }, intervalMs)
    },
    stop(): void {
      running = false
      if (timer !== null) {
        clearInterval(timer)
        timer = null
      }
    },
    isRunning(): boolean {
      return running
    },
    tick(): Promise<void> {
      return tick()
    },
  }
}
