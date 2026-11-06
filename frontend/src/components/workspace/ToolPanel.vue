<script setup lang="ts">
import { IconButton, QtermButton, QtermEmptyState, QtermInput } from '../common'
import {
  Pin,
  Check,
  History,
  Cpu,
  MemoryStick,
  Monitor,
  Network,
  HardDrive,
  Plus,
  RefreshCw,
  Search,
  X,
  Eraser,
  FileCode2,
  Folder,
  Unplug,
} from '@lucide/vue'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import SftpPanel from '../../features/sftp/SftpPanel.vue'
import DockerPanel from '../../features/docker/DockerPanel.vue'
import AiPanel from '../../features/ai/AiPanel.vue'
import PortForwardPanel from '../../features/forwards/PortForwardPanel.vue'
import AuthenticationPanel from '../../features/otp/AuthenticationPanel.vue'
import MonitorPanel from '../../features/monitor/MonitorPanel.vue'
import AcceleratorCards from '../../features/monitor/AcceleratorCards.vue'
import ServicePanel from '../../features/services/ServicePanel.vue'
import DeleteConfirmDialog from '../dialogs/DeleteConfirmDialog.vue'
import CommandHistoryContextMenu from './CommandHistoryContextMenu.vue'
import { errorMessage } from '../../services/errors'
import type { HostRecord } from '@/services/hosts.ts'
import type { SessionGateway } from '../../services/sessions'
import type { SftpGateway } from '../../services/sftp'
import type { DockerGateway } from '../../services/docker'
import type { SessionTab, ToolPanel } from '../../types/qterm'
import type { ResourceSnapshot } from '../../services/sessions'
import type { CommandHistory } from '../../../bindings/changeme/internal/model/models'
import { ScrollArea } from '../common/control-primitives'
import { createSnippetGateway, type Snippet } from '../../services/snippets'
import { sendTerminalInput } from '../../services/terminal'
import { clearCommandHistory, deleteCommandHistory, listCommandHistory } from '../../services/command-history'
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '../common/control-primitives'
import {
  getTerminalTheme,
  loadCustomTerminalThemes,
  TERMINAL_THEME_OPTIONS,
  type TerminalTheme,
  type TerminalThemeId,
} from '../../features/sessions/terminalThemes'
import { useI18n } from '@/locales'
import SnippetIcon from '../../features/snippets/SnippetIcon.vue'
import ResourceSparkline from './ResourceSparkline.vue'
import { createResourceHistory } from '../../composables/useResourceHistory'
import { formatByteRate, formatPercent } from '../../lib/format'
import { snippetColorStyle } from '../../features/snippets/appearance'
import { useSnippetGroupPagination } from '../../features/snippets/useSnippetGroupPagination'
import QtermLoadMore from '../common/layout/QtermLoadMore.vue'

const props = defineProps<{
  tool: ToolPanel
  title: string
  activeHost: HostRecord | null
  activeSession: SessionTab | null
  sessionGateway: SessionGateway
  sftpGateway: SftpGateway

  dockerGateway: DockerGateway
  sftpAvailable: boolean
  dark?: boolean
  terminalLightTheme?: TerminalThemeId
  terminalDarkTheme?: TerminalThemeId
  directoryFollowMode?: import('../../features/settings/settingsStore').SftpDirectoryFollowMode
  autoDirectoryFollow?: boolean

  forwards?: import('../../services/forwards').PortForwardConfig[]

  forwardGroups?: string[]
  nextHosts?: import('../../services/hosts').HostRecord[]

  credentials?: import('../../services/credentials').CredentialRecord[]
  otpEntries?: import('../../services/otp').OtpEntry[]
}>()
const { t } = useI18n()
const serviceManager = ref('')
const displayedTitle = computed(() =>
  props.tool === 'services' && serviceManager.value ? `${props.title} (${serviceManager.value})` : props.title,
)
watch(
  () => [props.tool, props.activeSession?.id, props.activeSession?.state],
  () => {
    serviceManager.value = ''
  },
)

const SESSION_BOUND_TOOLS: readonly ToolPanel[] = ['resources', 'sftp', 'services', 'processes', 'docker']

const sessionClosed = computed(() => {
  const state = props.activeSession?.state
  return state === 'closing' || state === 'closed' || state === 'failed'
})

const showSessionClosed = computed(() => sessionClosed.value && SESSION_BOUND_TOOLS.includes(props.tool))

const effectiveDirectoryFollowMode = computed(() => {
  const mode = props.directoryFollowMode ?? 'remote'
  if (props.activeHost?.directoryFollow === 'disabled') return 'disabled'
  if (props.activeHost?.directoryFollow === 'enabled' && mode === 'disabled') return 'session'
  return mode
})

const emit = defineEmits<{
  close: []
  manageCredentials: []
  'update:forwards': [value: import('../../services/forwards').PortForwardConfig[]]
  'update:forwardGroups': [value: string[]]
  'update:otp-entries': [value: import('../../services/otp').OtpEntry[]]

  createSnippet: [content?: string]
  updateTerminalTheme: [kind: 'light' | 'dark', id: TerminalThemeId]
}>()
const resource = ref<ResourceSnapshot | null>(null)
const resourceError = ref('')
const resourceLoading = ref(false)

const resourceHistory = createResourceHistory()
const cpuTrend = computed(() => resourceHistory.points.value.map((point) => point.cpu))
const networkRxTrend = computed(() => resourceHistory.points.value.map((point) => point.rx))
const networkTxTrend = computed(() => resourceHistory.points.value.map((point) => point.tx))

