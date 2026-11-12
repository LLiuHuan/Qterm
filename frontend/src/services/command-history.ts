import type { CommandHistory } from '../../bindings/changeme/internal/model/models'

const STORAGE_KEY = 'qterm.command-history.v1'
const HOST_LIMIT = 500
const COMMAND_LIMIT = 4096
let commandHistoryMinLength = 1
let commandHistoryMaxLength = COMMAND_LIMIT

function nativeRuntimeReady(): boolean {
  return typeof window !== 'undefined' && window.location.protocol === 'wails:'
}

let commandHistoryEnabled = true

export function setCommandHistoryEnabled(enabled: boolean): void {
  commandHistoryEnabled = enabled !== false
}

export function isCommandHistoryEnabled(): boolean {
  return commandHistoryEnabled
}

export function setCommandHistoryLimits(minLength: number, maxLength: number): void {
  const min = Number.isFinite(minLength) ? Math.max(0, Math.min(COMMAND_LIMIT, Math.round(minLength))) : 1
  const max = Number.isFinite(maxLength) ? Math.max(min, Math.min(COMMAND_LIMIT, Math.round(maxLength))) : COMMAND_LIMIT
  commandHistoryMinLength = min
  commandHistoryMaxLength = max
}

export function commandHistoryLengthAllowed(command: string): boolean {
  const length = Array.from(command.trim()).length
  return length >= commandHistoryMinLength && length <= commandHistoryMaxLength
}

function readFallback(): CommandHistory[] {
  try {
    const value: unknown = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]')
    return Array.isArray(value) ? (value as CommandHistory[]) : []
  } catch {
    return []
  }
}

function writeFallback(items: CommandHistory[]): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(items))
  window.dispatchEvent(new CustomEvent('qterm:command-history-changed', { detail: { hostId: '' } }))
}

function newCommandID(): string {
  const random = Math.random().toString(36).slice(2, 10)
  return `cmd_${Date.now().toString(36)}_${random}`
}

export async function listCommandHistory(hostId: string): Promise<CommandHistory[]> {
  if (!hostId) return []
  if (nativeRuntimeReady()) {
    const { ListCommandHistory } = await import('../../bindings/changeme/internal/app/service')
    return (await ListCommandHistory(hostId)) ?? []
  }
  return readFallback()
    .filter((item) => item.hostId === hostId)
    .sort((a, b) => b.executedAt.localeCompare(a.executedAt))
    .slice(0, HOST_LIMIT)
}

function stripTerminalNoise(value: string): string {
  let out = ''
  for (let index = 0; index < value.length; index += 1) {
    const code = value.charCodeAt(index)
    if (code === 0x1b) {
      const next = value[index + 1]
      if (next === '[') {
        let cursor = index + 2
        while (cursor < value.length) {
          const current = value.charCodeAt(cursor)
          if (current >= 0x40 && current <= 0x7e) break
          cursor += 1
        }
        index = cursor
        continue
      }
      if (next === ']' || next === 'P' || next === '^' || next === '_') {
        let cursor = index + 2
        while (cursor < value.length) {
          if (value.charCodeAt(cursor) === 0x07) break
          if (value.charCodeAt(cursor) === 0x1b && value[cursor + 1] === '\\') {
            cursor += 1
            break
          }
          cursor += 1
        }
        index = cursor
        continue
      }
      index += 1
      continue
    }

    if (code < 0x20 && code !== 0x09 && code !== 0x0a) continue
    if (code === 0x7f) continue
    out += value[index]
  }
  const trimmed = out.trim()
  return trimmed.startsWith('>|') ? '' : trimmed
}

export async function saveCommandHistory(entry: {
  hostId: string
  hostName: string
  sessionId: string
  command: string
}): Promise<void> {
  if (!commandHistoryEnabled) return

  const command = stripTerminalNoise(entry.command).slice(0, COMMAND_LIMIT)
  if (!entry.hostId || !command || !commandHistoryLengthAllowed(command)) return
  if (nativeRuntimeReady()) {
    const { SaveCommandHistory } = await import('../../bindings/changeme/internal/app/service')
    await SaveCommandHistory({
      id: newCommandID(),
      hostId: entry.hostId,
      hostName: entry.hostName,
      sessionId: entry.sessionId,
      command,
      executedAt: new Date().toISOString(),
      suppressed: false,
    })
    window.dispatchEvent(new CustomEvent('qterm:command-history-changed', { detail: { hostId: entry.hostId } }))
    return
  }
  const item: CommandHistory = {
    id: newCommandID(),
    hostId: entry.hostId,
    hostName: entry.hostName,
    sessionId: entry.sessionId,
    command,
    executedAt: new Date().toISOString(),
    suppressed: false,
  }
  const all = readFallback()
  const forHost = [item, ...all.filter((entry) => entry.hostId === item.hostId)].slice(0, HOST_LIMIT)
  writeFallback([...forHost, ...all.filter((entry) => entry.hostId !== item.hostId)])
}

export async function deleteCommandHistory(id: string): Promise<void> {
  if (nativeRuntimeReady()) {
    const { DeleteCommandHistory } = await import('../../bindings/changeme/internal/app/service')
    await DeleteCommandHistory(id)
    return
  }
  writeFallback(readFallback().filter((item) => item.id !== id))
}

