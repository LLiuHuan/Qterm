export interface PendingHostCredential {
  password: string

  remember: boolean

  createdAt: number
}

const TTL_MS = 60_000

const pending = new Map<string, PendingHostCredential>()

export function stashHostCredential(hostId: string, password: string, remember: boolean): void {
  if (!hostId || !password) return
  pending.set(hostId, { password, remember, createdAt: Date.now() })
}

export function takeHostCredential(hostId: string): PendingHostCredential | null {
  const entry = pending.get(hostId)
  if (!entry) return null
  pending.delete(hostId)
  if (Date.now() - entry.createdAt > TTL_MS) return null
  return entry
}

export function dropHostCredential(hostId: string): void {
  pending.delete(hostId)
}

export function resetHostCredentials(): void {
  pending.clear()
}
