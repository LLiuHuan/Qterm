import { computed, ref, watch, type Ref } from 'vue'
import { forgetAdHocSession, isAdHocHostId } from '../services/adhocSession'
import { decodeAppError, translateErrorKey } from '../services/errors'
import type { HostGateway, HostRecord } from '../services/hosts'
import {
  sessionErrorMessage,
  type LocalShellOptions,
  type SessionAuthenticationChallenge,
  type SessionAuthenticationInput,
  type SessionCloseReason,
  type SessionGateway,
  type SessionOpenResult,
} from '../services/sessions'
import type { HostKeyPending, SessionAuthenticationPending, SessionTab } from '../types/qterm'
import type { SessionKind } from '../services/sessions'

function sessionTabKind(kind?: string | null): SessionKind {
  if (kind === 'local' || kind === 'telnet') return kind
  return 'ssh'
}
import { finishSessionHistory, recordSessionStart, type SessionHistoryRecord } from '../services/history'
import { toast } from 'vue-sonner'
import { useI18n } from '../locales'

export const AUTO_RECONNECT_DELAY_MS = 3000

const CLOSING_RECONCILE_POLL_DELAY_MS = 100

export const AUTO_RECONNECT_MAX_ATTEMPTS = 5

const NO_AUTHENTICATION_CHALLENGE_KEY = 'session.no_authentication_challenge'

const AUTHENTICATION_CHALLENGE_ATTEMPTS = 3
const AUTHENTICATION_CHALLENGE_RETRY_DELAYS_MS = [80, 180]

let paneKeySequence = 0

function nextPaneKey(): string {
  paneKeySequence += 1
  return `pane-${paneKeySequence}`
}

export type SessionOpenOrigin = 'user' | 'restore'

interface SessionStateOptions {
  hostGateway: Pick<HostGateway, 'persistent' | 'list'>
  sessionGateway: SessionGateway
  hosts: Ref<HostRecord[]>
  selectedHostId: Ref<string | null>

  autoReconnect?: Ref<boolean>

  localShellPath?: Ref<string>

  localShellWorkingDirectory?: Ref<string>

  onSessionOpened?: (host: HostRecord, origin: SessionOpenOrigin) => void

  onHostDetached?: (hostId: string) => void
}

export interface AuthenticationRetryState {
  hostId: string
  attempt: number
  error: string
  exhausted: boolean
  retrying: boolean
}

