<script setup lang="ts">
import { QtermButton } from '../common'
import {
  Activity,
  HardDrive,
  Laptop,
  LayoutTemplate,
  Lock,
  Maximize,
  Maximize2,
  Minimize2,
  Network,
  Radio,
  SquareTerminal,
  FolderOpen,
  Monitor,
  X,
  Zap,
} from '@lucide/vue'
import {
  computed,
  defineAsyncComponent,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
  type CSSProperties,
} from 'vue'
import { useResizeObserver } from '@vueuse/core'
import SessionContextMenu, { type SessionMenuCloseOperation } from './SessionContextMenu.vue'
import {
  nearestPaneInDirection,
  paneStyle,
  useSplitWorkspace,
  type PaneFocusDirection,
  type SplitDirection,
  type SplitResizeHandle,
} from '../../features/sessions/useSplitWorkspace'
import { useSessionTabDrag } from '../../features/sessions/useSessionTabDrag'
import { evenPixel } from '../../lib/geometry'
import HostIcon from '../common/icon/HostIcon.vue'
import { CPUIcon, GPUIcon, MemoryIcon } from '../common/icon'
import ResizeHandle from '../common/layout/ResizeHandle.vue'
import { latencyDisplay } from '../../features/sessions/latency'
import type { HostRecord } from '@/services/hosts.ts'
import type { SessionTab, SessionStatus, StreamStatusEvent } from '../../types/qterm'
import type { ResourceSnapshot, SessionGateway } from '../../services/sessions'
import { createMonitorGateway } from '../../services/monitor'
import { formatGpuMemory, gpuMemorySummary, type GpuMemorySummary } from '../../features/monitor/gpuMemory'
import { Separator } from '../common/control-primitives'
import BatchCommandBar from '../../features/sessions/BatchCommandBar.vue'
import SyncTargetPicker from '../../features/terminal-control/components/SyncTargetPicker.vue'
import SyncStatusBar from '../../features/terminal-control/components/SyncStatusBar.vue'
import BatchSendPanel from '../../features/terminal-control/components/BatchSendPanel.vue'
import { useTerminalControl } from '../../features/terminal-control/composables/useTerminalControl'
import type { TerminalControlSession } from '../../features/terminal-control/types'
import { useI18n } from '@/locales'

export interface RdpWorkspaceTab {
  id: string
  title: string

  protocol?: 'rdp' | 'vnc'

  locked?: boolean

  hostId?: string

  sessionId?: string

  host?: string
  port?: number
  username?: string
  width?: number
  height?: number

  order?: number
}

export interface SftpWorkspaceTab {
  id: string
  title: string

  hostId: string | null

  order: number
}

export interface WorkspaceTabOrderEntry {
  kind: 'session' | 'group' | 'sftp' | 'rdp'

  id: string

  order: number

  sessionIds?: string[]
}

const TerminalPane = defineAsyncComponent(() => import('../../features/sessions/TerminalPane.vue'))

const props = defineProps<{
  sessions: SessionTab[]
  terminalSessions: Array<SessionTab & { host?: HostRecord }>
  activeSessionId: string | null
  activeSession: SessionTab | null
  activeHost: HostRecord | null
  activeStatus: SessionStatus
  persistent: boolean
  dark: boolean
  transparentBackground?: boolean

  terminalFontSize: number
  terminalFontWeight?: import('../../features/settings/settingsStore').TerminalFontWeight

  terminalLineHeight?: number
  terminalFontFamily: string

  settingsReady?: boolean
  terminalLightTheme?: import('../../features/sessions/terminalThemes').TerminalThemeId
  terminalDarkTheme?: import('../../features/sessions/terminalThemes').TerminalThemeId
  terminalAutoCopySelection: boolean
  terminalCopyFeedback: boolean

  terminalScrollback?: number

  terminalConfirmMultiLinePaste?: boolean
  terminalCompletionEnabled?: boolean
  terminalHistoryMinLength?: number
  terminalHistoryMaxLength?: number
  terminalWordSeparators?: string

  terminalMacOptionIsMeta?: boolean
  terminalCursorBlink: boolean
  terminalCursorStyle: 'block' | 'underline' | 'bar'
  terminalRightClickBehavior?: 'none' | 'paste' | 'menu'
  terminalModifierRightClickBehavior?: 'none' | 'paste' | 'menu'
  terminalDisconnectBehavior?: 'reconnect' | 'close'

  terminalAutoReconnect?: boolean

  closeConfirmIds?: string[]
  terminalRenderEngine?: import('../../features/settings/settingsStore').TerminalRenderEngine

  showTabIndex?: boolean

  tabDoubleClickBehavior?: import('../../features/settings/settingsStore').TabDoubleClickBehavior
  keywordHighlightEnabled?: boolean
  keywordHighlightRules?: import('../../features/settings/settingsStore').KeywordHighlightRule[]
  credentialPromptRules?: import('../../features/sessions/credentialAutofill').CredentialPromptRule[]
  credentials?: import('../../features/sessions/credentialAutofill').PromptCredential[]
  typeCredential?: (sessionId: string, credentialId: string) => Promise<void>
  terminalShowLatency: boolean
  terminalShowMetrics?: boolean

  gutterMode?: import('../../features/settings/settingsStore').TerminalGutterMode

  translateEnabled?: boolean
  translateTarget?: string
  translateProvider?: import('../../services/translate').TranslateProvider
  translateEndpoint?: string
  gutterShowTimestamps?: boolean

  gutterTimestampFormat?: import('../../features/settings/settingsStore').TerminalGutterTimestampFormat
  terminalBatchCommand?: boolean
  sessionGateway?: SessionGateway
  sftpTabs?: SftpWorkspaceTab[]
  rdpTabs?: RdpWorkspaceTab[]
  activeRdpTabId?: string | null
  activeSftpTabId?: string | null
  sessionActionError: string
  iconLabel: (session: SessionTab) => string
  sessionWindow?: boolean

  assetLibraryCovered?: boolean
}>()
const { t } = useI18n()
const terminalControl = useTerminalControl(() => props.terminalSessions.map((session) => session.id))
const {
  group: syncGroup,
  task: batchTask,
  log: batchLog,
  busy: terminalControlBusy,
  error: terminalControlError,
  pickerOpen: syncPickerOpen,
  pickerMode: syncPickerMode,
  sourceId: controlSourceId,
  batchOpen: batchSendOpen,
} = terminalControl

const emit = defineEmits<{
  selectSession: [sessionId: string]

  fontSizeStep: [delta: number]
  selectSftp: [tabId: string]
  closeSession: [sessionId: string]

  toggleTabLock: [sessionId: string]

  tabLocked: [sessionId: string]
  reconnectSession: [sessionId: string]
  disconnectSession: [sessionId: string]
  openSettings: []
  closeSftp: [tabId: string]
  closeRdp: [tabId: string]

  duplicateRdpTab: [tabId: string]

  toggleRdpTabLock: [tabId: string]

  rdpTabLocked: [tabId: string]

  closeTabScope: [payload: { sessionIds: string[]; sftpTabIds: string[]; rdpTabIds: string[] }]

  openDesktopWindow: [tabId: string]
  selectRdp: [tabId: string]
  openSftp: [sessionId: string]
  aiAnalyze: [sessionId: string, kind: 'selection' | 'recent', text: string]

  saveAsSnippet: [content: string]
  createTerminal: []

  quickConnect: []

  createLocalTerminal: []
  streamStatus: [value: StreamStatusEvent]
  splitChange: [value: boolean]

  closeSessions: [sessionIds: string[]]
  openSessionWindow: [sessionId: string]
  duplicateSession: [sessionId: string]

  renameSession: [sessionId: string, title: string]

  renameSftpTab: [tabId: string, title: string]
  renameRdpTab: [tabId: string, title: string]
  reorderSessions: [value: { sourceId: string; targetId: string; position?: 'before' | 'after' }]

  reorderTabs: [value: WorkspaceTabOrderEntry[]]

  requestSplit: [sessionId: string, direction: SplitDirection]
}>()

type SplitGroupInfo = { id: string; count: number; firstSessionId: string }

type WorkspaceTabItem =
  | { type: 'session'; session: SessionTab; order: number }
  | { type: 'group'; group: SplitGroupInfo; order: number }
  | { type: 'sftp'; tab: SftpWorkspaceTab; order: number }
  | { type: 'rdp'; tab: RdpWorkspaceTab; order: number }

const stage = ref<HTMLElement | null>(null)
const tabBar = ref<HTMLElement | null>(null)
const stageSize = ref({ width: 0, height: 0 })
let revealActiveTabFrame: number | null = null

