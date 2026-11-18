export type DebugLogMode = 'off' | 'normal' | 'debug'

export interface DebugLogStatus {
  mode: DebugLogMode
  path: string
  sizeBytes: number
  limitBytes: number
  dropped: number
  present: boolean
}

const DEBUG_LOG_MODES: readonly DebugLogMode[] = ['off', 'normal', 'debug']

function hasNativeDiagnostics(): boolean {
  return Boolean((window as Window & { _wails?: { environment?: { OS?: string } } })._wails?.environment?.OS)
}

function normalizeDebugLogMode(value: string): DebugLogMode {
  return (DEBUG_LOG_MODES as readonly string[]).includes(value) ? (value as DebugLogMode) : 'normal'
}

export function diagnosticsAvailable(): boolean {
  return hasNativeDiagnostics()
}

export async function debugLogMode(): Promise<DebugLogMode> {
  if (!hasNativeDiagnostics()) return 'off'
  const { DebugLogMode: readMode } = await import('../../bindings/changeme/internal/app/service')
  return normalizeDebugLogMode(await readMode())
}

export async function setDebugLogMode(mode: DebugLogMode): Promise<void> {
  if (!hasNativeDiagnostics()) return
  const { SetDebugLogMode } = await import('../../bindings/changeme/internal/app/service')
  await SetDebugLogMode(mode)
}

export async function debugLogStatus(): Promise<DebugLogStatus | null> {
  if (!hasNativeDiagnostics()) return null
  const { DebugLogStatus: readStatus } = await import('../../bindings/changeme/internal/app/service')
  const status = await readStatus()
  return {
    mode: normalizeDebugLogMode(status.mode),
    path: status.path,
    sizeBytes: status.sizeBytes,
    limitBytes: status.limitBytes,
    dropped: status.dropped,
    present: status.present,
  }
}

export async function readDebugLog(): Promise<string> {
  if (!hasNativeDiagnostics()) return ''
  const { ReadDebugLog } = await import('../../bindings/changeme/internal/app/service')
  return ReadDebugLog()
}

export async function clearDebugLog(): Promise<void> {
  if (!hasNativeDiagnostics()) return
  const { ClearDebugLog } = await import('../../bindings/changeme/internal/app/service')
  await ClearDebugLog()
}

export async function exportDebugLog(): Promise<boolean> {
  if (!hasNativeDiagnostics()) return false
  const { ExportDebugLog } = await import('../../bindings/changeme/internal/app/service')
  return ExportDebugLog()
}
