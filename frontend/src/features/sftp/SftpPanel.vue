<script setup lang="ts">
import {
  FormField,
  QtermButton,
  QtermCheckbox,
  QtermFormMessage,
  QtermInput,
  RefreshIconButton,
} from '../../components/common'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Undo2, LocateFixed, Eye, EyeOff, FolderPlus, FolderUp, Trash2, Upload } from '@lucide/vue'
import { toast } from 'vue-sonner'
import {
  QtermDialog,
  QtermDialogContent,
  QtermDialogDescription,
  QtermDialogFooter,
  QtermDialogHeader,
} from '../../components/common/dialog'
import { Label } from '../../components/common/control-primitives'
import { sftpErrorMessage, type RemoteFileEntry, type SftpGateway } from '../../services/sftp'
import IconButton from '../../components/common/button/IconButton.vue'
import DeleteConfirmDialog from '../../components/dialogs/DeleteConfirmDialog.vue'
import TransferProgressPanel from '../../components/common/feedback/TransferProgressPanel.vue'
import { QtermCombobox, type QtermComboboxOption } from '../../components/common/form-controls'
import {
  readSessionBrowserDirectory,
  readSessionDirectoryFollowEnabled,
  readSessionFollowDirectory,
  rememberSessionBrowserDirectory,
  rememberSessionFollowDirectory,
  requestDirectoryFollow,
  setSessionDirectoryFollowEnabled,
} from '../../services/directory-follow'
import { getPreference, PREFERENCE_KEYS, readCachedPreference, setPreference } from '../../services/preferences'
import { useI18n } from '@/locales'
import { loadSettings, loadSettingsFromPreferences } from '../settings/settingsStore'
import { useSftpPanelTransfers } from './useSftpPanelTransfers'
import SftpBrowserPane, { type SftpBrowserEntry, type SftpBrowserSegment } from './SftpBrowserPane.vue'
import SftpFileInfoDialog from './SftpFileInfoDialog.vue'
import { writeClipboardText } from '../../services/clipboard'

const props = withDefaults(
  defineProps<{
    gateway: SftpGateway
    sessionId: string
    available: boolean
    preview: boolean

    showTransferPanel?: boolean
    directoryFollowMode?: import('../settings/settingsStore').SftpDirectoryFollowMode
    autoDirectoryFollow?: boolean
    directoryFollowEnabled?: boolean

    showHiddenFiles?: 'inherit' | 'enabled' | 'disabled'

    defaultDirectory?: string

    hideModifiedColumn?: boolean
  }>(),
  {
    showTransferPanel: true,
    directoryFollowMode: 'remote',
    autoDirectoryFollow: true,
    directoryFollowEnabled: false,
    showHiddenFiles: 'inherit',
    defaultDirectory: '',
    hideModifiedColumn: false,
  },
)

function resolveShowHidden(override: 'inherit' | 'enabled' | 'disabled' | undefined, globalValue: boolean): boolean {
  if (override === 'enabled') return true
  if (override === 'disabled') return false
  return globalValue
}
const { t } = useI18n()

const directory = ref('/')
const pathDraft = ref('/')
const entries = ref<RemoteFileEntry[]>([])
const loading = ref(false)
const error = ref('')
let requestSequence = 0
let sessionGeneration = 0

const showHidden = ref(resolveShowHidden(props.showHiddenFiles, loadSettings().sftpShowHiddenFiles ?? false))
const visibleEntries = computed(() => entries.value.filter((entry) => showHidden.value || !entry.name.startsWith('.')))
const pathSegments = computed(() => {
  const parts = directory.value.split('/').filter(Boolean)
  return [
    { label: '/', path: '/', ellipsis: false },
    ...parts.map((part, index) => ({ label: part, path: `/${parts.slice(0, index + 1).join('/')}`, ellipsis: false })),
  ]
})
const selectedEntries = ref<RemoteFileEntry[]>([])
const browserEntries = computed<SftpBrowserEntry[]>(() =>
  visibleEntries.value.map((entry) => ({
    key: entry.path,
    name: entry.name,
    type: entryIsDirectory(entry) ? 'directory' : entry.type === 'symlink' ? 'other' : entry.type,
    symlink: entry.type === 'symlink',
    mode: entry.mode,
    uid: entry.uid,
    gid: entry.gid,
    modTime: entry.modTime,
    size: entry.size,
    sizeText: entryIsDirectory(entry) ? t('sftp.ui.directoryName') : entry.sizeText || '—',
  })),
)
const browserSegments = computed<SftpBrowserSegment[]>(() =>
  pathSegments.value.map((segment) => ({ label: segment.label, path: segment.path })),
)
const selectedKeys = computed(() => selectedEntries.value.map((entry) => entry.path))

function browserEntrySource(entry: SftpBrowserEntry): RemoteFileEntry | undefined {
  return entries.value.find((item) => item.path === entry.key)
}

function selectBrowserEntry(entry: SftpBrowserEntry, checked: boolean): void {
  const source = browserEntrySource(entry)
  if (source) toggleEntry(source, checked)
}
const deleteTargets = ref<RemoteFileEntry[]>([])

const deleteElevated = ref(false)

const deleteElevatedPassword = ref('')

const archiveDownloading = ref(false)

