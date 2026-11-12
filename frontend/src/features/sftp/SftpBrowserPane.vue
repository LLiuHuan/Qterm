<script setup lang="ts">
import { QtermCheckbox } from '../../components/common'
import { QtermButton, QtermEmptyState, QtermInput } from '../../components/common'

import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  ChevronRight,
  CircleSlash,
  Copy,
  Download,
  ExternalLink,
  Eye,
  EyeOff,
  FilePen,
  FilePlus,
  FileQuestion,
  FolderOpen,
  FolderPlus,
  FolderUp,
  FileArchive,
  Image as ImageIcon,
  Info,
  MoreHorizontal,
  Pencil,
  RefreshCw,
  RotateCcw,
  Search,
  Shield,
  Trash2,
  Upload,
  X,
  Sparkles,
} from '@lucide/vue'
import { Popover, PopoverContent, PopoverTrigger } from '../../components/common/overlay-controls'
import { Tooltip, TooltipContent, TooltipTrigger } from '../../components/common/control-primitives'
import { ScrollArea } from '../../components/common/control-primitives'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from '../../components/common/overlay-controls'
import { writeClipboardText } from '../../services/clipboard'
import { requestAILaunch } from '../../services/aiAgentLaunch'
import { translateErrorKey } from '../../services/errors'
import { toast } from 'vue-sonner'
import SftpFileIcon from './SftpFileIcon.vue'
import SftpFavoritesMenu from './SftpFavoritesMenu.vue'
import {
  loadSFTPFavorites,
  resolveFavoritesHostId,
  saveSFTPFavorites,
  type SFTPFavorite,
} from '../../services/sftpFavorites'
import ImagePreviewDialog from './ImagePreviewDialog.vue'
import { isPreviewableImageName } from './previewableImage'
import { useI18n } from '@/locales'
import { useSftpBrowserList } from './useSftpBrowserList'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { editStateLocaleKey, editStateTone, type RemoteEditView } from '../remoteedit/remoteEditModel'
import {
  openWithLocalProgram,
  refreshRemoteEdits,
  reloadExternalEdit,
  requestRemoteTextEditor,
  stopRemoteEdit,
  syncRemoteEdit,
  useRemoteEdits,
} from '../remoteedit/useRemoteEdits'

export interface SftpBrowserSegment {
  label: string

  path: string

  title?: string

  ellipsis?: boolean
}

export interface SftpBrowserEntry {
  key: string
  name: string
  type: 'file' | 'directory' | 'symlink' | 'other'

  symlink?: boolean

  mode?: string
  uid?: number
  gid?: number

  modTime: string

  size: number
  sizeText: string
}

const props = withDefaults(
  defineProps<{
    segments: SftpBrowserSegment[]
    entries: SftpBrowserEntry[]
    loading: boolean
    error: string

    available: boolean
    canGoUp: boolean
    showHidden: boolean
    selectedKeys: string[]

    editablePath?: boolean
    pathDraft?: string

    unavailableTitle?: string
    unavailableHint?: string

    draggableEntries?: boolean

    dropSessionId?: string
    dropDirectory?: string

    readOnlyPath?: string

    sessionId?: string

    contextKind?: 'local' | 'remote'

    operationsDisabled?: boolean

    hideModifiedColumn?: boolean
  }>(),
  {
    editablePath: false,
    pathDraft: '',
    unavailableTitle: '',
    unavailableHint: '',
    draggableEntries: false,
    dropSessionId: '',
    dropDirectory: '',
    readOnlyPath: '',
    sessionId: '',
    contextKind: 'local',
    operationsDisabled: false,
    hideModifiedColumn: false,
  },
)
const { t, locale } = useI18n()

const emit = defineEmits<{
  navigate: [path: string]
  open: [entry: SftpBrowserEntry]
  goUp: []
  'update:showHidden': [value: boolean]
  'update:pathDraft': [value: string]
  select: [entry: SftpBrowserEntry, checked: boolean]
  retry: []
  dragStart: [entry: SftpBrowserEntry]
  dragEnd: []

  dropOnEntry: [entry: SftpBrowserEntry]

  dropOnDirectory: []

  unsupportedEntry: [entry: SftpBrowserEntry]
  createFile: []
  createFolder: []
  refresh: []
  uploadFile: []
  uploadDirectory: []
  copyPath: [entry: SftpBrowserEntry]
  rename: [entry: SftpBrowserEntry]
  chmod: [entry: SftpBrowserEntry]
  info: [entry: SftpBrowserEntry]
  deleteEntry: [entry: SftpBrowserEntry]
  download: [entry: SftpBrowserEntry]

  archiveDownload: [entry: SftpBrowserEntry]

  remoteEditSynced: [entry: SftpBrowserEntry]
}>()

const editing = ref(false)
const pathInput = ref<HTMLInputElement | null>(null)

const pathInputId = `sftp-browser-path-${Math.random().toString(36).slice(2, 9)}`
const dragOverKey = ref<string | null>(null)

const dragActive = ref(false)

const directoryKey = computed(() => props.segments.map((segment) => segment.path).join('\u0000'))

const {
  VIRTUAL_LIST_VERTICAL_PADDING,
  VIRTUAL_ROW_HEIGHT,
  bindScrollViewport,
  clearFilter,
  displayEntries,
  disposeScrollViewport,
  filterMatchCount,
  filterOpen,
  filterQuery,
  formatEntryTime,
  isVirtualized,
  resetVirtualScroll,
  scrollHost,
  selectedKeySet,
  sortMark,
  toggleSort,
  virtualContentHeight,
  virtualEntries,
  virtualOffset,
} = useSftpBrowserList({
  entries: () => props.entries,
  selectedKeys: () => props.selectedKeys,
  locale,
})

const contextEntry = ref<SftpBrowserEntry | null>(null)

const nativeDropDirectory = ref('')

