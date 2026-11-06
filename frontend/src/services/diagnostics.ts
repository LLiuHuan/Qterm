function hasNativeDiagnostics(): boolean {
  return Boolean((window as Window & { _wails?: { environment?: { OS?: string } } })._wails?.environment?.OS)
}

export function diagnosticsAvailable(): boolean {
  return hasNativeDiagnostics()
}

export async function debugLoggingEnabled(): Promise<boolean> {
  if (!hasNativeDiagnostics()) return false
  const { DebugLoggingEnabled } = await import('../../bindings/changeme/internal/app/service')
  return DebugLoggingEnabled()
}

export async function setDebugLoggingEnabled(enabled: boolean): Promise<void> {
  if (!hasNativeDiagnostics()) return
  const { SetDebugLoggingEnabled } = await import('../../bindings/changeme/internal/app/service')
  await SetDebugLoggingEnabled(enabled)
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
