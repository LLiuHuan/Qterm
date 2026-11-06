<script setup lang="ts">
import { QtermSettingsRow, QtermSettingsSectionHeader, QtermSwitch } from '../../components/common'
import { QtermButton, QtermInput, QtermTextarea } from '../../components/common'
import { computed, onMounted, ref, watch } from 'vue'
import { ImageIcon, Check, Plus, Copy, Pencil, Trash2, X } from '@lucide/vue'
import { readBackgroundImage } from './backgroundImage'
import { SegmentedTabs } from '../../components/common'
import { Slider } from '../../components/common/overlay-controls'
import { ScrollArea } from '../../components/common/control-primitives'
import TerminalColorPicker from './TerminalColorPicker.vue'
import {
  QtermCombobox,
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
  type QtermComboboxGroup,
} from '../../components/common/form-controls'
import {
  filterFamiliesByScript,
  isFontListNearBottom,
  preferredFontVariant,
  stripScriptVariantSuffix,
  useTerminalFontFamilies,
  visibleTerminalFontFamilies,
} from './useTerminalFontFamilies'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import {
  BACKGROUND_OPACITY_MAX,
  BACKGROUND_OPACITY_MIN,
  UI_SCALE_OPTIONS,
  type AssetLibraryLayout,
  type HostColorStyle,
  type ThemeMode,
} from './settingsStore'
import {
  TERMINAL_THEMES,
  loadCustomTerminalThemes,
  saveCustomTerminalThemes,
  type TerminalTheme,
  type TerminalThemeId,
} from '../sessions/terminalThemes'
import { useI18n } from '@/locales'

const { t, locale } = useI18n()

const props = withDefaults(
  defineProps<{
    themeMode: ThemeMode
    minimalAppearance?: boolean
    uiScale?: number

    uiFont?: string

    assetLibraryLayout?: AssetLibraryLayout
    hostColorStyle?: HostColorStyle
    showHostAddress: boolean
    lightBackgroundEnabled: boolean
    lightBackgroundOpacity: number
    lightBackgroundImage: string
    darkBackgroundEnabled: boolean
    darkBackgroundOpacity: number
    darkBackgroundImage: string
    terminalLightTheme?: TerminalThemeId
    terminalDarkTheme?: TerminalThemeId
    terminalThemeBackgroundScope?: import('./settingsStore').TerminalThemeBackgroundScope
  }>(),
  {
    minimalAppearance: false,
    uiScale: 1,
    uiFont: '',
    assetLibraryLayout: 'sidebar',
    hostColorStyle: 'block',
    terminalLightTheme: 'default-light',
    terminalDarkTheme: 'default-dark',
    terminalThemeBackgroundScope: 'terminal',
  },
)

const emit = defineEmits<{
  'update:themeMode': [mode: ThemeMode]
  'update:minimalAppearance': [value: boolean]
  'update:uiScale': [value: number]
  'update:uiFont': [value: string]
  'update:assetLibraryLayout': [value: AssetLibraryLayout]
  'update:hostColorStyle': [value: HostColorStyle]
  'update:showHostAddress': [value: boolean]
  'update:lightBackgroundEnabled': [value: boolean]
  'update:lightBackgroundOpacity': [value: number]
  'update:lightBackgroundImage': [value: string]
  'update:darkBackgroundEnabled': [value: boolean]
  'update:darkBackgroundOpacity': [value: number]
  'update:darkBackgroundImage': [value: string]
  'update:terminalLightTheme': [value: TerminalThemeId]
  'update:terminalDarkTheme': [value: TerminalThemeId]
  'update:terminalThemeBackgroundScope': [value: import('./settingsStore').TerminalThemeBackgroundScope]
}>()

const customThemes = ref<Array<TerminalTheme & { id: string }>>(loadCustomTerminalThemes())
const editorOpen = ref(false)
const editorType = ref<'light' | 'dark'>('light')
const editorName = ref('')
const editorJson = ref('')
const editorError = ref('')
const editingThemeId = ref<string | null>(null)
type ColorKey = Exclude<keyof TerminalTheme, 'name' | 'type'>
const previewColors: ColorKey[] = ['red', 'green', 'yellow', 'blue', 'magenta', 'cyan']
const previewBrightColors: ColorKey[] = [
  'brightRed',
  'brightGreen',
  'brightYellow',
  'brightBlue',
  'brightMagenta',
  'brightCyan',
]
const colorGroups = computed<Array<{ title: string; fields: Array<{ key: ColorKey; label: string }> }>>(() => [
  {
    title: t('appearance.core.colorGroupBase'),
    fields: [
      { key: 'background', label: t('appearance.core.color.background') },
      { key: 'foreground', label: t('appearance.core.color.foreground') },
      { key: 'cursor', label: t('appearance.core.color.cursor') },
      { key: 'cursorAccent', label: t('appearance.core.color.cursorAccent') },
      { key: 'selectionBackground', label: t('appearance.core.color.selectionBackground') },
    ],
  },
  {
    title: t('appearance.core.colorGroupAnsi'),
    fields: [
      { key: 'black', label: t('appearance.core.color.black') },
      { key: 'red', label: t('appearance.core.color.red') },
      { key: 'green', label: t('appearance.core.color.green') },
      { key: 'yellow', label: t('appearance.core.color.yellow') },
      { key: 'blue', label: t('appearance.core.color.blue') },
      { key: 'magenta', label: t('appearance.core.color.magenta') },
      { key: 'cyan', label: t('appearance.core.color.cyan') },
      { key: 'white', label: t('appearance.core.color.white') },
    ],
  },
  {
    title: t('appearance.core.colorGroupBrightAnsi'),
    fields: [
      { key: 'brightBlack', label: t('appearance.core.color.brightBlack') },
      { key: 'brightRed', label: t('appearance.core.color.brightRed') },
      { key: 'brightGreen', label: t('appearance.core.color.brightGreen') },
      { key: 'brightYellow', label: t('appearance.core.color.brightYellow') },
      { key: 'brightBlue', label: t('appearance.core.color.brightBlue') },
      { key: 'brightMagenta', label: t('appearance.core.color.brightMagenta') },
      { key: 'brightCyan', label: t('appearance.core.color.brightCyan') },
      { key: 'brightWhite', label: t('appearance.core.color.brightWhite') },
    ],
  },
])
const editorMode = ref('visual')
const editorColors = ref({ ...TERMINAL_THEMES['default-light'] })
const validColor = (value: unknown): value is string =>
  typeof value === 'string' && /^#(?:[\da-f]{6}|[\da-f]{8})$/i.test(value)