function revealActiveTab(): void {
  void nextTick(() => {
    if (revealActiveTabFrame !== null) window.cancelAnimationFrame(revealActiveTabFrame)
    revealActiveTabFrame = window.requestAnimationFrame(() => {
      revealActiveTabFrame = null
      const bar = tabBar.value
      const active = bar?.querySelector<HTMLElement>('.session-tab.is-active, .session-tab.is-split-active')
      if (!bar || !active) return
      const tabs = Array.from(bar.querySelectorAll<HTMLElement>('.session-tab'))
      if (tabs.at(-1) === active) {
        bar.scrollLeft = bar.scrollWidth
        return
      }
      active?.scrollIntoView?.({ block: 'nearest', inline: 'nearest' })
    })
  })
}

watch(
  [
    () => props.activeSessionId,
    () => props.activeSftpTabId,
    () => props.activeRdpTabId,
    () => props.sessions.length,
    () => props.sftpTabs?.length ?? 0,
    () => props.rdpTabs?.length ?? 0,
  ],
  revealActiveTab,
  { flush: 'post' },
)

onBeforeUnmount(() => {
  if (revealActiveTabFrame !== null) window.cancelAnimationFrame(revealActiveTabFrame)
})
const sftpActive = () => Boolean(props.activeSftpTabId)
const rdpActive = () => Boolean(props.activeRdpTabId)

const auxiliaryActive = () => sftpActive() || rdpActive()

const tabsCovered = () => Boolean(props.assetLibraryCovered)

const stageCovered = () => auxiliaryActive() || tabsCovered()

useResizeObserver(stage, (entries) => {
  const entry = entries[0]
  if (entry) stageSize.value = { width: entry.contentRect.width, height: entry.contentRect.height }
})

function requestCloseSession(sessionId: string): void {
  const session = props.sessions.find((item) => item.id === sessionId)
  if (session?.locked) {
    emit('tabLocked', sessionId)
    return
  }
  emit('closeSession', sessionId)
}

interface TabCloseUnit {
  key: string
  sessionIds: string[]
  sftpTabId?: string
  rdpTabId?: string
  locked: boolean
}

function unitKeyOf(item: WorkspaceTabItem): string {
  if (item.type === 'session') return `session:${item.session.id}`
  if (item.type === 'group') return `group:${item.group.id}`
  if (item.type === 'sftp') return `sftp:${item.tab.id}`
  return `rdp:${item.tab.id}`
}

function tabCloseUnits(): TabCloseUnit[] {
  return allTabs.value.map((item) => {
    if (item.type === 'session') {
      return { key: unitKeyOf(item), sessionIds: [item.session.id], locked: item.session.locked === true }
    }
    if (item.type === 'group') {
      return { key: unitKeyOf(item), sessionIds: groupSessionIds(item.group.id), locked: false }
    }
    if (item.type === 'sftp') {
      return { key: unitKeyOf(item), sessionIds: [], sftpTabId: item.tab.id, locked: false }
    }
    return { key: unitKeyOf(item), sessionIds: [], rdpTabId: item.tab.id, locked: item.tab.locked === true }
  })
}

function closeUnitsFor(unitKey: string, scope: 'left' | 'right' | 'others' | 'all'): TabCloseUnit[] {
  const units = tabCloseUnits().filter((unit) => !unit.locked)
  if (scope === 'all') return units
  const index = units.findIndex((unit) => unit.key === unitKey)
  if (index < 0) return []
  if (scope === 'left') return units.slice(0, index)
  if (scope === 'right') return units.slice(index + 1)
  return units.filter((unit) => unit.key !== unitKey)
}

function canCloseScopeFor(unitKey: string, scope: 'left' | 'right' | 'others'): boolean {
  return closeUnitsFor(unitKey, scope).length > 0
}

function requestCloseScope(unitKey: string, scope: SessionMenuCloseOperation): void {
  if (scope === 'tab') return
  const units = closeUnitsFor(unitKey, scope)
  if (!units.length) return
  emit('closeTabScope', {
    sessionIds: units.flatMap((unit) => unit.sessionIds),
    sftpTabIds: units.flatMap((unit) => (unit.sftpTabId ? [unit.sftpTabId] : [])),
    rdpTabIds: units.flatMap((unit) => (unit.rdpTabId ? [unit.rdpTabId] : [])),
  })
}

function closeSessions(ids: string[]): void {
  emit('closeSessions', ids)
}

function scrollTabs(event: WheelEvent): void {
  const element = tabBar.value
  if (!element || element.scrollWidth <= element.clientWidth) return
  const delta = Math.abs(event.deltaY) > Math.abs(event.deltaX) ? event.deltaY : event.deltaX
  if (!delta) return
  event.preventDefault()
  element.scrollLeft += delta
}

const hasTabs = computed(
  () => (props.sessions?.length ?? 0) > 0 || (props.sftpTabs?.length ?? 0) > 0 || (props.rdpTabs?.length ?? 0) > 0,
)

const TAB_ORDER_FALLBACK = Number.MAX_SAFE_INTEGER

const TAB_KIND_RANK: Record<WorkspaceTabItem['type'], number> = { session: 0, group: 1, sftp: 2, rdp: 3 }

const sessionTabIndex = computed(() => new Map(props.sessions.map((session, index) => [session.id, index])))

function finiteTabOrder(value: number | undefined): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

function sessionTabOrder(session: SessionTab): number {
  return finiteTabOrder(session.order) ?? sessionTabIndex.value.get(session.id) ?? 0
}

function sessionTabOrderById(sessionID: string): number | null {
  const session = props.sessions.find((item) => item.id === sessionID)
  return session ? sessionTabOrder(session) : null
}

function auxiliaryTabOrder(tab: { order?: number }): number {
  return finiteTabOrder(tab.order) ?? TAB_ORDER_FALLBACK
}

const allTabs = computed<WorkspaceTabItem[]>(() => {
  const items: WorkspaceTabItem[] = []
  for (const session of standaloneSessions.value) {
    items.push({ type: 'session', session, order: sessionTabOrder(session) })
  }

  for (const group of splitGroups.value) {
    items.push({
      type: 'group',
      group,
      order: finiteTabOrder(groupOrders.value[group.id]) ?? sessionTabOrderById(group.firstSessionId) ?? 0,
    })
  }
  for (const tab of props.sftpTabs ?? []) {
    items.push({ type: 'sftp', tab, order: auxiliaryTabOrder(tab) })
  }
  for (const tab of props.rdpTabs ?? []) {
    items.push({ type: 'rdp', tab, order: auxiliaryTabOrder(tab) })
  }
  items.sort((left, right) => left.order - right.order || TAB_KIND_RANK[left.type] - TAB_KIND_RANK[right.type])
  return items
})

function domTabId(item: WorkspaceTabItem): string {
  if (item.type === 'session') return item.session.id
  if (item.type === 'group') return item.group.firstSessionId
  return item.tab.id
}

const tabOrderIndex = computed(() => {
  const map = new Map<string, number>()
  for (const item of allTabs.value) {
    map.set(domTabId(item), item.order)
    if (item.type === 'group') map.set(item.group.id, item.order)
  }
  return map
})

function tabOrderOf(id: string): number | null {
  return tabOrderIndex.value.get(id) ?? null
}

function tabTitle(id: string): string {
  const session = props.sessions.find((item) => item.id === id)
  if (session) return session.title
  const tab =
    (props.sftpTabs ?? []).find((item) => item.id === id) ?? (props.rdpTabs ?? []).find((item) => item.id === id)
  return tab?.title ?? ''
}

function applyTabBarReorder(event: { sourceId: string; targetId: string; position: 'before' | 'after' }): void {
  const items = allTabs.value
  const sourceIndex = items.findIndex((item) => domTabId(item) === event.sourceId)
  if (sourceIndex < 0) return
  const source = items[sourceIndex]!
  const rest = items.filter((_, index) => index !== sourceIndex)
  const anchor = rest.findIndex((item) => domTabId(item) === event.targetId)
  if (anchor < 0) return
  const next = [...rest]
  next.splice(event.position === 'after' ? anchor + 1 : anchor, 0, source)

  const entries: WorkspaceTabOrderEntry[] = []
  next.forEach((item, index) => {
    const order = index + 1
    if (item.type === 'session') entries.push({ kind: 'session', id: item.session.id, order })
    else if (item.type === 'sftp') entries.push({ kind: 'sftp', id: item.tab.id, order })
    else if (item.type === 'rdp') entries.push({ kind: 'rdp', id: item.tab.id, order })
    else {
      setGroupOrder(item.group.id, order)
      entries.push({ kind: 'group', id: item.group.id, sessionIds: groupSessionIds(item.group.id), order })
    }
  })
  emit('reorderTabs', entries)

  const anchorItem = rest[anchor]!
  const targetSessionId =
    anchorItem.type === 'session'
      ? anchorItem.session.id
      : anchorItem.type === 'group'
        ? anchorItem.group.firstSessionId
        : null
  if (source.type === 'session' && targetSessionId) {
    emit('reorderSessions', { sourceId: source.session.id, targetId: targetSessionId, position: event.position })
  }
}

