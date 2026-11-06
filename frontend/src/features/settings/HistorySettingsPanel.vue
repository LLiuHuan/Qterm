<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch, type Ref } from 'vue'
import {
  Archive,
  ArchiveRestore,
  Check,
  Download,
  Edit3,
  Maximize,
  Minimize,
  Pause,
  Play,
  Search,
  Minus,
  Square,
  Trash2,
  X,
} from '@lucide/vue'
import { toast } from 'vue-sonner'
import {
  IconButton,
  QtermButton,
  QtermCheckbox,
  QtermInput,
  QtermSwitch,
  QtermTextarea,
  SegmentedTabs,
} from '../../components/common'

import { ScrollArea } from '../../components/common/control-primitives'
import {
  Select as QtermSelect,
  SelectItem as QtermSelectItem,
  SelectTrigger as QtermSelectTrigger,
} from '../../components/common/form-controls'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { SelectContent, SelectValue } from '../../components/common/form-controls'
import DeleteConfirmDialog from '../../components/dialogs/DeleteConfirmDialog.vue'
import ReplayTerminal from '../../components/workspace/ReplayTerminal.vue'
import AsciinemaReplay from '../../components/workspace/AsciinemaReplay.vue'
import { errorMessage } from '@/services/errors.ts'
import {
  archiveSessionHistories,
  clearUnarchivedSessionHistory,
  deleteSessionHistory,
  listSessionHistory,
  updateSessionHistoryNote,
  type SessionHistoryRecord,
} from '@/services/history.ts'
import {
  deleteRecording,
  deleteRecordings,
  exportRecordings,
  exportRecordingsText,
  listRecordingPage,
  listRecordings,
  updateRecordingNote,
} from '@/services/recordings.ts'
import type { TerminalRecording } from '../../../bindings/changeme/internal/model'
import type { TerminalThemeId } from '@/features/sessions/terminalThemes.ts'
import { useI18n } from '@/locales'
import { useHistoryReplay } from './useHistoryReplay'

const props = withDefaults(
  defineProps<{
    // eslint-disable-next-line vue/require-default-prop -- undefined 表示独立视图，此时自带页签切换（模板用 !props.view 判断）。
    view?: 'history' | 'recordings'

    dark?: boolean
    terminalLightTheme?: TerminalThemeId
    terminalDarkTheme?: TerminalThemeId
  }>(),
  {
    dark: false,
    terminalLightTheme: 'default-light',
    terminalDarkTheme: 'default-dark',
  },
)
const { t } = useI18n()

const items = ref<SessionHistoryRecord[]>([])
const loading = ref(false)
const error = ref('')
const query = ref('')
const recordingQuery = ref('')

const selectedHistory = ref<string[]>([])
const selectedRecordings = ref<string[]>([])
const busy = ref(false)
const pendingDelete = ref<SessionHistoryRecord | null>(null)
const deleting = ref(false)

const clearUnarchivedOpen = ref(false)
const clearingUnarchived = ref(false)
const pendingNote = ref<SessionHistoryRecord | null>(null)
const pendingRecordingNote = ref<TerminalRecording | null>(null)
const noteDraft = ref('')
const noteSaving = ref(false)
const pendingRecordingDelete = ref<TerminalRecording | null>(null)
const recordingDeleting = ref(false)

const recordingBatchDeleteOpen = ref(false)
const recordingBatchDeleting = ref(false)
const activeTab = ref<'history' | 'recordings'>(props.view ?? 'history')
const historyView = ref<'active' | 'archived'>('active')

const recordings = ref<TerminalRecording[]>([])
const recordingsLoading = ref(false)

const recordingsLoadingMore = ref(false)
const recordingsError = ref('')

const recordingPage = ref(1)
const recordingTotal = ref(0)
const RECORDING_PAGE_SIZE = 20
const HISTORY_PAGE_SIZE = 20
let recordingLoadGeneration = 0
let recordingSearchTimer: number | undefined

const historyVisibleCount = ref(HISTORY_PAGE_SIZE)

const recordingIndex = ref<Record<string, TerminalRecording>>({})
const recordingExportIndex = ref<Record<string, TerminalRecording>>({})
let recordingIndexGeneration = 0

const replayFrame = ref<HTMLElement | null>(null)
const {
  closeReplayDialog,
  dynamicReplayPlayer,
  formatPlaybackTime,
  onDynamicReplayEnded,
  onDynamicReplayError,
  onDynamicReplayPause,
  onDynamicReplayPlay,
  onDynamicReplayReady,
  onDynamicReplayTimeupdate,
  replayColumns,
  replayDurationMs,
  replayElapsedMs,
  replayEvents,
  replayFullscreen,
  replayLoading,
  replayMode,
  replayOpen,
  replayProgress,
  replayRecordingId,
  replayRows,
  replayRunning,
  replaySpeed,
  replaySuspended,
  replayText,
  replayRecording,
  seekReplay,
  stopReplay,
  toggleFullscreen,
  toggleReplayRunning,
} = useHistoryReplay({ t })
const exportOpen = ref(false)
const exporting = ref(false)
const exportFormat = ref<'txt' | 'cast'>('txt')
const exportIncludeOutput = ref(true)
const exportItems = ref<TerminalRecording[]>([])

watch(exportFormat, (format) => {
  if (format === 'cast') exportIncludeOutput.value = true
})

const HISTORY_STATUS_KEYS: Record<SessionHistoryRecord['status'], string> = {
  connected: 'history.status.connected',
  closed: 'history.status.closed',
  failed: 'history.status.failed',
  interrupted: 'history.status.interrupted',
}

const RECORDING_STATUS_KEYS: Record<string, string> = {
  recording: 'history.recording.status.recording',
  paused: 'history.recording.status.paused',
  completed: 'history.recording.status.completed',
  interrupted: 'history.recording.status.interrupted',
}

const SPEED_OPTIONS = [
  { value: '0.5', label: '0.5×' },
  { value: '1', label: '1×' },
  { value: '2', label: '2×' },
  { value: '4', label: '4×' },
]

const visibleItems = computed(() => {
  const keyword = query.value.trim().toLowerCase()
  const filtered = items.value.filter((item) =>
    historyView.value === 'archived' ? item.archived === true : item.archived !== true,
  )
  if (!keyword) return filtered
  return filtered.filter((item) =>
    `${item.hostName} ${item.address} ${statusLabel(item)} ${item.note ?? ''}`.toLowerCase().includes(keyword),
  )
})