function parseEditorJson(): TerminalTheme {
  const parsed = JSON.parse(editorJson.value)
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed))
    throw new Error(t('appearance.core.errorJsonObject'))
  if (parsed.type !== 'light' && parsed.type !== 'dark') throw new Error(t('appearance.core.errorType'))
  if (typeof parsed.name !== 'string') throw new Error(t('appearance.core.errorName'))
  const value = parsed.value
  if (!value || typeof value !== 'object') throw new Error(t('appearance.core.errorValue'))
  const theme = { ...TERMINAL_THEMES[parsed.type === 'light' ? 'default-light' : 'default-dark'] }
  for (const { fields } of colorGroups.value)
    for (const { key, label } of fields) {
      if (value[key] !== undefined) {
        if (!validColor(value[key])) throw new Error(t('appearance.core.errorColor', { label }))
        theme[key] = value[key]
      }
    }
  return { ...theme, name: parsed.name, type: parsed.type }
}
function syncEditorJson(): void {
  const value = Object.fromEntries(
    colorGroups.value.flatMap((group) => group.fields.map(({ key }) => [key, editorColors.value[key]])),
  )
  editorJson.value = JSON.stringify({ name: editorName.value, type: editorType.value, value }, null, 2)
}
function updateJsonMetadata(): void {
  try {
    const parsed = JSON.parse(editorJson.value)
    parsed.name = editorName.value
    parsed.type = editorType.value
    editorJson.value = JSON.stringify(parsed, null, 2)
  } catch {}
}
function applyEditorJson(): boolean {
  try {
    const theme = parseEditorJson()
    editorName.value = theme.name
    editorType.value = theme.type
    editorColors.value = theme
    editorError.value = ''
    return true
  } catch (error) {
    editorError.value = error instanceof Error ? error.message : t('appearance.core.errorJsonInvalid')
    return false
  }
}
function switchEditorMode(mode: string): void {
  if (mode === editorMode.value) return
  if (mode === 'visual' && !applyEditorJson()) return
  if (mode === 'json') syncEditorJson()
  editorMode.value = mode
  editorError.value = ''
}
const editorPreview = computed(() => {
  if (editorMode.value === 'json') {
    try {
      return parseEditorJson()
    } catch {
      return editorColors.value
    }
  }
  return editorColors.value
})
const lightThemes = computed(() =>
  Object.entries(TERMINAL_THEMES)
    .map(([id, theme]) => ({ id, ...theme }))
    .filter((theme) => theme.type === 'light')
    .concat(customThemes.value.filter((theme) => theme.type === 'light')),
)
const darkThemes = computed(() =>
  Object.entries(TERMINAL_THEMES)
    .map(([id, theme]) => ({ id, ...theme }))
    .filter((theme) => theme.type === 'dark')
    .concat(customThemes.value.filter((theme) => theme.type === 'dark')),
)
function openThemeEditor(type: 'light' | 'dark', source?: TerminalTheme & { id?: string }, copy = false): void {
  editorType.value = type
  editorError.value = ''
  editorMode.value = 'visual'
  editingThemeId.value = source && !copy ? (source.id ?? null) : null
  editorName.value = source ? source.name + (copy ? t('appearance.core.copySuffix') : '') : ''
  editorColors.value = { ...(source ?? TERMINAL_THEMES[type === 'light' ? 'default-light' : 'default-dark']) }
  syncEditorJson()
  editorOpen.value = true
}
function editTheme(theme: TerminalTheme & { id?: string }): void {
  if (!theme.id?.startsWith('custom-')) return
  openThemeEditor(theme.type, theme)
}
function copyTheme(theme: TerminalTheme & { id?: string }): void {
  openThemeEditor(theme.type, theme, true)
}
function deleteTheme(theme: TerminalTheme & { id?: string }): void {
  if (
    !theme.id?.startsWith('custom-') ||
    !window.confirm(t('appearance.core.confirmDeleteTheme', { name: theme.name }))
  )
    return
  customThemes.value = customThemes.value.filter((item) => item.id !== theme.id)
  saveCustomTerminalThemes(customThemes.value)
  if (props.terminalLightTheme === theme.id) emit('update:terminalLightTheme', 'default-light')
  if (props.terminalDarkTheme === theme.id) emit('update:terminalDarkTheme', 'default-dark')
}
function saveTheme(): void {
  try {
    if (editorMode.value === 'json' && !applyEditorJson()) return
    if (!editorName.value.trim()) throw new Error(t('appearance.core.errorThemeName'))
    for (const { fields } of colorGroups.value)
      for (const { key, label } of fields) {
        if (!validColor(editorColors.value[key])) throw new Error(t('appearance.core.errorColor', { label }))
      }
    const theme = {
      ...editorColors.value,
      name: editorName.value.trim(),
      type: editorType.value,
      id: editingThemeId.value ?? 'custom-' + crypto.randomUUID(),
    }
    customThemes.value = editingThemeId.value
      ? customThemes.value.map((item) => (item.id === editingThemeId.value ? theme : item))
      : [...customThemes.value, theme]
    saveCustomTerminalThemes(customThemes.value)
    window.dispatchEvent(new CustomEvent('qterm:terminal-theme-updated', { detail: { id: theme.id } }))
    if (editorType.value === 'light') emit('update:terminalLightTheme', theme.id)
    else emit('update:terminalDarkTheme', theme.id)
    editorOpen.value = false
  } catch (error) {
    editorError.value = error instanceof Error ? error.message : t('appearance.core.errorJsonInvalid')
  }
}