const sessionOrdinals = computed(() => new Map(props.sessions.map((session, index) => [session.id, index + 1])))

function tabOrdinal(sessionId: string): number | null {
  if (props.showTabIndex !== true) return null
  return sessionOrdinals.value.get(sessionId) ?? null
}

function tabLabel(sessionId: string, title: string, autoReconnectPending = false): string {
  const ordinal = tabOrdinal(sessionId)
  const label = ordinal === null ? title : `${ordinal} · ${title}`

  return autoReconnectPending ? `${label} · ${t('shell.terminal.autoReconnectPending')}` : label
}

const renamingTab = ref<{ kind: 'session' | 'sftp' | 'rdp'; id: string } | null>(null)

const openMenuKey = ref<string | null>(null)

function setMenuOpen(unitKey: string, open: boolean): void {
  if (open) openMenuKey.value = unitKey
  else if (openMenuKey.value === unitKey) openMenuKey.value = null
}

const renameDraft = ref('')

function isRenaming(kind: 'session' | 'sftp' | 'rdp', id: string): boolean {
  return renamingTab.value?.kind === kind && renamingTab.value.id === id
}

function requestCloseRdp(tabId: string): void {
  const tab = props.rdpTabs?.find((item) => item.id === tabId)
  if (tab?.locked) {
    emit('rdpTabLocked', tabId)
    return
  }
  emit('closeRdp', tabId)
}

function handleTabDoubleClick(item: WorkspaceTabItem, event: MouseEvent): void {
  if ((event.target as HTMLElement | null)?.closest('.session-tab-rename')) return
  if (props.tabDoubleClickBehavior !== 'duplicate') {
    if (item.type === 'session') beginRename('session', item.session.id, item.session.title)
    else if (item.type === 'sftp') beginRename('sftp', item.tab.id, item.tab.title)
    else if (item.type === 'rdp') beginRename('rdp', item.tab.id, item.tab.title)

    return
  }
  if (item.type === 'session') emit('duplicateSession', item.session.id)
}

function beginRename(kind: 'session' | 'sftp' | 'rdp', id: string, currentTitle: string): void {
  if (isRenaming(kind, id)) return
  renamingTab.value = { kind, id }
  renameDraft.value = currentTitle

  const focusInput = (): void => {
    const input = tabBar.value?.querySelector<HTMLInputElement>('.session-tab-rename')
    if (!input) return
    if (document.activeElement === input) return
    input.focus()
    input.select()
  }
  void nextTick(() => {
    focusInput()
    if (typeof requestAnimationFrame !== 'function') return
    requestAnimationFrame(() => requestAnimationFrame(focusInput))
  })
}

function commitRename(): void {
  const target = renamingTab.value
  if (!target) return
  renamingTab.value = null
  const title = renameDraft.value.trim()
  if (!title) return
  if (target.kind === 'session') emit('renameSession', target.id, title)
  else if (target.kind === 'sftp') emit('renameSftpTab', target.id, title)
  else emit('renameRdpTab', target.id, title)
}

function cancelRename(): void {
  renamingTab.value = null
  renameDraft.value = ''
}

const metrics = ref(new Map<string, ResourceSnapshot>())
let metricsTimer: number | undefined
let metricsVersion = 0

const gpuMemory = ref(new Map<string, GpuMemorySummary>())

const gpuProbes = new Map<string, number>()
const gpuProbeIntervalMs = 15_000
const monitorGateway = createMonitorGateway()
function metricSnapshot(session: SessionTab): ResourceSnapshot | null {
  return metrics.value.get(session.id) ?? null
}
function gpuSnapshot(session: SessionTab): GpuMemorySummary | null {
  return gpuMemory.value.get(session.id) ?? null
}
function metricText(value: number): string {
  return `${Math.round(value)}%`
}

async function probeGpuMemory(session: SessionTab): Promise<GpuMemorySummary | null | undefined> {
  try {
    const snapshot = await monitorGateway.accelerators(session.id)
    return gpuMemorySummary(snapshot.devices)
  } catch {
    return undefined
  }
}
async function refreshMetrics(): Promise<void> {
  const version = ++metricsVersion
  const sessions = props.terminalSessions.filter((session) => session.kind === 'ssh' && session.state === 'connected')

  const probedAt = Date.now()
  const due = sessions.filter((session) => probedAt - (gpuProbes.get(session.id) ?? 0) >= gpuProbeIntervalMs)

  const [results, gpuResults] = await Promise.all([
    Promise.allSettled(
      sessions.map(async (session) => [session.id, await props.sessionGateway?.readResources?.(session.id)] as const),
    ),
    Promise.allSettled(due.map(async (session) => [session.id, await probeGpuMemory(session)] as const)),
  ])
  if (version !== metricsVersion) return
  const next = new Map(metrics.value)
  for (const result of results)
    if (result.status === 'fulfilled' && result.value[1]) next.set(result.value[0], result.value[1])
  metrics.value = next

  const liveIds = new Set(sessions.map((session) => session.id))
  for (const id of gpuProbes.keys()) if (!liveIds.has(id)) gpuProbes.delete(id)
  const nextGpu = new Map<string, GpuMemorySummary>()
  for (const session of sessions) {
    const cached = gpuMemory.value.get(session.id)
    if (cached) nextGpu.set(session.id, cached)
  }
  for (const result of gpuResults) {
    if (result.status !== 'fulfilled') continue
    const [id, summary] = result.value

    if (summary === undefined) continue
    gpuProbes.set(id, probedAt)

    if (summary === null) nextGpu.delete(id)
    else nextGpu.set(id, summary)
  }
  gpuMemory.value = nextGpu
  if (props.terminalShowMetrics === true && metricsVersion === version) {
    metricsTimer = window.setTimeout(() => void refreshMetrics(), 5000)
  }
}
function restartMetrics(): void {
  metricsVersion += 1
  if (metricsTimer) window.clearTimeout(metricsTimer)
  metricsTimer = undefined

  if (props.terminalShowMetrics !== true) {
    gpuMemory.value = new Map()
    gpuProbes.clear()
  }
  if (props.terminalShowMetrics !== true || !props.sessionGateway?.readResources) return
  void refreshMetrics()
}
watch(
  () => [
    props.terminalShowMetrics,
    ...props.terminalSessions.map((session) => `${session.id}:${session.kind}:${session.state}`),
  ],
  restartMetrics,
  { immediate: true },
)
onBeforeUnmount(() => {
  metricsVersion += 1
  if (metricsTimer) window.clearTimeout(metricsTimer)
})

const {
  panes,
  splitHandles,
  isSplit,
  splitGroups,
  standaloneSessions,
  activeGroupId,
  dragging,
  preview,
  target,
  docking,
  selfSplit,
  groupOrders,
  setGroupOrder,
  dragTabIntoStage,
  dropTabIntoStage,
  commitSelfSplit,
  cancelTabDrop,
  splitWith,
  single,
  remove,
  start,
  click,
  showGroup,
  closeGroup,
  groupSessionIds,
  setSplitRatio,
  activeGroupSessionIds,
} = useSplitWorkspace(
  () => props.sessions,
  () => props.activeSessionId,
  stage,
  (id) => emit('selectSession', id),
  closeSessions,

  () => tabBar.value?.getBoundingClientRect() ?? null,
)

const {
  draggingTab,
  dragGhost,
  dropPlaceholderIndex,
  tabPlaceholderStyle,
  draggedTabStyle,
  handleTabPointerDown,
  selectTab,
  selectAuxiliaryTab,
} = useSessionTabDrag({
  stage,
  tabBar,
  t,

  tabOrderOf,
  tabTitle,
  selfSplit,
  dragTabIntoStage,
  dropTabIntoStage,
  commitSelfSplit,
  cancelTabDrop,
  click,
  openSessionWindow: (sessionId) => emit('openSessionWindow', sessionId),

  reorderTabs: applyTabBarReorder,
  requestSplit: (destination, direction) => emit('requestSplit', destination, direction),
})

