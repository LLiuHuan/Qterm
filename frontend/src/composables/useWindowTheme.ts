import { computed, onMounted, onUnmounted, provide, ref, watchEffect, type Ref } from 'vue'
import { dialogThemeContextKey } from '../components/common/dialog'
import { getTerminalTheme } from '../features/sessions/terminalThemes'
import type { QtermSettings } from '../features/settings/settingsStore'

type WindowThemeOptions = {
  [
    K in
      | 'lightBackgroundEnabled'
      | 'lightBackgroundOpacity'
      | 'lightBackgroundImage'
      | 'darkBackgroundEnabled'
      | 'darkBackgroundOpacity'
      | 'darkBackgroundImage'
      | 'pageBackgroundSize'
      | 'terminalLightTheme'
      | 'terminalDarkTheme'
      | 'terminalThemeBackgroundScope'
  ]: Readonly<Ref<NonNullable<QtermSettings[K]>>>
} & { isDark: Readonly<Ref<boolean>> }

export function useWindowTheme(options: WindowThemeOptions) {
  const {
    isDark,
    lightBackgroundEnabled,
    lightBackgroundOpacity,
    lightBackgroundImage,
    darkBackgroundEnabled,
    darkBackgroundOpacity,
    darkBackgroundImage,
    pageBackgroundSize,
    terminalLightTheme,
    terminalDarkTheme,
    terminalThemeBackgroundScope,
  } = options
  const terminalThemeRevision = ref(0)
  const handleTerminalThemeUpdated = () => terminalThemeRevision.value++
  onMounted(() => window.addEventListener('qterm:terminal-theme-updated', handleTerminalThemeUpdated))
  onUnmounted(() => window.removeEventListener('qterm:terminal-theme-updated', handleTerminalThemeUpdated))
  const backgroundEnabled = computed(() => {
    const enabled = isDark.value ? darkBackgroundEnabled.value : lightBackgroundEnabled.value
    const opacity = isDark.value ? darkBackgroundOpacity.value : lightBackgroundOpacity.value
    return enabled && opacity > 0
  })

  const pageBackgroundStyle = computed<Record<string, string>>(() => {
    const enabled = isDark.value ? darkBackgroundEnabled.value : lightBackgroundEnabled.value
    const image = isDark.value ? darkBackgroundImage.value : lightBackgroundImage.value
    const opacity = isDark.value ? darkBackgroundOpacity.value : lightBackgroundOpacity.value
    const strength = enabled ? opacity / 100 : 0
    const safeImage = /^(https?:\/\/|data:image\/|blob:|file:\/\/|\/|[A-Za-z]:[\\/])/i.test(image.trim())
      ? image.trim()
      : ''
    const panelPercent = Math.round(18 + strength * 14)
    const hoverPercent = Math.round(22 + strength * 24)
    const selectedPercent = Math.round(28 + strength * 22)
    const borderPercent = Math.round(22 + strength * 20)
    const mutedPercent = Math.round(64 + strength * 18)
    const haloAlpha = (0.18 + strength * 0.5).toFixed(2)
    const haloSoftAlpha = (0.1 + strength * 0.3).toFixed(2)
    const haloColor = isDark.value ? '0 0 0' : '255 255 255'
    const terminalTheme = getTerminalTheme(
      isDark.value ? terminalDarkTheme.value : terminalLightTheme.value,
      isDark.value ? 'default-dark' : 'default-light',
    )

    const terminalSurface = terminalTheme.background
    const outerSurface =
      terminalThemeBackgroundScope.value === 'terminal'
        ? 'var(--qt-panel)'
        : terminalThemeBackgroundScope.value === 'derived'
          ? `color-mix(in srgb, ${terminalTheme.background} 78%, ${isDark.value ? '#000000' : '#ffffff'} 22%)`
          : terminalTheme.background
    const themeBackground =
      terminalThemeBackgroundScope.value === 'shell'
        ? terminalTheme.background
        : terminalThemeBackgroundScope.value === 'derived'
          ? outerSurface
          : ''
    const lightStepPercents = [10, 20, 30, 40, 50, 60, 70, 78, 86, 94]
    const darkStepPercents = [2, 4, 6, 8, 10, 12, 14, 16, 18, 20]
    const basePanel = isDark.value ? '#151517' : '#ffffff'
    const themeStep = (index: number): string =>
      isDark.value
        ? `color-mix(in srgb, ${outerSurface} ${100 - darkStepPercents[index]}%, #ffffff ${darkStepPercents[index]}%)`
        : `color-mix(in srgb, ${outerSurface} ${lightStepPercents[index]}%, ${basePanel})`
    const terminalGlass = enabled
      ? terminalThemeBackgroundScope.value === 'terminal'
        ? `color-mix(in srgb, ${terminalSurface} 58%, transparent)`
        : 'transparent'
      : terminalSurface
    const shellGlass = enabled
      ? 'transparent'
      : terminalThemeBackgroundScope.value === 'shell'
        ? 'var(--qt-terminal)'
        : terminalThemeBackgroundScope.value === 'derived'
          ? 'var(--qt-theme-6)'
          : 'var(--qt-panel)'

    const assetLibrarySurface = shellGlass

    const assetLibraryCardSurface = enabled
      ? 'color-mix(in srgb, var(--qt-panel) 72%, transparent)'
      : terminalThemeBackgroundScope.value === 'terminal'
        ? 'var(--qt-panel)'
        : `color-mix(in srgb, ${assetLibrarySurface} 72%, var(--qt-panel))`
    const assetLibraryCardHover = enabled
      ? 'var(--qt-background-hover)'
      : `color-mix(in srgb, var(--qt-text) 7%, ${assetLibraryCardSurface})`

    const size = pageBackgroundSize.value
    const backgroundSize = size === 'stretch' ? '100% 100%' : size === 'tile' ? 'auto' : size
    const backgroundRepeat = size === 'tile' ? 'repeat' : 'no-repeat'
    return {
      '--qt-background-image': enabled && safeImage ? `url(${JSON.stringify(safeImage)})` : 'none',
      '--qt-background-size': backgroundSize,
      '--qt-background-repeat': backgroundRepeat,
      '--qt-background-color': 'transparent',
      '--qt-background-opacity': enabled ? String(opacity / 100) : '0',
      '--qt-background-opacity-percent': enabled ? `${opacity}%` : '0%',
      '--qt-background-control': `color-mix(in srgb, var(--qt-panel) ${panelPercent}%, transparent)`,
      '--qt-background-hover': `color-mix(in srgb, var(--qt-panel) ${hoverPercent}%, transparent)`,
      '--qt-background-selected': `color-mix(in srgb, var(--qt-panel) ${selectedPercent}%, transparent)`,
      '--qt-background-border': `color-mix(in srgb, var(--qt-text) ${borderPercent}%, transparent)`,
      '--qt-background-muted': `color-mix(in srgb, var(--qt-text) ${mutedPercent}%, transparent)`,
      '--qt-background-text-shadow': `0 1px 2px rgb(${haloColor} / ${haloAlpha}), 0 0 5px rgb(${haloColor} / ${haloSoftAlpha})`,

      '--qt-background-separator-shadow': `rgb(${haloColor} / ${haloSoftAlpha})`,
      '--qt-theme-revision': String(terminalThemeRevision.value),
      '--qt-terminal': terminalTheme.background,
      '--qt-terminal-text': terminalTheme.foreground,
      '--qt-theme-outer': outerSurface,
      '--qt-theme-terminal': terminalTheme.background,
      '--qt-theme-1': themeStep(0),
      '--qt-theme-2': themeStep(1),
      '--qt-theme-3': themeStep(2),
      '--qt-theme-4': themeStep(3),
      '--qt-theme-5': themeStep(4),
      '--qt-theme-6': themeStep(5),
      '--qt-theme-7': themeStep(6),
      '--qt-theme-8': themeStep(7),
      '--qt-theme-9': themeStep(8),
      '--qt-theme-10': themeStep(9),
      '--qt-glass-surface': shellGlass,
      '--qt-glass-terminal': terminalGlass,
      '--qt-asset-library-bg': assetLibrarySurface,
      '--qt-asset-library-card-bg': assetLibraryCardSurface,
      '--qt-asset-library-card-hover': assetLibraryCardHover,
      ...(themeBackground
        ? {
            '--qt-app': themeBackground,
          }
        : {}),
    }
  })

  watchEffect(() => {
    if (typeof document === 'undefined') return
    const backdrop = pageBackgroundStyle.value['--qt-background-border']
    if (backdrop) document.documentElement.style.setProperty('--qt-background-border', backdrop)
    else document.documentElement.style.removeProperty('--qt-background-border')
  })

  const dialogThemeContext = computed(() => ({
    className: [
      isDark.value ? 'is-dark' : '',
      backgroundEnabled.value ? 'has-page-background' : '',
      !backgroundEnabled.value && terminalThemeBackgroundScope.value !== 'terminal' ? 'has-terminal-theme' : '',
      `terminal-theme-background-${terminalThemeBackgroundScope.value}`,
    ]
      .filter(Boolean)
      .join(' '),
    style: pageBackgroundStyle.value,
  }))
  provide(dialogThemeContextKey, dialogThemeContext)
  return { backgroundEnabled, pageBackgroundStyle, dialogThemeContext }
}
