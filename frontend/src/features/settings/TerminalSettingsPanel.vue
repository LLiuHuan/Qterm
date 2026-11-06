<script setup lang="ts">
import { QtermHint, QtermSwitch } from '../../components/common'
import { QtermButton, QtermInput, QtermNumberField } from '../../components/common'
import { computed, onMounted, ref, watch } from 'vue'
import { File, FolderOpen } from '@lucide/vue'
import {
  QtermCombobox,
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
  type QtermComboboxGroup,
} from '../../components/common/form-controls'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../../components/common/control-primitives'
import { buildTerminalFontFamily, terminalFontStack } from '../sessions/terminalFont'
import {
  filterFamiliesByScript,
  isFontListNearBottom,
  preferredFontVariant,
  useTerminalFontFamilies,
  visibleTerminalFontFamilies,
} from './useTerminalFontFamilies'
import {
  TERMINAL_SCROLLBACK_MAX,
  TERMINAL_SCROLLBACK_MIN,
  TERMINAL_FONT_SIZE_MAX,
  TERMINAL_FONT_SIZE_MIN,
  TERMINAL_LINE_HEIGHT_MAX,
  TERMINAL_LINE_HEIGHT_MIN,
  TERMINAL_HISTORY_LENGTH_MAX,
  TERMINAL_HISTORY_LENGTH_MIN,
} from './settingsStore'
import { SegmentedTabs } from '../../components/common'
import { QtermSettingsRow, QtermSettingsSectionHeader } from '../../components/common'
import { hasNativeFileDialogs, pickLocalShellDirectory, pickLocalShellExecutable } from '@/services/file-dialogs.ts'
import { toast } from 'vue-sonner'
import TranslationCredentials from './TranslationCredentials.vue'
import type {
  TerminalCursorStyle,
  TerminalRightClickBehavior,
  TerminalDisconnectBehavior,
  TerminalRenderEngine,
  TerminalGutterMode,
  TerminalGutterTimestampFormat,
  TabDoubleClickBehavior,
} from './settingsStore'
import { isTranslateProvider, type TranslateProvider } from '@/services/translate.ts'
import { useI18n } from '@/locales'

const props = withDefaults(
  defineProps<{
    mode?: 'all' | 'typography' | 'interaction'
    fontSize: number

    lineHeight?: number

    fontFamily: string
    autoCopySelection?: boolean
    copyFeedback?: boolean
    showLatency?: boolean
    showMetrics?: boolean
    gutterMode?: TerminalGutterMode
    translateEnabled?: boolean
    translateTarget?: string
    translateProvider?: string
    translateEndpoint?: string
    gutterShowTimestamps?: boolean
    gutterTimestampFormat?: TerminalGutterTimestampFormat
    batchCommand?: boolean
    confirmClose?: boolean

    scrollback?: number

    confirmMultiLinePaste?: boolean
    cursorBlink?: boolean

    recordCommandHistory?: boolean
    completionEnabled?: boolean
    historyMinLength?: number
    historyMaxLength?: number
    wordSeparators?: string

    macOptionIsMeta?: boolean
    cursorStyle?: TerminalCursorStyle
    rightClickBehavior?: TerminalRightClickBehavior

    modifierRightClickBehavior?: TerminalRightClickBehavior

    modifierKeyLabel?: string
    disconnectBehavior?: TerminalDisconnectBehavior

    autoReconnect?: boolean
    renderEngine?: TerminalRenderEngine

    showTabIndex?: boolean

    tabDoubleClickBehavior?: TabDoubleClickBehavior

    localShellPath?: string

    localShellWorkingDirectory?: string
  }>(),
  {
    mode: 'all',
    lineHeight: 1.25,
    autoCopySelection: false,
    copyFeedback: true,
    showLatency: true,
    showMetrics: false,
    gutterMode: 'off',
    translateEnabled: true,
    translateTarget: 'zh-CN',
    translateProvider: 'google',
    translateEndpoint: '',
    gutterShowTimestamps: true,
    gutterTimestampFormat: 'time',
    batchCommand: false,
    confirmClose: true,
    scrollback: 5000,
    confirmMultiLinePaste: true,
    cursorBlink: false,
    recordCommandHistory: true,
    completionEnabled: true,
    historyMinLength: 1,
    historyMaxLength: 4096,
    wordSeparators: ' ()[]{}\'"',
    macOptionIsMeta: false,
    cursorStyle: 'block',
    rightClickBehavior: 'menu',
    modifierRightClickBehavior: 'none',
    modifierKeyLabel: 'Command',
    disconnectBehavior: 'close',
    autoReconnect: false,
    renderEngine: 'webgl',
    showTabIndex: false,
    tabDoubleClickBehavior: 'rename',
    localShellPath: '',
    localShellWorkingDirectory: '',
  },
)

const { t, locale } = useI18n()