const favoritesHostId = ref('')
const favoritePaths = ref<SFTPFavorite[]>([])
const favoritesBusy = ref(false)
const favoritesAvailable = computed(() => props.contextKind === 'remote' && Boolean(props.sessionId))
const favoritesCurrentPath = computed(() => props.segments[props.segments.length - 1]?.path ?? '/')

async function refreshFavorites(): Promise<void> {
  if (!favoritesAvailable.value) {
    favoritesHostId.value = ''
    favoritePaths.value = []
    return
  }
  const sessionId = props.sessionId
  try {
    const hostId = await resolveFavoritesHostId(sessionId)

    if (props.sessionId !== sessionId) return
    favoritesHostId.value = hostId
    favoritePaths.value = hostId ? await loadSFTPFavorites(hostId) : []
  } catch {
    favoritesHostId.value = ''
    favoritePaths.value = []
  }
}

watch(
  () => [props.sessionId, props.contextKind],
  () => void refreshFavorites(),
  { immediate: true },
)

async function addFavorite(path: string, name = ''): Promise<void> {
  if (!favoritesHostId.value || favoritesBusy.value || !path.startsWith('/')) return
  favoritesBusy.value = true
  try {
    favoritePaths.value = await saveSFTPFavorites(favoritesHostId.value, [...favoritePaths.value, { path, name }])
  } catch {
    toast.error(t('sftp.ui.favorites.saveFailed'))
  } finally {
    favoritesBusy.value = false
  }
}

async function renameFavorite(path: string, name: string): Promise<void> {
  if (!favoritesHostId.value || favoritesBusy.value) return
  const previous = favoritePaths.value
  favoritesBusy.value = true
  try {
    favoritePaths.value = await saveSFTPFavorites(
      favoritesHostId.value,
      previous.map((item) => (item.path === path ? { path: item.path, name } : item)),
    )
  } catch {
    favoritePaths.value = previous
    toast.error(t('sftp.ui.favorites.saveFailed'))
  } finally {
    favoritesBusy.value = false
  }
}

async function removeFavorite(path: string): Promise<void> {
  if (!favoritesHostId.value || favoritesBusy.value) return
  const previous = favoritePaths.value
  favoritesBusy.value = true
  try {
    favoritePaths.value = await saveSFTPFavorites(
      favoritesHostId.value,
      previous.filter((item) => item.path !== path),
    )
  } catch {
    favoritePaths.value = previous
    toast.error(t('sftp.ui.favorites.saveFailed'))
  } finally {
    favoritesBusy.value = false
  }
}

function captureContextEntry(event: MouseEvent): void {
  const target = event.target
  const row = target instanceof Element ? target.closest<HTMLElement>('[data-entry-key]') : null
  const key = row?.dataset.entryKey
  contextEntry.value = key ? (displayEntries.value.find((entry) => entry.key === key) ?? null) : null
}

function onEntryDoubleClick(entry: SftpBrowserEntry, event: MouseEvent): void {
  if ((event.target as Element | null)?.closest('[data-slot="checkbox"]')) return
  if (entry.type === 'directory') {
    emit('open', entry)
    return
  }

  if (entry.type === 'other') {
    emit('unsupportedEntry', entry)
    return
  }

  if (canPreview(entry)) {
    requestImagePreview(entry)
    return
  }
  if (canRemoteEdit(entry)) requestRemoteTextEditor(remoteSessionId.value, entry.key)
}

const remoteEdits = useRemoteEdits()

const remoteSessionId = computed(() => props.sessionId ?? '')

const currentDirectory = computed(() => props.segments.at(-1)?.path ?? '')

function canRemoteEdit(entry: SftpBrowserEntry | null): boolean {
  if (!entry || !remoteSessionId.value || props.contextKind !== 'remote' || !props.available) return false
  return entry.type !== 'directory' && !canPreview(entry)
}

const previewTarget = ref<SftpBrowserEntry | null>(null)

function canPreview(entry: SftpBrowserEntry | null): boolean {
  if (!entry || !remoteSessionId.value || props.contextKind !== 'remote' || !props.available) return false
  return entry.type === 'file' && isPreviewableImageName(entry.name)
}

function requestImagePreview(entry: SftpBrowserEntry): void {
  if (!canPreview(entry)) return
  previewTarget.value = entry
}

function remoteEditFor(entry: SftpBrowserEntry | null): RemoteEditView | null {
  if (!entry) return null
  return remoteEdits.editFor(remoteSessionId.value, entry.key)
}

const contextEdit = computed(() => remoteEditFor(contextEntry.value))

const virtualRows = computed(() =>
  virtualEntries.value.map((entry) => {
    const sync = remoteEdits.editFor(remoteSessionId.value, entry.key) ?? null
    return {
      entry,
      sync,

      syncDotHidden: sync ? remoteEdits.dotHiddenFor(sync) : false,
    }
  }),
)

const SYNC_TONE_CLASSES: Record<ReturnType<typeof editStateTone>, string> = {
  muted: 'text-qt-muted',
  accent: 'text-qt-accent',
  success: 'text-qt-success',
  danger: 'text-qt-danger',
}

function syncToneClass(view: RemoteEditView): string {
  return SYNC_TONE_CLASSES[editStateTone(view)]
}

function syncStateText(view: RemoteEditView): string {
  const state = t(editStateLocaleKey(view))
  const reasonKey = remoteEdits.reasonKeyFor(view)
  return reasonKey
    ? `${state} · ${translateErrorKey(reasonKey, t('error.unknown'))}`
    : t('remote.ui.stateLabel', { state })
}

function reasonText(key: string): string {
  return translateErrorKey(key, t('error.unknown'))
}

async function openEntryWithLocalProgram(entry: SftpBrowserEntry): Promise<void> {
  const result = await openWithLocalProgram(remoteSessionId.value, entry.key)
  if (result.ok) toast.success(t('remote.ui.toast.openedLocal', { path: entry.name }))
  else toast.error(t('remote.ui.toast.openFailed', { reason: reasonText(result.errorKey) }))
}