const historySelectAllState = computed<boolean | 'indeterminate'>(() => {
  if (!visibleHistoryIds.value.length) return false
  if (allVisibleHistorySelected.value) return true
  return selectedHistoryCount.value > 0 ? 'indeterminate' : false
})

const historyPageItems = computed(() => visibleItems.value.slice(0, historyVisibleCount.value))
const historyHasMore = computed(() => visibleItems.value.length > historyVisibleCount.value)

const visibleHistoryIds = computed(() => visibleItems.value.map((item) => item.id))
const selectedHistorySet = computed(() => new Set(selectedHistory.value))
const selectedHistoryCount = computed(() => selectedHistory.value.length)
const allVisibleHistorySelected = computed(
  () => visibleHistoryIds.value.length > 0 && visibleHistoryIds.value.every((id) => selectedHistorySet.value.has(id)),
)

const recordingSelectAllState = computed<boolean | 'indeterminate'>(() => {
  if (!visibleRecordings.value.length) return false
  if (allVisibleRecordingsSelected.value) return true
  return selectedRecordings.value.length > 0 ? 'indeterminate' : false
})

const visibleRecordings = computed(() => recordings.value)

const recordingsHasMore = computed(() => recordings.value.length < recordingTotal.value)
const selectedRecordingSet = computed(() => new Set(selectedRecordings.value))
const allVisibleRecordingsSelected = computed(
  () =>
    visibleRecordings.value.length > 0 &&
    visibleRecordings.value.every((item) => selectedRecordingSet.value.has(item.id)),
)
const selectedRecordingItems = computed(() =>
  visibleRecordings.value.filter((item) => selectedRecordingSet.value.has(item.id)),
)

const historyViews = computed(() => [
  { value: 'active', label: t('history.archive.active') },
  { value: 'archived', label: t('history.archive.viewArchived') },
])

function statusLabel(item: SessionHistoryRecord): string {
  const key = HISTORY_STATUS_KEYS[item.status]
  return key ? t(key) : item.status
}

function recordingStatusLabel(status: string): string {
  const key = RECORDING_STATUS_KEYS[status]
  return key ? t(key) : status
}

function recordingDeletable(status: string): boolean {
  return !['recording', 'paused'].includes(status)
}

