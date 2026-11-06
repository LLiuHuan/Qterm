export interface AICommandAudit {
  at: number

  sessionTitle: string
  sessionId: string
  command: string
}

const AUDIT_KEY = 'qterm.ai.audit.v1'
const AUDIT_LIMIT = 200

export function readAudit(): AICommandAudit[] {
  try {
    const raw = window.localStorage.getItem(AUDIT_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw) as unknown
    if (!Array.isArray(parsed)) return []
    return parsed.filter((entry): entry is AICommandAudit => {
      if (!entry || typeof entry !== 'object') return false
      const item = entry as Partial<AICommandAudit>
      return (
        typeof item.at === 'number' &&
        typeof item.sessionId === 'string' &&
        typeof item.sessionTitle === 'string' &&
        typeof item.command === 'string'
      )
    })
  } catch {
    return []
  }
}

export function recordAudit(entry: AICommandAudit): void {
  try {
    const next = [entry, ...readAudit()].slice(0, AUDIT_LIMIT)
    window.localStorage.setItem(AUDIT_KEY, JSON.stringify(next))
  } catch {}
}