const uiFontHint = computed(() => {
  const stored = (props.uiFont ?? '').trim()
  if (!stored) return t('appearance.core.uiFontHelpSystem')
  const matched = uiFontFamilies.value.find((item) => item.family === stored)
  return t('appearance.core.uiFontHelp', { font: matched ? uiFontLabel(matched) : stored })
})

const themeOptions = computed(() => [
  { value: 'light', label: t('appearance.core.light'), hint: t('appearance.core.lightHint') },
  { value: 'dark', label: t('appearance.core.dark'), hint: t('appearance.core.darkHint') },
  { value: 'system', label: t('appearance.core.system'), hint: t('appearance.core.systemHint') },
])

const themeHint = computed(() => {
  if (props.themeMode === 'light') return t('appearance.core.themeLightCurrent')
  if (props.themeMode === 'dark') return t('appearance.core.themeDarkCurrent')
  return t('appearance.core.themeSystemCurrent')
})

const uiScaleOptions = UI_SCALE_OPTIONS.map((scale) => {
  const percent = Math.round(scale * 100)
  return {
    value: String(scale),
    label: `${percent}%`,
    hint: scale === 1 ? t('appearance.core.uiScaleStandard') : t('appearance.core.uiScaleHint', { percent }),
  }
})

const UI_FONT_DEFAULT_VALUE = '__qterm_system_default_font__'
const UI_FONT_INITIAL_SIZE = 40
const UI_FONT_BATCH_SIZE = 40
const uiFontQuery = ref('')
const uiFontMenuOpen = ref(false)
const uiFontLimit = ref(UI_FONT_INITIAL_SIZE)
const {
  families: uiFontFamilies,
  loading: uiFontLoading,
  error: uiFontError,
  load: loadUiFonts,
} = useTerminalFontFamilies()

const FONT_LOCALE_KEYS: Record<string, string> = {
  'zh-CN': 'zh-Hans',
  'zh-Hant': 'zh-Hant',
  'en-US': 'en',
  'ja-JP': 'ja',
  'ko-KR': 'ko',
  'ru-RU': 'ru',
  'fr-FR': 'fr',
}
const fontNameKey = computed(() => FONT_LOCALE_KEYS[locale.value] ?? '')

function uiFontLabel(item: { family: string; localizedNames?: Record<string, string> }): string {
  const key = fontNameKey.value
  const localized = key ? item.localizedNames?.[key] : undefined
  const name = localized && localized.trim() !== '' ? localized : item.family
  return stripScriptVariantSuffix(name)
}

const localeUiFonts = computed(() =>
  filterFamiliesByScript(uiFontFamilies.value, preferredFontVariant(locale.value), props.uiFont ?? ''),
)

const filteredUiFonts = computed(() => {
  const query = uiFontQuery.value.trim().toLocaleLowerCase()
  const matched = query
    ? localeUiFonts.value.filter((item) => {
        if (uiFontLabel(item).toLocaleLowerCase().includes(query)) return true
        if (item.family.toLocaleLowerCase().includes(query)) return true
        const names = item.localizedNames
        if (!names) return false
        return Object.values(names).some((name) => name.toLocaleLowerCase().includes(query))
      })
    : [...localeUiFonts.value]
  return matched.sort((a, b) => uiFontLabel(a).localeCompare(uiFontLabel(b), locale.value))
})

function fontPreviewStyle(family: string): string {
  return `font-family: "${family.replace(/\\/g, '\\\\').replace(/"/g, '\\"')}", var(--qt-font-ui-stack)`
}

const visibleUiFonts = computed(() =>
  visibleTerminalFontFamilies(filteredUiFonts.value, uiFontLimit.value, props.uiFont ?? ''),
)

const uiFontSelectValue = computed(() => (props.uiFont ?? '') || UI_FONT_DEFAULT_VALUE)

const uiFontOptionGroups = computed<QtermComboboxGroup[]>(() => {
  const groups: QtermComboboxGroup[] = [
    { options: [{ value: UI_FONT_DEFAULT_VALUE, label: t('appearance.core.uiFontSystem') }] },
  ]

  const stored = (props.uiFont ?? '').trim()
  if (stored && !uiFontFamilies.value.some((item) => item.family === stored)) {
    groups.push({
      options: [{ value: stored, label: t('appearance.core.uiFontMissing', { font: stored }) }],
    })
  }
  if (visibleUiFonts.value.length > 0) {
    groups.push({
      options: visibleUiFonts.value.map((item) => ({
        value: item.family,
        label: uiFontLabel(item),
        style: fontPreviewStyle(item.family),
      })),
    })
  }
  return groups
})

