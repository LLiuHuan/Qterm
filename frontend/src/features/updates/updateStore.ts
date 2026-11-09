import { reactive } from 'vue'
import { QTERM_VERSION } from '../../services/appInfo'
import { whenWailsRuntime } from '../../services/runtime'
import { decodeAppError } from '../../services/errors'
import { normalizeStatus, updates, updatesAvailable, type UpdateStatus } from '../../services/updates'

export interface UpdateViewState {
  status: UpdateStatus

  busy: boolean

  errorKey: string
  errorDetail: string
}

const LEGACY_DISMISSED_STORAGE_KEY = 'qterm.update.dismissed.v1'

function dropLegacyDismissed(): void {
  try {
    window.localStorage.removeItem(LEGACY_DISMISSED_STORAGE_KEY)
  } catch {}
}

const state = reactive<UpdateViewState>({
  status: normalizeStatus({ state: 'idle', currentVersion: QTERM_VERSION }),
  busy: false,
  errorKey: '',
  errorDetail: '',
})

let started = false
let stopStatus: (() => void) | undefined

export function useUpdateState(): UpdateViewState {
  return state
}

function applyStatus(next: UpdateStatus): void {
  state.status = next
  if (next.state !== 'failed') {
    state.errorKey = ''
    state.errorDetail = ''
    return
  }
  state.errorKey = next.messageKey ?? ''
  state.errorDetail = next.safeDetail ?? ''
}

function recordError(cause: unknown): void {
  const appError = decodeAppError(cause)
  state.errorKey = appError?.messageKey ?? 'update.unavailable'
  state.errorDetail = appError?.safeDetail ?? ''
}

const RETRY_DELAYS = [8_000, 20_000, 45_000]
let retryAttempt = 0
let retryTimer: number | undefined
let retrying = false

function clearRetry(): void {
  window.clearTimeout(retryTimer)
  retryTimer = undefined
  retrying = false
}

function scheduleRetry(): void {
  if (retrying || retryAttempt >= RETRY_DELAYS.length) return
  retrying = true
  const delay = RETRY_DELAYS[retryAttempt] ?? RETRY_DELAYS[RETRY_DELAYS.length - 1]
  retryAttempt += 1
  window.clearTimeout(retryTimer)
  retryTimer = window.setTimeout(() => {
    retrying = false
    void checkUpdate({ autoDownload: true })
  }, delay)
}

const RECHECK_INTERVAL = 5 * 60_000

const RECHECK_AFTER_FAILURE = 30 * 60_000
let recheckTimer: number | undefined

function clearRecheck(): void {
  window.clearTimeout(recheckTimer)
  recheckTimer = undefined
}

function scheduleRecheck(delay = RECHECK_INTERVAL): void {
  if (!started || recheckTimer !== undefined) return
  recheckTimer = window.setTimeout(() => {
    recheckTimer = undefined
    if (!started) return
    void checkUpdate({ autoDownload: true })
  }, delay)
}

function recheckNow(): void {
  if (!started) return
  if (state.status.state === 'ready') return
  void checkUpdate({ autoDownload: true })
}

function onVisibilityChange(): void {
  if (document.visibilityState === 'visible') recheckNow()
}

let resumeBound = false

function bindResumeChecks(): void {
  if (resumeBound) return
  resumeBound = true
  document.addEventListener('visibilitychange', onVisibilityChange)

  window.addEventListener('online', recheckNow)
}

export function ensureUpdateChecks(): void {
  if (started) return
  started = true
  const start = (): void => {
    stopStatus ??= updates.subscribe(applyStatus)
    bindResumeChecks()
    dropLegacyDismissed()
    void (async () => {
      try {
        applyStatus(await updates.status())
        if (state.status.state !== 'idle' && state.status.state !== 'failed') return
        await checkUpdate({ autoDownload: true })
      } catch (cause) {
        recordError(cause)

        scheduleRetry()
      }
    })()
  }
  if (updatesAvailable()) start()
  else whenWailsRuntime(start, 10_000)
}

export async function checkUpdate(options: { autoDownload?: boolean } = {}): Promise<void> {
  state.busy = true
  state.errorKey = ''
  state.errorDetail = ''
  try {
    const release = await updates.check(QTERM_VERSION)
    const nextState = release.noRelease ? 'no-release' : release.hasUpdate ? 'available' : 'up-to-date'
    applyStatus({ ...state.status, state: nextState, release })

    clearRetry()
    retryAttempt = 0

    if (release.hasUpdate) clearRecheck()
    else scheduleRecheck()
    if (options.autoDownload && release.hasUpdate) await downloadUpdate()
  } catch (cause) {
    recordError(cause)

    state.status = { ...state.status, state: 'failed', percent: 0, downloaded: 0 }

    if (options.autoDownload) {
      scheduleRetry()
      scheduleRecheck(RECHECK_AFTER_FAILURE)
    }
  } finally {
    state.busy = false
  }
}

export async function downloadUpdate(): Promise<void> {
  state.busy = true
  try {
    await updates.download()
  } catch (cause) {
    recordError(cause)
  } finally {
    state.busy = false
  }
}

export async function installUpdate(): Promise<void> {
  state.busy = true
  try {
    await updates.install()
  } catch (cause) {
    recordError(cause)
  } finally {
    state.busy = false
  }
}

export function shouldNotifyUpdate(): boolean {
  const status = state.status
  if (status.state !== 'ready') return false
  return (status.release?.version ?? '') !== ''
}

export function stopUpdateChecks(): void {
  stopStatus?.()
  stopStatus = undefined
  window.clearTimeout(retryTimer)
  retryTimer = undefined
  clearRecheck()
  retrying = false
  retryAttempt = 0
  started = false
  document.removeEventListener('visibilitychange', onVisibilityChange)
  window.removeEventListener('online', recheckNow)
  resumeBound = false
}
