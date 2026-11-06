import type { TerminalRecording } from '../../bindings/changeme/internal/model/models'
import type { TerminalRecordingEvent } from '../../bindings/changeme/internal/model/models'

const STORAGE_KEY = 'qterm.recordings.v1'

export interface ActiveRecordingEvents {
  cursor: number
  events: TerminalRecordingEvent[]
  truncated: boolean
}

export interface TerminalRecordingPage {
  items: TerminalRecording[]
  page: number
  pageSize: number
  total: number
}

function nativeRuntimeReady(): boolean {
  return typeof window !== 'undefined' && window.location.protocol === 'wails:'
}

function readFallback(): TerminalRecording[] {
  try {
    const value: unknown = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]')
    return Array.isArray(value) ? (value as TerminalRecording[]) : []
  } catch {
    return []
  }
}

function notify(): void {
  window.dispatchEvent(new CustomEvent('qterm:recordings-changed'))
}

export async function listRecordings(): Promise<TerminalRecording[]> {
  if (nativeRuntimeReady()) {
    const { ListTerminalRecordings } = await import('../../bindings/changeme/internal/app/service')
    return (await ListTerminalRecordings()) ?? []
  }
  return readFallback()
}

export async function listRecordingPage(page: number, pageSize: number, query = ''): Promise<TerminalRecordingPage> {
  if (nativeRuntimeReady()) {
    const { ListTerminalRecordingsPage } = await import('../../bindings/changeme/internal/app/service')
    const result = await ListTerminalRecordingsPage(page, pageSize, query)
    return {
      items: result?.items ?? [],
      page: result?.page ?? page,
      pageSize: result?.pageSize ?? pageSize,
      total: result?.total ?? 0,
    }
  }
  const keyword = query.trim().toLowerCase()
  const filtered = readFallback()
    .filter((item) => {
      if (!keyword) return true
      return `${item.hostName} ${item.address} ${item.status} ${item.note ?? ''}`.toLowerCase().includes(keyword)
    })
    .sort((a, b) => b.startedAt.localeCompare(a.startedAt))
  const offset = (page - 1) * pageSize
  return { items: filtered.slice(offset, offset + pageSize), page, pageSize, total: filtered.length }
}

export async function startRecording(sessionId: string): Promise<TerminalRecording> {
  if (!nativeRuntimeReady()) {
    throw new Error('录制功能需要在 Qterm 桌面运行时中使用')
  }
  const { StartRecording } = await import('../../bindings/changeme/internal/app/service')
  const item = await StartRecording(sessionId)
  notify()
  return item
}

export async function startFullRecording(sessionId: string): Promise<TerminalRecording> {
  if (!nativeRuntimeReady()) {
    throw new Error('录制功能需要在 Qterm 桌面运行时中使用')
  }
  const { StartFullRecording } = await import('../../bindings/changeme/internal/app/service')
  return StartFullRecording(sessionId)
}

export async function stopRecording(sessionId: string): Promise<TerminalRecording> {
  if (!nativeRuntimeReady()) {
    throw new Error('录制功能需要在 Qterm 桌面运行时中使用')
  }
  const { StopRecording } = await import('../../bindings/changeme/internal/app/service')
  const item = await StopRecording(sessionId)
  notify()
  return item
}

export async function pauseRecording(sessionId: string): Promise<TerminalRecording> {
  if (!nativeRuntimeReady()) throw new Error('录制功能需要在 Qterm 桌面运行时中使用')
  const { PauseRecording } = await import('../../bindings/changeme/internal/app/service')
  const item = await PauseRecording(sessionId)
  notify()
  return item
}

export async function resumeRecording(sessionId: string): Promise<TerminalRecording> {
  if (!nativeRuntimeReady()) throw new Error('录制功能需要在 Qterm 桌面运行时中使用')
  const { ResumeRecording } = await import('../../bindings/changeme/internal/app/service')
  const item = await ResumeRecording(sessionId)
  notify()
  return item
}

export async function readRecording(id: string): Promise<TerminalRecordingEvent[]> {
  if (!nativeRuntimeReady()) throw new Error('回放功能需要在 Qterm 桌面运行时中使用')
  const { ReadTerminalRecording } = await import('../../bindings/changeme/internal/app/service')
  return (await ReadTerminalRecording(id)) ?? []
}