function updateUiFont(value: string | number): void {
  emit('update:uiFont', value === UI_FONT_DEFAULT_VALUE ? '' : String(value))
}

function handleUiFontScroll(event: Event): void {
  const element = event.target as HTMLElement | null
  if (!element) return
  if (!isFontListNearBottom(element.scrollTop, element.clientHeight, element.scrollHeight)) return
  if (visibleUiFonts.value.length >= filteredUiFonts.value.length) return
  uiFontLimit.value += UI_FONT_BATCH_SIZE
}

watch(uiFontQuery, () => {
  uiFontLimit.value = UI_FONT_INITIAL_SIZE
})
watch(uiFontMenuOpen, (open) => {
  if (!open) return

  uiFontQuery.value = ''
  uiFontLimit.value = UI_FONT_INITIAL_SIZE
})

onMounted(() => {
  void loadUiFonts()
})

const assetLibraryLayoutOptions = computed(() => [
  { value: 'sidebar', label: t('appearance.core.assetLibraryLayout.sidebar') },
  { value: 'workspace', label: t('appearance.core.assetLibraryLayout.workspace') },
])

function updateAssetLibraryLayout(value: string): void {
  if (value !== 'sidebar' && value !== 'workspace') return
  emit('update:assetLibraryLayout', value)
}

const hostColorStyleOptions = computed(() => [
  { value: 'block', label: t('appearance.core.hostColorStyle.block') },
  { value: 'background', label: t('appearance.core.hostColorStyle.background') },
])
function updateHostColorStyle(value: string): void {
  if (value !== 'block' && value !== 'background') return
  emit('update:hostColorStyle', value)
}

const browserImageInput = ref<HTMLInputElement | null>(null)
const pendingImageKind = ref<'light' | 'dark'>('light')
const selectedImageNames = ref<Record<'light' | 'dark', string>>({ light: '', dark: '' })

function updateBackgroundImage(kind: 'light' | 'dark', value: string): void {
  selectedImageNames.value[kind] = ''
  if (kind === 'light') emit('update:lightBackgroundImage', value)
  else emit('update:darkBackgroundImage', value)
}

function clearBackgroundImage(kind: 'light' | 'dark'): void {
  updateBackgroundImage(kind, '')
  imageError.value = ''
}

function imagePreview(value: string): string {
  return /^(data:image\/|https?:\/\/|blob:)/i.test(value.trim()) ? value.trim() : ''
}

function openImagePicker(kind: 'light' | 'dark'): void {
  if (readingImage.value) return
  pendingImageKind.value = kind
  browserImageInput.value?.click()
}

const imageError = ref('')
const readingImage = ref(false)
async function readBrowserImage(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  const kind = pendingImageKind.value
  input.value = ''
  if (!file) return
  imageError.value = ''
  readingImage.value = true
  try {
    const image = await readBackgroundImage(file, kind)
    updateBackgroundImage(kind, image)
    selectedImageNames.value[kind] = file.name
  } catch (error) {
    imageError.value =
      error instanceof DOMException && error.name === 'QuotaExceededError'
        ? t('background.core.quota')
        : error instanceof Error
          ? error.message
          : t('background.core.readFailed')
  } finally {
    readingImage.value = false
  }
}

function updateOpacity(kind: 'light' | 'dark', value: string | number): void {
  const opacity = Math.min(BACKGROUND_OPACITY_MAX, Math.max(BACKGROUND_OPACITY_MIN, Math.round(Number(value))))
  if (kind === 'light') emit('update:lightBackgroundOpacity', opacity)
  else emit('update:darkBackgroundOpacity', opacity)
}
</script>

