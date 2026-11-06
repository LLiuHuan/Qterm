<script setup lang="ts">
import TransferConflictDialog from '../components/dialogs/TransferConflictDialog.vue'
import RemoteTextEditDialog from '../features/remoteedit/RemoteTextEditDialog.vue'
import RemoteEditConflictDialog from '../features/remoteedit/RemoteEditConflictDialog.vue'
import SftpTransferConflicts from '../components/dialogs/SftpTransferConflicts.vue'
import TransferProgressPanel from '../components/common/feedback/TransferProgressPanel.vue'
import { QtermToaster } from '../components/common'
import { requestRemoteDirectory } from '../services/transfer-conflicts'
import {
  setSessionWindowTitle,
  openNewWorkspaceWindow,
  normalizeCurrentWindowGeometry,
  restoreWindowGeometry,
  scheduleWindowGeometryPersist,
  flushWindowGeometry,
  toggleCurrentWindowMaximise,
  isSettingsWindowMode,
  connectSettingsWindowState,
  connectHostsChanged,
  setSettingsWindowEnabled,
  openDialogWindow,
  openHostEditorWindow,
  closeHostEditorWindow,
  focusSettingsWindowInWindow,
  hideSettingsWindow,
  type DialogWindowKind,
} from '../services/sessionWindows'
import { toast } from 'vue-sonner'
import { transferQueueAction, type TransferQueueAction } from '../services/transferQueue'
import { setCloseToTray, setTrayMenuState, subscribeTrayActions, type TrayAction } from '../services/tray'
import { hasWailsRuntime, whenWailsRuntime } from '../services/runtime'
import { Events } from '@wailsio/runtime'

import {
  DeleteAssetDialog,
  DeleteConfirmDialog,
  GroupEditorDialog,
  HostEditorDialog,
  HostKeyDialog,
  SSHAuthenticationDialog,
  SettingsDialog,
  ZmodemDialog,
} from '../components/dialogs'
import CommandPaletteDialog, { type PaletteItem } from '../components/dialogs/CommandPaletteDialog.vue'
import { AppToolbar, AssetLibraryView, AssetSidebar } from '../components/layout'
import AppLockScreen from '../features/applock/AppLockScreen.vue'
import { useAppLock } from '../features/applock/useAppLock'
import { SessionWorkspace, ToolPanel as ToolPanelView, ToolRail } from '../components/workspace'
import SessionWindowTitlebar from '../components/workspace/SessionWindowTitlebar.vue'
import type { RdpWorkspaceTab, WorkspaceTabOrderEntry } from '../components/workspace/SessionWorkspace.vue'
import { IconButton, ResizeHandle, SessionOverlay } from '../components/common'
import { PanelTop, SquareTerminal } from '@lucide/vue'
import SftpWorkspace from '../features/sftp/SftpWorkspace.vue'
import { useTransferState } from '../composables/useTransferState'
import { claimSession, claimTransfer, currentOwnership, filterOwnedItems } from '../services/transferOwnership'
import { useSessionWindowHandover } from '../composables/useSessionWindowHandover'
import { useQtermShell } from '../composables/useQtermShell'
import { hostErrorMessage, type HostRecord } from '../services/hosts'
import { isAdHocHostId } from '../services/adhocSession'
import QuickConnectDialog from '../components/dialogs/QuickConnectDialog.vue'
import AiChatDialog from '../components/dialogs/AiChatDialog.vue'
import ImportConfigDialog from '../components/dialogs/ImportConfigDialog.vue'
import ExportConfigDialog from '../components/dialogs/ExportConfigDialog.vue'
import {
  QUIT_REQUEST_EVENT,
  claimQuitConfirm,
  resolveQuitConfirm,
  setQuitConfirmLabels,
  type QuitRequestPayload,
} from '../services/quitConfirm'
import { readTerminalTail } from '../features/sessions/terminalTail'
import {
  cancelRdpReturnToMain,
  completeRdpReturnToMain,
  findRdpSession,
  openRdpSessionWindow,
  openRdpWindow,
} from '../services/remotedesktop'
import { openVncWindow } from '../services/vnc'
import type { SessionAdoptionRebindRequest, VncSessionAdoptRequest } from '../services/sessionAdoptionChannel'
import { setCommandHistoryEnabled, setCommandHistoryLimits } from '../services/command-history'
import { sftpErrorMessage } from '../services/sftp'
import { decodeAppError, translateAppError, translateErrorKey } from '../services/errors'
import { FILE_DROP_EVENT, resolveSelfWindowName, shouldHandleDrop } from '../services/fileDropOwnership'
import { computed, nextTick, ref, watch, onMounted, onBeforeUnmount } from 'vue'
import type { SessionTab, ToolPanel } from '../types/qterm'
import SnippetsSettingsPanel from '../features/settings/SnippetsSettingsPanel.vue'
import AssetPickerDialog, { type AssetChoice, type AssetPickerMode } from '../components/dialogs/AssetPickerDialog.vue'
import type { SplitDirection } from '../features/sessions/useSplitWorkspace'
import { useWindowTheme } from '../composables/useWindowTheme'
import { boundShortcutAction } from '../features/settings/shortcutStore'
import { SETTINGS_CATEGORIES, type SettingsCategory } from '../features/settings'
import { createShortcutDispatch, shortcutNeedsActiveTab } from '../features/settings/shortcutDispatch'
import { provideRemoteEditorFocus } from '../features/remoteedit/remoteEditorFocus'
import {
  closeRemoteTextEditor,
  dismissRemoteEditConflict,
  ensureRemoteEditsLoaded,
  reloadRemoteEditForConflict,
  syncRemoteEdit,
  useRemoteEdits,
} from '../features/remoteedit/useRemoteEdits'
import type { ConflictChoice } from '../features/remoteedit/remoteEditModel'
import { provideTerminalPaneCommandBus } from '../features/sessions/terminalPaneCommandBus'
import { useI18n } from '@/locales'
import { requestAILaunch, useAILaunch } from '../services/aiAgentLaunch'
import { registerSnippetCreator } from '../services/snippetCreator'
import { evenPixel, observeEvenDialogHeights } from '../lib/geometry'
import { waitForAvailable } from '../lib/async'
import RdpCanvasPane from '@/features/remotedesktop/RdpCanvasPane.vue'
import VncCanvasPane from '@/features/vnc/VncCanvasPane.vue'
const standaloneSnippetPanel = ref<InstanceType<typeof SnippetsSettingsPanel> | null>(null)

const workspace = ref<InstanceType<typeof SessionWorkspace> | null>(null)

interface SftpWorkspaceTab {
  id: string
  title: string

  hostId: string | null

  order: number
}

const closeConfirmIds = ref<string[]>([])
const { t } = useI18n()
const credentialView = ref<'credentials' | 'otp'>('credentials')
function openOtpManagement(): void {
  credentialView.value = 'otp'
  if (openDialogWindowIfEnabled('settings', t('shell.toolbar.settings'), 'credentials:otp')) return
  void openSettings('credentials')
}
const requestedSessionWindowTitle = new URLSearchParams(window.location.search).get('openTitle')
const sessionWindowTitle = computed(() => {
  const base = requestedSessionWindowTitle || t('shell.app.sessionWindowTitle')

  const address = activeSession.value
    ? hosts.value.find((host) => host.id === activeSession.value?.hostId)?.address
    : ''
  return address ? t('shell.app.sessionWindowWithHost', { title: base, address }) : base
})
const isSessionWindow =
  typeof window !== 'undefined' &&
  (new URLSearchParams(window.location.search).has('openHost') ||
    new URLSearchParams(window.location.search).has('openSession') ||
    new URLSearchParams(window.location.search).get('openLocal') === '1')
const isChildWorkspaceWindow =
  typeof window !== 'undefined' && new URLSearchParams(window.location.search).get('newWindow') === '1'

const isSettingsWindow = isSettingsWindowMode()

const dialogWindowParams = new URLSearchParams(window.location.search)
const dialogWindowKind = ref<DialogWindowKind>(
  (dialogWindowParams.get('kind') as DialogWindowKind | null) ?? 'settings',
)
const dialogWindowArg = ref(dialogWindowParams.get('settingsCategory') ?? '')

const dialogWindowTitle = computed(() => {
  if (dialogWindowKind.value === 'import') return t('shell.importConfig.title')
  if (dialogWindowKind.value === 'export') return t('shell.exportConfig.title')
  return t('shell.toolbar.settings')
})

const isMainWindow = !isSessionWindow && !isChildWorkspaceWindow && !isSettingsWindow
onMounted(() => {
  if (isSessionWindow) void setSessionWindowTitle(sessionWindowTitle.value)
})

const {
  hostGateway,
  sessionGateway,
  sftpGateway,
  dockerGateway,
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
  rootEntries,
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
  credentialGateway,
  credentialPromptRuleGateway,
  credentialPromptRules,
  credentialPromptOptions,
  loadCredentialPromptRules,
  typeCredentialIntoSession,
  proxyGateway,
  proxies,
  proxiesLoading,
  proxiesError,
  loadProxies,
  credentialOptions,
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
  rdpFirstFrameTimeoutSeconds,
  setRdpFirstFrameTimeoutSeconds,
  rdpHostProbeTimeoutSeconds,
  setRdpHostProbeTimeoutSeconds,
  rdpFitGraceSeconds,
  setRdpFitGraceSeconds,
  rdpShowFps,
  setRdpShowFps,
  lightBackgroundEnabled,
  lightBackgroundOpacity,
  lightBackgroundImage,
  darkBackgroundEnabled,
  darkBackgroundOpacity,
  darkBackgroundImage,
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
  terminalAutoCopySelection,
  terminalCopyFeedback,
  terminalShowLatency,
  terminalShowMetrics,
  translateEnabled,
  translateTarget,
  translateProvider,
  translateEndpoint,
  setTranslateSetting,
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
  sftpMaxTransfers,
  sftpLocalDirectory,
  recordingDirectory,
  recordingDirectoryEffective,
  loadRecordingSettings,

  sftpEditorPath,
  x11Display,
  x11XauthPath,
  x11AuthorityFile,
  windowRememberSize,
  closeToTray,
  restoreOpenSessions,
  quitConfirm,
  settingsInWindow,
  windowRememberPosition,
  keywordHighlightEnabled,
  keywordHighlightRules,
  setKeywordHighlightEnabled,
  setKeywordHighlightRules,
  shortcuts,
  setShortcut,
  sidebarWidth,
  toolPanelWidth,
  setSidebarWidth,
  setToolPanelWidth,
  setTerminalSetting,
  setTerminalScrollback,
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
  renameSession,
  draft,
  groupedHosts,
  ungroupedHosts,
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
  closeSecuritySettings,
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
  openLocalSession,
  closeSession,
  detachSession,
  reorderSessions,
  disconnectActiveSession,
  reconnectActiveSession,
  sessionIconLabel,
  trustPendingHostKey,
  rejectPendingHostKey,
  submitAuthentication,
  handleTerminalStreamStatus,
  toggleTool,
  toggleTheme,
} = useQtermShell({ openRdpTab })

async function onRecordingDirectoryChange(value: string): Promise<void> {
  setAppSetting('recordingDirectory', value)
  await loadRecordingSettings()
}

const transferState = useTransferState(sftpGateway)

const quitConfirmOpen = ref(false)
const quitRequestId = ref(0)

const quitActiveSessions = ref(0)
const quitActiveForwards = ref(0)

const ACTIVE_TRANSFER_STATES = new Set(['queued', 'awaitingOverwrite', 'running', 'paused'])

function hasActiveWork(): boolean {
  if (quitActiveSessions.value > 0) return true
  if (sessions.value.length > 0) return true
  if (transferState.records.value.some((record) => ACTIVE_TRANSFER_STATES.has(record.state))) return true
  return quitActiveForwards.value > 0
}

function handleQuitRequest(payload: QuitRequestPayload | undefined): void {
  if (!payload || typeof payload.requestID !== 'number') return
  quitRequestId.value = payload.requestID
  quitActiveSessions.value = payload.activeSessions ?? 0
  quitActiveForwards.value = payload.activeForwards ?? 0
  if (!hasActiveWork()) {
    void resolveQuitConfirm(payload.requestID, true)
    return
  }

  void claimQuitConfirm(payload.requestID)
  quitConfirmOpen.value = true
}

function answerQuit(confirmed: boolean): void {
  quitConfirmOpen.value = false
  void resolveQuitConfirm(quitRequestId.value, confirmed)
}

function pushQuitConfirmLabels(): void {
  if (isSessionWindow || isChildWorkspaceWindow || isSettingsWindow) return
  void setQuitConfirmLabels({
    title: t('shell.quitConfirm.title'),
    message: t('shell.quitConfirm.message'),
    confirm: t('shell.quitConfirm.confirm'),
    cancel: t('shell.common.cancel'),
  })
}

watch(appLocale, pushQuitConfirmLabels, { immediate: true })

let stopQuitRequest: (() => void) | undefined
let stopTrayActions: (() => void) | undefined
let stopHostsChanged: (() => void) | undefined
let stopSettingsWindowState: (() => void) | undefined