export function useSessionState(options: SessionStateOptions) {
  const { t } = useI18n()
  const { hostGateway, sessionGateway, hosts, selectedHostId, autoReconnect } = options
  const sessions = ref<SessionTab[]>([])
  const activeSessionId = ref<string | null>(null)
  const pendingHostKey = ref<HostKeyPending | null>(null)
  const pendingAuthentication = ref<SessionAuthenticationPending | null>(null)
  const submittingAuthentication = ref(false)
  const confirmingHostKey = ref(false)
  const hostKeyError = ref('')
  const sessionActionError = ref('')

  const authenticationRetry = ref<AuthenticationRetryState | null>(null)

  const manualPasswordSessions = new Set<string>()

  const authAttempts = new Map<string, number>()

  const MAX_AUTH_ATTEMPTS = 3
  const activeSession = computed(() => sessions.value.find((session) => session.id === activeSessionId.value) ?? null)
  const activeHost = computed(() => hosts.value.find((host) => host.id === activeSession.value?.hostId) ?? null)
  let disposed = false
  const sessionPollTimers = new Map<string, number>()

  const pendingSessionHistoryStarts = new Map<string, Promise<void>>()
  const finishedSessionHistory = new Set<string>()
  const finishingSessionHistory = new Map<string, Promise<void>>()

  function startSessionHistory(record: Omit<SessionHistoryRecord, 'status'>): void {
    const operation = recordSessionStart(record)
    pendingSessionHistoryStarts.set(record.id, operation)
    void operation
      .catch(() => undefined)
      .finally(() => {
        if (pendingSessionHistoryStarts.get(record.id) === operation) pendingSessionHistoryStarts.delete(record.id)
      })
      .catch(() => undefined)
  }

  async function finishSessionHistorySafely(
    id: string,
    status: SessionHistoryRecord['status'],
    error?: string,
  ): Promise<void> {
    if (!id || finishedSessionHistory.has(id)) return
    const existing = finishingSessionHistory.get(id)
    if (existing) {
      try {
        await existing
      } catch {}
      return
    }
    const operation = (async () => {
      const pendingStart = pendingSessionHistoryStarts.get(id)
      if (pendingStart) await pendingStart
      await finishSessionHistory(id, status, error)
      finishedSessionHistory.add(id)
    })()
    finishingSessionHistory.set(id, operation)
    try {
      await operation
    } catch {
    } finally {
      if (finishingSessionHistory.get(id) === operation) finishingSessionHistory.delete(id)
    }
  }

  function canReconnectSession(session: SessionTab): boolean {
    return sessionGateway.persistent && session.state !== 'preview' && session.state !== 'closing'
  }

  const canReconnect = computed(() => !!activeSession.value && canReconnectSession(activeSession.value))

  const canDisconnect = computed(
    () =>
      sessionGateway.persistent &&
      !!activeSession.value &&
      !['preview', 'closing', 'closed', 'failed'].includes(activeSession.value.state),
  )

  const autoReconnectTimers = new Map<SessionTab, number>()

  function autoReconnectEnabled(): boolean {
    return autoReconnect?.value === true
  }

  function recordsSessionHistory(kind: SessionKind): boolean {
    return kind === 'ssh' || kind === 'telnet'
  }

  function cancelAutoReconnect(tab: SessionTab): void {
    const timer = autoReconnectTimers.get(tab)
    if (timer !== undefined) window.clearTimeout(timer)
    autoReconnectTimers.delete(tab)
    tab.autoReconnectPending = false
  }

  function cancelAllAutoReconnects(): void {
    for (const tab of [...autoReconnectTimers.keys()]) cancelAutoReconnect(tab)
  }

  function isUnexpectedSessionEnd(tab: SessionTab): boolean {
    if (tab.kind === 'telnet') return false
    if (tab.autoReconnectSuppressed === true) return false
    if (tab.state !== 'closed' && tab.state !== 'failed') return false
    if (tab.closeReason === 'user_closed' || tab.closeReason === 'orphan_timeout') return false
    if (tab.exitCode === 0) return false
    return true
  }

  function observeSessionEnd(
    tab: SessionTab,
    end: { closeReason?: SessionCloseReason; exitCode?: number | null },
  ): void {
    tab.closeReason = end.closeReason
    tab.exitCode = typeof end.exitCode === 'number' ? end.exitCode : undefined
    if (isUnexpectedSessionEnd(tab)) scheduleAutoReconnect(tab)
    else cancelAutoReconnect(tab)
  }

  function scheduleAutoReconnect(tab: SessionTab): void {
    if (disposed || !autoReconnectEnabled() || !sessionGateway.persistent) return
    if (autoReconnectTimers.has(tab)) return
    const failures = tab.autoReconnectFailures ?? 0
    if (failures >= AUTO_RECONNECT_MAX_ATTEMPTS) {
      tab.autoReconnectPending = false
      if (!tab.autoReconnectGivenUp) {
        tab.autoReconnectGivenUp = true
        toast.error(t('shell.terminal.autoReconnectStopped', { count: AUTO_RECONNECT_MAX_ATTEMPTS }), {
          id: `auto-reconnect-stopped-${tab.id}`,
        })
      }
      return
    }
    tab.autoReconnectPending = true
    const timer = window.setTimeout(() => {
      autoReconnectTimers.delete(tab)
      void runAutoReconnect(tab)
    }, AUTO_RECONNECT_DELAY_MS)
    autoReconnectTimers.set(tab, timer)
  }

  async function runAutoReconnect(tab: SessionTab): Promise<void> {
    if (disposed || !autoReconnectEnabled() || !sessions.value.includes(tab)) return
    if (!isUnexpectedSessionEnd(tab)) {
      cancelAutoReconnect(tab)
      return
    }
    tab.autoReconnectFailures = (tab.autoReconnectFailures ?? 0) + 1
    await reconnectSession(tab.id)
    if (disposed || !sessions.value.includes(tab)) return

    if (isUnexpectedSessionEnd(tab)) scheduleAutoReconnect(tab)
    else cancelAutoReconnect(tab)
  }

  function resetAutoReconnectFailures(tab: SessionTab): void {
    tab.autoReconnectFailures = 0
    tab.autoReconnectGivenUp = false
  }

  function uniqueSessionTitle(base: string, excludeId = ''): string {
    let title = base
    let suffix = 1

    while (sessions.value.some((session) => session.id !== excludeId && session.title === title))
      title = `${base}(${suffix++})`
    return title
  }

  function renameSession(sessionID: string, title: string): void {
    const target = sessions.value.find((session) => session.id === sessionID)
    const next = (title ?? '').trim().slice(0, 64)
    if (!target || !next) return
    target.title = uniqueSessionTitle(next, sessionID)
  }

  function attachOpenedSession(opened: SessionOpenResult, title: string, hostId = ''): void {
    const session: SessionTab = {
      id: opened.session.id,

      paneKey: nextPaneKey(),
      hostId: hostId || opened.session.hostId || '',
      kind: sessionTabKind(opened.session.kind),
      title: uniqueSessionTitle(title),
      state: sessionGateway.persistent ? opened.session.state : 'preview',
      streamToken: opened.streamToken,
      streamState: 'idle',
      error: '',
      attachEpoch: opened.session.attachEpoch ?? 0,
    }
    sessions.value.push(session)
    activeSessionId.value = session.id
    if (sessionGateway.persistent) scheduleSessionPoll(session.id, 0)
  }

  async function adoptSession(sessionID: string, title?: string): Promise<boolean> {
    if (disposed) return false
    sessionActionError.value = ''
    try {
      const opened = await sessionGateway.rebind(sessionID)
      if (disposed) return false
      const host = hosts.value.find((item) => item.id === opened.session.hostId)
      const fallbackTitle =
        opened.session.kind === 'local' ? t('shell.session.localTitle') : host?.name || t('shell.session.sshTitle')
      attachOpenedSession(opened, title || fallbackTitle, opened.session.hostId || '')
      return true
    } catch (error) {
      sessionActionError.value = sessionErrorMessage(error, t('shell.error.adoptSession'))
      return false
    }
  }

  async function openSession(host: HostRecord, origin: SessionOpenOrigin = 'user'): Promise<SessionTab | null> {
    if (disposed) return null
    selectedHostId.value = host.id
    sessionActionError.value = ''
    try {
      const opened = await sessionGateway.open(host.id)
      if (disposed) {
        if (sessionGateway.persistent) await sessionGateway.close(opened.session.id)
        return null
      }
      return openSessionWithResult(host, opened, origin)
    } catch (error) {
      sessionActionError.value = sessionErrorMessage(error, t('shell.error.openSession'))
      return null
    }
  }

  async function openSessionWithResult(
    host: HostRecord,
    opened: SessionOpenResult,
    origin: SessionOpenOrigin = 'user',
  ): Promise<SessionTab | null> {
    if (disposed) return null
    selectedHostId.value = host.id
    sessionActionError.value = ''
    try {
      const session: SessionTab = {
        id: opened.session.id,

        paneKey: nextPaneKey(),
        hostId: host.id,

        address: host.address,

        kind: host.protocol === 'telnet' ? 'telnet' : 'ssh',
        title: uniqueSessionTitle(host.name),
        state: sessionGateway.persistent ? opened.session.state : 'preview',
        streamToken: opened.streamToken,
        streamState: 'idle',
        error: '',
        attachEpoch: opened.session.attachEpoch ?? 0,
      }
      sessions.value.push(session)
      activeSessionId.value = session.id
      options.onSessionOpened?.(host, origin)
      startSessionHistory({
        id: session.id,
        hostId: host.id,
        hostName: host.name,
        address: host.address,
        startedAt: new Date().toISOString(),
      })
      if (sessionGateway.persistent) {
        scheduleSessionPoll(session.id, 0)
      }
      return session
    } catch (error) {
      sessionActionError.value = sessionErrorMessage(error, t('shell.error.openSession'))
      return null
    }
  }

  function localShellOptions(): LocalShellOptions {
    return {
      shellPath: options.localShellPath?.value ?? '',
      workingDirectory: options.localShellWorkingDirectory?.value ?? '',
    }
  }

  function notifyLocalShellWarnings(warnings: string[] | undefined): void {
    const messages = (warnings ?? []).map((key) => translateErrorKey(key, '')).filter((text) => text.length > 0)
    if (messages.length === 0) return
    toast.warning(messages.join(' '))
  }

  async function openLocalSession(): Promise<SessionTab | null> {
    if (disposed) return null
    sessionActionError.value = ''
    try {
      const opened = await sessionGateway.openLocal(localShellOptions())
      if (disposed) {
        if (sessionGateway.persistent) await sessionGateway.close(opened.session.id)
        return null
      }
      const session: SessionTab = {
        id: opened.session.id,

        paneKey: nextPaneKey(),
        hostId: opened.session.hostId ?? '',
        kind: 'local',
        title: uniqueSessionTitle(t('shell.session.localTitle')),
        state: sessionGateway.persistent ? opened.session.state : 'preview',
        streamToken: opened.streamToken,
        streamState: 'idle',
        error: '',
        attachEpoch: opened.session.attachEpoch ?? 0,
      }
      sessions.value.push(session)
      activeSessionId.value = session.id

      notifyLocalShellWarnings(opened.warnings)
      if (sessionGateway.persistent) scheduleSessionPoll(session.id, 0)
      return session
    } catch (error) {
      sessionActionError.value = sessionErrorMessage(error, t('shell.error.openLocal'))
      return null
    }
  }

  function renameHostSessions(hostId: string, hostName: string): void {
    if (!hostName.trim()) return
    const usedTitles = new Set(
      sessions.value.filter((session) => session.hostId !== hostId).map((session) => session.title),
    )
    for (const session of sessions.value.filter((item) => item.hostId === hostId)) {
      let title = hostName
      let suffix = 1
      while (usedTitles.has(title)) title = `${hostName}(${suffix++})`
      session.title = title
      usedTitles.add(title)
    }
  }

  function hostHasLiveSession(hostId: string): boolean {
    if (!hostId) return false
    return sessions.value.some(
      (session) => session.hostId === hostId && session.state !== 'closed' && session.state !== 'failed',
    )
  }

  function notifyHostDetached(hostId: string): void {
    if (!hostId || hostHasLiveSession(hostId)) return
    options.onHostDetached?.(hostId)
  }

  async function closeHostSessions(hostId: string): Promise<void> {
    const related = sessions.value.filter((session) => session.hostId === hostId)
    await Promise.all(related.map((session) => closeSession(session.id)))
  }

  async function closeSession(sessionID: string): Promise<void> {
    const index = sessions.value.findIndex((session) => session.id === sessionID)
    if (index < 0) return
    const tab = sessions.value[index]
    const previousState = tab.state
    tab.state = 'closing'

    cancelAutoReconnect(tab)
    stopSessionPolling(sessionID)
    sessionActionError.value = ''
    if (sessionGateway.persistent) {
      try {
        await sessionGateway.close(sessionID)
        if (recordsSessionHistory(tab.kind)) void finishSessionHistorySafely(sessionID, 'closed')

        if (isAdHocHostId(tab.hostId)) void forgetAdHocSession(tab.hostId)
      } catch (error) {
        if (isCurrentSessionTab(tab, sessionID)) {
          tab.state = previousState
          scheduleSessionPoll(sessionID)
        }
        sessionActionError.value = sessionErrorMessage(error, t('shell.error.closeSession'))
        return
      }
    }
    const currentIndex = sessions.value.findIndex((session) => session.id === sessionID)
    if (currentIndex < 0) return
    sessions.value.splice(currentIndex, 1)

    notifyHostDetached(tab.hostId)

    if (authenticationRetry.value?.hostId === tab.hostId) {
      authenticationRetry.value = null
      authAttempts.delete(tab.hostId)
    }
    manualPasswordSessions.delete(sessionID)
    if (pendingHostKey.value?.sessionId === sessionID) pendingHostKey.value = null
    if (pendingAuthentication.value?.sessionId === sessionID) pendingAuthentication.value = null
    if (activeSessionId.value !== sessionID) return
    activeSessionId.value = sessions.value[Math.min(currentIndex, sessions.value.length - 1)]?.id ?? null
  }

  function detachSession(sessionID: string): void {
    const index = sessions.value.findIndex((session) => session.id === sessionID)
    if (index < 0) return

    cancelAutoReconnect(sessions.value[index])
    stopSessionPolling(sessionID)
    sessions.value.splice(index, 1)
    if (pendingHostKey.value?.sessionId === sessionID) pendingHostKey.value = null
    if (pendingAuthentication.value?.sessionId === sessionID) pendingAuthentication.value = null
    if (activeSessionId.value === sessionID) {
      activeSessionId.value = sessions.value[Math.min(index, sessions.value.length - 1)]?.id ?? null
    }
  }

  function reorderSessions(sourceID: string, targetID: string, position: 'before' | 'after' = 'before'): void {
    if (sourceID === targetID) return
    const sourceIndex = sessions.value.findIndex((session) => session.id === sourceID)
    const targetIndex = sessions.value.findIndex((session) => session.id === targetID)
    if (sourceIndex < 0 || targetIndex < 0) return
    const [source] = sessions.value.splice(sourceIndex, 1)
    if (!source) return

    const beforeIndex = sourceIndex < targetIndex ? targetIndex - 1 : targetIndex
    const insertIndex = position === 'after' ? beforeIndex + 1 : beforeIndex
    sessions.value.splice(Math.max(0, Math.min(insertIndex, sessions.value.length)), 0, source)
  }

  async function disconnectActiveSession(): Promise<void> {
    const session = activeSession.value
    if (!session) return

    cancelAutoReconnect(session)

    session.autoReconnectSuppressed = true
    if (!canDisconnect.value) return
    stopSessionPolling(session.id)
    session.state = 'closing'
    sessionActionError.value = ''
    try {
      await sessionGateway.close(session.id)
      if (recordsSessionHistory(session.kind)) await finishSessionHistorySafely(session.id, 'closed')
      session.state = 'closed'
      session.streamState = 'closed'
      session.streamToken = ''

      notifyHostDetached(session.hostId)
    } catch (error) {
      session.state = 'failed'
      session.streamState = 'failed'
      session.error = sessionErrorMessage(error, t('shell.error.disconnect'))
      sessionActionError.value = session.error
    }
  }

  async function reconnectSession(sessionID: string): Promise<void> {
    const session = sessions.value.find((item) => item.id === sessionID)
    if (!session) return
    const host = hosts.value.find((item) => item.id === session.hostId)

    const adHoc = isAdHocHostId(session.hostId)
    if (recordsSessionHistory(session.kind) && !host && !adHoc) return
    if (!canReconnectSession(session)) return
    const previousID = session.id
    const previousState = session.state

    cancelAutoReconnect(session)

    session.autoReconnectSuppressed = false
    session.closeReason = undefined
    session.exitCode = undefined
    stopSessionPolling(previousID)
    session.reconnectInFlight = true
    session.state = 'closing'
    session.streamState = 'closed'
    sessionActionError.value = ''
    try {
      await sessionGateway.close(previousID)
      if (recordsSessionHistory(session.kind)) {
        await finishSessionHistorySafely(
          previousID,
          previousState === 'failed' ? 'failed' : 'closed',
          previousState === 'failed' ? session.error : undefined,
        )
      }
      if (!isCurrentSessionTab(session, previousID)) return
      const opened =
        session.kind === 'local'
          ? await sessionGateway.openLocal(localShellOptions())
          : await sessionGateway.open(host?.id ?? session.hostId)
      if (!isCurrentSessionTab(session, previousID)) {
        await sessionGateway.close(opened.session.id)
        return
      }

      session.id = opened.session.id
      session.state = opened.session.state
      session.streamToken = opened.streamToken
      session.streamState = 'idle'
      session.latencyMs = null
      session.error = ''

      session.attachEpoch = opened.session.attachEpoch ?? 0

      if (activeSessionId.value === previousID) activeSessionId.value = session.id
      if (recordsSessionHistory(session.kind)) {
        startSessionHistory({
          id: session.id,
          hostId: host?.id ?? session.hostId,

          hostName: host?.name ?? session.title,
          address: host?.address ?? session.address ?? '',
          startedAt: new Date().toISOString(),
        })
      }
      scheduleSessionPoll(session.id, 0)

      if (host) options.onSessionOpened?.(host, 'user')

      notifyLocalShellWarnings(opened.warnings)
    } catch (error) {
      session.state = 'failed'
      session.streamState = 'failed'
      session.error = sessionErrorMessage(error, t('shell.error.reconnect'))
      sessionActionError.value = session.error

      if (authenticationRetry.value?.hostId === session.hostId) authenticationRetry.value = null
    } finally {
      session.reconnectInFlight = false
    }
  }

  async function reconnectActiveSession(): Promise<void> {
    const session = activeSession.value
    if (!session) return
    await reconnectSession(session.id)
  }

  function scheduleSessionPoll(sessionID: string, delay = 250): void {
    if (disposed) return
    stopSessionPolling(sessionID)
    sessionPollTimers.set(
      sessionID,
      window.setTimeout(() => void pollSession(sessionID), delay),
    )
  }

  function stopSessionPolling(sessionID: string): void {
    const timer = sessionPollTimers.get(sessionID)
    if (timer !== undefined) window.clearTimeout(timer)
    sessionPollTimers.delete(sessionID)
  }

  async function resolveAuthenticationChallenge(
    sessionID: string,
    tab: SessionTab,
  ): Promise<SessionAuthenticationChallenge | null> {
    let lastError: unknown
    for (let attempt = 0; attempt < AUTHENTICATION_CHALLENGE_ATTEMPTS; attempt += 1) {
      if (!isCurrentSessionTab(tab, sessionID)) return null
      try {
        return await sessionGateway.getAuthenticationChallenge(sessionID)
      } catch (error) {
        if (decodeAppError(error)?.messageKey === NO_AUTHENTICATION_CHALLENGE_KEY) return null
        lastError = error
      }
      const delay = AUTHENTICATION_CHALLENGE_RETRY_DELAYS_MS[attempt]
      if (delay !== undefined) await new Promise<void>((resolve) => window.setTimeout(resolve, delay))
    }
    throw lastError
  }

  async function pollSession(sessionID: string): Promise<void> {
    sessionPollTimers.delete(sessionID)
    const tab = sessions.value.find((session) => session.id === sessionID)
    if (!tab || !sessionGateway.persistent) return
    try {
      const snapshot = await sessionGateway.get(sessionID)

      if (tab.autoReconnectSuppressed === true) return

      const terminalTabState = ['closed', 'failed'].includes(tab.state)
      if (!isCurrentSessionTab(tab, sessionID)) return
      if (terminalTabState && snapshot.state !== 'hostKeyPending') return
      tab.state = snapshot.state
      tab.latencyMs = snapshot.latencyMs ?? null

      tab.attachEpoch = snapshot.attachEpoch ?? tab.attachEpoch ?? 0
      if (snapshot.state === 'connected') {
        if (pendingAuthentication.value?.sessionId === sessionID) pendingAuthentication.value = null

        manualPasswordSessions.delete(sessionID)
        authAttempts.delete(tab.hostId)
        if (authenticationRetry.value?.hostId === tab.hostId) authenticationRetry.value = null
        if (tab.streamState === 'idle') {
          tab.streamState = 'opening'

          if (tab.kind === 'ssh') void refreshConnectedHost(tab.hostId)
        }
        scheduleSessionPoll(sessionID, 1000)
        return
      }
      if (snapshot.state === 'hostKeyPending') {
        await showHostKeyChallenge(sessionID, tab)
        return
      }

      if (snapshot.state === 'awaitingAuth') {
        try {
          const challenge = await resolveAuthenticationChallenge(sessionID, tab)
          if (challenge && isCurrentSessionTab(tab, sessionID)) {
            pendingAuthentication.value = { sessionId: sessionID, challenge }

            if (challenge.kind === 'keyboard-interactive') manualPasswordSessions.delete(sessionID)
            const retry = authenticationRetry.value
            if (retry) {
              authenticationRetry.value = retry.hostId === challenge.hostId ? { ...retry, retrying: false } : null
            }
          }
        } catch (error) {
          if (isCurrentSessionTab(tab, sessionID)) {
            sessionActionError.value = sessionErrorMessage(error, t('shell.error.authenticationChallenge'))
          }
        }
        scheduleSessionPoll(sessionID, 350)
        return
      }
      if (snapshot.state === 'failed') {
        let failureKey = ''
        try {
          failureKey = await sessionGateway.getFailure(sessionID)
          if (isCurrentSessionTab(tab, sessionID))
            tab.error = translateErrorKey(failureKey, t('shell.error.connectionFailed'))
        } catch {
          if (isCurrentSessionTab(tab, sessionID)) tab.error = t('shell.error.connectionFailed')
        }
        if (
          isCurrentSessionTab(tab, sessionID) &&
          manualPasswordSessions.has(sessionID) &&
          isAuthenticationFailure(failureKey) &&
          tab.hostId
        ) {
          manualPasswordSessions.delete(sessionID)
          const attempt = (authAttempts.get(tab.hostId) ?? 0) + 1
          authAttempts.set(tab.hostId, attempt)

          tab.autoReconnectSuppressed = true
          cancelAutoReconnect(tab)
          if (attempt < MAX_AUTH_ATTEMPTS) {
            const message = t('shell.authDialog.wrongPassword', { attempt, max: MAX_AUTH_ATTEMPTS })
            authenticationRetry.value = {
              hostId: tab.hostId,
              attempt,
              error: message,
              exhausted: false,
              retrying: true,
            }
            tab.error = message
            sessionActionError.value = message
            toast.error(message)

            void reconnectSession(sessionID)
            return
          }
          const ended = t('shell.authDialog.tooManyAttempts', { attempts: MAX_AUTH_ATTEMPTS })
          authenticationRetry.value = { hostId: tab.hostId, attempt, error: ended, exhausted: true, retrying: false }
          tab.error = `${ended} ${t('shell.session.authRetryHint')}`
          sessionActionError.value = tab.error
          toast.error(ended)
        } else if (authenticationRetry.value?.hostId === tab.hostId && authenticationRetry.value.retrying) {
          authenticationRetry.value = null
        }
        if (isAuthenticationFailure(failureKey) && isCurrentSessionTab(tab, sessionID)) {
          if (authenticationRetry.value?.hostId === tab.hostId && !authenticationRetry.value.exhausted) {
            authenticationRetry.value = null
          }

          tab.autoReconnectSuppressed = true
          cancelAutoReconnect(tab)
          tab.error = `${tab.error} ${t('shell.session.authRetryHint')}`
          sessionActionError.value = tab.error
          toast.error(tab.error)
        }
        if (recordsSessionHistory(tab.kind)) void finishSessionHistorySafely(sessionID, 'failed', tab.error)
        if (!isCurrentSessionTab(tab, sessionID)) return

        observeSessionEnd(tab, { closeReason: snapshot.closeReason, exitCode: snapshot.exitCode })
        try {
          await showHostKeyChallenge(sessionID, tab)
        } catch {}
        return
      }
      if (snapshot.state === 'closed') {
        stopSessionPolling(sessionID)
        if (recordsSessionHistory(tab.kind)) void finishSessionHistorySafely(sessionID, 'closed')

        observeSessionEnd(tab, { closeReason: snapshot.closeReason, exitCode: snapshot.exitCode })
        return
      }
      scheduleSessionPoll(sessionID)
    } catch (error) {
      if (!isCurrentSessionTab(tab, sessionID)) return
      tab.state = 'failed'
      tab.error = sessionErrorMessage(error, t('shell.error.sessionState'))
      if (recordsSessionHistory(tab.kind)) void finishSessionHistorySafely(sessionID, 'failed', tab.error)
    }
  }

  const AUTHENTICATION_FAILURE_KEYS = new Set([
    'session.auth_failed',
    'session.jump_auth_failed',
    'session.proxy_auth_failed',
  ])

  function isAuthenticationFailure(key: string): boolean {
    return AUTHENTICATION_FAILURE_KEYS.has(key)
  }

  function isCurrentSessionTab(tab: SessionTab, sessionID: string): boolean {
    return !disposed && tab.id === sessionID && sessions.value.includes(tab)
  }

  async function refreshConnectedHost(hostID: string): Promise<void> {
    if (!hostGateway.persistent) return
    try {
      const refreshed = (await hostGateway.list()).find((host) => host.id === hostID)
      if (!refreshed) return
      const index = hosts.value.findIndex((host) => host.id === hostID)

      if (index >= 0) hosts.value.splice(index, 1, refreshed)
    } catch {}
  }

  async function showHostKeyChallenge(sessionID: string, expectedTab?: SessionTab): Promise<void> {
    let challenge: HostKeyPending['challenge']
    let challenges: HostKeyPending['challenges']
    if (sessionGateway.getHostKeyPending) {
      const pending = await sessionGateway.getHostKeyPending(sessionID)
      challenge = pending.challenge
      challenges = pending.challenges?.length ? pending.challenges : [pending.challenge]
    } else {
      challenge = await sessionGateway.getHostKeyChallenge(sessionID)
      challenges = [challenge]
    }
    if (disposed || (expectedTab && !isCurrentSessionTab(expectedTab, sessionID))) return
    if (!challenge) throw new Error(t('shell.error.invalidHostChallenge'))
    pendingHostKey.value = { sessionId: sessionID, challenge, challenges }
    hostKeyError.value = ''
  }

  async function trustPendingHostKey(): Promise<void> {
    const pending = pendingHostKey.value
    if (!pending || disposed || confirmingHostKey.value) return
    const tab = sessions.value.find((session) => session.id === pending.sessionId)
    if (!tab) return

    confirmingHostKey.value = true
    hostKeyError.value = ''
    try {
      const result = await sessionGateway.trustHostKey(pending.sessionId)
      if (isCurrentSessionTab(tab, pending.sessionId)) {
        tab.state = result.session.state
        tab.streamToken = result.streamToken
        tab.streamState = 'idle'
        tab.error = ''

        tab.attachEpoch = result.session.attachEpoch ?? 0
        scheduleSessionPoll(pending.sessionId, 0)
      }
      if (pendingHostKey.value === pending) pendingHostKey.value = null
    } catch (error) {
      if (!disposed && pendingHostKey.value === pending)
        hostKeyError.value = sessionErrorMessage(error, t('shell.error.saveHostKey'))
    } finally {
      confirmingHostKey.value = false
    }
  }

  async function rejectPendingHostKey(): Promise<void> {
    const pending = pendingHostKey.value
    if (!pending || confirmingHostKey.value) return
    pendingHostKey.value = null
    await closeSession(pending.sessionId)
  }

  async function submitAuthentication(input: SessionAuthenticationInput): Promise<void> {
    const pending = pendingAuthentication.value
    if (!pending || submittingAuthentication.value || disposed) return
    submittingAuthentication.value = true
    sessionActionError.value = ''

    const manualPassword =
      pending.challenge.kind !== 'keyboard-interactive' && !input.credentialId && input.secret.trim() !== ''
    const manualPasswordHostId = pending.challenge.hostId ?? ''
    if (manualPassword && manualPasswordHostId) {
      manualPasswordSessions.add(pending.sessionId)
      const previous = authenticationRetry.value
      const continued = previous && previous.hostId === manualPasswordHostId ? previous : null
      authenticationRetry.value = {
        hostId: manualPasswordHostId,
        attempt: continued ? continued.attempt : (authAttempts.get(manualPasswordHostId) ?? 0),
        error: continued ? continued.error : '',
        exhausted: false,
        retrying: true,
      }
    }
    try {
      await sessionGateway.submitAuthentication(pending.sessionId, input)

      if (
        pendingAuthentication.value?.sessionId === pending.sessionId &&
        pendingAuthentication.value?.challenge.requestId === pending.challenge.requestId
      )
        pendingAuthentication.value = null
      scheduleSessionPoll(pending.sessionId, 0)
    } catch (error) {
      sessionActionError.value = sessionErrorMessage(error, t('shell.error.authenticationFailed'))

      if (manualPassword && authenticationRetry.value?.retrying) {
        authenticationRetry.value = { ...authenticationRetry.value, retrying: false }
      }
    } finally {
      submittingAuthentication.value = false
    }
  }

  async function cancelAuthentication(): Promise<void> {
    const retry = authenticationRetry.value
    authenticationRetry.value = null
    if (retry) authAttempts.delete(retry.hostId)
    const pending = pendingAuthentication.value
    pendingAuthentication.value = null
    const sessionID =
      pending?.sessionId ?? (retry ? sessions.value.find((item) => item.hostId === retry.hostId)?.id : undefined)
    if (sessionID) await closeSession(sessionID)
  }

  async function reconcileTerminalStreamEnd(
    value: { sessionId: string; state: 'closed' | 'failed'; message?: string },
    tab: SessionTab,
  ): Promise<void> {
    if (tab.autoReconnectSuppressed === true) return
    let finalState: 'closed' | 'failed' = value.state

    let end: { closeReason?: SessionCloseReason; exitCode?: number | null } = {}
    if (sessionGateway.persistent) {
      try {
        const snapshot = await sessionGateway.get(value.sessionId)
        if (!isCurrentSessionTab(tab, value.sessionId)) return
        if (snapshot.state !== 'closed' && snapshot.state !== 'failed') {
          if (snapshot.state === 'closing') {
            tab.state = 'closing'
            tab.closeReason = snapshot.closeReason
            tab.exitCode = typeof snapshot.exitCode === 'number' ? snapshot.exitCode : undefined
            cancelAutoReconnect(tab)
            scheduleSessionPoll(value.sessionId, CLOSING_RECONCILE_POLL_DELAY_MS)
            return
          }

          tab.state = snapshot.state
          tab.streamState = 'idle'

          cancelAutoReconnect(tab)
          scheduleSessionPoll(value.sessionId, 0)
          return
        }
        finalState = snapshot.state
        end = { closeReason: snapshot.closeReason, exitCode: snapshot.exitCode }
      } catch {
        if (isCurrentSessionTab(tab, value.sessionId)) scheduleSessionPoll(value.sessionId, 250)
        return
      }
    }
    if (!isCurrentSessionTab(tab, value.sessionId)) return
    tab.state = finalState
    stopSessionPolling(value.sessionId)

    observeSessionEnd(tab, end)
    if (recordsSessionHistory(tab.kind)) {
      await finishSessionHistorySafely(
        value.sessionId,
        finalState === 'failed' ? 'failed' : 'closed',
        value.message || tab.error || undefined,
      )
    }
  }

  function handleTerminalStreamStatus(value: {
    sessionId: string
    state: 'ready' | 'closed' | 'failed'
    message?: string
  }): void {
    const tab = sessions.value.find((session) => session.id === value.sessionId)
    if (!tab) return
    tab.streamState = value.state
    if (value.message) tab.error = value.message
    if (value.state === 'ready') {
      resetAutoReconnectFailures(tab)
      cancelAutoReconnect(tab)
      return
    }
    const endState = value.state
    if (endState === 'closed' || endState === 'failed') {
      void reconcileTerminalStreamEnd({ ...value, state: endState }, tab)
    }
  }

  function close(): void {
    disposed = true
    for (const sessionID of sessionPollTimers.keys()) stopSessionPolling(sessionID)

    cancelAllAutoReconnects()
  }

  if (autoReconnect)
    watch(autoReconnect, (enabled) => {
      if (!enabled) cancelAllAutoReconnects()
    })

  return {
    sessions,
    activeSessionId,
    activeSession,
    activeHost,
    pendingHostKey,
    pendingAuthentication,
    submittingAuthentication,
    confirmingHostKey,
    hostKeyError,
    sessionActionError,
    canReconnect,
    canDisconnect,
    openSession,
    openSessionWithResult,
    adoptSession,
    detachSession,
    openLocalSession,
    closeSession,
    reorderSessions,
    closeHostSessions,
    renameHostSessions,
    renameSession,
    disconnectActiveSession,
    reconnectActiveSession,
    reconnectSession,
    stopSessionPolling,
    showHostKeyChallenge,
    trustPendingHostKey,
    rejectPendingHostKey,
    submitAuthentication,
    authenticationRetry,
    cancelAuthentication,
    handleTerminalStreamStatus,
    close,
  }
}