async function syncEntryNow(entry: SftpBrowserEntry): Promise<void> {
  const edit = remoteEditFor(entry)
  if (!edit) return
  const result = await syncRemoteEdit(edit.id)

  if (result.conflict) return
  if (result.ok) {
    toast.success(t('remote.ui.toast.synced'))

    return
  }
  toast.error(t('remote.ui.toast.syncFailed', { reason: reasonText(result.errorKey) }))
}

const pendingEditAction = ref<{ action: 'stop' | 'discard'; entry: SftpBrowserEntry } | null>(null)

function requestEditAction(action: 'stop' | 'discard'): void {
  const entry = contextEntry.value
  if (!entry) return
  pendingEditAction.value = { action, entry }
}

async function confirmEditAction(): Promise<void> {
  const pending = pendingEditAction.value
  pendingEditAction.value = null
  if (!pending) return
  const { action, entry } = pending
  const edit = remoteEditFor(entry)
  if (!edit) return
  if (action === 'stop') {
    const result = await stopRemoteEdit(edit.id)
    if (result.ok) toast.success(t('remote.ui.toast.stopped'))
    else toast.error(t('remote.ui.toast.stopFailed', { reason: reasonText(result.errorKey) }))
    return
  }

  const result = await reloadExternalEdit(edit.id, remoteSessionId.value, entry.key)
  if (result.ok) toast.success(t('remote.ui.toast.openedLocal', { path: entry.name }))
  else toast.error(t('remote.ui.toast.openFailed', { reason: reasonText(result.errorKey) }))
}

watch(
  () => remoteEdits.lastSynced.value,
  (synced) => {
    if (!synced || synced.sessionId !== remoteSessionId.value) return
    const slash = synced.remotePath.lastIndexOf('/')
    const directory = slash < 0 ? '/' : synced.remotePath.slice(0, slash) || '/'
    if (directory !== currentDirectory.value) return
    const entry = displayEntries.value.find((item) => item.key === synced.remotePath)
    if (entry) emit('remoteEditSynced', entry)
  },
)

function onEntryKeydown(entry: SftpBrowserEntry, event: KeyboardEvent): void {
  if (event.target !== event.currentTarget) return
  if (event.key !== 'Enter' || entry.type !== 'directory') return
  event.preventDefault()
  emit('open', entry)
}

type EntryAction =
  | 'aiAnalyze'
  | 'copyPath'
  | 'rename'
  | 'chmod'
  | 'info'
  | 'deleteEntry'
  | 'download'
  | 'archiveDownload'
  | 'preview'
  | 'openWithLocal'
  | 'openInEditor'
  | 'syncNow'
  | 'stopSync'
  | 'discardLocal'

function emitEntryAction(action: EntryAction): void {
  const entry = contextEntry.value
  if (!entry) return
  switch (action) {
    case 'aiAnalyze':
      requestAILaunch({ kind: 'file', text: entry.key, mode: 'agent' })
      break
    case 'copyPath':
      emit('copyPath', entry)
      break
    case 'rename':
      emit('rename', entry)
      break
    case 'chmod':
      emit('chmod', entry)
      break
    case 'info':
      emit('info', entry)
      break
    case 'deleteEntry':
      emit('deleteEntry', entry)
      break
    case 'download':
      emit('download', entry)
      break
    case 'archiveDownload':
      emit('archiveDownload', entry)
      break
    case 'preview':
      requestImagePreview(entry)
      break
    case 'openWithLocal':
      void openEntryWithLocalProgram(entry)
      break
    case 'openInEditor':
      if (canPreview(entry)) requestImagePreview(entry)
      else requestRemoteTextEditor(remoteSessionId.value, entry.key)
      break
    case 'syncNow':
      void syncEntryNow(entry)
      break
    case 'stopSync':
      requestEditAction('stop')
      break
    case 'discardLocal':
      requestEditAction('discard')
      break
  }
}

watch(directoryKey, () => {
  clearFilter()
  contextEntry.value = null
  resetVirtualScroll()
})

const breadcrumbNav = ref<HTMLElement | null>(null)
const breadcrumbMeasure = ref<HTMLElement | null>(null)

const breadcrumbTail = ref(0)

const breadcrumbLeading = ref(false)

const ELLIPSIS_WIDTH = 30

const visibleSegments = computed<SftpBrowserSegment[]>(() => {
  if (breadcrumbTail.value === 0) return props.segments
  const ellipsis: SftpBrowserSegment = {
    label: '…',
    path: '__ellipsis__',
    title: t('sftp.ui.chooseMiddlePath'),
    ellipsis: true,
  }
  const tail = props.segments.slice(props.segments.length - breadcrumbTail.value)
  const first = props.segments[0]
  return breadcrumbLeading.value && first ? [first, ellipsis, ...tail] : [ellipsis, ...tail]
})

const hiddenSegments = computed(() => {
  if (breadcrumbTail.value === 0) return []
  const start = breadcrumbLeading.value ? 1 : 0
  const end = props.segments.length - breadcrumbTail.value
  return end > start ? props.segments.slice(start, end) : []
})

function updateBreadcrumbOverflow(): void {
  const nav = breadcrumbNav.value
  const measure = breadcrumbMeasure.value
  if (!nav || !measure) return
  const nodes = Array.from(measure.children) as HTMLElement[]
  if (nodes.length === 0 || nodes.length !== props.segments.length) {
    breadcrumbTail.value = 0
    breadcrumbLeading.value = false
    return
  }
  const widths = nodes.map((node) => node.offsetWidth)
  const available = nav.clientWidth
  const total = widths.reduce((sum, width) => sum + width, 0)

  if (total <= available + 1) {
    breadcrumbTail.value = 0
    breadcrumbLeading.value = false
    return
  }

  const budget = Math.max(0, available - ELLIPSIS_WIDTH)

  let used = 0
  let tail = 0
  for (let index = widths.length - 1; index >= 0; index -= 1) {
    const next = used + (widths[index] ?? 0)
    if (tail > 0 && next > budget) break
    used = next
    tail += 1
  }
  breadcrumbTail.value = Math.max(1, tail)

  const remaining = budget - used
  const hiddenCount = widths.length - breadcrumbTail.value
  breadcrumbLeading.value = hiddenCount > 1 && remaining >= (widths[0] ?? 0)
}