const controlSessions = computed<TerminalControlSession[]>(() =>
  props.terminalSessions
    .filter((session) => session.kind === 'ssh')
    .map((session, index) => {
      const split = splitGroups.value.find((group) => groupSessionIds(group.id).includes(session.id))
      return {
        id: session.id,
        title:
          props.terminalSessions.filter((item) => item.title === session.title).length > 1
            ? `${session.title} · ${index + 1}`
            : session.title,
        address: session.host?.address ?? session.address ?? '',
        username: session.host?.username ?? '',
        tabId: split?.id ?? session.id,
        tabTitle: split
          ? (props.sessions.find((item) => item.id === split.firstSessionId)?.title ?? session.title)
          : session.title,
        ready:
          session.state === 'connected' &&
          session.streamState === 'ready' &&
          terminalControl.eligibleIds.value.includes(session.id),
      }
    }),
)

function openBatchSend(sessionId: string): void {
  emit('selectSession', sessionId)
  terminalControl.openBatch(sessionId)
}

async function handleSyncAction(
  action: 'create' | 'join' | 'manage' | 'pause' | 'resume' | 'leave' | 'stop',
  sessionId = '',
): Promise<void> {
  await terminalControl.action(action, sessionId)
}

function requestSplit(sessionID: string, direction: SplitDirection): void {
  emit('requestSplit', sessionID, direction)
}

watch(isSplit, (value) => emit('splitChange', value), { immediate: true })

function handleToggleMaximizeEvent(): void {
  const active = props.activeSessionId
  if (!active || !panes.value.some((pane) => pane.id === active)) return
  toggleMaximizePane(active)
}

function handleMaximizeEscape(event: KeyboardEvent): void {
  if (!maximizedPaneId.value || event.key !== 'Escape') return
  const target = event.target instanceof HTMLElement ? event.target : null
  if (target?.closest('input, textarea, [contenteditable="true"]')) return
  maximizedPaneId.value = null
}

onMounted(() => {
  window.addEventListener('qterm:toggle-pane-maximize', handleToggleMaximizeEvent)
  window.addEventListener('keydown', handleMaximizeEscape)
})
onBeforeUnmount(() => {
  window.removeEventListener('qterm:toggle-pane-maximize', handleToggleMaximizeEvent)
  window.removeEventListener('keydown', handleMaximizeEscape)
})

function activateAdjacentTab(offset: 1 | -1): boolean {
  const tabs = allTabs.value
  if (tabs.length < 2) return false
  const currentIndex = tabs.findIndex((item) => {
    if (item.type === 'sftp') return props.activeSftpTabId === item.tab.id
    if (item.type === 'rdp') return props.activeRdpTabId === item.tab.id
    if (auxiliaryActive()) return false
    if (item.type === 'group') return activeGroupId.value === item.group.id
    return props.activeSessionId === item.session.id
  })
  if (currentIndex < 0) return false
  const target = tabs[(currentIndex + offset + tabs.length) % tabs.length]
  if (!target) return false
  if (target.type === 'session') click(target.session.id)
  else if (target.type === 'group') showGroup(target.group.id)
  else if (target.type === 'sftp') emit('selectSftp', target.tab.id)
  else emit('selectRdp', target.tab.id)
  return true
}

function focusAdjacentPane(direction: PaneFocusDirection | 1 | -1): boolean {
  const list = panes.value
  if (list.length < 2) return false
  const currentIndex = list.findIndex((pane) => pane.id === props.activeSessionId)
  if (currentIndex < 0) return false
  if (direction === 1 || direction === -1) {
    const next = list[(currentIndex + direction + list.length) % list.length]
    if (!next) return false
    click(next.id)
    return true
  }
  const current = list[currentIndex]
  if (!current) return false
  const target = nearestPaneInDirection(list, current, direction)
  if (!target) return false
  click(target.id)
  return true
}

function removePane(sessionId: string | null): boolean {
  if (!sessionId || !isSplit.value) return false
  if (!panes.value.some((pane) => pane.id === sessionId)) return false
  remove(sessionId)
  return true
}

function applySplit(destination: string, source: string, direction: SplitDirection): boolean {
  return splitWith(destination, source, direction)
}

defineExpose({ activateAdjacentTab, focusAdjacentPane, removePane, applySplit })

const terminalLayoutRevision = ref(0)
const terminalLayoutSignature = computed(() => {
  const members = panes.value.map((pane) => pane.id).join('\u0000')

  const topology = splitHandles.value.map((handle) => `${handle.key}:${handle.axis}`).join('\u0000')
  return `${isSplit.value ? 'split' : 'single'}:${members}:${topology}`
})
watch(
  terminalLayoutSignature,
  (next, previous) => {
    if (next.startsWith('split:') || previous.startsWith('split:')) terminalLayoutRevision.value += 1
  },
  { flush: 'sync' },
)

const maximizedPaneId = ref<string | null>(null)

function toggleMaximizePane(sessionId: string): void {
  maximizedPaneId.value = maximizedPaneId.value === sessionId ? null : sessionId
}

watch(isSplit, (value) => {
  if (!value) maximizedPaneId.value = null
})
watch(
  () => panes.value.map((pane) => pane.id),
  (ids) => {
    if (maximizedPaneId.value && !ids.includes(maximizedPaneId.value)) maximizedPaneId.value = null
  },
)

function visible(id: string): boolean {
  return panes.value.some((pane) => pane.id === id)
}
function position(id: string): CSSProperties {
  if (maximizedPaneId.value === id) {
    return { left: '2px', top: '2px', right: '2px', bottom: '2px', width: 'auto', height: 'auto', zIndex: 30 }
  }
  return paneStyle(
    panes.value.find((pane) => pane.id === id),
    stageSize.value,
  )
}

let activeSplitResize: { key: string; ratio: number; size: number } | null = null

function splitHandleStyle(handle: SplitResizeHandle): CSSProperties {
  if (handle.axis === 'x') {
    const left = Math.round(((handle.x + handle.width * handle.ratio) * stageSize.value.width) / 100)
    const top = evenPixel((handle.y * stageSize.value.height) / 100)
    const height = evenPixel((handle.height * stageSize.value.height) / 100)
    return { left: `${left - 5}px`, top: `${top}px`, height: `${height}px` }
  }
  const left = Math.round((handle.x * stageSize.value.width) / 100)
  const top = evenPixel(((handle.y + handle.height * handle.ratio) * stageSize.value.height) / 100)
  const width = Math.round((handle.width * stageSize.value.width) / 100)
  return { left: `${left}px`, top: `${top - 5}px`, width: `${width}px` }
}

function beginSplitResize(handle: SplitResizeHandle): void {
  const size =
    handle.axis === 'x' ? (stageSize.value.width * handle.width) / 100 : (stageSize.value.height * handle.height) / 100
  activeSplitResize = { key: handle.key, ratio: handle.ratio, size: Math.max(1, size) }
}

function resizeSplit(handle: SplitResizeHandle, delta: number): void {
  if (!activeSplitResize || activeSplitResize.key !== handle.key) beginSplitResize(handle)
  if (!activeSplitResize) return
  setSplitRatio(handle.path, activeSplitResize.ratio + delta / activeSplitResize.size)
}

function finishSplitResize(): void {
  activeSplitResize = null
}
</script>

