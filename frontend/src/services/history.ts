import type { SessionHistory as SessionHistoryDTO } from '../../bindings/changeme/internal/model/models'

export interface SessionHistoryRecord {
  id: string
  hostId: string
  hostName: string
  address: string
  startedAt: string
  endedAt?: string

  status: 'connected' | 'failed' | 'closed' | 'interrupted'
  error?: string
  note?: string
  archived?: boolean
}

const STORAGE_KEY = 'qterm.session-history.v1'
export const DEFAULT_SESSION_HISTORY_LIMIT = 200

export function optionalTimestamp(value?: string): string | undefined {
  if (!value || value.startsWith('0001-01-01')) return undefined
  return value
}

type SessionHistoryPayload = Omit<SessionHistoryDTO, 'endedAt'> & { endedAt?: string }

function historyPayload(record: SessionHistoryRecord): SessionHistoryPayload {
  const payload: SessionHistoryPayload = {
    id: record.id,
    hostId: record.hostId,
    hostName: record.hostName,
    address: record.address,
    startedAt: record.startedAt,
    status: record.status,
    errorKey: record.error ?? '',
    note: record.note ?? '',
    archived: record.archived === true,
  }
  const endedAt = optionalTimestamp(record.endedAt)
  if (endedAt) payload.endedAt = endedAt
  return payload
}

function asBindingPayload(payload: SessionHistoryPayload): SessionHistoryDTO {
  return payload as unknown as SessionHistoryDTO
}

function nativeRuntimeReady(): boolean {
  return typeof window !== 'undefined' && window.location.protocol === 'wails:'
}

function read(): SessionHistoryRecord[] {
  try {
    const value: unknown = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]')
    return Array.isArray(value)
      ? value.filter((item): item is SessionHistoryRecord => Boolean(item && typeof item === 'object'))
      : []
  } catch {
    return []
  }
}

function write(records: SessionHistoryRecord[]): void {
  const active = records.filter((item) => item.archived !== true).slice(0, DEFAULT_SESSION_HISTORY_LIMIT)
  const archived = records.filter((item) => item.archived === true)
  localStorage.setItem(STORAGE_KEY, JSON.stringify([...active, ...archived]))
  window.dispatchEvent(new CustomEvent('qterm:session-history-changed'))
}

export async function listSessionHistory(): Promise<SessionHistoryRecord[]> {
  if (nativeRuntimeReady()) {
    const { ListSessionHistory } = await import('../../bindings/changeme/internal/app/service')
    const items = await ListSessionHistory()
    return (items ?? []).map((item) => ({
      id: item.id,
      hostId: item.hostId,
      hostName: item.hostName,
      address: item.address,
      startedAt: item.startedAt,
      endedAt: optionalTimestamp(item.endedAt),
      status: item.status as SessionHistoryRecord['status'],
      error: item.errorKey || undefined,
      note: item.note || undefined,
      archived: item.archived === true,
    }))
  }
  return read()
    .map((item) => ({ ...item, endedAt: optionalTimestamp(item.endedAt) }))
    .sort((a, b) => b.startedAt.localeCompare(a.startedAt))
}

export async function recordSessionStart(record: Omit<SessionHistoryRecord, 'status'>): Promise<void> {
  if (nativeRuntimeReady()) {
    const { SaveSessionHistory } = await import('../../bindings/changeme/internal/app/service')
    await SaveSessionHistory(asBindingPayload(historyPayload({ ...record, status: 'connected' })))
    window.dispatchEvent(new CustomEvent('qterm:session-history-changed'))
    return
  }
  write([{ ...record, status: 'connected' }, ...read().filter((item) => item.id !== record.id)])
}

export async function finishSessionHistory(
  id: string,
  status: SessionHistoryRecord['status'],
  error?: string,
): Promise<void> {
  if (nativeRuntimeReady()) {
    const records = await listSessionHistory()
    const current = records.find((item) => item.id === id)
    if (!current) return
    const { SaveSessionHistory } = await import('../../bindings/changeme/internal/app/service')
    await SaveSessionHistory(
      asBindingPayload(
        historyPayload({
          ...current,
          status,
          endedAt: new Date().toISOString(),
          error: error ?? current.error,
        }),
      ),
    )
    window.dispatchEvent(new CustomEvent('qterm:session-history-changed'))
    return
  }
  write(
    read().map((item) =>
      item.id === id ? { ...item, status, endedAt: new Date().toISOString(), ...(error ? { error } : {}) } : item,
    ),
  )
}

export async function deleteSessionHistory(id: string): Promise<void> {
  if (nativeRuntimeReady()) {
    const { DeleteSessionHistory } = await import('../../bindings/changeme/internal/app/service')
    await DeleteSessionHistory(id)
    window.dispatchEvent(new CustomEvent('qterm:session-history-changed'))
    return
  }
  write(read().filter((item) => item.id !== id))
}

