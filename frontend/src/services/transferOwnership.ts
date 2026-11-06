import { shallowRef } from 'vue'
import { Events } from '@wailsio/runtime'

export const TRANSFER_CLAIM_EVENT = 'qterm:transfer-claim'

export const CONFLICT_TAKEOVER_GRACE_MS = 6_000

const CLAIM_KEY_PREFIX = 'qterm.transfer-claim.'
const TRANSFER_KEY_PREFIX = `${CLAIM_KEY_PREFIX}t.`
const SESSION_KEY_PREFIX = `${CLAIM_KEY_PREFIX}s.`
const CONFLICT_KEY_PREFIX = `${CLAIM_KEY_PREFIX}c.`

const CLAIM_TTL_MS = 15_000

const CLAIM_HEARTBEAT_MS = 5_000

const CLAIM_GC_MS = 60 * 60 * 1000

const windowIdentity = `transfer-window-${randomToken()}`

export interface TransferOwnershipTarget {
  id: string

  sessionId: string
}

export interface TransferOwnership {
  ownTransferIds: ReadonlySet<string>

  ownSessionIds: ReadonlySet<string>

  ownerByTransferId: ReadonlyMap<string, string>

  ownerBySessionId: ReadonlyMap<string, string>
}

interface StoredClaim {
  owner: string
  at: number
}

interface ClaimBroadcast extends StoredClaim {
  kind: 'transfer' | 'session'
  id: string
}

const ownTransferIds = shallowRef<ReadonlySet<string>>(new Set<string>())
const ownSessionIds = shallowRef<ReadonlySet<string>>(new Set<string>())
const ownerByTransferId = shallowRef<ReadonlyMap<string, string>>(new Map<string, string>())
const ownerBySessionId = shallowRef<ReadonlyMap<string, string>>(new Map<string, string>())

let heartbeat: ReturnType<typeof setInterval> | undefined
let stopClaimListener: (() => void) | undefined

function randomToken(): string {
  return globalThis.crypto?.randomUUID?.() ?? `token-${Math.random().toString(36).slice(2)}`
}

function nativeRuntime(): boolean {
  return (
    window.location.protocol === 'wails:' ||
    Boolean((window as Window & { _wails?: { environment?: { OS?: string } } })._wails?.environment?.OS)
  )
}

function transferKey(id: string): string {
  return `${TRANSFER_KEY_PREFIX}${id}`
}

function sessionKey(id: string): string {
  return `${SESSION_KEY_PREFIX}${id}`
}

function conflictKey(id: string): string {
  return `${CONFLICT_KEY_PREFIX}${id}`
}

function readClaim(key: string): StoredClaim | null {
  try {
    const raw = localStorage.getItem(key)
    if (!raw) return null
    const claim = JSON.parse(raw) as StoredClaim | null
    if (!claim || typeof claim.owner !== 'string' || typeof claim.at !== 'number') return null
    return claim
  } catch {
    return null
  }
}

function takeClaim(key: string): 'claimed' | 'taken' | 'unavailable' {
  try {
    const existing = readClaim(key)
    if (existing && existing.owner !== windowIdentity && Date.now() - existing.at < CLAIM_TTL_MS) return 'taken'
    localStorage.setItem(key, JSON.stringify({ owner: windowIdentity, at: Date.now() } satisfies StoredClaim))
    return 'claimed'
  } catch {
    return 'unavailable'
  }
}

function renewClaim(key: string): void {
  try {
    const existing = readClaim(key)
    if (existing && existing.owner !== windowIdentity) return
    localStorage.setItem(key, JSON.stringify({ owner: windowIdentity, at: Date.now() } satisfies StoredClaim))
  } catch {}
}

function withId(current: ReadonlySet<string>, rawId: string): ReadonlySet<string> {
  const id = rawId.trim()
  if (!id || current.has(id)) return current
  const next = new Set(current)
  next.add(id)
  return next
}

function withOwner(current: ReadonlyMap<string, string>, id: string, owner: string): ReadonlyMap<string, string> {
  if (!owner || current.get(id) === owner) return current
  const next = new Map(current)
  next.set(id, owner)
  return next
}

function readSharedClaims(): { transfers: Map<string, string>; sessions: Map<string, string> } {
  const transfers = new Map<string, string>()
  const sessions = new Map<string, string>()
  try {
    const now = Date.now()
    const stale: string[] = []
    for (let index = 0; index < localStorage.length; index += 1) {
      const key = localStorage.key(index)
      if (!key?.startsWith(CLAIM_KEY_PREFIX)) continue
      const claim = readClaim(key)
      if (!claim) {
        stale.push(key)
        continue
      }
      const age = now - claim.at

      if (age >= CLAIM_TTL_MS) {
        if (age >= CLAIM_GC_MS) stale.push(key)
        continue
      }
      if (key.startsWith(TRANSFER_KEY_PREFIX)) transfers.set(key.slice(TRANSFER_KEY_PREFIX.length), claim.owner)
      else if (key.startsWith(SESSION_KEY_PREFIX)) sessions.set(key.slice(SESSION_KEY_PREFIX.length), claim.owner)
    }
    for (const key of stale) localStorage.removeItem(key)
  } catch {}
  return { transfers, sessions }
}

function renewOwnClaims(): void {
  for (const id of ownTransferIds.value) renewClaim(transferKey(id))
  for (const id of ownSessionIds.value) renewClaim(sessionKey(id))
}

function ensureHeartbeat(): void {
  if (heartbeat) return
  heartbeat = setInterval(renewOwnClaims, CLAIM_HEARTBEAT_MS)
}