<template>
  <section class="contents" :aria-label="t('shell.workspace.aria')">
    <div
      v-if="dragGhost"
      :style="{
        position: 'fixed',
        left: '0',
        top: '0',
        transform: `translate(${dragGhost.x + 14}px, ${dragGhost.y + 12}px)`,
        zIndex: 9998,
        pointerEvents: 'none',
        display: 'flex',
        alignItems: 'center',
        gap: '6px',
        padding: '4px 8px',
        borderRadius: '6px',
        border: '1px solid var(--qt-accent)',
        background: 'var(--qt-panel, rgba(0,0,0,0.75))',
        color: 'inherit',
        fontSize: '12px',
        boxShadow: '0 6px 18px rgba(0,0,0,0.35)',
        whiteSpace: 'nowrap',
      }"
      aria-hidden="true"
    >
      <span v-if="dragGhost.hint" :style="{ color: 'var(--qt-accent)' }">{{ dragGhost.hint }}</span>
    </div>

    <nav
      v-show="!props.sessionWindow && hasTabs"
      ref="tabBar"
      class="session-tabs"
      :aria-label="t('shell.workspace.tabs')"
      role="tablist"
      :class="{ 'is-dock-target': docking }"
      @pointerdown.capture="handleTabPointerDown"
      @wheel="scrollTabs"
    >
      <template
        v-for="(item, index) in allTabs"
        :key="item.type === 'session' ? item.session.id : item.type === 'group' ? item.group.id : item.tab.id"
      >
        <div
          v-if="dropPlaceholderIndex === index"
          :key="`placeholder-${index}`"
          class="session-tab-placeholder"
          :style="tabPlaceholderStyle"
          aria-hidden="true"
        />

        <SessionContextMenu
          v-if="item.type === 'session'"
          :is-split="isSplit"
          :visible="visible(item.session.id)"
          :locked="item.session.locked === true"
          :session-actions="true"
          :open="openMenuKey === unitKeyOf(item)"
          :can-close-left="canCloseScopeFor(unitKeyOf(item), 'left')"
          :can-close-right="canCloseScopeFor(unitKeyOf(item), 'right')"
          :can-close-others="canCloseScopeFor(unitKeyOf(item), 'others')"
          @update:open="(open) => setMenuOpen(unitKeyOf(item), open)"
          @rename="beginRename('session', item.session.id, item.session.title)"
          @toggle-lock="emit('toggleTabLock', item.session.id)"
          @duplicate="emit('duplicateSession', item.session.id)"
          @close-scope="(scope) => requestCloseScope(unitKeyOf(item), scope)"
          @single="single(item.session.id)"
          @close="requestCloseSession(item.session.id)"
          @open-window="emit('openSessionWindow', item.session.id)"
        >
          <div
            class="session-tab"
            :data-session-id="item.session.id"
            :style="draggingTab === item.session.id ? draggedTabStyle : undefined"
            :class="{
              'is-active': !auxiliaryActive() && !tabsCovered() && props.activeSessionId === item.session.id,
            }"
            @click="selectTab(item.session.id)"
            @dblclick="handleTabDoubleClick(item, $event)"
          >
            <span
              class="session-tab-trigger inline-flex items-center self-stretch gap-[7px] min-w-0 flex-1"
              :class="{ 'is-renaming': isRenaming('session', item.session.id) }"
              role="tab"
              :aria-selected="!auxiliaryActive() && !tabsCovered() && props.activeSessionId === item.session.id"
              :aria-label="item.session.title"
              tabindex="0"
              @click.stop="selectTab(item.session.id)"
              @dblclick.stop="handleTabDoubleClick(item, $event)"
              @keydown.enter="click(item.session.id)"
              @keydown.space.prevent="click(item.session.id)"
            >
              <Laptop
                v-if="item.session.kind === 'local'"
                class="flex-none text-qt-muted"
                :size="16"
                aria-hidden="true"
              />
              <HostIcon
                v-else
                :icon="props.terminalSessions.find((entry) => entry.id === item.session.id)?.host?.icon"
                :protocol="props.terminalSessions.find((entry) => entry.id === item.session.id)?.host?.protocol"
                :size="16"
              />
              <Radio
                v-if="syncGroup?.sessionIds.includes(item.session.id)"
                class="size-3.5 shrink-0 text-qt-accent"
                :title="t('terminalControl.sync.badge', { count: syncGroup.sessionIds.length })"
              />

              <Lock
                v-if="item.session.locked"
                :size="11"
                class="flex-none text-qt-muted"
                :aria-label="t('shell.sessionMenu.locked')"
              />

              <span
                v-if="!isRenaming('session', item.session.id)"
                class="session-tab-title min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap"
                :class="{ 'tabular-nums': tabOrdinal(item.session.id) !== null }"
                >{{ tabLabel(item.session.id, item.session.title, item.session.autoReconnectPending === true) }}</span
              >
            </span>
            <input
              v-if="isRenaming('session', item.session.id)"
              v-model="renameDraft"
              class="session-tab-rename min-w-0 flex-1 rounded-[4px] border-d bg-qt-panel px-1 h-5 text-sm text-qt-text outline-none focus-visible:border-qt-accent"
              type="text"
              maxlength="64"
              autocomplete="off"
              autocorrect="off"
              autocapitalize="off"
              spellcheck="false"
              :aria-label="t('shell.workspace.renameTab', { title: item.session.title })"
              @click.stop
              @dblclick.stop
              @keydown.stop
              @keydown.enter.prevent="commitRename()"
              @keydown.esc.prevent="cancelRename()"
              @blur="commitRename()"
            />
            <QtermButton
              variant="ghost"
              class="session-close"
              size="icon-xs"
              type="button"
              :aria-label="t('shell.workspace.closeTab', { title: item.session.title })"
              @click.stop="requestCloseSession(item.session.id)"
            >
              <X :size="13" aria-hidden="true" />
            </QtermButton>
          </div>
        </SessionContextMenu>
        <SessionContextMenu
          v-else-if="item.type === 'group'"
          :is-split="true"
          :visible="false"
          :open="openMenuKey === unitKeyOf(item)"
          can-close-tab
          can-close-scope
          :can-close-left="canCloseScopeFor(unitKeyOf(item), 'left')"
          :can-close-right="canCloseScopeFor(unitKeyOf(item), 'right')"
          :can-close-others="canCloseScopeFor(unitKeyOf(item), 'others')"
          @update:open="(open) => setMenuOpen(unitKeyOf(item), open)"
          @close-scope="(scope) => requestCloseScope(unitKeyOf(item), scope)"
          @single="single(item.group.firstSessionId)"
          @open-window="emit('openSessionWindow', item.group.firstSessionId)"
          @close="closeGroup(item.group.id)"
        >
          <div
            class="session-tab is-split-group"
            :data-split-group-session-id="item.group.firstSessionId"
            :style="draggingTab === item.group.firstSessionId ? draggedTabStyle : undefined"
            :class="{ 'is-split-active': !auxiliaryActive() && activeGroupId === item.group.id }"
            @click="showGroup(item.group.id)"
            @dblclick="handleTabDoubleClick(item, $event)"
          >
            <span
              class="session-tab-trigger inline-flex items-center self-stretch gap-[7px] min-w-0 flex-1"
              role="tab"
              :aria-selected="!auxiliaryActive() && activeGroupId === item.group.id"
              :aria-label="t('shell.workspace.splitCount', { count: item.group.count })"
              tabindex="0"
              @click.stop="showGroup(item.group.id)"
              @dblclick.stop="handleTabDoubleClick(item, $event)"
              @keydown.enter="showGroup(item.group.id)"
              @keydown.space.prevent="showGroup(item.group.id)"
            >
              <span class="inline-flex items-center text-qt-muted">
                <LayoutTemplate :size="14" aria-hidden="true" />
              </span>

              <Radio
                v-if="groupSessionIds(item.group.id).some((id) => syncGroup?.sessionIds.includes(id))"
                class="size-3.5 shrink-0 text-qt-accent"
                :title="t('terminalControl.sync.badge', { count: syncGroup?.sessionIds.length ?? 0 })"
              />
              <span
                class="session-tab-title min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap"
                :class="{ 'tabular-nums': tabOrdinal(item.group.firstSessionId) !== null }"
                >{{
                  tabLabel(
                    item.group.firstSessionId,
                    t('shell.workspace.splitCount', { count: item.group.count }),
                    props.sessions.find((session) => session.id === item.group.firstSessionId)?.autoReconnectPending ===
                      true,
                  )
                }}</span
              >
            </span>
            <QtermButton
              variant="ghost"
              size="icon-xs"
              class="session-close"
              type="button"
              :aria-label="t('shell.workspace.closeSplit', { count: item.group.count })"
              @click.stop="closeGroup(item.group.id)"
            >
              <X :size="13" aria-hidden="true" />
            </QtermButton>
          </div>
        </SessionContextMenu>

        <SessionContextMenu
          v-else-if="item.type === 'sftp'"
          :is-split="false"
          :visible="false"
          can-rename
          can-close-tab
          can-close-scope
          :can-open-window="false"
          :can-close-session="false"
          :open="openMenuKey === unitKeyOf(item)"
          :can-close-left="canCloseScopeFor(unitKeyOf(item), 'left')"
          :can-close-right="canCloseScopeFor(unitKeyOf(item), 'right')"
          :can-close-others="canCloseScopeFor(unitKeyOf(item), 'others')"
          @update:open="(open) => setMenuOpen(unitKeyOf(item), open)"
          @close-scope="(scope) => requestCloseScope(unitKeyOf(item), scope)"
          @rename="beginRename('sftp', item.tab.id, item.tab.title)"
          @close="emit('closeSftp', item.tab.id)"
        >
          <div
            class="session-tab is-sftp-tab"
            :data-sftp-tab-id="item.tab.id"
            :style="draggingTab === item.tab.id ? draggedTabStyle : undefined"
            :class="{ 'is-active': !tabsCovered() && props.activeSftpTabId === item.tab.id }"
            @click="selectAuxiliaryTab(() => emit('selectSftp', item.tab.id))"
            @dblclick="handleTabDoubleClick(item, $event)"
          >
            <span
              class="session-tab-trigger inline-flex items-center self-stretch gap-[7px] min-w-0 flex-1"
              :class="{ 'is-renaming': isRenaming('sftp', item.tab.id) }"
              role="tab"
              :aria-selected="!tabsCovered() && props.activeSftpTabId === item.tab.id"
              :aria-label="item.tab.title"
              tabindex="0"
              @click.stop="selectAuxiliaryTab(() => emit('selectSftp', item.tab.id))"
              @dblclick.stop="handleTabDoubleClick(item, $event)"
              @keydown.enter="emit('selectSftp', item.tab.id)"
              @keydown.space.prevent="emit('selectSftp', item.tab.id)"
            >
              <FolderOpen :size="16" aria-hidden="true" />
              <span
                v-if="!isRenaming('sftp', item.tab.id)"
                class="session-tab-title min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap"
                >{{ item.tab.title }}</span
              >
            </span>
            <input
              v-if="isRenaming('sftp', item.tab.id)"
              v-model="renameDraft"
              class="session-tab-rename min-w-0 flex-1 rounded-[4px] border-d bg-qt-panel px-1 h-5 text-sm text-qt-text outline-none focus-visible:border-qt-accent"
              type="text"
              maxlength="64"
              autocomplete="off"
              autocorrect="off"
              autocapitalize="off"
              spellcheck="false"
              :aria-label="t('shell.workspace.renameTab', { title: item.tab.title })"
              @click.stop
              @dblclick.stop
              @keydown.stop
              @keydown.enter.prevent="commitRename()"
              @keydown.esc.prevent="cancelRename()"
              @blur="commitRename()"
            />
            <QtermButton
              variant="ghost"
              size="icon-xs"
              class="session-close"
              type="button"
              :aria-label="t('shell.workspace.closeTab', { title: item.tab.title })"
              @click.stop="emit('closeSftp', item.tab.id)"
            >
              <X :size="13" aria-hidden="true" />
            </QtermButton>
          </div>
        </SessionContextMenu>

        <SessionContextMenu
          v-else-if="item.type === 'rdp'"
          :is-split="false"
          :visible="false"
          can-rename
          can-duplicate
          can-lock
          can-close-tab
          can-close-scope
          :open="openMenuKey === unitKeyOf(item)"
          :locked="item.tab.locked === true"
          :can-close-left="canCloseScopeFor(unitKeyOf(item), 'left')"
          :can-close-right="canCloseScopeFor(unitKeyOf(item), 'right')"
          :can-close-others="canCloseScopeFor(unitKeyOf(item), 'others')"
          @update:open="(open) => setMenuOpen(unitKeyOf(item), open)"
          @close-scope="(scope) => requestCloseScope(unitKeyOf(item), scope)"
          @rename="beginRename('rdp', item.tab.id, item.tab.title)"
          @duplicate="emit('duplicateRdpTab', item.tab.id)"
          @toggle-lock="emit('toggleRdpTabLock', item.tab.id)"
          @open-window="emit('openDesktopWindow', item.tab.id)"
          @close="requestCloseRdp(item.tab.id)"
        >
          <div
            class="session-tab is-rdp-tab"
            :data-rdp-tab-id="item.tab.id"
            :style="draggingTab === item.tab.id ? draggedTabStyle : undefined"
            :class="{ 'is-active': !tabsCovered() && props.activeRdpTabId === item.tab.id }"
            @click="selectAuxiliaryTab(() => emit('selectRdp', item.tab.id))"
            @dblclick="handleTabDoubleClick(item, $event)"
          >
            <span
              class="session-tab-trigger inline-flex items-center self-stretch gap-[7px] min-w-0 flex-1"
              :class="{ 'is-renaming': isRenaming('rdp', item.tab.id) }"
              role="tab"
              :aria-selected="!tabsCovered() && props.activeRdpTabId === item.tab.id"
              :aria-label="item.tab.title"
              tabindex="0"
              @click.stop="selectAuxiliaryTab(() => emit('selectRdp', item.tab.id))"
              @dblclick.stop="handleTabDoubleClick(item, $event)"
              @keydown.enter="emit('selectRdp', item.tab.id)"
              @keydown.space.prevent="emit('selectRdp', item.tab.id)"
            >
              <Monitor :size="16" aria-hidden="true" />
              <span
                v-if="!isRenaming('rdp', item.tab.id)"
                class="session-tab-title min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap"
                >{{ item.tab.title }}</span
              >
            </span>
            <input
              v-if="isRenaming('rdp', item.tab.id)"
              v-model="renameDraft"
              class="session-tab-rename min-w-0 flex-1 rounded-[4px] border-d bg-qt-panel px-1 h-5 text-sm text-qt-text outline-none focus-visible:border-qt-accent"
              type="text"
              maxlength="64"
              autocomplete="off"
              autocorrect="off"
              autocapitalize="off"
              spellcheck="false"
              :aria-label="t('shell.workspace.renameTab', { title: item.tab.title })"
              @click.stop
              @dblclick.stop
              @keydown.stop
              @keydown.enter.prevent="commitRename()"
              @keydown.esc.prevent="cancelRename()"
              @blur="commitRename()"
            />
            <QtermButton
              variant="ghost"
              size="icon-xs"
              class="session-close"
              type="button"
              :aria-label="t('shell.workspace.closeTab', { title: item.tab.title })"
              @click.stop="requestCloseRdp(item.tab.id)"
            >
              <X :size="13" aria-hidden="true" />
            </QtermButton>
          </div>
        </SessionContextMenu>
      </template>

      <div
        v-if="dropPlaceholderIndex === allTabs.length"
        class="session-tab-placeholder"
        :style="tabPlaceholderStyle"
        aria-hidden="true"
      />
      <span v-if="docking" class="tab-dock-hint text-xs">{{ t('shell.workspace.dropToTabs') }}</span>
    </nav>

    <div
      v-show="!props.assetLibraryCovered"
      class="terminal-workspace-body relative col-[1] row-[2] flex flex-col min-w-0 min-h-0 overflow-hidden"
      :inert="props.assetLibraryCovered || undefined"
      :aria-hidden="props.assetLibraryCovered || undefined"
    >
      <section
        ref="stage"
        class="terminal-stage relative col-[1] row-[2] min-w-0 min-h-0 text-qt-terminal-text bg-qt-terminal overflow-hidden"
        :class="{ 'is-split': isSplit, 'is-dragging': dragging, 'is-sftp-covered': stageCovered() }"
        :data-layout-revision="terminalLayoutRevision"
        :aria-label="t('shell.workspace.aria')"
        :aria-hidden="stageCovered() || undefined"
        :inert="stageCovered() || undefined"
      >
        <div
          v-for="session in props.terminalSessions"
          :key="session.paneKey"
          class="workspace-pane absolute inset-[2px] z-0 min-w-0 min-h-0 overflow-hidden invisible pointer-events-none"
          :class="{
            'is-pane-visible': visible(session.id),
            'is-pane-maximized': maximizedPaneId === session.id,
            'is-pane-covered': maximizedPaneId !== null && maximizedPaneId !== session.id,
            'is-focused': props.activeSessionId === session.id,
            'has-terminal-metrics':
              props.terminalShowMetrics && session.kind === 'ssh' && session.state === 'connected',
          }"
          :style="position(session.id)"
          :data-pane-id="session.id"
          :aria-hidden="!visible(session.id)"
          :inert="!visible(session.id)"
          @pointerdown="emit('selectSession', session.id)"
          @focusin="emit('selectSession', session.id)"
        >
          <SessionContextMenu
            :is-split="isSplit"
            :visible="true"
            @single="single(session.id)"
            @remove="remove(session.id)"
            @close="requestCloseSession(session.id)"
            @open-window="emit('openSessionWindow', session.id)"
          >
            <header
              v-show="isSplit"
              class="pane-titlebar text-sm"
              tabindex="0"
              :aria-label="t('shell.workspace.paneTitlebar', { title: session.title })"
              @pointerdown.stop="start($event, session.id)"
              @dragstart.prevent
              @click="selectTab(session.id)"
              @keydown.enter="click(session.id)"
              @keydown.space.prevent="click(session.id)"
            >
              <span
                class="status-dot"
                :class="`status-dot--${session.state === 'connected' ? 'connected' : session.state === 'failed' ? 'error' : 'idle'}`"
              ></span>
              <span class="pane-title min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap">{{
                session.title
              }}</span>
              <Radio
                v-if="syncGroup?.sessionIds.includes(session.id)"
                class="size-3.5 shrink-0 text-qt-accent"
                :title="t('terminalControl.sync.badge', { count: syncGroup.sessionIds.length })"
              />
              <QtermButton
                variant="ghost"
                size="icon-xs"
                :aria-label="
                  maximizedPaneId === session.id ? t('shell.workspace.restorePane') : t('shell.workspace.maximizePane')
                "
                :title="
                  maximizedPaneId === session.id ? t('shell.workspace.restorePane') : t('shell.workspace.maximizePane')
                "
                data-pane-maximize
                @click.stop="toggleMaximizePane(session.id)"
              >
                <Minimize2 v-if="maximizedPaneId === session.id" :size="12" />
                <Maximize v-else :size="12" />
              </QtermButton>
              <QtermButton
                variant="ghost"
                size="icon-xs"
                :aria-label="t('shell.workspace.single')"
                :title="t('shell.workspace.single')"
                @click.stop="single(session.id)"
              >
                <Maximize2 :size="12" />
              </QtermButton>
              <QtermButton
                variant="ghost"
                size="icon-xs"
                :aria-label="t('shell.workspace.closeSplitSession', { title: session.title })"
                :title="t('shell.workspace.closeSession')"
                @click.stop="requestCloseSession(session.id)"
              >
                <X :size="13" />
              </QtermButton>
            </header>
          </SessionContextMenu>
          <div
            v-if="props.terminalShowLatency && session.state === 'connected' && session.kind === 'ssh'"
            class="session-status session-latency text-sm absolute top-[14px] right-[16px] z-4 flex items-center gap-[6px] text-qt-muted pointer-events-none pt-[3px] pr-[6px] pb-[3px] pl-[6px] [border:1px_solid_color-mix(in_srgb,_currentColor_24%,_transparent)] rounded-[999px] [background:color-mix(in_srgb,_var(--qt-terminal)_72%,_transparent)] shadow-[0_1px_4px_rgb(24_24_27_/_10%)] [font-variant-numeric:tabular-nums] font-medium"
            :class="`session-latency--${latencyDisplay(session.latencyMs).tone}`"
            :title="latencyDisplay(session.latencyMs).title"
            :aria-label="t('shell.workspace.latency')"
          >
            <Radio
              v-if="syncGroup?.sessionIds.includes(session.id) && !isSplit"
              class="size-3 shrink-0"
              :class="syncGroup.status === 'active' ? 'text-qt-success' : 'text-qt-warning'"
              data-terminal-sync-icon
            />
            {{ latencyDisplay(session.latencyMs).text }}
          </div>
          <div
            v-else-if="session.id === props.activeSessionId && !isSplit"
            class="session-status session-connection-indicator size-[7px] text-sm absolute top-[14px] right-[16px] z-4 flex items-center gap-[6px] text-qt-muted pointer-events-none"
            :title="props.activeStatus.label"
            :aria-label="t('shell.workspace.connectionStatus')"
            aria-live="polite"
          >
            <span class="status-dot" :class="`status-dot--${props.activeStatus.tone}`"></span>
          </div>
          <TerminalPane
            v-if="props.settingsReady !== false"
            :transparent-background="props.transparentBackground"
            :sync-group="syncGroup"
            :session-id="session.id"
            :host-id="session.hostId"
            :kind="session.kind"
            :backspace-mode="session.host?.backspaceMode || 'del'"
            :host-name="session.host?.name ?? session.title"
            :username="session.host?.username ?? ''"
            :address="session.host?.address ?? ''"
            :port="session.host?.port ?? 22"
            :active="!auxiliaryActive() && session.id === props.activeSessionId"
            :visible="visible(session.id)"
            :layout-revision="terminalLayoutRevision"
            :dark="props.dark"
            :persistent="props.persistent"
            :font-size="props.terminalFontSize"
            :font-weight="props.terminalFontWeight ?? 400"
            @font-size-step="emit('fontSizeStep', $event)"
            :line-height="props.terminalLineHeight ?? 1.25"
            :font-family="props.terminalFontFamily"
            :gutter-mode="props.gutterMode ?? 'off'"
            :translate-enabled="props.translateEnabled !== false"
            :translate-target="props.translateTarget ?? 'zh-CN'"
            :translate-provider="props.translateProvider ?? 'google'"
            :translate-endpoint="props.translateEndpoint ?? ''"
            :gutter-show-timestamps="props.gutterShowTimestamps !== false"
            :gutter-timestamp-format="props.gutterTimestampFormat ?? 'HH:mm:ss'"
            :light-theme="props.terminalLightTheme"
            :dark-theme="props.terminalDarkTheme"
            :auto-copy-selection="props.terminalAutoCopySelection"
            :copy-feedback="props.terminalCopyFeedback"
            :scrollback="props.terminalScrollback ?? 5000"
            :confirm-multi-line-paste="props.terminalConfirmMultiLinePaste !== false"
            :completion-enabled="props.terminalCompletionEnabled"
            :history-min-length="props.terminalHistoryMinLength"
            :history-max-length="props.terminalHistoryMaxLength"
            :word-separators="props.terminalWordSeparators"
            :mac-option-is-meta="props.terminalMacOptionIsMeta === true"
            :cursor-blink="props.terminalCursorBlink"
            :cursor-style="props.terminalCursorStyle"
            :right-click-behavior="props.terminalRightClickBehavior"
            :modifier-right-click-behavior="props.terminalModifierRightClickBehavior"
            :disconnect-behavior="props.terminalDisconnectBehavior"
            :auto-reconnect="props.terminalAutoReconnect === true"
            :auto-reconnect-pending="session.autoReconnectPending === true"
            :close-confirm-pending="props.closeConfirmIds?.includes(session.id) === true"
            :render-engine="props.terminalRenderEngine"
            :keyword-highlight-enabled="props.keywordHighlightEnabled"
            :keyword-highlight-rules="props.keywordHighlightRules"
            :credential-prompt-rules="props.credentialPromptRules"
            :credentials="props.credentials"
            :type-credential="props.typeCredential"
            :state="session.state"
            :stream-token="session.streamToken"
            :reconnecting="session.reconnectInFlight === true"
            @stream-status="emit('streamStatus', $event)"
            @close-session="requestCloseSession($event)"
            @reconnect-session="emit('reconnectSession', $event)"
            @disconnect-session="emit('disconnectSession', $event)"
            @terminal-sync="handleSyncAction"
            @open-batch-send="openBatchSend"
            @open-sftp="emit('openSftp', session.id)"
            @ai-analyze="
              (sessionId: string, kind: 'selection' | 'recent', text: string) =>
                emit('aiAnalyze', sessionId, kind, text)
            "
            @open-settings="emit('openSettings')"
            @save-as-snippet="emit('saveAsSnippet', $event)"
            @request-split="requestSplit"
          />
          <Separator
            v-if="props.terminalShowMetrics && session.kind === 'ssh' && session.state === 'connected'"
            class="terminal-metrics-separator"
          />
          <div
            v-if="props.terminalShowMetrics && session.kind === 'ssh' && session.state === 'connected'"
            class="terminal-metrics text-xs"
            :aria-label="t('shell.workspace.metrics')"
          >
            <template v-if="metricSnapshot(session)?.metricsSupported">
              <span class="terminal-metric-item metric-cpu inline-flex items-center gap-[8px] flex-none">
                <CPUIcon :size="13" aria-hidden="true" />
                CPU <b>{{ metricText(metricSnapshot(session)?.cpuUsagePercent ?? 0) }}</b>
              </span>
              <span class="terminal-metric-item metric-memory inline-flex items-center gap-[8px] flex-none">
                <Separator orientation="vertical" />
                <MemoryIcon :size="13" aria-hidden="true" />
                Mem <b>{{ metricText(metricSnapshot(session)?.memoryUsagePercent ?? 0) }}</b>
                <span class="terminal-metrics-detail text-qt-muted">
                  {{ metricSnapshot(session)?.memoryUsedText }} / {{ metricSnapshot(session)?.memoryTotalText }}
                </span>
              </span>
              <span class="terminal-metric-item metric-load inline-flex items-center gap-[8px] flex-none">
                <Separator orientation="vertical" />
                <Activity :size="13" stroke-width="1.8" aria-hidden="true" />
                Load <b>{{ metricSnapshot(session)?.loadAverage1?.toFixed(2) ?? '—' }}</b>
              </span>
              <span class="terminal-metric-item metric-disk inline-flex items-center gap-[8px] flex-none">
                <Separator orientation="vertical" />
                <HardDrive :size="13" stroke-width="1.8" aria-hidden="true" />
                Disk
                <b>
                  {{
                    metricSnapshot(session)
                      ?.disks.find((item) => item.mount === '/')
                      ?.usedPercent?.toFixed(0) ?? '—'
                  }}%
                </b>
                <span class="terminal-metrics-detail text-qt-muted">
                  {{ metricSnapshot(session)?.disks.find((item) => item.mount === '/')?.usedText }} /
                  {{ metricSnapshot(session)?.disks.find((item) => item.mount === '/')?.totalText }}
                </span>
              </span>
              <span
                v-if="gpuSnapshot(session)"
                class="terminal-metric-item metric-gpu inline-flex items-center gap-[8px] flex-none"
              >
                <Separator orientation="vertical" />
                <GPUIcon :size="13" aria-hidden="true" />
                GPU <b>{{ Math.round(gpuSnapshot(session)?.usedMb ?? 0) }}</b>
                <span class="terminal-metrics-detail text-qt-muted">
                  <template v-if="(gpuSnapshot(session)?.totalMb ?? 0) > 0">
                    / {{ formatGpuMemory(gpuSnapshot(session)?.totalMb ?? 0) }} ({{
                      metricText(gpuSnapshot(session)?.usedPercent ?? 0)
                    }})
                  </template>
                </span>
              </span>
              <span
                v-if="metricSnapshot(session)?.networks[0]"
                class="terminal-metric-item metric-network inline-flex items-center gap-[8px] flex-none"
              >
                <Separator orientation="vertical" />
                <Network :size="13" stroke-width="1.8" aria-hidden="true" />
                Net
                <b>
                  {{ metricSnapshot(session)?.networks[0]?.receiveText }}↓ /
                  {{ metricSnapshot(session)?.networks[0]?.transmitText }}↑
                </b>
              </span>
            </template>
            <span v-else class="terminal-metrics-loading text-qt-muted">{{
              t('shell.workspace.collectingMetrics')
            }}</span>
          </div>
        </div>
        <ResizeHandle
          v-for="handle in splitHandles"
          v-show="isSplit && !maximizedPaneId"
          :key="handle.key"
          class="split-resize-handle z-7"
          :class="`is-axis-${handle.axis}`"
          :orientation="handle.axis === 'x' ? 'vertical' : 'horizontal'"
          :label="handle.axis === 'x' ? t('shell.workspace.resizeHorizontal') : t('shell.workspace.resizeVertical')"
          :style="splitHandleStyle(handle)"
          @resize-start="beginSplitResize(handle)"
          @resize="resizeSplit(handle, $event)"
          @resize-end="finishSplitResize"
        />
        <div v-if="dragging" class="split-drag-shield absolute inset-0 z-8 [cursor:grabbing]" aria-hidden="true"></div>
        <div v-if="preview" class="split-drop-preview" :style="paneStyle(preview, stageSize)" aria-hidden="true">
          <span>{{
            target?.direction === 'center' ? t('shell.workspace.dropMove') : t('shell.workspace.dropSplit')
          }}</span>
        </div>
        <div v-if="!props.sessions.length" class="workspace-empty">
          <SquareTerminal :size="42" aria-hidden="true" />
          <h1>
            {{
              props.sessionWindow
                ? props.sessionActionError
                  ? t('shell.workspace.takeoverFailed')
                  : t('shell.workspace.takingOver')
                : t('shell.workspace.start')
            }}
          </h1>
          <p>
            {{
              props.sessionWindow
                ? props.sessionActionError || t('shell.workspace.restoring')
                : t('shell.workspace.addHostHint')
            }}
          </p>
          <div v-if="!props.sessionWindow" class="flex items-center gap-2">
            <QtermButton
              class="primary-button"
              type="button"
              data-empty-action="quick-connect"
              @click="emit('quickConnect')"
            >
              <Zap :size="16" aria-hidden="true" />
              {{ t('shell.workspace.quickConnect') }}
            </QtermButton>
            <QtermButton
              class="secondary-button"
              variant="outline"
              type="button"
              data-empty-action="new-local-terminal"
              @click="emit('createLocalTerminal')"
            >
              <SquareTerminal :size="16" aria-hidden="true" />
              {{ t('shell.workspace.newLocalTerminal') }}
            </QtermButton>
          </div>
        </div>
        <p v-if="props.sessionActionError" class="session-action-error text-sm" role="alert">
          {{ props.sessionActionError }}
        </p>
      </section>

      <div v-show="rdpActive()" class="rdp-workspace-stage" :aria-label="t('shell.workspace.rdp')">
        <slot name="rdp-workspace" :tabs="props.rdpTabs ?? []" :active-tab-id="props.activeRdpTabId ?? null" />
      </div>
      <div v-show="sftpActive()" class="sftp-workspace-stage" :aria-label="t('shell.workspace.sftp')">
        <slot name="sftp-workspace" :tabs="props.sftpTabs ?? []" :active-tab-id="props.activeSftpTabId ?? null" />
      </div>
      <p
        v-if="terminalControlError && !syncPickerOpen"
        class="terminal-control-bar m-0 border-t border-qt-border px-3 py-2 text-xs text-qt-danger"
        role="alert"
      >
        {{ terminalControlError }}
      </p>
      <div
        v-if="batchTask?.status === 'running' && !batchSendOpen"
        class="terminal-control-bar flex min-h-8 shrink-0 items-center gap-2 border-t border-qt-border px-3 py-1 text-xs"
        data-batch-running-bar
      >
        <QtermButton variant="ghost" size="xs" :aria-expanded="batchSendOpen" @click="batchSendOpen = true">
          <Network class="size-3.5" />{{ t('terminalControl.batch.title') }}
          <span class="text-qt-accent">· {{ batchTask.completedRounds ?? 0 }} / {{ batchTask.repeatCount }}</span>
        </QtermButton>
        <QtermButton variant="ghost" size="xs" :disabled="terminalControlBusy" @click="terminalControl.cancelBatch">
          <X class="size-3.5" />{{ t('terminalControl.batch.stop') }}
        </QtermButton>
      </div>
      <SyncStatusBar
        :group="syncGroup"
        @action="(action) => handleSyncAction(action === 'end' ? 'stop' : action, props.activeSessionId ?? '')"
      />
      <BatchSendPanel
        v-show="batchSendOpen"
        :sessions="controlSessions"
        :current-session-id="auxiliaryActive() ? '' : (props.activeSessionId ?? '')"
        :initial-session-id="controlSourceId"
        :sync-group="syncGroup"
        :task="batchTask"
        :log="batchLog"
        :busy="terminalControlBusy"
        :error="terminalControlError"
        @start="terminalControl.startBatch"
        @cancel="terminalControl.cancelBatch"
        @close="batchSendOpen = false"
      />
      <SyncTargetPicker
        :open="syncPickerOpen"
        :sessions="controlSessions"
        :source-session-id="controlSourceId"
        :selected-ids="syncPickerMode === 'manage' ? (syncGroup?.sessionIds ?? []) : []"
        :mode="syncPickerMode"
        :resume-on-apply="terminalControl.resumeAfterManage.value"
        :busy="terminalControlBusy"
        :error="terminalControlError"
        @submit="terminalControl.applyMembers"
        @cancel="terminalControl.cancelPicker"
      />
      <BatchCommandBar
        v-if="props.terminalBatchCommand && isSplit && !auxiliaryActive() && !batchSendOpen"
        :key="activeGroupId ?? ''"
        :sessions="props.terminalSessions.filter((session) => activeGroupSessionIds.includes(session.id))"
      />
    </div>
  </section>
</template>

<style>
.session-tabs.is-dock-target {
  outline: 2px dashed var(--qt-accent);
  outline-offset: -2px;
}

.tab-dock-hint {
  margin-left: 8px;
  flex: none;
  color: var(--qt-accent);
  white-space: nowrap;
}

.rdp-workspace-stage {
  position: absolute;
  z-index: 6;
  inset: 0;
  display: flex;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  background: var(--qt-terminal);
}

.rdp-workspace-stage > * {
  flex: 1;
  min-width: 0;
  min-height: 0;
}

.sftp-workspace-stage {
  position: absolute;
  z-index: 6;
  inset: 0;
  display: flex;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  background: var(--qt-terminal);
}

.sftp-workspace-stage > * {
  flex: 1;
  min-width: 0;
  min-height: 0;
}

.session-tab.is-sftp-tab .session-tab-trigger > svg {
  flex: none;
  color: var(--qt-accent);
}
</style>