let breadcrumbResizeObserver: ResizeObserver | undefined

watch(
  () => props.segments,
  () => {
    void nextTick(updateBreadcrumbOverflow)
  },
  { flush: 'post' },
)

onMounted(() => {
  if (remoteSessionId.value) void refreshRemoteEdits()
  if (typeof ResizeObserver !== 'undefined') {
    breadcrumbResizeObserver = new ResizeObserver(updateBreadcrumbOverflow)
    if (breadcrumbNav.value) breadcrumbResizeObserver.observe(breadcrumbNav.value)
  }
  void nextTick(() => {
    updateBreadcrumbOverflow()
    bindScrollViewport()
  })
})

onBeforeUnmount(() => {
  breadcrumbResizeObserver?.disconnect()
  disposeScrollViewport()
})

async function beginPathEditing(): Promise<void> {
  emit('update:pathDraft', props.segments.at(-1)?.path ?? '')
  editing.value = true
  await nextTick()
  pathInput.value?.focus()
  pathInput.value?.select()
}

async function copyReadOnlyPath(): Promise<void> {
  const value = props.readOnlyPath
  if (!value) return
  try {
    await writeClipboardText(value)
    toast.success(t('sftp.ui.copyPathSuccess'), { description: value })
  } catch {
    toast.error(t('sftp.ui.copyPathFailed'), { description: t('sftp.ui.copyPathManual') })
  }
}

function submitPath(): void {
  const draft = (props.pathDraft ?? '').trim()
  editing.value = false

  if (!draft) return
  emit('navigate', draft)
}

function cancelPathEditing(): void {
  editing.value = false
}

function onEntryDragStart(entry: SftpBrowserEntry, event: DragEvent): void {
  if (!props.draggableEntries) return
  event.dataTransfer?.setData('text/plain', entry.name)
  if (event.dataTransfer) event.dataTransfer.effectAllowed = 'copy'
  emit('dragStart', entry)
}

function onEntryDragOver(entry: SftpBrowserEntry, event: DragEvent): void {
  if (!props.draggableEntries) return

  if (entry.type !== 'directory') return
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
  dragOverKey.value = entry.key

  if (isNativeFileDrag(event)) nativeDropDirectory.value = entry.key
}

function onEntryDragLeave(entry: SftpBrowserEntry): void {
  if (dragOverKey.value === entry.key) dragOverKey.value = null
  if (nativeDropDirectory.value === entry.key) nativeDropDirectory.value = ''
}

function onEntryDrop(entry: SftpBrowserEntry, event: DragEvent): void {
  if (entry.type !== 'directory') return
  event.preventDefault()

  if (!isNativeFileDrag(event)) event.stopPropagation()
  dragOverKey.value = null
  dragActive.value = false
  nativeDropDirectory.value = ''
  emit('dropOnEntry', entry)
}

function isNativeFileDrag(event: DragEvent): boolean {
  return Array.from(event.dataTransfer?.types ?? []).includes('Files')
}

function onPaneDragOver(event: DragEvent): void {
  if (!props.draggableEntries) return
  event.preventDefault()
  dragActive.value = true
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
}

function onPaneDragLeave(event: DragEvent): void {
  const next = event.relatedTarget as Node | null
  if (next && (event.currentTarget as HTMLElement).contains(next)) return
  dragActive.value = false
}

function onDragEnd(): void {
  dragActive.value = false
  dragOverKey.value = null
  nativeDropDirectory.value = ''
  emit('dragEnd')
}

function onPaneDrop(event: DragEvent): void {
  if (!props.draggableEntries) return
  event.preventDefault()
  dragActive.value = false
  dragOverKey.value = null
  nativeDropDirectory.value = ''
  emit('dropOnDirectory')
}
</script>

