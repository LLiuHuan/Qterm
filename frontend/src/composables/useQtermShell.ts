import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useCredentialState } from './useCredentialState'
import { useSessionState, type SessionOpenOrigin } from './useSessionState'
import { createHostGateway, type HostRecord } from '../services/hosts'
import { openRdpWindow } from '../services/remotedesktop'
import { hostProtocolDefinition } from '../features/hosts/hostProtocols'
import { isAdHocHostId } from '../services/adhocSession'
import { createSessionGateway, sessionErrorMessage, type SessionState } from '../services/sessions'
import { createSftpGateway } from '../services/sftp'
import { createDockerGateway } from '../services/docker'
import { errorMessage } from '../services/errors'
import { getRecordingSettings, startFullRecording, type RecordingSettings } from '../services/recordings'
import { createCredentialPromptRuleGateway, type CredentialPromptRuleRecord } from '../services/credentialPromptRules'
import { createProxyGateway, proxyErrorMessage, type ProxyRecord } from '../services/proxies'
import type { CredentialPromptRule, PromptCredential } from '../features/sessions/credentialAutofill'
import { useAssetState } from '../features/hosts/useAssetState'
import { useAssetDeletion } from '../features/hosts/useAssetDeletion'
import { useGroupEditor } from '../features/hosts/useGroupEditor'
import { useHostEditor } from '../features/hosts/useHostEditor'
import { useSettingsState } from '../features/settings/useSettingsState'

import { isMacPlatform as isMac, refreshIsMacPlatform } from '../lib/platform'
import type { SettingsCategory } from '../features/settings'
import { createPortForwardGateway, type PortForwardConfig } from '../services/forwards'
import type { OtpEntry } from '../services/otp'
import { isTranslateProvider, type TranslateProvider } from '../services/translate'
import {
  SIDEBAR_WIDTH_MAX,
  SIDEBAR_WIDTH_MIN,
  DEFAULT_SETTINGS,
  normalizeSettings,
  RDP_FIRST_FRAME_TIMEOUT_MAX,
  RDP_FIRST_FRAME_TIMEOUT_MIN,
  RDP_FIT_GRACE_MAX,
  RDP_FIT_GRACE_MIN,
  RDP_HOST_PROBE_TIMEOUT_MAX,
  RDP_HOST_PROBE_TIMEOUT_MIN,
  TAB_DOUBLE_CLICK_BEHAVIORS,
  TERMINAL_SCROLLBACK_MAX,
  TERMINAL_SCROLLBACK_MIN,
  TOOL_PANEL_WIDTH_MAX,
  TOOL_PANEL_WIDTH_MIN,
  UI_SCALE_OPTIONS,
  type AssetLibraryLayout,
  type HostColorStyle,
  type SftpDirectoryFollowMode,
  type SftpDragFolderUploadMode,
  type TabDoubleClickBehavior,
  type TerminalGutterMode,
  type TerminalGutterTimestampFormat,
  type TerminalRenderEngine,
} from '../features/settings/settingsStore'
import {
  hostIconLabel as getHostIconLabel,
  hostIconTitle as getHostIconTitle,
  sessionIconLabel as getSessionIconLabel,
} from '../features/hosts'
import type { SessionTab, ToolPanel } from '../types/qterm'
import { isSupportedLocale, setLocale, useI18n } from '../locales'
import { isSettingsWindowMode, openSettingsWindow, showCurrentWindow } from '../services/sessionWindows'
import { hasWailsRuntime, whenWailsRuntime } from '../services/runtime'
import { readOpenSessions, writeOpenSessions } from '../services/sessionRestore'
import { setQuitConfirm } from '../services/quitConfirm'

export interface QtermShellOptions {
  openRdpTab?: (host: HostRecord) => void
}