const loadItems = computed(() => {
  const values = [resource.value?.loadAverage1, resource.value?.loadAverage5, resource.value?.loadAverage15]
  if (values.every((value) => value === null || value === undefined)) return []
  return [1, 5, 15].map((minutes, index) => {
    const value = values[index]
    return {
      minutes,
      label: t('shell.toolPanel.loadMinutes', { count: minutes }),
      value: value === null || value === undefined ? '—' : value.toFixed(2),
    }
  })
})
const memoryTrend = computed(() => resourceHistory.points.value.map((point) => point.memory))

const memoryCells = computed(() => {
  const value = resource.value
  if (!value || !value.metricsSupported) return []
  return [
    { label: t('shell.toolPanel.memoryUsed'), value: value.memoryUsedText || '—' },
    { label: t('shell.toolPanel.remaining'), value: value.memoryAvailableText || '—' },
    { label: t('shell.toolPanel.cached'), value: value.memoryCachedText || '—' },
  ]
})

const historyCaption = computed(() => {
  const points = resourceHistory.points.value
  if (points.length < 2) return ''
  const spanMs = points[points.length - 1].timestamp - points[0].timestamp
  const minutes = Math.max(1, Math.round(spanMs / 60000))
  return t('shell.toolPanel.historySpan', { count: minutes })
})
const snippets = ref<Snippet[]>([])
const snippetGroups = ref<Array<{ id: string; name: string }>>([])
const openSnippetGroups = ref<string[]>([])

const snippetsLoading = ref(true)
const snippetsError = ref('')

const snippetsLoadingVisible = ref(false)
let snippetsLoadingTimer: number | undefined
watch(
  snippetsLoading,
  (value) => {
    if (snippetsLoadingTimer !== undefined) window.clearTimeout(snippetsLoadingTimer)
    snippetsLoadingTimer = undefined
    if (!value) {
      snippetsLoadingVisible.value = false
      return
    }
    snippetsLoadingTimer = window.setTimeout(() => {
      snippetsLoadingVisible.value = true
    }, 200)
  },
  { immediate: true },
)

const commands = ref<CommandHistory[]>([])
const commandQuery = ref('')
const commandError = ref('')
const commandLoading = ref(false)
const commandBusy = ref(false)
const clearCommandsOpen = ref(false)

const {
  loadMore: loadMoreCommandPage,
  reset: resetCommandPagination,
  visibleItems: visibleCommandPage,
} = useSnippetGroupPagination(40)

const snippetQuery = ref('')
const snippetGateway = createSnippetGateway()
const {
  loadMore: loadMoreToolSnippetGroup,
  reset: resetToolSnippetGroupPagination,
  visibleCount: visibleToolSnippetGroupCount,
  visibleItems: visibleToolSnippetGroupItems,
} = useSnippetGroupPagination(20)
const customThemes = ref<Array<TerminalTheme & { id: string }>>(loadCustomTerminalThemes())
const terminalThemes = computed(() => {
  const kind = props.dark === true ? 'dark' : 'light'
  const builtins = TERMINAL_THEME_OPTIONS.filter((option) => option.type === kind).map((option) => ({
    id: option.value,
    ...getTerminalTheme(option.value, option.value),
  }))
  return [...builtins, ...customThemes.value.filter((theme) => theme.type === kind)]
})
const selectedTerminalTheme = computed(() => (props.dark === true ? props.terminalDarkTheme : props.terminalLightTheme))
const showCores = ref(false)
const showNetworks = ref(false)
const showDisks = ref(false)
const visibleNetworks = computed(() => {
  const items = resource.value?.networks ?? []
  if (showNetworks.value) return items
  const primary = items.find((item) => item.name === 'eth0') ?? items[0]
  return primary ? [primary] : []
})
const visibleDisks = computed(() => {
  const items = resource.value?.disks ?? []
  if (showDisks.value) return items
  const xfs = items.filter((item) => item.fileSystemType.toLowerCase() === 'xfs')
  if (xfs.length > 0) return xfs

  return items.filter((item) => item.mount.startsWith('/dev/'))
})
function tone(value: number, kind: 'cpu' | 'memory' | 'disk'): string {
  const limits = kind === 'cpu' ? [60, 80, 90] : [70, 85, 95]
  return value < limits[0] ? 'is-green' : value < limits[1] ? 'is-yellow' : value < limits[2] ? 'is-orange' : 'is-red'
}
function boundedPercent(value: number): number {
  return Number.isFinite(value) ? Math.min(100, Math.max(0, value)) : 0
}
let timer: number | undefined
let pollingVersion = 0
async function pollResource(version: number, sessionId: string) {
  resourceLoading.value = resource.value === null
  try {
    const next = await props.sessionGateway.readResources!(sessionId)
    if (version !== pollingVersion) return
    resource.value = next
    resourceError.value = ''
    resourceHistory.push(next)
  } catch (error) {
    if (version !== pollingVersion) return
    resource.value = null
    resourceError.value = errorMessage(error, t('shell.toolPanel.resourceUnavailable'))
  } finally {
    if (version === pollingVersion) {
      resourceLoading.value = false
      timer = window.setTimeout(() => void pollResource(version, sessionId), 5000)
    }
  }
}
function restartResourcePolling() {
  pollingVersion += 1
  if (timer) window.clearTimeout(timer)
  timer = undefined
  resource.value = null
  resourceError.value = ''
  resourceLoading.value = false
  resourceHistory.reset()
  const session = props.activeSession

  if (
    props.tool === 'resources' &&
    session?.kind !== 'local' &&
    session?.state === 'connected' &&
    props.sessionGateway.readResources
  ) {
    void pollResource(pollingVersion, session.id)
  }
}

