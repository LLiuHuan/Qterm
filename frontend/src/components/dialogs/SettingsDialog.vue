<script setup lang="ts">
import type { HostRecord } from '@/services/hosts.ts'
import { computed, nextTick, ref, watch } from 'vue'
import {
  Highlighter,
  History,
  Info,
  Keyboard,
  KeyRound,
  ShieldCheck,
  Sparkles,
  Lightbulb,
  Monitor,
  Network,
  Palette,
  SquareTerminal,
  Terminal,
  AppWindow,
  PlayCircle,
  Route,
  X,
} from '@lucide/vue'
import { ScrollArea } from '../common/control-primitives'
import { QtermButton } from '../common'
import { QtermDialog, QtermDialogContent, QtermDialogDescription, QtermDialogTitle } from '../common/dialog'
import AppearanceSettingsPanel from '../../features/settings/AppearanceSettingsPanel.vue'
import TerminalSettingsPanel from '../../features/settings/TerminalSettingsPanel.vue'
import SnippetsSettingsPanel from '../../features/settings/SnippetsSettingsPanel.vue'
import CredentialsSettingsPanel from '../../features/settings/CredentialsSettingsPanel.vue'
import ProxySettingsPanel from '../../features/settings/ProxySettingsPanel.vue'
import PortForwardSettingsPanel from '../../features/forwards/PortForwardSettingsPanel.vue'
import HistorySettingsPanel from '../../features/settings/HistorySettingsPanel.vue'
import ShortcutsSettingsPanel from '../../features/settings/ShortcutsSettingsPanel.vue'
import KeywordHighlightSettingsPanel from '../../features/settings/KeywordHighlightSettingsPanel.vue'
import CredentialPromptSettingsPanel from '../../features/settings/CredentialPromptSettingsPanel.vue'
import ApplicationSettingsPanel from '../../features/settings/ApplicationSettingsPanel.vue'
import RdpSettingsPanel from '../../features/settings/RdpSettingsPanel.vue'
import AboutSettingsPanel from '../../features/settings/AboutSettingsPanel.vue'
import { useI18n } from '@/locales'
import { createKnownHostsGateway } from '@/services/knownHosts.ts'
import AiSettingsPanel from '../../features/settings/AiSettingsPanel.vue'

void QtermDialogDescription
void QtermDialogTitle
import KnownHostsSettingsPanel from '../../features/settings/KnownHostsSettingsPanel.vue'
import { SETTINGS_CATEGORY_GROUPS, type SettingsCategory } from '@/features/settings'
import type { CredentialPromptRuleGateway } from '@/services/credentialPromptRules.ts'
import type { ProxyGateway, ProxyRecord } from '@/services/proxies.ts'
import type { PortForwardConfig } from '@/services/forwards.ts'
import type {
  KeywordHighlightRule,
  ThemeMode,
  TerminalGutterMode,
  TerminalGutterTimestampFormat,
  TerminalRenderEngine,
} from '../../features/settings/settingsStore'
import { DEFAULT_SHORTCUTS, type ShortcutAction, type ShortcutBindings } from '../../features/settings/shortcutStore'
import type {
  ChangeMasterPasswordInput,
  ChangeEncryptionKeyInput,
  CredentialGateway,
  CredentialRecord,
  CredentialVaultStatus,
  ExportCredentialVaultKeyInput,
  ImportCredentialVaultKeyInput,
  ResetCredentialVaultInput,
  SetAutoUnlockInput,
  SetupMasterPasswordInput,
} from '@/services/credentials.ts'