const settingsWindowOpen = ref(false)
let cancelTrayRuntimeWait: (() => void) | undefined
let trayActionsSubscribed = false
let trayNativeReady = false
let trayInitializationRunning = false

const progressItems = computed(() => filterOwnedItems(transferState.items.value, currentOwnership(), isMainWindow))

const transferRecords = transferState.records
async function handleTransferAction(id: string, action?: TransferQueueAction): Promise<void> {
  const record = transferState.records.value.find((r) => r.id === id)
  if (!record) return
  try {
    switch (action ?? transferQueueAction(record, sftpGateway)) {
      case 'retry':
        await sftpGateway.retrySFTPTransfer(id)
        return
      case 'pause':
        await sftpGateway.pauseSFTPTransfer?.(id)
        return
      case 'resume':
        await sftpGateway.resumeSFTPTransfer?.(id)
        return
      default:
        await sftpGateway.cancelSFTPTransfer(id)
    }
  } catch {}
}

const { backgroundEnabled, pageBackgroundStyle } = useWindowTheme({
  isDark,
  lightBackgroundEnabled,
  lightBackgroundOpacity,
  lightBackgroundImage,
  darkBackgroundEnabled,
  darkBackgroundOpacity,
  darkBackgroundImage,
  terminalLightTheme,
  terminalDarkTheme,
  terminalThemeBackgroundScope,
})
const layoutSizeStyle = computed<Record<string, string>>(() => ({
  '--qt-sidebar-width': `${sidebarWidth.value}px`,
  '--qt-tool-panel-width': `${toolPanelWidth.value}px`,
}))
let sidebarResizeOrigin = sidebarWidth.value
let toolPanelResizeOrigin = toolPanelWidth.value

function beginSidebarResize(): void {
  sidebarResizeOrigin = sidebarWidth.value
}

function resizeSidebar(delta: number): void {
  setSidebarWidth(sidebarResizeOrigin + delta)
}

function beginToolPanelResize(): void {
  toolPanelResizeOrigin = toolPanelWidth.value
}

function resizeToolPanel(delta: number): void {
  setToolPanelWidth(toolPanelResizeOrigin - delta)
}

const sftpTabs = ref<SftpWorkspaceTab[]>([])
const activeSftpTabId = ref<string | null>(null)
let sftpTabSequence = 0

const rdpTabs = ref<RdpWorkspaceTab[]>([])
const activeRdpTabId = ref<string | null>(null)
let rdpTabSequence = 0

const adoptedRdpTabIds = new Set<string>()
const rdpAttachWaiters = new Map<
  string,
  { resolve: (attached: boolean) => void; timer: ReturnType<typeof setTimeout> }
>()

let tabOrderSequence = 0

function nextTabOrder(): number {
  const known = [
    tabOrderSequence,
    ...sessions.value.map((session) => session.order),
    ...sftpTabs.value.map((tab) => tab.order),
    ...rdpTabs.value.map((tab) => tab.order),
  ].filter((value): value is number => typeof value === 'number' && Number.isFinite(value))
  tabOrderSequence = (known.length ? Math.max(...known) : 0) + 1
  return tabOrderSequence
}

function sessionTabOrderValue(session: SessionTab): number {
  return typeof session.order === 'number' && Number.isFinite(session.order)
    ? session.order
    : sessions.value.indexOf(session)
}

watch(
  () => sessions.value.map((session) => session.id),
  () => {
    for (const session of sessions.value) {
      if (typeof session.order !== 'number' || !Number.isFinite(session.order)) session.order = nextTabOrder()
    }
  },
  { flush: 'sync', immediate: true },
)

function syncSessionArrayOrderWithTabs(): void {
  const ordered = [...sessions.value].sort((left, right) => sessionTabOrderValue(left) - sessionTabOrderValue(right))
  if (ordered.every((session, index) => sessions.value[index] === session)) return

  sessions.value.splice(0, sessions.value.length, ...ordered)
}

function applyTabOrder(entries: WorkspaceTabOrderEntry[]): void {
  for (const entry of entries) {
    if (entry.kind === 'sftp') {
      const tab = sftpTabs.value.find((item) => item.id === entry.id)
      if (tab) tab.order = entry.order
      continue
    }
    if (entry.kind === 'rdp') {
      const tab = rdpTabs.value.find((item) => item.id === entry.id)
      if (tab) tab.order = entry.order
      continue
    }
    const ids = entry.kind === 'group' ? (entry.sessionIds ?? []) : [entry.id]
    for (const id of ids) {
      const session = sessions.value.find((item) => item.id === id)
      if (session) session.order = entry.order
    }
  }
  syncSessionArrayOrderWithTabs()
}

const rdpPaneRefs = ref<Record<string, unknown>>({})

function registerRdpPane(tabId: string, pane: unknown): void {
  if (pane) {
    rdpPaneRefs.value[tabId] = pane
    return
  }
  delete rdpPaneRefs.value[tabId]
}

const activeRdpPane = computed(() => {
  const id = activeRdpTabId.value
  if (!id) return null
  return (rdpPaneRefs.value[id] ?? null) as DesktopPaneActions | null
})

const activeDesktopProtocol = computed<'rdp' | 'vnc' | null>(() => {
  const id = activeRdpTabId.value
  if (!id) return null
  const tab = rdpTabs.value.find((item) => item.id === id)
  if (!tab) return null
  return tab.protocol ?? 'rdp'
})

interface DesktopPaneActions {
  reconnect: () => Promise<void> | void

  disconnect: () => void

  sessionId: string
  statusTone?: 'idle' | 'connected' | 'error'

  openLog?: () => void

  pasteToRemote?: () => Promise<void> | void
  copyFromRemote?: () => Promise<void> | void

  fitEnabled?: boolean

  toggleFit?: () => void

  canPasteToRemote?: boolean
  canCopyFromRemote?: boolean

  beginHandover?: () => void

  cancelHandover?: () => void
}

function nextRdpTabId(): string {
  rdpTabSequence += 1
  return `rdp-tab-${Date.now().toString(36)}-${rdpTabSequence.toString(36)}`
}

function waitForRdpAttach(sessionId: string): Promise<boolean> {
  const id = sessionId.trim()
  if (!id) return Promise.resolve(false)
  const previous = rdpAttachWaiters.get(id)
  if (previous) {
    clearTimeout(previous.timer)
    previous.resolve(false)
  }
  return new Promise((resolve) => {
    const timer = setTimeout(() => {
      rdpAttachWaiters.delete(id)
      resolve(false)
    }, 5000)
    rdpAttachWaiters.set(id, { resolve, timer })
  })
}

function settleRdpAttach(sessionId: string, attached: boolean): void {
  const id = sessionId.trim()
  const waiter = rdpAttachWaiters.get(id)
  if (!waiter) return
  clearTimeout(waiter.timer)
  rdpAttachWaiters.delete(id)
  waiter.resolve(attached)
}

function handleRdpPaneOpened(tabId: string, sessionId: string): void {
  const tab = rdpTabs.value.find((item) => item.id === tabId)
  if (tab) tab.sessionId = sessionId
  settleRdpAttach(sessionId, true)

  void loadHosts()
}

function openRdpTab(host: HostRecord): void {
  assetLibraryVisible.value = false
  activeSftpTabId.value = null

  const protocol: 'rdp' | 'vnc' = host.protocol === 'vnc' ? 'vnc' : 'rdp'
  const existing = rdpTabs.value.find((tab) => tab.hostId === host.id && (tab.protocol ?? 'rdp') === protocol)
  if (existing) {
    activeRdpTabId.value = existing.id
    return
  }
  const tab: RdpWorkspaceTab = {
    id: nextRdpTabId(),
    title: host.name || host.address,
    hostId: host.id,
    protocol,

    order: nextTabOrder(),
  }
  rdpTabs.value.push(tab)
  activeRdpTabId.value = tab.id
}

async function adoptRdpSession(request: SessionAdoptionRebindRequest): Promise<boolean> {
  const sessionId = request.sessionId.trim()
  if (!sessionId) return false
  const existingSession = await findRdpSession(sessionId).catch(() => null)
  if (!existingSession) return false

  let tab = rdpTabs.value.find((item) => item.sessionId === sessionId)
  const isNewTab = !tab
  const attachResult = isNewTab ? waitForRdpAttach(sessionId) : null
  if (!tab) {
    tab = {
      id: nextRdpTabId(),
      title: request.title?.trim() || existingSession.host || 'RDP',
      sessionId,
      hostId: request.hostId?.trim() || undefined,

      order: nextTabOrder(),
    }
    rdpTabs.value.push(tab)

    adoptedRdpTabIds.add(tab.id)
  }
  selectRdpTab(tab.id)
  await nextTick()

  const pane = (rdpPaneRefs.value[tab.id] ?? null) as DesktopPaneActions | null

  if (pane && pane.sessionId === sessionId && pane.statusTone === 'connected') {
    try {
      await completeRdpReturnToMain(sessionId)
      settleRdpAttach(sessionId, true)
      return true
    } catch {
      settleRdpAttach(sessionId, false)
      await cancelRdpReturnToMain(sessionId).catch(() => undefined)
      closeRdpTab(tab.id)
      return false
    }
  }

  const attached = await (attachResult ?? waitForRdpAttach(sessionId))
  if (attached) return true
  await cancelRdpReturnToMain(sessionId).catch(() => undefined)
  closeRdpTab(tab.id)
  return false
}

function adoptVncTabRequest(request: VncSessionAdoptRequest): boolean {
  const hostId = request.hostId.trim()
  if (!hostId) return false
  const host = hosts.value.find((item) => item.id === hostId)

  if (!host) return false

  if (host.protocol !== 'vnc') return false
  openRdpTab(host)
  toast.success(t('shell.app.returnToTabsDone'), { id: `vnc-returned-${hostId}` })
  return true
}

function selectRdpTab(tabId: string): void {
  if (rdpTabs.value.some((tab) => tab.id === tabId)) {
    assetLibraryVisible.value = false
    activeSftpTabId.value = null
    activeRdpTabId.value = tabId
  }
}

async function openRdpInWindow(tab: RdpWorkspaceTab): Promise<void> {
  const pane = (rdpPaneRefs.value[tab.id] ?? null) as DesktopPaneActions | null
  const sessionId = pane?.sessionId ?? ''

  if (tab.protocol === 'vnc') {
    if (!tab.hostId) return
    try {
      await openVncWindow(tab.hostId, tab.title)
    } catch (error) {
      const decoded = decodeAppError(error)
      toast.error(decoded ? translateAppError(decoded, t('shell.vnc.openFailed')) : t('shell.vnc.openFailed'))
      return
    }

    closeRdpTab(tab.id)
    return
  }
  if (!pane?.beginHandover || !pane?.cancelHandover) {
    if (!tab.hostId) return
    void openRdpWindow(tab.hostId, tab.title)
    return
  }
  if (!sessionId) {
    if (!tab.hostId) return
    void openRdpWindow(tab.hostId, tab.title)
    return
  }
  pane.beginHandover()
  try {
    await openRdpSessionWindow(sessionId, tab.hostId ?? '', tab.title)
  } catch (error) {
    pane.cancelHandover()
    const decoded = decodeAppError(error)
    toast.error(
      decoded ? translateAppError(decoded, t('shell.rdp.windowHandoverFailed')) : t('shell.rdp.windowHandoverFailed'),
    )
    return
  }

  closeRdpTab(tab.id)
}

function openDesktopTabInWindow(tabId: string): void {
  const tab = rdpTabs.value.find((item) => item.id === tabId)
  if (!tab) return
  void openRdpInWindow(tab)
}

function openActiveDesktopInWindow(): void {
  const tabId = activeRdpTabId.value
  if (!tabId) return
  openDesktopTabInWindow(tabId)
}

function closeTabScope(payload: { sessionIds: string[]; sftpTabIds: string[]; rdpTabIds: string[] }): void {
  if (payload.sessionIds.length) closeSessions(payload.sessionIds)
  for (const id of payload.sftpTabIds) closeSftpTab(id)
  for (const id of payload.rdpTabIds) closeRdpTab(id)
}

function closeRdpTab(tabId: string): void {
  const index = rdpTabs.value.findIndex((tab) => tab.id === tabId)
  if (index < 0) return
  delete rdpPaneRefs.value[tabId]
  adoptedRdpTabIds.delete(tabId)
  const wasActive = activeRdpTabId.value === tabId
  rdpTabs.value.splice(index, 1)
  if (wasActive) {
    activeRdpTabId.value = rdpTabs.value[Math.min(index, rdpTabs.value.length - 1)]?.id ?? null
  }
}

function handleRdpConnectionFailed(tabId: string, message: string): void {
  const tab = rdpTabs.value.find((item) => item.id === tabId)
  if (!tab) return
  if (tab.sessionId) settleRdpAttach(tab.sessionId, false)
  if (!adoptedRdpTabIds.has(tabId)) return
  adoptedRdpTabIds.delete(tabId)
  toast.error(message || t('shell.rdp.connectionFailed'))

  suppressAssetLibraryAutoShow = true
  closeRdpTab(tabId)
  assetLibraryVisible.value = false
  void nextTick(() => {
    suppressAssetLibraryAutoShow = false
  })
}

