type InputWriter = (payload: string) => boolean

const writers = new Map<string, InputWriter>()

export function registerTerminalInput(sessionId: string, writer: InputWriter): void {
  if (!sessionId) return
  writers.set(sessionId, writer)
}

export function forgetTerminalInput(sessionId: string): void {
  if (!sessionId) return
  writers.delete(sessionId)
}

export function hasTerminalInput(sessionId: string): boolean {
  return Boolean(sessionId && writers.get(sessionId))
}

export function sanitizeCommandForInput(command: string): string | null {
  const trimmed = command.trim()
  if (!trimmed) return null
  if (trimmed.includes('\n') || trimmed.includes('\r')) return null

  const cleaned = Array.from(trimmed)
    .filter((char) => {
      const code = char.codePointAt(0) ?? 0
      if (code === 0x09) return true
      return code >= 0x20 && code !== 0x7f
    })
    .join('')
  return cleaned.trim() || null
}

export function runCommandInSession(sessionId: string, command: string): boolean {
  const writer = writers.get(sessionId)
  if (!writer) return false
  const sanitized = sanitizeCommandForInput(command)
  if (!sanitized) return false
  try {
    return writer(`${sanitized}\r`)
  } catch {
    return false
  }
}

type OutputListener = (chunk: string) => void

const outputListeners = new Map<string, Set<OutputListener>>()

export function subscribeTerminalOutput(sessionId: string, listener: OutputListener): () => void {
  if (!sessionId) return () => {}
  const listeners = outputListeners.get(sessionId) ?? new Set<OutputListener>()
  listeners.add(listener)
  outputListeners.set(sessionId, listeners)
  return () => {
    listeners.delete(listener)
    if (!listeners.size) outputListeners.delete(sessionId)
  }
}

export function publishTerminalOutput(sessionId: string, chunk: string): void {
  const listeners = outputListeners.get(sessionId)
  if (!listeners || !listeners.size || !chunk) return
  for (const listener of listeners) {
    try {
      listener(chunk)
    } catch {}
  }
}