const props = withDefaults(
  defineProps<{
    open: boolean
    category: SettingsCategory

    locked: boolean

    settingsInWindow?: boolean

    closeOnEscape?: boolean

    page?: boolean
    status: CredentialVaultStatus | null
    working: boolean
    error: string
    themeMode: ThemeMode
    minimalAppearance?: boolean
    uiScale?: number

    uiFont?: string
    showHostAddress: boolean
    lightBackgroundEnabled: boolean
    lightBackgroundOpacity: number
    lightBackgroundImage: string
    darkBackgroundEnabled: boolean
    darkBackgroundOpacity: number
    darkBackgroundImage: string

    dark?: boolean
    terminalLightTheme?: import('../../features/sessions/terminalThemes').TerminalThemeId
    terminalDarkTheme?: import('../../features/sessions/terminalThemes').TerminalThemeId
    terminalThemeBackgroundScope?: import('../../features/settings/settingsStore').TerminalThemeBackgroundScope
    backgroundEnabled?: boolean
    terminalFontSize: number
    terminalLineHeight?: number
    terminalFontFamily: string
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
    cursorStyle?: import('../../features/settings/settingsStore').TerminalCursorStyle
    rightClickBehavior?: import('../../features/settings/settingsStore').TerminalRightClickBehavior
    modifierRightClickBehavior?: import('../../features/settings/settingsStore').TerminalRightClickBehavior
    modifierKeyLabel?: string
    disconnectBehavior?: import('../../features/settings/settingsStore').TerminalDisconnectBehavior

    autoReconnect?: boolean
    renderEngine?: TerminalRenderEngine
    showTabIndex?: boolean
    tabDoubleClickBehavior?: import('../../features/settings/settingsStore').TabDoubleClickBehavior

    localShellPath?: string

    localShellWorkingDirectory?: string

    rdpShowResolution?: boolean
    rdpFirstFrameTimeoutSeconds?: number
    rdpHostProbeTimeoutSeconds?: number

    rdpFitGraceSeconds?: number

    rdpShowFps?: boolean
    locale?: import('../../features/settings/settingsStore').AppLocaleSetting

    assetLibraryLayout?: import('../../features/settings/settingsStore').AssetLibraryLayout
    hostColorStyle?: import('../../features/settings/settingsStore').HostColorStyle
    sessionHistoryLimit?: number
    recordAllSessions?: boolean
    sftpDirectoryFollowMode?: import('../../features/settings/settingsStore').SftpDirectoryFollowMode
    sftpDragFolderUploadMode?: import('../../features/settings/settingsStore').SftpDragFolderUploadMode
    sftpAutoDirectoryFollow?: boolean
    sftpShowHiddenFiles?: boolean
    sftpMaxTransfers?: number
    sftpLocalDirectory?: string

    recordingDirectory?: string

    recordingDirectoryEffective?: string
    sftpEditorPath?: string
    x11Display?: string
    x11XauthPath?: string
    x11AuthorityFile?: string
    windowRememberSize?: boolean

    closeToTray?: boolean

    restoreOpenSessions?: boolean

    quitConfirm?: boolean
    windowRememberPosition?: boolean
    keywordHighlightEnabled?: boolean
    keywordHighlightRules?: KeywordHighlightRule[]
    shortcuts?: ShortcutBindings
    credentialGateway?: CredentialGateway
    credentialPromptRuleGateway?: CredentialPromptRuleGateway
    otpEntries?: import('../../services/otp').OtpEntry[]
    credentialView?: 'credentials' | 'otp'
    credentials?: CredentialRecord[]

    hosts?: HostRecord[]
    credentialsLoading?: boolean
    credentialsError?: string

    proxyGateway?: ProxyGateway
    proxies?: ProxyRecord[]
    proxiesLoading?: boolean
    proxiesError?: string
    forwards?: PortForwardConfig[]

    forwardGroups?: string[]

    portForwardSearch?: string

    portForwardSearchRevision?: number

    forwardsLoading?: boolean

    forwardsError?: string
  }>(),
  {
    otpEntries: () => [],
    credentialView: 'credentials',
    closeOnEscape: false,
    page: false,
    settingsInWindow: true,
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
    gutterTimestampFormat: 'HH:mm:ss',
    batchCommand: true,
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
    rdpShowResolution: false,
    rdpFirstFrameTimeoutSeconds: 25,
    rdpHostProbeTimeoutSeconds: 3,
    rdpFitGraceSeconds: 6,
    rdpShowFps: false,
    locale: 'zh-CN',
    assetLibraryLayout: 'sidebar',
    hostColorStyle: 'block',
    sessionHistoryLimit: 200,
    recordAllSessions: false,
    sftpDirectoryFollowMode: 'remote',
    sftpDragFolderUploadMode: 'normal',
    sftpAutoDirectoryFollow: true,
    sftpShowHiddenFiles: false,
    sftpMaxTransfers: 3,
    sftpLocalDirectory: '',
    recordingDirectory: '',
    recordingDirectoryEffective: '',
    sftpEditorPath: '',
    x11Display: '',
    x11XauthPath: '',
    x11AuthorityFile: '',
    windowRememberSize: true,
    windowRememberPosition: true,
    keywordHighlightEnabled: false,
    keywordHighlightRules: () => [],
    backgroundEnabled: false,
    terminalLightTheme: 'default-light',
    dark: false,
    terminalDarkTheme: 'default-dark',
    terminalThemeBackgroundScope: 'terminal',
    credentials: () => [],
    credentialGateway: undefined,
    credentialPromptRuleGateway: undefined,
    credentialsLoading: false,
    credentialsError: '',
    proxyGateway: undefined,
    proxies: () => [],
    proxiesLoading: false,
    proxiesError: '',
    forwards: () => [],
    forwardGroups: () => [],
    portForwardSearch: '',
    portForwardSearchRevision: 0,
    forwardsLoading: false,
    forwardsError: '',
    minimalAppearance: false,
    uiScale: 1,
    uiFont: '',
    terminalLineHeight: 1.25,
    shortcuts: () => ({ ...DEFAULT_SHORTCUTS }),
    hosts: () => [],
  },
)