function nextSftpTabId(): string {
  sftpTabSequence += 1
  return `sftp-tab-${Date.now().toString(36)}-${sftpTabSequence.toString(36)}`
}

function selectSessionTab(sessionId: string): void {
  assetLibraryVisible.value = false
  activeSftpTabId.value = null
  activeRdpTabId.value = null
  activeSessionId.value = sessionId
}

function returnToTerminalFromSftp(): void {
  const sessionId = activeSessionId.value ?? terminalSessions.value[0]?.id
  if (!sessionId) return
  selectSessionTab(sessionId)
}

function selectSftpTab(tabId: string): void {
  if (sftpTabs.value.some((tab) => tab.id === tabId)) {
    assetLibraryVisible.value = false
    activeRdpTabId.value = null
    activeSftpTabId.value = tabId
  }
}

function closeSftpTab(tabId: string): void {
  const index = sftpTabs.value.findIndex((tab) => tab.id === tabId)
  if (index < 0) return
  const wasActive = activeSftpTabId.value === tabId
  sftpTabs.value.splice(index, 1)
  if (wasActive) activeSftpTabId.value = null
}

function isSnippetPanelReady(
  panel: InstanceType<typeof SnippetsSettingsPanel> | null | undefined,
): panel is InstanceType<typeof SnippetsSettingsPanel> {
  return (
    !!panel &&
    typeof (panel as { openCreateDialog?: unknown }).openCreateDialog === 'function' &&
    typeof (panel as { openCreateDialogWithContent?: unknown }).openCreateDialogWithContent === 'function'
  )
}

const SNIPPET_PANEL_WAIT_MS = 1000
async function openSnippetCreator(content = ''): Promise<void> {
  const panel = await waitForAvailable(() => standaloneSnippetPanel.value, { timeoutMs: SNIPPET_PANEL_WAIT_MS })
  if (!isSnippetPanelReady(panel)) {
    toast.warning(t('shell.terminal.snippetUnavailable'))
    return
  }
  try {
    if (content) panel.openCreateDialogWithContent(content)
    else panel.openCreateDialog()
  } catch {
    console.warn('[QtermShell] 打开创建快捷命令弹窗失败')
    toast.warning(t('shell.terminal.snippetUnavailable'))
  }
}

const adhocConnectOpen = ref(false)

const aiChatOpen = ref(false)

const importConfigOpen = ref(false)

const exportConfigOpen = ref(false)

const aiTerminalContext = ref('')
const aiTerminalContextSessionId = ref('')

const activeSessionTitle = computed(() => {
  const id = activeSessionId.value
  if (!id) return ''
  return sessions.value.find((tab) => tab.id === id)?.title || id
})

watch([aiChatOpen, activeSessionId], ([open, sessionId]) => {
  if (!open) return
  aiTerminalContextSessionId.value = sessionId ?? ''
  aiTerminalContext.value = readTerminalTail(sessionId ?? '')
})

async function handleAdHocOpened(payload: {
  hostId: string
  displayName: string
  protocol: 'ssh' | 'telnet'
  address: string
  result: import('../services/adhocSession').AdHocOpenResult
}): Promise<void> {
  const host = {
    id: payload.hostId,
    name: payload.displayName,
    address: payload.address,
    protocol: payload.protocol,
  } as unknown as HostRecord
  const now = new Date().toISOString()
  await openSessionWithResult(host, {
    session: {
      id: payload.result.sessionId,
      hostId: payload.hostId,
      kind: payload.protocol === 'telnet' ? 'telnet' : 'ssh',
      state: payload.result.state,
      createdAt: now,
      updatedAt: now,
      attachEpoch: payload.result.attachEpoch,
    },
    streamToken: payload.result.streamToken,
  })
}

const pendingAuthenticationIsAdHoc = computed(() => {
  const sessionId = pendingAuthentication.value?.sessionId
  if (!sessionId) return false
  return isAdHocHostId(sessions.value.find((session) => session.id === sessionId)?.hostId)
})

async function handleCloseSession(sessionId: string): Promise<void> {
  if (terminalConfirmClose.value) {
    closeConfirmIds.value = [sessionId]
    return
  }
  delete toolStateMap.value[sessionId]

  await closeSession(sessionId)
}

function isShortcutEditingTarget(target: EventTarget | null): boolean {
  const element = target instanceof HTMLElement ? target : null
  if (!element) return false
  if (element.closest('[data-shortcut-capture="true"]')) return true
  if (element.classList.contains('xterm-helper-textarea')) return false

  if (element.closest('[data-shortcut-passthrough="true"]')) return false
  return !!element.closest('input, textarea, select, [contenteditable="true"]')
}

const PASSTHROUGH_SHORTCUT_ACTIONS: ReadonlySet<string> = new Set(['remoteFileSave'])

watch(activeSftpTabId, () => {
  if (workspaceSplit.value) return
  suppressToolTransition.value = true
  requestAnimationFrame(() => {
    suppressToolTransition.value = false
  })
})

watch(
  () => hosts.value.map((host) => host.id),
  (ids) => {
    const keep = new Set(ids)
    const removedRdp = rdpTabs.value.filter((tab) => tab.hostId && !keep.has(tab.hostId))
    for (const tab of removedRdp) {
      const i = rdpTabs.value.indexOf(tab)
      if (i >= 0) rdpTabs.value.splice(i, 1)
    }
    if (
      removedRdp.some((tab) => tab.id === activeRdpTabId.value) &&
      !rdpTabs.value.some((tab) => tab.id === activeRdpTabId.value)
    ) {
      activeRdpTabId.value = null
    }
    const removed = sftpTabs.value.filter((tab) => tab.hostId && !keep.has(tab.hostId))
    if (removed.length === 0) return
    for (const tab of removed) {
      const i = sftpTabs.value.indexOf(tab)
      if (i >= 0) sftpTabs.value.splice(i, 1)
    }
    if (
      removed.some((tab) => tab.id === activeSftpTabId.value) &&
      !sftpTabs.value.some((tab) => tab.id === activeSftpTabId.value)
    ) {
      activeSftpTabId.value = null
    }
  },
  { deep: true },
)

const terminalPaneCommandBus = provideTerminalPaneCommandBus()

const remoteEditorFocus = provideRemoteEditorFocus()

const remoteEditState = useRemoteEdits()
const remoteEditorRequest = computed(() => remoteEditState.editorRequest.value)
const remoteEditConflict = computed(() => remoteEditState.conflict.value)
const remoteEditConflictBusy = ref(false)

function sendTerminalPaneCommand(command: Parameters<typeof terminalPaneCommandBus.send>[1]): void {
  const sessionId = activeSessionId.value
  if (!sessionId) return
  terminalPaneCommandBus.send(sessionId, command)
}

const shortcutDispatch = createShortcutDispatch({
  nextTab: () => {
    workspace.value?.activateAdjacentTab(1)
  },
  previousTab: () => workspace.value?.activateAdjacentTab(-1),
  commandPalette: () => {
    paletteOpen.value = !paletteOpen.value
  },
  togglePaneMaximize: () => {
    window.dispatchEvent(new CustomEvent('qterm:toggle-pane-maximize'))
  },
  closeTab: () => {
    if (activeSftpTabId.value) closeSftpTab(activeSftpTabId.value)
    else if (activeRdpTabId.value) closeRdpTab(activeRdpTabId.value)
    else if (activeSessionId.value) void handleCloseSession(activeSessionId.value)
  },
  switchTab: (tabNumber) => {
    if (!tabNumber) return
    const target = sessions.value[Number(tabNumber) - 1]
    if (!target) return

    selectSessionTab(target.id)
  },
  newWindow: () => {
    void openNewWorkspaceWindow(sessionGateway.persistent).catch((error) =>
      toast.error(hostErrorMessage(error, t('shell.app.openWindowFailed'))),
    )
  },
  newLocalTab: () => void openLocalSession(),
  clearTerminal: () => sendTerminalPaneCommand('clear'),
  openTerminalSearch: () => sendTerminalPaneCommand('search'),
  copyTerminalSelection: () => sendTerminalPaneCommand('copySelection'),
  pasteTerminal: () => sendTerminalPaneCommand('paste'),
  reconnectSession: () => void reconnectActiveSession(),
  disconnectSession: () => void disconnectActiveSession(),
  focusNextPane: (direction) => {
    if (!direction) return
    workspace.value?.focusAdjacentPane(direction)
  },
  closePane: () => {
    if (activeSessionId.value) workspace.value?.removePane(activeSessionId.value)
  },
  splitRight: () => {
    requestSplit(activeSessionId.value, 'right')
  },
  splitBottom: () => {
    requestSplit(activeSessionId.value, 'bottom')
  },
  toggleSidebar: () => {
    if (supportsAssetLibrary.value) {
      toggleAssetLibrary()
      return
    }
    sidebarCollapsed.value = !sidebarCollapsed.value
  },
  remoteFileSave: () => {
    remoteEditorFocus.requestSave()
  },
})

function closeRemoteEditor(): void {
  closeRemoteTextEditor()
}

onMounted(() => {
  void ensureRemoteEditsLoaded()
})

async function chooseExternalConflict(choice: ConflictChoice): Promise<void> {
  const request = remoteEditConflict.value
  if (!request) return
  if (choice === 'cancel') {
    dismissRemoteEditConflict()
    return
  }
  remoteEditConflictBusy.value = true
  try {
    if (choice === 'overwrite') {
      const result = await syncRemoteEdit(request.editId)
      if (result.conflict) return
      dismissRemoteEditConflict()
      if (result.ok) toast.success(t('remote.ui.toast.synced'))
      else
        toast.error(t('remote.ui.toast.syncFailed', { reason: translateErrorKey(result.errorKey, t('error.unknown')) }))
      return
    }

    const result = await reloadRemoteEditForConflict(request)
    if (!result.ok) {
      toast.error(t('remote.ui.toast.openFailed', { reason: translateErrorKey(result.errorKey, t('error.unknown')) }))
      return
    }
    dismissRemoteEditConflict()
    toast.success(t('remote.ui.toast.synced'))
  } finally {
    remoteEditConflictBusy.value = false
  }
}

function toggleTabLock(sessionId: string): void {
  const session = sessions.value.find((item) => item.id === sessionId)
  if (!session) return
  session.locked = !session.locked
}

function notifyTabLocked(): void {
  toast.warning(t('shell.sessionMenu.lockedHint'))
}

const paletteOpen = ref(false)

const PALETTE_SETTING_CATEGORIES = SETTINGS_CATEGORIES

const paletteItems = computed<PaletteItem[]>(() => {
  const items: PaletteItem[] = []
  for (const session of terminalSessions.value) {
    items.push({
      id: `session:${session.id}`,
      label: session.title || session.id,
      hint: session.kind ?? undefined,
      group: t('shell.palette.sessions'),
    })
  }
  for (const host of hosts.value) {
    items.push({
      id: `host:${host.id}`,
      label: host.name || host.address,
      hint: `${host.address}${host.port ? `:${host.port}` : ''}`,
      group: t('shell.palette.hosts'),
    })
  }
  for (const category of PALETTE_SETTING_CATEGORIES) {
    items.push({
      id: `settings:${category}`,
      label: t(`settings.category.${category}`),
      group: t('shell.palette.settings'),
    })
  }
  items.push(
    { id: 'action:new-local', label: t('shell.palette.newLocalTerminal'), group: t('shell.palette.actions') },
    { id: 'action:open-settings', label: t('shell.palette.openSettingsAction'), group: t('shell.palette.actions') },
    { id: 'action:ai-chat', label: t('shell.aiChat.title'), group: t('shell.palette.actions') },
    {
      id: 'action:toggle-output-pause',
      label: t('shell.palette.toggleOutputPause'),
      group: t('shell.palette.actions'),
    },
  )
  return items
})

function runPaletteItem(item: PaletteItem): void {
  const [kind, value] = item.id.split(':', 2)
  if (kind === 'session') {
    selectSessionTab(value)
    return
  }
  if (kind === 'host') {
    const host = hosts.value.find((entry) => entry.id === value)
    if (host) void openHost(host)
    return
  }
  if (kind === 'settings') {
    void openSettings(value as Parameters<typeof openSettings>[0])
    return
  }
  if (item.id === 'action:new-local') {
    void openLocalSession()
    return
  }
  if (item.id === 'action:ai-chat') {
    aiChatOpen.value = true
    return
  }
  if (item.id === 'action:open-settings') {
    void openSettings(settingsCategory.value)
    return
  }
  if (item.id === 'action:toggle-output-pause') {
    window.dispatchEvent(new CustomEvent('qterm:toggle-terminal-pause'))
  }
}

