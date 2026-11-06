import { Events } from '@wailsio/runtime'

export const SESSION_ADOPT_REQUEST_EVENT = 'qterm:adopt-session'
export const SESSION_ADOPT_RESULT_EVENT = 'qterm:adopt-session-result'

export const SESSION_ADOPT_TIMEOUT_MS = 6500

export type SessionAdoptionKind = 'terminal' | 'rdp' | 'vnc'

export const SESSION_ADOPT_VNC_KIND = 'vnc' satisfies SessionAdoptionKind

export type SessionAdoptionRebindKind = 'terminal' | 'rdp'

interface SessionAdoptRequestBase {
  title?: string

  source: string

  requestId: string
}

export interface TerminalSessionAdoptRequest extends SessionAdoptRequestBase {
  kind?: 'terminal'
  sessionId: string

  hostId?: string
}

export interface RdpSessionAdoptRequest extends SessionAdoptRequestBase {
  kind: 'rdp'
  sessionId: string
  hostId?: string
}

export interface VncSessionAdoptRequest extends SessionAdoptRequestBase {
  kind: typeof SESSION_ADOPT_VNC_KIND
  hostId: string
}

export type SessionAdoptRequest = TerminalSessionAdoptRequest | RdpSessionAdoptRequest | VncSessionAdoptRequest

export type SessionAdoptionRebindRequest = TerminalSessionAdoptRequest | RdpSessionAdoptRequest

export interface SessionAdoptResult {
  source: string
  requestId: string
  ok: boolean

  sessionId?: string

  hostId?: string
}

export type SessionAdoptionFailure = 'unsupported' | 'no-listener' | 'rejected' | 'invalid'

export type SessionAdoptionOutcome = { ok: true } | { ok: false; reason: SessionAdoptionFailure }

const windowIdentity = `adopt-window-${randomToken()}`

const ADOPT_CLAIM_PREFIX = 'qterm.session-adopt-claim.'

const ADOPT_CLAIM_TTL_MS = 30_000

interface AdoptionClaim {
  owner: string
  at: number
}

function randomToken(): string {
  return globalThis.crypto?.randomUUID?.() ?? `token-${Math.random().toString(36).slice(2)}`
}

function nativeRuntime(): boolean {
  return (
    window.location.protocol === 'wails:' ||
    Boolean((window as Window & { _wails?: { environment?: { OS?: string } } })._wails?.environment?.OS)
  )
}

function claimAdoptionRequest(requestId: string): boolean {
  const key = `${ADOPT_CLAIM_PREFIX}${requestId}`
  try {
    pruneAdoptionClaims()
    const existing = readAdoptionClaim(key)
    if (existing && existing.owner !== windowIdentity && Date.now() - existing.at < ADOPT_CLAIM_TTL_MS) return false
    localStorage.setItem(key, JSON.stringify({ owner: windowIdentity, at: Date.now() } satisfies AdoptionClaim))
    return true
  } catch {
    return true
  }
}

function readAdoptionClaim(key: string): AdoptionClaim | null {
  const raw = localStorage.getItem(key)
  if (!raw) return null
  try {
    const claim = JSON.parse(raw) as AdoptionClaim | null
    return claim && typeof claim.owner === 'string' && typeof claim.at === 'number' ? claim : null
  } catch {
    return null
  }
}

function pruneAdoptionClaims(): void {
  const now = Date.now()
  const stale: string[] = []
  for (let index = 0; index < localStorage.length; index += 1) {
    const key = localStorage.key(index)
    if (!key?.startsWith(ADOPT_CLAIM_PREFIX)) continue
    const claim = readAdoptionClaim(key)
    if (!claim || now - claim.at >= ADOPT_CLAIM_TTL_MS) stale.push(key)
  }
  for (const key of stale) localStorage.removeItem(key)
}

export function requestSessionAdoption(
  sessionId: string,
  title?: string,
  kind: SessionAdoptionRebindKind = 'terminal',
  hostId?: string,
): Promise<SessionAdoptionOutcome> {
  const id = sessionId.trim()
  if (!id) return Promise.resolve({ ok: false, reason: 'invalid' })
  if (!nativeRuntime()) return Promise.resolve({ ok: false, reason: 'unsupported' })

  const requestId = randomToken()
  return new Promise<SessionAdoptionOutcome>((resolve) => {
    const state = { settled: false, timer: 0 }
    const stopListening = Events.On(SESSION_ADOPT_RESULT_EVENT, (event) => {
      const data = event.data as SessionAdoptResult | null
      if (!data || data.source !== windowIdentity || data.requestId !== requestId || data.sessionId !== id) return
      finish(data.ok ? { ok: true } : { ok: false, reason: 'rejected' })
    })
    function finish(outcome: SessionAdoptionOutcome): void {
      if (state.settled) return
      state.settled = true
      window.clearTimeout(state.timer)
      stopListening()
      resolve(outcome)
    }
    state.timer = window.setTimeout(() => finish({ ok: false, reason: 'no-listener' }), SESSION_ADOPT_TIMEOUT_MS)
    void Events.Emit(SESSION_ADOPT_REQUEST_EVENT, {
      sessionId: id,
      kind,
      hostId,
      title,
      source: windowIdentity,
      requestId,
    } satisfies SessionAdoptRequest).catch(() => finish({ ok: false, reason: 'unsupported' }))
  })
}

export function listenSessionAdoptionRequests(
  handle: (request: SessionAdoptRequest) => Promise<boolean> | boolean,
): () => void {
  return Events.On(SESSION_ADOPT_REQUEST_EVENT, (event) => {
    const request = event.data as SessionAdoptRequest | null
    void respondToAdoptionRequest(request, handle)
  })
}

async function respondToAdoptionRequest(
  request: SessionAdoptRequest | null,
  handle: (request: SessionAdoptRequest) => Promise<boolean> | boolean,
): Promise<void> {
  if (!validSessionAdoptionRequest(request)) return
  if (!claimAdoptionRequest(request.requestId)) return
  const ok = await adoptSafely(handle, request)

  await Events.Emit(SESSION_ADOPT_RESULT_EVENT, {
    sessionId: 'sessionId' in request ? request.sessionId : undefined,
    hostId: request.kind === SESSION_ADOPT_VNC_KIND ? request.hostId : undefined,
    source: request.source,
    requestId: request.requestId,
    ok,
  } satisfies SessionAdoptResult).catch(() => undefined)
}

function validSessionAdoptionRequest(request: SessionAdoptRequest | null): request is SessionAdoptRequest {
  if (!request?.source || !request.requestId || typeof request.requestId !== 'string') {
    return false
  }
  switch (request.kind) {
    case undefined:
    case 'terminal':
    case 'rdp':
      return typeof request.sessionId === 'string' && request.sessionId.trim().length > 0
    case SESSION_ADOPT_VNC_KIND:
      return typeof request.hostId === 'string' && request.hostId.trim().length > 0
    default:
      console.warn(`[sessionAdoption] 收到未知 kind，已拒绝接管请求：${String((request as { kind?: unknown }).kind)}`)
      return false
  }
}

async function adoptSafely(
  handle: (request: SessionAdoptRequest) => Promise<boolean> | boolean,
  request: SessionAdoptRequest,
): Promise<boolean> {
  try {
    return await handle(request)
  } catch {
    return false
  }
}
