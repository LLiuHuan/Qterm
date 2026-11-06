import { onUnmounted, ref } from 'vue'
import { whenWailsRuntime } from '../../services/runtime'
import { createFontGateway, fontErrorMessage, type TerminalFontFamily } from '../../services/fonts'
import { translate } from '@/locales'

export function visibleTerminalFontFamilies(
  families: TerminalFontFamily[],
  limit: number,
  selectedFamily: string,
): TerminalFontFamily[] {
  const visible = families.slice(0, limit)
  if (!selectedFamily || visible.some((item) => item.family === selectedFamily)) return visible
  const selected = families.find((item) => item.family === selectedFamily)
  if (!selected) return visible
  if (visible.length < limit) return [...visible, selected]
  return [selected, ...visible.slice(0, Math.max(0, limit - 1))]
}

export function isFontListNearBottom(
  scrollTop: number,
  clientHeight: number,
  scrollHeight: number,
  threshold = 48,
): boolean {
  return scrollHeight - scrollTop - clientHeight <= threshold
}

export type FontScriptVariant = 'hans' | 'hant' | 'common'

export function preferredFontVariant(locale: string): 'hans' | 'hant' {
  const tag = locale.toLowerCase()
  return tag.startsWith('zh-hant') || tag.startsWith('zh-tw') || tag.startsWith('zh-hk') ? 'hant' : 'hans'
}

const HANS_SUFFIX = /(?:[-－–—]\s*(?:简|簡)|简体中文|簡體中文)\s*$/u
const HANT_SUFFIX = /(?:[-－–—]\s*繁|繁体中文|繁體中文)\s*$/u

function variantNameOf(item: TerminalFontFamily): string {
  return item.localizedNames?.['zh-Hans'] ?? item.localizedNames?.['zh-Hant'] ?? item.family
}

export function fontScriptVariant(item: TerminalFontFamily): FontScriptVariant {
  const name = variantNameOf(item)
  if (HANS_SUFFIX.test(name)) return 'hans'
  if (HANT_SUFFIX.test(name)) return 'hant'
  return 'common'
}

export function fontVariantBaseName(item: TerminalFontFamily): string {
  return stripScriptVariantSuffix(variantNameOf(item))
}

export function stripScriptVariantSuffix(name: string): string {
  return name
    .replace(/[-－–—]\s*(?:简|簡|繁)\s*$/u, '')
    .replace(/\s*(?:简体中文|簡體中文|繁体中文|繁體中文)\s*$/u, '')
    .trim()
}

export function filterFamiliesByScript(
  families: TerminalFontFamily[],
  wanted: 'hans' | 'hant',
  selectedFamily: string,
): TerminalFontFamily[] {
  const kept = families.filter((item) => {
    const variant = fontScriptVariant(item)
    return variant === 'common' || variant === wanted
  })
  if (!selectedFamily || kept.some((item) => item.family === selectedFamily)) return kept
  const selected = families.find((item) => item.family === selectedFamily)
  return selected ? [...kept, selected] : kept
}

export function useTerminalFontFamilies() {
  const gateway = createFontGateway()
  const families = ref<TerminalFontFamily[]>([])
  const loading = ref(false)
  const error = ref('')

  const persistent = ref(gateway.persistent)
  let requested = false
  let hasLoaded = false
  const stopRuntimeWatch = whenWailsRuntime(() => {
    persistent.value = gateway.persistent
    if (!loading.value && hasLoaded) void load()
  })
  onUnmounted(stopRuntimeWatch)

  async function load(force = false): Promise<void> {
    if (loading.value) return
    hasLoaded = true
    const startedInPreview = !gateway.persistent
    loading.value = true
    error.value = ''
    try {
      families.value = await gateway.listTerminalFonts(force)
    } catch (caught) {
      families.value = []
      error.value = fontErrorMessage(caught, translate('font.core.readFailed'))
    } finally {
      loading.value = false
      persistent.value = gateway.persistent
      if (startedInPreview && gateway.persistent) void load()
    }
  }

  async function ensureLoaded(): Promise<void> {
    if (requested) return
    requested = true
    await load()
  }

  return {
    persistent,
    families,
    loading,
    error,
    load,
    ensureLoaded,
  }
}
