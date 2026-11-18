import { Events } from '@wailsio/runtime'
import { hasWailsRuntime } from './runtime'
import { fromWailsAccelerator, toWailsAccelerator } from './globalShortcutKeyMap'

export const GLOBAL_SHORTCUT_CHANGED_EVENT = 'qterm:global-shortcut-changed'

export const GLOBAL_SHORTCUT_FAILED_EVENT = 'qterm:global-shortcut-failed'

export interface GlobalShortcutFailure {
  binding: string
  reason: 'occupied' | 'invalid'
}

let setQueue: Promise<void> = Promise.resolve()

export function setGlobalShortcut(binding: string): Promise<void> {
  if (typeof window === 'undefined' || !hasWailsRuntime()) return Promise.resolve()
  const update = setQueue.then(async () => {
    const { SetGlobalShortcut } = await import('../../bindings/changeme/internal/app/service')

    const accelerator = binding ? (toWailsAccelerator(binding) ?? '') : ''
    if (binding && !accelerator) {
      throw new Error(`unsupported global shortcut: ${binding}`)
    }
    await SetGlobalShortcut(accelerator)
  })
  setQueue = update.catch(() => undefined)
  return update
}

export async function readGlobalShortcut(): Promise<string> {
  if (typeof window === 'undefined' || !hasWailsRuntime()) return ''
  try {
    const { GlobalShortcut } = await import('../../bindings/changeme/internal/app/service')
    const accelerator = await GlobalShortcut()
    return fromWailsAccelerator(accelerator ?? '')
  } catch {
    return ''
  }
}

export function subscribeGlobalShortcutChanged(handler: (binding: string) => void): () => void {
  if (typeof window === 'undefined' || !hasWailsRuntime()) return () => {}
  try {
    return Events.On(GLOBAL_SHORTCUT_CHANGED_EVENT, (event: { data?: unknown }) => {
      const data = event?.data as { accelerator?: string } | undefined
      handler(fromWailsAccelerator(data?.accelerator ?? ''))
    })
  } catch {
    return () => {}
  }
}

export function subscribeGlobalShortcutFailed(handler: (failure: GlobalShortcutFailure) => void): () => void {
  if (typeof window === 'undefined' || !hasWailsRuntime()) return () => {}
  try {
    return Events.On(GLOBAL_SHORTCUT_FAILED_EVENT, (event: { data?: unknown }) => {
      const data = event?.data as { accelerator?: string; reason?: string } | undefined
      const reason: GlobalShortcutFailure['reason'] = data?.reason === 'invalid' ? 'invalid' : 'occupied'
      handler({ binding: fromWailsAccelerator(data?.accelerator ?? ''), reason })
    })
  } catch {
    return () => {}
  }
}
