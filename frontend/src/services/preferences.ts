export const PREFERENCE_KEYS = {
  settings: 'qterm.settings.v1',
  shortcuts: 'qterm.shortcuts.v1',
  terminalThemes: 'qterm.terminal-themes.v1',
  sftpFollow: 'qterm.sftp.follow.v1',
} as const

type PreferenceBinding = {
  GetPreference: (key: string) => Promise<string>
  SetPreference: (key: string, value: string) => Promise<void>
}

type WriteState = {
  storage: Storage | null
  pending?: string
  timer?: ReturnType<typeof setTimeout>
  running?: Promise<void>
  waiters: Array<{ resolve: () => void; reject: (reason: unknown) => void }>
  retryDelay: number
}

const cacheByStorage = new WeakMap<object, Map<string, string>>()
const fallbackCache = new Map<string, string>()
const writes = new Map<string, WriteState>()
const PENDING_STORAGE_KEY = 'qterm.preferences.pending.v1'
const WRITE_DEBOUNCE_MS = 60
const INITIAL_RETRY_MS = 500
const MAX_RETRY_MS = 30_000

function defaultStorage(): Storage | null {
  try {
    return globalThis.localStorage ?? null
  } catch {
    return null
  }
}

function nativeRuntimeReady(): boolean {
  if (typeof window === 'undefined') return false
  if (window.location.protocol === 'wails:') return true
  return Boolean((window as Window & { _wails?: { environment?: { OS?: string } } })._wails?.environment?.OS)
}

function cacheFor(storage: Storage | null): Map<string, string> {
  if (!storage) return fallbackCache
  let cache = cacheByStorage.get(storage)
  if (!cache) {
    cache = new Map<string, string>()
    cacheByStorage.set(storage, cache)
  }
  return cache
}

function writeStorage(storage: Storage | null, key: string, value: string): void {
  cacheFor(storage).set(key, value)
  try {
    storage?.setItem(key, value)
  } catch {}
}

function readPending(storage: Storage | null): Record<string, string> {
  if (!storage) return {}
  try {
    const raw = storage.getItem(PENDING_STORAGE_KEY)
    if (!raw) return {}
    const value = JSON.parse(raw) as unknown
    if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
    return Object.fromEntries(Object.entries(value).filter(([, item]) => typeof item === 'string'))
  } catch {
    return {}
  }
}

function writePending(storage: Storage | null, key: string, value?: string): void {
  if (!storage) return
  const pending = readPending(storage)
  if (value === undefined) delete pending[key]
  else pending[key] = value
  try {
    if (Object.keys(pending).length === 0) storage.removeItem(PENDING_STORAGE_KEY)
    else storage.setItem(PENDING_STORAGE_KEY, JSON.stringify(pending))
  } catch {}
}

export function readCachedPreference(key: string, storage: Storage | null = defaultStorage()): string {
  if (!storage) return cacheFor(storage).get(key) ?? ''
  try {
    return storage.getItem(key) ?? ''
  } catch {
    return cacheFor(storage).get(key) ?? ''
  }
}

export function cachePreference(key: string, value: string, storage: Storage | null = defaultStorage()): void {
  writeStorage(storage, key, value)
}

async function bindings(): Promise<PreferenceBinding> {
  const module = (await import('../../bindings/changeme/internal/app/service')) as unknown as PreferenceBinding
  return module
}

function writeState(key: string): WriteState {
  let state = writes.get(key)
  if (!state) {
    state = { storage: null, waiters: [], retryDelay: INITIAL_RETRY_MS }
    writes.set(key, state)
  }
  return state
}

function notifyWriteFailure(key: string, error: unknown): void {
  if (typeof window === 'undefined') return
  window.dispatchEvent(new CustomEvent('qterm:preference-write-failed', { detail: { key, error } }))
}

function scheduleWrite(key: string, delay: number): void {
  const state = writeState(key)
  if (state.timer) clearTimeout(state.timer)
  state.timer = setTimeout(() => {
    state.timer = undefined
    void drainWrite(key)
  }, delay)
}

async function drainWrite(key: string): Promise<void> {
  const state = writeState(key)
  if (state.running || state.pending === undefined) return
  state.running = (async () => {
    while (state.pending !== undefined) {
      const value = state.pending
      try {
        if (nativeRuntimeReady()) {
          const api = await bindings()
          await api.SetPreference(key, value)
        }
        state.retryDelay = INITIAL_RETRY_MS
        if (state.pending === value) {
          state.pending = undefined
          writePending(state.storage, key, undefined)
          const waiters = state.waiters.splice(0)
          for (const waiter of waiters) waiter.resolve()
        }
      } catch (error) {
        const waiters = state.waiters.splice(0)
        for (const waiter of waiters) waiter.reject(error)
        notifyWriteFailure(key, error)
        scheduleWrite(key, state.retryDelay)
        state.retryDelay = Math.min(MAX_RETRY_MS, state.retryDelay * 2)
        return
      }
    }
  })().finally(() => {
    state.running = undefined
    if (state.pending !== undefined && !state.timer) scheduleWrite(key, WRITE_DEBOUNCE_MS)
  })
  await state.running
}

export async function getPreference(key: string, storage: Storage | null = defaultStorage()): Promise<string> {
  const cached = readCachedPreference(key, storage)
  const pending = readPending(storage)[key]
  if (pending !== undefined) {
    cachePreference(key, pending, storage)
    void setPreference(key, pending, storage).catch(() => undefined)
    return pending
  }
  if (!nativeRuntimeReady()) return cached
  try {
    const api = await bindings()
    const remote = (await api.GetPreference(key)) ?? ''
    if (remote !== '') {
      cachePreference(key, remote, storage)
      return remote
    }
    if (cached !== '') void setPreference(key, cached, storage).catch(() => undefined)
  } catch {}
  return cached
}

export function setPreference(key: string, value: string, storage: Storage | null = defaultStorage()): Promise<void> {
  cachePreference(key, value, storage)
  const state = writeState(key)
  state.storage = storage
  writePending(storage, key, value)
  state.pending = value
  const promise = new Promise<void>((resolve, reject) => state.waiters.push({ resolve, reject }))
  scheduleWrite(key, WRITE_DEBOUNCE_MS)
  return promise
}
