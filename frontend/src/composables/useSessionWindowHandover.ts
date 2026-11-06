import { ref, type Ref } from 'vue'
import { toast } from 'vue-sonner'
import { useI18n } from '../locales'
import { hostErrorMessage, type HostRecord } from '../services/hosts'
import { closeCurrentWindow, openSessionWindow } from '../services/sessionWindows'
import {
  listenSessionAdoptionRequests,
  requestSessionAdoption,
  type SessionAdoptRequest,
  type SessionAdoptionRebindRequest,
  type VncSessionAdoptRequest,
} from '../services/sessionAdoptionChannel'
import type { SessionGateway } from '../services/sessions'
import type { SessionTab } from '../types/qterm'
import { createSessionHandoverWatcher, type HandoverNotice } from './useSessionHandover'

export interface SessionWindowHandoverOptions {
  sessionGateway: SessionGateway

  isSessionWindow: boolean
  sessions: Ref<SessionTab[]>
  activeSession: Readonly<Ref<SessionTab | null>>
  activeSessionId: Ref<string | null>
  hosts: Ref<HostRecord[]>

  openSession: (host: HostRecord) => Promise<SessionTab | null>
  openLocalSession: () => Promise<SessionTab | null>
  adoptSession: (sessionId: string, title?: string) => Promise<boolean>
  detachSession: (sessionId: string) => void
  clearSessionToolState: (sessionId: string) => void

  adoptRdpSession?: (request: SessionAdoptionRebindRequest) => Promise<boolean>

  adoptVncTab?: (request: VncSessionAdoptRequest) => Promise<boolean> | boolean
}

export interface SessionWindowHandover {
  returningToTabs: Readonly<Ref<boolean>>
  returnActiveSessionToTabs(): Promise<void>
  openSessionInNewWindow(sessionId: string): Promise<void>

  start(): void

  dispose(): void

  stopAdoptionListener(): void
}

export function useSessionWindowHandover(options: SessionWindowHandoverOptions): SessionWindowHandover {
  const { t } = useI18n()
  const returningToTabs = ref(false)
  let stopAdoptionRequests: (() => void) | undefined

  const handoverWatcher = createSessionHandoverWatcher({
    sessionGateway: options.sessionGateway,
    onNotice: handleHandoverNotice,
  })

  function handleHandoverNotice(notice: HandoverNotice): void {
    if (notice.reason === 'closed') {
      toast.warning(t('shell.app.handoverClosed'), {
        id: `handover-closed-${notice.sessionId}`,
        action: { label: t('shell.app.handoverClosedAction'), onClick: () => void reconnectHandoverSession(notice) },
      })
      return
    }
    toast.warning(t('shell.app.handoverPending'), {
      id: `handover-pending-${notice.sessionId}`,
      action: { label: t('shell.app.handoverReclaimAction'), onClick: () => void reclaimHandoverSession(notice) },
    })
  }

  async function reconnectHandoverSession(notice: HandoverNotice): Promise<void> {
    if (notice.kind === 'local') {
      await options.openLocalSession()
      return
    }
    const host = options.hosts.value.find((item) => item.id === notice.hostId)
    if (!host) {
      toast.error(t('shell.error.targetHostMissing'))
      return
    }
    await options.openSession(host)
  }

  async function reclaimHandoverSession(notice: HandoverNotice): Promise<void> {
    const adopted = await options.adoptSession(notice.sessionId, notice.title)
    if (adopted) return
    toast.warning(t('shell.app.handoverReclaim'))
    await reconnectHandoverSession(notice)
  }

  async function returnActiveSessionToTabs(): Promise<void> {
    const session = options.activeSession.value
    if (!session || returningToTabs.value) return
    returningToTabs.value = true
    try {
      await options.sessionGateway.prepareHandover(session.id).catch(() => undefined)
      const outcome = await requestSessionAdoption(session.id, session.title)
      if (!outcome.ok) {
        returningToTabs.value = false
        toast.warning(
          outcome.reason === 'no-listener' ? t('shell.app.returnToTabsNoWindow') : t('shell.app.returnToTabsFailed'),
        )
        return
      }

      await closeCurrentWindow()
      returningToTabs.value = false
    } catch (error) {
      returningToTabs.value = false
      toast.error(hostErrorMessage(error, t('shell.app.returnToTabsFailed')))
    }
  }

  async function handleSessionAdoptRequest(request: SessionAdoptRequest): Promise<boolean> {
    if (request.kind === 'rdp') {
      return (await options.adoptRdpSession?.(request)) ?? false
    }

    if (request.kind === 'vnc') {
      return (await options.adoptVncTab?.(request)) ?? false
    }
    try {
      await options.sessionGateway.get(request.sessionId)
    } catch {
      return false
    }
    if (options.sessions.value.some((session) => session.id === request.sessionId)) {
      options.activeSessionId.value = request.sessionId
      toast.success(t('shell.app.returnToTabsDone'), { id: `session-returned-${request.sessionId}` })
      return true
    }
    const adopted = await options.adoptSession(request.sessionId, request.title)
    if (!adopted) return false

    handoverWatcher.forget(request.sessionId)
    toast.success(t('shell.app.returnToTabsDone'), { id: `session-returned-${request.sessionId}` })
    return true
  }

  async function openSessionInNewWindow(sessionId: string): Promise<void> {
    const session = options.sessions.value.find((item) => item.id === sessionId)
    if (!session) return

    const epochAtDetach = session.attachEpoch ?? 0
    try {
      const windowTitle = session.title

      await options.sessionGateway.prepareHandover(session.id).catch(() => undefined)
      await openSessionWindow(session.kind, session.hostId, options.sessionGateway.persistent, session.id, windowTitle)
      options.clearSessionToolState(sessionId)
      options.detachSession(sessionId)
      handoverWatcher.watch({
        sessionId: session.id,
        hostId: session.hostId,
        kind: session.kind,
        title: session.title,
        epochAtDetach,
      })
    } catch (error) {
      toast.error(hostErrorMessage(error, t('shell.app.openWindowFailed')))
    }
  }

  function start(): void {
    if (!options.isSessionWindow && options.sessionGateway.persistent) {
      stopAdoptionRequests = listenSessionAdoptionRequests(handleSessionAdoptRequest)
    }
  }

  function dispose(): void {
    handoverWatcher.dispose()
  }

  function stopAdoptionListener(): void {
    stopAdoptionRequests?.()
    stopAdoptionRequests = undefined
  }

  return {
    returningToTabs,
    returnActiveSessionToTabs,
    openSessionInNewWindow,
    start,
    dispose,
    stopAdoptionListener,
  }
}