const deleteTargetsIncludeDirectory = computed(() => deleteTargets.value.some((entry) => entry.type === 'directory'))
function toggleEntry(entry: RemoteFileEntry, checked: boolean): void {
  selectedEntries.value = checked
    ? [...selectedEntries.value.filter((item) => item.path !== entry.path), entry]
    : selectedEntries.value.filter((item) => item.path !== entry.path)
}
const createFolderOpen = ref(false)
const deleteEntryOpen = ref(false)
const folderName = ref('')
const entryError = ref('')
const entryBusy = ref(false)
function toggleHidden(): void {
  showHidden.value = !showHidden.value
  selectedEntries.value = []
}
function openCreateFolder(): void {
  folderName.value = ''
  entryError.value = ''
  createFolderOpen.value = true
}
function openDeleteEntry(): void {
  deleteElevated.value = false
  deleteElevatedPassword.value = ''
  deleteTargets.value = [...selectedEntries.value]
  entryError.value = ''
  deleteEntryOpen.value = true
}
async function mutateEntry(kind: 'create' | 'delete'): Promise<void> {
  if (entryBusy.value || !props.available) return
  const generation = sessionGeneration
  const sessionID = props.sessionId
  const currentDirectory = directory.value
  entryBusy.value = true
  entryError.value = ''
  try {
    if (kind === 'create') await props.gateway.createRemoteDirectory(sessionID, currentDirectory, folderName.value)
    else {
      const failed: RemoteFileEntry[] = []
      const messages: string[] = []
      for (const entry of deleteTargets.value) {
        if (!isCurrentSessionOperation(generation, sessionID)) return
        try {
          if (deleteElevated.value && entry.type !== 'directory') {
            await props.gateway.deleteRemoteEntryElevated(sessionID, entry.path, deleteElevatedPassword.value)
          } else {
            await props.gateway.deleteRemoteEntry(sessionID, entry.path)
          }
        } catch (reason) {
          failed.push(entry)
          messages.push(entry.name + '：' + sftpErrorMessage(reason, t('sftp.ui.delete')))
        }
      }
      if (!isCurrentSessionOperation(generation, sessionID)) return
      await loadDirectory(currentDirectory)
      if (!isCurrentSessionOperation(generation, sessionID)) return
      selectedEntries.value = failed
      deleteTargets.value = failed
      entryError.value = messages.join('；')
      deleteEntryOpen.value = failed.length > 0
      return
    }
    if (!isCurrentSessionOperation(generation, sessionID)) return
    createFolderOpen.value = false
    deleteEntryOpen.value = false
    selectedEntries.value = []
    await loadDirectory(currentDirectory)
  } catch (reason) {
    if (isCurrentSessionOperation(generation, sessionID))
      entryError.value = sftpErrorMessage(reason, t('sftp.ui.operationFailed'))
  } finally {
    if (generation === sessionGeneration) entryBusy.value = false

    deleteElevatedPassword.value = ''
  }
}
const canGoUp = computed(() => props.available && directory.value !== '/')
const {
  filePickerBusy,
  transferError,
  legacyOverwriteTransfer,
  progressItems,
  handleTransferAction,
  dismissTransfer,
  selectUpload,
  selectDownload,
  refreshTransfers,
  resetForSession,
} = useSftpPanelTransfers({
  gateway: () => props.gateway,
  sessionId: () => props.sessionId,
  available: () => props.available,
  directory,
  sessionGeneration: () => sessionGeneration,
  isCurrentSessionOperation,
  loadDirectory,
  t,
})
interface FollowPreference {
  enabled: boolean
  promptDismissed: boolean
}

function readFollowPreference(): FollowPreference {
  try {
    const cached = readCachedPreference(PREFERENCE_KEYS.sftpFollow)
    if (cached) {
      const value = JSON.parse(cached) as Partial<FollowPreference>
      if (typeof value.enabled === 'boolean' && typeof value.promptDismissed === 'boolean')
        return value as FollowPreference
    }
  } catch {}
  return {
    enabled: localStorage.getItem('qterm:sftp-follow-directory') === 'true',
    promptDismissed: localStorage.getItem('qterm:sftp-follow-prompt-dismissed') === 'true',
  }
}

const initialFollowPreference = readFollowPreference()

const remoteFollowEnabled = ref(initialFollowPreference.enabled)

const sessionFollowEnabled = ref(true)
const followDirectory = computed(() => {
  if (props.directoryFollowMode === 'disabled') return false
  if (props.directoryFollowMode === 'session') return sessionFollowEnabled.value
  return remoteFollowEnabled.value
})
const followPromptDismissed = ref(initialFollowPreference.promptDismissed)
let followPreferenceEdited = false
const followedDirectory = ref('')
const followPromptOpen = ref(false)
const followDetected = ref<boolean | null>(null)
let followAbort: AbortController | null = null

function handleCurrentDirectory(event: Event): void {
  const detail = (event as CustomEvent<{ sessionId?: string; directory?: string }>).detail
  if (!detail?.sessionId || !detail.directory) return
  if (!followDirectory.value || detail.sessionId !== props.sessionId) return
  rememberSessionFollowDirectory(detail.sessionId, detail.directory)
  followedDirectory.value = detail.directory
  void loadDirectory(detail.directory)
}

const followBusy = ref(false)
const followError = ref('')
const followNotice = ref('')
const followNoticeOpen = ref(false)
async function runFollow(action: 'probe' | 'configure'): Promise<void> {
  if (props.directoryFollowMode === 'session' || props.directoryFollowMode === 'disabled') return
  if (!props.available || !props.sessionId || props.preview) return
  followAbort?.abort()
  const controller = new AbortController()
  followAbort = controller
  followBusy.value = true
  followError.value = ''
  followNotice.value = ''
  followNoticeOpen.value = false
  try {
    const result = await requestDirectoryFollow(props.gateway, props.sessionId, action, controller.signal)
    if (controller.signal.aborted) return
    if (result === 'unsupported') throw new Error(t('sftp.ui.directoryFollowUnsupportedShell'))
    followDetected.value = result === 'present'
    if (action === 'configure') {
      followPromptOpen.value = false
      followNotice.value = t('sftp.ui.directoryFollowConfigured')
      followNoticeOpen.value = true
    } else if (result === 'missing' && followDirectory.value && !followPromptDismissed.value)
      followPromptOpen.value = true
  } catch (reason) {
    if (!controller.signal.aborted)
      followError.value = sftpErrorMessage(reason, t('sftp.ui.directoryFollowDetectFailed'))
  } finally {
    if (followAbort === controller) followBusy.value = false
  }
}
function probeFollow(): Promise<void> {
  return runFollow('probe')
}
function configureFollow(): Promise<void> {
  return runFollow('configure')
}