const emit = defineEmits<{
  'update:fontSize': [size: number]
  'update:lineHeight': [lineHeight: number]
  'update:fontFamily': [family: string]
  reset: []
  'update:autoCopySelection': [value: boolean]
  'update:copyFeedback': [value: boolean]
  'update:showLatency': [value: boolean]
  'update:showMetrics': [value: boolean]
  'update:gutterMode': [value: TerminalGutterMode]
  'update:translateEnabled': [value: boolean]
  'update:translateTarget': [value: string]
  'update:translateProvider': [value: string]
  'update:translateEndpoint': [value: string]
  'update:gutterShowTimestamps': [value: boolean]
  'update:gutterTimestampFormat': [value: TerminalGutterTimestampFormat]
  'update:batchCommand': [value: boolean]
  'update:confirmClose': [value: boolean]
  'update:scrollback': [value: number]
  'update:confirmMultiLinePaste': [value: boolean]
  'update:cursorBlink': [value: boolean]
  'update:recordCommandHistory': [value: boolean]
  'update:completionEnabled': [value: boolean]
  'update:historyMinLength': [value: number]
  'update:historyMaxLength': [value: number]
  'update:wordSeparators': [value: string]
  'update:macOptionIsMeta': [value: boolean]
  'update:cursorStyle': [value: TerminalCursorStyle]
  'update:rightClickBehavior': [value: TerminalRightClickBehavior]
  'update:modifierRightClickBehavior': [value: TerminalRightClickBehavior]
  'update:disconnectBehavior': [value: TerminalDisconnectBehavior]
  'update:autoReconnect': [value: boolean]
  'update:renderEngine': [value: TerminalRenderEngine]
  'update:showTabIndex': [value: boolean]
  'update:tabDoubleClickBehavior': [value: TabDoubleClickBehavior]
  'update:localShellPath': [value: string]
  'update:localShellWorkingDirectory': [value: string]
}>()

const DEFAULT_FONT_VALUE = '__qterm_default_font__'
const FONT_OPTION_INITIAL_SIZE = 60
const FONT_OPTION_BATCH_SIZE = 30
const fontQuery = ref('')
const fontMenuOpen = ref(false)
const visibleFontLimit = ref(FONT_OPTION_INITIAL_SIZE)

const choosingLocalShellPath = ref<'shell' | 'directory' | null>(null)

async function chooseLocalShellExecutable(): Promise<void> {
  if (choosingLocalShellPath.value !== null) return
  choosingLocalShellPath.value = 'shell'
  try {
    const value = await pickLocalShellExecutable()

    if (value) emit('update:localShellPath', value)
  } catch {
    toast.error(t('terminal.local.selectionFailed'))
  } finally {
    choosingLocalShellPath.value = null
  }
}

async function chooseLocalShellDirectory(): Promise<void> {
  if (choosingLocalShellPath.value !== null) return
  choosingLocalShellPath.value = 'directory'
  try {
    const value = await pickLocalShellDirectory()
    if (value) emit('update:localShellWorkingDirectory', value)
  } catch {
    toast.error(t('terminal.local.selectionFailed'))
  } finally {
    choosingLocalShellPath.value = null
  }
}

const {
  persistent: fontListFromBackend,
  families,
  loading: loadingFamilies,
  error: familiesError,
  load,
} = useTerminalFontFamilies()

const monospaceFamilies = computed(() =>
  filterFamiliesByScript(families.value, preferredFontVariant(locale.value), props.fontFamily ?? '').filter(
    (item) => item.monospace,
  ),
)

const filteredFamilies = computed(() => {
  const query = fontQuery.value.trim().toLocaleLowerCase()
  const matched = query
    ? monospaceFamilies.value.filter((item) => item.family.toLocaleLowerCase().includes(query))
    : [...monospaceFamilies.value]
  return matched.sort((a, b) => a.family.localeCompare(b.family))
})

const visibleFamilies = computed(() => {
  return visibleTerminalFontFamilies(filteredFamilies.value, visibleFontLimit.value, props.fontFamily)
})
const hiddenFontCount = computed(() => Math.max(0, filteredFamilies.value.length - visibleFamilies.value.length))

