import { connectThemeSync, themeSnapshot } from '../../services/themeSync'
import { connectSettingsSync, type SettingsChangedPayload } from '../../services/settingsSync'
import { createFontGateway, type TerminalFontFamily } from '../../services/fonts'
import { whenWailsRuntime } from '../../services/runtime'
import { fontScriptVariant, fontVariantBaseName, preferredFontVariant } from './useTerminalFontFamilies'
import { useI18n } from '@/locales'
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import {
  DEFAULT_SETTINGS,
  normalizeTerminalFontWeight,
  loadSettings,
  loadSettingsFromPreferences,
  saveSettings,
  TERMINAL_FONT_SIZE_MAX,
  TERMINAL_FONT_SIZE_MIN,
  TERMINAL_LINE_HEIGHT_MAX,
  TERMINAL_LINE_HEIGHT_MIN,
  THEME_MODES,
  type KeywordHighlightRule,
  type QtermSettings,
  type ThemeMode,
} from './settingsStore'
import {
  DEFAULT_SHORTCUTS,
  loadShortcuts,
  loadShortcutsFromPreferences,
  saveShortcuts,
  shortcutBindings,
  type ShortcutAction,
} from './shortcutStore'
import { loadCustomTerminalThemesFromPreferences } from '../sessions/terminalThemes'
import { applyRdpDisplayPreferences } from './rdpDisplayPreferences'