<template>
  <div
    class="sftp-panel sftp-browser-pane h-full min-h-0 min-w-0 flex flex-col overflow-hidden"
    :class="{
      'is-drop-active': dragActive,
      'sftp-browser-pane--name-size': props.hideModifiedColumn,
    }"
    :data-file-drop-target="dropSessionId ? '' : undefined"
    :data-drop-kind="dropSessionId ? 'sftp' : undefined"
    :data-drop-session="dropSessionId || undefined"
    :data-drop-directory="nativeDropDirectory || dropDirectory || undefined"
    :data-drop-hint="t('sftp.ui.uploadHint')"
    @dragover="onPaneDragOver"
    @dragleave="onPaneDragLeave"
    @drop="onPaneDrop"
  >
    <span
      v-if="dropSessionId"
      class="file-drop-hint absolute z-21 inset-0 grid place-items-center invisible pointer-events-none"
      aria-hidden="true"
      ><span class="text-sm">{{ t('sftp.ui.uploadHint') }}</span></span
    >
    <div class="path-bar" :class="{ 'is-editing': editing }">
      <label class="sr-only" :for="pathInputId">{{ t('sftp.ui.directory') }}</label>
      <input
        v-if="editing"
        :id="pathInputId"
        ref="pathInput"
        data-slot="input"
        class="sftp-path-input qterm-reference-input text-xs"
        :value="pathDraft"
        :disabled="!available"
        autocomplete="off"
        autocorrect="off"
        autocapitalize="off"
        spellcheck="false"
        @input="emit('update:pathDraft', ($event.target as HTMLInputElement).value)"
        @keydown.enter.prevent="submitPath"
        @keydown.esc.prevent="cancelPathEditing"
      />
      <nav
        v-else
        ref="breadcrumbNav"
        class="sftp-breadcrumbs min-w-0 flex-1 flex items-center gap-[2px] overflow-hidden text-qt-muted text-sm"
        :aria-label="t('sftp.ui.directoryPath')"
        :title="readOnlyPath ? t('sftp.ui.pathReadOnlyTitle', { path: readOnlyPath }) : undefined"
      >
        <template v-for="(segment, index) in visibleSegments" :key="segment.path">
          <ChevronRight v-if="index > 0" :size="13" aria-hidden="true" />
          <DropdownMenu v-if="segment.ellipsis">
            <DropdownMenuTrigger as-child>
              <QtermButton variant="ghost" size="icon-sm" type="button" :aria-label="t('sftp.ui.chooseMiddlePath')"
                ><MoreHorizontal :size="16"
              /></QtermButton>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" class="sftp-breadcrumb-menu">
              <DropdownMenuItem
                v-for="hidden in hiddenSegments"
                :key="hidden.path"
                @select="emit('navigate', hidden.path)"
              >
                {{ hidden.title ?? hidden.path }}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <button
            v-else
            type="button"
            :disabled="!available"
            :title="segment.title ?? segment.path"
            :class="{ 'sftp-breadcrumb-current': index === visibleSegments.length - 1 }"
            @click="emit('navigate', segment.path)"
          >
            {{ segment.label }}
          </button>
        </template>
      </nav>

      <nav ref="breadcrumbMeasure" class="sftp-breadcrumb-measure text-sm" aria-hidden="true">
        <span v-for="(segment, index) in segments" :key="segment.path" class="sftp-breadcrumb-measure-item">
          <ChevronRight v-if="index > 0" :size="13" />
          <span>{{ segment.label }}</span>
        </span>
      </nav>

      <SftpFavoritesMenu
        v-if="!editing && favoritesAvailable && favoritesHostId"
        :favorites="favoritePaths"
        :current-path="favoritesCurrentPath"
        :disabled="!available"
        :busy="favoritesBusy"
        @navigate="emit('navigate', $event)"
        @add="(path, name) => void addFavorite(path, name)"
        @remove="(path) => void removeFavorite(path)"
        @rename="(path, name) => void renameFavorite(path, name)"
      />
      <QtermButton
        v-if="editing"
        variant="ghost"
        size="icon-sm"
        class="sftp-path-clear"
        type="button"
        :aria-label="t('sftp.ui.cancelEditPath')"
        @click="cancelPathEditing"
        ><X :size="16"
      /></QtermButton>
      <QtermButton
        v-else-if="editablePath"
        variant="ghost"
        size="icon-sm"
        type="button"
        :aria-label="t('sftp.ui.editPath')"
        :disabled="!available"
        @click="beginPathEditing"
        ><Pencil :size="16"
      /></QtermButton>
      <QtermButton
        v-else-if="readOnlyPath"
        variant="ghost"
        size="icon-sm"
        type="button"
        :aria-label="t('sftp.ui.copyPath')"
        :title="t('sftp.ui.copyCurrentPathTitle', { path: readOnlyPath })"
        @click="copyReadOnlyPath"
        ><Copy :size="16"
      /></QtermButton>
    </div>

    <div class="sftp-function-bar" role="toolbar" :aria-label="t('sftp.ui.fileActions')">
      <slot name="toolbar" />
    </div>

    <div class="sftp-list-header text-xs" role="row">
      <span aria-hidden="true"></span>
      <div class="sftp-list-name-cell overflow-hidden">
        <button type="button" class="sftp-list-sort" @click="toggleSort('name')">
          {{ t('sftp.ui.info.name') }}<span class="sftp-list-sort-mark">{{ sortMark('name') }}</span>
        </button>

        <Popover v-model:open="filterOpen">
          <PopoverTrigger class="sftp-list-filter-button" :aria-label="t('sftp.ui.filterByName')">
            <Search :size="13" aria-hidden="true" />
          </PopoverTrigger>
          <PopoverContent
            class="qterm-dialog sftp-filter-popover"
            :side-offset="6"
            :collision-padding="12"
            align="start"
          >
            <label class="sftp-filter-search">
              <Search :size="15" aria-hidden="true" />
              <span class="sr-only">{{ t('sftp.ui.filterName') }}</span>
              <QtermInput
                v-model="filterQuery"
                data-qterm-composite-input
                :placeholder="t('sftp.ui.enterName')"
                type="search"
                autofocus
              />
            </label>
            <div class="sftp-filter-count text-xs">
              {{ t('sftp.ui.copiedItems', { matched: filterMatchCount, total: props.entries.length }) }}
            </div>
          </PopoverContent>
        </Popover>
      </div>
      <button
        v-if="!props.hideModifiedColumn"
        type="button"
        class="sftp-list-sort is-right"
        @click="toggleSort('modTime')"
      >
        {{ t('sftp.ui.modifiedDate') }}<span class="sftp-list-sort-mark">{{ sortMark('modTime') }}</span>
      </button>
      <button type="button" class="sftp-list-sort is-right" @click="toggleSort('size')">
        {{ t('sftp.ui.info.size') }}<span class="sftp-list-sort-mark">{{ sortMark('size') }}</span>
      </button>
    </div>
    <div ref="scrollHost" class="sftp-browser-scroll-host">
      <ContextMenu
        @update:open="
          (open: boolean) => {
            if (!open) contextEntry = null
          }
        "
      >
        <ContextMenuTrigger as-child>
          <div class="sftp-context-menu-surface" @contextmenu.capture="captureContextEntry">
            <ScrollArea class="flex-[1_1_0] min-h-0 overflow-hidden" type="hover" :scroll-hide-delay="600">
              <QtermEmptyState
                v-if="!available"
                :icon="FolderOpen"
                :title="unavailableTitle || t('sftp.ui.unavailableTitle')"
                :description="unavailableHint || t('sftp.ui.unavailableHint')"
              />
              <QtermEmptyState
                v-else-if="error"
                :icon="FileQuestion"
                :title="t('sftp.ui.readDirectoryFailed')"
                :description="error"
                role="alert"
              >
                <QtermButton variant="outline" type="button" @click="emit('retry')">{{
                  t('sftp.ui.retry')
                }}</QtermButton>
              </QtermEmptyState>
              <QtermEmptyState
                v-else-if="loading && entries.length === 0"
                :icon="RefreshCw"
                icon-class="spin"
                :description="t('sftp.ui.readingDirectory')"
                busy
              />
              <QtermEmptyState
                v-else-if="displayEntries.length === 0 && filterQuery.trim()"
                class="sftp-filter-empty-state"
                :description="t('sftp.ui.noMatchingFiles')"
              >
                <button class="sftp-clear-filter text-xs" type="button" @click="clearFilter">
                  {{ t('sftp.ui.clearFilter') }}
                </button>
              </QtermEmptyState>
              <QtermEmptyState
                v-else-if="displayEntries.length === 0"
                :icon="FolderOpen"
                :description="t('sftp.ui.emptyDirectory')"
              />

              <div
                v-else
                class="sftp-list py-[4px] px-[8px] pt-[4px] pr-[8px] pb-[4px] pl-[8px]"
                :class="{ 'sftp-list-virtual': isVirtualized }"
                role="list"
                :aria-busy="loading"
                :style="
                  isVirtualized ? { height: `${virtualContentHeight + VIRTUAL_LIST_VERTICAL_PADDING}px` } : undefined
                "
              >
                <div
                  class="sftp-list-window"
                  :class="{ 'is-virtual': isVirtualized }"
                  :style="isVirtualized ? { transform: `translateY(${virtualOffset}px)` } : undefined"
                >
                  <div
                    v-for="row in virtualRows"
                    :key="row.entry.key"
                    :data-entry-key="row.entry.key"
                    class="sftp-entry group"
                    role="listitem"
                    tabindex="0"
                    :class="{ 'is-drop-target': dragOverKey === row.entry.key }"
                    :draggable="
                      Boolean(draggableEntries) && (row.entry.type === 'file' || row.entry.type === 'directory')
                    "
                    :style="isVirtualized ? { height: `${VIRTUAL_ROW_HEIGHT - 2}px` } : undefined"
                    @dblclick="onEntryDoubleClick(row.entry, $event)"
                    @keydown.enter="onEntryKeydown(row.entry, $event)"
                    @dragstart="onEntryDragStart(row.entry, $event)"
                    @dragend="onDragEnd"
                    @dragover="onEntryDragOver(row.entry, $event)"
                    @dragleave="onEntryDragLeave(row.entry)"
                    @drop="onEntryDrop(row.entry, $event)"
                  >
                    <QtermCheckbox
                      :model-value="selectedKeySet.has(row.entry.key)"
                      :aria-label="t('sftp.ui.selectEntry', { name: row.entry.name })"
                      @click.stop
                      @update:model-value="(checked) => emit('select', row.entry, checked === true)"
                    />
                    <QtermButton
                      variant="ghost"
                      class="sftp-entry-open overflow-hidden"
                      :class="{ 'is-directory': row.entry.type === 'directory' }"
                      type="button"
                      :title="row.entry.key"
                      @dblclick.stop="onEntryDoubleClick(row.entry, $event)"
                    >
                      <SftpFileIcon
                        :name="row.entry.name"
                        :directory="row.entry.type === 'directory'"
                        :symlink="row.entry.symlink ?? row.entry.type === 'symlink'"
                      />
                      <span class="sftp-entry-main min-w-0 flex-1 flex flex-col gap-[2px]">
                        <span class="flex min-w-0 items-center gap-1.5">
                          <Tooltip>
                            <TooltipTrigger as-child>
                              <strong>{{ row.entry.name }}</strong>
                            </TooltipTrigger>
                            <TooltipContent>{{ row.entry.name }}</TooltipContent>
                          </Tooltip>

                          <span
                            v-if="row.sync"
                            class="sftp-entry-sync shrink-0"
                            :class="[
                              syncToneClass(row.sync),
                              row.syncDotHidden ? 'opacity-0 group-hover:opacity-100' : '',
                            ]"
                            :data-sync-state="row.sync.conflict ? 'conflict' : row.sync.state"
                            :data-sync-dot-hidden="row.syncDotHidden ? 'true' : undefined"
                            role="img"
                            :aria-label="syncStateText(row.sync)"
                            :title="syncStateText(row.sync)"
                          >
                            <span class="sftp-entry-sync-dot" aria-hidden="true" />
                          </span>
                        </span>
                        <small v-if="row.entry.mode" class="text-xs">{{ row.entry.mode }}</small>
                      </span>
                    </QtermButton>

                    <span
                      v-if="!props.hideModifiedColumn"
                      class="sftp-entry-date min-w-0 truncate text-xs"
                      :title="formatEntryTime(row.entry.modTime)"
                      >{{ formatEntryTime(row.entry.modTime) }}</span
                    >
                    <span
                      class="sftp-entry-size text-qt-muted min-w-0 truncate text-xs flex-none"
                      :title="row.entry.sizeText"
                      >{{ row.entry.sizeText }}</span
                    >
                  </div>
                </div>
              </div>
            </ScrollArea>
          </div>
        </ContextMenuTrigger>
        <ContextMenuContent class="sftp-context-menu">
          <template v-if="contextEntry">
            <ContextMenuItem
              v-if="contextKind === 'remote' && contextEntry.type === 'file'"
              :disabled="!available"
              data-ai-analyze-file
              @select="emitEntryAction('aiAnalyze')"
            >
              <Sparkles :size="14" aria-hidden="true" />{{ t('sftp.ui.aiAnalyzeFile') }}
            </ContextMenuItem>
            <ContextMenuItem :disabled="!available" @select="emitEntryAction('copyPath')">
              <Copy :size="14" aria-hidden="true" />{{ t('sftp.ui.copyPath') }}
            </ContextMenuItem>
            <ContextMenuItem
              v-if="contextKind === 'remote' && (contextEntry.type === 'file' || contextEntry.type === 'directory')"
              :disabled="!available || operationsDisabled"
              @select="emitEntryAction('download')"
            >
              <Download :size="14" aria-hidden="true" />{{ t('sftp.ui.download') }}
            </ContextMenuItem>

            <ContextMenuItem
              v-if="contextKind === 'remote' && contextEntry.type === 'directory'"
              :disabled="!available || operationsDisabled"
              data-sftp-archive-download
              @select="emitEntryAction('archiveDownload')"
            >
              <FileArchive :size="14" aria-hidden="true" />{{ t('sftp.ui.archiveDownload') }}
            </ContextMenuItem>

            <ContextMenuItem
              v-if="canPreview(contextEntry)"
              :disabled="!available || operationsDisabled"
              data-preview-action="preview"
              @select="emitEntryAction('preview')"
            >
              <ImageIcon :size="14" aria-hidden="true" />{{ t('preview.ui.menu') }}
            </ContextMenuItem>

            <template v-if="canRemoteEdit(contextEntry)">
              <ContextMenuSeparator />
              <ContextMenuItem data-remote-edit-action="open-local" @select="emitEntryAction('openWithLocal')">
                <ExternalLink :size="14" aria-hidden="true" />{{ t('remote.ui.openWithLocal') }}
              </ContextMenuItem>
              <ContextMenuItem data-remote-edit-action="open-editor" @select="emitEntryAction('openInEditor')">
                <FilePen :size="14" aria-hidden="true" />{{ t('remote.ui.openInEditor') }}
              </ContextMenuItem>
            </template>

            <template v-if="contextEdit">
              <ContextMenuSeparator />
              <ContextMenuItem
                :disabled="contextEdit ? !contextEdit.canSync : true"
                data-remote-edit-action="sync-now"
                @select="emitEntryAction('syncNow')"
              >
                <RefreshCw :size="14" aria-hidden="true" />{{ t('remote.ui.syncNow') }}
              </ContextMenuItem>
              <ContextMenuItem data-remote-edit-action="stop-sync" @select="emitEntryAction('stopSync')">
                <CircleSlash :size="14" aria-hidden="true" />{{ t('remote.ui.stopSync') }}
              </ContextMenuItem>
              <ContextMenuItem data-remote-edit-action="discard-local" @select="emitEntryAction('discardLocal')">
                <RotateCcw :size="14" aria-hidden="true" />{{ t('remote.ui.discardLocal') }}
              </ContextMenuItem>
            </template>
            <ContextMenuSeparator />
            <ContextMenuItem :disabled="!available || operationsDisabled" @select="emitEntryAction('rename')">
              <Pencil :size="14" aria-hidden="true" />{{ t('sftp.ui.rename') }}
            </ContextMenuItem>
            <ContextMenuItem :disabled="!available || operationsDisabled" @select="emitEntryAction('chmod')">
              <Shield :size="14" aria-hidden="true" />{{ t('sftp.ui.changePermissions') }}
            </ContextMenuItem>
            <ContextMenuItem :disabled="!available" @select="emitEntryAction('info')">
              <Info :size="14" aria-hidden="true" />{{ t('sftp.ui.fileInfo') }}
            </ContextMenuItem>
            <ContextMenuItem
              variant="destructive"
              :disabled="!available || operationsDisabled"
              @select="emitEntryAction('deleteEntry')"
            >
              <Trash2 :size="14" aria-hidden="true" />{{ t('sftp.ui.delete') }}
            </ContextMenuItem>
          </template>
          <template v-else>
            <ContextMenuItem :disabled="!available || operationsDisabled" @select="emit('refresh')">
              <RefreshCw :size="14" aria-hidden="true" />{{ t('sftp.ui.refresh') }}
            </ContextMenuItem>
            <ContextMenuItem
              :disabled="!available || operationsDisabled || Boolean(error) || loading"
              @select="emit('createFile')"
            >
              <FilePlus :size="14" aria-hidden="true" />{{ t('sftp.ui.createFile') }}
            </ContextMenuItem>
            <ContextMenuItem
              :disabled="!available || operationsDisabled || Boolean(error) || loading"
              @select="emit('createFolder')"
            >
              <FolderPlus :size="14" aria-hidden="true" />{{ t('sftp.ui.createFolder') }}
            </ContextMenuItem>
            <template v-if="contextKind === 'remote'">
              <ContextMenuSeparator />
              <ContextMenuItem
                :disabled="!available || operationsDisabled || Boolean(error) || loading"
                @select="emit('uploadFile')"
              >
                <Upload :size="14" aria-hidden="true" />{{ t('sftp.ui.uploadFile') }}
              </ContextMenuItem>
              <ContextMenuItem
                :disabled="!available || operationsDisabled || Boolean(error) || loading"
                @select="emit('uploadDirectory')"
              >
                <FolderUp :size="14" aria-hidden="true" />{{ t('sftp.ui.uploadFolder') }}
              </ContextMenuItem>
            </template>
            <ContextMenuSeparator />
            <ContextMenuItem :disabled="!available" @select="emit('update:showHidden', !showHidden)">
              <Eye v-if="!showHidden" :size="14" aria-hidden="true" /><EyeOff v-else :size="14" aria-hidden="true" />
              {{ t(showHidden ? 'sftp.ui.hideHidden' : 'sftp.ui.showHidden') }}
            </ContextMenuItem>
          </template>
        </ContextMenuContent>
      </ContextMenu>
    </div>

    <QtermDialog :open="pendingEditAction !== null" @update:open="(open) => !open && (pendingEditAction = null)">
      <QtermDialogContent v-if="pendingEditAction" size="sm" @pointer-down-outside.prevent>
        <QtermDialogHeader
          :title="t(pendingEditAction.action === 'stop' ? 'remote.ui.stop.title' : 'remote.ui.editor.discardTitle')"
          :show-close="false"
        />
        <div class="dialog-body">
          <p class="text-xs text-qt-muted">
            {{ t(pendingEditAction.action === 'stop' ? 'remote.ui.stop.message' : 'remote.ui.editor.discardMessage') }}
          </p>
        </div>
        <QtermDialogFooter class="flex justify-end gap-2">
          <QtermButton variant="outline" @click="pendingEditAction = null">{{ t('sftp.ui.cancel') }}</QtermButton>
          <QtermButton variant="destructive" data-testid="remote-edit-confirm-pending" @click="confirmEditAction">
            {{ t(pendingEditAction.action === 'stop' ? 'remote.ui.stop.confirm' : 'remote.ui.editor.discardConfirm') }}
          </QtermButton>
        </QtermDialogFooter>
      </QtermDialogContent>
    </QtermDialog>

    <ImagePreviewDialog
      v-if="previewTarget"
      :session-id="remoteSessionId"
      :remote-path="previewTarget.key"
      :name="previewTarget.name"
      :size-text="previewTarget.sizeText"
      :mod-time="previewTarget.modTime"
      @close="previewTarget = null"
    />
  </div>