function handleShortcut(event: KeyboardEvent): void {
  if (appLocked.value) return

  if (isMainWindow && settingsWindowOpen.value) return
  if (event.defaultPrevented || securityDialogOpen.value || isShortcutEditingTarget(event.target)) return

  const action = boundShortcutAction(event, shortcuts.value, isMac.value)
  if (!action) return

  const target = event.target instanceof HTMLElement ? event.target : null
  if (target?.closest('[data-shortcut-passthrough="true"]') && !PASSTHROUGH_SHORTCUT_ACTIONS.has(action)) return
  event.preventDefault()

  if (shortcutNeedsActiveTab(action) && !activeSessionId.value && !activeSftpTabId.value && !activeRdpTabId.value)
    return
  shortcutDispatch[action](event)
}

function handleSettingsShortcut(event: KeyboardEvent): void {
  if (isSettingsWindow && event.key === 'Escape') {
    event.preventDefault()
    closeSecuritySettings()
    return
  }
  if (!(event.metaKey || event.ctrlKey) || event.altKey || event.shiftKey || event.key !== ',') return

  if (event.defaultPrevented) return
  if (appLocked.value || isShortcutEditingTarget(event.target)) return
  event.preventDefault()
  void openSettings(settingsCategory.value)
}

function openDialogWindowIfEnabled(kind: DialogWindowKind, title: string, arg = ''): boolean {
  if (isSettingsWindow || !settingsInWindow.value || !sftpGateway.persistent) return false
  if (kind === 'hostEditor') {
    void openHostEditorWindow(arg).catch(() => undefined)
    return true
  }
  if (kind !== 'settings') return false
  void openDialogWindow(kind, title, arg, true).catch(() => undefined)
  return true
}

function windowedDialogDone(action: () => unknown): void {
  void action()
  if (isSettingsWindow) void hideSettingsWindow()
}

function closeWindowedDialog(closeLocally: () => void): void {
  closeLocally()
  if (isSettingsWindow) void hideSettingsWindow()
}

function requestImportConfig(): void {
  if (openDialogWindowIfEnabled('import', t('shell.toolbar.settings'), '')) return
  importConfigOpen.value = true
}

function requestExportConfig(): void {
  if (openDialogWindowIfEnabled('export', t('shell.toolbar.settings'), '')) return
  exportConfigOpen.value = true
}

function requestCreateHostDialog(): void {
  if (openDialogWindowIfEnabled('hostEditor', t('shell.toolbar.settings'), '')) return
  openCreateDialog()
}

function requestEditHostDialog(host: HostRecord): void {
  if (openDialogWindowIfEnabled('hostEditor', t('shell.toolbar.settings'), host.id)) return
  openEditHostDialog(host)
}

let dialogWindowSeq = 0
let dialogWindowDispatched = -1

function dispatchDialogWindow(): void {
  if (!isSettingsWindow || !settingsReady.value) return
  if (dialogWindowDispatched === dialogWindowSeq) return
  dialogWindowDispatched = dialogWindowSeq

  importConfigOpen.value = false
  exportConfigOpen.value = false
  if (dialogWindowKind.value !== 'settings') securityDialogOpen.value = false
  switch (dialogWindowKind.value) {
    case 'import':
      importConfigOpen.value = true
      break
    case 'export':
      exportConfigOpen.value = true
      break
    default: {
      const requested = dialogWindowArg.value
      credentialView.value = requested === 'credentials:otp' ? 'otp' : 'credentials'
      void openSettings(
        (requested === 'credentials:otp' ? 'credentials' : requested || 'application') as SettingsCategory,
      )
      break
    }
  }
}

watch([settingsReady, hosts], dispatchDialogWindow, { immediate: true })

function focusSettingsWindow(): void {
  if (settingsInWindow.value) {
    void focusSettingsWindowInWindow()
    return
  }
  void openSettings(settingsCategory.value)
}

async function closeSessions(sessionIds: string[]): Promise<void> {
  if (terminalConfirmClose.value) {
    closeConfirmIds.value = [...sessionIds]
    return
  }
  for (const id of sessionIds) delete toolStateMap.value[id]
  await Promise.allSettled(sessionIds.map((id) => closeSession(id)))
}

async function confirmCloseSessions(): Promise<void> {
  const ids = closeConfirmIds.value
  closeConfirmIds.value = []
  for (const id of ids) delete toolStateMap.value[id]
  await Promise.allSettled(ids.map((id) => closeSession(id)))
}

function reorderSessionTabs(value: { sourceId: string; targetId: string; position?: 'before' | 'after' }): void {
  reorderSessions(value.sourceId, value.targetId, value.position ?? 'before')
}

async function reconnectSessionFromTerminal(sessionId: string): Promise<void> {
  if (activeSessionId.value !== sessionId) activeSessionId.value = sessionId
  await reconnectActiveSessionPreservingTool()
}

async function disconnectSessionFromTerminal(sessionId: string): Promise<void> {
  if (activeSessionId.value !== sessionId) activeSessionId.value = sessionId
  await disconnectActiveSession()
}

function openTerminalSettings(): void {
  void openSettings('terminal')
}

async function duplicateSession(sessionId: string): Promise<void> {
  const source = sessions.value.find((session) => session.id === sessionId)
  if (!source) return

  if (source.kind === 'local') {
    await openLocalSession()
    return
  }
  const host = hosts.value.find((item) => item.id === source.hostId)
  if (host) await openHost(host)
}

function renameRdpTab(tabId: string, title: string): void {
  const target = rdpTabs.value.find((tab) => tab.id === tabId)
  const next = (title ?? '').trim().slice(0, 64)
  if (!target || !next) return
  target.title = next
}

function duplicateRdpTab(tabId: string): void {
  const source = rdpTabs.value.find((item) => item.id === tabId)
  if (!source || !source.hostId) return
  const host = hosts.value.find((item) => item.id === source.hostId)
  if (!host) return
  const tab: RdpWorkspaceTab = {
    id: nextRdpTabId(),
    title: source.title || host.name || host.address,
    hostId: host.id,
    protocol: source.protocol ?? 'rdp',
    order: nextTabOrder(),
  }
  rdpTabs.value.push(tab)
  activeRdpTabId.value = tab.id
}

function toggleRdpTabLock(tabId: string): void {
  const tab = rdpTabs.value.find((item) => item.id === tabId)
  if (!tab) return
  tab.locked = !tab.locked
}

function renameSftpTab(tabId: string, title: string): void {
  const target = sftpTabs.value.find((tab) => tab.id === tabId)
  const next = (title ?? '').trim().slice(0, 64)
  if (!target || !next) return
  target.title = next
}

function cancelCloseSessions(): void {
  closeConfirmIds.value = []
}

async function copyHost(
  host: { id: string; address: string; username: string; port: number },
  kind: 'ip' | 'connection' | 'id',
) {
  const value = kind === 'ip' ? host.address : kind === 'id' ? host.id : `${host.username}@${host.address}:${host.port}`
  try {
    if (!navigator.clipboard) throw new Error(t('shell.app.clipboardUnsupported'))
    await navigator.clipboard.writeText(value)
    toast.success(t('shell.app.copied'))
  } catch (error) {
    toast.error(hostErrorMessage(error, t('shell.app.copyFailed')))
  }
}

async function moveAsset(
  assetId: string,
  groupId: string,
  targetId: string,
  position: 'before' | 'after' | 'inside',
): Promise<void> {
  try {
    sortMode.value = 'custom'
    await hostGateway.reorderHost(assetId, groupId, targetId, position)
    await loadHosts()
  } catch (error) {
    await loadHosts()
    toast.error(hostErrorMessage(error, t('shell.app.moveHostFailed')))
  }
}
async function moveGroup(
  id: string,
  parentId: string,
  targetId: string,
  position: 'before' | 'after' | 'inside',
): Promise<void> {
  try {
    sortMode.value = 'custom'
    await hostGateway.reorderGroup(id, parentId, targetId, position)
    await loadHosts()
  } catch (error) {
    await loadHosts()
    toast.error(hostErrorMessage(error, t('shell.app.moveGroupFailed')))
  }
}

function createHostInGroup(groupId: string): void {
  const group = groups.value.find((item) => item.id === groupId)
  openCreateDialog()
  if (group) draft.value.group = group.name
}
function selectHost(id: string): void {
  selectedHostId.value = id
  selectedGroupId.value = null
}
function selectGroup(id: string): void {
  selectedGroupId.value = id
  selectedHostId.value = null
}
function clearAssetSelection(): void {
  selectedHostId.value = null
  selectedGroupId.value = null
}

const workspaceSplit = ref(false)

const isWorkspaceAssetLibrary = computed(() => assetLibraryLayout.value === 'workspace')
const assetLibraryVisible = ref(false)

const supportsAssetLibrary = computed(() => isWorkspaceAssetLibrary.value && !isSessionWindow)

const showAssetLibrary = computed(() => supportsAssetLibrary.value && assetLibraryVisible.value)

watch(
  isWorkspaceAssetLibrary,
  (enabled) => {
    assetLibraryVisible.value = enabled
  },
  { immediate: true },
)

const isFullWidthMode = computed(() => workspaceSplit.value || Boolean(activeSftpTabId.value) || showAssetLibrary.value)

const hasWorkspaceTabs = computed(
  () => sessions.value.length > 0 || sftpTabs.value.length > 0 || rdpTabs.value.length > 0,
)

const showToolArea = computed(() => !isFullWidthMode.value && hasWorkspaceTabs.value)

const showToolRail = computed(
  () => ((showToolArea.value && !activeRdpTabId.value) || Boolean(activeRdpTabId.value)) && !showAssetLibrary.value,
)

watch([activeSessionId, activeSftpTabId, activeRdpTabId], ([sessionId, sftpTabId, rdpTabId]) => {
  if (!supportsAssetLibrary.value) return
  if (sessionId || sftpTabId || rdpTabId) assetLibraryVisible.value = false
})
watch(hasWorkspaceTabs, (hasTabs) => {
  if (!supportsAssetLibrary.value) return

  if (!hasTabs && !suppressAssetLibraryAutoShow) assetLibraryVisible.value = true
})

let suppressAssetLibraryAutoShow = false

function toggleAssetLibrary(): void {
  if (!supportsAssetLibrary.value) return
  assetLibraryVisible.value = !assetLibraryVisible.value
}

const renderedTool = ref<ToolPanel>('sftp')

const suppressToolTransition = ref(false)

const toolStateMap = ref<Record<string, ToolPanel | null>>({})
const reconnectingSessionId = ref<string | null>(null)
let restoringToolState = false

const {
  returningToTabs,
  returnActiveSessionToTabs,
  openSessionInNewWindow,
  start: startSessionWindowHandover,
  dispose: disposeSessionWindowHandover,
  stopAdoptionListener,
} = useSessionWindowHandover({
  sessionGateway,
  isSessionWindow,
  sessions,
  activeSession,
  activeSessionId,
  hosts,
  openSession,
  openLocalSession,
  adoptSession,
  detachSession,
  clearSessionToolState: (sessionId) => {
    delete toolStateMap.value[sessionId]
  },
  adoptRdpSession,
  adoptVncTab: (request) => adoptVncTabRequest(request),
})

function rememberToolState(sessionId: string | null, tool: ToolPanel | null): void {
  if (sessionId) toolStateMap.value[sessionId] = tool
}

async function restoreToolState(sessionId: string | null): Promise<void> {
  suppressToolTransition.value = true
  restoringToolState = true
  try {
    activeTool.value =
      sessionId && Object.prototype.hasOwnProperty.call(toolStateMap.value, sessionId)
        ? (toolStateMap.value[sessionId] ?? null)
        : null
  } finally {
    restoringToolState = false
  }
  await nextTick()
  requestAnimationFrame(() => {
    suppressToolTransition.value = false
  })
}

watch(
  activeTool,
  (tool) => {
    if (tool) renderedTool.value = tool
    if (!restoringToolState && !workspaceSplit.value && !activeSftpTabId.value)
      rememberToolState(activeSessionId.value, tool)
  },
  { flush: 'sync' },
)

watch(terminalRecordCommandHistory, (enabled) => setCommandHistoryEnabled(enabled), { immediate: true })
watch([terminalHistoryMinLength, terminalHistoryMaxLength], ([min, max]) => setCommandHistoryLimits(min, max), {
  immediate: true,
})

watch(
  activeSessionId,
  (newId, oldId) => {
    activeSftpTabId.value = null
    activeRdpTabId.value = null
    if (oldId) rememberToolState(oldId, activeTool.value)
    if (newId === oldId || workspaceSplit.value) return

    if (oldId && newId && reconnectingSessionId.value === oldId) {
      rememberToolState(newId, toolStateMap.value[oldId] ?? activeTool.value)
      return
    }

    if (!oldId && newId && !Object.prototype.hasOwnProperty.call(toolStateMap.value, newId)) {
      rememberToolState(newId, activeTool.value)
      return
    }
    void restoreToolState(newId)
  },
  { flush: 'sync' },
)

function closeActiveTool(): void {
  activeTool.value = null
}

function nextSftpTabTitle(baseTitle: string): string {
  const titles = new Set(sftpTabs.value.map((tab) => tab.title))
  if (!titles.has(baseTitle)) return baseTitle
  let index = 1
  while (titles.has(`${baseTitle} (${index})`)) index += 1
  return `${baseTitle} (${index})`
}