export async function archiveSessionHistory(id: string, archived = true): Promise<void> {
  if (nativeRuntimeReady()) {
    const { ArchiveSessionHistory } = await import('../../bindings/changeme/internal/app/service')
    await ArchiveSessionHistory(id, archived)
    window.dispatchEvent(new CustomEvent('qterm:session-history-changed'))
    return
  }
  write(read().map((item) => (item.id === id ? { ...item, archived } : item)))
}

export interface SessionHistoryArchiveResult {
  updated: number
  missing: number
}

export async function archiveSessionHistories(ids: string[], archived = true): Promise<SessionHistoryArchiveResult> {
  if (!ids.length) return { updated: 0, missing: 0 }
  if (nativeRuntimeReady()) {
    const { SetSessionHistoryArchived } = await import('../../bindings/changeme/internal/app/service')
    const result = await SetSessionHistoryArchived(ids, archived)
    window.dispatchEvent(new CustomEvent('qterm:session-history-changed'))
    return { updated: result?.updated ?? 0, missing: result?.missing ?? 0 }
  }
  const records = read()
  const wanted = new Set(records.filter((item) => ids.includes(item.id)).map((item) => item.id))
  let updated = 0
  let missing = 0
  for (const id of ids) {
    if (!wanted.has(id)) missing += 1
  }
  write(
    records.map((item) => {
      if (!wanted.has(item.id) || (item.archived === true) === archived) return item
      updated += 1
      return { ...item, archived }
    }),
  )
  return { updated, missing }
}

export async function trimSessionHistory(limit: number): Promise<void> {
  if (nativeRuntimeReady()) {
    const { TrimSessionHistory } = await import('../../bindings/changeme/internal/app/service')
    await TrimSessionHistory(limit)
    window.dispatchEvent(new CustomEvent('qterm:session-history-changed'))
    return
  }
  const records = read()
  const active = records.filter((item) => item.archived !== true).slice(0, limit)
  const archived = records.filter((item) => item.archived === true)
  write([...active, ...archived])
}

export async function setSessionHistoryLimit(limit: number): Promise<void> {
  if (nativeRuntimeReady()) {
    const bindings =
      (await import('../../bindings/changeme/internal/app/service')) as typeof import('../../bindings/changeme/internal/app/service') & {
        SetSessionHistoryLimit?: (value: number) => PromiseLike<void>
      }
    if (bindings.SetSessionHistoryLimit) await bindings.SetSessionHistoryLimit(limit)
    else await bindings.TrimSessionHistory(limit)
    window.dispatchEvent(new CustomEvent('qterm:session-history-changed'))
    return
  }
  await trimSessionHistory(limit)
}

export async function updateSessionHistoryNote(id: string, note: string): Promise<void> {
  const records = await listSessionHistory()
  const current = records.find((item) => item.id === id)
  if (!current) return
  if (nativeRuntimeReady()) {
    const { SaveSessionHistory } = await import('../../bindings/changeme/internal/app/service')
    await SaveSessionHistory(asBindingPayload(historyPayload({ ...current, note })))
    window.dispatchEvent(new CustomEvent('qterm:session-history-changed'))
    return
  }
  write(records.map((item) => (item.id === id ? { ...item, note } : item)))
}

export async function exportSessionHistoryText(id: string): Promise<boolean> {
  if (!nativeRuntimeReady()) throw new Error('导出会话记录需要在 Qterm 桌面运行时中使用')
  const { ExportSessionHistoryText } = await import('../../bindings/changeme/internal/app/service')
  return Boolean(await ExportSessionHistoryText(id))
}

export interface SessionHistoryClearResult {
  sessions: number

  recordings: number

  skipped: number

  failed: number
}

export async function clearUnarchivedSessionHistory(): Promise<SessionHistoryClearResult> {
  if (nativeRuntimeReady()) {
    const { ClearUnarchivedSessionHistory } = await import('../../bindings/changeme/internal/app/service')
    const result = await ClearUnarchivedSessionHistory()
    window.dispatchEvent(new CustomEvent('qterm:session-history-changed'))

    window.dispatchEvent(new CustomEvent('qterm:recordings-changed'))
    return {
      sessions: result?.sessions ?? 0,
      recordings: result?.recordings ?? 0,
      skipped: result?.skipped ?? 0,
      failed: result?.failed ?? 0,
    }
  }
  const records = read()
  const archived = records.filter((item) => item.archived === true)
  const removed = records.length - archived.length
  write(archived)
  return { sessions: removed, recordings: 0, skipped: 0, failed: 0 }
}
