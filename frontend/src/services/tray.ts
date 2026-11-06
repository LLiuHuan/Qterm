import { Events } from '@wailsio/runtime'
import { hasWailsRuntime } from './runtime'

export const TRAY_ACTION_EVENT = 'qterm:tray-action'

export type TrayAction = 'newLocalTerminal' | 'openSettings'

export interface TrayMenuState {
  ready: boolean
  locked: boolean
  newLocalLabel: string
  showLabel: string
  settingsLabel: string
  quitLabel: string
}

export function subscribeTrayActions(handler: (action: TrayAction) => void): () => void {
  try {
    return Events.On(TRAY_ACTION_EVENT, (event: { data?: unknown }) => {
      if (event?.data === 'newLocalTerminal' || event?.data === 'openSettings') {
        handler(event.data)
      }
    })
  } catch {
    return () => {}
  }
}

export function setTrayMenuState(state: TrayMenuState): Promise<void> {
  if (typeof window === 'undefined' || !hasWailsRuntime()) return Promise.resolve()
  const snapshot = { ...state }
  const update = trayMenuStateUpdateQueue.then(async () => {
    const { SetTrayMenuState } = await import('../../bindings/changeme/internal/app/service')
    await SetTrayMenuState(snapshot)
  })
  trayMenuStateUpdateQueue = update.catch(() => undefined)
  return update
}

let trayMenuStateUpdateQueue: Promise<void> = Promise.resolve()
let closeToTrayUpdateQueue: Promise<void> = Promise.resolve()

export function setCloseToTray(enabled: boolean): Promise<void> {
  if (typeof window === 'undefined' || !hasWailsRuntime()) return Promise.resolve()

  const update = closeToTrayUpdateQueue.then(async () => {
    const { SetCloseToTray } = await import('../../bindings/changeme/internal/app/service')
    await SetCloseToTray(enabled)
  })
  closeToTrayUpdateQueue = update.catch(() => undefined)
  return update
}