async function openSftpForSession(sessionId?: string): Promise<void> {
  const targetSessionId = sessionId || activeSessionId.value
  const session = sessions.value.find((item) => item.id === targetSessionId)
  const usableSession = session && session.kind === 'ssh' && session.hostId ? session : null
  const host = usableSession ? hosts.value.find((item) => item.id === usableSession.hostId) : undefined
  const tab = {
    id: nextSftpTabId(),
    title: nextSftpTabTitle(host ? t('shell.app.sftpHostTitle', { name: host.name }) : t('shell.app.sftpTitle')),
    hostId: host?.id ?? null,
    order: nextTabOrder(),
  }
  sftpTabs.value.push(tab)
  activeRdpTabId.value = null
  activeSftpTabId.value = tab.id
}

registerSnippetCreator((content) => {
  void openSnippetCreator(content)
})

function analyzeWithAI(sessionId: string, kind: 'selection' | 'recent', text: string): void {
  if (!sessionId) return
  requestAILaunch({ kind, text, mode: 'agent' })
}

watch(useAILaunch(), (value) => {
  if (value) activeTool.value = 'ai'
})

function openNewSftpTab(): void {
  const tab = {
    id: nextSftpTabId(),
    title: nextSftpTabTitle(t('shell.app.sftpTitle')),
    hostId: null,
    order: nextTabOrder(),
  }
  sftpTabs.value.push(tab)
  activeRdpTabId.value = null
  activeSftpTabId.value = tab.id
}

async function createWorkspaceWindow(): Promise<void> {
  try {
    await openNewWorkspaceWindow(sessionGateway.persistent)
  } catch (error) {
    toast.error(hostErrorMessage(error, t('shell.app.openWindowFailed')))
  }
}

async function reconnectActiveSessionPreservingTool(): Promise<void> {
  const sessionId = activeSessionId.value
  if (!sessionId) return
  const rememberedTool = activeTool.value
  rememberToolState(sessionId, rememberedTool)
  reconnectingSessionId.value = sessionId
  try {
    await reconnectActiveSession()

    const nextSessionId = activeSessionId.value
    if (nextSessionId && nextSessionId !== sessionId) {
      rememberToolState(nextSessionId, rememberedTool)
      activeTool.value = rememberedTool
    }
  } finally {
    reconnectingSessionId.value = null
  }
}

function onSplitChange(value: boolean): void {
  if (value) {
    rememberToolState(activeSessionId.value, activeTool.value)
    workspaceSplit.value = true
    suppressToolTransition.value = true
    activeTool.value = null
    requestAnimationFrame(() => {
      suppressToolTransition.value = false
    })
    return
  }
  workspaceSplit.value = false
  if (activeSessionId.value) void restoreToolState(activeSessionId.value)
}

const pickerOpen = ref(false)

const pickerMode = ref<AssetPickerMode>('split')

const pickerDirection = ref<SplitDirection>('right')

const pickerTargetId = ref<string | null>(null)

function requestSplit(sessionId: string | null, direction: SplitDirection = 'right'): void {
  const target =
    sessionId && sessions.value.some((session) => session.id === sessionId) ? sessionId : activeSessionId.value
  if (!target) return
  pickerTargetId.value = target
  pickerDirection.value = direction
  pickerMode.value = 'split'
  pickerOpen.value = true
}

function requestQuickConnect(): void {
  pickerTargetId.value = null
  pickerMode.value = 'connect'
  pickerOpen.value = true
}

const pickerTarget = computed(() => sessions.value.find((session) => session.id === pickerTargetId.value) ?? null)

const pickerCurrent = computed<AssetChoice | null>(() => {
  const target = pickerTarget.value
  if (!target) return null
  if (target.kind === 'local') return { kind: 'local' }
  const host = hosts.value.find((item) => item.id === target.hostId)
  return host ? { kind: 'host', host } : null
})

async function selectAsset(choice: AssetChoice): Promise<void> {
  const mode = pickerMode.value
  const target = pickerTargetId.value
  const direction = pickerDirection.value
  pickerOpen.value = false
  if (mode === 'connect') {
    if (choice.kind === 'local') await openLocalSession()
    else await openHost(choice.host)
    return
  }
  if (!target) return
  const opened = choice.kind === 'local' ? await openLocalSession() : await openSession(choice.host)
  if (!opened) return
  workspace.value?.applySplit(target, opened.id, direction)
}

const zmodemDialogRef = ref<InstanceType<typeof ZmodemDialog> | null>(null)

function handleZmodemDownloadOffer(e: Event): void {
  const detail = (e as CustomEvent).detail
  if (detail?.sessionId && detail?.name !== undefined && zmodemDialogRef.value) {
    zmodemDialogRef.value.onDownloadOffer(detail.sessionId, detail.name, detail.size)
  }
}

function handleZmodemStatus(e: Event): void {
  const detail = (e as CustomEvent).detail
  if (detail?.sessionId && zmodemDialogRef.value) {
    zmodemDialogRef.value.onProgress(detail)
  }
}

interface FileDropEvent {
  id?: string
  sessionId?: string
  directory?: string
  paths?: string[]
  messageKey?: string
}

let selfWindowNamePromise: Promise<string | null> | null = null

function currentSelfWindowName(): Promise<string | null> {
  selfWindowNamePromise ??= resolveSelfWindowName()
  return selfWindowNamePromise
}

const SFTP_DROP_PACKING_TOAST_ID = 'sftp-drop-packing'

async function handleNativeFileDrop(event: { data: FileDropEvent; sender?: string | null }): Promise<void> {
  if (!shouldHandleDrop(event, await currentSelfWindowName())) return
  const detail = event.data
  if (!detail?.sessionId) return
  if (detail.paths?.length) {
    window.dispatchEvent(new CustomEvent('qterm:local-file-drop', { detail }))
    return
  }
  if (!detail.id) {
    toast.error(detail.messageKey ? t('shell.app.dropFileFailed') : t('shell.app.dropFileRejected'))
    return
  }
  const directory =
    detail.directory?.trim() || (await requestRemoteDirectory(new AbortController().signal, 'terminal'))?.trim()
  if (!directory) {
    await sftpGateway.cancelDroppedFiles(detail.id)
    return
  }

  const packing = sftpDragFolderUploadMode.value === 'archive'
  if (packing) toast.loading(t('shell.app.packingUpload'), { id: SFTP_DROP_PACKING_TOAST_ID, duration: Infinity })
  try {
    const result = await sftpGateway.uploadDroppedFiles(detail.id, directory)

    for (const transfer of result.transfers) claimTransfer(transfer.id)
    if ((result.queued ?? 0) > 0) claimSession(detail.sessionId)
    const queued = result.transfers.length + (result.queued ?? 0)

    if (result.fellBack) toast.warning(t('shell.app.uploadFellBackToNormal'))
    if (result.rejected.length) toast.error(t('shell.app.rejectedUploads', { count: result.rejected.length }))

    if (queued > 0) toast.success(t('shell.app.queuedUploads', { count: queued }))
  } catch (error) {
    await sftpGateway.cancelDroppedFiles(detail.id).catch(() => undefined)
    toast.error(sftpErrorMessage(error, t('shell.app.createUploadFailed')))
  } finally {
    if (packing) toast.dismiss(SFTP_DROP_PACKING_TOAST_ID)
  }
}

let stopNativeFileDrop: (() => void) | undefined
let stopNativeWindowResize: (() => void) | undefined
let stopNativeWindowMove: (() => void) | undefined
let stopNativeWindowClosing: (() => void) | undefined
let stopEvenDialogHeights: (() => void) | undefined
let cancelCloseToTrayRuntimeWait: (() => void) | undefined

function updateEvenViewportMetrics(): void {
  const viewportHeight = evenPixel(window.innerHeight, 2)
  const viewportContentHeight = evenPixel(Math.max(2, viewportHeight - 32), 2)
  const settingsHeight = evenPixel(Math.min(viewportHeight * 0.8, viewportContentHeight), 2)
  const root = document.documentElement
  root.style.setProperty('--qt-viewport-height-even', `${viewportHeight}px`)
  root.style.setProperty('--qt-viewport-content-height-even', `${viewportContentHeight}px`)
  root.style.setProperty('--qt-settings-dialog-height-even', `${settingsHeight}px`)
  root.style.setProperty(
    '--qt-terminal-theme-list-height-even',
    `${evenPixel(Math.min(viewportHeight * 0.48, 560), 2)}px`,
  )
  root.style.setProperty(
    '--qt-terminal-theme-editor-height-even',
    `${evenPixel(Math.min(660, viewportContentHeight), 2)}px`,
  )
  root.style.setProperty(
    '--qt-snippet-picker-height-even',
    `${evenPixel(Math.min(viewportHeight * 0.8, viewportContentHeight), 2)}px`,
  )
  root.style.setProperty(
    '--qt-snippet-picker-list-height-even',
    `${evenPixel(Math.min(viewportHeight * 0.56, 420), 2)}px`,
  )
  root.style.setProperty('--qt-history-replay-height-even', `${evenPixel(Math.min(viewportHeight * 0.88, 780), 2)}px`)
  root.style.setProperty('--qt-snippet-scroll-height-even', `${evenPixel(Math.min(viewportHeight * 0.6, 560), 2)}px`)
  root.style.setProperty('--qt-sftp-session-list-height-even', `${evenPixel(Math.min(viewportHeight * 0.4, 280), 2)}px`)
  root.style.setProperty('--qt-security-settings-height-even', `${evenPixel(Math.min(620, viewportHeight - 180), 2)}px`)
  root.style.setProperty('--qt-transfer-queue-height-even', `${evenPixel(Math.min(420, viewportHeight - 44), 2)}px`)
}

function scheduleGeometryPersist(): void {
  if (isSessionWindow || isChildWorkspaceWindow || isSettingsWindow) return
  scheduleWindowGeometryPersist(windowRememberSize.value, windowRememberPosition.value)
}

function handleWindowResize(): void {
  updateEvenViewportMetrics()
  if (sftpGateway.persistent) void normalizeCurrentWindowGeometry()
  scheduleGeometryPersist()
}

function handleWindowMove(): void {
  scheduleGeometryPersist()
}

const { locked: appLocked, ready: appLockReady, enabled: appLockEnabled, lock: lockAppNow, initAppLock } = useAppLock()

const appLockThemeStyle = computed<Record<string, string>>(() => {
  const style = pageBackgroundStyle.value
  const picked: Record<string, string> = {}
  for (const key of ['--qt-theme-6', '--qt-theme-8']) {
    const value = style[key]
    if (value) picked[key] = value
  }
  return picked
})

watch(appLocked, (locked) => {
  if (!locked) return
  closeSecuritySettings()

  void closeHostEditorWindow()
})

function publishTrayMenuState(): void {
  if (!isMainWindow || !trayActionsSubscribed) return
  void setTrayMenuState({
    ready: trayNativeReady && settingsReady.value && appLockReady.value,
    locked: appLocked.value,
    newLocalLabel: t('shell.palette.newLocalTerminal'),
    showLabel: t('shell.tray.show'),
    settingsLabel: t('shell.toolbar.settings'),
    quitLabel: t('shell.tray.quit'),
  })
}

watch([settingsReady, appLockReady, appLocked, appLocale], publishTrayMenuState, { immediate: true })

function handleTrayAction(action: TrayAction): void {
  if (!isMainWindow || !trayNativeReady || !settingsReady.value || !appLockReady.value || appLocked.value) return
  if (action === 'newLocalTerminal') {
    void openLocalSession()
    return
  }
  void openSettings()
}

async function initializeTrayNativeState(): Promise<void> {
  if (!isMainWindow || !trayActionsSubscribed || trayInitializationRunning) return
  if (!hasWailsRuntime()) {
    cancelTrayRuntimeWait?.()
    cancelTrayRuntimeWait = whenWailsRuntime(() => {
      cancelTrayRuntimeWait = undefined
      void initializeTrayNativeState()
    })
    return
  }
  trayInitializationRunning = true
  try {
    await initAppLock()
    if (trayActionsSubscribed && hasWailsRuntime()) {
      trayNativeReady = true
      publishTrayMenuState()
    }
  } catch {
    trayNativeReady = false
  } finally {
    trayInitializationRunning = false
  }
}

function syncSettingsWindow(enabled: boolean): void {
  if (!isMainWindow || !settingsReady.value) return
  void setSettingsWindowEnabled(enabled)
}

watch(
  [settingsReady, settingsInWindow],
  ([ready, enabled]) => {
    if (!ready || !isMainWindow) return
    syncSettingsWindow(enabled)
  },
  { immediate: true },
)

function syncCloseToTray(enabled: boolean): void {
  if (!isMainWindow || !settingsReady.value) return
  if (hasWailsRuntime()) {
    cancelCloseToTrayRuntimeWait?.()
    cancelCloseToTrayRuntimeWait = undefined
    void setCloseToTray(enabled)
    return
  }

  cancelCloseToTrayRuntimeWait?.()
  cancelCloseToTrayRuntimeWait = whenWailsRuntime(() => {
    cancelCloseToTrayRuntimeWait = undefined
    if (isMainWindow && settingsReady.value) void setCloseToTray(closeToTray.value)
  })
}