const emit = defineEmits<{
  'update:otpEntries': [value: import('../../services/otp').OtpEntry[]]
  close: []
  'update:category': [category: SettingsCategory]
  'update:forwards': [value: PortForwardConfig[]]
  'update:forwardGroups': [value: string[]]
  'update:themeMode': [mode: ThemeMode]
  'update:minimalAppearance': [value: boolean]
  'update:uiScale': [value: number]
  'update:uiFont': [value: string]
  'update:showHostAddress': [value: boolean]
  'update:lightBackgroundEnabled': [value: boolean]
  'update:lightBackgroundOpacity': [value: number]
  'update:lightBackgroundImage': [value: string]
  'update:darkBackgroundEnabled': [value: boolean]
  'update:darkBackgroundOpacity': [value: number]
  'update:darkBackgroundImage': [value: string]
  'update:terminalLightTheme': [value: import('../../features/sessions/terminalThemes').TerminalThemeId]
  'update:terminalDarkTheme': [value: import('../../features/sessions/terminalThemes').TerminalThemeId]
  'update:terminalThemeBackgroundScope': [
    value: import('../../features/settings/settingsStore').TerminalThemeBackgroundScope,
  ]
  'update:terminalFontSize': [size: number]
  'update:terminalLineHeight': [lineHeight: number]
  'update:terminalFontFamily': [family: string]
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
  'update:cursorStyle': [value: import('../../features/settings/settingsStore').TerminalCursorStyle]
  'update:rightClickBehavior': [value: import('../../features/settings/settingsStore').TerminalRightClickBehavior]
  'update:modifierRightClickBehavior': [
    value: import('../../features/settings/settingsStore').TerminalRightClickBehavior,
  ]
  'update:disconnectBehavior': [value: import('../../features/settings/settingsStore').TerminalDisconnectBehavior]
  'update:autoReconnect': [value: boolean]
  'update:renderEngine': [value: TerminalRenderEngine]
  'update:showTabIndex': [value: boolean]
  'update:tabDoubleClickBehavior': [value: import('../../features/settings/settingsStore').TabDoubleClickBehavior]
  'update:localShellPath': [value: string]
  'update:localShellWorkingDirectory': [value: string]
  'update:rdpShowResolution': [value: boolean]
  'update:rdpFirstFrameTimeoutSeconds': [value: number | undefined]
  'update:rdpHostProbeTimeoutSeconds': [value: number | undefined]
  'update:rdpFitGraceSeconds': [value: number | undefined]
  'update:rdpShowFps': [value: boolean]
  'update:locale': [value: import('../../features/settings/settingsStore').AppLocaleSetting]
  'update:assetLibraryLayout': [value: import('../../features/settings/settingsStore').AssetLibraryLayout]
  'update:hostColorStyle': [value: import('../../features/settings/settingsStore').HostColorStyle]
  'update:sessionHistoryLimit': [value: number]
  'update:recordAllSessions': [value: boolean]
  'update:sftpDirectoryFollowMode': [value: import('../../features/settings/settingsStore').SftpDirectoryFollowMode]
  'update:sftpDragFolderUploadMode': [value: import('../../features/settings/settingsStore').SftpDragFolderUploadMode]
  'update:sftpAutoDirectoryFollow': [value: boolean]
  'update:sftpShowHiddenFiles': [value: boolean]
  'update:sftpMaxTransfers': [value: number]
  'update:sftpLocalDirectory': [value: string]
  'update:recordingDirectory': [value: string]
  'update:sftpEditorPath': [value: string]
  'update:x11Display': [value: string]
  'update:x11XauthPath': [value: string]
  'update:x11AuthorityFile': [value: string]
  'update:windowRememberSize': [value: boolean]
  'update:closeToTray': [value: boolean]
  'update:restoreOpenSessions': [value: boolean]
  'update:quitConfirm': [value: boolean]
  'update:settingsInWindow': [value: boolean]
  'update:windowRememberPosition': [value: boolean]
  'update:keywordHighlightEnabled': [value: boolean]
  'update:keywordHighlightRules': [value: KeywordHighlightRule[]]
  'update:shortcut': [action: ShortcutAction, binding: string]
  resetTerminalTypography: []
  setup: [input: SetupMasterPasswordInput]
  unlock: [password: string]
  unlockAutomatic: []
  changePassword: [input: ChangeMasterPasswordInput]
  changeEncryptionKey: [input: ChangeEncryptionKeyInput]
  unlockCustomKey: [key: string]
  setAutoUnlock: [input: SetAutoUnlockInput]
  exportRecovery: [input: ExportCredentialVaultKeyInput]
  importRecovery: [input: ImportCredentialVaultKeyInput]
  resetVault: [input: ResetCredentialVaultInput]
  lock: []
  refreshCredentials: []
  refreshProxies: []
  refreshCredentialPromptRules: []
  resourcesImported: []
}>()