function persistFollowPreference(): void {
  followPreferenceEdited = true
  void setPreference(
    PREFERENCE_KEYS.sftpFollow,
    JSON.stringify({ enabled: remoteFollowEnabled.value, promptDismissed: followPromptDismissed.value }),
  ).catch(() => undefined)
}

async function hydrateFollowPreference(): Promise<void> {
  const cached = readFollowPreference()
  try {
    const raw = await getPreference(PREFERENCE_KEYS.sftpFollow)
    if (!raw) {
      if (!followPreferenceEdited)
        void setPreference(PREFERENCE_KEYS.sftpFollow, JSON.stringify(cached)).catch(() => undefined)
      return
    }
    if (followPreferenceEdited) return
    const value = JSON.parse(raw) as Partial<FollowPreference>
    if (typeof value.enabled !== 'boolean' || typeof value.promptDismissed !== 'boolean') return
    if (!props.directoryFollowEnabled) remoteFollowEnabled.value = value.enabled
    followPromptDismissed.value = value.promptDismissed
    if (cached.enabled !== value.enabled || cached.promptDismissed !== value.promptDismissed) {
      if (
        props.directoryFollowMode === 'remote' &&
        props.autoDirectoryFollow &&
        followDirectory.value &&
        props.available &&
        props.sessionId &&
        !value.promptDismissed
      )
        void probeFollow()
    }
  } catch {}
}

function dismissFollowPrompt(): void {
  if (followBusy.value) return
  followPromptOpen.value = false
  followPromptDismissed.value = true
  remoteFollowEnabled.value = false
  persistFollowPreference()
}

async function toggleDirectoryFollow(): Promise<void> {
  if (props.directoryFollowMode === 'disabled') return
  if (!props.available) return
  if (props.directoryFollowMode === 'session') {
    sessionFollowEnabled.value = !sessionFollowEnabled.value
    setSessionDirectoryFollowEnabled(props.sessionId, sessionFollowEnabled.value)
    if (!sessionFollowEnabled.value) {
      followAbort?.abort()
      followDetected.value = null
      followPromptOpen.value = false
      followNoticeOpen.value = false
      followBusy.value = false
      followError.value = ''
    } else {
      followedDirectory.value = readSessionFollowDirectory(props.sessionId)
      if (followedDirectory.value) void loadDirectory(followedDirectory.value)
    }
    return
  }
  remoteFollowEnabled.value = !followDirectory.value
  persistFollowPreference()
  if (!followDirectory.value) {
    followAbort?.abort()
    followDetected.value = null
    followPromptOpen.value = false
    followNoticeOpen.value = false
    followBusy.value = false
    followError.value = ''
    return
  }
  followPromptDismissed.value = false
  persistFollowPreference()
  await probeFollow()
  if (followedDirectory.value) void loadDirectory(followedDirectory.value)
}

watch(
  () => [props.sessionId, props.available] as const,
  ([sessionId, available]) => {
    sessionFollowEnabled.value = sessionId ? readSessionDirectoryFollowEnabled(sessionId) : true
    const rememberedFollowDirectory = sessionId ? readSessionFollowDirectory(sessionId) : ''
    const rememberedBrowserDirectory = sessionId ? readSessionBrowserDirectory(sessionId) : ''

    const configuredDirectory = props.defaultDirectory?.trim() || ''
    const initialDirectory = configuredDirectory || rememberedBrowserDirectory || rememberedFollowDirectory || '/'
    followAbort?.abort()
    followBusy.value = false
    followPromptOpen.value = false
    followNoticeOpen.value = false
    followError.value = ''
    followNotice.value = ''
    followDetected.value = null
    followedDirectory.value = rememberedFollowDirectory
    sessionGeneration += 1
    requestSequence += 1
    resetForSession()
    selectedEntries.value = []
    createFolderOpen.value = false
    deleteEntryOpen.value = false
    entryBusy.value = false
    directory.value = initialDirectory
    pathDraft.value = initialDirectory
    entries.value = []
    error.value = ''
    loading.value = false
    if (sessionId && available) {
      void loadDirectory(initialDirectory)
      void refreshTransfers()
    }
  },
  { immediate: true },
)

watch(
  () =>
    [
      props.sessionId,
      props.available,
      props.directoryFollowMode,
      props.directoryFollowEnabled,
      props.autoDirectoryFollow,
    ] as const,
  ([sessionId, , directoryFollowMode, directoryFollowEnabled], previous) => {
    if (
      !previous ||
      sessionId !== previous[0] ||
      directoryFollowMode !== previous[2] ||
      directoryFollowEnabled !== previous[3]
    ) {
      sessionFollowEnabled.value = sessionId ? readSessionDirectoryFollowEnabled(sessionId) : true
    }
    followAbort?.abort()
    followBusy.value = false
    followDetected.value = null
    followPromptOpen.value = false
    followNoticeOpen.value = false
    followError.value = ''
    followNotice.value = ''
    remoteFollowEnabled.value = props.directoryFollowEnabled || readFollowPreference().enabled
    if (
      props.directoryFollowMode === 'remote' &&
      props.autoDirectoryFollow &&
      followDirectory.value &&
      !followPromptDismissed.value
    )
      void probeFollow()
  },
  { immediate: true },
)

onMounted(() => {
  window.addEventListener('qterm:current-dir', handleCurrentDirectory)
  void hydrateFollowPreference()
  void hydrateSftpSettings()
})

async function hydrateSftpSettings(): Promise<void> {
  try {
    const settings = await loadSettingsFromPreferences()
    showHidden.value = resolveShowHidden(props.showHiddenFiles, settings.sftpShowHiddenFiles ?? false)
  } catch {}
}

onBeforeUnmount(() => {
  sessionGeneration += 1
  requestSequence += 1
  window.removeEventListener('qterm:current-dir', handleCurrentDirectory)
  followAbort?.abort()
})