watch(
  [settingsReady, closeToTray],
  ([ready, enabled]) => {
    if (!ready || !isMainWindow) return
    syncCloseToTray(enabled)
  },
  { immediate: true },
)

onMounted(() => {
  void initAppLock()
  updateEvenViewportMetrics()
  stopEvenDialogHeights = observeEvenDialogHeights()
  window.addEventListener('resize', handleWindowResize)
  if (sftpGateway.persistent) {
    stopNativeWindowResize = Events.On('common:WindowDidResize', handleWindowResize)
    if (isMainWindow) {
      stopNativeWindowMove = Events.On('common:WindowDidMove', handleWindowMove)
      stopNativeWindowClosing = Events.On('common:WindowClosing', flushWindowGeometry)
    }
  }
  window.addEventListener('qterm:zmodem-download-offer', handleZmodemDownloadOffer)
  window.addEventListener('qterm:zmodem-status', handleZmodemStatus)
  if (isMainWindow) {
    stopHostsChanged = connectHostsChanged(() => {
      void resourcesImported()
    })
    stopTrayActions = subscribeTrayActions(handleTrayAction)
    trayActionsSubscribed = true
    void initializeTrayNativeState()
    stopSettingsWindowState = connectSettingsWindowState((state) => {
      settingsWindowOpen.value = state.open

      if (state.open && state.kind) {
        dialogWindowKind.value = state.kind
        dialogWindowArg.value = state.category || dialogWindowArg.value
      }
    })

    stopQuitRequest = Events.On(QUIT_REQUEST_EVENT, (event: { data?: unknown }) =>
      handleQuitRequest(event?.data as QuitRequestPayload | undefined),
    )
  }
  if (isSettingsWindow) {
    stopSettingsWindowState = connectSettingsWindowState((state) => {
      if (!state.open || state.kind !== 'settings') return

      const sameKind = state.kind === dialogWindowKind.value
      const alreadyOpen = securityDialogOpen.value || importConfigOpen.value || exportConfigOpen.value
      if (sameKind && alreadyOpen && (!state.category || state.category === dialogWindowArg.value)) return
      dialogWindowKind.value = state.kind
      dialogWindowArg.value = state.category || dialogWindowArg.value
      dialogWindowSeq += 1
      dispatchDialogWindow()

      requestAnimationFrame(() => {
        window.dispatchEvent(new Event('resize'))
        requestAnimationFrame(() => window.dispatchEvent(new Event('resize')))
      })
    })
  }
  if (sftpGateway.persistent && !isSettingsWindow) {
    void currentSelfWindowName()
    stopNativeFileDrop = Events.On(FILE_DROP_EVENT, handleNativeFileDrop)
  }

  if (!isSettingsWindow) startSessionWindowHandover()

  if (!isSettingsWindow) window.addEventListener('keydown', handleShortcut)

  window.addEventListener('keydown', handleSettingsShortcut)
  if (isMainWindow) {
    void restoreWindowGeometry(windowRememberSize.value, windowRememberPosition.value).then(() => {
      if (sftpGateway.persistent) void normalizeCurrentWindowGeometry()
    })
  } else if (sftpGateway.persistent) {
    void normalizeCurrentWindowGeometry()
  }
})