interface CategoryItem {
  id: SettingsCategory
  icon: typeof KeyRound

  disabled?: boolean
}

interface CategoryGroup {
  id: string
  items: CategoryItem[]
}

const CATEGORY_ICONS: Record<SettingsCategory, typeof KeyRound> = {
  credentials: KeyRound,
  knownHosts: ShieldCheck,
  proxies: Network,
  portForwards: Route,
  snippets: Terminal,
  history: History,
  recordings: PlayCircle,
  application: AppWindow,
  appearance: Palette,
  terminal: SquareTerminal,
  rdp: Monitor,
  highlight: Highlighter,
  credentialPrompt: Lightbulb,
  shortcuts: Keyboard,
  ai: Sparkles,
  about: Info,
}

const categoryGroups: CategoryGroup[] = SETTINGS_CATEGORY_GROUPS.map((group) => ({
  id: group.id,
  items: group.items.map((id) => ({ id, icon: CATEGORY_ICONS[id] })),
}))

const categories = categoryGroups.flatMap((group) => group.items)
const knownHostsGateway = createKnownHostsGateway()

const { t } = useI18n()

function categoryText(item: CategoryItem): string {
  return t(`settings.category.${item.id}`)
}

function categoryDescription(item: CategoryItem): string {
  return t(`settings.category.${item.id}.description`)
}

const activeCategory = computed(() => {
  const item = categories.find((entry) => entry.id === props.category) ?? categories[0]!
  return { ...item, label: categoryText(item), description: categoryDescription(item) }
})

const innerScrollLayout = computed(() =>
  ['credentials', 'proxies', 'portForwards', 'credentialPrompt', 'snippets', 'history', 'recordings'].includes(
    props.category,
  ),
)

const mainScroll = ref<{
  viewport?: { viewportElement?: HTMLElement | null } | HTMLElement | null
} | null>(null)

async function resetMainScroll(): Promise<void> {
  await nextTick()
  scrollMainToTop()
}

function scrollMainToTop(): void {
  const exposedViewport = mainScroll.value?.viewport
  const viewport =
    exposedViewport && 'viewportElement' in exposedViewport
      ? exposedViewport.viewportElement
      : exposedViewport instanceof HTMLElement
        ? exposedViewport
        : null
  if (viewport) viewport.scrollTop = 0
}

watch(
  () => props.category,
  () => void resetMainScroll(),
)
watch(
  () => props.open,
  () => void resetMainScroll(),
)

function selectCategory(category: SettingsCategory): void {
  if (props.locked || category === props.category) return
  const item = categories.find((entry) => entry.id === category)
  if (!item || item.disabled) return
  scrollMainToTop()
  emit('update:category', category)
}

function close(): void {
  if (props.locked) return
  emit('close')
}