async function loadDirectory(value: string): Promise<void> {
  if (!props.available || !props.sessionId) return
  const sessionId = props.sessionId
  const requested = normalizePathDraft(value)
  if (!requested) {
    error.value = t('sftp.ui.inputRemotePath')
    return
  }
  const sequence = ++requestSequence
  loading.value = true
  error.value = ''
  try {
    const result = await props.gateway.listRemoteDirectory(sessionId, requested)
    if (sequence !== requestSequence || sessionId !== props.sessionId) return
    directory.value = requested
    pathDraft.value = requested
    entries.value = result
    selectedEntries.value = []
    rememberSessionBrowserDirectory(sessionId, requested)
  } catch (reason) {
    if (sequence !== requestSequence) return

    error.value = `${requested}：${sftpErrorMessage(reason, t('sftp.ui.readRemoteDirectoryFailed'))}`
  } finally {
    if (sequence === requestSequence) loading.value = false
  }
}

function entryIsDirectory(entry: RemoteFileEntry): boolean {
  return entry.type === 'directory' || (entry.type === 'symlink' && entry.targetType === 'directory')
}

function openEntry(entry: RemoteFileEntry): void {
  if (entryIsDirectory(entry)) void loadDirectory(entry.path)
}

function goUp(): void {
  if (!canGoUp.value) return
  const parent = directory.value.replace(/\/+$/, '').replace(/\/[^/]*$/, '') || '/'
  void loadDirectory(parent)
}

function isCurrentSessionOperation(generation: number, sessionID: string): boolean {
  return generation === sessionGeneration && sessionID === props.sessionId && props.available
}

function normalizePathDraft(value: string): string | null {
  const trimmed = value.trim()
  if (!trimmed.startsWith('/')) return null
  const segments: string[] = []
  for (const segment of trimmed.split('/')) {
    if (!segment || segment === '.') continue
    if (segment === '..') segments.pop()
    else segments.push(segment)
  }
  return `/${segments.join('/')}`
}

const nameDialog = ref({
  open: false,
  action: 'file' as 'file' | 'rename',
  entry: null as RemoteFileEntry | null,
  value: '',
  saving: false,
  error: '',
})
const permissionDialog = ref({
  open: false,
  entry: null as RemoteFileEntry | null,
  mode: '000',
  uid: 0,
  gid: 0,
  users: [] as Array<{ id: number; name: string }>,
  groups: [] as Array<{ id: number; name: string }>,
  principalsLoading: false,
  saving: false,
  error: '',
})

const permissionUserOptions = computed<QtermComboboxOption[]>(() =>
  (permissionDialog.value.users ?? []).map((user) => ({ value: user.id, label: user.name })),
)
const permissionGroupOptions = computed<QtermComboboxOption[]>(() =>
  (permissionDialog.value.groups ?? []).map((group) => ({ value: group.id, label: group.name })),
)
const permissionBits = ref({
  ownerRead: true,
  ownerWrite: true,
  ownerExec: false,
  groupRead: true,
  groupWrite: false,
  groupExec: false,
  otherRead: true,
  otherWrite: false,
  otherExec: false,
})
const infoEntry = ref<RemoteFileEntry | null>(null)

function modeDigitsFromEntry(mode?: string): string {
  const value = mode?.trim() ?? ''
  if (/^0?[0-7]{3}$/.test(value)) return value.slice(-3)
  const symbolic = value.length >= 10 ? value.slice(-9) : value
  if (!/^[rwxstST-]{9}$/.test(symbolic)) return ''
  const digit = (offset: number): number => {
    const group = symbolic.slice(offset, offset + 3)
    return (group.includes('r') ? 4 : 0) + (group.includes('w') ? 2 : 0) + (/[xst]/i.test(group) ? 1 : 0)
  }
  return `${digit(0)}${digit(3)}${digit(6)}`
}

function bitsFromDigits(value: string): typeof permissionBits.value {
  const digits = value
    .replace(/[^0-7]/g, '')
    .padStart(3, '0')
    .slice(-3)
    .split('')
    .map(Number)
  const bits = (digit: number) => ({ read: (digit & 4) !== 0, write: (digit & 2) !== 0, exec: (digit & 1) !== 0 })
  const [owner, group, other] = digits.map(bits)
  return {
    ownerRead: owner?.read ?? false,
    ownerWrite: owner?.write ?? false,
    ownerExec: owner?.exec ?? false,
    groupRead: group?.read ?? false,
    groupWrite: group?.write ?? false,
    groupExec: group?.exec ?? false,
    otherRead: other?.read ?? false,
    otherWrite: other?.write ?? false,
    otherExec: other?.exec ?? false,
  }
}

function syncPermissionMode(): void {
  const b = permissionBits.value
  const digit = (read: boolean, write: boolean, exec: boolean) => (read ? 4 : 0) + (write ? 2 : 0) + (exec ? 1 : 0)
  permissionDialog.value.mode = `${digit(b.ownerRead, b.ownerWrite, b.ownerExec)}${digit(b.groupRead, b.groupWrite, b.groupExec)}${digit(b.otherRead, b.otherWrite, b.otherExec)}`
}

function syncPermissionBits(value: string | number): void {
  const digits = String(value)
    .replace(/[^0-7]/g, '')
    .slice(-4)
  permissionDialog.value.mode = digits
  permissionBits.value = bitsFromDigits(digits || '000')
}

function ensurePrincipal(items: Array<{ id: number; name: string }>, id: number): Array<{ id: number; name: string }> {
  return items.some((item) => item.id === id) ? items : [{ id, name: String(id) }, ...items]
}

function createFile(): void {
  nameDialog.value = { open: true, action: 'file', entry: null, value: '', saving: false, error: '' }
}

function renameBrowserEntry(entry: SftpBrowserEntry): void {
  const source = browserEntrySource(entry)
  if (!source) return
  nameDialog.value = { open: true, action: 'rename', entry: source, value: source.name, saving: false, error: '' }
}