const fontOptionGroups = computed<QtermComboboxGroup[]>(() => {
  const groups: QtermComboboxGroup[] = [
    { options: [{ value: DEFAULT_FONT_VALUE, label: t('terminal.core.defaultFont') }] },
  ]
  if (storedFontMissing.value) {
    groups.push({
      options: [{ value: props.fontFamily, label: t('terminal.core.fontMissing', { font: props.fontFamily }) }],
    })
  }
  if (visibleFamilies.value.length > 0) {
    groups.push({
      options: visibleFamilies.value.map((item) => ({
        value: item.family,

        label: item.family,
        style: `font-family: "${item.family.replace(/\\/g, '\\\\').replace(/"/g, '\\"')}", var(--qt-font-ui-stack)`,
      })),
    })
  }
  return groups
})
const cursorStyleOptions = computed(() => [
  { value: 'block', label: t('terminal.core.block'), icon: 'terminal-cursor-preview terminal-cursor-preview--block' },
  {
    value: 'underline',
    label: t('terminal.core.underline'),
    icon: 'terminal-cursor-preview terminal-cursor-preview--underline',
  },
  { value: 'bar', label: t('terminal.core.bar'), icon: 'terminal-cursor-preview terminal-cursor-preview--bar' },
])
const modifierRightClickBehaviorOptions = computed(() => [
  { value: 'none', label: t('terminal.core.none') },
  { value: 'paste', label: t('terminal.core.paste') },
  { value: 'menu', label: t('terminal.core.showMenu') },
])
const rightClickBehaviorOptions = computed(() => [
  { value: 'none', label: t('terminal.core.none') },
  { value: 'paste', label: t('terminal.core.paste') },
  { value: 'menu', label: t('terminal.core.showMenu') },
])
const disconnectBehaviorOptions = computed(() => [
  { value: 'close', label: t('terminal.core.disconnectClose') },
  { value: 'reconnect', label: t('terminal.core.disconnectReconnect') },
])
const renderEngineOptions = computed(() => [
  { value: 'webgl', label: t('terminal.core.renderEngineWebGL') },
  { value: 'dom', label: t('terminal.core.renderEngineDom') },
])

const translateLanguageOptions = [
  { value: 'zh-CN', label: '简体中文' },
  { value: 'zh-TW', label: '繁體中文' },
  { value: 'en', label: 'English' },
  { value: 'ja', label: '日本語' },
  { value: 'ko', label: '한국어' },
  { value: 'fr', label: 'Français' },
  { value: 'de', label: 'Deutsch' },
  { value: 'ru', label: 'Русский' },
  { value: 'es', label: 'Español' },
]
const translateProviderOptions = computed(() => [
  { value: 'google', label: t('terminal.core.translateProviderGoogle') },
  { value: 'microsoft', label: t('terminal.core.translateProviderMicrosoft') },
  { value: 'deepl', label: t('terminal.core.translateProviderDeepL') },
  { value: 'baidu', label: t('terminal.core.translateProviderBaidu') },
  { value: 'ali', label: t('terminal.core.translateProviderAli') },
  { value: 'youdao', label: t('terminal.core.translateProviderYoudao') },
  { value: 'libretranslate', label: t('terminal.core.translateProviderLibre') },
])
const selectedTranslateProvider = computed<TranslateProvider>(() =>
  isTranslateProvider(props.translateProvider) ? props.translateProvider : 'google',
)

function selectTranslateProvider(value: string): void {
  if (!isTranslateProvider(value)) return
  if (value === selectedTranslateProvider.value) return
  emit('update:translateProvider', value)
}

const gutterModeOptions = computed(() => [
  { value: 'off', label: t('terminal.core.gutterModeOff') },
  { value: 'lines', label: t('terminal.core.gutterModeLines') },
  { value: 'commands', label: t('terminal.core.gutterModeCommands') },
])
const tabDoubleClickBehaviorOptions = computed(() => [
  { value: 'rename', label: t('terminal.core.tabDoubleClickRename') },
  { value: 'duplicate', label: t('terminal.core.tabDoubleClickDuplicate') },
])

const storedFontMissing = computed(
  () => props.fontFamily !== '' && !families.value.some((item) => item.family === props.fontFamily),
)
const fontSelectValue = computed(() => props.fontFamily || DEFAULT_FONT_VALUE)

const defaultEffectiveFont = computed(() => {
  const installed = new Set(families.value.map((item) => item.family))
  const fromStack = terminalFontStack().find((family) => installed.has(family))
  if (fromStack) return fromStack
  return families.value.find((item) => item.hasNerdIcons)?.family ?? families.value[0]?.family ?? ''
})
const selectedFamily = computed(() => families.value.find((item) => item.family === props.fontFamily) ?? null)
const effectiveFont = computed(() => selectedFamily.value?.family ?? defaultEffectiveFont.value)

const previewStyle = computed(() => ({
  fontFamily: buildTerminalFontFamily(props.fontFamily),
  fontSize: `${props.fontSize}px`,

  lineHeight: String(props.lineHeight),
}))

const familyHint = computed(() => {
  if (familiesError.value) return familiesError.value
  if (loadingFamilies.value) return t('font.core.loading')
  if (families.value.length === 0) {
    return fontListFromBackend.value ? t('font.core.emptySystem') : t('font.core.emptyBrowser')
  }
  const source = fontListFromBackend.value ? t('font.core.loaded') : t('font.core.browserDetected')
  return t('font.core.count', { source, count: families.value.length })
})

function handleFontChange(value: unknown): void {
  const selected = String(value)
  emit('update:fontFamily', selected === DEFAULT_FONT_VALUE ? '' : selected)
}

function handleFontListScroll(event: Event): void {
  if (hiddenFontCount.value === 0) return
  const viewport = event.currentTarget as HTMLElement | null
  if (!viewport || !isFontListNearBottom(viewport.scrollTop, viewport.clientHeight, viewport.scrollHeight)) return
  visibleFontLimit.value += FONT_OPTION_BATCH_SIZE
}