<template>
  <section class="settings-panel flex flex-col gap-4 appearance-settings-panel" :aria-label="t('appearance.core.aria')">
    <QtermSettingsSectionHeader
      class="m-0"
      :label="t('appearance.core.appTitle')"
      :description="t('appearance.core.appDescription')"
    />

    <div
      class="settings-card border border-solid border-qt-border rounded-xl [background:color-mix(in_srgb,var(--qt-panel)_32%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
    >
      <QtermSettingsRow :label="t('appearance.core.themeMode')" :description="themeHint">
        <SegmentedTabs
          id="settings-theme-mode"
          :label="t('appearance.core.themeMode')"
          :model-value="props.themeMode"
          :options="themeOptions"
          @update:model-value="(value) => emit('update:themeMode', value as ThemeMode)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        class="settings-switch-row c-p"
        :label="t('appearance.core.minimalAppearance')"
        :description="t('appearance.core.minimalAppearanceHelp')"
      >
        <QtermSwitch
          id="settings-minimal-appearance"
          :model-value="props.minimalAppearance"
          :aria-label="t('appearance.core.minimalAppearance')"
          @update:model-value="(value) => emit('update:minimalAppearance', value === true)"
        />
      </QtermSettingsRow>

      <QtermSettingsRow :label="t('appearance.core.uiScale')" :description="t('appearance.core.uiScaleHelp')">
        <SegmentedTabs
          id="settings-ui-scale"
          :label="t('appearance.core.uiScale')"
          :model-value="String(props.uiScale ?? 1)"
          :options="uiScaleOptions"
          @update:model-value="(value) => emit('update:uiScale', Number(value))"
        />
      </QtermSettingsRow>

      <QtermSettingsRow :label="t('appearance.core.uiFont')" :description="uiFontHint">
        <div class="w-60 shrink-0">
          <QtermCombobox
            id="settings-ui-font"
            :class="uiFontMenuOpen ? undefined : 'qterm-combobox-show-value'"
            menu-class="terminal-font-menu"
            menu-width="auto"
            :model-value="uiFontSelectValue"
            :groups="uiFontOptionGroups"
            :open="uiFontMenuOpen"
            :filter="false"
            :search="uiFontQuery"
            :disabled="uiFontLoading"
            :placeholder="uiFontLoading ? t('appearance.core.uiFontLoading') : t('appearance.core.uiFontSearch')"
            :aria-label="t('appearance.core.uiFont')"
            @update:model-value="updateUiFont"
            @update:open="uiFontMenuOpen = $event"
            @update:search="uiFontQuery = $event"
            @scroll="handleUiFontScroll"
          >
            <template v-if="uiFontError" #footer>
              <div class="terminal-font-status text-xs">{{ uiFontError }}</div>
            </template>
          </QtermCombobox>
        </div>
      </QtermSettingsRow>

      <QtermSettingsRow
        :label="t('appearance.core.assetLibraryLayout')"
        :description="t(`appearance.core.assetLibraryLayout.${props.assetLibraryLayout}Help`)"
      >
        <SegmentedTabs
          id="settings-asset-library-layout"
          :label="t('appearance.core.assetLibraryLayout')"
          :model-value="props.assetLibraryLayout"
          :options="assetLibraryLayoutOptions"
          @update:model-value="updateAssetLibraryLayout"
        />
      </QtermSettingsRow>

      <QtermSettingsRow
        :label="t('appearance.core.hostColorStyle')"
        :description="t(`appearance.core.hostColorStyle.${props.hostColorStyle ?? 'block'}Help`)"
      >
        <SegmentedTabs
          id="settings-host-color-style"
          :label="t('appearance.core.hostColorStyle')"
          :model-value="props.hostColorStyle ?? 'block'"
          :options="hostColorStyleOptions"
          @update:model-value="updateHostColorStyle"
        />
      </QtermSettingsRow>
    </div>

    <QtermSettingsSectionHeader
      class="settings-subsection-header mt-2"
      :label="t('appearance.core.pageBackground')"
      :description="t('appearance.core.pageBackgroundDescription')"
    />
    <div
      class="settings-card settings-background-card overflow-hidden border border-solid border-qt-border rounded-xl [background:color-mix(in_srgb,var(--qt-panel)_32%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
    >
      <div class="settings-background-environment">
        <QtermSettingsRow
          class="settings-switch-row c-p"
          :label="t('appearance.core.lightEnvironment')"
          :description="t('appearance.core.lightBackgroundHelp')"
        >
          <QtermSwitch
            id="settings-light-background-enabled"
            :model-value="props.lightBackgroundEnabled"
            :aria-label="t('appearance.core.lightBackgroundAria')"
            @update:model-value="(value) => emit('update:lightBackgroundEnabled', value === true)"
          />
        </QtermSettingsRow>
        <div class="settings-background-fields grid gap-3 p-[12px_14px_14px]">
          <div class="settings-image-picker flex items-center gap-2">
            <button
              type="button"
              class="settings-image-preview"
              :class="{ 'is-empty': !imagePreview(props.lightBackgroundImage) }"
              :disabled="!props.lightBackgroundEnabled || readingImage"
              :aria-label="t('appearance.core.chooseLightImage')"
              @click="openImagePicker('light')"
            >
              <img
                v-if="imagePreview(props.lightBackgroundImage)"
                :src="imagePreview(props.lightBackgroundImage)"
                alt=""
              />
              <ImageIcon v-else :size="16" />
            </button>
            <QtermButton
              v-if="props.lightBackgroundImage"
              class="!p-3 w-4 h-4 hover:border hover:border-qt-background-border"
              variant="ghost"
              size="sm"
              :disabled="!props.lightBackgroundEnabled || readingImage"
              @click="clearBackgroundImage('light')"
              ><X
            /></QtermButton>
            <small class="text-xs" v-if="selectedImageNames.light">{{ selectedImageNames.light }}</small>
          </div>
          <p v-if="imageError" class="dialog-error text-xs">{{ imageError }}</p>
          <label class="settings-field-label grid gap-1.5 text-qt-muted text-sm"
            ><span
              >{{ t('appearance.core.backgroundOpacity') }} <output>{{ props.lightBackgroundOpacity }}%</output></span
            ><Slider
              :model-value="[props.lightBackgroundOpacity]"
              :min="BACKGROUND_OPACITY_MIN"
              :max="BACKGROUND_OPACITY_MAX"
              :step="1"
              :disabled="!props.lightBackgroundEnabled"
              @update:model-value="(value) => updateOpacity('light', value?.[0] ?? props.lightBackgroundOpacity)"
          /></label>
        </div>
      </div>
      <div class="settings-background-environment">
        <QtermSettingsRow
          class="settings-switch-row c-p"
          :label="t('appearance.core.darkEnvironment')"
          :description="t('appearance.core.darkBackgroundHelp')"
        >
          <QtermSwitch
            id="settings-dark-background-enabled"
            :model-value="props.darkBackgroundEnabled"
            :aria-label="t('appearance.core.darkBackgroundAria')"
            @update:model-value="(value) => emit('update:darkBackgroundEnabled', value === true)"
          />
        </QtermSettingsRow>
        <div class="settings-background-fields grid gap-3 p-[12px_14px_14px]">
          <div class="settings-image-picker flex items-center gap-2">
            <button
              type="button"
              class="settings-image-preview"
              :class="{ 'is-empty': !imagePreview(props.darkBackgroundImage) }"
              :disabled="!props.darkBackgroundEnabled || readingImage"
              :aria-label="t('appearance.core.chooseDarkImage')"
              @click="openImagePicker('dark')"
            >
              <img
                v-if="imagePreview(props.darkBackgroundImage)"
                :src="imagePreview(props.darkBackgroundImage)"
                alt=""
              />
              <ImageIcon v-else :size="16" />
            </button>
            <QtermButton
              v-if="props.darkBackgroundImage"
              class="p-3! w-4 h-4 hover:border hover:border-qt-background-border"
              variant="ghost"
              size="sm"
              :disabled="!props.darkBackgroundEnabled || readingImage"
              @click="clearBackgroundImage('dark')"
              ><X
            /></QtermButton>
            <small class="text-xs" v-if="selectedImageNames.dark">{{ selectedImageNames.dark }}</small>
          </div>
          <p v-if="imageError" class="dialog-error text-xs">{{ imageError }}</p>
          <label class="settings-field-label grid gap-1.5 text-qt-muted text-sm"
            ><span
              >{{ t('appearance.core.backgroundOpacity') }} <output>{{ props.darkBackgroundOpacity }}%</output></span
            ><Slider
              :model-value="[props.darkBackgroundOpacity]"
              :min="BACKGROUND_OPACITY_MIN"
              :max="BACKGROUND_OPACITY_MAX"
              :step="1"
              :disabled="!props.darkBackgroundEnabled"
              @update:model-value="(value) => updateOpacity('dark', value?.[0] ?? props.darkBackgroundOpacity)"
          /></label>
        </div>
      </div>
    </div>
    <input ref="browserImageInput" type="file" accept="image/*" hidden @change="readBrowserImage" />
    <QtermSettingsSectionHeader
      class="settings-subsection-header mt-2"
      :label="t('appearance.core.assetTree')"
      :description="t('appearance.core.assetTreeDescription')"
    />
    <div
      class="settings-card border border-solid border-qt-border rounded-xl [background:color-mix(in_srgb,var(--qt-panel)_32%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
    >
      <QtermSettingsRow
        as="label"
        for-id="settings-show-host-address"
        class="settings-switch-row c-p"
        :label="t('appearance.core.showHostAddress')"
        :description="t('appearance.core.showHostAddressHelp')"
      >
        <QtermSwitch
          id="settings-show-host-address"
          :model-value="props.showHostAddress"
          :aria-label="t('appearance.core.showHostAddress')"
          @update:model-value="(value) => emit('update:showHostAddress', value === true)"
        />
      </QtermSettingsRow>
    </div>
    <QtermSettingsSectionHeader
      class="settings-subsection-header mt-2"
      :label="t('appearance.core.terminalTheme')"
      :description="t('appearance.core.terminalThemeDescription')"
    />
    <div
      class="settings-card border border-solid border-qt-border rounded-xl [background:color-mix(in_srgb,var(--qt-panel)_32%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
    >
      <QtermSettingsRow
        class="terminal-theme-background-scope flex-cb gap-4"
        :label="t('appearance.core.backgroundScope')"
        :description="t('appearance.core.backgroundScopeHelp')"
      >
        <SegmentedTabs
          id="terminal-theme-background-scope"
          :label="t('appearance.core.backgroundScopeAria')"
          :model-value="props.terminalThemeBackgroundScope ?? 'terminal'"
          :options="[
            { value: 'terminal', label: t('appearance.core.scopeTerminal') },
            { value: 'shell', label: t('appearance.core.scopeShell') },
            { value: 'derived', label: t('appearance.core.scopeDerived') },
          ]"
          @update:model-value="
            (value) =>
              emit(
                'update:terminalThemeBackgroundScope',
                value as import('./settingsStore').TerminalThemeBackgroundScope,
              )
          "
        />
      </QtermSettingsRow>
    </div>
    <div class="terminal-theme-columns grid grid-cols-2 gap-3">
      <section
        class="terminal-theme-column settings-card px-4 py-2 grid min-h-0 min-w-0 grid-rows-[auto_minmax(0,1fr)] gap-2 border border-solid border-qt-border rounded-[8px] [background:color-mix(in_srgb,_var(--qt-panel)_32%,_transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
        :aria-label="t('appearance.core.lightTerminalTheme')"
      >
        <header class="flex min-h-7.5 items-center justify-between">
          <strong class="text-sm">{{ t('appearance.core.lightTerminalTheme') }}</strong
          ><QtermButton variant="ghost" size="sm" @click="openThemeEditor('light')"
            ><Plus :size="14" />{{ t('appearance.core.new') }}</QtermButton
          >
        </header>
        <ScrollArea type="hover" :scroll-hide-delay="500" class="terminal-theme-list">
          <div class="terminal-theme-list-content grid gap-2 p-[1px_6px_2px_1px]">
            <article
              v-for="theme in lightThemes"
              :key="theme.id"
              class="terminal-theme-card"
              :class="{ 'is-selected': props.terminalLightTheme === theme.id }"
              :style="{ '--theme-bg': theme.background, '--theme-fg': theme.foreground }"
              @click="emit('update:terminalLightTheme', theme.id)"
              @keydown.enter="emit('update:terminalLightTheme', theme.id)"
              tabindex="0"
              role="button"
            >
              <span class="terminal-theme-card-title flex-cb pr-21.5 text-sm">
                <span class="terminal-theme-card-name inline-flex min-w-0 items-center gap-1">
                  {{ theme.name }}
                  <Check
                    v-if="props.terminalLightTheme === theme.id"
                    class="terminal-theme-card-check flex-none text-qt-accent"
                    :size="15"
                  />
                </span>
                <span class="terminal-theme-card-actions absolute top-1.75 right-2 inline-flex items-center gap-0.5">
                  <button type="button" :aria-label="t('appearance.core.copyTheme')" @click.stop="copyTheme(theme)">
                    <Copy :size="13" />
                  </button>
                  <button
                    v-if="theme.id.startsWith('custom-')"
                    type="button"
                    :aria-label="t('appearance.core.editTheme')"
                    @click.stop="editTheme(theme)"
                  >
                    <Pencil :size="13" />
                  </button>
                  <button
                    v-if="theme.id.startsWith('custom-')"
                    type="button"
                    :aria-label="t('appearance.core.deleteTheme')"
                    @click.stop="deleteTheme(theme)"
                  >
                    <Trash2 :size="13" />
                  </button>
                </span> </span
              ><span class="terminal-theme-preview text-xs">
                <span><b :style="{ color: theme.green }">root@qterm</b>$ ls -l</span>
                <span
                  ><span :style="{ color: theme.cyan }">drwxr-xr-x</span> 1 root
                  <span :style="{ color: theme.blue }">boot</span></span
                >
                <span
                  ><span :style="{ color: theme.cyan }">drwxr-xr-x</span> 1 root
                  <span :style="{ color: theme.blue }">data</span></span
                >
                <span class="terminal-theme-color-samples flex flex-wrap gap-[3px_5px]"
                  ><span v-for="color in previewColors" :key="color" :style="{ color: theme[color] }">{{
                    color
                  }}</span></span
                >
                <span class="terminal-theme-swatches flex gap-1 pt-0.75" aria-hidden="true"
                  ><span v-for="color in previewBrightColors" :key="color" :style="{ backgroundColor: theme[color] }"
                /></span>
              </span>
            </article>
          </div>
        </ScrollArea>
      </section>
      <section
        class="terminal-theme-column settings-card px-4 py-2 grid min-h-0 min-w-0 grid-rows-[auto_minmax(0,1fr)] gap-2 border border-solid border-qt-border rounded-[8px] [background:color-mix(in_srgb,_var(--qt-panel)_32%,_transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
        :aria-label="t('appearance.core.darkTerminalTheme')"
      >
        <header class="flex min-h-7.5 items-center justify-between">
          <strong class="text-sm">{{ t('appearance.core.darkTerminalTheme') }}</strong
          ><QtermButton variant="ghost" size="sm" @click="openThemeEditor('dark')"
            ><Plus :size="14" />{{ t('appearance.core.new') }}</QtermButton
          >
        </header>
        <ScrollArea type="hover" :scroll-hide-delay="500" class="terminal-theme-list">
          <div class="terminal-theme-list-content grid gap-2 p-[1px_6px_2px_1px]">
            <article
              v-for="theme in darkThemes"
              :key="theme.id"
              class="terminal-theme-card"
              :class="{ 'is-selected': props.terminalDarkTheme === theme.id }"
              :style="{ '--theme-bg': theme.background, '--theme-fg': theme.foreground }"
              @click="emit('update:terminalDarkTheme', theme.id)"
              @keydown.enter="emit('update:terminalDarkTheme', theme.id)"
              tabindex="0"
              role="button"
            >
              <span class="terminal-theme-card-title flex-cb pr-21.5 text-sm">
                <span class="terminal-theme-card-name inline-flex min-w-0 items-center gap-1">
                  {{ theme.name }}
                  <Check
                    v-if="props.terminalDarkTheme === theme.id"
                    class="terminal-theme-card-check flex-none text-qt-accent"
                    :size="15"
                  />
                </span>
                <span class="terminal-theme-card-actions absolute top-1.75 right-2 inline-flex items-center gap-0.5">
                  <button type="button" :aria-label="t('appearance.core.copyTheme')" @click.stop="copyTheme(theme)">
                    <Copy :size="13" />
                  </button>
                  <button
                    v-if="theme.id.startsWith('custom-')"
                    type="button"
                    :aria-label="t('appearance.core.editTheme')"
                    @click.stop="editTheme(theme)"
                  >
                    <Pencil :size="13" />
                  </button>
                  <button
                    v-if="theme.id.startsWith('custom-')"
                    type="button"
                    :aria-label="t('appearance.core.deleteTheme')"
                    @click.stop="deleteTheme(theme)"
                  >
                    <Trash2 :size="13" />
                  </button>
                </span> </span
              ><span class="terminal-theme-preview text-xs">
                <span><b :style="{ color: theme.green }">root@qterm</b>$ ls -l</span>
                <span
                  ><span :style="{ color: theme.cyan }">drwxr-xr-x</span> 1 root
                  <span :style="{ color: theme.blue }">boot</span></span
                >
                <span
                  ><span :style="{ color: theme.cyan }">drwxr-xr-x</span> 1 root
                  <span :style="{ color: theme.blue }">data</span></span
                >
                <span class="terminal-theme-color-samples flex flex-wrap gap-[3px_5px]"
                  ><span v-for="color in previewColors" :key="color" :style="{ color: theme[color] }">{{
                    color
                  }}</span></span
                >
                <span class="terminal-theme-swatches flex gap-1 pt-0.75" aria-hidden="true"
                  ><span v-for="color in previewBrightColors" :key="color" :style="{ backgroundColor: theme[color] }"
                /></span>
              </span>
            </article>
          </div>
        </ScrollArea>
      </section>
    </div>
    <QtermDialog v-model:open="editorOpen">
      <QtermDialogContent class="terminal-theme-editor text-sm" :show-close-button="true">
        <QtermDialogHeader
          class="terminal-theme-editor-header pt-4 pr-11 pb-3 pl-4 shrink-0"
          :title="editingThemeId ? t('appearance.core.editThemeTitle') : t('appearance.core.newThemeTitle')"
          :description="t('appearance.core.themeDialogDescription')"
          :show-close="false"
        />
        <div
          class="terminal-theme-editor-grid grid flex-1 min-h-0 grid-cols-[minmax(0,2fr)_minmax(220px,1fr)] gap-4.5 p-4"
        >
          <div
            class="terminal-theme-editor-form grid grid-cols-2 grid-rows-[auto_auto_minmax(0,1fr)] [align-content:start] gap-[14px_12px]"
          >
            <label class="form-field min-w-0 grid gap-1.5 text-qt-muted">
              <span>{{ t('appearance.core.themeName') }}</span>
              <QtermInput
                v-model="editorName"
                @update:model-value="editorMode === 'json' && updateJsonMetadata()"
                :placeholder="t('appearance.core.themeNamePlaceholder')"
              />
            </label>
            <label class="form-field min-w-0 grid gap-1.5 text-qt-muted">
              <span>{{ t('appearance.core.themeType') }}</span>
              <Select v-model="editorType" @update:model-value="editorMode === 'json' && updateJsonMetadata()"
                ><SelectTrigger :aria-label="t('appearance.core.themeType')"><SelectValue /></SelectTrigger
                ><SelectContent
                  ><SelectItem value="light">{{ t('appearance.core.themeTypeLight') }}</SelectItem
                  ><SelectItem value="dark">{{ t('appearance.core.themeTypeDark') }}</SelectItem></SelectContent
                ></Select
              >
            </label>
            <SegmentedTabs
              id="terminal-theme-editor-mode"
              :label="t('appearance.core.editMode')"
              :model-value="editorMode"
              :options="[
                { value: 'visual', label: t('appearance.core.visual') },
                { value: 'json', label: t('appearance.core.json') },
              ]"
              @update:model-value="switchEditorMode"
            />
            <ScrollArea
              v-if="editorMode === 'visual'"
              type="hover"
              class="terminal-theme-colors min-h-0 h-full overflow-hidden"
              ><div class="terminal-theme-colors-content pr-2">
                <section v-for="group in colorGroups" :key="group.title">
                  <h4 class="text-xs">{{ group.title }}</h4>
                  <div class="terminal-theme-color-grid grid grid-cols-2 gap-3">
                    <label
                      v-for="field in group.fields"
                      :key="field.key"
                      class="form-field min-w-0 grid gap-1.5 text-qt-muted"
                    >
                      <span>{{ field.label }}</span>
                      <div class="terminal-theme-color-input flex min-w-0 gap-2">
                        <TerminalColorPicker v-model="editorColors[field.key]" :label="field.label" />
                        <QtermInput
                          v-model="editorColors[field.key]"
                          class="text-xs"
                          :aria-label="field.label"
                          :aria-invalid="!validColor(editorColors[field.key])"
                        />
                      </div>
                    </label>
                  </div>
                </section></div
            ></ScrollArea>
            <label
              v-else
              class="form-field terminal-theme-json-field min-w-0 grid gap-1.5 text-qt-muted min-h-0 grid-rows-[auto_minmax(0,1fr)]"
            >
              <span>{{ t('appearance.core.themeJson') }}</span>
              <QtermTextarea
                v-model="editorJson"
                class="terminal-theme-json text-xs"
                :aria-label="t('appearance.core.themeJson')"
                spellcheck="false"
              />
            </label>
          </div>
          <aside class="terminal-theme-editor-preview flex min-w-0 flex-col gap-2">
            <span class="terminal-theme-editor-preview-label text-qt-muted text-xs">{{
              t('appearance.core.preview')
            }}</span>
            <div
              class="terminal-theme-preview terminal-theme-preview-large text-xs"
              :style="{ '--theme-bg': editorPreview.background, '--theme-fg': editorPreview.foreground }"
            >
              <span>ABCDEFGHIJKLMNOPQRSTUVWXYZ</span>
              <span>abcdefghijklmnopqrstuvwxyz</span>
              <span>0123456789 !@#$%^&amp;*()[]{}&lt;&gt;</span>
              <span class="terminal-theme-color-samples flex flex-wrap gap-[3px_5px]"
                ><span v-for="color in previewColors" :key="color" :style="{ color: editorPreview[color] }">{{
                  color
                }}</span></span
              >
              <span class="terminal-theme-color-samples flex flex-wrap gap-[3px_5px]"
                ><span v-for="color in previewBrightColors" :key="color" :style="{ color: editorPreview[color] }">{{
                  color
                }}</span></span
              >
            </div>
            <small class="text-xs">{{
              t('appearance.core.backgroundForeground', {
                background: editorPreview.background,
                foreground: editorPreview.foreground,
              })
            }}</small>
          </aside>
        </div>
        <p v-if="editorError" role="alert" class="dialog-error text-xs">{{ editorError }}</p>
        <QtermDialogFooter class="terminal-theme-editor-footer">
          <QtermButton variant="outline" @click="editorOpen = false">{{ t('appearance.core.cancel') }}</QtermButton>
          <QtermButton @click="saveTheme">{{ t('appearance.core.saveTheme') }}</QtermButton>
        </QtermDialogFooter>
      </QtermDialogContent>
    </QtermDialog>
  </section>
</template>