async function submitNameDialog(): Promise<void> {
  const state = nameDialog.value
  const name = state.value.trim()
  if (!name) {
    state.error = t('sftp.ui.enterName')
    return
  }
  state.saving = true
  state.error = ''
  try {
    if (state.action === 'file') {
      if (!props.gateway.createRemoteFile) throw new Error(t('sftp.ui.unsupportedRemoteFile'))
      await props.gateway.createRemoteFile(props.sessionId, directory.value, name)
    } else {
      if (!state.entry || !props.gateway.renameRemoteEntry) throw new Error(t('sftp.ui.unsupportedRemoteRename'))
      await props.gateway.renameRemoteEntry(props.sessionId, state.entry.path, name)
    }
    state.open = false
    await loadDirectory(directory.value)
  } catch (reason) {
    state.error = sftpErrorMessage(reason, t('sftp.ui.operationFailed'))
  } finally {
    state.saving = false
  }
}

async function openPermission(entry: SftpBrowserEntry): Promise<void> {
  const source = browserEntrySource(entry)
  const digits = modeDigitsFromEntry(source?.mode)
  if (!source || !digits) {
    toast.error(t('sftp.ui.permissionInvalid'), { description: t('sftp.ui.permissionMissing') })
    return
  }
  permissionBits.value = bitsFromDigits(digits)
  permissionDialog.value = {
    open: true,
    entry: source,
    mode: digits,
    uid: source.uid ?? 0,
    gid: source.gid ?? 0,
    users: [],
    groups: [],
    principalsLoading: true,
    saving: false,
    error: '',
  }
  try {
    if (!props.gateway.listRemotePrincipals) throw new Error(t('sftp.ui.principalsLoadFailed'))
    const principals = await props.gateway.listRemotePrincipals(props.sessionId)
    if (!permissionDialog.value.open || permissionDialog.value.entry?.path !== source.path) return
    permissionDialog.value.users = ensurePrincipal(principals.users, source.uid ?? 0)
    permissionDialog.value.groups = ensurePrincipal(principals.groups, source.gid ?? 0)
  } catch (reason) {
    permissionDialog.value.error = sftpErrorMessage(reason, t('sftp.ui.principalsLoadFailed'))
    permissionDialog.value.users = ensurePrincipal([], source.uid ?? 0)
    permissionDialog.value.groups = ensurePrincipal([], source.gid ?? 0)
  } finally {
    permissionDialog.value.principalsLoading = false
  }
}

async function submitPermission(): Promise<void> {
  const state = permissionDialog.value
  if (!state.entry) return
  const digits = state.mode.replace(/[^0-7]/g, '')
  if (!/^[0-7]{3,4}$/.test(digits)) {
    state.error = t('sftp.ui.permissionFormat')
    return
  }
  state.saving = true
  state.error = ''
  try {
    if (props.gateway.setRemoteEntryPermissions) {
      await props.gateway.setRemoteEntryPermissions(
        props.sessionId,
        state.entry.path,
        digits.slice(-3),
        state.uid,
        state.gid,
      )
    } else if (props.gateway.chmodRemoteEntry) {
      await props.gateway.chmodRemoteEntry(props.sessionId, state.entry.path, digits.slice(-3))
    } else {
      throw new Error(t('sftp.ui.unsupportedRemotePermissions'))
    }
    state.open = false
    await loadDirectory(directory.value)
  } catch (reason) {
    state.error = sftpErrorMessage(reason, t('sftp.ui.permissionInvalid'))
  } finally {
    state.saving = false
  }
}

async function copyBrowserPath(entry: SftpBrowserEntry): Promise<void> {
  await copyRemotePath(entry.key)
}

async function copyRemotePath(path: string): Promise<void> {
  try {
    await writeClipboardText(path)
    toast.success(t('sftp.ui.copyPathSuccess'), { description: path })
  } catch {
    toast.error(t('sftp.ui.copyPathFailed'), { description: t('sftp.ui.copyPathManual') })
  }
}

function deleteBrowserEntry(entry: SftpBrowserEntry): void {
  deleteElevated.value = false
  deleteElevatedPassword.value = ''
  const source = browserEntrySource(entry)
  if (!source) return
  selectedEntries.value = [source]
  deleteTargets.value = [source]
  entryError.value = ''
  deleteEntryOpen.value = true
}

function showBrowserInfo(entry: SftpBrowserEntry): void {
  infoEntry.value = browserEntrySource(entry) ?? null
}

function downloadBrowserEntry(entry: SftpBrowserEntry): void {
  const source = browserEntrySource(entry)
  if (source) void selectDownload(source)
}

async function archiveDownloadBrowserEntry(entry: SftpBrowserEntry): Promise<void> {
  const source = browserEntrySource(entry)
  const sessionID = props.sessionId
  if (!source || !sessionID || archiveDownloading.value) return
  const canEnqueue = Boolean(props.gateway.enqueueDirectoryArchiveDownload)
  if (!props.gateway.chooseDirectoryPath || (!canEnqueue && !props.gateway.downloadRemoteDirectoryArchive)) {
    toast.error(t('sftp.ui.archiveDownloadUnavailable'))
    return
  }
  let localDirectory: string
  try {
    localDirectory = await props.gateway.chooseDirectoryPath()
  } catch (reason) {
    toast.error(sftpErrorMessage(reason, t('sftp.ui.archiveDownloadFailed')))
    return
  }
  if (!localDirectory) return
  archiveDownloading.value = true
  if (canEnqueue) {
    try {
      await props.gateway.enqueueDirectoryArchiveDownload!(sessionID, source.path, localDirectory)
      toast.success(t('sftp.ui.archiveDownloadQueued', { name: entry.name }))
    } catch (reason) {
      toast.error(t('sftp.ui.archiveDownloadFailed'), {
        description: sftpErrorMessage(reason, t('sftp.ui.archiveDownloadFailed')),
      })
    } finally {
      archiveDownloading.value = false
    }
    return
  }
  const toastID = toast.loading(t('sftp.ui.archiveDownloadRunning', { name: entry.name }))
  try {
    await props.gateway.downloadRemoteDirectoryArchive!(sessionID, source.path, localDirectory)
    toast.success(t('sftp.ui.archiveDownloadDone', { name: entry.name }), { id: toastID })
  } catch (reason) {
    toast.error(t('sftp.ui.archiveDownloadFailed'), {
      id: toastID,
      description: sftpErrorMessage(reason, t('sftp.ui.archiveDownloadFailed')),
    })
  } finally {
    archiveDownloading.value = false
  }
}

