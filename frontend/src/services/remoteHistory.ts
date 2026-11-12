import { createRuntimeGateway } from './runtime'

export type RemoteHistoryShell = 'bash' | 'zsh' | 'fish'

export interface RemoteHistoryEntry {
  shell: string

  command: string

  timestamp: number

  source: string
}

export interface RemoteHistoryResult {
  entries: RemoteHistoryEntry[]

  truncated: boolean

  home?: string
}

export interface RemoteHistoryGateway {
  readonly persistent: boolean
  listRemoteHistory(sessionId: string): Promise<RemoteHistoryResult>
}

function normalizeRemoteHistory(value: unknown): RemoteHistoryResult {
  const payload = (value ?? {}) as Partial<RemoteHistoryResult>
  const entries = Array.isArray(payload.entries) ? payload.entries : []
  return {
    entries: entries
      .filter((entry): entry is RemoteHistoryEntry => Boolean(entry) && typeof entry.command === 'string')
      .map((entry) => ({
        shell: typeof entry.shell === 'string' ? entry.shell : '',
        command: entry.command,
        timestamp: Number.isFinite(entry.timestamp) ? entry.timestamp : 0,
        source: typeof entry.source === 'string' ? entry.source : '',
      })),
    truncated: Boolean(payload.truncated),
    home: typeof payload.home === 'string' && payload.home ? payload.home : undefined,
  }
}

class WailsRemoteHistoryGateway implements RemoteHistoryGateway {
  readonly persistent = true

  async listRemoteHistory(sessionId: string): Promise<RemoteHistoryResult> {
    const { ListRemoteHistory } = await import('../../bindings/changeme/internal/app/service')
    return normalizeRemoteHistory(await ListRemoteHistory(sessionId))
  }
}

class MemoryRemoteHistoryGateway implements RemoteHistoryGateway {
  readonly persistent = false

  async listRemoteHistory(): Promise<RemoteHistoryResult> {
    return { entries: [], truncated: false }
  }
}

export function createRemoteHistoryGateway(): RemoteHistoryGateway {
  return createRuntimeGateway<RemoteHistoryGateway>(
    () => new WailsRemoteHistoryGateway(),
    () => new MemoryRemoteHistoryGateway(),
  )
}

export function sortEntriesForDisplay(entries: RemoteHistoryEntry[]): RemoteHistoryEntry[] {
  const stamped = entries.filter((entry) => entry.timestamp > 0)
  const unstamped = entries.filter((entry) => entry.timestamp <= 0)
  const sorted = [...stamped].sort((left, right) => right.timestamp - left.timestamp)
  return [...sorted, ...unstamped.reverse()]
}

export function filterEntries(entries: RemoteHistoryEntry[], query: string): RemoteHistoryEntry[] {
  const keyword = query.trim().toLowerCase()
  if (!keyword) return entries
  return entries.filter(
    (entry) => entry.command.toLowerCase().includes(keyword) || entry.shell.toLowerCase().includes(keyword),
  )
}
