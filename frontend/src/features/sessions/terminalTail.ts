export const TERMINAL_TAIL_LINES = 80

type TailReader = () => string

const readers = new Map<string, TailReader>()

export function registerTerminalTailReader(sessionId: string, reader: TailReader): void {
  if (!sessionId) return
  readers.set(sessionId, reader)
}

export function forgetTerminalTailReader(sessionId: string): void {
  if (!sessionId) return
  readers.delete(sessionId)
}

export function readTerminalTail(sessionId: string): string {
  if (!sessionId) return ''
  const reader = readers.get(sessionId)
  if (!reader) return ''
  try {
    return reader()
  } catch {
    return ''
  }
}
