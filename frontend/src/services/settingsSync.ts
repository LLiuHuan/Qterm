import { Events } from '@wailsio/runtime'

export const SETTINGS_CHANGED_EVENT = 'qterm:settings-changed'

export interface SettingsChangedPayload {
  key: string

  source: number
}

export function connectSettingsSync(onChanged: (payload: SettingsChangedPayload) => void): { close(): void } {
  let unsubscribe: (() => void) | undefined
  try {
    unsubscribe = Events.On(SETTINGS_CHANGED_EVENT, (event) => {
      const data = event.data as { key?: unknown; source?: unknown } | null

      const key = typeof data?.key === 'string' ? data.key : ''
      const source = typeof data?.source === 'number' && Number.isFinite(data.source) ? data.source : 0
      onChanged({ key, source })
    })
  } catch {
    return { close: () => {} }
  }
  return { close: () => unsubscribe?.() }
}
