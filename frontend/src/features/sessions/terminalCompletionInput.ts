type EscapeState = 'normal' | 'esc' | 'csi' | 'ss3' | 'string' | 'stringEsc'

export type TerminalCompletionInputAction =
  { type: 'append'; value: string } | { type: 'reset' } | { type: 'backspace' } | { type: 'delete-word' }

export interface TerminalCompletionInputTracker {
  feed(data: string): TerminalCompletionInputAction[]
  reset(): void
}

export function createTerminalCompletionInputTracker(): TerminalCompletionInputTracker {
  let state: EscapeState = 'normal'

  function reset(): void {
    state = 'normal'
  }

  function feed(data: string): TerminalCompletionInputAction[] {
    const actions: TerminalCompletionInputAction[] = []

    for (const char of data) {
      const code = char.codePointAt(0) ?? 0

      if (state !== 'normal') {
        if (state === 'esc') {
          if (char === '[') state = 'csi'
          else if (char === 'O') state = 'ss3'
          else if (char === ']' || char === 'P' || char === '^' || char === '_') state = 'string'
          else if (code === 0x1b) state = 'esc'
          else state = 'normal'
          continue
        }

        if (state === 'csi') {
          if (code === 0x1b) state = 'esc'
          else if (code >= 0x40 && code <= 0x7e) state = 'normal'
          continue
        }

        if (state === 'ss3') {
          state = code === 0x1b ? 'esc' : 'normal'
          continue
        }

        if (state === 'string') {
          if (char === '\x07') state = 'normal'
          else if (code === 0x1b) state = 'stringEsc'
          continue
        }

        if (state === 'stringEsc') {
          if (char === '\\') state = 'normal'
          else if (code === 0x1b) state = 'stringEsc'
          else state = 'string'
          continue
        }
      }

      if (code === 0x1b) {
        state = 'esc'
        continue
      }
      if (char === '\r' || char === '\n' || char === '\x03' || char === '\x15') {
        actions.push({ type: 'reset' })
        continue
      }
      if (char === '\x7f' || char === '\b') {
        actions.push({ type: 'backspace' })
        continue
      }
      if (char === '\x17') {
        actions.push({ type: 'delete-word' })
        continue
      }
      if (code < 0x20 || code === 0x7f) continue
      actions.push({ type: 'append', value: char })
    }

    return actions
  }

  return { feed, reset }
}