export function useSettingsState() {
  const settings = reactive<QtermSettings>(loadSettings())

  const shortcuts = shortcutBindings

  Object.assign(shortcuts, loadShortcuts())
  const systemPrefersDark = ref(false)
  let mediaQuery: MediaQueryList | null = null
  let hydrating = true
  let applyingRemote = false
  let settingsEditedDuringHydration = false
  let shortcutsEditedDuringHydration = false
  let disposed = false

  const settingsReady = ref(false)

  const isDark = computed(
    () => settings.themeMode === 'dark' || (settings.themeMode === 'system' && systemPrefersDark.value),
  )

  watch(
    isDark,
    (enabled) => {
      document.documentElement.classList.toggle('dark', enabled)
    },
    { immediate: true },
  )

  watch(
    () => settings.minimalAppearance,
    (enabled) => {
      document.documentElement.classList.toggle('qterm-minimal-appearance', enabled)
    },
    { immediate: true },
  )

  watch(
    () => settings.uiScale,
    (scale) => {
      document.documentElement.style.setProperty('--qt-ui-scale', String(scale ?? 1))
    },
    { immediate: true },
  )

  watch(
    () => settings.uiFont,
    (font) => {
      const family = (font ?? '').trim()
      if (!family) {
        document.documentElement.style.removeProperty('--qt-ui-font')
        return
      }
      const escaped = family
        .replace(/\\/g, '\\\\')
        .replace(/"/g, '\\"')
        .replace(/[\r\n\f]/g, ' ')
      document.documentElement.style.setProperty('--qt-ui-font', `"${escaped}", var(--qt-font-ui-stack)`)
    },
    { immediate: true },
  )

  const { locale: uiLocale } = useI18n()
  let variantFamilies: TerminalFontFamily[] | null = null

  async function loadVariantFamilies(): Promise<TerminalFontFamily[]> {
    if (variantFamilies && variantFamilies.length > 0) return variantFamilies
    try {
      const gateway = createFontGateway()
      const list = await gateway.listTerminalFonts()
      if (gateway.persistent && list.length > 0) variantFamilies = list
      return list
    } catch {
      return []
    }
  }

  async function syncUiFontVariant(): Promise<void> {
    const current = (settings.uiFont ?? '').trim()
    if (!current) return
    const families = await loadVariantFamilies()
    const selected = families.find((item) => item.family === current)
    if (!selected) return
    const wanted = preferredFontVariant(uiLocale.value)
    const variant = fontScriptVariant(selected)
    if (variant === 'common' || variant === wanted) return
    const base = fontVariantBaseName(selected)
    const counterpart = families.find(
      (item) => fontScriptVariant(item) === wanted && fontVariantBaseName(item) === base,
    )
    settings.uiFont = counterpart ? counterpart.family : ''
  }

  watch(uiLocale, () => {
    void syncUiFontVariant()
  })
  onMounted(() => {
    void syncUiFontVariant()
  })
  const stopFontRuntimeWatch = whenWailsRuntime(() => {
    variantFamilies = null
    void syncUiFontVariant()
  })
  onUnmounted(stopFontRuntimeWatch)

  let applyingTheme = false
  let themeSync: ReturnType<typeof connectThemeSync> | undefined
  let settingsSync: ReturnType<typeof connectSettingsSync> | undefined

  applyRdpDisplayPreferences(settings)
  watch(
    settings,
    () => {
      if (hydrating && !applyingRemote) settingsEditedDuringHydration = true
      applyRdpDisplayPreferences(settings)

      if (applyingRemote) return
      saveSettings({ ...settings })
    },
    { deep: true, flush: 'sync' },
  )
  watch(
    shortcuts,
    () => {
      if (hydrating && !applyingRemote) shortcutsEditedDuringHydration = true

      if (applyingRemote) return
      saveShortcuts({ ...shortcuts })
    },
    { deep: true, flush: 'sync' },
  )
  watch(
    () => themeSnapshot(settings),
    () => {
      if (!applyingTheme) themeSync?.publish()
    },
    { deep: true, flush: 'sync' },
  )

  function handleSchemeChange(event: MediaQueryListEvent): void {
    systemPrefersDark.value = event.matches
  }

  function setThemeMode(mode: ThemeMode): void {
    if (!THEME_MODES.includes(mode)) return
    settings.themeMode = mode
  }

  function toggleTheme(): void {
    settings.themeMode = isDark.value ? 'light' : 'dark'
  }

  function setTerminalFontWeight(weight: number): void {
    settings.terminalFontWeight = normalizeTerminalFontWeight(weight)
  }

  function setTerminalFontSize(size: number): void {
    const value = Number(size)
    if (!Number.isFinite(value)) return
    settings.terminalFontSize = Math.min(TERMINAL_FONT_SIZE_MAX, Math.max(TERMINAL_FONT_SIZE_MIN, Math.round(value)))
  }

  function setTerminalLineHeight(lineHeight: number): void {
    const value = Number(lineHeight)
    if (!Number.isFinite(value)) return
    const clamped = Math.min(TERMINAL_LINE_HEIGHT_MAX, Math.max(TERMINAL_LINE_HEIGHT_MIN, value))
    settings.terminalLineHeight = Math.round(clamped * 100) / 100
  }

  function setTerminalFontFamily(family: string): void {
    settings.terminalFontFamily = (family ?? '').trim()
  }

  function setTerminalTheme(kind: 'light' | 'dark', id: QtermSettings['terminalLightTheme']): void {
    if (kind === 'light') settings.terminalLightTheme = id
    else settings.terminalDarkTheme = id
  }

  function setTerminalThemeBackgroundScope(scope: QtermSettings['terminalThemeBackgroundScope']): void {
    if (scope === 'terminal' || scope === 'shell' || scope === 'derived') settings.terminalThemeBackgroundScope = scope
  }

  function setKeywordHighlightEnabled(enabled: boolean): void {
    settings.keywordHighlightEnabled = enabled === true
  }

  function setKeywordHighlightRules(rules: KeywordHighlightRule[]): void {
    settings.keywordHighlightRules = rules.map((rule) => ({ ...rule }))
  }

  function resetTerminalTypography(): void {
    settings.terminalFontSize = DEFAULT_SETTINGS.terminalFontSize
    settings.terminalFontWeight = DEFAULT_SETTINGS.terminalFontWeight
    settings.terminalLineHeight = DEFAULT_SETTINGS.terminalLineHeight
    settings.terminalFontFamily = DEFAULT_SETTINGS.terminalFontFamily
  }

  function setShortcut(action: ShortcutAction, binding: string): void {
    if (!Object.prototype.hasOwnProperty.call(DEFAULT_SHORTCUTS, action)) return
    shortcuts[action] = binding
  }

  function resetShortcut(action: ShortcutAction): void {
    if (!Object.prototype.hasOwnProperty.call(DEFAULT_SHORTCUTS, action)) return
    shortcuts[action] = DEFAULT_SHORTCUTS[action]
  }

  async function applyRemoteSettings(payload: SettingsChangedPayload): Promise<void> {
    void payload

    if (disposed) return
    applyingRemote = true
    try {
      Object.assign(settings, await loadSettingsFromPreferences())
      Object.assign(shortcuts, await loadShortcutsFromPreferences())
      await loadCustomTerminalThemesFromPreferences()
    } catch {
    } finally {
      applyingRemote = false
    }
  }

  onMounted(() => {
    themeSync = connectThemeSync(
      () => settings,
      (theme) => {
        applyingTheme = true
        try {
          Object.assign(settings, theme)
        } finally {
          applyingTheme = false
        }
      },
    )
    settingsSync = connectSettingsSync((payload) => {
      void applyRemoteSettings(payload)
    })
    mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
    systemPrefersDark.value = mediaQuery.matches
    mediaQuery.addEventListener('change', handleSchemeChange)

    void Promise.all([
      loadSettingsFromPreferences(),
      loadShortcutsFromPreferences(),
      loadCustomTerminalThemesFromPreferences(),
    ])
      .then(([remoteSettings, remoteShortcuts]) => {
        if (disposed) return
        applyingRemote = true
        try {
          if (!settingsEditedDuringHydration) Object.assign(settings, remoteSettings)
          if (!shortcutsEditedDuringHydration) Object.assign(shortcuts, remoteShortcuts)
        } finally {
          applyingRemote = false
        }
      })
      .catch(() => undefined)
      .finally(() => {
        hydrating = false
        settingsReady.value = true
      })
  })

  onUnmounted(() => {
    disposed = true
    themeSync?.close()
    settingsSync?.close()
    mediaQuery?.removeEventListener('change', handleSchemeChange)
    mediaQuery = null
    document.documentElement.classList.remove('dark')
    document.documentElement.classList.remove('qterm-minimal-appearance')
  })

  return {
    settings,
    settingsReady,
    shortcuts,
    isDark,
    setThemeMode,
    toggleTheme,
    setTerminalFontSize,
    setTerminalFontWeight,
    setTerminalLineHeight,
    setTerminalFontFamily,
    setTerminalTheme,
    setTerminalThemeBackgroundScope,
    setKeywordHighlightEnabled,
    setKeywordHighlightRules,
    resetTerminalTypography,
    setShortcut,
    resetShortcut,
  }
}
