import { Events } from '@wailsio/runtime'
import {
  SESSION_ADOPT_REQUEST_EVENT,
  SESSION_ADOPT_RESULT_EVENT,
  SESSION_ADOPT_TIMEOUT_MS,
  SESSION_ADOPT_VNC_KIND,
  type SessionAdoptResult,
  type SessionAdoptionOutcome,
  type VncSessionAdoptRequest,
} from '../../services/sessionAdoptionChannel'

const windowIdentity = `vnc-window-${randomToken()}`

function randomToken(): string {
  return globalThis.crypto?.randomUUID?.() ?? `token-${Math.random().toString(36).slice(2)}`
}

function nativeRuntime(): boolean {
  return (
    window.location.protocol === 'wails:' ||
    Boolean((window as Window & { _wails?: { environment?: { OS?: string } } })._wails?.environment?.OS)
  )
}

export function requestVncTabOpen(hostId: string, title?: string): Promise<SessionAdoptionOutcome> {
  const id = hostId.trim()
  if (!id) return Promise.resolve({ ok: false, reason: 'invalid' })
  if (!nativeRuntime()) return Promise.resolve({ ok: false, reason: 'unsupported' })

  const requestId = randomToken()
  return new Promise<SessionAdoptionOutcome>((resolve) => {
    const state = { settled: false, timer: 0 }
    const stopListening = Events.On(SESSION_ADOPT_RESULT_EVENT, (event) => {
      const data = event.data as SessionAdoptResult | null

      if (!data || data.source !== windowIdentity || data.requestId !== requestId || data.hostId !== id) return
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
      kind: SESSION_ADOPT_VNC_KIND,

      hostId: id,
      title,
      source: windowIdentity,
      requestId,
    } satisfies VncSessionAdoptRequest).catch(() => finish({ ok: false, reason: 'unsupported' }))
  })
}