export async function suppressCommandHistory(hostId: string, command: string): Promise<number> {
  const normalizedCommand = command.trim()
  if (!hostId || !normalizedCommand) return 0
  if (nativeRuntimeReady()) {
    const { SuppressCommandHistory } = await import('../../bindings/changeme/internal/app/service')
    const count = (await SuppressCommandHistory(hostId, normalizedCommand)) ?? 0
    window.dispatchEvent(new CustomEvent('qterm:command-history-changed', { detail: { hostId } }))
    return count
  }
  let count = 0
  const next = readFallback().map((item) => {
    if (item.hostId === hostId && item.command === normalizedCommand && !item.suppressed) {
      count += 1
      return { ...item, suppressed: true }
    }
    return item
  })
  if (count > 0) writeFallback(next)
  return count
}

export async function clearCommandHistory(hostId: string): Promise<number> {
  if (!hostId) return 0
  if (nativeRuntimeReady()) {
    const { ClearCommandHistory } = await import('../../bindings/changeme/internal/app/service')
    const count = await ClearCommandHistory(hostId)
    return count ?? 0
  }
  const before = readFallback()
  const removed = before.filter((item) => item.hostId === hostId).length
  writeFallback(before.filter((item) => item.hostId !== hostId))
  return removed
}

export interface CommandLineBuffer {
  feed(data: string): string[]
}

export function extractSubmittedScreenCommand(screenLine: string): string {
  const rawLine = screenLine
  const line = screenLine.replace(/\s+$/, '')
  const promptCommand = rawLine.match(/(?:^|\n).*?[$#%>]\s+(.+?)\s*$/)?.[1]?.trim()
  if (promptCommand) return promptCommand

  if (/(?:^|\n).*?[$#%>]$/.test(line)) return ''
  return line.trim()
}

export function resolveSubmittedCommand(screenLine: string, typed: string): string {
  const line = screenLine.replace(/\s+$/, '')
  if (!typed) return line.trim()
  const start = line.lastIndexOf(typed)
  if (start >= 0) {
    const command = line.slice(start).trim()

    if (command.length >= typed.trim().length) return command
    return typed
  }

  const promptCommand = extractSubmittedScreenCommand(line)
  if (promptCommand && promptCommand !== line.trim() && promptCommand.length >= typed.trim().length)
    return promptCommand
  return typed
}

export function mergeShellTabCommand(screenLine: string, typed: string, tabPrefix: string, completed: string): string {
  const screenCommand = resolveSubmittedCommand(screenLine, typed).trim()
  const base = completed.trim()
  const raw = typed.trim()
  const prefix = tabPrefix.trim()
  if (!base || !prefix || !raw.startsWith(prefix)) return extractSubmittedScreenCommand(screenLine) || typed

  const suffix = raw.slice(prefix.length)
  const merged = `${base}${suffix}`.trim()
  if (!merged) return screenCommand || typed

  if (screenCommand.length >= merged.length && screenCommand.startsWith(base)) return screenCommand
  return merged
}

export function createCommandLineBuffer(): CommandLineBuffer {
  let current: string[] = []
  const line = (): string => current.join('')

  let state: 'normal' | 'esc' | 'csi' | 'ss3' | 'osc' | 'oscEsc' = 'normal'

  let searching = false
  return {
    feed(data: string): string[] {
      const submitted: string[] = []
      for (const char of data) {
        const code = char.codePointAt(0) ?? 0
        if (state !== 'normal') {
          if (state === 'esc') {
            if (char === '[') state = 'csi'
            else if (char === 'O') state = 'ss3'
            else if (char === ']') state = 'osc'
            else state = 'normal'
            continue
          }
          if (state === 'osc') {
            if (char === '\x07') state = 'normal'
            else if (code === 0x1b) state = 'oscEsc'
            continue
          }
          if (state === 'oscEsc') {
            state = char === '\\' ? 'normal' : 'osc'
            continue
          }
          if (state === 'ss3') {
            state = 'normal'
            continue
          }

          if (code >= 0x40 && code <= 0x7e) state = 'normal'
          continue
        }
        if (searching) {
          if (char === '\r' || char === '\n' || char === '\x07') {
            searching = false
            current = []
          }
          continue
        }
        if (code === 0x1b) {
          state = 'esc'
          continue
        }
        if (code === 0x12) {
          searching = true
          current = []
          continue
        }
        if (char === '\r' || char === '\n') {
          const command = line().trim()
          if (command) submitted.push(command)
          current = []
          continue
        }
        if (char === '\x7f' || char === '\b') {
          current.pop()
          continue
        }

        if (char === '\x03' || char === '\x15') {
          current = []
          continue
        }

        if (char === '\x17') {
          while (current.length && current[current.length - 1] === ' ') current.pop()
          while (current.length && current[current.length - 1] !== ' ') current.pop()
          continue
        }
        if (code < 0x20) continue
        if (current.length < COMMAND_LIMIT) current.push(char)
      }
      return submitted
    },
  }
}
