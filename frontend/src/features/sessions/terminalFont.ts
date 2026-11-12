const NERD_FONT_FAMILIES = [
  'Maple Mono NF CN',
  'Maple Mono NF',
  'MesloLGS NF',
  'JetBrainsMono Nerd Font Mono',
  'JetBrainsMono Nerd Font',
  'Hack Nerd Font Mono',
  'Hack Nerd Font',
  'FiraCode Nerd Font Mono',
  'FiraCode Nerd Font',
  'CaskaydiaCove Nerd Font Mono',
  'CaskaydiaCove Nerd Font',
  'SauceCodePro Nerd Font Mono',
  'SauceCodePro Nerd Font',
]

const SYMBOL_FONT_FAMILIES = ['Symbols Nerd Font Mono', 'Symbols Nerd Font']

const SYSTEM_MONO_FAMILIES = ['SFMono-Regular', 'Menlo', 'Consolas', 'Liberation Mono']

export const TERMINAL_FONT_OPTIONS: Array<{ label: string; value: string; hint: string }> = [
  {
    label: '默认',
    value: '',
    hint: '自动选择本机已安装的 Nerd Font，缺失字形逐个回退',
  },
  ...NERD_FONT_FAMILIES.map((family) => ({
    label: family,
    value: family,
    hint: 'Nerd Font 家族，支持 lsd / eza 图标',
  })),
  ...SYSTEM_MONO_FAMILIES.map((family) => ({
    label: family,
    value: family,
    hint: '系统等宽字体，不包含 Nerd Font 图标字形',
  })),
]

export function terminalFontStack(primary?: string): string[] {
  const selected = (primary ?? '').trim()
  return [
    ...(selected ? [selected] : []),
    ...NERD_FONT_FAMILIES.filter((family) => family !== selected),
    ...SYMBOL_FONT_FAMILIES,
    ...SYSTEM_MONO_FAMILIES,
    'monospace',
  ]
}

export function buildTerminalFontFamily(primary?: string): string {
  const selected = (primary ?? '').trim()

  const family = selected
    ? `'${selected.replaceAll('\\', '\\\\').replaceAll("'", "\\'")}'`
    : terminalFontStack()
        .map((item) => (item.includes(' ') ? `'${item}'` : item))
        .join(', ')
  return family
}

const terminalFontLoadCache = new WeakMap<FontFaceSet, Map<string, Promise<boolean>>>()
const TERMINAL_FONT_LOAD_TIMEOUT_MS = 2000

const TERMINAL_FONT_SELECTED_TIMEOUT_MS = 3000

const TERMINAL_FONT_PROBE_GLYPHS = 'MW09'

function cssFontFamilyLiteral(family: string): string {
  return `"${family.replaceAll('\\', '\\\\').replaceAll('"', '\\"')}"`
}

export async function ensureTerminalFontReady(size: number, primary?: string, weight = 400): Promise<boolean> {
  if (typeof document === 'undefined' || !document.fonts) return false
  const fonts = document.fonts
  let cache = terminalFontLoadCache.get(fonts)
  if (!cache) {
    cache = new Map()
    terminalFontLoadCache.set(fonts, cache)
  }
  const selected = primary?.trim() ?? ''
  const family = buildTerminalFontFamily(selected)
  const descriptor = selected
    ? `${weight} ${size}px ${cssFontFamilyLiteral(selected)}`
    : `${weight} ${size}px ${family}`
  const key = `${weight}:${size}:${selected || family}`
  let loading = cache.get(key)
  if (!loading) {
    loading = Promise.resolve()
      .then(() => fonts.load(descriptor, TERMINAL_FONT_PROBE_GLYPHS))
      .then(() => true)
      .catch(() => false)
    cache.set(key, loading)
  }

  return await raceFontLoad(loading, selected ? TERMINAL_FONT_SELECTED_TIMEOUT_MS : TERMINAL_FONT_LOAD_TIMEOUT_MS)
}

async function raceFontLoad(loading: Promise<boolean>, timeoutMs: number): Promise<boolean> {
  let timeout: number | undefined
  try {
    return await Promise.race([
      loading,
      new Promise<boolean>((resolve) => {
        timeout = window.setTimeout(() => resolve(false), timeoutMs)
      }),
    ])
  } finally {
    if (timeout !== undefined) window.clearTimeout(timeout)
  }
}

const FONT_PROBE_TEXT = 'mmmmmmmmmmlli'
const FONT_PROBE_SIZE = '72px'
const GENERIC_FAMILIES = ['monospace', 'sans-serif', 'serif']

function getProbeContext(): CanvasRenderingContext2D | null {
  try {
    return document.createElement('canvas').getContext('2d')
  } catch {
    return null
  }
}

export function isTerminalFontAvailable(family: string): boolean {
  const name = family.trim()
  if (!name) return true
  const context = getProbeContext()

  if (!context || typeof context.measureText !== 'function') return true
  return GENERIC_FAMILIES.some((generic) => {
    context.font = `${FONT_PROBE_SIZE} ${generic}`
    const base = context.measureText(FONT_PROBE_TEXT).width
    context.font = `${FONT_PROBE_SIZE} "${name}", ${generic}`
    return context.measureText(FONT_PROBE_TEXT).width !== base
  })
}

export function isKnownNerdFontFamily(family: string): boolean {
  return NERD_FONT_FAMILIES.includes(family.trim())
}

export const TERMINAL_FONT_FAMILY = buildTerminalFontFamily()