function openBrowserEntry(entry: SftpBrowserEntry): void {
  const source = browserEntrySource(entry)
  if (source) openEntry(source)
}
</script>

<template>
  <div class="sftp-panel-shell">
    <SftpBrowserPane
      :segments="browserSegments"
      :entries="browserEntries"
      :loading="loading"
      :error="error"
      :available="available"
      :can-go-up="canGoUp"
      :show-hidden="showHidden"
      :selected-keys="selectedKeys"
      :editable-path="true"
      :path-draft="pathDraft"
      :drop-session-id="props.sessionId"
      :drop-directory="directory"
      :session-id="props.sessionId"
      context-kind="remote"
      :operations-disabled="entryBusy || filePickerBusy"
      :hide-modified-column="props.hideModifiedColumn"
      :unavailable-title="props.preview ? t('sftp.ui.previewSessionTitle') : t('sftp.ui.unavailableTitle')"
      :unavailable-hint="props.preview ? t('sftp.ui.previewSessionDescription') : t('sftp.ui.unavailableHint')"
      @navigate="loadDirectory"
      @open="openBrowserEntry"
      @go-up="goUp"
      @update:show-hidden="(value) => (showHidden = value)"
      @update:path-draft="(value) => (pathDraft = value)"
      @select="selectBrowserEntry"
      @retry="loadDirectory(directory)"
      @create-file="createFile"
      @create-folder="openCreateFolder"
      @refresh="loadDirectory(directory)"
      @upload-file="selectUpload"
      @upload-directory="selectUpload(true)"
      @copy-path="copyBrowserPath"
      @rename="renameBrowserEntry"
      @chmod="openPermission"
      @info="showBrowserInfo"
      @delete-entry="deleteBrowserEntry"
      @download="downloadBrowserEntry"
      @archive-download="(entry) => void archiveDownloadBrowserEntry(entry)"
      @remote-edit-synced="loadDirectory(directory)"
    >
      <template #toolbar>
        <IconButton :label="t('sftp.ui.parentDirectory')" size="icon" :disabled="!canGoUp" @click="goUp"
          ><Undo2 :size="16"
        /></IconButton>
        <RefreshIconButton
          :label="t('sftp.ui.refreshDirectory')"
          size="icon"
          :spinning="loading"
          :disabled="!available || loading"
          icon-class="size-4"
          @click="loadDirectory(directory)"
        />
        <IconButton
          :label="showHidden ? t('sftp.ui.hideHidden') : t('sftp.ui.showHidden')"
          size="icon"
          :active="showHidden"
          :pressed="showHidden"
          @click="toggleHidden"
          ><Eye v-if="showHidden" :size="16" /><EyeOff v-else :size="16"
        /></IconButton>
        <IconButton
          v-if="props.directoryFollowMode !== 'disabled'"
          :label="followDirectory ? t('sftp.ui.directoryFollowCancel') : t('sftp.ui.directoryFollowEnable')"
          size="icon"
          :active="followDirectory"
          :pressed="followDirectory"
          :disabled="!available"
          @click="toggleDirectoryFollow"
          ><LocateFixed :size="16"
        /></IconButton>
        <IconButton
          :label="t('sftp.ui.uploadFile')"
          size="icon"
          :disabled="!available || filePickerBusy"
          :pressed="filePickerBusy"
          :aria-busy="filePickerBusy"
          @click="selectUpload()"
          ><Upload :size="16"
        /></IconButton>
        <IconButton
          :label="t('sftp.ui.uploadFolder')"
          size="icon"
          :disabled="!available || filePickerBusy"
          @click="selectUpload(true)"
          ><FolderUp :size="16"
        /></IconButton>
        <IconButton
          :label="t('sftp.ui.createFolder')"
          size="icon"
          :disabled="!available || entryBusy"
          @click="openCreateFolder"
          ><FolderPlus :size="16"
        /></IconButton>
        <IconButton
          :label="t('sftp.ui.deleteSelected')"
          size="icon"
          :disabled="!available || selectedEntries.length === 0 || entryBusy"
          @click="openDeleteEntry"
          danger
          ><Trash2 :size="16"
        /></IconButton>
      </template>
    </SftpBrowserPane>
    <p v-if="followNotice" role="status">{{ followNotice }}</p>
    <p v-if="followError && !followPromptOpen" class="sftp-transfer-error text-xs" role="alert">{{ followError }}</p>
    <p v-if="transferError" class="sftp-transfer-error text-xs" role="alert">{{ transferError }}</p>
    <TransferProgressPanel
      v-if="showTransferPanel"
      :key="sessionId"
      :items="progressItems"
      @action="handleTransferAction"
      @dismiss="dismissTransfer"
    />

    <QtermDialog :open="createFolderOpen" @update:open="(open) => !entryBusy && (createFolderOpen = open)">
      <QtermDialogContent v-if="createFolderOpen">
        <form :aria-label="t('sftp.ui.createRemoteFolder')" @submit.prevent="mutateEntry('create')">
          <QtermDialogHeader
            :title="t('sftp.ui.createFolder')"
            :description="t('sftp.ui.createFolderDescription', { directory })"
            :close-disabled="entryBusy"
            @close="createFolderOpen = false"
          />
          <div class="dialog-body">
            <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
              <Label for="sftp-folder-name">{{ t('sftp.ui.folderName') }}</Label>
              <QtermInput
                id="sftp-folder-name"
                v-model="folderName"
                name="folderName"
                :aria-label="t('sftp.ui.folderName')"
                autocomplete="off"
                autocorrect="off"
                autocapitalize="off"
                spellcheck="false"
                :disabled="entryBusy"
                :placeholder="t('sftp.ui.folderNamePlaceholder')"
                autofocus
              />
            </div>
            <QtermFormMessage v-if="entryError" variant="error">{{ entryError }}</QtermFormMessage>
          </div>
          <QtermDialogFooter>
            <QtermButton variant="outline" type="button" :disabled="entryBusy" @click="createFolderOpen = false">{{
              t('sftp.ui.cancel')
            }}</QtermButton
            ><QtermButton type="submit" :disabled="entryBusy || !folderName">{{
              t(entryBusy ? 'sftp.ui.creating' : 'sftp.ui.create')
            }}</QtermButton>
          </QtermDialogFooter>
        </form>
      </QtermDialogContent>
    </QtermDialog>
    <DeleteConfirmDialog
      :open="deleteEntryOpen"
      :title="t('sftp.ui.deleteSelected')"
      :message="t('sftp.ui.deleteSelectedMessage', { count: deleteTargets.length })"
      :items="deleteTargets.map((entry) => `「${entry.path}」`)"
      :hint="deleteTargetsIncludeDirectory ? t('sftp.ui.deleteDirectoryWarning') : ''"
      :error="entryError"
      :loading="entryBusy"
      @close="deleteEntryOpen = false"
      @confirm="mutateEntry('delete')"
    >
      <label
        v-if="!deleteTargetsIncludeDirectory"
        class="flex cursor-pointer items-start gap-1.5 text-xs text-qt-muted"
      >
        <input
          v-model="deleteElevated"
          type="checkbox"
          class="mt-0.5 size-3.5 accent-[var(--qt-accent)]"
          data-sftp-delete-elevated
          :disabled="entryBusy"
        />
        <span>
          {{ t('sftp.ui.deleteElevated') }}
          <span class="block text-qt-muted/70">{{ t('sftp.ui.deleteElevatedHint') }}</span>
        </span>
      </label>

      <label v-if="deleteElevated && !deleteTargetsIncludeDirectory" class="flex flex-col gap-1 text-xs text-qt-muted">
        <span>{{ t('sftp.ui.deletePassword') }}</span>
        <input
          v-model="deleteElevatedPassword"
          type="password"
          autocomplete="off"
          autocorrect="off"
          autocapitalize="off"
          spellcheck="false"
          class="h-7 rounded-md border border-qt-border bg-transparent px-2 text-xs text-qt-text outline-none focus:border-qt-accent"
          :placeholder="t('sftp.ui.deletePasswordPlaceholder')"
          :disabled="entryBusy"
          data-sftp-delete-password
        />
        <span class="text-qt-muted/70">{{ t('sftp.ui.deletePasswordHint') }}</span>
      </label>
    </DeleteConfirmDialog>

    <QtermDialog :open="nameDialog.open" @update:open="(open) => !nameDialog.saving && (nameDialog.open = open)">
      <QtermDialogContent v-if="nameDialog.open">
        <form @submit.prevent="submitNameDialog">
          <QtermDialogHeader
            :title="nameDialog.action === 'file' ? t('sftp.ui.createFile') : t('sftp.ui.rename')"
            @close="nameDialog.open = false"
          />
          <div class="dialog-body">
            <FormField :label="t('sftp.ui.info.name')" for-id="sftp-panel-entry-name">
              <QtermInput id="sftp-panel-entry-name" v-model="nameDialog.value" autofocus autocomplete="off" />
            </FormField>
            <QtermFormMessage v-if="nameDialog.error" variant="error">{{ nameDialog.error }}</QtermFormMessage>
          </div>
          <QtermDialogFooter>
            <QtermButton type="button" variant="outline" @click="nameDialog.open = false">{{
              t('sftp.ui.cancel')
            }}</QtermButton>
            <QtermButton type="submit" :disabled="nameDialog.saving">{{ t('sftp.ui.confirm') }}</QtermButton>
          </QtermDialogFooter>
        </form>
      </QtermDialogContent>
    </QtermDialog>

    <QtermDialog
      :open="permissionDialog.open"
      @update:open="(open) => !permissionDialog.saving && (permissionDialog.open = open)"
    >
      <QtermDialogContent v-if="permissionDialog.open" size="lg">
        <form @submit.prevent="submitPermission">
          <QtermDialogHeader :title="t('sftp.ui.permissionDialog')" @close="permissionDialog.open = false" />
          <div class="dialog-body flex-col">
            <div class="grid grid-cols-3 gap-3">
              <FormField :label="t('sftp.ui.permission.user')" for-id="sftp-panel-owner">
                <QtermCombobox
                  id="sftp-panel-owner"
                  v-model="permissionDialog.uid"
                  :options="permissionUserOptions"
                  :disabled="permissionDialog.principalsLoading"
                  :aria-label="t('sftp.ui.permission.user')"
                />
              </FormField>
              <FormField :label="t('sftp.ui.permission.group')" for-id="sftp-panel-group">
                <QtermCombobox
                  id="sftp-panel-group"
                  v-model="permissionDialog.gid"
                  :options="permissionGroupOptions"
                  :disabled="permissionDialog.principalsLoading"
                  :aria-label="t('sftp.ui.permission.group')"
                />
              </FormField>
              <FormField :label="t('sftp.ui.octalPermissions')" for-id="sftp-panel-mode">
                <QtermInput
                  id="sftp-panel-mode"
                  :model-value="permissionDialog.mode"
                  inputmode="numeric"
                  maxlength="4"
                  @update:model-value="syncPermissionBits"
                />
              </FormField>
            </div>
            <div class="text-sm text-muted-foreground">{{ t('sftp.ui.adjustPermissions') }}</div>
            <div class="sftp-permission-grid">
              <span></span><span>{{ t('sftp.ui.permission.read') }}</span
              ><span>{{ t('sftp.ui.permission.write') }}</span
              ><span>{{ t('sftp.ui.permission.execute') }}</span>
              <span class="permission-row-label">{{ t('sftp.ui.permission.user') }}</span>
              <QtermCheckbox v-model="permissionBits.ownerRead" @update:model-value="syncPermissionMode" />
              <QtermCheckbox v-model="permissionBits.ownerWrite" @update:model-value="syncPermissionMode" />
              <QtermCheckbox v-model="permissionBits.ownerExec" @update:model-value="syncPermissionMode" />
              <span class="permission-row-label">{{ t('sftp.ui.permission.group') }}</span>
              <QtermCheckbox v-model="permissionBits.groupRead" @update:model-value="syncPermissionMode" />
              <QtermCheckbox v-model="permissionBits.groupWrite" @update:model-value="syncPermissionMode" />
              <QtermCheckbox v-model="permissionBits.groupExec" @update:model-value="syncPermissionMode" />
              <span class="permission-row-label">{{ t('sftp.ui.permission.other') }}</span>
              <QtermCheckbox v-model="permissionBits.otherRead" @update:model-value="syncPermissionMode" />
              <QtermCheckbox v-model="permissionBits.otherWrite" @update:model-value="syncPermissionMode" />
              <QtermCheckbox v-model="permissionBits.otherExec" @update:model-value="syncPermissionMode" />
            </div>
            <QtermFormMessage v-if="permissionDialog.error" variant="error">{{
              permissionDialog.error
            }}</QtermFormMessage>
          </div>
          <QtermDialogFooter>
            <QtermButton type="button" variant="outline" @click="permissionDialog.open = false">{{
              t('sftp.ui.cancel')
            }}</QtermButton>
            <QtermButton type="submit" :disabled="permissionDialog.saving">{{ t('sftp.ui.save') }}</QtermButton>
          </QtermDialogFooter>
        </form>
      </QtermDialogContent>
    </QtermDialog>

    <SftpFileInfoDialog
      :open="Boolean(infoEntry)"
      :entry="infoEntry"
      :path="infoEntry?.path"
      @update:open="(open) => !open && (infoEntry = null)"
      @copy-path="copyRemotePath"
    />

    <QtermDialog :open="followPromptOpen" @update:open="(open) => !open && dismissFollowPrompt()">
      <QtermDialogContent v-if="followPromptOpen" class="sm:max-w-[440px]" @pointer-down-outside.prevent>
        <QtermDialogHeader :title="t('sftp.ui.directoryFollowEnableTitle')" :show-close="false" />
        <div class="dialog-body">
          <QtermDialogDescription class="break-all">{{ t('sftp.ui.directoryFollowPrompt') }}</QtermDialogDescription>
          <QtermFormMessage v-if="followError" variant="error">{{ followError }}</QtermFormMessage>
        </div>
        <QtermDialogFooter>
          <QtermButton variant="outline" type="button" :disabled="followBusy" @click="dismissFollowPrompt">{{
            t('sftp.ui.cancel')
          }}</QtermButton>
          <QtermButton type="button" :disabled="followBusy" @click="configureFollow">{{
            t('sftp.ui.configureFollow')
          }}</QtermButton>
        </QtermDialogFooter>
      </QtermDialogContent>
    </QtermDialog>

    <QtermDialog :open="followNoticeOpen" @update:open="(open) => (followNoticeOpen = open)">
      <QtermDialogContent v-if="followNoticeOpen" class="sm:max-w-[440px]" @pointer-down-outside.prevent>
        <QtermDialogHeader :title="t('sftp.ui.directoryFollowNoticeTitle')" :show-close="false" />
        <div class="dialog-body">
          <QtermDialogDescription>
            {{ t('sftp.ui.directoryFollowNotice') }}
          </QtermDialogDescription>
        </div>
        <QtermDialogFooter>
          <QtermButton type="button" @click="followNoticeOpen = false">{{ t('sftp.ui.done') }}</QtermButton>
        </QtermDialogFooter>
      </QtermDialogContent>
    </QtermDialog>
  </div>
  <div v-if="legacyOverwriteTransfer" class="qterm-dialog" hidden>{{ legacyOverwriteTransfer.localName }}</div>