</template>

<style scoped>
.sftp-browser-pane {
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
}

.sftp-browser-scroll-host,
.sftp-context-menu-surface {
  display: flex;
  flex: 1 1 0;
  min-height: 0;
  min-width: 0;
  flex-direction: column;
}

.sftp-context-menu-surface {
  overflow: hidden;
}

.sftp-browser-pane {
  --qt-sftp-date-column: 8.5rem;
  --qt-sftp-size-column: 4.25rem;
}

.sftp-list-header,
.sftp-entry {
  display: grid;
  grid-template-columns: 28px minmax(0, 1fr) var(--qt-sftp-date-column) var(--qt-sftp-size-column);
  align-items: center;
  gap: 8px;
}

.sftp-browser-pane--name-size .sftp-list-header,
.sftp-browser-pane--name-size .sftp-entry {
  grid-template-columns: 28px minmax(0, 1fr) var(--qt-sftp-size-column);
}
.sftp-list-header {
  flex: none;
  min-height: 30px;

  padding: 0 18px;
  border-bottom: 1px solid var(--qt-border);
  color: var(--qt-muted);
}
.sftp-list-name-cell {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.sftp-list-window {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.sftp-list-virtual {
  position: relative;
  display: block;
  overflow: hidden;
}

.sftp-list-window.is-virtual {
  position: absolute;
  top: 4px;
  right: 8px;
  left: 8px;
}
.sftp-list-sort {
  display: inline-flex;
  flex: none;
  align-items: center;
  gap: 3px;
  border: 0;
  padding: 0;
  color: inherit;
  background: transparent;
  cursor: pointer;
  font: inherit;
  white-space: nowrap;
}
.sftp-list-sort:hover {
  color: var(--qt-text);
}

.sftp-list-sort.is-right {
  justify-content: flex-start;
}
.sftp-list-sort-mark {
  color: var(--primary);
}
.sftp-list-filter-button {
  display: inline-grid;
  flex: none;
  place-items: center;
  width: 20px;
  height: 20px;
  border: 0;
  border-radius: 4px;
  color: inherit;
  background: transparent;
  cursor: pointer;
}
.sftp-list-filter-button:hover {
  color: var(--qt-text);
  background: transparent;
}

.sftp-filter-popover {
  display: flex;
  width: 320px;
  flex-direction: column;
  gap: 8px;
  padding: 10px;
}
.sftp-filter-search {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 10px;
  border: 1px solid var(--qt-border);
  border-radius: 6px;
}
.sftp-filter-search :deep(input) {
  border: 0;
  background: transparent !important;
  box-shadow: none !important;
}
.sftp-filter-search:focus-within {
  border-color: var(--qt-focus);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--qt-focus) 28%, transparent);
}
.sftp-filter-count {
  color: var(--qt-muted);
}
.sftp-filter-empty-state {
  min-height: 100%;
  color: var(--qt-muted);
}
.sftp-clear-filter {
  padding: 0;
  border: 0;
  color: var(--qt-muted);
  background: transparent;
  cursor: pointer;
}
.sftp-clear-filter:hover {
  color: var(--qt-text);
}

.sftp-entry-open {
  min-width: 0;
  padding: 0;
  justify-content: flex-start;
}
.sftp-entry-date {
  color: var(--qt-muted);
  text-align: left;
  white-space: nowrap;
}

.sftp-entry-size {
  text-align: left;
}

.sftp-entry-sync {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  white-space: nowrap;
}
.sftp-entry-sync-dot {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: currentColor;
}
.sftp-entry-sync[data-sync-state='syncing'] .sftp-entry-sync-dot {
  animation: sftp-entry-sync-pulse 1.2s ease-in-out infinite;
}
@keyframes sftp-entry-sync-pulse {
  50% {
    opacity: 0.25;
  }
}

.sftp-entry.is-drop-target {
  outline: 1px solid var(--qt-accent, var(--primary));
  outline-offset: -1px;
  background: var(--qt-hover);
}

.sftp-browser-pane.is-drop-active :deep([data-slot='scroll-area-viewport']) {
  background: color-mix(in srgb, var(--qt-muted) 8%, transparent);
}

.sftp-breadcrumb-measure-item {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  white-space: nowrap;
}
.sftp-breadcrumb-measure-item > svg {
  flex: 0 0 13px;
  width: 13px;
  min-width: 13px;
}
</style>