const resourceEmptyNote = computed(() =>
  props.activeSession?.kind === 'local' ? t('shell.toolPanel.localResourceNote') : t('shell.toolPanel.sshResourceNote'),
)
const visibleSnippets = computed(() =>
  snippets.value.filter(
    (item) =>
      (!item.hostIds?.length || (!!props.activeHost && item.hostIds.includes(props.activeHost.id))) &&
      (!snippetQuery.value.trim() ||
        `${item.name} ${item.content}`.toLowerCase().includes(snippetQuery.value.trim().toLowerCase())),
  ),
)
const snippetSections = computed(() => {
  const byGroup = new Map<string, Snippet[]>()
  for (const item of visibleSnippets.value) {
    const key = item.pinned ? '__pinned__' : item.groupId || '__ungrouped__'
    const current = byGroup.get(key) ?? []
    current.push(item)
    byGroup.set(key, current)
  }
  const result: Array<{ id: string; name: string; items: Snippet[] }> = []
  const pinned = byGroup.get('__pinned__') ?? []
  if (pinned.length) result.push({ id: '__pinned__', name: t('settings.snippets.pinned'), items: pinned })
  byGroup.delete('__pinned__')
  for (const group of snippetGroups.value) {
    const items = byGroup.get(group.id) ?? []
    if (!snippetQuery.value.trim() || items.length) result.push({ id: group.id, name: group.name, items })
    byGroup.delete(group.id)
  }
  const ungrouped = byGroup.get('__ungrouped__') ?? []
  if (ungrouped.length) {
    result.push({ id: '__ungrouped__', name: t('shell.toolPanel.ungrouped'), items: ungrouped })
  }
  return result
})

function toolSnippetGroupExpanded(groupID: string): boolean {
  return openSnippetGroups.value.includes(groupID)
}

async function loadSnippets(): Promise<void> {
  if (props.tool !== 'snippets') return
  snippetsLoading.value = true
  snippetsError.value = ''
  try {
    const [items, groups] = await Promise.all([snippetGateway.list(), snippetGateway.listGroups()])
    snippets.value = items
    snippetGroups.value = groups.map((group) => ({ id: group.id, name: group.name }))
    resetToolSnippetGroupPagination()
    openSnippetGroups.value = snippetQuery.value.trim() ? snippetSections.value.map((group) => group.id) : []
  } catch (cause) {
    snippetsError.value = errorMessage(cause, t('shell.toolPanel.snippetsFailed'))
  } finally {
    snippetsLoading.value = false
  }
}
function runSnippet(item: Snippet): void {
  if (!props.activeSession || props.activeSession.state !== 'connected') return
  sendTerminalInput(props.activeSession.id, `${item.content}${item.autoExecute ? '\r' : ''}`)
}

const visibleCommands = computed(() => {
  if (!props.activeHost?.id) return []
  const query = commandQuery.value.trim().toLowerCase()
  if (!query) return commands.value
  return commands.value.filter((item) => item.command.toLowerCase().includes(query))
})

const paginatedCommands = computed(() => visibleCommandPage('command-history', visibleCommands.value, true))
function loadMoreCommands(): void {
  loadMoreCommandPage('command-history', visibleCommands.value.length)
}

let commandLoadVersion = 0
async function loadCommands(): Promise<void> {
  const hostID = props.activeHost?.id
  const version = ++commandLoadVersion

  resetCommandPagination()
  if (props.tool !== 'history' || !hostID) {
    commands.value = []
    commandError.value = ''
    commandLoading.value = false
    return
  }
  commandLoading.value = true
  try {
    const items = await listCommandHistory(hostID)
    if (version !== commandLoadVersion) return
    commands.value = items
    commandError.value = ''
  } catch (cause) {
    if (version !== commandLoadVersion) return
    commands.value = []
    commandError.value = errorMessage(cause, t('shell.toolPanel.historyFailed'))
  } finally {
    if (version === commandLoadVersion) commandLoading.value = false
  }
}

function runCommand(command: string): void {
  if (!props.activeSession || props.activeSession.state !== 'connected') return
  sendTerminalInput(props.activeSession.id, `${command}\r`)
}
async function removeCommand(id: string): Promise<void> {
  if (commandBusy.value) return
  commandBusy.value = true
  try {
    await deleteCommandHistory(id)
    await loadCommands()
  } catch (cause) {
    commandError.value = errorMessage(cause, t('shell.toolPanel.deleteHistoryFailed'))
  } finally {
    commandBusy.value = false
  }
}
async function clearCommands(): Promise<void> {
  const hostID = props.activeHost?.id
  if (!hostID || commandBusy.value) return
  commandBusy.value = true
  try {
    await clearCommandHistory(hostID)
    clearCommandsOpen.value = false
    await loadCommands()
  } catch (cause) {
    commandError.value = errorMessage(cause, t('shell.toolPanel.clearHistoryFailed'))
  } finally {
    commandBusy.value = false
  }
}
function commandTime(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleString()
}
watch(() => [props.tool, props.activeSession?.id, props.activeSession?.state], restartResourcePolling, {
  immediate: true,
})
watch(() => props.tool, loadSnippets, { immediate: true })
watch(snippetQuery, (query) => {
  resetToolSnippetGroupPagination()
  openSnippetGroups.value = query.trim() ? snippetSections.value.map((group) => group.id) : []
})

