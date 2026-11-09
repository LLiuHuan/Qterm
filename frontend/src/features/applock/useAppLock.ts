import { computed, readonly, ref } from 'vue'
import { createAppLockGateway, type AppLockStatus } from '../../services/appLock'
import { hasWailsRuntime, whenWailsRuntime } from '../../services/runtime'

const EMPTY_STATUS: AppLockStatus = {
  enabled: false,
  hasPassword: false,
  autoLockMinutes: 0,
  hasRecovery: false,
  recoveryQuestion: '',
  touchIdEnabled: false,
  touchIdAvailable: false,
}

const status = ref<AppLockStatus>({ ...EMPTY_STATUS })
const locked = ref(false)
const ready = ref(false)
const gateway = createAppLockGateway()

let idleTimer: number | undefined
let listening = false

const ACTIVITY_EVENTS: Array<keyof WindowEventMap> = [
  'pointerdown',
  'pointermove',
  'keydown',
  'wheel',
  'touchstart',
  'focus',
]

function clearIdleTimer(): void {
  if (idleTimer !== undefined) {
    window.clearTimeout(idleTimer)
    idleTimer = undefined
  }
}

function restartIdleTimer(): void {
  clearIdleTimer()
  const minutes = status.value.autoLockMinutes
  if (!status.value.enabled || minutes <= 0 || locked.value) return
  idleTimer = window.setTimeout(
    () => {
      lock()
    },
    minutes * 60 * 1000,
  )
}

function ensureActivityListener(): void {
  if (listening || typeof window === 'undefined') return
  listening = true
  for (const event of ACTIVITY_EVENTS) {
    window.addEventListener(event, restartIdleTimer, { passive: true })
  }
}

function stopActivityListener(): void {
  if (!listening || typeof window === 'undefined') return
  listening = false
  for (const event of ACTIVITY_EVENTS) {
    window.removeEventListener(event, restartIdleTimer)
  }
  clearIdleTimer()
}

async function refresh(lockOnStart = false): Promise<void> {
  try {
    status.value = await gateway.status()
  } catch {
    status.value = { ...EMPTY_STATUS }
  }
  if (!status.value.enabled) {
    locked.value = false
    stopActivityListener()
  } else {
    ensureActivityListener()
    if (lockOnStart) locked.value = true
    restartIdleTimer()
  }
  ready.value = true
}

function lock(): void {
  if (!status.value.enabled || !status.value.hasPassword) return
  clearIdleTimer()
  locked.value = true
  void gateway.broadcastLocked(true)
}

async function unlock(password: string): Promise<boolean> {
  const ok = await gateway.verify(password)
  if (!ok) return false
  finishUnlock()
  return true
}

function finishUnlock(): void {
  locked.value = false
  restartIdleTimer()
  void gateway.broadcastLocked(false)
}

async function unlockWithTouchId(): Promise<boolean> {
  const ok = await gateway.verifyTouchId()
  if (!ok) return false
  finishUnlock()
  return true
}

async function setTouchIdEnabled(enabled: boolean): Promise<void> {
  await gateway.setTouchIdEnabled(enabled)
  await refresh()
}

async function enable(password: string, autoLockMinutes: number, question = '', answer = ''): Promise<void> {
  await gateway.enable(password, autoLockMinutes, question, answer)
  await refresh()
}

async function resetByRecovery(answer: string, next: string): Promise<void> {
  await gateway.resetByRecovery(answer, next)
  await refresh()
}

async function changePassword(current: string, next: string): Promise<void> {
  await gateway.changePassword(current, next)
  await refresh()
}

async function disable(password: string): Promise<void> {
  await gateway.disable(password)
  await refresh()
}

async function setAutoLockMinutes(minutes: number): Promise<void> {
  await gateway.setAutoLockMinutes(minutes)
  await refresh()
}

let stopBroadcast: (() => void) | undefined

async function attachBroadcast(): Promise<void> {
  if (stopBroadcast) return
  try {
    const { Events } = await import('@wailsio/runtime')
    stopBroadcast = Events.On('qterm:applock-state', (event: { data?: unknown }) => {
      if (typeof event?.data === 'boolean') locked.value = event.data
    })
  } catch {
    stopBroadcast = undefined
  }
}

async function initAppLock(): Promise<void> {
  await refresh(true)
  if (hasWailsRuntime()) {
    await attachBroadcast()
    return
  }
  whenWailsRuntime(() => {
    void (async () => {
      await refresh(true)
      await attachBroadcast()
    })()
  })
}

export function useAppLock() {
  return {
    status: readonly(status),
    locked: readonly(locked),
    ready: readonly(ready),
    enabled: computed(() => status.value.enabled),
    autoLockMinutes: computed(() => status.value.autoLockMinutes),

    touchIdUnlockAvailable: computed(
      () => status.value.enabled && status.value.touchIdEnabled && status.value.touchIdAvailable,
    ),
    initAppLock,
    refresh,
    lock,
    unlock,
    enable,
    resetByRecovery,
    changePassword,
    disable,
    setAutoLockMinutes,
    unlockWithTouchId,
    setTouchIdEnabled,
  }
}