watch(fontQuery, () => {
  visibleFontLimit.value = FONT_OPTION_INITIAL_SIZE
})

watch(fontMenuOpen, (open) => {
  if (open) visibleFontLimit.value = FONT_OPTION_INITIAL_SIZE
})

onMounted(() => {
  void load()
})

const ICON_FOLDER = '\u{F01A8}'
const ICON_JAVA = '\u{E738}'
const ICON_FILE = '\u{F15B}'
</script>

<template>
  <section class="settings-panel flex flex-col gap-4 terminal-settings-panel" :aria-label="t('terminal.core.aria')">
    <QtermSettingsSectionHeader
      v-if="props.mode !== 'interaction'"
      :label="t('terminal.core.typography')"
      :description="t('terminal.core.typographyDescription')"
    />

    <div
      v-if="props.mode !== 'interaction'"
      class="settings-card border border-solid border-qt-border rounded-xl [background:color-mix(in_srgb,var(--qt-panel)_32%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
    >
      <QtermSettingsRow
        :label="t('terminal.core.fontSize')"
        :description="t('terminal.core.fontSizeHelp', { min: TERMINAL_FONT_SIZE_MIN, max: TERMINAL_FONT_SIZE_MAX })"
      >
        <QtermNumberField
          id="settings-terminal-font-size"
          class="terminal-font-size"
          :model-value="props.fontSize"
          :min="TERMINAL_FONT_SIZE_MIN"
          :max="TERMINAL_FONT_SIZE_MAX"
          :step="1"
          :aria-label="t('terminal.core.fontSizeAria')"
          :decrease-label="t('terminal.core.decreaseFontSize')"
          :increase-label="t('terminal.core.increaseFontSize')"
          @update:model-value="(value) => value !== undefined && emit('update:fontSize', value)"
        />
      </QtermSettingsRow>

      <QtermSettingsRow
        :label="t('terminal.core.lineHeight')"
        :description="
          t('terminal.core.lineHeightHelp', { min: TERMINAL_LINE_HEIGHT_MIN, max: TERMINAL_LINE_HEIGHT_MAX })
        "
      >
        <QtermNumberField
          id="settings-terminal-line-height"
          class="terminal-line-height w-30"
          :model-value="props.lineHeight"
          :min="TERMINAL_LINE_HEIGHT_MIN"
          :max="TERMINAL_LINE_HEIGHT_MAX"
          :step="0.05"
          :integer-only="false"
          :aria-label="t('terminal.core.lineHeightAria')"
          :decrease-label="t('terminal.core.decreaseLineHeight')"
          :increase-label="t('terminal.core.increaseLineHeight')"
          @update:model-value="(value) => value !== undefined && emit('update:lineHeight', value)"
        />
      </QtermSettingsRow>

      <div class="settings-card-row flex-cb gap-4 pt-3 pr-3.5 pb-3 pl-3.5 border-b border-b-qt-divider">
        <div>
          <strong class="text-sm">{{ t('terminal.core.font') }}</strong>
          <small class="text-xs">{{ familyHint }}</small>
        </div>
        <div class="terminal-font-picker flex items-center gap-2 flex-none">
          <QtermCombobox
            id="settings-terminal-font"
            class="terminal-font-select w-60 flex-none"
            menu-class="terminal-font-menu"
            menu-width="auto"
            :model-value="fontSelectValue"
            :groups="fontOptionGroups"
            :open="fontMenuOpen"
            :filter="false"
            :search="fontQuery"
            :placeholder="t('terminal.core.fontSearchPlaceholder')"
            :aria-label="t('terminal.core.fontSelectAria')"
            @update:model-value="handleFontChange"
            @update:open="fontMenuOpen = $event"
            @update:search="fontQuery = $event"
            @scroll="handleFontListScroll"
          >
          </QtermCombobox>
          <QtermButton
            variant="outline"
            class="terminal-typography-action text-xs"
            type="button"
            :disabled="loadingFamilies"
            @click="load(true)"
          >
            {{ loadingFamilies ? t('terminal.core.detecting') : t('terminal.core.redetect') }}
          </QtermButton>
        </div>
      </div>

      <QtermSettingsRow :label="t('terminal.core.renderEngine')" :description="t('terminal.core.renderEngineHelp')">
        <SegmentedTabs
          id="settings-terminal-render-engine"
          :label="t('terminal.core.renderEngine')"
          :model-value="props.renderEngine"
          :options="renderEngineOptions"
          @update:model-value="emit('update:renderEngine', $event as TerminalRenderEngine)"
        />
      </QtermSettingsRow>

      <QtermSettingsRow :label="t('terminal.core.preview')" :description="t('terminal.core.previewHelp')">
        <QtermButton variant="outline" class="terminal-typography-action text-xs" type="button" @click="emit('reset')">
          {{ t('terminal.core.resetDefault') }}
        </QtermButton>
      </QtermSettingsRow>

      <div class="terminal-font-preview" :style="previewStyle" :aria-label="t('terminal.core.fontPreviewAria')">
        <p class="terminal-font-preview-line m-0 leading-normal whitespace-pre">
          <span class="terminal-font-preview-dir">{{ ICON_FOLDER }} src</span>
          <span class="terminal-font-preview-file">{{ ICON_JAVA }} Main.java</span>
          <span class="terminal-font-preview-file">{{ ICON_FILE }} README.md</span>
        </p>
        <p class="terminal-font-preview-line m-0 leading-normal whitespace-pre">
          abcdefghijklmnopqrstuvwxyz 0123456789
        </p>
        <p class="terminal-font-preview-line m-0 leading-normal whitespace-pre">你好，Qterm 终端预览 ~/project</p>
      </div>

      <p
        class="terminal-font-effective text-xs mt-0 mr-3.5 mb-3.5 ml-3.5 text-qt-muted leading-[1.55]"
        aria-live="polite"
      >
        {{ t('terminal.core.currentFont')
        }}<strong class="text-sm">{{ effectiveFont || t('terminal.core.unknownFont') }}</strong>
        <template v-if="storedFontMissing">{{ t('terminal.core.fontFallback', { font: props.fontFamily }) }}</template>
        <template v-else-if="selectedFamily && !selectedFamily.monospace">
          {{ t('terminal.core.nonMonospaceWarning') }}
        </template>
      </p>
    </div>

    <QtermSettingsSectionHeader
      v-if="props.mode !== 'typography'"
      class="settings-subsection-header mt-2"
      :label="t('terminal.core.interaction')"
      :description="t('terminal.core.interactionDescription')"
    />
    <div
      v-if="props.mode !== 'typography'"
      class="settings-card border border-solid border-qt-border rounded-xl [background:color-mix(in_srgb,var(--qt-panel)_32%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
    >
      <QtermSettingsRow
        :label="t('terminal.core.disconnectBehavior')"
        :description="t('terminal.core.disconnectBehaviorHelp')"
      >
        <SegmentedTabs
          id="settings-terminal-disconnect-behavior"
          :label="t('terminal.core.disconnectBehavior')"
          :model-value="props.disconnectBehavior"
          :options="disconnectBehaviorOptions"
          @update:model-value="emit('update:disconnectBehavior', $event as TerminalDisconnectBehavior)"
        />
      </QtermSettingsRow>

      <QtermSettingsRow
        class="settings-switch-row c-p"
        :label="t('terminal.core.autoReconnect')"
        :description="t('terminal.core.autoReconnectHelp')"
      >
        <QtermSwitch
          :model-value="props.autoReconnect"
          :aria-label="t('terminal.core.autoReconnect')"
          @update:model-value="emit('update:autoReconnect', $event)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow :label="t('terminal.core.rightClick')" :description="t('terminal.core.rightClickHelp')">
        <SegmentedTabs
          id="settings-terminal-right-click-behavior"
          :label="t('terminal.core.rightClick')"
          :model-value="props.rightClickBehavior"
          :options="rightClickBehaviorOptions"
          @update:model-value="emit('update:rightClickBehavior', $event as TerminalRightClickBehavior)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        :label="t('terminal.core.modifierRightClick', { key: props.modifierKeyLabel ?? 'Command' })"
        :description="t('terminal.core.modifierRightClickHelp', { key: props.modifierKeyLabel ?? 'Command' })"
      >
        <SegmentedTabs
          id="settings-terminal-modifier-right-click-behavior"
          :label="t('terminal.core.modifierRightClick', { key: props.modifierKeyLabel ?? 'Command' })"
          :model-value="props.modifierRightClickBehavior"
          :options="modifierRightClickBehaviorOptions"
          @update:model-value="emit('update:modifierRightClickBehavior', $event as TerminalRightClickBehavior)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        class="settings-switch-row c-p"
        :label="t('terminal.core.copyOnSelect')"
        :description="t('terminal.core.copyOnSelectHelp')"
      >
        <QtermSwitch
          :model-value="props.autoCopySelection"
          :aria-label="t('terminal.core.copyOnSelect')"
          @update:model-value="emit('update:autoCopySelection', $event)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        class="settings-switch-row c-p"
        :label="t('terminal.core.copyFeedback')"
        :description="t('terminal.core.copyFeedbackHelp')"
      >
        <QtermSwitch
          :model-value="props.copyFeedback"
          :aria-label="t('terminal.core.copyFeedback')"
          @update:model-value="emit('update:copyFeedback', $event)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        class="settings-switch-row c-p"
        :label="t('terminal.core.networkLatency')"
        :description="t('terminal.core.networkLatencyHelp')"
      >
        <QtermSwitch
          :model-value="props.showLatency"
          :aria-label="t('terminal.core.networkLatency')"
          @update:model-value="emit('update:showLatency', $event)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        class="settings-switch-row c-p"
        :label="t('terminal.core.metrics')"
        :description="t('terminal.core.metricsHelp')"
      >
        <QtermSwitch
          :model-value="props.showMetrics"
          :aria-label="t('terminal.core.metrics')"
          @update:model-value="emit('update:showMetrics', $event)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow :label="t('terminal.core.gutterMode')" :description="t('terminal.core.gutterModeHelp')">
        <SegmentedTabs
          id="settings-terminal-gutter-mode"
          :label="t('terminal.core.gutterMode')"
          :model-value="props.gutterMode"
          :options="gutterModeOptions"
          @update:model-value="emit('update:gutterMode', $event as TerminalGutterMode)"
        />
      </QtermSettingsRow>

      <QtermSettingsRow
        v-if="props.gutterMode !== 'off'"
        class="settings-switch-row c-p"
        :label="t('terminal.core.showTimestamps')"
        :description="t('terminal.core.showTimestampsHelp')"
      >
        <QtermSwitch
          :model-value="props.gutterShowTimestamps"
          :aria-label="t('terminal.core.showTimestamps')"
          @update:model-value="emit('update:gutterShowTimestamps', $event)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        v-if="props.gutterMode !== 'off' && props.gutterShowTimestamps"
        :label="t('terminal.core.timestampFormat')"
      >
        <template #description>
          <span>{{ t('terminal.core.timestampFormatHelp') }}</span>

          <QtermHint class="mt-1.5 mb-0 px-2 py-1">{{ t('terminal.core.timestampFormatTokens') }}</QtermHint>
        </template>
        <QtermInput
          id="settings-terminal-timestamp-format"
          class="w-55 font-mono text-xs"
          :model-value="props.gutterTimestampFormat"
          :aria-label="t('terminal.core.timestampFormat')"
          :placeholder="t('terminal.core.timestampFormatPlaceholder')"
          spellcheck="false"
          autocomplete="off"
          @update:model-value="(value) => emit('update:gutterTimestampFormat', String(value))"
        />
      </QtermSettingsRow>
      <QtermHint v-if="props.gutterMode === 'commands'" class="mx-3.5 -mt-1 mb-3.5">
        {{ t('terminal.core.commandApproximate') }}
      </QtermHint>
      <QtermSettingsRow
        class="settings-switch-row c-p"
        :label="t('terminal.core.batchCommand')"
        :description="t('terminal.core.batchCommandHelp')"
      >
        <QtermSwitch
          :model-value="props.batchCommand"
          :aria-label="t('terminal.core.batchCommand')"
          @update:model-value="emit('update:batchCommand', $event)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow :label="t('terminal.core.scrollback')" :description="t('terminal.core.scrollbackHint')">
        <QtermNumberField
          class="terminal-scrollback"
          :model-value="props.scrollback"
          :min="TERMINAL_SCROLLBACK_MIN"
          :max="TERMINAL_SCROLLBACK_MAX"
          :step="1000"
          :step-snapping="false"
          :aria-label="t('terminal.core.scrollback')"
          :decrease-label="t('terminal.core.scrollback')"
          :increase-label="t('terminal.core.scrollback')"
          @update:model-value="(value) => value !== undefined && emit('update:scrollback', value)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        class="settings-switch-row c-p"
        :label="t('terminal.core.confirmMultiLinePaste')"
        :description="t('terminal.core.confirmMultiLinePasteHint')"
      >
        <QtermSwitch
          :model-value="props.confirmMultiLinePaste"
          :aria-label="t('terminal.core.confirmMultiLinePaste')"
          @update:model-value="emit('update:confirmMultiLinePaste', $event)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        class="settings-switch-row c-p"
        :label="t('terminal.core.confirmClose')"
        :description="t('terminal.core.confirmCloseHelp')"
      >
        <QtermSwitch
          :model-value="props.confirmClose"
          :aria-label="t('terminal.core.confirmClose')"
          @update:model-value="emit('update:confirmClose', $event)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        class="settings-switch-row c-p"
        :label="t('terminal.core.commandHistory')"
        :description="t('terminal.core.commandHistoryHelp')"
      >
        <QtermSwitch
          :model-value="props.recordCommandHistory"
          :aria-label="t('terminal.core.commandHistory')"
          @update:model-value="emit('update:recordCommandHistory', $event)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        class="settings-switch-row c-p"
        :label="t('terminal.core.completion')"
        :description="t('terminal.core.completionHelp')"
      >
        <QtermSwitch
          :model-value="props.completionEnabled"
          :aria-label="t('terminal.core.completion')"
          @update:model-value="emit('update:completionEnabled', $event)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        :label="t('terminal.core.historyMinLength')"
        :description="t('terminal.core.historyMinLengthHelp')"
      >
        <QtermNumberField
          class="terminal-history-number w-30"
          :min="TERMINAL_HISTORY_LENGTH_MIN"
          :max="TERMINAL_HISTORY_LENGTH_MAX"
          :model-value="props.historyMinLength"
          @update:model-value="emit('update:historyMinLength', $event ?? 0)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        :label="t('terminal.core.historyMaxLength')"
        :description="t('terminal.core.historyMaxLengthHelp')"
      >
        <QtermNumberField
          class="terminal-history-number w-30"
          :min="TERMINAL_HISTORY_LENGTH_MIN"
          :max="TERMINAL_HISTORY_LENGTH_MAX"
          :model-value="props.historyMaxLength"
          @update:model-value="emit('update:historyMaxLength', $event ?? 0)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        layout="stacked"
        :label="t('terminal.core.wordSeparators')"
        :description="t('terminal.core.wordSeparatorsHelp')"
      >
        <div class="flex w-full items-center gap-2">
          <QtermInput
            class="terminal-word-separators flex-1"
            :model-value="props.wordSeparators"
            :aria-label="t('terminal.core.wordSeparators')"
            @update:model-value="(value) => emit('update:wordSeparators', String(value))"
          />
        </div>
      </QtermSettingsRow>
      <QtermSettingsRow
        class="settings-switch-row c-p"
        :label="t('terminal.core.macOptionIsMeta')"
        :description="t('terminal.core.macOptionIsMetaHelp')"
      >
        <QtermSwitch
          :model-value="props.macOptionIsMeta"
          :aria-label="t('terminal.core.macOptionIsMeta')"
          @update:model-value="emit('update:macOptionIsMeta', $event)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        class="settings-switch-row c-p"
        :label="t('terminal.core.cursorBlink')"
        :description="t('terminal.core.cursorBlinkHelp')"
      >
        <QtermSwitch
          :model-value="props.cursorBlink"
          :aria-label="t('terminal.core.cursorBlink')"
          @update:model-value="emit('update:cursorBlink', $event)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow :label="t('terminal.core.cursorStyle')" :description="t('terminal.core.cursorStyleHelp')">
        <SegmentedTabs
          id="settings-terminal-cursor-style"
          :label="t('terminal.core.cursorStyle')"
          :model-value="props.cursorStyle"
          :options="cursorStyleOptions"
          @update:model-value="emit('update:cursorStyle', $event as TerminalCursorStyle)"
        />
      </QtermSettingsRow>

      <QtermSettingsRow
        class="settings-switch-row c-p"
        :label="t('terminal.core.showTabIndex')"
        :description="t('terminal.core.showTabIndexHelp')"
      >
        <QtermSwitch
          :model-value="props.showTabIndex"
          :aria-label="t('terminal.core.showTabIndex')"
          @update:model-value="emit('update:showTabIndex', $event)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow :label="t('terminal.core.tabDoubleClick')" :description="t('terminal.core.tabDoubleClickHelp')">
        <SegmentedTabs
          id="settings-terminal-tab-double-click-behavior"
          :label="t('terminal.core.tabDoubleClick')"
          :model-value="props.tabDoubleClickBehavior"
          :options="tabDoubleClickBehaviorOptions"
          @update:model-value="emit('update:tabDoubleClickBehavior', $event as TabDoubleClickBehavior)"
        />
      </QtermSettingsRow>
    </div>

    <QtermSettingsSectionHeader
      v-if="props.mode !== 'typography'"
      class="settings-subsection-header mt-2"
      :label="t('terminal.core.translateSection')"
      :description="t('terminal.core.translateSectionHelp')"
    />
    <div
      v-if="props.mode !== 'typography'"
      class="settings-card border border-solid border-qt-border rounded-xl [background:color-mix(in_srgb,var(--qt-panel)_32%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
    >
      <QtermSettingsRow
        class="settings-switch-row c-p"
        :label="t('terminal.core.translate')"
        :description="t('terminal.core.translateHelp')"
      >
        <QtermSwitch
          :model-value="props.translateEnabled"
          :aria-label="t('terminal.core.translate')"
          @update:model-value="emit('update:translateEnabled', $event)"
        />
      </QtermSettingsRow>
      <template v-if="props.translateEnabled">
        <QtermSettingsRow :label="t('terminal.core.translateTarget')">
          <Select
            :model-value="props.translateTarget"
            @update:model-value="(value) => typeof value === 'string' && emit('update:translateTarget', value)"
          >
            <SelectTrigger class="min-w-45" :aria-label="t('terminal.core.translateTarget')">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="option in translateLanguageOptions" :key="option.value" :value="option.value">
                {{ option.label }}
              </SelectItem>
            </SelectContent>
          </Select>
        </QtermSettingsRow>

        <div class="pt-3 pb-3">
          <Tabs
            :model-value="selectedTranslateProvider"
            class="flex-col"
            @update:model-value="(value) => typeof value === 'string' && selectTranslateProvider(value)"
          >
            <div class="px-3.5">
              <TabsList
                :aria-label="t('terminal.core.translateProviderTabs')"
                class="qterm-segmented-tabs__list flex h-auto w-full flex-wrap items-center justify-start gap-1"
              >
                <TabsTrigger
                  v-for="option in translateProviderOptions"
                  :key="option.value"
                  :value="option.value"
                  class="qterm-segmented-tabs__trigger flex-none"
                >
                  {{ option.label }}
                </TabsTrigger>
              </TabsList>
            </div>
            <TabsContent value="google">
              <p class="m-0 px-3.5 pt-2.5 pb-0 text-xs text-qt-muted">
                {{ t('terminal.core.translateNoKeyNeeded') }}
              </p>
            </TabsContent>
            <TabsContent value="microsoft">
              <p class="m-0 px-3.5 pt-2.5 pb-0 text-xs text-qt-muted">
                {{ t('terminal.core.translateNoKeyNeeded') }}
              </p>
            </TabsContent>
            <TabsContent value="deepl">
              <TranslationCredentials provider="deepl" />
            </TabsContent>
            <TabsContent value="baidu">
              <TranslationCredentials provider="baidu" />
            </TabsContent>
            <TabsContent value="ali">
              <TranslationCredentials provider="ali" />
            </TabsContent>
            <TabsContent value="youdao">
              <TranslationCredentials provider="youdao" />
            </TabsContent>
            <TabsContent value="libretranslate">
              <QtermSettingsRow
                :label="t('terminal.core.translateEndpoint')"
                :description="t('terminal.core.translateEndpointHelp')"
              >
                <QtermInput
                  class="w-55 max-w-full font-mono text-xs"
                  :model-value="props.translateEndpoint"
                  :aria-label="t('terminal.core.translateEndpoint')"
                  :placeholder="t('terminal.core.translateEndpointPlaceholder')"
                  spellcheck="false"
                  autocomplete="off"
                  @update:model-value="(value) => emit('update:translateEndpoint', String(value))"
                />
              </QtermSettingsRow>
              <p class="m-0 px-3.5 pt-2.5 pb-0 text-xs text-qt-muted">
                {{ t('terminal.core.translateLibreNoKey') }}
              </p>
            </TabsContent>
          </Tabs>
        </div>
      </template>
    </div>

    <QtermSettingsSectionHeader
      v-if="props.mode !== 'typography'"
      class="settings-subsection-header mt-2"
      :label="t('terminal.local.title')"
      :description="t('terminal.local.description')"
    />

    <div
      v-if="props.mode !== 'typography'"
      class="settings-card border border-solid border-qt-border rounded-xl [background:color-mix(in_srgb,var(--qt-panel)_32%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
    >
      <div class="settings-card-row flex-col flex-cb gap-4 pt-3 pr-3.5 pb-3 pl-3.5 border-b-d">
        <div class="w-full">
          <strong class="text-sm">{{ t('terminal.local.path') }}</strong>
          <small class="text-xs">{{ t('terminal.local.pathHelp') }}</small>
        </div>
        <div class="flex w-full shrink-0 items-center gap-2">
          <QtermInput
            class="flex-1 font-mono text-xs"
            :model-value="props.localShellPath"
            :aria-label="t('terminal.local.path')"
            :placeholder="t('terminal.local.pathPlaceholder')"
            spellcheck="false"
            autocomplete="off"
            @update:model-value="(value) => emit('update:localShellPath', String(value))"
          />
          <QtermButton
            v-if="hasNativeFileDialogs()"
            variant="outline"
            type="button"
            :disabled="choosingLocalShellPath !== null"
            @click="chooseLocalShellExecutable"
          >
            <File class="size-3.5" aria-hidden="true" />
            {{ t('terminal.local.chooseFile') }}
          </QtermButton>
        </div>
      </div>
      <div class="settings-card-row flex-col flex-cb gap-4 pt-3 pr-3.5 pb-3 pl-3.5 border-b-d">
        <div class="w-full">
          <strong class="text-sm">{{ t('terminal.local.workingDirectory') }}</strong>
          <small class="text-xs">{{ t('terminal.local.workingDirectoryHelp') }}</small>
        </div>
        <div class="flex w-full shrink-0 items-center gap-2">
          <QtermInput
            class="flex-1 font-mono text-xs"
            :model-value="props.localShellWorkingDirectory"
            :aria-label="t('terminal.local.workingDirectory')"
            :placeholder="t('terminal.local.workingDirectoryPlaceholder')"
            spellcheck="false"
            autocomplete="off"
            @update:model-value="(value) => emit('update:localShellWorkingDirectory', String(value))"
          />
          <QtermButton
            v-if="hasNativeFileDialogs()"
            variant="outline"
            type="button"
            :disabled="choosingLocalShellPath !== null"
            @click="chooseLocalShellDirectory"
          >
            <FolderOpen class="size-3.5" aria-hidden="true" />
            {{ t('terminal.local.chooseDirectory') }}
          </QtermButton>
        </div>
      </div>
    </div>
  </section>
</template>
