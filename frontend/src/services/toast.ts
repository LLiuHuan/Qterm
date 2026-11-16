import { ref, type Ref } from 'vue'
import { toast as sonnerToast } from 'vue-sonner'
import type { ExternalToast } from 'vue-sonner'

export type ToastOptions = ExternalToast
export type ToastKind = 'success' | 'error' | 'warning' | 'info' | 'loading'
export type ToastEngine = 'sonner' | 'ding'

export type ToastHandle = string | number

const ENGINE_STORAGE_KEY = 'qterm.toastEngine'

const DEFAULT_DURATION = 3600

const DEFAULT_ENGINE: ToastEngine = import.meta.env.MODE === 'test' ? 'sonner' : 'ding'

function readStoredEngine(): ToastEngine {
  try {
    const stored = window.localStorage.getItem(ENGINE_STORAGE_KEY)
    return stored === 'sonner' || stored === 'ding' ? stored : DEFAULT_ENGINE
  } catch {
    return DEFAULT_ENGINE
  }
}

const engine: Ref<ToastEngine> = ref(readStoredEngine())

export function useToastEngine(): Readonly<Ref<ToastEngine>> {
  return engine
}

export function setToastEngine(next: ToastEngine): void {
  engine.value = next
  try {
    window.localStorage.setItem(ENGINE_STORAGE_KEY, next)
  } catch {}
}

type DingModule = typeof import('@yusufceng1z/ding-vue')
type DingToast = DingModule['toast']
type DingOptions = NonNullable<Parameters<DingToast['success']>[1]>

let dingToast: DingToast | null = null
let dingLoad: Promise<void> | null = null

const pendingOps: Array<(ding: DingToast) => void> = []
let localId = 0

function ensureDing(): void {
  if (dingToast || dingLoad) return
  dingLoad = import('@yusufceng1z/ding-vue')
    .then((mod) => {
      dingToast = mod.toast
      for (const op of pendingOps.splice(0)) op(dingToast)
    })
    .catch((error: unknown) => {
      pendingOps.length = 0
      console.error('[toast] ding 引擎加载失败，回退 sonner', error)
      setToastEngine('sonner')
    })
}

function runOnDing(op: (ding: DingToast) => void): void {
  ensureDing()
  if (dingToast) op(dingToast)
  else pendingOps.push(op)
}

function toDingOptions(id: string, kind: ToastKind, options?: ToastOptions): DingOptions {
  const mapped: DingOptions = { id }

  const fallbackDuration = kind === 'loading' ? 0 : DEFAULT_DURATION
  if (!options) {
    mapped.duration = fallbackDuration
    return mapped
  }

  if (typeof options.description === 'string' || typeof options.description === 'number') {
    mapped.description = String(options.description)
  }
  if (options.duration === Infinity) mapped.duration = 0
  else if (typeof options.duration === 'number') mapped.duration = options.duration
  else mapped.duration = fallbackDuration

  if (options.action && !Array.isArray(options.action)) {
    const action = options.action as { label?: unknown; onClick?: (event: MouseEvent) => void }
    mapped.action = {
      label: String(action.label ?? ''),
      onClick: () => {
        action.onClick?.({} as MouseEvent)
      },
    }
  }
  return mapped
}

function showDing(kind: ToastKind, message: string, options?: ToastOptions): ToastHandle {
  const handle = options?.id !== undefined ? String(options.id) : `qterm-toast-${++localId}`
  runOnDing((ding) => {
    const mapped = toDingOptions(handle, kind, options)
    switch (kind) {
      case 'success':
        ding.success(message, mapped)
        break
      case 'error':
        ding.error(message, mapped)
        break
      case 'warning':
        ding.warning(message, mapped)
        break
      case 'info':
        ding.info(message, mapped)
        break
      case 'loading':
        ding.loading(message, mapped)
        break
    }
  })
  return handle
}

function callSonner(kind: ToastKind, message: string, options?: ToastOptions): ToastHandle {
  const emit: (message: string, options?: ToastOptions) => ToastHandle =
    kind === 'success'
      ? sonnerToast.success
      : kind === 'error'
        ? sonnerToast.error
        : kind === 'warning'
          ? sonnerToast.warning
          : kind === 'info'
            ? sonnerToast.info
            : sonnerToast.loading
  return options === undefined ? emit(message) : emit(message, options)
}

function showSonner(kind: ToastKind, message: string, options?: ToastOptions): ToastHandle {
  return callSonner(kind, message, options)
}

function show(kind: ToastKind, message: string, options?: ToastOptions): ToastHandle {
  if (engine.value === 'ding') return showDing(kind, message, options)
  return showSonner(kind, message, options)
}

export function dismissToast(id?: ToastHandle): void {
  if (engine.value !== 'ding') {
    sonnerToast.dismiss(id)
    return
  }
  runOnDing((ding) => {
    if (id === undefined) ding.dismiss()
    else ding.dismiss(String(id), { immediate: true })
  })
}

export const toast = {
  success: (message: string, options?: ToastOptions) => show('success', message, options),
  error: (message: string, options?: ToastOptions) => show('error', message, options),
  warning: (message: string, options?: ToastOptions) => show('warning', message, options),
  info: (message: string, options?: ToastOptions) => show('info', message, options),
  loading: (message: string, options?: ToastOptions) => show('loading', message, options),
  dismiss: dismissToast,
}