</template>

<style scoped>
.sftp-panel-shell {
  display: flex;
  width: 100%;
  height: 100%;
  min-height: 0;
  flex: 1;
  flex-direction: column;
  overflow: hidden;
}

.sftp-panel-shell :deep(.sftp-list-header),
.sftp-panel-shell :deep(.sftp-entry) {
  grid-template-columns: 22px minmax(0, 1fr) var(--qt-sftp-date-column) var(--qt-sftp-size-column);
  gap: 4px;
}

.sftp-panel-shell :deep(.sftp-browser-pane--name-size .sftp-list-header),
.sftp-panel-shell :deep(.sftp-browser-pane--name-size .sftp-entry) {
  grid-template-columns: 22px minmax(0, 1fr) var(--qt-sftp-size-column);
  gap: 4px;
}
.sftp-permission-grid {
  display: grid;
  grid-template-columns: minmax(88px, 1fr) repeat(3, minmax(72px, 1fr));
  align-items: center;
  gap: 12px 8px;
  padding: 2px 0 4px;
  text-align: center;
}
.sftp-permission-grid span {
  color: var(--qt-muted);
}
.sftp-permission-grid .permission-row-label {
  text-align: left;
}
.sftp-permission-grid :deep([data-slot='checkbox']) {
  justify-self: center;
}
.sftp-info-grid {
  padding: 0 16px 16px;
}
.sftp-info-grid > div {
  display: grid;
  grid-template-columns: 100px minmax(0, 1fr);
  align-items: center;
  gap: 12px;
  padding: 10px 0;
  border-bottom: 1px solid color-mix(in srgb, var(--qt-border) 55%, transparent);
}
.sftp-info-grid > div:last-child {
  border-bottom: 0;
}
.sftp-info-grid > div > span:first-child {
  color: var(--qt-muted);
}
.sftp-info-path-value {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 6px;
}
.sftp-info-path-value > span {
  min-width: 0;
  flex: 1;
  overflow-wrap: anywhere;
}
</style>