export function useQtermShell(options: QtermShellOptions = {}) {
  const { t } = useI18n()
  const hostGateway = createHostGateway()
  const sessionGateway = createSessionGateway()
  const sftpGateway = createSftpGateway()

  const dockerGateway = createDockerGateway()

  const credentialPromptRuleGateway = createCredentialPromptRuleGateway()

  const proxyGateway = createProxyGateway()
  const proxies = ref<ProxyRecord[]>([])
  const proxiesLoading = ref(false)
  const proxiesError = ref('')

  async function loadProxies(): Promise<void> {
    proxiesLoading.value = true
    proxiesError.value = ''
    try {
      proxies.value = await proxyGateway.list()
    } catch (error) {
      proxies.value = []
      proxiesError.value = proxyErrorMessage(error, t('settings.proxies.error.load'))
    } finally {
      proxiesLoading.value = false
    }
  }

  const recordingSettings = ref<RecordingSettings>({ configured: '', effective: '', default: '' })

  async function loadRecordingSettings(): Promise<void> {
    try {
      recordingSettings.value = await getRecordingSettings()
    } catch {}
  }

  const credentialPromptRules = ref<CredentialPromptRule[]>([])
  const credentialPromptRulesError = ref('')

  async function loadCredentialPromptRules(): Promise<void> {
    try {
      const rules: CredentialPromptRuleRecord[] = await credentialPromptRuleGateway.list()
      credentialPromptRules.value = rules
        .filter((rule) => rule.enabled)
        .map((rule) => ({ id: rule.id, kind: rule.kind, pattern: rule.pattern, enabled: rule.enabled }))
      credentialPromptRulesError.value = ''
    } catch (error) {
      credentialPromptRules.value = []
      credentialPromptRulesError.value = errorMessage(error, t('settings.credentialPrompt.error.load'))
    }
  }

  const credentialPromptOptions = computed<PromptCredential[]>(() =>
    credentialOptions.value.map((item) => ({
      id: item.id,
      name: item.name,
      kind: item.kind,
      username: item.username,
    })),
  )

  async function typeCredentialIntoSession(sessionId: string, credentialId: string): Promise<void> {
    await credentialPromptRuleGateway.typeIntoSession(sessionId, credentialId)
  }
  const activeTool = ref<ToolPanel | null>('sftp')
  const sidebarCollapsed = ref(false)
  const credentialState = useCredentialState()
  const {
    credentialGateway,
    credentialOptions,
    credentialVaultStatus,
    credentialOptionsLoading,
    credentialOptionsError,
    securityDialogOpen,
    securityWorking,
    securityError,
    openSecuritySettings,
    closeSecuritySettings,
    refreshSecurityStatus,
    setupMasterPassword,
    unlockWithPassword,
    unlockAutomatically,
    changeMasterPassword,
    changeEncryptionKey,
    unlockWithCustomKey,
    setAutoUnlock,
    lockCredentialVault,
    exportRecovery,
    importRecovery,
    resetVault,
    refreshCredentials,
  } = credentialState
  const {
    settings,
    settingsReady,
    isDark,
    setThemeMode,
    toggleTheme,
    setTerminalFontSize,
    setTerminalLineHeight,
    setTerminalFontFamily,
    setTerminalTheme,
    setTerminalThemeBackgroundScope,
    setKeywordHighlightEnabled,
    setKeywordHighlightRules,
    resetTerminalTypography,
    shortcuts,
    setShortcut,
    resetShortcut,
  } = useSettingsState()
  const settingsCategory = ref<SettingsCategory>('application')
  const themeMode = computed(() => settings.themeMode)
  const minimalAppearance = computed(() => settings.minimalAppearance)
  const uiScale = computed(() => settings.uiScale ?? 1)

  const uiFont = computed(() => settings.uiFont ?? '')
  const showHostAddress = computed(() => settings.showHostAddress)

  const rdpShowResolution = computed(() => settings.rdpShowResolution ?? false)
  const rdpFirstFrameTimeoutSeconds = computed(() => settings.rdpFirstFrameTimeoutSeconds ?? 25)
  const rdpHostProbeTimeoutSeconds = computed(() => settings.rdpHostProbeTimeoutSeconds ?? 3)
  const rdpFitGraceSeconds = computed(() => settings.rdpFitGraceSeconds ?? 6)
  const rdpShowFps = computed(() => settings.rdpShowFps ?? false)
  const lightBackgroundEnabled = computed(() => settings.lightBackgroundEnabled)
  const lightBackgroundOpacity = computed(() => settings.lightBackgroundOpacity)
  const lightBackgroundImage = computed(() => settings.lightBackgroundImage)
  const lightBackgroundColor = computed(() => settings.lightBackgroundColor)
  const darkBackgroundEnabled = computed(() => settings.darkBackgroundEnabled)
  const darkBackgroundOpacity = computed(() => settings.darkBackgroundOpacity)
  const darkBackgroundImage = computed(() => settings.darkBackgroundImage)
  const darkBackgroundColor = computed(() => settings.darkBackgroundColor)
  const terminalFontSize = computed(() => settings.terminalFontSize)
  const terminalLineHeight = computed(() => settings.terminalLineHeight ?? 1.25)
  const terminalFontFamily = computed(() => settings.terminalFontFamily)
  const terminalLightTheme = computed(() => settings.terminalLightTheme ?? 'default-light')
  const terminalDarkTheme = computed(() => settings.terminalDarkTheme ?? 'default-dark')
  const terminalThemeBackgroundScope = computed(() => settings.terminalThemeBackgroundScope ?? 'terminal')
  const terminalAutoCopySelection = computed(() => settings.terminalAutoCopySelection ?? false)
  const terminalCopyFeedback = computed(() => settings.terminalCopyFeedback ?? true)
  const terminalShowLatency = computed(() => settings.terminalShowLatency ?? true)
  const terminalShowMetrics = computed(() => settings.terminalShowMetrics ?? false)

  const translateEnabled = computed(() => settings.translateEnabled ?? true)
  const translateTarget = computed(() => settings.translateTarget ?? 'zh-CN')
  const translateProvider = computed<TranslateProvider>(() => settings.translateProvider ?? 'google')
  const translateEndpoint = computed(() => settings.translateEndpoint ?? '')

  const otpEntries = computed<OtpEntry[]>(() => settings.otpEntries ?? [])

  const portForwards = computed<PortForwardConfig[]>(() => settings.portForwards ?? [])

  const portForwardGroups = computed<string[]>(() => settings.portForwardGroups ?? [])
  const forwardGateway = createPortForwardGateway()

  function openAutoForwards(host: HostRecord, origin: SessionOpenOrigin): void {
    if (origin === 'restore') return
    for (const forward of portForwards.value) {
      if (!forward.autoOpen || forward.hostId !== host.id) continue
      void forwardGateway.open(forward).catch(() => {})
    }
  }

  function closeHostForwards(hostId: string): void {
    for (const forward of portForwards.value) {
      if (forward.hostId !== hostId) continue
      void forwardGateway.close(forward.id).catch(() => {})
    }
  }

  const gutterMode = computed<TerminalGutterMode>(() => settings.terminalGutterMode ?? 'off')

  const gutterShowTimestamps = computed(() => settings.terminalGutterShowTimestamps ?? true)

  const gutterTimestampFormat = computed<TerminalGutterTimestampFormat>(
    () => settings.terminalGutterTimestampFormat ?? 'HH:mm:ss',
  )
  const terminalBatchCommand = computed(() => settings.terminalBatchCommand ?? false)
  const terminalConfirmClose = computed(() => settings.terminalConfirmClose ?? true)

  const terminalScrollback = computed(() => settings.terminalScrollback ?? 5000)

  const terminalConfirmMultiLinePaste = computed(() => settings.terminalConfirmMultiLinePaste ?? true)
  const terminalCursorBlink = computed(() => settings.terminalCursorBlink ?? false)
  const terminalRecordCommandHistory = computed(() => settings.terminalRecordCommandHistory ?? true)
  const terminalCompletionEnabled = computed(() => settings.terminalCompletionEnabled ?? true)
  const terminalHistoryMinLength = computed(() => settings.terminalHistoryMinLength ?? 1)
  const terminalHistoryMaxLength = computed(() => settings.terminalHistoryMaxLength ?? 4096)
  const terminalWordSeparators = computed(() => settings.terminalWordSeparators ?? ' ()[]{}\'"')

  const terminalMacOptionIsMeta = computed(() => settings.terminalMacOptionIsMeta ?? false)
  const terminalCursorStyle = computed(() => settings.terminalCursorStyle ?? 'block')
  const terminalRightClickBehavior = computed(() => settings.terminalRightClickBehavior ?? 'menu')

  const terminalModifierRightClickBehavior = computed(() => settings.terminalModifierRightClickBehavior ?? 'none')

  const modifierKeyLabel = computed(() => (isMac.value ? 'Command' : 'Ctrl'))
  const terminalDisconnectBehavior = computed(() => settings.terminalDisconnectBehavior ?? 'close')

  const terminalAutoReconnect = computed(() => settings.autoReconnect ?? false)
  const terminalRenderEngine = computed<TerminalRenderEngine>(() => settings.terminalRenderEngine ?? 'webgl')

  const showTabIndex = computed(() => settings.showTabIndex ?? false)

  const tabDoubleClickBehavior = computed<TabDoubleClickBehavior>(() => settings.tabDoubleClickBehavior ?? 'rename')

  const localShellPath = computed(() => settings.localShellPath ?? '')

  const localShellWorkingDirectory = computed(() => settings.localShellWorkingDirectory ?? '')
  const appLocale = computed(() => settings.locale ?? 'zh-CN')

  const assetLibraryLayout = computed<AssetLibraryLayout>(() => settings.assetLibraryLayout ?? 'sidebar')

  const hostColorStyle = computed<HostColorStyle>(() => settings.hostColorStyle ?? 'block')
  const sessionHistoryLimit = computed(() => settings.sessionHistoryLimit ?? 200)
  const recordAllSessions = computed(() => settings.recordAllSessions ?? false)
  const sftpDirectoryFollowMode = computed<SftpDirectoryFollowMode>(() => settings.sftpDirectoryFollowMode ?? 'remote')
  const sftpDragFolderUploadMode = computed<SftpDragFolderUploadMode>(
    () => settings.sftpDragFolderUploadMode ?? 'normal',
  )
  const sftpAutoDirectoryFollow = computed(() => settings.sftpAutoDirectoryFollow ?? true)
  const sftpShowHiddenFiles = computed(() => settings.sftpShowHiddenFiles ?? false)
  const x11Display = computed(() => settings.x11Display ?? '')
  const x11XauthPath = computed(() => settings.x11XauthPath ?? '')
  const x11AuthorityFile = computed(() => settings.x11AuthorityFile ?? '')
  const sftpMaxTransfers = computed(() => settings.sftpMaxTransfers ?? 3)
  const sftpLocalDirectory = computed(() => settings.sftpLocalDirectory ?? '')

  const recordingDirectory = computed(() => settings.recordingDirectory ?? '')

  const recordingDirectoryEffective = computed(() => recordingSettings.value.effective)
  const sftpEditorPath = computed(() => settings.sftpEditorPath ?? '')
  const windowRememberSize = computed(() => settings.windowRememberSize ?? true)
  const closeToTray = computed(() => settings.closeToTray ?? true)
  const restoreOpenSessions = computed(() => settings.restoreOpenSessions ?? true)
  const quitConfirm = computed(() => settings.quitConfirm ?? true)

  const settingsInWindow = computed(() => settings.settingsInWindow ?? true)
  const windowRememberPosition = computed(() => settings.windowRememberPosition ?? true)
  watch(appLocale, (value) => setLocale(value), { immediate: true })
  const keywordHighlightEnabled = computed(() => settings.keywordHighlightEnabled ?? false)
  const keywordHighlightRules = computed(() => settings.keywordHighlightRules ?? [])
  const sidebarWidth = computed(() => settings.layoutSidebarWidth ?? 224)
  const toolPanelWidth = computed(() => settings.layoutToolPanelWidth ?? 360)
  function setSidebarWidth(value: number): void {
    settings.layoutSidebarWidth = Math.min(SIDEBAR_WIDTH_MAX, Math.max(SIDEBAR_WIDTH_MIN, Math.round(value)))
  }
  function setToolPanelWidth(value: number): void {
    settings.layoutToolPanelWidth = Math.min(TOOL_PANEL_WIDTH_MAX, Math.max(TOOL_PANEL_WIDTH_MIN, Math.round(value)))
  }
  function setTerminalSetting(
    key:
      | 'terminalAutoCopySelection'
      | 'terminalCopyFeedback'
      | 'terminalShowLatency'
      | 'terminalShowMetrics'
      | 'terminalBatchCommand'
      | 'terminalConfirmClose'
      | 'terminalConfirmMultiLinePaste'
      | 'terminalCursorBlink'
      | 'terminalRecordCommandHistory'
      | 'terminalCompletionEnabled'
      | 'terminalMacOptionIsMeta'
      | 'showTabIndex'
      | 'autoReconnect',
    value: boolean,
  ): void {
    settings[key] = value === true
  }

  function setTerminalScrollback(value: number): void {
    if (!Number.isFinite(value)) return
    settings.terminalScrollback = Math.min(
      TERMINAL_SCROLLBACK_MAX,
      Math.max(TERMINAL_SCROLLBACK_MIN, Math.round(value)),
    )
  }

  function setTranslateSetting(
    key: 'translateEnabled' | 'translateTarget' | 'translateProvider' | 'translateEndpoint',
    value: boolean | string,
  ): void {
    if (key === 'translateEnabled') {
      settings.translateEnabled = value === true
      return
    }
    const text = typeof value === 'string' ? value : ''
    if (key === 'translateProvider') {
      if (isTranslateProvider(text)) settings.translateProvider = text
      return
    }
    if (key === 'translateTarget') {
      if (text.trim()) settings.translateTarget = text.trim()
      return
    }
    settings.translateEndpoint = text.trim()
  }
  function setOtpEntries(value: OtpEntry[]): void {
    settings.otpEntries = Array.isArray(value) ? normalizeSettings({ otpEntries: value }).otpEntries : []
  }
  function setPortForwards(value: PortForwardConfig[]): void {
    settings.portForwards = Array.isArray(value) ? normalizeSettings({ portForwards: value }).portForwards : []
  }
  function setPortForwardGroups(value: string[]): void {
    settings.portForwardGroups = Array.isArray(value)
      ? normalizeSettings({ portForwardGroups: value }).portForwardGroups
      : []
  }
  function setGutterMode(value: TerminalGutterMode): void {
    if (value === 'off' || value === 'lines' || value === 'commands') settings.terminalGutterMode = value
  }
  function setGutterShowTimestamps(value: boolean): void {
    settings.terminalGutterShowTimestamps = value === true
  }
  function setGutterTimestampFormat(value: TerminalGutterTimestampFormat): void {
    const trimmed = String(value ?? '')
      .trim()
      .slice(0, 40)
    settings.terminalGutterTimestampFormat = trimmed.length > 0 ? trimmed : 'HH:mm:ss'
  }
  function setTerminalNumberSetting(key: 'terminalHistoryMinLength' | 'terminalHistoryMaxLength', value: number): void {
    const parsed = Number(value)
    if (!Number.isFinite(parsed)) return
    const rounded = Math.max(0, Math.min(4096, Math.round(parsed)))
    if (key === 'terminalHistoryMinLength')
      settings.terminalHistoryMinLength = Math.min(rounded, settings.terminalHistoryMaxLength ?? 4096)
    else settings.terminalHistoryMaxLength = Math.max(rounded, settings.terminalHistoryMinLength ?? 1)
  }
  function setTerminalWordSeparators(value: string): void {
    settings.terminalWordSeparators = Array.from(value ?? '')
      .slice(0, 64)
      .join('')
  }
  function setTerminalCursorStyle(value: 'block' | 'underline' | 'bar'): void {
    if (['block', 'underline', 'bar'].includes(value)) settings.terminalCursorStyle = value
  }
  function setTerminalRightClickBehavior(value: 'none' | 'paste' | 'menu'): void {
    if (['none', 'paste', 'menu'].includes(value)) settings.terminalRightClickBehavior = value
  }
  function setTerminalModifierRightClickBehavior(value: 'none' | 'paste' | 'menu'): void {
    if (['none', 'paste', 'menu'].includes(value)) settings.terminalModifierRightClickBehavior = value
  }
  function setTerminalDisconnectBehavior(value: 'reconnect' | 'close'): void {
    if (value === 'reconnect' || value === 'close') settings.terminalDisconnectBehavior = value
  }
  function setTerminalRenderEngine(value: TerminalRenderEngine): void {
    if (value === 'webgl' || value === 'dom') settings.terminalRenderEngine = value
  }

  function setTabDoubleClickBehavior(value: TabDoubleClickBehavior): void {
    if (TAB_DOUBLE_CLICK_BEHAVIORS.includes(value)) settings.tabDoubleClickBehavior = value
  }

  function setLocalShellSetting(key: 'localShellPath' | 'localShellWorkingDirectory', value: string): void {
    settings[key] = String(value ?? '').trim()
  }
  function setAppSetting(
    key:
      | 'locale'
      | 'assetLibraryLayout'
      | 'hostColorStyle'
      | 'sessionHistoryLimit'
      | 'recordAllSessions'
      | 'sftpDirectoryFollowMode'
      | 'sftpDragFolderUploadMode'
      | 'sftpAutoDirectoryFollow'
      | 'sftpShowHiddenFiles'
      | 'sftpMaxTransfers'
      | 'sftpLocalDirectory'
      | 'sftpEditorPath'
      | 'x11Display'
      | 'x11XauthPath'
      | 'x11AuthorityFile'
      | 'recordingDirectory'
      | 'windowRememberSize'
      | 'closeToTray'
      | 'restoreOpenSessions'
      | 'quitConfirm'
      | 'settingsInWindow'
      | 'windowRememberPosition',
    value: string | boolean | number,
  ): void {
    if (key === 'locale') {
      if (isSupportedLocale(value)) {
        settings.locale = value
        setLocale(value)
      }
      return
    }
    if (key === 'assetLibraryLayout') {
      if (value === 'sidebar' || value === 'workspace') settings.assetLibraryLayout = value
      return
    }
    if (key === 'hostColorStyle') {
      if (value === 'block' || value === 'background') settings.hostColorStyle = value
      return
    }
    if (key === 'sessionHistoryLimit') {
      settings.sessionHistoryLimit = Math.min(10_000, Math.max(20, Math.round(Number(value) || 200)))
      return
    }
    if (key === 'recordAllSessions') {
      settings.recordAllSessions = value === true
      return
    }
    if (key === 'sftpDirectoryFollowMode') {
      if (value === 'session' || value === 'remote' || value === 'disabled') {
        settings.sftpDirectoryFollowMode = value
      }
      return
    }
    if (key === 'sftpDragFolderUploadMode') {
      if (value === 'normal' || value === 'archive') {
        settings.sftpDragFolderUploadMode = value
      }
      return
    }
    if (
      key === 'sftpAutoDirectoryFollow' ||
      key === 'sftpShowHiddenFiles' ||
      key === 'windowRememberSize' ||
      key === 'windowRememberPosition' ||
      key === 'closeToTray' ||
      key === 'restoreOpenSessions' ||
      key === 'quitConfirm' ||
      key === 'settingsInWindow'
    ) {
      settings[key] = value === true
      return
    }
    if (key === 'sftpMaxTransfers') {
      settings.sftpMaxTransfers = Math.min(16, Math.max(1, Math.round(Number(value) || 3)))
      return
    }
    if (
      key === 'sftpLocalDirectory' ||
      key === 'sftpEditorPath' ||
      key === 'recordingDirectory' ||
      key === 'x11Display' ||
      key === 'x11XauthPath' ||
      key === 'x11AuthorityFile'
    ) {
      settings[key] = String(value).trim()
      return
    }
  }
  function setShowHostAddress(value: boolean): void {
    settings.showHostAddress = value === true
  }
  function setRdpShowResolution(value: boolean): void {
    settings.rdpShowResolution = value === true
  }
  function setRdpShowFps(value: boolean): void {
    settings.rdpShowFps = value === true
  }

  function setRdpFirstFrameTimeoutSeconds(value: number | undefined): void {
    settings.rdpFirstFrameTimeoutSeconds =
      typeof value === 'number' && Number.isFinite(value)
        ? Math.min(RDP_FIRST_FRAME_TIMEOUT_MAX, Math.max(RDP_FIRST_FRAME_TIMEOUT_MIN, Math.round(value)))
        : (DEFAULT_SETTINGS.rdpFirstFrameTimeoutSeconds ?? 25)
  }
  function setRdpHostProbeTimeoutSeconds(value: number | undefined): void {
    settings.rdpHostProbeTimeoutSeconds =
      typeof value === 'number' && Number.isFinite(value)
        ? Math.min(RDP_HOST_PROBE_TIMEOUT_MAX, Math.max(RDP_HOST_PROBE_TIMEOUT_MIN, Math.round(value)))
        : (DEFAULT_SETTINGS.rdpHostProbeTimeoutSeconds ?? 3)
  }

  function setRdpFitGraceSeconds(value: number | undefined): void {
    settings.rdpFitGraceSeconds =
      typeof value === 'number' && Number.isFinite(value)
        ? Math.min(RDP_FIT_GRACE_MAX, Math.max(RDP_FIT_GRACE_MIN, Math.round(value)))
        : (DEFAULT_SETTINGS.rdpFitGraceSeconds ?? 6)
  }
  function setMinimalAppearance(value: boolean): void {
    settings.minimalAppearance = value === true
  }

  function setUIScale(value: number): void {
    const scale = Number(value)
    settings.uiScale = UI_SCALE_OPTIONS.find((option) => Math.abs(option - scale) < 1e-6) ?? 1
  }

  function setUIFont(value: string): void {
    settings.uiFont = typeof value === 'string' ? value.trim().slice(0, 128) : ''
  }
  function setBackgroundSetting(
    key:
      | 'lightBackgroundEnabled'
      | 'lightBackgroundOpacity'
      | 'lightBackgroundImage'
      | 'lightBackgroundColor'
      | 'darkBackgroundEnabled'
      | 'darkBackgroundOpacity'
      | 'darkBackgroundImage'
      | 'darkBackgroundColor',
    value: boolean | number | string,
  ): void {
    switch (key) {
      case 'lightBackgroundEnabled':
        settings.lightBackgroundEnabled = value === true
        break
      case 'lightBackgroundOpacity':
        settings.lightBackgroundOpacity = Number(value)
        break
      case 'lightBackgroundImage':
        settings.lightBackgroundImage = String(value)
        break
      case 'lightBackgroundColor':
        settings.lightBackgroundColor = String(value)
        break
      case 'darkBackgroundEnabled':
        settings.darkBackgroundEnabled = value === true
        break
      case 'darkBackgroundOpacity':
        settings.darkBackgroundOpacity = Number(value)
        break
      case 'darkBackgroundImage':
        settings.darkBackgroundImage = String(value)
        break
      case 'darkBackgroundColor':
        settings.darkBackgroundColor = String(value)
        break
    }
  }

  const settingsLocked = computed(() =>
    ['setupRequired', 'recoveryRequired'].includes(credentialVaultStatus.value?.state ?? ''),
  )

  watch(settingsLocked, (locked) => {
    if (locked) settingsCategory.value = 'application'
  })

  const SETTINGS_CATEGORY_SET: Record<SettingsCategory, true> = {
    credentials: true,
    knownHosts: true,
    ai: true,
    proxies: true,
    portForwards: true,
    application: true,
    appearance: true,
    terminal: true,
    snippets: true,
    shortcuts: true,
    history: true,
    recordings: true,
    highlight: true,
    credentialPrompt: true,
    rdp: true,
    about: true,
  }

  function isSettingsCategory(value: string | null): value is SettingsCategory {
    return value !== null && Object.prototype.hasOwnProperty.call(SETTINGS_CATEGORY_SET, value)
  }

  async function openSettings(category: SettingsCategory = 'application'): Promise<void> {
    if (settingsInWindow.value && !isSettingsWindowMode() && sessionGateway.persistent) {
      try {
        await openSettingsWindow(t('shell.toolbar.settings'), category)
        return
      } catch {}
    }
    settingsCategory.value = category
    await openSecuritySettings()
  }

  const assetState = useAssetState(hostGateway)
  const {
    hosts,
    groups,
    selectedHostId,
    selectedGroupId,
    searchVisible,
    searchQuery,
    sortMode,
    hostLoadError,
    loadingHosts,
    ungroupedHosts,
    groupedHosts,
    rootEntries,
    loadHosts,
    toggleSort,
  } = assetState

  async function resourcesImported(): Promise<void> {
    await Promise.all([refreshCredentials(), assetState.loadHosts(), loadCredentialPromptRules()])
    if (typeof window !== 'undefined') window.dispatchEvent(new CustomEvent('qterm:assets-imported'))
  }

  const sessionState = useSessionState({
    hostGateway,
    sessionGateway,
    hosts,
    selectedHostId,
    autoReconnect: terminalAutoReconnect,

    localShellPath,
    localShellWorkingDirectory,
    onSessionOpened: openAutoForwards,

    onHostDetached: closeHostForwards,
  })
  const {
    sessions,
    activeSessionId,
    activeSession,
    activeHost,
    pendingHostKey,
    pendingAuthentication,
    authenticationRetry,
    cancelAuthentication,
    submittingAuthentication,
    confirmingHostKey,
    hostKeyError,
    sessionActionError,
    canReconnect,
    canDisconnect,
    openSession,
    openSessionWithResult,
    adoptSession,
    detachSession,
    openLocalSession,
    closeSession,
    reorderSessions,
    disconnectActiveSession,
    reconnectActiveSession,
    showHostKeyChallenge,
    trustPendingHostKey,
    rejectPendingHostKey,
    submitAuthentication,
    handleTerminalStreamStatus,
    renameHostSessions,
    renameSession,
  } = sessionState

  function handleTerminalStreamStatusWithRecording(value: {
    sessionId: string
    state: 'ready' | 'closed' | 'failed'
    message?: string
  }): void {
    handleTerminalStreamStatus(value)
    if (value.state === 'ready' && recordAllSessions.value && sessionGateway.persistent) {
      startFullRecording(value.sessionId).catch(() => undefined)
    }
  }

  const terminalSessions = computed(() =>
    sessions.value.map((session) => ({
      ...session,
      host: hosts.value.find((host) => host.id === session.hostId),
    })),
  )

  const activeToolTitle = computed(() => {
    const titles: Record<ToolPanel, string> = {
      resources: t('shell.toolRail.resources'),
      snippets: t('shell.toolRail.snippets'),
      sftp: t('shell.toolRail.sftp'),
      tunnels: t('shell.toolRail.tunnels'),
      otp: t('shell.toolRail.otp'),
      processes: t('shell.monitor.tabProcesses'),
      services: t('shell.services.title'),
      history: t('shell.toolRail.history'),
      themes: t('shell.toolRail.themesTitle'),
      docker: t('shell.docker.title'),
      ai: t('shell.aiChat.title'),
    }
    return activeTool.value ? titles[activeTool.value] : ''
  })

  const activeSessionStatus = computed(() => {
    const session = activeSession.value
    if (!session) return { label: '', tone: 'idle' }

    if (session.autoReconnectPending === true)
      return { label: t('shell.terminal.autoReconnectPending'), tone: 'active' }
    if (session.kind === 'local') return localSessionStatus(session)
    if (session.state === 'preview') return { label: t('shell.status.preview'), tone: 'idle' }

    const telnet = session.kind === 'telnet'
    if (session.streamState === 'ready') {
      return { label: t(telnet ? 'shell.status.telnetReady' : 'shell.status.sshReady'), tone: 'connected' }
    }
    if (session.streamState === 'opening') return { label: t('shell.status.openingRemote'), tone: 'active' }
    if (session.streamState === 'failed')
      return { label: session.error || t('shell.status.channelFailed'), tone: 'error' }
    const labels: Record<Exclude<SessionState, 'preview'>, { label: string; tone: string }> = {
      connecting: { label: t('shell.status.connecting'), tone: 'active' },
      awaitingAuth: { label: t('shell.status.awaitingAuth'), tone: 'active' },
      hostKeyPending: { label: t('shell.status.hostKeyPending'), tone: 'warning' },
      authenticating: { label: t('shell.status.authenticating'), tone: 'active' },
      connected: {
        label: t(telnet ? 'shell.status.telnetConnected' : 'shell.status.authenticated'),
        tone: 'connected',
      },
      closing: { label: t('shell.status.closing'), tone: 'idle' },
      closed: { label: t('shell.status.closed'), tone: 'idle' },
      failed: {
        label: session.error || t(telnet ? 'shell.status.telnetFailed' : 'shell.status.sshFailed'),
        tone: 'error',
      },
    }
    return labels[session.state]
  })

  function localSessionStatus(session: SessionTab): { label: string; tone: string } {
    if (session.state === 'preview') return { label: t('shell.status.preview'), tone: 'idle' }
    if (session.streamState === 'ready') return { label: t('shell.status.localReady'), tone: 'connected' }
    if (session.streamState === 'failed')
      return { label: session.error || t('shell.status.localChannelFailed'), tone: 'error' }
    if (session.state === 'failed') return { label: session.error || t('shell.status.localShellFailed'), tone: 'error' }
    if (session.state === 'closing') return { label: t('shell.status.closingLocal'), tone: 'idle' }
    if (session.state === 'closed') return { label: t('shell.status.localClosed'), tone: 'idle' }
    if (session.state === 'connected') return { label: t('shell.status.startingLocal'), tone: 'active' }
    return { label: t('shell.status.startingShell'), tone: 'active' }
  }

  const SSH_ONLY_TOOL_PANELS: readonly ToolPanel[] = ['resources', 'sftp', 'tunnels', 'docker', 'processes', 'services']

  const sshCapable = computed(() => activeSession.value?.kind === 'ssh')

  const ADHOC_UNSUPPORTED_TOOL_PANELS: readonly ToolPanel[] = ['docker']

  const unsupportedToolPanels = computed<readonly ToolPanel[]>(() =>
    isAdHocHostId(activeSession.value?.hostId) ? ADHOC_UNSUPPORTED_TOOL_PANELS : [],
  )

  function canOpenToolPanel(tool: ToolPanel): boolean {
    if (activeSession.value?.kind === 'local') return tool === 'snippets' || tool === 'themes'
    if (!sshCapable.value && SSH_ONLY_TOOL_PANELS.includes(tool)) return false
    if (unsupportedToolPanels.value.includes(tool)) return false
    return true
  }

  const sftpAvailable = computed(
    () =>
      sessionGateway.persistent &&
      sshCapable.value &&
      activeSession.value?.state === 'connected' &&
      !!activeSession.value.id,
  )

  const hostEditor = useHostEditor({
    hostGateway,
    sessionGateway,
    credentials: credentialState,
    onSaved: async (host) => {
      assetState.upsertHost(host)
      renameHostSessions(host.id, host.name)
      await assetState.loadHosts()
    },
  })
  const {
    hostDialogOpen,
    editingHostId,
    formError,
    savingHost,
    testingConnection,
    detectHostIcon,
    detectingIcon,
    authMode,
    directSecret,
    draft,
    openCreateDialog,
    openEditHostDialog,
    credentialKindLabel,
    closeHostDialog,
    setAuthMode,
    setPasswordSource,
    saveHost,
    testHostConnection,
  } = hostEditor

  const groupEditor = useGroupEditor({ hostGateway, onSaved: assetState.loadHosts })
  const {
    groupDialogOpen,
    editingGroupId,
    groupName,
    groupFormError,
    savingGroup,
    openCreateGroupDialog: openCreateGroupDialogBase,
    openEditGroupDialog,
    closeGroupDialog: closeGroupDialogBase,
    saveGroup: saveGroupBase,
  } = groupEditor

  const selectCreatedGroupForHost = ref(false)

  function openCreateGroupDialog(parentId?: string): void {
    selectCreatedGroupForHost.value = false
    openCreateGroupDialogBase(parentId)
  }

  function openCreateGroupForHost(): void {
    selectCreatedGroupForHost.value = true
    openCreateGroupDialogBase()
  }

  function closeGroupDialog(): void {
    selectCreatedGroupForHost.value = false
    closeGroupDialogBase()
  }

  async function saveGroup(): Promise<void> {
    const group = await saveGroupBase()

    if (group && selectCreatedGroupForHost.value) {
      draft.value.group = group.name
    }
    selectCreatedGroupForHost.value = false
  }

  async function handleCredentialCreated(credentialId: string): Promise<void> {
    await refreshCredentials()
    draft.value.credentialId = credentialId
  }

  async function handleProxyCreated(proxyId: string): Promise<void> {
    await loadProxies()
    draft.value.proxyId = proxyId
    draft.value.proxyMode = 'none'
    draft.value.proxyHost = ''
    draft.value.proxyPort = 0
    draft.value.proxyUsername = ''
    draft.value.proxyPassword = ''
    draft.value.clearProxyPassword = false
    draft.value.proxyPasswordConfigured = false
  }
  const deletion = useAssetDeletion({
    hostGateway,
    closeHostSessions: sessionState.closeHostSessions,
    onDeleted: assetState.loadHosts,
  })
  const {
    pendingDelete,
    deleteError,
    deletingAsset,
    requestDeleteHost,
    requestDeleteGroup,
    closeDeleteDialog,
    confirmDelete,
  } = deletion

  async function openHost(host: HostRecord): Promise<void> {
    const protocol = hostProtocolDefinition(host.protocol).id

    if (protocol !== 'rdp' && protocol !== 'vnc') {
      await openSession(host)
      return
    }

    if (options.openRdpTab) {
      options.openRdpTab(host)
      return
    }
    try {
      await openRdpWindow(host.id, host.name)
    } catch (error) {
      sessionActionError.value = sessionErrorMessage(error, t('shell.error.openSession'))
    }
  }

  let restoreAttempted = false
  async function restoreOpenSessionsIfNeeded(): Promise<void> {
    if (restoreAttempted) return
    restoreAttempted = true

    if (isSettingsWindowMode()) return
    if (!restoreOpenSessions.value || sessions.value.length > 0) return
    for (const hostId of readOpenSessions()) {
      const host = hosts.value.find((item) => item.id === hostId)
      if (!host) continue

      await openSession(host, 'restore')
    }
  }

  watch(
    [settingsReady, quitConfirm],
    ([ready, enabled]) => {
      if (ready) void setQuitConfirm(enabled)
    },
    { immediate: true },
  )

  watch(
    () => sessions.value.map((session) => session.hostId),
    (hostIds) => writeOpenSessions(hostIds),
    { flush: 'post' },
  )

  watch(
    loadingHosts,
    (loading) => {
      if (!loading) void restoreOpenSessionsIfNeeded()
    },
    { immediate: true },
  )

  function openSelectedHost(): void {
    const selected = hosts.value.find((host) => host.id === selectedHostId.value)
    if (selected) void openHost(selected)
    else openCreateDialog()
  }

  function hostIconLabel(host: HostRecord): string {
    return getHostIconLabel(host)
  }

  function hostIconTitle(host: HostRecord): string {
    const title = getHostIconTitle(host)
    return title === '自定义图标' ? t('shell.hostIcon.custom') : title === '自动识别' ? t('shell.hostIcon.auto') : title
  }

  function sessionIconLabel(session: SessionTab): string {
    return getSessionIconLabel(session, hosts.value)
  }

  function hostSessionStatus(hostID: string): { tone: string; label: string } {
    const hostSessions = sessions.value.filter((session) => session.hostId === hostID)
    if (hostSessions.some((session) => session.state === 'connected')) {
      return { tone: 'connected', label: t('shell.status.connected') }
    }
    if (hostSessions.some((session) => session.state === 'hostKeyPending')) {
      return { tone: 'warning', label: t('shell.status.hostKeyPending') }
    }
    if (hostSessions.some((session) => ['connecting', 'awaitingAuth', 'authenticating'].includes(session.state))) {
      return { tone: 'active', label: t('shell.status.connecting') }
    }
    if (hostSessions.some((session) => session.state === 'failed')) {
      return { tone: 'error', label: t('shell.status.connectionError') }
    }
    return { tone: 'idle', label: t('shell.status.notConnected') }
  }

  function openToolPanel(tool: ToolPanel): void {
    if (!canOpenToolPanel(tool)) return
    activeTool.value = tool
  }

  function toggleTool(tool: ToolPanel): void {
    if (!canOpenToolPanel(tool)) return
    activeTool.value = activeTool.value === tool ? null : tool
  }

  watch(
    () => activeSession.value?.hostId,
    (hostId) => {
      if (!isAdHocHostId(hostId)) return
      if (activeTool.value && ADHOC_UNSUPPORTED_TOOL_PANELS.includes(activeTool.value)) activeTool.value = null
    },
  )

  watch(
    () => activeSession.value?.kind,
    (kind) => {
      if (activeTool.value === null) return
      if (kind === 'local') {
        if (activeTool.value !== 'snippets') activeTool.value = 'snippets'
        return
      }
      if (kind === 'telnet' && SSH_ONLY_TOOL_PANELS.includes(activeTool.value)) {
        activeTool.value = null
      }
    },
  )

  function hasOpenPopupLayer(): boolean {
    return (
      document.querySelector(
        '[data-slot="select-content"][data-state="open"], [data-slot="dropdown-menu-content"][data-state="open"], [role="listbox"], [role="menu"]',
      ) !== null
    )
  }

  function handleEscape(event: KeyboardEvent): void {
    if (event.key !== 'Escape' || event.defaultPrevented) return
    if (document.querySelector('[data-slot="dialog-content"][data-state="open"]') || hasOpenPopupLayer()) return
    if (pendingHostKey.value) void rejectPendingHostKey()
    else if (pendingDelete.value) closeDeleteDialog()
    else if (securityDialogOpen.value) closeSecuritySettings()
    else if (groupDialogOpen.value) closeGroupDialog()
  }

  let openHostTimer: number | undefined

  let cancelRuntimeRetry: (() => void) | undefined

  onMounted(() => {
    void loadCredentialPromptRules()
    void loadRecordingSettings()
    refreshIsMacPlatform()
    window.addEventListener('keydown', handleEscape)
    const params = new URLSearchParams(window.location.search)
    const transferredSessionID = params.get('openSession')
    if (isSettingsWindowMode()) {
      const requested = params.get('settingsCategory')
      const prewarmed = params.get('prewarm') === '1'

      const kind = params.get('kind') ?? 'settings'
      settingsCategory.value = isSettingsCategory(requested) ? requested : 'application'
      void (async () => {
        if (kind === 'settings') await openSecuritySettings().catch(() => undefined)
        try {
          await loadHosts()
        } catch {
        } finally {
          if (!prewarmed) await showCurrentWindow()
        }
      })()
    } else if (transferredSessionID) {
      void Promise.allSettled([
        adoptSession(transferredSessionID, params.get('openTitle') || undefined),
        loadHosts(),
      ]).then(() => showCurrentWindow())
    } else if (params.get('newWindow') === '1') {
      void loadHosts().finally(() => showCurrentWindow())
    } else if (params.get('openLocal') === '1') {
      void openLocalSession()
    } else {
      const hostID = params.get('openHost')
      if (!hostID) {
        void loadHosts()
      } else {
        void loadHosts().then(() => {
          const deadline = Date.now() + 5000
          let reloadedWithRuntime = false
          const openRequestedHost = (): void => {
            const host = hosts.value.find((item) => item.id === hostID)
            if (host) {
              if (openHostTimer) window.clearInterval(openHostTimer)
              openHostTimer = undefined
              void openSession(host)
              return
            }
            if (!reloadedWithRuntime && hostGateway.persistent) {
              reloadedWithRuntime = true
              void loadHosts()
              return
            }
            if (Date.now() >= deadline) {
              if (openHostTimer) window.clearInterval(openHostTimer)
              openHostTimer = undefined
              sessionActionError.value = t('shell.error.targetHostMissing')
            }
          }
          openRequestedHost()
          if (!openHostTimer && !hosts.value.some((item) => item.id === hostID)) {
            openHostTimer = window.setInterval(openRequestedHost, 100)
          }
        })
      }
    }
    void refreshSecurityStatus()

    void loadProxies()

    if (!hasWailsRuntime()) {
      cancelRuntimeRetry = whenWailsRuntime(() => {
        void refreshSecurityStatus()
        void loadCredentialPromptRules()
        void loadRecordingSettings()
        void loadHosts()

        void loadProxies()
      })
    }
  })

  onUnmounted(() => {
    if (openHostTimer) window.clearInterval(openHostTimer)
    cancelRuntimeRetry?.()
    cancelRuntimeRetry = undefined
    window.removeEventListener('keydown', handleEscape)
    sessionState.close()
  })

  return {
    hostGateway,
    sessionGateway,
    sftpGateway,
    dockerGateway,

    proxyGateway,
    proxies,
    proxiesLoading,
    proxiesError,
    loadProxies,
    hosts,
    groups,
    sessions,
    selectedHostId,
    selectedGroupId,
    activeSessionId,
    activeTool,

    unsupportedToolPanels,
    sidebarCollapsed,
    searchVisible,
    searchQuery,
    sortMode,
    toggleSort,
    hostDialogOpen,
    editingHostId,
    groupDialogOpen,
    editingGroupId,
    groupName,
    groupFormError,
    savingGroup,
    pendingDelete,
    deleteError,
    deletingAsset,
    formError,
    hostLoadError,
    loadingHosts,
    savingHost,
    testingConnection,
    detectingIcon,
    detectHostIcon,
    credentialOptions,
    credentialGateway,
    credentialPromptRuleGateway,
    credentialPromptRules,
    credentialPromptRulesError,
    loadCredentialPromptRules,
    credentialPromptOptions,
    typeCredentialIntoSession,
    credentialVaultStatus,
    credentialOptionsLoading,
    credentialOptionsError,
    authMode,
    directSecret,
    securityDialogOpen,
    securityWorking,
    securityError,
    settingsCategory,
    settingsLocked,
    themeMode,
    minimalAppearance,
    setMinimalAppearance,
    uiScale,
    setUIScale,
    uiFont,
    setUIFont,
    showHostAddress,
    setShowHostAddress,
    rdpShowResolution,
    setRdpShowResolution,
    rdpShowFps,
    setRdpShowFps,
    rdpFirstFrameTimeoutSeconds,
    setRdpFirstFrameTimeoutSeconds,
    rdpHostProbeTimeoutSeconds,
    setRdpHostProbeTimeoutSeconds,
    rdpFitGraceSeconds,
    setRdpFitGraceSeconds,
    lightBackgroundEnabled,
    lightBackgroundOpacity,
    lightBackgroundImage,
    lightBackgroundColor,
    darkBackgroundEnabled,
    darkBackgroundOpacity,
    darkBackgroundImage,
    darkBackgroundColor,
    setBackgroundSetting,
    terminalFontSize,
    terminalLineHeight,
    terminalFontFamily,
    settingsReady,
    terminalLightTheme,
    terminalDarkTheme,
    terminalThemeBackgroundScope,
    setTerminalTheme,
    setTerminalThemeBackgroundScope,
    keywordHighlightEnabled,
    keywordHighlightRules,
    setKeywordHighlightEnabled,
    setKeywordHighlightRules,

    shortcuts: computed(() => shortcuts),
    setShortcut,
    resetShortcut,
    terminalAutoCopySelection,
    terminalCopyFeedback,
    terminalShowLatency,
    terminalShowMetrics,
    translateEnabled,
    translateTarget,
    translateProvider,
    translateEndpoint,
    otpEntries,
    portForwards,
    portForwardGroups,
    gutterMode,
    gutterShowTimestamps,
    gutterTimestampFormat,
    terminalBatchCommand,
    terminalConfirmClose,
    terminalScrollback,
    terminalConfirmMultiLinePaste,
    terminalCursorBlink,
    terminalRecordCommandHistory,
    terminalCompletionEnabled,
    terminalHistoryMinLength,
    terminalHistoryMaxLength,
    terminalWordSeparators,
    terminalMacOptionIsMeta,
    terminalCursorStyle,
    terminalRightClickBehavior,
    terminalModifierRightClickBehavior,
    modifierKeyLabel,
    terminalDisconnectBehavior,
    terminalAutoReconnect,
    terminalRenderEngine,
    showTabIndex,
    tabDoubleClickBehavior,
    localShellPath,
    localShellWorkingDirectory,
    appLocale,
    assetLibraryLayout,
    hostColorStyle,
    sessionHistoryLimit,
    recordAllSessions,
    sftpDirectoryFollowMode,
    sftpDragFolderUploadMode,
    sftpAutoDirectoryFollow,
    sftpShowHiddenFiles,
    x11Display,
    x11XauthPath,
    x11AuthorityFile,
    sftpMaxTransfers,
    sftpLocalDirectory,
    sftpEditorPath,
    recordingDirectory,
    recordingDirectoryEffective,
    loadRecordingSettings,
    windowRememberSize,
    windowRememberPosition,
    closeToTray,
    restoreOpenSessions,
    quitConfirm,
    settingsInWindow,
    sidebarWidth,
    toolPanelWidth,
    setSidebarWidth,
    setToolPanelWidth,
    setTerminalSetting,
    setTerminalScrollback,
    setTranslateSetting,
    setOtpEntries,
    setPortForwards,
    setPortForwardGroups,
    setGutterMode,
    setGutterShowTimestamps,
    setGutterTimestampFormat,
    setTerminalNumberSetting,
    setTerminalWordSeparators,
    setTerminalCursorStyle,
    setTerminalRightClickBehavior,
    setTerminalModifierRightClickBehavior,
    setTerminalDisconnectBehavior,
    setTerminalRenderEngine,
    setTabDoubleClickBehavior,
    setLocalShellSetting,
    setAppSetting,
    setThemeMode,
    setTerminalFontSize,
    setTerminalLineHeight,
    setTerminalFontFamily,
    resetTerminalTypography,
    openSettings,
    isDark,
    isMac,
    pendingHostKey,
    pendingAuthentication,
    authenticationRetry,
    cancelAuthentication,
    submittingAuthentication,
    confirmingHostKey,
    hostKeyError,
    sessionActionError,
    draft,
    groupedHosts,
    ungroupedHosts,
    rootEntries,
    activeSession,
    terminalSessions,
    activeHost,
    activeToolTitle,
    activeSessionStatus,
    canReconnect,
    canDisconnect,
    sftpAvailable,
    credentialKindLabel,
    openCreateDialog,
    openEditHostDialog,
    setAuthMode,
    setPasswordSource,
    closeHostDialog,
    openSecuritySettings,
    closeSecuritySettings,
    setupMasterPassword,
    unlockWithPassword,
    unlockAutomatically,
    changeMasterPassword,
    changeEncryptionKey,
    unlockWithCustomKey,
    setAutoUnlock,
    lockCredentialVault,
    exportRecovery,
    importRecovery,
    resetVault,
    refreshCredentials,
    resourcesImported,
    loadHosts,
    saveHost,
    testHostConnection,
    openCreateGroupDialog,
    openCreateGroupForHost,
    openEditGroupDialog,
    closeGroupDialog,
    saveGroup,
    handleCredentialCreated,
    handleProxyCreated,
    requestDeleteHost,
    requestDeleteGroup,
    closeDeleteDialog,
    confirmDelete,
    openSession,
    openSessionWithResult,
    openHost,
    adoptSession,
    detachSession,
    openLocalSession,
    openSelectedHost,
    closeSession,
    reorderSessions,
    renameSession,
    disconnectActiveSession,
    reconnectActiveSession,
    hostIconLabel,
    hostIconTitle,
    sessionIconLabel,
    hostSessionStatus,
    showHostKeyChallenge,
    trustPendingHostKey,
    rejectPendingHostKey,
    submitAuthentication,
    handleTerminalStreamStatus: handleTerminalStreamStatusWithRecording,
    toggleTool,
    openToolPanel,
    toggleTheme,
  }
}