function broadcastClaim(kind: 'transfer' | 'session', id: string): void {
  if (!nativeRuntime()) return
  void Events.Emit(TRANSFER_CLAIM_EVENT, {
    kind,
    id,
    owner: windowIdentity,
    at: Date.now(),
  } satisfies ClaimBroadcast).catch(() => undefined)
}

export function claimTransfer(id: string): void {
  const trimmed = id.trim()
  if (!trimmed) return
  const key = transferKey(trimmed)
  if (takeClaim(key) === 'taken') {
    const owner = readClaim(key)?.owner
    if (owner) ownerByTransferId.value = withOwner(ownerByTransferId.value, trimmed, owner)
    return
  }
  ownTransferIds.value = withId(ownTransferIds.value, trimmed)
  ownerByTransferId.value = withOwner(ownerByTransferId.value, trimmed, windowIdentity)
  ensureHeartbeat()
  broadcastClaim('transfer', trimmed)
}

export function claimSession(sessionId: string): void {
  const trimmed = sessionId.trim()
  if (!trimmed) return
  const key = sessionKey(trimmed)
  if (takeClaim(key) === 'taken') {
    const owner = readClaim(key)?.owner
    if (owner) ownerBySessionId.value = withOwner(ownerBySessionId.value, trimmed, owner)
    return
  }
  ownSessionIds.value = withId(ownSessionIds.value, trimmed)
  ownerBySessionId.value = withOwner(ownerBySessionId.value, trimmed, windowIdentity)
  ensureHeartbeat()
  broadcastClaim('session', trimmed)
}

export function claimConflictAnswer(transferId: string): boolean {
  const trimmed = transferId.trim()
  return !trimmed || takeClaim(conflictKey(trimmed)) !== 'taken'
}

export function renewConflictAnswer(transferId: string): void {
  const trimmed = transferId.trim()
  if (trimmed) renewClaim(conflictKey(trimmed))
}

export function readSharedOwnership(): Pick<TransferOwnership, 'ownerByTransferId' | 'ownerBySessionId'> {
  const shared = readSharedClaims()
  return { ownerByTransferId: shared.transfers, ownerBySessionId: shared.sessions }
}

export function refreshSharedOwnership(): void {
  const shared = readSharedOwnership()
  ownerByTransferId.value = shared.ownerByTransferId
  ownerBySessionId.value = shared.ownerBySessionId
  renewOwnClaims()
}

export function listenSharedTransferClaims(): () => void {
  if (stopClaimListener) return stopClaimListener
  if (!nativeRuntime()) return () => undefined
  const stop = Events.On(TRANSFER_CLAIM_EVENT, (event) => {
    const claim = event.data as ClaimBroadcast | null
    if (!claim?.id || !claim.owner) return
    if (claim.kind === 'transfer') ownerByTransferId.value = withOwner(ownerByTransferId.value, claim.id, claim.owner)
    else if (claim.kind === 'session') ownerBySessionId.value = withOwner(ownerBySessionId.value, claim.id, claim.owner)
  })
  stopClaimListener = stop
  return stop
}

export function currentOwnership(): TransferOwnership {
  return {
    ownTransferIds: ownTransferIds.value,
    ownSessionIds: ownSessionIds.value,
    ownerByTransferId: ownerByTransferId.value,
    ownerBySessionId: ownerBySessionId.value,
  }
}

export function isOwned(record: TransferOwnershipTarget, owned: TransferOwnership): boolean {
  return owned.ownTransferIds.has(record.id) || owned.ownSessionIds.has(record.sessionId)
}

export function isClaimedByOther(record: TransferOwnershipTarget, owned: TransferOwnership): boolean {
  if (isOwned(record, owned)) return false
  return owned.ownerByTransferId.has(record.id) || owned.ownerBySessionId.has(record.sessionId)
}

export function isUnclaimed(record: TransferOwnershipTarget, owned: TransferOwnership): boolean {
  return !isOwned(record, owned) && !isClaimedByOther(record, owned)
}

export function owns(record: TransferOwnershipTarget): boolean {
  return isOwned(record, currentOwnership())
}

export function filterOwnedItems<T extends TransferOwnershipTarget>(
  records: readonly T[],
  owned: TransferOwnership,
  includeUnclaimed: boolean,
): T[] {
  return records.filter((record) => isOwned(record, owned) || (includeUnclaimed && isUnclaimed(record, owned)))
}

export function shouldHandleConflict(
  record: TransferOwnershipTarget,
  owned: TransferOwnership,
  options: { primaryWindow: boolean; conflictAgeMs: number },
): boolean {
  if (isOwned(record, owned)) return true
  if (!options.primaryWindow) return false
  if (isUnclaimed(record, owned)) return true
  return options.conflictAgeMs >= CONFLICT_TAKEOVER_GRACE_MS
}

export function resetTransferOwnershipForTests(): void {
  if (heartbeat) clearInterval(heartbeat)
  heartbeat = undefined
  stopClaimListener?.()
  stopClaimListener = undefined
  ownTransferIds.value = new Set<string>()
  ownSessionIds.value = new Set<string>()
  ownerByTransferId.value = new Map<string, string>()
  ownerBySessionId.value = new Map<string, string>()
  try {
    const stale: string[] = []
    for (let index = 0; index < localStorage.length; index += 1) {
      const key = localStorage.key(index)
      if (key?.startsWith(CLAIM_KEY_PREFIX)) stale.push(key)
    }
    for (const key of stale) localStorage.removeItem(key)
  } catch {}
}
