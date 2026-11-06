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

  dismissedVersion: string
}

const DISMISSED_STORAGE_KEY = 'qterm.update.dismissed.v1'

function readDismissedVersion(): string {
  try {
    return window.localStorage.getItem(DISMISSED_STORAGE_KEY) ?? ''
  } catch {
    return ''
  }
}

const state = reactive<UpdateViewState>({
  status: normalizeStatus({ state: 'idle', currentVersion: QTERM_VERSION }),
  busy: false,
  errorKey: '',
  errorDetail: '',
  dismissedVersion: readDismissedVersion(),
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

export function ensureUpdateChecks(): void {
  if (started) return
  started = true
  const start = (): void => {
    stopStatus ??= updates.subscribe(applyStatus)
    void (async () => {
      try {
        applyStatus(await updates.status())
        if (state.status.state !== 'idle' && state.status.state !== 'failed') return
        await checkUpdate({ autoDownload: true })
      } catch (cause) {
        recordError(cause)
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
    if (options.autoDownload && release.hasUpdate) await downloadUpdate()
  } catch (cause) {
    recordError(cause)

    state.status = { ...state.status, state: 'failed', percent: 0, downloaded: 0 }
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
  const version = status.release?.version ?? ''
  return version !== '' && version !== state.dismissedVersion
}

export function dismissUpdate(): void {
  const version = state.status.release?.version ?? ''
  state.dismissedVersion = version
  try {
    window.localStorage.setItem(DISMISSED_STORAGE_KEY, version)
  } catch {}
}

export function stopUpdateChecks(): void {
  stopStatus?.()
  stopStatus = undefined
  started = false
}
