const STORAGE_KEY = 'qterm.open-sessions.v1'

export function isRestorableHostId(hostId: string): boolean {
  return !!hostId && !hostId.startsWith('adhoc-')
}

export function readOpenSessions(): string[] {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw) as { hostIds?: unknown }
    if (!Array.isArray(parsed.hostIds)) return []
    return parsed.hostIds.filter((value): value is string => typeof value === 'string' && isRestorableHostId(value))
  } catch {
    return []
  }
}

export function writeOpenSessions(hostIds: string[]): void {
  try {
    const restorable = hostIds.filter(isRestorableHostId)
    if (!restorable.length) {
      window.localStorage.removeItem(STORAGE_KEY)
      return
    }
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify({ hostIds: restorable }))
  } catch {}
}