onBeforeUnmount(() => {
  disposeSessionWindowHandover()
  stopEvenDialogHeights?.()
  window.removeEventListener('resize', handleWindowResize)
  stopNativeWindowResize?.()
  stopNativeWindowMove?.()
  stopNativeWindowClosing?.()
  cancelCloseToTrayRuntimeWait?.()
  cancelCloseToTrayRuntimeWait = undefined
  window.removeEventListener('qterm:zmodem-download-offer', handleZmodemDownloadOffer)
  window.removeEventListener('qterm:zmodem-status', handleZmodemStatus)
  window.removeEventListener('keydown', handleShortcut)
  window.removeEventListener('keydown', handleSettingsShortcut)
  stopNativeFileDrop?.()
  stopQuitRequest?.()
  stopTrayActions?.()
  stopHostsChanged?.()
  stopHostsChanged = undefined
  stopSettingsWindowState?.()
  trayActionsSubscribed = false
  trayNativeReady = false
  trayInitializationRunning = false
  cancelTrayRuntimeWait?.()
  cancelTrayRuntimeWait = undefined
  stopAdoptionListener()

  if (isMainWindow) flushWindowGeometry()
})
</script>
<template>
  <div
    class="qterm-shell text-sm"
    :style="[pageBackgroundStyle, layoutSizeStyle]"
    :data-host-color="hostColorStyle"
    :inert="appLocked || (isMainWindow && settingsWindowOpen) || undefined"
    :class="{
      dark: isDark,
      'is-dark': isDark,
      'has-page-background': backgroundEnabled,
      'terminal-theme-background-terminal': terminalThemeBackgroundScope === 'terminal',
      'terminal-theme-background-shell': terminalThemeBackgroundScope === 'shell',
      'terminal-theme-background-derived': terminalThemeBackgroundScope === 'derived',
      'has-terminal-sessions': terminalSessions.length > 0,
      'is-mac': isMac,
      // 全屏资产库布局下侧栏恒定隐藏：复用既有的 0 宽度列折叠，不新增一套布局 CSS。
      'is-sidebar-collapsed': sidebarCollapsed || supportsAssetLibrary,
      'is-session-window': isSessionWindow,
      'is-settings-window': isSettingsWindow,
    }"
  >
    <SessionWindowTitlebar
      v-if="isSessionWindow"
      :title="activeSession?.title || sessionWindowTitle"
      :status-tone="activeSessionStatus.tone"
    >
      <template #actions>
        <IconButton v-if="activeSftpTabId" :label="t('shell.app.returnToTerminal')" @click="returnToTerminalFromSftp">
          <SquareTerminal :size="16" aria-hidden="true" />
        </IconButton>
        <IconButton
          v-if="sessionGateway.persistent && activeSession"
          :label="t('shell.app.returnToTabs')"
          :disabled="returningToTabs"
          :aria-busy="returningToTabs"
          @click="returnActiveSessionToTabs"
        >
          <PanelTop :size="16" aria-hidden="true" />
        </IconButton>
      </template>
    </SessionWindowTitlebar>

    <SessionWindowTitlebar
      v-if="isSettingsWindow"
      class="pointer-events-auto"
      :style="{ zIndex: 60 }"
      :title="dialogWindowTitle"
      :show-status="false"
    />

    <SessionOverlay
      v-if="returningToTabs"
      variant="mask"
      tone="info"
      busy
      class="session-window-handover-mask"
      :message="t('shell.app.returnToTabsBusy')"
    />

    <AppToolbar
      v-if="!isSessionWindow"
      v-show="!isSettingsWindow"
      :sidebar-collapsed="sidebarCollapsed"
      :is-dark="isDark"
      :sftp-active="Boolean(activeSftpTabId)"
      :asset-library-mode="supportsAssetLibrary"
      :asset-library-active="showAssetLibrary"
      :app-lock-enabled="appLockEnabled"
      @lock-app="lockAppNow"
      @toggle-sidebar="sidebarCollapsed = !sidebarCollapsed"
      @toggle-asset-library="toggleAssetLibrary"
      @create-terminal="openCreateDialog"
      @create-local-terminal="openLocalSession"
      @temporary-connection="adhocConnectOpen = true"
      @open-sftp="openNewSftpTab"
      @create-window="createWorkspaceWindow"
      @toggle-maximise="toggleCurrentWindowMaximise"
      @open-settings="openSettings"
      @import-config="requestImportConfig"
      @export-config="requestExportConfig"
      @toggle-theme="toggleTheme"
    />

    <div
      class="app-body"
      v-show="!isSettingsWindow"
      :class="{
        'has-split-workspace': isFullWidthMode,
        'has-tool-rail': showToolRail,
        'no-tool-transition': suppressToolTransition || !settingsReady,
      }"
    >
      <AssetSidebar
        v-if="!isSessionWindow"
        id="asset-sidebar"
        :inert="supportsAssetLibrary || undefined"
        :aria-hidden="supportsAssetLibrary || undefined"
        :search-visible="searchVisible"
        :search-query="searchQuery"
        :sort-mode="sortMode"
        :hosts="hosts"
        :ungrouped-hosts="ungroupedHosts"
        :grouped-hosts="groupedHosts"
        :root-entries="rootEntries"
        :loading="loadingHosts"
        :load-error="hostLoadError"
        :selected-host-id="selectedHostId"
        :selected-group-id="selectedGroupId"
        :is-dark="isDark"
        :show-host-address="showHostAddress"
        @create-host="requestCreateHostDialog"
        @create-host-in-group="createHostInGroup"
        @create-subgroup="openCreateGroupDialog"
        @create-group="openCreateGroupDialog"
        @toggle-search="searchVisible = !searchVisible"
        @toggle-sort="toggleSort"
        @refresh="loadHosts"
        @select-host="selectHost"
        @select-group="selectGroup"
        @clear-selection="clearAssetSelection"
        @open-host="openHost"
        @edit-host="requestEditHostDialog"
        @delete-host="requestDeleteHost"
        @copy-host="copyHost"
        @move-asset="moveAsset"
        @move-group="moveGroup"
        @edit-group="openEditGroupDialog"
        @delete-group="requestDeleteGroup"
        @update-search-query="searchQuery = $event"
      />

      <ResizeHandle
        v-if="!isSessionWindow && !sidebarCollapsed && !supportsAssetLibrary"
        class="app-sidebar-resize top-0 bottom-0 left-[calc(var(--qt-sidebar-effective-width)_-_5px)]"
        orientation="vertical"
        :label="t('shell.app.resizeAssets')"
        @resize-start="beginSidebarResize"
        @resize="resizeSidebar"
      />

      <main
        class="workspace"
        :class="{
          'has-tool-panel': activeTool && showToolArea && !activeRdpTabId,
          'has-workspace-tabs': hasWorkspaceTabs,
          'no-tool-transition': suppressToolTransition,
        }"
        @click="clearAssetSelection"
      >
        <SessionWorkspace
          ref="workspace"
          :sessions="sessions"
          :terminal-sessions="terminalSessions"
          :active-session-id="activeSessionId"
          :active-session="activeSession"
          :active-host="activeHost"
          :active-status="activeSessionStatus"
          :asset-library-covered="showAssetLibrary"
          :persistent="sessionGateway.persistent"
          :session-gateway="sessionGateway"
          :dark="isDark"
          :transparent-background="backgroundEnabled"
          :terminal-font-size="terminalFontSize"
          :terminal-line-height="terminalLineHeight"
          :terminal-font-family="terminalFontFamily"
          :settings-ready="settingsReady"
          :terminal-light-theme="terminalLightTheme"
          :terminal-dark-theme="terminalDarkTheme"
          :terminal-auto-copy-selection="terminalAutoCopySelection"
          :terminal-copy-feedback="terminalCopyFeedback"
          :terminal-scrollback="terminalScrollback"
          :terminal-confirm-multi-line-paste="terminalConfirmMultiLinePaste"
          :terminal-completion-enabled="terminalCompletionEnabled"
          :terminal-history-min-length="terminalHistoryMinLength"
          :terminal-history-max-length="terminalHistoryMaxLength"
          :terminal-word-separators="terminalWordSeparators"
          :terminal-mac-option-is-meta="terminalMacOptionIsMeta"
          :terminal-cursor-blink="terminalCursorBlink"
          :terminal-cursor-style="terminalCursorStyle"
          :terminal-right-click-behavior="terminalRightClickBehavior"
          :terminal-modifier-right-click-behavior="terminalModifierRightClickBehavior"
          :modifier-key-label="modifierKeyLabel"
          :terminal-disconnect-behavior="terminalDisconnectBehavior"
          :terminal-auto-reconnect="terminalAutoReconnect"
          :terminal-render-engine="terminalRenderEngine"
          :show-tab-index="showTabIndex"
          :tab-double-click-behavior="tabDoubleClickBehavior"
          :keyword-highlight-enabled="keywordHighlightEnabled"
          :keyword-highlight-rules="keywordHighlightRules"
          :credential-prompt-rules="credentialPromptRules"
          :credentials="credentialPromptOptions"
          :type-credential="typeCredentialIntoSession"
          :terminal-show-latency="terminalShowLatency"
          :terminal-show-metrics="terminalShowMetrics"
          :translate-enabled="translateEnabled"
          :translate-target="translateTarget"
          :translate-provider="translateProvider"
          :translate-endpoint="translateEndpoint"
          :gutter-mode="gutterMode"
          :gutter-show-timestamps="gutterShowTimestamps"
          :gutter-timestamp-format="gutterTimestampFormat"
          :terminal-batch-command="terminalBatchCommand"
          :session-action-error="sessionActionError"
          :icon-label="sessionIconLabel"
          :session-window="isSessionWindow"
          @select-session="selectSessionTab"
          @close-session="handleCloseSession"
          @reconnect-session="reconnectSessionFromTerminal"
          @disconnect-session="disconnectSessionFromTerminal"
          @open-settings="openTerminalSettings"
          @save-as-snippet="openSnippetCreator"
          @duplicate-session="duplicateSession"
          @toggle-tab-lock="toggleTabLock"
          @tab-locked="notifyTabLocked"
          @rename-session="renameSession"
          @rename-sftp-tab="renameSftpTab"
          @rename-rdp-tab="renameRdpTab"
          @quick-connect="requestQuickConnect"
          @create-local-terminal="openLocalSession"
          @stream-status="handleTerminalStreamStatus"
          @close-sessions="closeSessions"
          @reorder-sessions="reorderSessionTabs"
          @reorder-tabs="applyTabOrder"
          @open-session-window="openSessionInNewWindow"
          @request-split="requestSplit"
          :sftp-tabs="sftpTabs"
          :active-sftp-tab-id="activeSftpTabId"
          :rdp-tabs="rdpTabs"
          :active-rdp-tab-id="activeRdpTabId"
          @select-rdp="selectRdpTab"
          @close-rdp="closeRdpTab"
          @close-tab-scope="closeTabScope"
          @duplicate-rdp-tab="duplicateRdpTab"
          @toggle-rdp-tab-lock="toggleRdpTabLock"
          @rdp-tab-locked="notifyTabLocked"
          @open-desktop-window="openDesktopTabInWindow"
          @select-sftp="selectSftpTab"
          @close-sftp="closeSftpTab"
          @open-sftp="openSftpForSession"
          @ai-analyze="analyzeWithAI"
          @split-change="onSplitChange"
        >
          <template #rdp-workspace="{ tabs, activeTabId }">
            <div
              v-for="tab in tabs"
              v-show="tab.id === activeTabId"
              :key="tab.id"
              class="h-full min-h-0 w-full min-w-0 flex-1"
            >
              <VncCanvasPane
                v-if="(tab.protocol ?? 'rdp') === 'vnc'"
                :key="`vnc-${tab.id}`"
                :ref="(instance) => registerRdpPane(tab.id, instance)"
                :host-id="tab.hostId ?? ''"
                :host-title="tab.title"
                @close="closeRdpTab(tab.id)"
              />
              <RdpCanvasPane
                v-else
                :ref="(instance) => registerRdpPane(tab.id, instance)"
                :host-id="tab.hostId"
                :host-title="tab.title"
                :initial-session-id="tab.sessionId"
                :initial-session-mode="tab.sessionId ? 'main' : 'window'"
                @connection-failed="handleRdpConnectionFailed(tab.id, $event)"
                @opened="handleRdpPaneOpened(tab.id, $event)"
                @close="closeRdpTab(tab.id)"
                @open-window="openRdpInWindow(tab)"
              />
            </div>
          </template>

          <template #sftp-workspace="{ tabs, activeTabId }">
            <SftpWorkspace
              v-for="tab in tabs"
              v-show="tab.id === activeTabId"
              :key="tab.id"
              :gateway="sftpGateway"
              :hosts="hosts"
              :initial-host-id="tab.hostId"
              :transfers="transferRecords"
            />
          </template>
        </SessionWorkspace>

        <div
          v-if="supportsAssetLibrary"
          v-show="showAssetLibrary"
          class="relative z-10 col-start-1 row-start-2 min-h-0 min-w-0 overflow-hidden bg-[var(--qt-asset-library-bg,var(--qt-panel))]"
        >
          <AssetLibraryView
            :search-visible="searchVisible"
            :search-query="searchQuery"
            :sort-mode="sortMode"
            :hosts="hosts"
            :ungrouped-hosts="ungroupedHosts"
            :grouped-hosts="groupedHosts"
            :loading="loadingHosts"
            :load-error="hostLoadError"
            :selected-host-id="selectedHostId"
            :show-host-address="showHostAddress"
            @create-host="requestCreateHostDialog"
            @create-host-in-group="createHostInGroup"
            @create-subgroup="openCreateGroupDialog"
            @create-group="openCreateGroupDialog"
            @toggle-search="searchVisible = !searchVisible"
            @toggle-sort="toggleSort"
            @refresh="loadHosts"
            @select-host="selectHost"
            @select-group="selectGroup"
            @open-host="openHost"
            @edit-host="requestEditHostDialog"
            @delete-host="requestDeleteHost"
            @copy-host="copyHost"
            @edit-group="openEditGroupDialog"
            @delete-group="requestDeleteGroup"
            @update-search-query="searchQuery = $event"
          />
        </div>

        <ResizeHandle
          v-if="activeTool && showToolArea && !activeRdpTabId"
          class="tool-panel-resize"
          orientation="vertical"
          :label="t('shell.app.resizeToolPanel')"
          @resize-start="beginToolPanelResize"
          @resize="resizeToolPanel"
        />

        <ToolPanelView
          v-if="showToolArea && activeTool && !activeRdpTabId"
          :tool="renderedTool"
          :title="activeToolTitle"
          :active-host="activeHost"
          :active-session="activeSession"
          :session-gateway="sessionGateway"
          :sftp-gateway="sftpGateway"
          :docker-gateway="dockerGateway"
          :forwards="portForwards"
          :forward-groups="portForwardGroups"
          :next-hosts="hosts"
          :otp-entries="otpEntries"
          :credentials="credentialOptions"
          @manage-credentials="openOtpManagement"
          :sftp-available="sftpAvailable"
          :directory-follow-mode="sftpDirectoryFollowMode"
          :auto-directory-follow="sftpAutoDirectoryFollow"
          :dark="isDark"
          :terminal-light-theme="terminalLightTheme"
          :terminal-dark-theme="terminalDarkTheme"
          @close="closeActiveTool"
          @update:forwards="setPortForwards"
          @update:forward-groups="setPortForwardGroups"
          @update:otp-entries="setOtpEntries"
          @create-snippet="openSnippetCreator"
          @update-terminal-theme="setTerminalTheme"
        />
      </main>

      <ToolRail
        v-if="showToolRail"
        :active-tool="activeTool"
        :disabled-tools="unsupportedToolPanels"
        :local-session="activeSession?.kind === 'local'"
        :ssh-capable="activeSession?.kind === 'ssh'"
        :can-reconnect="canReconnect"
        :can-disconnect="canDisconnect"
        :rdp-active="Boolean(activeRdpTabId)"
        :vnc-active="activeDesktopProtocol === 'vnc'"
        :vnc-fit="activeRdpPane?.fitEnabled ?? true"
        :rdp-can-paste="activeRdpPane?.canPasteToRemote ?? true"
        :rdp-can-copy="activeRdpPane?.canCopyFromRemote ?? true"
        @toggle-tool="toggleTool"
        @reconnect="reconnectActiveSessionPreservingTool"
        @disconnect="disconnectActiveSession"
        @rdp-paste="activeRdpPane?.pasteToRemote?.()"
        @rdp-copy="activeRdpPane?.copyFromRemote?.()"
        :rdp-window-label="activeDesktopProtocol === 'vnc' ? t('shell.vnc.openWindow') : undefined"
        @rdp-window="openActiveDesktopInWindow"
        @rdp-reconnect="activeRdpPane?.reconnect()"
        @rdp-disconnect="activeRdpPane?.disconnect()"
        @rdp-log="activeRdpPane?.openLog?.()"
        @vnc-fit="activeRdpPane?.toggleFit?.()"
        @vnc-paste="activeRdpPane?.pasteToRemote?.()"
      />
    </div>

    <AssetPickerDialog
      :open="pickerOpen"
      :mode="pickerMode"
      :direction="pickerDirection"
      :hosts="hosts"
      :current="pickerCurrent"
      :show-address="showHostAddress"
      @update:open="pickerOpen = $event"
      @select="selectAsset"
    />

    <HostEditorDialog
      :otp-entries="otpEntries"
      :open="hostDialogOpen"
      :editing="!!editingHostId"
      :persistent="hostGateway.persistent"
      :draft="draft"
      :groups="groups"
      :auth-mode="authMode"
      :direct-secret="directSecret"
      :credentials="credentialOptions"
      :credential-gateway="credentialGateway"
      :proxy-gateway="proxyGateway"
      :hosts="hosts"
      :proxies="proxies"
      :editing-host-id="editingHostId || undefined"
      :vault-status="credentialVaultStatus"
      :credentials-loading="credentialOptionsLoading"
      :credentials-error="credentialOptionsError"
      :form-error="formError"
      :testing-connection="testingConnection"
      :detecting-icon="detectingIcon"
      :saving="savingHost"
      :credential-label="credentialKindLabel"
      @close="closeHostDialog"
      @submit="saveHost"
      @test-connection="testHostConnection"
      @detect-icon="detectHostIcon"
      @open-security="openSettings('application')"
      @create-group="openCreateGroupForHost"
      @credential-created="handleCredentialCreated"
      @proxy-created="handleProxyCreated"
      @update-draft="Object.assign(draft, $event)"
      @update-auth-mode="setAuthMode"
      @update-password-source="setPasswordSource"
      @update-secret="directSecret = $event"
    />
    <QuickConnectDialog :open="adhocConnectOpen" @close="adhocConnectOpen = false" @opened="handleAdHocOpened" />

    <ImportConfigDialog
      :open="importConfigOpen"
      :credentials="credentialOptions"
      :vault-status="credentialVaultStatus"
      @close="closeWindowedDialog(() => (importConfigOpen = false))"
      @imported="windowedDialogDone(resourcesImported)"
    />

    <ExportConfigDialog
      :open="exportConfigOpen"
      :gateway="credentialGateway"
      @close="closeWindowedDialog(() => (exportConfigOpen = false))"
      @exported="windowedDialogDone(resourcesImported)"
    />

    <AiChatDialog
      :open="aiChatOpen"
      :terminal-context="aiTerminalContextSessionId === activeSessionId ? aiTerminalContext : undefined"
      :host-id="sessions.find((session) => session.id === activeSessionId)?.hostId ?? ''"
      :session-id="activeSessionId ?? ''"
      :session-title="activeSessionTitle"
      :session-state="sessions.find((session) => session.id === activeSessionId)?.state ?? 'closed'"
      @close="aiChatOpen = false"
    />

    <CommandPaletteDialog
      :open="paletteOpen"
      :items="paletteItems"
      @update:open="paletteOpen = $event"
      @select="runPaletteItem"
    />

    <SettingsDialog
      :open="securityDialogOpen"
      :category="settingsCategory"
      :locked="settingsLocked"
      :status="credentialVaultStatus"
      :working="securityWorking"
      :error="securityError"
      :theme-mode="themeMode"
      :minimal-appearance="minimalAppearance"
      :ui-scale="uiScale"
      :ui-font="uiFont"
      :dark="isDark"
      :show-host-address="showHostAddress"
      :rdp-show-resolution="rdpShowResolution"
      :rdp-first-frame-timeout-seconds="rdpFirstFrameTimeoutSeconds"
      :rdp-host-probe-timeout-seconds="rdpHostProbeTimeoutSeconds"
      :rdp-fit-grace-seconds="rdpFitGraceSeconds"
      :rdp-show-fps="rdpShowFps"
      :light-background-enabled="lightBackgroundEnabled"
      :light-background-opacity="lightBackgroundOpacity"
      :light-background-image="lightBackgroundImage"
      :dark-background-enabled="darkBackgroundEnabled"
      :dark-background-opacity="darkBackgroundOpacity"
      :dark-background-image="darkBackgroundImage"
      :terminal-light-theme="terminalLightTheme"
      :terminal-dark-theme="terminalDarkTheme"
      :terminal-theme-background-scope="terminalThemeBackgroundScope"
      :background-enabled="backgroundEnabled"
      :terminal-font-size="terminalFontSize"
      :terminal-line-height="terminalLineHeight"
      :terminal-font-family="terminalFontFamily"
      :auto-copy-selection="terminalAutoCopySelection"
      :copy-feedback="terminalCopyFeedback"
      :show-latency="terminalShowLatency"
      :show-metrics="terminalShowMetrics"
      :translate-enabled="translateEnabled"
      :translate-target="translateTarget"
      :translate-provider="translateProvider"
      :translate-endpoint="translateEndpoint"
      :gutter-mode="gutterMode"
      :gutter-show-timestamps="gutterShowTimestamps"
      :gutter-timestamp-format="gutterTimestampFormat"
      :batch-command="terminalBatchCommand"
      :confirm-close="terminalConfirmClose"
      :scrollback="terminalScrollback"
      :confirm-multi-line-paste="terminalConfirmMultiLinePaste"
      :cursor-blink="terminalCursorBlink"
      :record-command-history="terminalRecordCommandHistory"
      :completion-enabled="terminalCompletionEnabled"
      :history-min-length="terminalHistoryMinLength"
      :history-max-length="terminalHistoryMaxLength"
      :word-separators="terminalWordSeparators"
      :mac-option-is-meta="terminalMacOptionIsMeta"
      :cursor-style="terminalCursorStyle"
      :right-click-behavior="terminalRightClickBehavior"
      :modifier-right-click-behavior="terminalModifierRightClickBehavior"
      :disconnect-behavior="terminalDisconnectBehavior"
      :auto-reconnect="terminalAutoReconnect"
      :render-engine="terminalRenderEngine"
      :show-tab-index="showTabIndex"
      :tab-double-click-behavior="tabDoubleClickBehavior"
      :locale="appLocale"
      :asset-library-layout="assetLibraryLayout"
      :host-color-style="hostColorStyle"
      :session-history-limit="sessionHistoryLimit"
      :record-all-sessions="recordAllSessions"
      :sftp-directory-follow-mode="sftpDirectoryFollowMode"
      :sftp-drag-folder-upload-mode="sftpDragFolderUploadMode"
      :sftp-auto-directory-follow="sftpAutoDirectoryFollow"
      :sftp-show-hidden-files="sftpShowHiddenFiles"
      :sftp-max-transfers="sftpMaxTransfers"
      :sftp-local-directory="sftpLocalDirectory"
      :recording-directory="recordingDirectory"
      :recording-directory-effective="recordingDirectoryEffective"
      :sftp-editor-path="sftpEditorPath"
      :x11-display="x11Display"
      :x11-xauth-path="x11XauthPath"
      :x11-authority-file="x11AuthorityFile"
      :window-remember-size="windowRememberSize"
      :close-to-tray="closeToTray"
      :restore-open-sessions="restoreOpenSessions"
      :quit-confirm="quitConfirm"
      :settings-in-window="settingsInWindow"
      @update:settings-in-window="setAppSetting('settingsInWindow', $event)"
      :window-remember-position="windowRememberPosition"
      :keyword-highlight-enabled="keywordHighlightEnabled"
      :keyword-highlight-rules="keywordHighlightRules"
      :local-shell-path="localShellPath"
      :local-shell-working-directory="localShellWorkingDirectory"
      :shortcuts="shortcuts"
      :credential-gateway="credentialGateway"
      :otp-entries="otpEntries"
      :credential-view="credentialView"
      @update:otp-entries="setOtpEntries"
      :credential-prompt-rule-gateway="credentialPromptRuleGateway"
      :proxy-gateway="proxyGateway"
      :proxies="proxies"
      :proxies-loading="proxiesLoading"
      :proxies-error="proxiesError"
      :forwards="portForwards"
      :forward-groups="portForwardGroups"
      :credentials="credentialOptions"
      :hosts="hosts"
      :credentials-loading="credentialOptionsLoading"
      :credentials-error="credentialOptionsError"
      :close-on-escape="isSettingsWindow"
      :page="isSettingsWindow"
      @close="closeSecuritySettings"
      @update:category="settingsCategory = $event"
      @update:theme-mode="setThemeMode"
      @update:minimal-appearance="setMinimalAppearance"
      @update:ui-scale="setUIScale"
      @update:ui-font="setUIFont"
      @update:show-host-address="setShowHostAddress"
      @update:rdp-show-resolution="setRdpShowResolution"
      @update:rdp-first-frame-timeout-seconds="setRdpFirstFrameTimeoutSeconds"
      @update:rdp-host-probe-timeout-seconds="setRdpHostProbeTimeoutSeconds"
      @update:rdp-fit-grace-seconds="setRdpFitGraceSeconds"
      @update:rdp-show-fps="setRdpShowFps"
      @update:light-background-enabled="setBackgroundSetting('lightBackgroundEnabled', $event)"
      @update:light-background-opacity="setBackgroundSetting('lightBackgroundOpacity', $event)"
      @update:light-background-image="setBackgroundSetting('lightBackgroundImage', $event)"
      @update:dark-background-enabled="setBackgroundSetting('darkBackgroundEnabled', $event)"
      @update:dark-background-opacity="setBackgroundSetting('darkBackgroundOpacity', $event)"
      @update:dark-background-image="setBackgroundSetting('darkBackgroundImage', $event)"
      @update:terminal-light-theme="setTerminalTheme('light', $event)"
      @update:terminal-dark-theme="setTerminalTheme('dark', $event)"
      @update:terminal-theme-background-scope="setTerminalThemeBackgroundScope"
      @update:terminal-font-size="setTerminalFontSize"
      @update:terminal-line-height="setTerminalLineHeight"
      @update:terminal-font-family="setTerminalFontFamily"
      @reset-terminal-typography="resetTerminalTypography"
      @update:auto-copy-selection="setTerminalSetting('terminalAutoCopySelection', $event)"
      @update:copy-feedback="setTerminalSetting('terminalCopyFeedback', $event)"
      @update:show-latency="setTerminalSetting('terminalShowLatency', $event)"
      @update:show-metrics="setTerminalSetting('terminalShowMetrics', $event)"
      @update:gutter-mode="setGutterMode"
      @update:translate-enabled="setTranslateSetting('translateEnabled', $event)"
      @update:translate-target="setTranslateSetting('translateTarget', $event)"
      @update:translate-provider="setTranslateSetting('translateProvider', $event)"
      @update:translate-endpoint="setTranslateSetting('translateEndpoint', $event)"
      @update:gutter-show-timestamps="setGutterShowTimestamps"
      @update:gutter-timestamp-format="setGutterTimestampFormat"
      @update:batch-command="setTerminalSetting('terminalBatchCommand', $event)"
      @update:confirm-close="setTerminalSetting('terminalConfirmClose', $event)"
      @update:scrollback="setTerminalScrollback($event)"
      @update:confirm-multi-line-paste="setTerminalSetting('terminalConfirmMultiLinePaste', $event)"
      @update:cursor-blink="setTerminalSetting('terminalCursorBlink', $event)"
      @update:record-command-history="setTerminalSetting('terminalRecordCommandHistory', $event)"
      @update:completion-enabled="setTerminalSetting('terminalCompletionEnabled', $event)"
      @update:history-min-length="setTerminalNumberSetting('terminalHistoryMinLength', $event)"
      @update:history-max-length="setTerminalNumberSetting('terminalHistoryMaxLength', $event)"
      @update:word-separators="setTerminalWordSeparators"
      @update:mac-option-is-meta="setTerminalSetting('terminalMacOptionIsMeta', $event)"
      @update:cursor-style="setTerminalCursorStyle"
      @update:right-click-behavior="setTerminalRightClickBehavior"
      @update:modifier-right-click-behavior="setTerminalModifierRightClickBehavior"
      @update:disconnect-behavior="setTerminalDisconnectBehavior"
      @update:auto-reconnect="setTerminalSetting('autoReconnect', $event)"
      @update:render-engine="setTerminalRenderEngine"
      @update:show-tab-index="setTerminalSetting('showTabIndex', $event)"
      @update:tab-double-click-behavior="setTabDoubleClickBehavior"
      @update:local-shell-path="setLocalShellSetting('localShellPath', $event)"
      @update:local-shell-working-directory="setLocalShellSetting('localShellWorkingDirectory', $event)"
      @update:locale="setAppSetting('locale', $event)"
      @update:asset-library-layout="setAppSetting('assetLibraryLayout', $event)"
      @update:host-color-style="setAppSetting('hostColorStyle', $event)"
      @update:session-history-limit="setAppSetting('sessionHistoryLimit', $event)"
      @update:record-all-sessions="setAppSetting('recordAllSessions', $event)"
      @update:sftp-directory-follow-mode="setAppSetting('sftpDirectoryFollowMode', $event)"
      @update:sftp-drag-folder-upload-mode="setAppSetting('sftpDragFolderUploadMode', $event)"
      @update:sftp-auto-directory-follow="setAppSetting('sftpAutoDirectoryFollow', $event)"
      @update:sftp-show-hidden-files="setAppSetting('sftpShowHiddenFiles', $event)"
      @update:sftp-max-transfers="setAppSetting('sftpMaxTransfers', $event)"
      @update:sftp-local-directory="setAppSetting('sftpLocalDirectory', $event)"
      @update:recording-directory="onRecordingDirectoryChange"
      @update:sftp-editor-path="setAppSetting('sftpEditorPath', $event)"
      @update:x11-display="setAppSetting('x11Display', $event)"
      @update:x11-xauth-path="setAppSetting('x11XauthPath', $event)"
      @update:x11-authority-file="setAppSetting('x11AuthorityFile', $event)"
      @update:window-remember-size="setAppSetting('windowRememberSize', $event)"
      @update:close-to-tray="setAppSetting('closeToTray', $event)"
      @update:restore-open-sessions="setAppSetting('restoreOpenSessions', $event)"
      @update:quit-confirm="setAppSetting('quitConfirm', $event)"
      @update:window-remember-position="setAppSetting('windowRememberPosition', $event)"
      @update:keyword-highlight-enabled="setKeywordHighlightEnabled"
      @update:keyword-highlight-rules="setKeywordHighlightRules"
      @update:shortcut="setShortcut"
      @setup="setupMasterPassword"
      @unlock="unlockWithPassword"
      @unlock-automatic="unlockAutomatically"
      @change-password="changeMasterPassword"
      @change-encryption-key="changeEncryptionKey"
      @unlock-custom-key="unlockWithCustomKey"
      @set-auto-unlock="setAutoUnlock"
      @export-recovery="exportRecovery"
      @import-recovery="importRecovery"
      @reset-vault="resetVault"
      @lock="lockCredentialVault"
      @refresh-credentials="refreshCredentials"
      @refresh-proxies="loadProxies"
      @update:forwards="setPortForwards"
      @update:forward-groups="setPortForwardGroups"
      @refresh-credential-prompt-rules="loadCredentialPromptRules"
      @resources-imported="resourcesImported"
    />
    <SnippetsSettingsPanel ref="standaloneSnippetPanel" standalone />
    <GroupEditorDialog
      :open="groupDialogOpen"
      :editing="!!editingGroupId"
      :name="groupName"
      :error="groupFormError"
      :saving="savingGroup"
      @close="closeGroupDialog"
      @submit="saveGroup"
      @update-name="groupName = $event"
    />
    <DeleteAssetDialog
      :target="pendingDelete"
      :deleting="deletingAsset"
      :error="deleteError"
      @close="closeDeleteDialog"
      @confirm="confirmDelete"
    />
    <DeleteConfirmDialog
      :open="quitConfirmOpen"
      :title="t('shell.quitConfirm.title')"
      :message="t('shell.quitConfirm.message')"
      :confirm-text="t('shell.quitConfirm.confirm')"
      :show-close-button="false"
      :background-enabled="backgroundEnabled"
      @close="answerQuit(false)"
      @confirm="answerQuit(true)"
    />
    <DeleteConfirmDialog
      :open="closeConfirmIds.length > 0"
      :title="t('shell.app.closeTerminalTitle')"
      :message="
        closeConfirmIds.length > 1
          ? t('shell.app.closeTerminalMany', { count: closeConfirmIds.length })
          : t('shell.app.closeTerminalOne')
      "
      :confirm-text="t('shell.app.close')"
      :show-close-button="false"
      :background-enabled="backgroundEnabled"
      @close="cancelCloseSessions"
      @confirm="confirmCloseSessions"
    />
    <HostKeyDialog
      :pending="pendingHostKey"
      :confirming="confirmingHostKey"
      :error="hostKeyError"
      @reject="rejectPendingHostKey"
      @trust="trustPendingHostKey"
    />
    <SSHAuthenticationDialog
      :hosts="hosts"
      :pending="pendingAuthentication"
      :retry="authenticationRetry"
      :otp-entries="otpEntries"
      :credentials="credentialOptions"
      :credential-label="credentialKindLabel"
      :submitting="submittingAuthentication"
      :can-remember="!pendingAuthenticationIsAdHoc"
      @submit="submitAuthentication"
      @cancel="cancelAuthentication"
    />
    <ZmodemDialog ref="zmodemDialogRef" />

    <RemoteTextEditDialog
      v-if="remoteEditorRequest"
      :session-id="remoteEditorRequest.sessionId"
      :remote-path="remoteEditorRequest.remotePath"
      :name="remoteEditorRequest.name"
      @close="closeRemoteEditor"
    />

    <RemoteEditConflictDialog
      :open="Boolean(remoteEditConflict)"
      kind="external"
      :remote-path="remoteEditConflict?.remotePath ?? ''"
      :busy="remoteEditConflictBusy"
      @choose="chooseExternalConflict"
    />
    <TransferConflictDialog />
    <SftpTransferConflicts :gateway="sftpGateway" :primary-window="isMainWindow" />
    <QtermToaster
      position="top-center"
      :rich-colors="true"
      :close-button="false"
      :expand="false"
      :visible-toasts="3"
      :duration="3600"
    />
    <TransferProgressPanel
      :items="progressItems"
      @action="handleTransferAction"
      @dismiss="transferState.dismissTransfer"
    />

    <AppLockScreen v-if="appLocked" :theme-style="appLockThemeStyle" />
  </div>

  <div
    v-if="isMainWindow && settingsWindowOpen && !appLocked"
    class="qterm-dialog-overlay settings-window-scrim fixed inset-0 isolate z-[70] bg-black/10"
    @click="focusSettingsWindow"
  ></div>
</template>