function restoreDialogFocusAfterEscape(event: KeyboardEvent): void {
  if (props.page) return
  const active = document.activeElement
  const dialog = event.currentTarget
  if (
    !(active instanceof HTMLElement) ||
    !(dialog instanceof HTMLElement) ||
    active.dataset.slot !== 'scroll-area-viewport' ||
    !active.closest('.settings-nav')
  ) {
    return
  }
  event.preventDefault()
  dialog.focus({ preventScroll: true })
}
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => !open && close()">
    <component
      :is="props.page ? 'div' : QtermDialogContent"
      v-if="props.open"
      class="settings-dialog"
      :class="{ 'has-page-background': props.backgroundEnabled, 'settings-page': props.page }"
      v-bind="props.page ? {} : { closeOnEscape: props.closeOnEscape }"
      :style="props.page ? { background: 'var(--qt-app)' } : undefined"
      @pointer-down-outside.prevent
      @interact-outside.prevent
      @keydown.esc="restoreDialogFocusAfterEscape"
    >
      <div class="settings-shell grid h-full min-h-0 grid-cols-[208px_minmax(0,1fr)]">
        <aside
          class="settings-nav flex min-h-0 flex-col gap-2 px-2.5 pt-4 pb-3 border-r border-r-qt-divider bg-transparent"
          :aria-label="t('settings.nav.aria')"
        >
          <ScrollArea type="hover" :scroll-hide-delay="600" class="settings-nav-scroll min-h-0 grow">
            <div class="flex min-h-full flex-col pr-2">
              <nav class="settings-nav-list flex flex-col gap-1.5">
                <section
                  v-for="(group, groupIndex) in categoryGroups"
                  :key="group.id"
                  class="settings-nav-group flex flex-col gap-0.75"
                  :class="{ 'mt-3.5': groupIndex > 0 }"
                >
                  <p
                    class="settings-nav-group-title text-qt-muted font-medium tracking-[0.04em] text-xs m-0 mb-1 ml-2.5"
                  >
                    {{ t(`settings.group.${group.id}`) }}
                  </p>
                  <button
                    v-for="item in group.items"
                    :key="item.id"
                    class="settings-nav-item text-sm flex h-8.5 items-center gap-2.25 px-2.5 border-0 rounded-[7px] c-p bg-transparent text-qt-muted text-left [transition:background-color_150ms_ease]"
                    :class="{ 'is-active': item.id === props.category }"
                    type="button"
                    :aria-current="item.id === props.category ? 'page' : undefined"
                    :disabled="item.disabled || (props.locked && item.id !== 'application')"
                    :title="categoryDescription(item)"
                    @click="selectCategory(item.id)"
                  >
                    <component :is="item.icon" :size="16" aria-hidden="true" />
                    <span>{{ categoryText(item) }}</span>
                    <em v-if="item.disabled" class="settings-nav-badge not-italic tracking-[0.02em] text-xs ms-auto">{{
                      t('settings.nav.badge.comingSoon')
                    }}</em>
                  </button>
                </section>
              </nav>
              <p class="mt-4 mb-1 px-2.5 text-xs leading-normal text-qt-muted">
                {{ t('settings.nav.note') }}
              </p>
            </div>
          </ScrollArea>
        </aside>

        <section
          class="settings-main flex min-h-0 min-w-0 flex-col"
          :aria-label="`${activeCategory.label}${t('settings.title.suffix')}`"
        >
          <header
            class="border-b border-b-qt-divider settings-main-header flex flex-none items-start justify-between gap-3 px-4 py-3.5"
          >
            <div class="min-w-0">
              <QtermDialogTitle as="h2" class="settings-main-title font-semibold text-base m-0">{{
                activeCategory.label
              }}</QtermDialogTitle>
              <QtermDialogDescription class="settings-main-description text-qt-muted text-xs m-0 mt-0.75">{{
                activeCategory.description
              }}</QtermDialogDescription>
            </div>

            <QtermButton
              v-if="!props.page"
              variant="ghost"
              size="icon-sm"
              class="flex-none hover:border-qt-border"
              type="button"
              :aria-label="t('settings.close')"
              :disabled="props.locked || props.working"
              @click="close"
            >
              <X :size="16" aria-hidden="true" />
            </QtermButton>
          </header>

          <ScrollArea
            ref="mainScroll"
            type="hover"
            :scroll-hide-delay="600"
            class="settings-main-scroll flex-1 min-h-0 overflow-hidden"
            :class="{ 'is-inner-scroll': innerScrollLayout }"
          >
            <div
              class="settings-main-body flex-[1_1_auto] min-h-0 overflow-visible p-4"
              :class="{ 'is-inner-scroll': innerScrollLayout }"
            >
              <CredentialsSettingsPanel
                v-if="props.category === 'credentials' && props.credentialGateway"
                :gateway="props.credentialGateway"
                :otp-entries="props.otpEntries"
                :initial-view="props.credentialView"
                @update:otp-entries="emit('update:otpEntries', $event)"
                :credentials="props.credentials"
                :hosts="props.hosts"
                :status="props.status"
                :loading="props.credentialsLoading"
                :error="props.credentialsError"
                @changed="emit('refreshCredentials')"
                @open-security="selectCategory('application')"
              />
              <AiSettingsPanel v-else-if="props.category === 'ai'" />

              <KnownHostsSettingsPanel v-else-if="props.category === 'knownHosts'" :gateway="knownHostsGateway" />
              <ProxySettingsPanel
                v-else-if="props.category === 'proxies' && props.proxyGateway"
                :gateway="props.proxyGateway"
                :proxies="props.proxies ?? []"
                :hosts="props.hosts ?? []"
                :loading="props.proxiesLoading ?? false"
                :error="props.proxiesError ?? ''"
                @changed="emit('refreshProxies')"
              />
              <PortForwardSettingsPanel
                v-else-if="props.category === 'portForwards'"
                :forwards="props.forwards ?? []"
                :forward-groups="props.forwardGroups ?? []"
                :hosts="props.hosts ?? []"
                :search-query="props.portForwardSearch ?? ''"
                :search-revision="props.portForwardSearchRevision ?? 0"
                :loading="props.forwardsLoading ?? false"
                :error="props.forwardsError ?? ''"
                @update:forwards="emit('update:forwards', $event)"
                @update:forward-groups="emit('update:forwardGroups', $event)"
              />
              <CredentialPromptSettingsPanel
                v-else-if="props.category === 'credentialPrompt' && props.credentialPromptRuleGateway"
                :gateway="props.credentialPromptRuleGateway"
                @changed="emit('refreshCredentialPromptRules')"
              />
              <ApplicationSettingsPanel
                v-else-if="props.category === 'application'"
                :recording-directory="props.recordingDirectory"
                :recording-directory-effective="props.recordingDirectoryEffective"
                @update:recording-directory="emit('update:recordingDirectory', $event)"
                :locale="props.locale"
                :session-history-limit="props.sessionHistoryLimit"
                :record-all-sessions="props.recordAllSessions"
                :sftp-directory-follow-mode="props.sftpDirectoryFollowMode"
                :sftp-drag-folder-upload-mode="props.sftpDragFolderUploadMode"
                :sftp-auto-directory-follow="props.sftpAutoDirectoryFollow"
                :sftp-show-hidden-files="props.sftpShowHiddenFiles"
                :sftp-max-transfers="props.sftpMaxTransfers"
                :sftp-local-directory="props.sftpLocalDirectory"
                :sftp-editor-path="props.sftpEditorPath"
                :x11-display="props.x11Display"
                :x11-xauth-path="props.x11XauthPath"
                :x11-authority-file="props.x11AuthorityFile"
                :window-remember-size="props.windowRememberSize"
                :close-to-tray="props.closeToTray ?? true"
                :restore-open-sessions="props.restoreOpenSessions ?? true"
                :quit-confirm="props.quitConfirm ?? true"
                @update:quit-confirm="emit('update:quitConfirm', $event)"
                :settings-in-window="props.settingsInWindow ?? true"
                @update:settings-in-window="emit('update:settingsInWindow', $event)"
                @update:restore-open-sessions="emit('update:restoreOpenSessions', $event)"
                @update:close-to-tray="emit('update:closeToTray', $event)"
                :window-remember-position="props.windowRememberPosition"
                :vault-status="props.status"
                :security-working="props.working"
                :security-error="props.error"
                :locked="props.locked"
                @update:locale="emit('update:locale', $event)"
                @update:session-history-limit="emit('update:sessionHistoryLimit', $event)"
                @update:record-all-sessions="emit('update:recordAllSessions', $event)"
                @update:sftp-directory-follow-mode="emit('update:sftpDirectoryFollowMode', $event)"
                @update:sftp-drag-folder-upload-mode="emit('update:sftpDragFolderUploadMode', $event)"
                @update:sftp-auto-directory-follow="emit('update:sftpAutoDirectoryFollow', $event)"
                @update:sftp-show-hidden-files="emit('update:sftpShowHiddenFiles', $event)"
                @update:sftp-max-transfers="emit('update:sftpMaxTransfers', $event)"
                @update:sftp-local-directory="emit('update:sftpLocalDirectory', $event)"
                @update:sftp-editor-path="emit('update:sftpEditorPath', $event)"
                @update:x11-display="emit('update:x11Display', $event)"
                @update:x11-xauth-path="emit('update:x11XauthPath', $event)"
                @update:x11-authority-file="emit('update:x11AuthorityFile', $event)"
                @update:window-remember-size="emit('update:windowRememberSize', $event)"
                @update:window-remember-position="emit('update:windowRememberPosition', $event)"
                @change-encryption-key="emit('changeEncryptionKey', $event)"
                @export-recovery="emit('exportRecovery', $event)"
                @import-recovery="emit('importRecovery', $event)"
                @unlock-custom-key="emit('unlockCustomKey', $event)"
                @reset-vault="emit('resetVault', $event)"
              />
              <AppearanceSettingsPanel
                v-else-if="props.category === 'appearance'"
                :theme-mode="props.themeMode"
                :minimal-appearance="props.minimalAppearance"
                :ui-scale="props.uiScale"
                :ui-font="props.uiFont"
                :asset-library-layout="props.assetLibraryLayout"
                :host-color-style="props.hostColorStyle"
                :show-host-address="props.showHostAddress"
                :light-background-enabled="props.lightBackgroundEnabled"
                :light-background-opacity="props.lightBackgroundOpacity"
                :light-background-image="props.lightBackgroundImage"
                :dark-background-enabled="props.darkBackgroundEnabled"
                :dark-background-opacity="props.darkBackgroundOpacity"
                :dark-background-image="props.darkBackgroundImage"
                :terminal-light-theme="props.terminalLightTheme"
                :terminal-dark-theme="props.terminalDarkTheme"
                :terminal-theme-background-scope="props.terminalThemeBackgroundScope"
                @update:theme-mode="emit('update:themeMode', $event)"
                @update:minimal-appearance="emit('update:minimalAppearance', $event)"
                @update:ui-scale="emit('update:uiScale', $event)"
                @update:ui-font="emit('update:uiFont', $event)"
                @update:asset-library-layout="emit('update:assetLibraryLayout', $event)"
                @update:host-color-style="emit('update:hostColorStyle', $event)"
                @update:show-host-address="emit('update:showHostAddress', $event)"
                @update:light-background-enabled="emit('update:lightBackgroundEnabled', $event)"
                @update:light-background-opacity="emit('update:lightBackgroundOpacity', $event)"
                @update:light-background-image="emit('update:lightBackgroundImage', $event)"
                @update:dark-background-enabled="emit('update:darkBackgroundEnabled', $event)"
                @update:dark-background-opacity="emit('update:darkBackgroundOpacity', $event)"
                @update:dark-background-image="emit('update:darkBackgroundImage', $event)"
                @update:terminal-light-theme="emit('update:terminalLightTheme', $event)"
                @update:terminal-dark-theme="emit('update:terminalDarkTheme', $event)"
                @update:terminal-theme-background-scope="emit('update:terminalThemeBackgroundScope', $event)"
              />
              <SnippetsSettingsPanel v-else-if="props.category === 'snippets'" />
              <RdpSettingsPanel
                v-else-if="props.category === 'rdp'"
                :show-resolution="props.rdpShowResolution"
                :show-fps="props.rdpShowFps"
                :first-frame-timeout-seconds="props.rdpFirstFrameTimeoutSeconds"
                :host-probe-timeout-seconds="props.rdpHostProbeTimeoutSeconds"
                :fit-grace-seconds="props.rdpFitGraceSeconds"
                @update:show-resolution="emit('update:rdpShowResolution', $event)"
                @update:show-fps="emit('update:rdpShowFps', $event)"
                @update:first-frame-timeout-seconds="emit('update:rdpFirstFrameTimeoutSeconds', $event)"
                @update:host-probe-timeout-seconds="emit('update:rdpHostProbeTimeoutSeconds', $event)"
                @update:fit-grace-seconds="emit('update:rdpFitGraceSeconds', $event)"
              />
              <HistorySettingsPanel
                v-else-if="props.category === 'history'"
                view="history"
                :dark="props.dark"
                :terminal-light-theme="props.terminalLightTheme"
                :terminal-dark-theme="props.terminalDarkTheme"
              />
              <HistorySettingsPanel
                v-else-if="props.category === 'recordings'"
                view="recordings"
                :dark="props.dark"
                :terminal-light-theme="props.terminalLightTheme"
                :terminal-dark-theme="props.terminalDarkTheme"
              />
              <KeywordHighlightSettingsPanel
                v-else-if="props.category === 'highlight'"
                :enabled="props.keywordHighlightEnabled"
                :rules="props.keywordHighlightRules"
                @update:enabled="emit('update:keywordHighlightEnabled', $event)"
                @update:rules="emit('update:keywordHighlightRules', $event)"
              />
              <ShortcutsSettingsPanel
                v-else-if="props.category === 'shortcuts'"
                :shortcuts="props.shortcuts"
                @update:shortcut="(action, binding) => emit('update:shortcut', action, binding)"
              />
              <AboutSettingsPanel
                v-else-if="props.category === 'about'"
                :credential-gateway="props.credentialGateway"
                :locked="props.locked"
                @resources-imported="emit('resourcesImported')"
              />
              <TerminalSettingsPanel
                v-else-if="props.category === 'terminal'"
                :font-size="props.terminalFontSize"
                :line-height="props.terminalLineHeight"
                :font-family="props.terminalFontFamily"
                :auto-copy-selection="props.autoCopySelection"
                :copy-feedback="props.copyFeedback"
                :show-latency="props.showLatency"
                :show-metrics="props.showMetrics"
                :gutter-mode="props.gutterMode"
                :translate-enabled="props.translateEnabled"
                :translate-target="props.translateTarget"
                :translate-provider="props.translateProvider"
                :translate-endpoint="props.translateEndpoint"
                :gutter-show-timestamps="props.gutterShowTimestamps"
                :gutter-timestamp-format="props.gutterTimestampFormat"
                :batch-command="props.batchCommand"
                :confirm-close="props.confirmClose"
                :scrollback="props.scrollback"
                :confirm-multi-line-paste="props.confirmMultiLinePaste"
                :cursor-blink="props.cursorBlink"
                :record-command-history="props.recordCommandHistory"
                :completion-enabled="props.completionEnabled"
                :history-min-length="props.historyMinLength"
                :history-max-length="props.historyMaxLength"
                :word-separators="props.wordSeparators"
                :mac-option-is-meta="props.macOptionIsMeta"
                :cursor-style="props.cursorStyle"
                :right-click-behavior="props.rightClickBehavior"
                :modifier-right-click-behavior="props.modifierRightClickBehavior"
                :modifier-key-label="props.modifierKeyLabel"
                :disconnect-behavior="props.disconnectBehavior"
                :auto-reconnect="props.autoReconnect"
                :render-engine="props.renderEngine"
                :show-tab-index="props.showTabIndex"
                :tab-double-click-behavior="props.tabDoubleClickBehavior"
                :local-shell-path="props.localShellPath"
                :local-shell-working-directory="props.localShellWorkingDirectory"
                @update:font-size="emit('update:terminalFontSize', $event)"
                @update:line-height="emit('update:terminalLineHeight', $event)"
                @update:font-family="emit('update:terminalFontFamily', $event)"
                @update:auto-copy-selection="emit('update:autoCopySelection', $event)"
                @update:copy-feedback="emit('update:copyFeedback', $event)"
                @update:show-latency="emit('update:showLatency', $event)"
                @update:show-metrics="emit('update:showMetrics', $event)"
                @update:gutter-mode="emit('update:gutterMode', $event as TerminalGutterMode)"
                @update:translate-enabled="emit('update:translateEnabled', $event)"
                @update:translate-target="emit('update:translateTarget', $event)"
                @update:translate-provider="emit('update:translateProvider', $event)"
                @update:translate-endpoint="emit('update:translateEndpoint', $event)"
                @update:gutter-show-timestamps="emit('update:gutterShowTimestamps', $event)"
                @update:gutter-timestamp-format="
                  emit('update:gutterTimestampFormat', $event as TerminalGutterTimestampFormat)
                "
                @update:batch-command="emit('update:batchCommand', $event)"
                @update:confirm-close="emit('update:confirmClose', $event)"
                @update:scrollback="emit('update:scrollback', $event)"
                @update:confirm-multi-line-paste="emit('update:confirmMultiLinePaste', $event)"
                @update:cursor-blink="emit('update:cursorBlink', $event)"
                @update:record-command-history="emit('update:recordCommandHistory', $event)"
                @update:completion-enabled="emit('update:completionEnabled', $event)"
                @update:history-min-length="emit('update:historyMinLength', $event)"
                @update:history-max-length="emit('update:historyMaxLength', $event)"
                @update:word-separators="emit('update:wordSeparators', $event)"
                @update:mac-option-is-meta="emit('update:macOptionIsMeta', $event)"
                @update:cursor-style="emit('update:cursorStyle', $event)"
                @update:right-click-behavior="emit('update:rightClickBehavior', $event)"
                @update:modifier-right-click-behavior="emit('update:modifierRightClickBehavior', $event)"
                @update:disconnect-behavior="emit('update:disconnectBehavior', $event)"
                @update:auto-reconnect="emit('update:autoReconnect', $event)"
                @update:render-engine="emit('update:renderEngine', $event)"
                @update:show-tab-index="emit('update:showTabIndex', $event)"
                @update:tab-double-click-behavior="emit('update:tabDoubleClickBehavior', $event)"
                @update:local-shell-path="emit('update:localShellPath', $event)"
                @update:local-shell-working-directory="emit('update:localShellWorkingDirectory', $event)"
                @reset="emit('resetTerminalTypography')"
              />
            </div>
          </ScrollArea>
        </section>
      </div>
    </component>
  </QtermDialog>
</template>