function formatBytes(bytes?: number): string {
  const value = typeof bytes === 'number' && Number.isFinite(bytes) ? bytes : 0
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${(value / (1024 * 1024)).toFixed(1)} MB`
}

function hasTimestamp(value?: string): value is string {
  return Boolean(value) && !value!.startsWith('0001-01-01')
}

function formatTime(value?: string): string {
  return value ? new Date(value).toLocaleString() : ''
}

function formatTimeRange(startedAt: string, endedAt?: string): string {
  const start = formatTime(startedAt)
  return hasTimestamp(endedAt) ? `${start} - ${formatTime(endedAt)}` : start
}

async function load(): Promise<void> {
  loading.value = true
  try {
    items.value = await listSessionHistory()
    error.value = ''
  } catch (cause) {
    error.value = errorMessage(cause, t('history.load.failure'))
    items.value = []
  } finally {
    loading.value = false
  }
}

async function loadRecordings(page = recordingPage.value, append = false): Promise<void> {
  const generation = ++recordingLoadGeneration
  if (append) recordingsLoadingMore.value = true
  else recordingsLoading.value = true
  try {
    const result = await listRecordingPage(page, RECORDING_PAGE_SIZE, recordingQuery.value)
    if (generation !== recordingLoadGeneration) return
    const merged = append ? mergeRecordings(recordings.value, result.items) : result.items
    recordings.value = merged
    recordingTotal.value = result.total
    recordingPage.value = append ? Math.max(page, result.page) : result.page

    selectedRecordings.value = selectedRecordings.value.filter((id) => merged.some((item) => item.id === id))
    recordingsError.value = ''
  } catch (cause) {
    if (generation !== recordingLoadGeneration) return
    recordingsError.value = errorMessage(cause, t('history.recording.load.failure'))
    if (!append) {
      recordings.value = []
      recordingTotal.value = 0
    }
  } finally {
    if (generation === recordingLoadGeneration) {
      recordingsLoading.value = false
      recordingsLoadingMore.value = false
    }
  }
}

function mergeRecordings(current: TerminalRecording[], incoming: TerminalRecording[]): TerminalRecording[] {
  const seen = new Set(current.map((item) => item.id))
  return [...current, ...incoming.filter((item) => !seen.has(item.id))]
}

function loadMoreRecordings(): void {
  if (!recordingsHasMore.value || recordingsLoading.value || recordingsLoadingMore.value) return
  void loadRecordings(recordingPage.value + 1, true)
}

async function reloadLoadedRecordings(): Promise<void> {
  const pages = Math.max(1, Math.ceil(recordings.value.length / RECORDING_PAGE_SIZE))
  const generation = ++recordingLoadGeneration
  recordingsLoadingMore.value = true
  try {
    const results = await Promise.all(
      Array.from({ length: pages }, (_value, index) =>
        listRecordingPage(index + 1, RECORDING_PAGE_SIZE, recordingQuery.value),
      ),
    )
    if (generation !== recordingLoadGeneration) return
    const merged = results.reduce<TerminalRecording[]>(
      (accumulator, result) => mergeRecordings(accumulator, result.items),
      [],
    )
    recordings.value = merged
    recordingTotal.value = results[results.length - 1]?.total ?? merged.length
    recordingPage.value = pages
    selectedRecordings.value = selectedRecordings.value.filter((id) => merged.some((item) => item.id === id))
  } catch (cause) {
    if (generation !== recordingLoadGeneration) return
    recordingsError.value = errorMessage(cause, t('history.recording.load.failure'))
  } finally {
    if (generation === recordingLoadGeneration) recordingsLoadingMore.value = false
  }
}

async function loadRecordingIndex(): Promise<void> {
  const generation = ++recordingIndexGeneration
  try {
    const all = await listRecordings()
    if (generation !== recordingIndexGeneration) return
    const replay: Record<string, TerminalRecording> = {}
    const exportable: Record<string, TerminalRecording> = {}

    for (const item of all) {
      if (item.mode === 'full') {
        if (!replay[item.sessionId]) replay[item.sessionId] = item
        if (!exportable[item.sessionId]) exportable[item.sessionId] = item
        continue
      }

      if (!exportable[item.sessionId]) exportable[item.sessionId] = item
    }
    recordingIndex.value = replay
    recordingExportIndex.value = exportable
  } catch {
    if (generation === recordingIndexGeneration) {
      recordingIndex.value = {}
      recordingExportIndex.value = {}
    }
  }
}

function loadMoreHistory(): void {
  if (!historyHasMore.value) return
  historyVisibleCount.value += HISTORY_PAGE_SIZE
}

function resolveScrollViewport(area: { viewport?: unknown } | null): HTMLElement | null {
  const exposed = area?.viewport as { viewportElement?: HTMLElement | null } | HTMLElement | null | undefined
  if (!exposed) return null
  if (exposed instanceof HTMLElement) return exposed
  return exposed.viewportElement instanceof HTMLElement ? exposed.viewportElement : null
}

const SCROLL_LOAD_THRESHOLD = 64

function bindScrollLoad(area: { viewport?: unknown } | null, loadMore: () => void): (() => void) | undefined {
  const viewport = resolveScrollViewport(area)
  if (!viewport) return undefined
  const handler = (): void => {
    if (viewport.scrollTop + viewport.clientHeight >= viewport.scrollHeight - SCROLL_LOAD_THRESHOLD) loadMore()
  }
  viewport.addEventListener('scroll', handler, { passive: true })
  return () => viewport.removeEventListener('scroll', handler)
}

function bindScrollLoadOnRef(area: Ref<{ viewport?: unknown } | null>, loadMore: () => void): void {
  let unbind: (() => void) | undefined
  watch(
    area,
    (value) => {
      unbind?.()
      unbind = bindScrollLoad(value, loadMore)
    },
    { flush: 'post' },
  )
  onBeforeUnmount(() => {
    unbind?.()
    unbind = undefined
  })
}

function toggleRecordingSelection(id: string): void {
  selectedRecordings.value = selectedRecordings.value.includes(id)
    ? selectedRecordings.value.filter((selected) => selected !== id)
    : [...selectedRecordings.value, id]
}

function onRecordingSelectionKeydown(event: KeyboardEvent, id: string): void {
  if (event.key !== 'Enter' && event.key !== ' ') return
  event.preventDefault()
  toggleRecordingSelection(id)
}

function toggleHistorySelection(id: string): void {
  selectedHistory.value = selectedHistory.value.includes(id)
    ? selectedHistory.value.filter((selected) => selected !== id)
    : [...selectedHistory.value, id]
}

function onHistorySelectionKeydown(event: KeyboardEvent, id: string): void {
  if (event.key !== 'Enter' && event.key !== ' ') return
  event.preventDefault()
  toggleHistorySelection(id)
}

function toggleSelectAllHistory(): void {
  const ids = visibleHistoryIds.value
  if (!ids.length) return
  if (allVisibleHistorySelected.value) {
    const removed = new Set(ids)
    selectedHistory.value = selectedHistory.value.filter((id) => !removed.has(id))
    return
  }
  selectedHistory.value = [...new Set([...selectedHistory.value, ...ids])]
}

function clearHistorySelection(): void {
  selectedHistory.value = []
}

function toggleSelectAllRecordings(): void {
  const ids = visibleRecordings.value.map((item) => item.id)
  if (!ids.length) return
  if (allVisibleRecordingsSelected.value) {
    const removed = new Set(ids)
    selectedRecordings.value = selectedRecordings.value.filter((id) => !removed.has(id))
    return
  }
  selectedRecordings.value = [...new Set([...selectedRecordings.value, ...ids])]
}

function clearRecordingSelection(): void {
  selectedRecordings.value = []
}

async function exportSelectedHistory(): Promise<void> {
  const ids = selectedHistory.value
  if (!ids.length) return
  const recordingIds: string[] = []
  let withoutRecordings = 0
  for (const id of ids) {
    const item = recordingExportIndex.value[id]
    if (item) recordingIds.push(item.id)
    else withoutRecordings += 1
  }
  if (!recordingIds.length) {
    toast.warning(t('history.batch.export.noRecordings', { count: ids.length }))
    return
  }
  try {
    const saved = await exportRecordingsText(recordingIds, 'qterm-session-history.txt')
    if (!saved) return
    const parts = [t('history.batch.export.success', { count: recordingIds.length })]
    if (withoutRecordings > 0) parts.push(t('history.batch.export.skipped', { count: withoutRecordings }))
    if (withoutRecordings > 0) toast.warning(parts.join(' '))
    else toast.success(parts[0]!)
  } catch (cause) {
    toast.error(errorMessage(cause, t('history.batch.export.failure')))
  }
}

async function archiveSelectedHistory(): Promise<void> {
  const ids = selectedHistory.value
  if (!ids.length || busy.value) return
  const archived = historyView.value !== 'archived'
  busy.value = true
  try {
    const result = await archiveSessionHistories(ids, archived)
    selectedHistory.value = []
    await load()
    const parts = [
      t(archived ? 'history.batch.archive.success' : 'history.batch.unarchive.success', { count: result.updated }),
    ]
    if (result.missing > 0) parts.push(t('history.batch.missing', { count: result.missing }))
    if (result.missing > 0) toast.warning(parts.join(' '))
    else toast.success(parts[0]!)
  } catch (cause) {
    toast.error(errorMessage(cause, t('history.batch.archive.failure')))
  } finally {
    busy.value = false
  }
}

async function confirmRemoveRecordings(): Promise<void> {
  if (recordingBatchDeleting.value) return
  const ids = [...selectedRecordings.value]
  if (!ids.length) {
    recordingBatchDeleteOpen.value = false
    return
  }
  recordingBatchDeleting.value = true
  try {
    const result = await deleteRecordings(ids)
    recordingBatchDeleteOpen.value = false
    selectedRecordings.value = []
    for (const id of ids) {
      if (result.deleted > 0 && replayOpen.value && replayRecordingId.value === id) {
        closeReplayDialog(false)
        break
      }
    }
    await reloadLoadedRecordings()
    const parts = [t('history.recording.batch.delete.done', { count: result.deleted })]
    if (result.skipped > 0) parts.push(t('history.recording.batch.delete.skipped', { count: result.skipped }))
    if (result.missing > 0) parts.push(t('history.recording.batch.delete.missing', { count: result.missing }))
    if (result.failed > 0) parts.push(t('history.recording.batch.delete.failed', { count: result.failed }))
    if (result.skipped > 0 || result.failed > 0 || result.missing > 0) toast.warning(parts.join(' '))
    else toast.success(parts[0]!)
  } catch (cause) {
    toast.error(errorMessage(cause, t('history.recording.batch.delete.failure')))
  } finally {
    recordingBatchDeleting.value = false
  }
}

function exportSelectedRecordings(): void {
  openExportDialog(selectedRecordingItems.value)
}

function openNoteDialog(item: SessionHistoryRecord): void {
  pendingNote.value = item
  noteDraft.value = item.note ?? ''
}

function closeNoteDialog(open: boolean): void {
  if (!open && !noteSaving.value) {
    pendingNote.value = null
    pendingRecordingNote.value = null
    noteDraft.value = ''
  }
}

async function confirmNote(): Promise<void> {
  const item = pendingNote.value
  if (!item || noteSaving.value) return
  noteSaving.value = true
  try {
    await updateSessionHistoryNote(item.id, noteDraft.value)
    pendingNote.value = null
    noteDraft.value = ''
    toast.success(t('history.note.saved'))
    await load()
  } catch (cause) {
    toast.error(errorMessage(cause, t('history.note.saveFailure')))
  } finally {
    noteSaving.value = false
  }
}

function askRemove(item: SessionHistoryRecord): void {
  pendingDelete.value = item
}

async function confirmRemove(): Promise<void> {
  const item = pendingDelete.value
  if (!item || deleting.value) return
  deleting.value = true
  try {
    await deleteSessionHistory(item.id)
    pendingDelete.value = null
    await load()
  } catch (cause) {
    toast.error(errorMessage(cause, t('history.delete.failure')))
  } finally {
    deleting.value = false
  }
}

const unarchivedCount = computed(() => items.value.filter((item) => item.archived !== true).length)

function askClearUnarchived(): void {
  if (clearingUnarchived.value || unarchivedCount.value === 0) return
  clearUnarchivedOpen.value = true
}

async function confirmClearUnarchived(): Promise<void> {
  if (clearingUnarchived.value) return
  clearingUnarchived.value = true
  try {
    const result = await clearUnarchivedSessionHistory()
    clearUnarchivedOpen.value = false
    await Promise.all([load(), loadRecordings(), loadRecordingIndex()])

    const parts = [t('history.clear.done', { count: result.sessions, recordings: result.recordings })]
    if (result.skipped > 0) parts.push(t('history.clear.skipped', { count: result.skipped }))
    if (result.failed > 0) parts.push(t('history.clear.failed', { count: result.failed }))
    if (result.skipped > 0 || result.failed > 0) toast.warning(parts.join(' '))
    else toast.success(parts[0]!)
  } catch (cause) {
    toast.error(errorMessage(cause, t('history.clear.failure')))
  } finally {
    clearingUnarchived.value = false
  }
}

async function toggleArchive(item: SessionHistoryRecord): Promise<void> {
  if (busy.value) return
  busy.value = true
  try {
    const result = await archiveSessionHistories([item.id], item.archived !== true)
    if (result.missing > 0) {
      await load()
      return
    }
    toast.success(item.archived === true ? t('history.archive.restored') : t('history.archive.archived'))
    await load()
  } catch (cause) {
    toast.error(
      errorMessage(cause, item.archived === true ? t('history.archive.restoreFailure') : t('history.archive.failure')),
    )
  } finally {
    busy.value = false
  }
}

function sessionType(item: SessionHistoryRecord): string {
  return item.hostId ? t('history.type.ssh') : t('history.type.local')
}

function statusTone(item: SessionHistoryRecord): string {
  if (item.status === 'connected') return 'status-dot--connected'
  if (item.status === 'failed') return 'status-dot--error'

  if (item.status === 'interrupted') return 'status-dot--warning'
  return 'status-dot--idle'
}

function openExportDialog(items: TerminalRecording[]): void {
  if (!items.length) return
  exportItems.value = items
  exportFormat.value = 'txt'
  exportIncludeOutput.value = true
  exportOpen.value = true
}

async function confirmExport(): Promise<void> {
  if (exporting.value || !exportItems.value.length) return
  exporting.value = true

  const includeOutput = exportFormat.value === 'cast' ? true : exportIncludeOutput.value
  try {
    const saved = await exportRecordings(
      exportItems.value.map((item) => item.id),
      exportFormat.value,
      includeOutput,
    )
    if (saved) {
      exportOpen.value = false
      toast.success(t('history.recording.export.success'))
    }
  } catch (cause) {
    toast.error(errorMessage(cause, t('history.recording.export.failure')))
  } finally {
    exporting.value = false
  }
}

function askRemoveRecording(item: TerminalRecording): void {
  pendingRecordingDelete.value = item
}

async function confirmRemoveRecording(): Promise<void> {
  const item = pendingRecordingDelete.value
  if (!item || recordingDeleting.value) return
  recordingDeleting.value = true
  try {
    await deleteRecording(item.id)
    pendingRecordingDelete.value = null
    await reloadLoadedRecordings()
    if (replayOpen.value && replayRecordingId.value === item.id) closeReplayDialog(false)
  } catch (cause) {
    toast.error(errorMessage(cause, t('history.recording.deleteFailure')))
  } finally {
    recordingDeleting.value = false
  }
}

function recordingHostLabel(item: TerminalRecording): string {
  if (item.hostName) return item.address ? `${item.hostName} · ${item.address}` : item.hostName
  const history = items.value.find((record) => record.hostId === item.hostId)
  if (history?.hostName) return `${history.hostName} · ${history.address}`
  return item.hostId ? `${t('history.recording.unknownHost')} · ${item.hostId}` : t('history.recording.unknownHost')
}

function recordingNote(item: TerminalRecording): string {
  return item.note ?? ''
}

function openRecordingNote(item: TerminalRecording): void {
  pendingRecordingNote.value = item
  noteDraft.value = item.note ?? ''
}

async function confirmRecordingNote(): Promise<void> {
  const item = pendingRecordingNote.value
  if (!item || noteSaving.value) return
  noteSaving.value = true
  try {
    await updateRecordingNote(item.id, noteDraft.value)
    item.note = noteDraft.value.trim()
    pendingRecordingNote.value = null
    noteDraft.value = ''
    toast.success(t('history.note.saved'))
  } catch (cause) {
    toast.error(errorMessage(cause, t('history.note.saveFailure')))
  } finally {
    noteSaving.value = false
  }
}

const historyScroll = ref<{ viewport?: unknown } | null>(null)
const recordingsScroll = ref<{ viewport?: unknown } | null>(null)
bindScrollLoadOnRef(historyScroll, loadMoreHistory)
bindScrollLoadOnRef(recordingsScroll, loadMoreRecordings)

onMounted(() => {
  void load()
  void loadRecordings()
})

watch(replayMode, (mode, previous) => {
  if (mode === previous || !replayOpen.value || !replayRecordingId.value) return
  if (replayRunning.value) stopReplay()
  void replayRecording(replayRecordingId.value)
})

watch(
  () => props.view,
  (value) => {
    activeTab.value = value ?? 'history'
  },
)

watch(
  activeTab,
  (tab) => {
    if (tab === 'history') void loadRecordingIndex()
  },
  { immediate: true },
)

watch(recordingQuery, () => {
  if (recordingSearchTimer !== undefined) window.clearTimeout(recordingSearchTimer)
  recordingSearchTimer = window.setTimeout(() => {
    recordingSearchTimer = undefined
    recordingPage.value = 1
    selectedRecordings.value = []
    void loadRecordings(1)
  }, 200)
})

watch([query, historyView, items], () => {
  historyVisibleCount.value = HISTORY_PAGE_SIZE
})

watch(visibleItems, (list) => {
  const allowed = new Set(list.map((item) => item.id))
  const next = selectedHistory.value.filter((id) => allowed.has(id))
  if (next.length !== selectedHistory.value.length) selectedHistory.value = next
})

onBeforeUnmount(() => {
  recordingLoadGeneration += 1
  recordingIndexGeneration += 1
  if (recordingSearchTimer !== undefined) window.clearTimeout(recordingSearchTimer)
})
</script>

<template>
  <div class="settings-history-panel is-inner-scroll grid gap-4">
    <SegmentedTabs
      v-if="!props.view"
      v-model="activeTab"
      :options="[
        { value: 'history', label: t('settings.category.history') },
        { value: 'recordings', label: t('settings.category.recordings') },
      ]"
      :label="t('history.tabs.label')"
    />

    <div
      v-if="activeTab === 'history'"
      id="history-panel-history"
      class="settings-history-head-area flex flex-col gap-2"
      role="tabpanel"
    >
      <div class="settings-history-toolbar flex flex-wrap gap-2 items-center">
        <SegmentedTabs v-model="historyView" :options="historyViews" :label="t('history.archive.active')" />

        <label class="relative ml-auto flex min-w-40 flex-1 items-center text-qt-muted">
          <Search class="pointer-events-none absolute left-2.5 z-1" :size="16" aria-hidden="true" />
          <QtermInput
            v-model="query"
            class="qterm-reference-input w-full pl-8"
            :placeholder="t('history.search.placeholder')"
            :aria-label="t('history.search.aria')"
          />
        </label>
      </div>

      <div
        v-if="historyView === 'active' || visibleHistoryIds.length || selectedHistoryCount"
        class="settings-batch-bar flex flex-none items-center justify-between gap-2 min-h-8"
      >
        <label v-if="visibleHistoryIds.length" class="flex c-p items-center gap-2 text-xs text-qt-text">
          <QtermCheckbox
            :model-value="historySelectAllState"
            :aria-label="t('history.batch.selectAll')"
            @update:model-value="toggleSelectAllHistory()"
          >
            <Minus v-if="historySelectAllState === 'indeterminate'" :size="14" aria-hidden="true" />
            <Check v-else :size="14" aria-hidden="true" />
          </QtermCheckbox>
          <span>{{ t('history.batch.selectAll') }}</span>
        </label>
        <div
          class="settings-batch-actions settings-history-batch-actions inline-flex flex-wrap items-center gap-1.5 ml-auto"
        >
          <span v-if="selectedHistoryCount" class="text-xs text-qt-muted">
            {{ t('history.batch.selected', { count: selectedHistoryCount }) }}
          </span>
          <QtermButton v-if="selectedHistoryCount" size="sm" variant="outline" @click="exportSelectedHistory()">
            {{ t('history.batch.export') }}
          </QtermButton>
          <QtermButton
            v-if="selectedHistoryCount"
            size="sm"
            variant="outline"
            :disabled="busy"
            @click="archiveSelectedHistory()"
          >
            {{ historyView === 'archived' ? t('history.batch.unarchive') : t('history.batch.archive') }}
          </QtermButton>

          <QtermButton
            v-if="historyView === 'active'"
            size="sm"
            variant="destructive"
            :disabled="busy || clearingUnarchived || unarchivedCount === 0"
            @click="askClearUnarchived()"
          >
            {{ t('history.clear.action') }}
          </QtermButton>
          <IconButton
            v-if="selectedHistoryCount"
            :label="t('history.batch.exit')"
            size="icon-sm"
            @click="clearHistorySelection()"
          >
            <X :size="15" aria-hidden="true" />
          </IconButton>
        </div>
      </div>
    </div>

    <p v-if="activeTab === 'history' && loading" class="panel-empty-note text-qt-muted text-xs">
      {{ t('history.loading') }}
    </p>
    <p v-else-if="activeTab === 'history' && error" class="panel-empty-note text-qt-muted text-xs">{{ error }}</p>

    <p
      v-else-if="activeTab === 'history' && items.length && !visibleItems.length"
      class="panel-empty-note text-qt-muted text-xs"
    >
      {{ t('history.empty.filtered') }}
    </p>

    <ScrollArea
      v-if="activeTab === 'history' && historyPageItems.length"
      ref="historyScroll"
      type="hover"
      :scroll-hide-delay="600"
      class="settings-history-list-area"
    >
      <div class="settings-history-list grid gap-3 [align-content:start] pt-px pr-1.5 pb-px pl-px">
        <article
          v-for="item in historyPageItems"
          :key="item.id"
          class="settings-history-item is-selectable grid gap-3 p-3 border border-solid border-qt-border rounded-xl"
          :class="{ 'is-selected': selectedHistorySet.has(item.id) }"
          role="checkbox"
          :aria-checked="selectedHistorySet.has(item.id)"
          :aria-label="t('history.batch.selectOne', { host: item.hostName })"
          tabindex="0"
          @click="toggleHistorySelection(item.id)"
          @keydown="onHistorySelectionKeydown($event, item.id)"
        >
          <div class="settings-history-head flex min-w-0 flex-wrap items-center gap-2">
            <QtermCheckbox
              :model-value="selectedHistorySet.has(item.id)"
              :aria-label="t('history.batch.selectOne', { host: item.hostName })"
              @click.stop
              @update:model-value="toggleHistorySelection(item.id)"
            />
            <span class="settings-history-type">{{ sessionType(item) }}</span>
            <strong class="text-sm">{{ item.hostName }}</strong>
            <span
              class="settings-history-address min-w-0 overflow-hidden text-qt-muted text-ellipsis whitespace-nowrap"
              >{{ item.address }}</span
            >
            <span class="settings-history-status inline-flex flex-none ml-auto items-center gap-1.25 text-qt-muted">
              <span class="status-dot" :class="statusTone(item)" aria-hidden="true"></span>{{ statusLabel(item) }}
            </span>
          </div>
          <div class="settings-history-dates flex min-w-0 flex-wrap items-center gap-[0.25rem_0.75rem] text-qt-muted">
            <span>{{ formatTimeRange(item.startedAt, item.endedAt) }}</span>
            <span
              class="settings-history-actions inline-flex flex-none ml-auto items-center gap-0.5"
              @click.stop
              @keydown.stop
            >
              <IconButton
                v-if="recordingIndex[item.id]"
                :label="t('history.action.replay')"
                size="icon-sm"
                :disabled="replayRunning"
                @click="replayRecording(recordingIndex[item.id]?.id ?? '')"
              >
                <Play :size="15" aria-hidden="true" />
              </IconButton>

              <IconButton
                v-if="recordingExportIndex[item.id]"
                :label="t('history.action.export')"
                size="icon-sm"
                @click="openExportDialog([recordingExportIndex[item.id]!])"
              >
                <Download :size="15" aria-hidden="true" />
              </IconButton>
              <IconButton
                :label="t('history.action.editNote')"
                size="icon-sm"
                :disabled="busy"
                @click="openNoteDialog(item)"
              >
                <Edit3 :size="15" aria-hidden="true" />
              </IconButton>
              <IconButton
                :label="item.archived === true ? t('history.action.restore') : t('history.action.archive')"
                size="icon-sm"
                :disabled="busy"
                @click="toggleArchive(item)"
              >
                <component :is="item.archived === true ? ArchiveRestore : Archive" :size="15" aria-hidden="true" />
              </IconButton>
              <IconButton
                :label="t('history.action.delete')"
                size="icon-sm"
                :disabled="busy"
                danger
                @click="askRemove(item)"
              >
                <Trash2 :size="15" aria-hidden="true" />
              </IconButton>
            </span>
          </div>
          <div v-if="item.note" class="settings-history-note w-full pt-2 text-qt-muted leading-normal wrap-anywhere">
            {{ item.note }}
          </div>
        </article>
      </div>
    </ScrollArea>

    <div
      v-if="activeTab === 'history' && historyPageItems.length"
      class="settings-history-pagination flex flex-none items-center justify-end gap-[0.5rem] text-qt-muted"
    >
      <span class="text-xs tabular-nums" aria-live="polite"
        >{{ historyPageItems.length }}/{{ visibleItems.length }}</span
      >
      <QtermButton v-if="historyHasMore" class="sr-only" size="sm" variant="outline" @click="loadMoreHistory()">
        {{ t('history.pagination.loadMore') }}
      </QtermButton>
    </div>

    <div
      v-if="activeTab === 'recordings'"
      id="history-panel-recordings"
      class="settings-history-recordings min-h-0 flex flex-[1_1_0] flex-col gap-2"
      role="tabpanel"
    >
      <div class="settings-recording-toolbar flex flex-none items-center gap-2">
        <label class="relative flex min-w-0 flex-1 items-center text-qt-muted">
          <Search class="pointer-events-none absolute left-2.5 z-1" :size="16" aria-hidden="true" />
          <QtermInput
            v-model="recordingQuery"
            class="qterm-reference-input w-full pl-8"
            :placeholder="t('history.recording.search.placeholder')"
            :aria-label="t('history.recording.search.aria')"
          />
        </label>
      </div>

      <div
        v-if="visibleRecordings.length || selectedRecordings.length"
        class="settings-batch-bar flex flex-none items-center justify-between gap-2 min-h-8"
      >
        <label v-if="visibleRecordings.length" class="flex c-p items-center gap-2 text-xs text-qt-text">
          <QtermCheckbox
            :model-value="recordingSelectAllState"
            :aria-label="t('history.recording.batch.selectAll')"
            @update:model-value="toggleSelectAllRecordings()"
          >
            <Minus v-if="recordingSelectAllState === 'indeterminate'" :size="14" aria-hidden="true" />
            <Check v-else :size="14" aria-hidden="true" />
          </QtermCheckbox>
          <span>{{ t('history.recording.batch.selectAll') }}</span>
        </label>
        <div
          v-if="selectedRecordings.length"
          class="settings-batch-actions settings-recording-batch-actions inline-flex flex-wrap items-center gap-1.5 ml-auto"
        >
          <span v-if="selectedRecordings.length" class="text-xs text-qt-muted">
            {{ t('history.recording.batch.selected', { count: selectedRecordings.length }) }}
          </span>
          <QtermButton v-if="selectedRecordings.length" size="sm" variant="outline" @click="exportSelectedRecordings()">
            {{ t('history.recording.batch.export') }}
          </QtermButton>
          <QtermButton
            v-if="selectedRecordings.length"
            size="sm"
            variant="destructive"
            :disabled="recordingBatchDeleting"
            @click="recordingBatchDeleteOpen = true"
          >
            {{ t('history.recording.batch.remove') }}
          </QtermButton>
          <IconButton
            v-if="selectedRecordings.length"
            :label="t('history.recording.batch.exit')"
            size="icon-sm"
            @click="clearRecordingSelection()"
          >
            <X :size="15" aria-hidden="true" />
          </IconButton>
        </div>
      </div>
      <p v-if="recordingsLoading" class="panel-empty-note text-qt-muted text-xs">
        {{ t('history.recording.loading') }}
      </p>
      <p v-else-if="recordingsError" class="panel-empty-note text-qt-muted text-xs">{{ recordingsError }}</p>
      <p v-else-if="recordingQuery.trim() && !visibleRecordings.length" class="panel-empty-note text-qt-muted text-xs">
        {{ t('history.recording.empty.filtered') }}
      </p>
      <ScrollArea
        v-else
        ref="recordingsScroll"
        type="hover"
        :scroll-hide-delay="600"
        class="settings-history-recording-area"
        aria-live="polite"
      >
        <div class="settings-history-recording-list grid content-start gap-3 px-1.5 py-px pl-px">
          <article
            v-for="item in visibleRecordings"
            :key="item.id"
            class="settings-history-item settings-history-recording-item is-selectable grid gap-3 p-3 border border-solid border-qt-border rounded-[0.5rem] m-0"
            :class="{ 'is-selected': selectedRecordingSet.has(item.id) }"
            role="checkbox"
            :aria-checked="selectedRecordingSet.has(item.id)"
            :aria-label="t('history.recording.select', { host: recordingHostLabel(item) })"
            tabindex="0"
            @click="toggleRecordingSelection(item.id)"
            @keydown="onRecordingSelectionKeydown($event, item.id)"
          >
            <div class="settings-history-head flex min-w-0 flex-wrap items-center gap-2">
              <QtermCheckbox
                :model-value="selectedRecordingSet.has(item.id)"
                :aria-label="t('history.recording.batch.selectOne', { host: recordingHostLabel(item) })"
                @click.stop
                @update:model-value="toggleRecordingSelection(item.id)"
              />
              <strong class="text-sm">{{ recordingHostLabel(item) }}</strong>
              <span class="settings-history-status inline-flex flex-none ml-auto items-center gap-1.25 text-qt-muted">{{
                recordingStatusLabel(item.status)
              }}</span>
            </div>
            <div class="settings-history-dates flex min-w-0 flex-wrap items-center gap-[0.25rem_0.75rem] text-qt-muted">
              <span
                >{{ formatTimeRange(item.startedAt, item.endedAt) }} · {{ formatBytes(item.bytes) }} ·
                {{ t('history.recording.events', { count: item.eventCount }) }}</span
              >
              <span
                class="settings-history-actions inline-flex flex-none ml-auto items-center gap-0.5"
                @click.stop
                @keydown.stop
              >
                <IconButton
                  :label="t('history.action.editNote')"
                  size="icon-sm"
                  :disabled="busy"
                  @click="openRecordingNote(item)"
                >
                  <Edit3 :size="15" aria-hidden="true" />
                </IconButton>
                <IconButton
                  :label="t('history.recording.action.replay')"
                  size="icon-sm"
                  :disabled="replayRunning"
                  @click="replayRecording(item.id)"
                >
                  <Play :size="15" aria-hidden="true" />
                </IconButton>
                <IconButton
                  :label="t('history.recording.action.export')"
                  size="icon-sm"
                  @click="openExportDialog([item])"
                  ><Download :size="15" aria-hidden="true"
                /></IconButton>
                <IconButton
                  size="icon-sm"
                  :label="
                    recordingDeletable(item.status)
                      ? t('history.recording.action.delete')
                      : t('history.recording.action.stopFirst')
                  "
                  :disabled="!recordingDeletable(item.status)"
                  danger
                  @click="askRemoveRecording(item)"
                >
                  <Trash2 :size="15" aria-hidden="true" />
                </IconButton>
              </span>
            </div>
            <div
              v-if="recordingNote(item)"
              class="settings-history-note w-full pt-2 text-qt-muted leading-normal wrap-anywhere"
            >
              {{ recordingNote(item) }}
            </div>
          </article>
        </div>
      </ScrollArea>

      <div
        v-if="visibleRecordings.length"
        class="settings-recording-pagination flex flex-none items-center justify-end gap-2 text-qt-muted"
      >
        <span class="text-xs tabular-nums" aria-live="polite">{{ visibleRecordings.length }}/{{ recordingTotal }}</span>
        <QtermButton
          v-if="recordingsHasMore"
          class="sr-only"
          size="sm"
          variant="outline"
          :disabled="recordingsLoadingMore"
          @click="loadMoreRecordings()"
        >
          {{ t('history.recording.pagination.loadMore') }}
        </QtermButton>
      </div>
    </div>

    <QtermDialog :open="pendingNote !== null || pendingRecordingNote !== null" @update:open="closeNoteDialog">
      <QtermDialogContent v-if="pendingNote || pendingRecordingNote" size="sm">
        <QtermDialogHeader
          :title="t('history.recording.note.title')"
          :description="pendingNote?.hostName ?? pendingRecordingNote?.hostName ?? t('settings.category.recordings')"
          :show-close="true"
          @close="closeNoteDialog(false)"
        />
        <div class="dialog-body">
          <QtermTextarea
            v-model="noteDraft"
            rows="4"
            :placeholder="t('history.recording.note.placeholder')"
            :aria-label="t('history.recording.note.aria')"
            autofocus
          />
        </div>
        <QtermDialogFooter>
          <QtermButton variant="ghost" :disabled="noteSaving" @click="closeNoteDialog(false)">{{
            t('history.dialog.cancel')
          }}</QtermButton>
          <QtermButton :disabled="noteSaving" @click="pendingRecordingNote ? confirmRecordingNote() : confirmNote()">{{
            t('history.dialog.save')
          }}</QtermButton>
        </QtermDialogFooter>
      </QtermDialogContent>
    </QtermDialog>

    <DeleteConfirmDialog
      :open="clearUnarchivedOpen"
      :title="t('history.clear.title')"
      :message="t('history.clear.message', { count: unarchivedCount })"
      :hint="t('history.clear.hint')"
      :confirm-text="t('history.clear.confirm')"
      :loading="clearingUnarchived"
      @close="clearUnarchivedOpen = false"
      @confirm="confirmClearUnarchived"
    />

    <DeleteConfirmDialog
      :open="pendingDelete !== null"
      :title="t('history.delete.title')"
      :message="pendingDelete ? t('history.delete.message', { host: pendingDelete.hostName }) : ''"
      :hint="t('history.delete.hint')"
      :loading="deleting"
      @close="pendingDelete = null"
      @confirm="confirmRemove"
    />

    <DeleteConfirmDialog
      :open="pendingRecordingDelete !== null"
      :title="t('history.recording.delete.title')"
      :message="
        pendingRecordingDelete
          ? t('history.recording.delete.message', { host: recordingHostLabel(pendingRecordingDelete) })
          : ''
      "
      :hint="t('history.recording.delete.hint')"
      :loading="recordingDeleting"
      @close="pendingRecordingDelete = null"
      @confirm="confirmRemoveRecording"
    />

    <DeleteConfirmDialog
      :open="recordingBatchDeleteOpen"
      :title="t('history.recording.batch.delete.title')"
      :message="t('history.recording.batch.delete.message', { count: selectedRecordings.length })"
      :hint="t('history.recording.batch.delete.hint')"
      :loading="recordingBatchDeleting"
      @close="recordingBatchDeleteOpen = false"
      @confirm="confirmRemoveRecordings"
    />

    <QtermDialog :open="replayOpen" @update:open="closeReplayDialog">
      <QtermDialogContent
        v-if="replayOpen"
        size="3xl"
        class="history-replay-dialog"
        :show-close-button="true"
        @open-auto-focus.prevent
      >
        <QtermDialogHeader
          :title="t('history.replay.title')"
          :description="t('history.replay.description')"
          :show-close="false"
        />
        <div
          ref="replayFrame"
          class="history-replay-frame flex min-h-0 flex-[1_1_auto] flex-col overflow-hidden px-4 pb-4"
          :class="{ 'is-expanded': replayFullscreen }"
        >
          <AsciinemaReplay
            v-if="replayMode === 'dynamic' && replayEvents.length"
            ref="dynamicReplayPlayer"
            :events="replayEvents"
            :columns="replayColumns"
            :rows="replayRows"
            :dark="props.dark"
            :light-theme="props.terminalLightTheme"
            :dark-theme="props.terminalDarkTheme"
            :speed="Number(replaySpeed)"
            :autoplay="false"
            @ready="onDynamicReplayReady"
            @timeupdate="onDynamicReplayTimeupdate"
            @play="onDynamicReplayPlay"
            @pause="onDynamicReplayPause"
            @ended="onDynamicReplayEnded"
            @error="onDynamicReplayError"
          />
          <ReplayTerminal
            v-else-if="replayMode === 'static' && replayEvents.length"
            :text="replayText"
            :columns="replayColumns"
            :rows="replayRows"
            :dark="props.dark"
            :light-theme="props.terminalLightTheme"
            :dark-theme="props.terminalDarkTheme"
          />

          <div
            v-else
            class="history-replay-terminal history-replay-empty flex-cc px-4 text-qt-muted text-xs"
            role="status"
          >
            {{ replayLoading ? t('history.replay.loading') : t('history.replay.empty') }}
          </div>
          <div class="history-replay-player" :aria-label="t('history.replay.player')">
            <div class="history-replay-progress flex min-w-0 items-center" :aria-label="t('history.replay.progress')">
              <input
                class="history-replay-progress-range"
                type="range"
                min="0"
                max="100"
                step="0.1"
                :value="replayProgress"
                :style="{ '--replay-ratio': (replayProgress / 100).toFixed(4) }"
                :disabled="replayLoading || replayMode === 'static' || replayDurationMs <= 0"
                :aria-label="t('history.replay.progress')"
                @input="seekReplay"
              />
            </div>
            <div class="history-replay-player-controls flex min-w-0 items-center gap-2">
              <div class="history-replay-toolbar-actions inline-flex items-center gap-1">
                <IconButton
                  :label="!replayRunning || replaySuspended ? t('history.replay.resume') : t('history.replay.pause')"
                  :disabled="replayLoading || !replayEvents.length || replayMode === 'static'"
                  @click="toggleReplayRunning"
                >
                  <component :is="!replayRunning || replaySuspended ? Play : Pause" :size="15" aria-hidden="true" />
                </IconButton>
                <IconButton
                  :label="t('history.replay.stop')"
                  :disabled="replayLoading || replayMode === 'static'"
                  @click="stopReplay"
                >
                  <Square :size="15" aria-hidden="true" />
                </IconButton>
              </div>
              <span class="history-replay-time text-qt-muted tabular-nums whitespace-nowrap text-xs">
                {{ formatPlaybackTime(replayElapsedMs) }} / {{ formatPlaybackTime(replayDurationMs) }}
              </span>
              <span class="history-replay-player-spacer flex-[1_1_auto] min-w-2" />
              <span class="history-replay-speed inline-flex items-center gap-2">
                <QtermSelect v-model="replaySpeed" :disabled="replayMode === 'static'">
                  <QtermSelectTrigger size="sm" :aria-label="t('history.replay.speed')"
                    ><SelectValue
                  /></QtermSelectTrigger>
                  <SelectContent position="popper" side="top">
                    <QtermSelectItem v-for="option in SPEED_OPTIONS" :key="option.value" :value="option.value">{{
                      option.label
                    }}</QtermSelectItem>
                  </SelectContent>
                </QtermSelect>
              </span>
              <SegmentedTabs
                v-model="replayMode"
                :options="[
                  // 静态回放是默认档，因此排在第一位，和打开时的实际状态一致。
                  { value: 'static', label: t('history.replay.static') },
                  { value: 'dynamic', label: t('history.replay.dynamic') },
                ]"
                :label="t('history.replay.mode')"
              />
              <QtermButton
                size="sm"
                variant="outline"
                @click="openExportDialog(recordings.filter((item) => item.id === replayRecordingId))"
                >{{ t('history.replay.export') }}</QtermButton
              >
              <IconButton
                :label="t(replayFullscreen ? 'history.replay.exitExpanded' : 'history.replay.expand')"
                @click="toggleFullscreen"
              >
                <component :is="replayFullscreen ? Minimize : Maximize" :size="16" />
              </IconButton>
            </div>
          </div>
        </div>
      </QtermDialogContent>
    </QtermDialog>

    <QtermDialog :open="exportOpen" @update:open="(open) => !open && !exporting && (exportOpen = false)">
      <QtermDialogContent size="sm" class="history-export-dialog z-70" overlay-class="history-export-overlay z-70">
        <QtermDialogHeader
          :title="t('history.export.title')"
          :description="t('history.export.description')"
          :show-close="!exporting"
          @close="!exporting && (exportOpen = false)"
        />
        <div class="dialog-body">
          <SegmentedTabs
            v-model="exportFormat"
            :options="[
              { value: 'txt', label: 'TXT' },
              { value: 'cast', label: 'ASCIINEMA' },
            ]"
            :label="t('history.export.format')"
          />

          <label v-if="exportFormat === 'txt'" class="settings-toggle-row mt-3">
            <span>
              <strong class="text-sm">{{ t('history.export.includeOutput') }}</strong>
              <small class="text-xs">{{ t('history.export.includeOutputHint') }}</small>
            </span>
            <QtermSwitch v-model="exportIncludeOutput" :aria-label="t('history.export.includeOutput')" />
          </label>
        </div>
        <QtermDialogFooter>
          <QtermButton variant="outline" type="button" :disabled="exporting" @click="exportOpen = false">{{
            t('history.dialog.cancel')
          }}</QtermButton>
          <QtermButton type="button" :disabled="exporting" @click="confirmExport">{{
            t('history.dialog.export')
          }}</QtermButton>
        </QtermDialogFooter>
      </QtermDialogContent>
    </QtermDialog>
  </div>
</template>