export async function readActiveRecordingEvents(sessionId: string, cursor: number): Promise<ActiveRecordingEvents> {
  if (!nativeRuntimeReady()) return { cursor, events: [], truncated: false }
  const { ReadActiveRecordingEvents } = await import('../../bindings/changeme/internal/app/service')
  const batch = await ReadActiveRecordingEvents(sessionId, cursor)
  return { cursor: batch?.cursor ?? cursor, events: batch?.events ?? [], truncated: batch?.truncated ?? false }
}

export async function deleteRecording(id: string): Promise<void> {
  if (!nativeRuntimeReady()) throw new Error('删除录制需要在 Qterm 桌面运行时中使用')
  const { DeleteTerminalRecording } = await import('../../bindings/changeme/internal/app/service')
  await DeleteTerminalRecording(id)
  notify()
}

export interface RecordingClearResult {
  deleted: number

  skipped: number

  missing: number

  failed: number
}

export async function deleteRecordings(ids: string[]): Promise<RecordingClearResult> {
  const wanted = [...new Set(ids.filter(Boolean))]
  if (!wanted.length) return { deleted: 0, skipped: 0, missing: 0, failed: 0 }
  if (nativeRuntimeReady()) {
    const { DeleteTerminalRecordings } = await import('../../bindings/changeme/internal/app/service')
    const result = await DeleteTerminalRecordings(wanted)
    notify()
    return {
      deleted: result?.deleted ?? 0,
      skipped: result?.skipped ?? 0,
      missing: result?.missing ?? 0,
      failed: result?.failed ?? 0,
    }
  }
  const index = readFallback()
  const kept = index.filter((item) => !wanted.includes(item.id))
  const removed = index.filter((item) => wanted.includes(item.id))
  const result: RecordingClearResult = {
    deleted: removed.filter((item) => item.status !== 'recording' && item.status !== 'paused').length,
    skipped: removed.filter((item) => item.status === 'recording' || item.status === 'paused').length,
    missing: wanted.filter((id) => !index.some((item) => item.id === id)).length,
    failed: 0,
  }
  localStorage.setItem(STORAGE_KEY, JSON.stringify(kept))
  notify()
  return result
}

export async function exportRecordingsText(ids: string[], filename = 'qterm-recordings.txt'): Promise<boolean> {
  if (!ids.length) return false
  if (!nativeRuntimeReady()) throw new Error('导出录制需要在 Qterm 桌面运行时中使用')
  const { ExportTerminalRecordingsText } = await import('../../bindings/changeme/internal/app/service')
  return (await ExportTerminalRecordingsText(ids, filename)) === true
}

export async function updateRecordingNote(id: string, note: string): Promise<void> {
  if (!nativeRuntimeReady()) throw new Error('录制备注需要在 Qterm 桌面运行时中使用')
  const { UpdateTerminalRecordingNote } = await import('../../bindings/changeme/internal/app/service')
  await UpdateTerminalRecordingNote(id, note)
  notify()
}

export async function exportRecordings(
  ids: string[],
  format: 'txt' | 'cast',
  includeOutput: boolean,
): Promise<boolean> {
  if (!ids.length) return false
  const { ExportTerminalRecordingsWithOptions } = await import('../../bindings/changeme/internal/app/service')
  return (await ExportTerminalRecordingsWithOptions(ids, format, includeOutput)) === true
}

export interface RecordingSettings {
  configured: string
  effective: string
  default: string
}

export async function getRecordingSettings(): Promise<RecordingSettings> {
  if (!nativeRuntimeReady()) return { configured: '', effective: '', default: '' }
  const { GetRecordingSettings } = await import('../../bindings/changeme/internal/app/service')
  const value = await GetRecordingSettings()
  return {
    configured: value?.configured ?? '',
    effective: value?.effective ?? '',
    default: value?.default ?? '',
  }
}

export async function selectRecordingDirectory(): Promise<string> {
  if (!nativeRuntimeReady()) throw new Error('选择录制目录需要在 Qterm 桌面运行时中使用')
  const { SelectRecordingDirectory } = await import('../../bindings/changeme/internal/app/service')
  return (await SelectRecordingDirectory()) ?? ''
}

export async function openRecordingDirectory(): Promise<void> {
  if (!nativeRuntimeReady()) throw new Error('打开录制目录需要在 Qterm 桌面运行时中使用')
  const { OpenRecordingDirectory } = await import('../../bindings/changeme/internal/app/service')
  await OpenRecordingDirectory()
}