watch(
  () => [props.tool, props.activeHost?.id, props.activeSession?.id],
  () => {
    commandQuery.value = ''
    void loadCommands()
  },
  { immediate: true },
)

watch(commandQuery, () => resetCommandPagination())

function handleCommandHistoryChanged(event: Event): void {
  const hostID = (event as CustomEvent<{ hostId?: string }>).detail?.hostId
  if (hostID && hostID !== props.activeHost?.id) return
  void loadCommands()
}
onBeforeUnmount(() => {
  pollingVersion += 1
  if (timer) window.clearTimeout(timer)
  if (snippetsLoadingTimer !== undefined) window.clearTimeout(snippetsLoadingTimer)
  window.removeEventListener('qterm:command-history-changed', handleCommandHistoryChanged)
  window.removeEventListener('qterm:assets-imported', handleAssetsImported)
})
onMounted(() => {
  window.addEventListener('qterm:command-history-changed', handleCommandHistoryChanged)
  window.addEventListener('qterm:assets-imported', handleAssetsImported)
})

function handleAssetsImported(): void {
  void loadSnippets()
}
</script>

<template>
  <aside class="tool-panel" :aria-label="displayedTitle">
    <header class="tool-panel-header">
      <strong class="text-sm">{{ displayedTitle }}</strong>
      <IconButton class="tool-panel-close" size="icon" :label="t('shell.toolPanel.close')" @click="emit('close')"
        ><X :size="16"
      /></IconButton>
    </header>
    <div
      class="tool-panel-content min-h-0 h-full overflow-hidden p-0 bg-(--qt-glass-surface,var(--qt-bar,var(--qt-panel)))"
    >
      <QtermEmptyState
        v-if="showSessionClosed"
        class="h-full"
        :icon="Unplug"
        :title="t('shell.toolPanel.sessionClosed')"
        :description="t('shell.toolPanel.sessionClosedHint')"
      />
      <template v-else-if="props.tool === 'resources'">
        <ScrollArea class="h-full" type="hover" :scroll-hide-delay="600">
          <div class="resource-scroll-content px-2 pt-2 pb-2">
            <template v-if="resource">
              <div class="resource-card">
                <div class="resource-card-title flex items-center gap-[7px] font-normal mb-[10px]">
                  <Monitor :size="15" />{{ t('shell.toolPanel.system') }}
                </div>
                <div class="resource-grid text-sm grid grid-cols-[1fr_1.3fr] gap-[7px_10px]">
                  <span>{{ t('shell.toolPanel.host') }}</span
                  ><strong>{{ resource.hostname }}</strong
                  ><span>{{ t('shell.toolPanel.architecture') }}</span
                  ><strong>{{ resource.architecture || '—' }}</strong
                  ><span>{{ t('shell.toolPanel.os') }}</span
                  ><strong>{{ resource.os || '—' }}</strong
                  ><span>{{ t('shell.toolPanel.uptime') }}</span
                  ><strong>{{
                    t('shell.toolPanel.uptimeDays', { count: Math.floor(resource.uptimeSeconds / 86400) })
                  }}</strong>
                </div>
              </div>
              <div class="resource-card">
                <button
                  class="resource-card-title resource-card-button flex items-center gap-[7px] font-normal mb-[10px] w-full border-0 bg-transparent p-0 [color:inherit] text-left c-p"
                  type="button"
                  @click="showCores = !showCores"
                >
                  <Cpu :size="15" />CPU <span class="resource-card-spacer flex-1" /><strong>{{
                    t('shell.toolPanel.cpuCores', { count: resource.cpuCores || '—' })
                  }}</strong>
                </button>
                <div class="metric-row min-h-[28px] flex-cb text-qt-muted">
                  <span>{{ t('shell.toolPanel.averageUsage') }}</span
                  ><strong class="text-qt-accent font-mono tabular-nums"
                    >{{ resource.cpuUsagePercent.toFixed(1) }}%</strong
                  >
                </div>
                <div
                  class="metric-track h-[5px] rounded-[999px] [background:color-mix(in_srgb,_var(--qt-muted)_18%,_transparent)] overflow-hidden"
                >
                  <span
                    :class="tone(resource.cpuUsagePercent, 'cpu')"
                    :style="{ width: `${boundedPercent(resource.cpuUsagePercent)}%` }"
                  />
                </div>
                <div v-if="loadItems.length" class="grid grid-cols-3 gap-2 mt-[8px]">
                  <div
                    v-for="item in loadItems"
                    :key="item.minutes"
                    class="rounded-md border border-qt-border px-2 py-[6px] text-center"
                  >
                    <strong class="block text-sm font-semibold text-qt-accent font-mono tabular-nums">{{
                      item.value
                    }}</strong>
                    <span class="block text-[0.625rem] text-qt-muted">{{ item.label }}</span>
                  </div>
                </div>
                <ResourceSparkline
                  class="mt-[6px]"
                  :series="[{ label: 'CPU', color: '#f59e0b', values: cpuTrend }]"
                  :empty-text="t('shell.toolPanel.collecting')"
                  :peak-label="t('shell.toolPanel.peakRate')"
                  :format-value="formatPercent"
                  :caption="historyCaption"
                />
                <div v-if="showCores" class="core-list mt-[8px] border-t-d pt-[6px]">
                  <div
                    v-for="(usage, index) in resource.cpuPerCore"
                    :key="index"
                    class="metric-row min-h-[28px] flex-cb text-qt-muted"
                  >
                    <span>{{ index + 1 }}</span>
                    <div
                      class="metric-track core-progress flex-1 mx-[10px] min-w-[90px] h-[5px] rounded-[999px] [background:color-mix(in_srgb,_var(--qt-muted)_18%,_transparent)] overflow-hidden"
                    >
                      <span :class="tone(usage, 'cpu')" :style="{ width: `${boundedPercent(usage)}%` }" />
                    </div>
                    <strong :class="tone(usage, 'cpu')">{{ usage.toFixed(1) }}%</strong>
                  </div>
                </div>
              </div>
              <div class="resource-card">
                <div class="resource-card-title flex items-center gap-[7px] font-normal mb-[10px]">
                  <MemoryStick :size="15" />{{ t('shell.toolPanel.memory') }}
                </div>
                <div class="metric-row min-h-[28px] flex-cb text-qt-muted">
                  <span>{{ t('shell.toolPanel.usage') }}</span
                  ><strong class="text-qt-accent font-mono tabular-nums"
                    >{{ resource.memoryUsagePercent.toFixed(1) }}%</strong
                  >
                </div>
                <div
                  class="metric-track h-[5px] rounded-[999px] [background:color-mix(in_srgb,_var(--qt-muted)_18%,_transparent)] overflow-hidden"
                >
                  <span
                    :class="tone(resource.memoryUsagePercent, 'memory')"
                    :style="{ width: `${boundedPercent(resource.memoryUsagePercent)}%` }"
                  />
                </div>
                <div class="metric-row min-h-[28px] flex justify-end text-qt-muted">
                  <strong
                    ><span class="text-qt-accent font-mono tabular-nums">{{ resource.memoryUsedText }}</span>
                    <span class="text-xs text-qt-muted"> / {{ resource.memoryTotalText }}</span></strong
                  >
                </div>
                <div v-if="memoryCells.length" class="grid grid-cols-3 gap-2 mt-[8px]">
                  <div
                    v-for="item in memoryCells"
                    :key="item.label"
                    class="min-w-0 rounded-md border border-qt-border px-2 py-[6px] text-center"
                  >
                    <strong
                      class="block truncate text-sm font-semibold text-qt-accent font-mono tabular-nums"
                      :title="item.value"
                      >{{ item.value }}</strong
                    >
                    <span class="block truncate text-[0.625rem] text-qt-muted">{{ item.label }}</span>
                  </div>
                </div>
                <ResourceSparkline
                  class="mt-[6px]"
                  :series="[{ label: 'MEM', color: '#8b5cf6', values: memoryTrend }]"
                  :empty-text="t('shell.toolPanel.collecting')"
                  :peak-label="t('shell.toolPanel.peakRate')"
                  :format-value="formatPercent"
                  :caption="historyCaption"
                />
              </div>

              <AcceleratorCards
                :session-id="props.activeSession?.id ?? ''"
                :available="props.activeSession?.kind === 'ssh' && props.activeSession?.state === 'connected'"
              />
              <div class="resource-card">
                <button
                  class="resource-card-title resource-card-button flex items-center gap-[7px] font-normal mb-[10px] w-full border-0 bg-transparent p-0 [color:inherit] text-left c-p"
                  type="button"
                  @click="showNetworks = !showNetworks"
                >
                  <Network :size="15" />{{ t('shell.toolPanel.network') }}
                  <span class="resource-card-spacer flex-1" />{{
                    showNetworks ? t('shell.toolPanel.collapse') : t('shell.toolPanel.expand')
                  }}
                </button>
                <div
                  v-for="item in visibleNetworks"
                  :key="item.name"
                  class="metric-row min-h-[28px] flex-cb text-qt-muted"
                >
                  <span>{{ item.name }}</span
                  ><strong
                    ><span class="network-rx text-[#22c55e]">↓ {{ item.receiveText }}/s</span> ·
                    <span class="network-tx text-[#3b82f6]">↑ {{ item.transmitText }}/s</span></strong
                  >
                </div>
                <ResourceSparkline
                  class="mt-[6px]"
                  :series="[
                    { label: 'RX', color: '#22c55e', values: networkRxTrend },
                    { label: 'TX', color: '#3b82f6', values: networkTxTrend },
                  ]"
                  :empty-text="t('shell.toolPanel.collecting')"
                  :peak-label="t('shell.toolPanel.peakRate')"
                  :format-value="formatByteRate"
                  :caption="historyCaption"
                />
              </div>
              <div class="resource-card">
                <button
                  class="resource-card-title resource-card-button flex items-center gap-[7px] font-normal mb-[10px] w-full border-0 bg-transparent p-0 [color:inherit] text-left c-p"
                  type="button"
                  @click="showDisks = !showDisks"
                >
                  <HardDrive :size="15" />{{ t('shell.toolPanel.disk') }} <span class="resource-card-spacer flex-1" />{{
                    showDisks ? t('shell.toolPanel.collapse') : t('shell.toolPanel.expand')
                  }}
                </button>
                <div v-for="item in visibleDisks" :key="item.mount" class="disk-item">
                  <div class="metric-row min-h-[28px] flex-cb text-qt-muted">
                    <span>{{ item.mount }}</span
                    ><strong class="text-qt-accent font-mono tabular-nums">{{ item.usedPercent.toFixed(0) }}%</strong>
                  </div>
                  <div
                    class="metric-track h-[5px] rounded-[999px] [background:color-mix(in_srgb,_var(--qt-muted)_18%,_transparent)] overflow-hidden"
                  >
                    <span
                      :class="tone(item.usedPercent, 'disk')"
                      :style="{ width: `${boundedPercent(item.usedPercent)}%` }"
                    />
                  </div>
                  <div class="metric-row disk-size min-h-[24px] flex-cb text-qt-muted">
                    <span
                      ><span class="text-qt-accent font-mono tabular-nums">{{ item.usedText }}</span>
                      <span class="text-xs text-qt-muted"> / {{ item.totalText }}</span></span
                    >
                  </div>
                </div>
              </div>
            </template>
            <p v-if="!resource" class="panel-empty-note text-qt-muted text-xs">
              {{ resourceLoading ? t('shell.toolPanel.readingResources') : resourceError || resourceEmptyNote }}
            </p>
          </div>
        </ScrollArea>
      </template>

      <template v-else-if="props.tool === 'themes'">
        <ScrollArea class="h-full" type="hover" :scroll-hide-delay="500">
          <div class="tool-themes-content min-h-full p-[12px]">
            <header class="tool-themes-header grid gap-[3px] mb-[10px] text-sm text-qt-text">
              <strong>{{ props.dark ? t('shell.toolPanel.darkTheme') : t('shell.toolPanel.lightTheme') }}</strong>
              <span class="text-xs">{{ t('shell.toolPanel.themeHint') }}</span>
            </header>
            <div class="terminal-theme-list-content grid gap-2 p-[1px_6px_2px_1px]">
              <article
                v-for="theme in terminalThemes"
                :key="theme.id"
                class="terminal-theme-card"
                :data-theme-id="theme.id"
                :class="{ 'is-selected': selectedTerminalTheme === theme.id }"
                :style="{ '--theme-bg': theme.background, '--theme-fg': theme.foreground }"
                tabindex="0"
                role="button"
                :aria-pressed="selectedTerminalTheme === theme.id"
                @click="emit('updateTerminalTheme', props.dark ? 'dark' : 'light', theme.id)"
                @keydown.enter="emit('updateTerminalTheme', props.dark ? 'dark' : 'light', theme.id)"
                @keydown.space.prevent="emit('updateTerminalTheme', props.dark ? 'dark' : 'light', theme.id)"
              >
                <span class="terminal-theme-card-title flex-cb pr-[86px] text-sm">
                  <span class="terminal-theme-card-name inline-flex min-w-0 items-center gap-1">
                    {{ theme.name }}
                    <Check
                      v-if="selectedTerminalTheme === theme.id"
                      class="terminal-theme-card-check flex-none text-qt-accent"
                      :size="15"
                    />
                  </span>
                </span>
                <span class="terminal-theme-preview text-xs">
                  <span><b :style="{ color: theme.green }">root@qterm</b>$ ls</span>
                  <span
                    ><span :style="{ color: theme.cyan }">drwxr-xr-x</span> 1 root
                    <span :style="{ color: theme.blue }">boot</span></span
                  >
                  <span
                    ><span :style="{ color: theme.cyan }">drwxr-xr-x</span> 1 root
                    <span :style="{ color: theme.blue }">data</span></span
                  >
                </span>
              </article>
            </div>
          </div>
        </ScrollArea>
      </template>

      <template v-else-if="props.tool === 'snippets'">
        <div class="flex h-full min-h-0 flex-col">
          <div class="snippet-panel-toolbar flex flex-none items-center gap-[6px] px-2 pt-2 pb-2">
            <label class="snippet-panel-search relative flex items-center text-qt-muted">
              <Search :size="16" aria-hidden="true" />
              <QtermInput
                v-model="snippetQuery"
                class="qterm-reference-input text-qt-text"
                :placeholder="t('shell.toolPanel.searchSnippets')"
                :aria-label="t('shell.toolPanel.searchSnippets')"
              />
              <QtermButton
                v-if="snippetQuery"
                variant="ghost"
                size="icon-xs"
                class="snippet-panel-search-clear"
                type="button"
                :aria-label="t('shell.toolPanel.clearSearch')"
                @click.prevent="snippetQuery = ''"
                ><X :size="16"
              /></QtermButton>
            </label>
            <header class="snippet-panel-header flex-none">
              <div>
                <QtermButton
                  variant="ghost"
                  size="icon"
                  :aria-label="t('shell.toolPanel.createSnippet')"
                  @click="emit('createSnippet')"
                  ><Plus :size="16" /></QtermButton
                ><QtermButton
                  variant="ghost"
                  size="icon"
                  :aria-label="t('shell.toolPanel.refreshSnippets')"
                  @click="loadSnippets"
                  ><RefreshCw :size="16"
                /></QtermButton>
              </div>
            </header>
          </div>
          <ScrollArea class="tool-snippets-scroll min-h-0 flex-1" type="hover" :scroll-hide-delay="600">
            <div class="tool-snippets-content min-h-full px-2 pb-2">
              <p v-if="snippetsLoadingVisible" class="panel-empty-note text-qt-muted text-xs">
                {{ t('shell.toolPanel.loadingSnippets') }}
              </p>
              <p v-else-if="snippetsError" class="panel-empty-note text-qt-muted text-xs">{{ snippetsError }}</p>
              <template v-else-if="snippetSections.length">
                <Accordion v-model="openSnippetGroups" type="multiple" class="snippet-panel-groups mt-0.5">
                  <AccordionItem
                    v-for="group in snippetSections"
                    :key="group.id"
                    :value="group.id"
                    class="snippet-panel-group overflow-hidden rounded-[8px] border border-qt-border not-first:mt-2"
                  >
                    <AccordionTrigger
                      class="snippet-panel-group-trigger w-full rounded-none border-0 px-2.5 py-2 text-sm hover:no-underline"
                    >
                      <span class="flex min-w-0 items-center gap-1.5 text-qt-muted">
                        <Pin v-if="group.id === '__pinned__'" :size="13" class="shrink-0" aria-hidden="true" />
                        <Folder v-else :size="13" class="shrink-0" aria-hidden="true" />
                        <span class="min-w-0 [overflow-wrap:anywhere] text-qt-text">{{ group.name }}</span>
                        <small class="text-xs font-normal text-qt-muted">（{{ group.items.length }}）</small>
                      </span>
                    </AccordionTrigger>
                    <AccordionContent>
                      <QtermButton
                        v-for="item in visibleToolSnippetGroupItems(
                          group.id,
                          group.items,
                          toolSnippetGroupExpanded(group.id),
                        )"
                        :key="item.id"
                        variant="ghost"
                        class="snippet-panel-row flex w-full min-w-0 items-start text-left"
                        :class="{ 'has-snippet-color': !!snippetColorStyle(item.color) }"
                        :style="snippetColorStyle(item.color)"
                        type="button"
                        :disabled="!props.activeSession || props.activeSession.state !== 'connected'"
                        @click="runSnippet(item)"
                      >
                        <SnippetIcon :icon="item.icon" /><span
                          ><strong>{{ item.name }}</strong
                          ><small>{{ item.content }}</small></span
                        >
                      </QtermButton>
                      <QtermLoadMore
                        v-if="toolSnippetGroupExpanded(group.id) && group.items.length"
                        class="px-2 py-1.5"
                        :visible="visibleToolSnippetGroupCount(group.id, group.items.length, true)"
                        :total="group.items.length"
                        :load-more-label="t('settings.snippets.pagination.loadMore')"
                        @load-more="loadMoreToolSnippetGroup(group.id, group.items.length)"
                      />
                    </AccordionContent>
                  </AccordionItem>
                </Accordion>
              </template>

              <div v-else class="py-2">
                <QtermEmptyState
                  :icon="snippetQuery.trim() ? Search : FileCode2"
                  :title="snippetQuery.trim() ? t('shell.toolPanel.noSnippetsMatch') : t('shell.toolPanel.noSnippets')"
                  :description="snippetQuery.trim() ? '' : t('shell.toolPanel.noSnippetsHint')"
                  variant="outlined"
                  class="mx-1 px-3"
                >
                  <QtermButton
                    v-if="!snippetQuery.trim()"
                    variant="outline"
                    size="sm"
                    class="mt-1"
                    @click="emit('createSnippet')"
                  >
                    <Plus :size="14" />{{ t('shell.toolPanel.createSnippet') }}
                  </QtermButton>
                </QtermEmptyState>
              </div>
            </div>
          </ScrollArea>
        </div>
      </template>

      <SftpPanel
        v-else-if="props.tool === 'sftp'"
        :gateway="props.sftpGateway"
        :session-id="props.activeSession?.id ?? ''"
        :available="props.sftpAvailable"
        :preview="!props.sftpGateway.persistent"
        :show-transfer-panel="false"
        :directory-follow-mode="effectiveDirectoryFollowMode"
        :auto-directory-follow="props.autoDirectoryFollow"
        :directory-follow-enabled="props.activeHost?.directoryFollow === 'enabled'"
        :show-hidden-files="props.activeHost?.sftpShowHiddenFiles || 'inherit'"
        :default-directory="props.activeHost?.sftpDefaultDirectory || ''"
        hide-modified-column
      />

      <ServicePanel
        v-else-if="props.tool === 'services'"
        :session-id="props.activeSession?.id ?? ''"
        :available="props.activeSession?.kind === 'ssh' && props.activeSession?.state === 'connected'"
        @manager-changed="serviceManager = $event"
      />

      <MonitorPanel
        v-else-if="props.tool === 'processes'"
        kind="processes"
        :session-id="props.activeSession?.id ?? ''"
        :available="props.activeSession?.kind === 'ssh' && props.activeSession?.state === 'connected'"
      />

      <AuthenticationPanel
        v-else-if="props.tool === 'otp'"
        :credentials="props.credentials ?? []"
        :host="props.activeHost"
        @manage="emit('manageCredentials')"
        :entries="props.otpEntries ?? []"
        :hosts="props.nextHosts ?? []"
        :active-host-id="props.activeHost?.id ?? ''"
        @update:entries="emit('update:otp-entries', $event)"
      />

      <PortForwardPanel
        v-else-if="props.tool === 'tunnels'"
        :forwards="props.forwards ?? []"
        :forward-groups="props.forwardGroups ?? []"
        :hosts="props.nextHosts ?? []"
        :active-host-id="props.activeHost?.id ?? ''"
        @update:forwards="emit('update:forwards', $event)"
        @update:forward-groups="emit('update:forwardGroups', $event)"
      />

      <DockerPanel
        v-else-if="props.tool === 'docker'"
        :gateway="props.dockerGateway"
        :session-id="props.activeSession?.id ?? ''"
        :host-id="props.activeHost?.id ?? ''"
      />

      <AiPanel
        v-else-if="props.tool === 'ai'"
        :host-id="props.activeSession?.hostId ?? ''"
        :session-id="props.activeSession?.id ?? ''"
        :session-title="props.activeSession?.title ?? ''"
        :session-state="props.activeSession?.state ?? 'closed'"
      />

      <div
        v-else-if="props.tool === 'history'"
        class="command-history-panel flex flex-col h-full min-h-0 pt-2 pb-2 bg-(--qt-glass-surface,var(--qt-bar,var(--qt-panel)))"
      >
        <div class="command-history-fixed grid flex-none gap-3 px-2 pb-2">
          <QtermInput
            v-model="commandQuery"
            :placeholder="
              props.activeHost
                ? t('shell.toolPanel.searchCommands', { host: props.activeHost.name })
                : t('shell.toolPanel.selectHostForHistory')
            "
            :disabled="!props.activeHost"
            :aria-label="t('shell.toolPanel.commandHistoryAria')"
          />
          <div class="command-history-toolbar text-xs flex-cb gap-[0.5rem] text-qt-muted">
            <small class="text-xs">{{
              props.activeHost
                ? t('shell.toolPanel.commandCount', { count: visibleCommands.length })
                : t('shell.toolPanel.noHost')
            }}</small>
            <IconButton
              :label="t('shell.toolPanel.clearHistory')"
              :disabled="!props.activeHost || !commands.length || commandBusy"
              @click="clearCommandsOpen = true"
              ><Eraser
            /></IconButton>
          </div>
        </div>

        <div
          v-if="props.activeHost && (commands.length || commandLoading || commandError)"
          class="command-history-list relative ml-2 mr-0 mb-2 flex min-h-0 flex-[1_1_auto]"
        >
          <ScrollArea class="command-history-scroll min-h-0 w-full flex-1">
            <p v-if="commandLoading" class="panel-empty-note py-2 pl-2 pr-4 text-qt-muted text-xs">
              {{ t('shell.toolPanel.loadingHistory') }}
            </p>
            <p v-else-if="commandError" class="panel-empty-note py-2 pl-2 pr-4 text-qt-muted text-xs">
              {{ commandError }}
            </p>

            <template v-else-if="visibleCommands.length">
              <div class="flex flex-col space-y-0.5 py-1 pl-2 pr-4">
                <CommandHistoryContextMenu
                  v-for="item in paginatedCommands"
                  :key="item.id"
                  :can-delete="!commandBusy"
                  @add-snippet="emit('createSnippet', item.command)"
                  @remove="removeCommand(item.id)"
                >
                  <div
                    class="command-history-item c-p grid grid-cols-[minmax(0,_1fr)_auto] items-center gap-[9px] min-h-[32px] rounded-md px-1.5 py-2"
                  >
                    <button
                      class="command-history-text text-xs"
                      type="button"
                      :title="
                        props.activeSession?.state === 'connected'
                          ? t('shell.toolPanel.doubleClickRun', { command: item.command })
                          : t('shell.toolPanel.connectToRun')
                      "
                      :disabled="props.activeSession?.state !== 'connected'"
                      @dblclick="runCommand(item.command)"
                    >
                      {{ item.command }}
                    </button>
                    <small class="command-history-time text-qt-muted whitespace-nowrap text-xs">{{
                      commandTime(item.executedAt)
                    }}</small>
                  </div>
                </CommandHistoryContextMenu>
              </div>

              <QtermLoadMore
                class="pb-1 pl-2 pr-4"
                :visible="paginatedCommands.length"
                :total="visibleCommands.length"
                :load-more-label="t('shell.common.loadMore')"
                @load-more="loadMoreCommands"
              />
            </template>
            <p v-else class="panel-empty-note py-2 pl-2 pr-4 text-qt-muted text-xs">
              {{ t('shell.toolPanel.noMatchingHistory') }}
            </p>
          </ScrollArea>

          <span
            class="pointer-events-none absolute inset-y-0 left-0 right-2 rounded-[8px] border border-qt-border"
            aria-hidden="true"
          />
        </div>

        <ScrollArea v-else class="command-history-scroll flex-[1_1_auto] min-h-0">
          <QtermEmptyState
            v-if="!props.activeHost"
            :icon="History"
            :title="t('shell.toolPanel.noHostHistoryTitle')"
            :description="t('shell.toolPanel.noHostHistoryDescription')"
          />
          <QtermEmptyState
            v-else
            :icon="History"
            :title="t('shell.toolPanel.emptyHistoryTitle')"
            :description="t('shell.toolPanel.emptyHistoryDescription')"
          />
        </ScrollArea>
        <DeleteConfirmDialog
          :open="clearCommandsOpen"
          :title="t('shell.toolPanel.clearHistory')"
          :message="props.activeHost ? t('shell.toolPanel.clearHistoryMessage', { host: props.activeHost.name }) : ''"
          :hint="t('shell.toolPanel.clearHistoryHint')"
          :confirm-text="t('shell.toolPanel.clearHistory')"
          :loading="commandBusy"
          @close="clearCommandsOpen = false"
          @confirm="clearCommands"
        />
      </div>
    </div>
  </aside>
</template>
