import { Events } from '@wailsio/runtime'
import { normalizeSettings, type QtermSettings } from '../features/settings/settingsStore'

const changedEvent = 'qterm:theme-changed'
const requestEvent = 'qterm:theme-request'
export const themeKeys = [
  'themeMode',
  'minimalAppearance',
  'terminalLightTheme',
  'terminalDarkTheme',
  'terminalThemeBackgroundScope',
  'lightBackgroundEnabled',
  'lightBackgroundOpacity',
  'lightBackgroundImage',
  'lightBackgroundColor',
  'darkBackgroundEnabled',
  'darkBackgroundOpacity',
  'darkBackgroundImage',
  'darkBackgroundColor',
] as const satisfies readonly (keyof QtermSettings)[]

export function themeSnapshot(settings: QtermSettings) {
  return Object.fromEntries(themeKeys.map((key) => [key, settings[key]]))
}

export function connectThemeSync(read: () => QtermSettings, apply: (value: Partial<QtermSettings>) => void) {
  const source = globalThis.crypto?.randomUUID?.() ?? `theme-${Math.random().toString(36).slice(2)}`
  const native = () =>
    window.location.protocol === 'wails:' ||
    Boolean((window as Window & { _wails?: { environment?: { OS?: string } } })._wails?.environment?.OS)
  const publish = (target?: string) => {
    if (native())
      void Events.Emit(changedEvent, { source, target, theme: themeSnapshot(read()) }).catch(() => undefined)
  }
  const stopChanged = Events.On(changedEvent, (event) => {
    const data = event.data as { source?: string; target?: string; theme?: unknown } | null
    if (!data || data.source === source || (data.target && data.target !== source) || !data.theme) return
    apply(themeSnapshot(normalizeSettings(data.theme)))
  })
  const stopRequest = Events.On(requestEvent, (event) => {
    const data = event.data as { source?: string } | null
    if (data?.source && data.source !== source) publish(data.source)
  })
  if (native()) void Events.Emit(requestEvent, { source }).catch(() => undefined)
  return {
    publish: () => publish(),
    close: () => {
      stopChanged()
      stopRequest()
    },
  }
}
