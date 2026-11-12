export type WebFetchMode = 'system' | 'direct' | 'custom'

export interface WebFetchSettings {
  mode: WebFetchMode
  proxyUrl: string
  allowPrivate: boolean
}

const defaultSettings: WebFetchSettings = { mode: 'system', proxyUrl: '', allowPrivate: false }
let memorySettings: WebFetchSettings = { ...defaultSettings }

function hasWailsRuntime(): boolean {
  return Boolean((window as Window & { _wails?: { environment?: { OS?: string } } })._wails?.environment?.OS)
}

function normalize(settings: Partial<WebFetchSettings> | null | undefined): WebFetchSettings {
  const mode = settings?.mode
  return {
    mode: mode === 'direct' || mode === 'custom' ? mode : 'system',
    proxyUrl: typeof settings?.proxyUrl === 'string' ? settings.proxyUrl.trim() : '',
    allowPrivate: Boolean(settings?.allowPrivate),
  }
}

export async function loadWebFetchSettings(): Promise<WebFetchSettings> {
  if (!hasWailsRuntime()) return { ...memorySettings }
  const { WebFetchSettings: readSettings } = await import('../../bindings/changeme/internal/app/service')
  const settings = (await readSettings()) as Partial<WebFetchSettings> | null
  memorySettings = normalize(settings)
  return { ...memorySettings }
}

export async function saveWebFetchSettings(settings: WebFetchSettings): Promise<void> {
  const next = normalize(settings)
  memorySettings = next
  if (!hasWailsRuntime()) return
  const { SetWebFetchSettings } = await import('../../bindings/changeme/internal/app/service')
  await SetWebFetchSettings(next)
}
