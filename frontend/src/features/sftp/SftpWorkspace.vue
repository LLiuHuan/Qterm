<script setup lang="ts">
import { FormField, QtermButton, QtermCheckbox, QtermFormMessage, QtermInput } from '../../components/common'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import DeleteConfirmDialog from '../../components/dialogs/DeleteConfirmDialog.vue'
import { toast } from 'vue-sonner'
import {
  CheckCircle2,
  Copy,
  Eye,
  EyeOff,
  FolderPlus,
  FolderUp,
  Laptop,
  RefreshCw,
  Search,
  Server,
  Trash2,
  Undo2,
  Upload,
} from '@lucide/vue'
import { QtermCombobox, type QtermComboboxOption } from '../../components/common/form-controls'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { ScrollArea } from '../../components/common/control-primitives'
import { Separator } from '../../components/common/control-primitives'
import HostIcon from '../../components/common/icon/HostIcon.vue'
import IconButton from '../../components/common/button/IconButton.vue'
import SftpBrowserPane, { type SftpBrowserEntry, type SftpBrowserSegment } from './SftpBrowserPane.vue'
import type { HostRecord } from '@/services/hosts.ts'
import { sftpErrorMessage, type SFTPTransferRecord, type SftpGateway } from '../../services/sftp'
import { translateErrorKey } from '../../services/errors'
import { writeClipboardText } from '../../services/clipboard'
import { useSftpWorkspacePane, type SftpWorkspacePaneState } from './useSftpWorkspacePane'
import { useSftpWorkspaceTransfers } from './useSftpWorkspaceTransfers'
import { useI18n } from '@/locales'
import { loadSettingsFromPreferences } from '../settings/settingsStore'

import { formatDateTime } from '../../lib/datetime'

const props = defineProps<{
  gateway: SftpGateway
  hosts: HostRecord[]
  initialHostId?: string | null

  transfers?: SFTPTransferRecord[]
}>()
const { t, locale } = useI18n()
const left = useSftpWorkspacePane('local')

const right = useSftpWorkspacePane('remote')
const pickerOpen = ref(false)
const pickerSide = ref<'left' | 'right'>('right')
const search = ref('')
type DeleteTarget = {
  name: string
  path: string

  type?: 'file' | 'directory' | 'symlink' | 'other'
}

type DeletePending = {
  pane: SftpWorkspacePaneState
  kind: 'local' | 'remote'

  localToken: string | null
  sessionId: string | null
  directory: string
  generation: number
  targets: DeleteTarget[]
  items: string[]

  hasDirectory: boolean
  busy: boolean
}

const deletePending = ref<DeletePending | null>(null)
const filteredHosts = computed(() => {
  const q = search.value.trim().toLocaleLowerCase()
  return q ? props.hosts.filter((h) => `${h.name} ${h.address}`.toLocaleLowerCase().includes(q)) : props.hosts
})
const hostFor = (pane: SftpWorkspacePaneState) =>
  computed(() => props.hosts.find((h) => h.id === pane.selectedHostId) ?? null)
const leftHost = hostFor(left)
const rightHost = hostFor(right)
type NameDialogAction = 'file' | 'folder' | 'rename'
const nameDialog = ref<{
  open: boolean
  title: string
  value: string
  action: NameDialogAction
  pane: SftpWorkspacePaneState | null
  entry: SftpBrowserEntry | null
  saving: boolean
  error: string
}>({
  open: false,
  title: '',
  value: '',
  action: 'file',
  pane: null,
  entry: null,
  saving: false,
  error: '',
})
const permissionDialog = ref<{
  open: boolean
  pane: SftpWorkspacePaneState | null
  entry: SftpBrowserEntry | null
  mode: string
  uid: number
  gid: number
  users: Array<{ id: number; name: string }>
  groups: Array<{ id: number; name: string }>
  principalsLoading: boolean
  saving: boolean
  error: string
}>({
  open: false,
  pane: null,
  entry: null,
  mode: '000',
  uid: 0,
  gid: 0,
  users: [],
  groups: [],
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
const infoDialog = ref<{ open: boolean; pane: SftpWorkspacePaneState | null; entry: SftpBrowserEntry | null }>({
  open: false,
  pane: null,
  entry: null,
})

type PermissionBits = typeof permissionBits.value

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

function bitsFromDigits(value: string): PermissionBits {
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

function modeFromBits(bits: PermissionBits): string {
  const digit = (read: boolean, write: boolean, exec: boolean) => (read ? 4 : 0) + (write ? 2 : 0) + (exec ? 1 : 0)
  return `${digit(bits.ownerRead, bits.ownerWrite, bits.ownerExec)}${digit(bits.groupRead, bits.groupWrite, bits.groupExec)}${digit(bits.otherRead, bits.otherWrite, bits.otherExec)}`
}

function paneTitle(pane: SftpWorkspacePaneState): string {
  if (pane.kind === 'local') return t('sftp.ui.currentDevice')
  return props.hosts.find((h) => h.id === pane.selectedHostId)?.name ?? t('sftp.ui.selectHost')
}
function paneStatus(pane: SftpWorkspacePaneState): string {
  if (pane.kind === 'local') return t('sftp.ui.connected')
  if (pane.connecting) return t('sftp.ui.connecting')
  return pane.selectedSessionId ? t('sftp.ui.connected') : t('sftp.ui.disconnected')
}
function paneUnavailableTitle(pane: SftpWorkspacePaneState): string {
  if (pane.connectError) return t('sftp.ui.connectionFailedToast')
  return pane.selectedHostId ? t('sftp.ui.connecting') : t('sftp.ui.noHostSelected')
}
function paneUnavailableHint(pane: SftpWorkspacePaneState): string {
  if (pane.connectError) return pane.connectError
  return pane.selectedHostId ? t('sftp.ui.connectionReady') : t('sftp.ui.selectHostHint')
}
const localEntries = (pane: SftpWorkspacePaneState) =>
  computed<SftpBrowserEntry[]>(() =>
    (pane.localDirectory?.entries ?? [])
      .filter((e) => pane.localShowHidden || !e.name.startsWith('.'))
      .map((e) => ({
        key: e.name,
        name: e.name,
        type: e.type,
        symlink: e.isSymlink === true,
        mode: e.mode,
        modTime: e.modTime,
        size: e.size,
        sizeText: e.type === 'directory' ? t('sftp.ui.directoryName') : e.sizeText || '—',
      })),
  )
const remoteEntries = (pane: SftpWorkspacePaneState) =>
  computed<SftpBrowserEntry[]>(() =>
    pane.remoteEntries
      .filter((e) => pane.remoteShowHidden || !e.name.startsWith('.'))
      .map((e) => ({
        key: e.path,
        name: e.name,

        type:
          e.type === 'symlink'
            ? e.targetType === 'directory' || e.targetType === 'file'
              ? e.targetType
              : 'other'
            : e.type,
        symlink: e.type === 'symlink',
        mode: e.mode,
        uid: e.uid,
        gid: e.gid,
        modTime: e.modTime,
        size: e.size,
        sizeText: e.type === 'directory' ? t('sftp.ui.directoryName') : e.sizeText || '—',
      })),
  )
const leftLocalEntries = localEntries(left)
const leftRemoteEntries = remoteEntries(left)
const rightRemoteEntries = remoteEntries(right)
const rightLocalEntries = localEntries(right)
const segments = (pane: SftpWorkspacePaneState) =>
  computed<SftpBrowserSegment[]>(() => {
    if (pane.kind === 'local')
      return (pane.localDirectory?.segments ?? []).map((s, i) => ({
        label: i === 0 ? t('sftp.ui.currentDevice') : s.name,
        path: s.token,
        title: s.name,
      }))
    const parts = pane.remoteDirectory.split('/').filter(Boolean)
    return [
      { label: '/', path: '/' },
      ...parts.map((p, i) => ({ label: p, path: `/${parts.slice(0, i + 1).join('/')}` })),
    ]
  })
const leftSegments = segments(left)
const rightSegments = segments(right)

function openNameDialog(pane: SftpWorkspacePaneState, action: NameDialogAction, entry?: SftpBrowserEntry): void {
  nameDialog.value = {
    open: true,
    title:
      action === 'file'
        ? t('sftp.ui.createFile')
        : action === 'folder'
          ? t('sftp.ui.createFolder')
          : t('sftp.ui.rename'),
    value: entry?.name ?? '',
    action,
    pane,
    entry: entry ?? null,
    saving: false,
    error: '',
  }
}
function createFile(pane: SftpWorkspacePaneState): void {
  openNameDialog(pane, 'file')
}
function createFolderForPane(pane: SftpWorkspacePaneState): void {
  openNameDialog(pane, 'folder')
}
async function submitNameDialog(): Promise<void> {
  const state = nameDialog.value
  const pane = state.pane
  const name = state.value.trim()
  if (!pane || !name) {
    state.error = t('sftp.ui.enterName')
    return
  }
  state.saving = true
  state.error = ''
  try {
    if (state.action === 'file') {
      if (pane.kind === 'local') {
        if (!pane.localDirectory || !props.gateway.createLocalFile) throw new Error(t('sftp.ui.unsupportedLocalFile'))
        await props.gateway.createLocalFile(pane.localDirectory.token, name)
      } else {
        if (!pane.selectedSessionId || !props.gateway.createRemoteFile)
          throw new Error(t('sftp.ui.unsupportedRemoteFile'))
        await props.gateway.createRemoteFile(pane.selectedSessionId, pane.remoteDirectory, name)
      }
    } else if (state.action === 'folder') {
      if (pane.kind === 'local') {
        if (!pane.localDirectory || !props.gateway.createLocalDirectory)
          throw new Error(t('sftp.ui.unsupportedLocalFolder'))
        await props.gateway.createLocalDirectory(pane.localDirectory.token, name)
      } else {
        if (!pane.selectedSessionId) throw new Error(t('sftp.ui.remoteNotConnected'))
        await props.gateway.createRemoteDirectory(pane.selectedSessionId, pane.remoteDirectory, name)
      }
    } else {
      if (!state.entry) throw new Error(t('sftp.ui.renameMissingEntry'))
      if (pane.kind === 'local') {
        if (!pane.localDirectory || !props.gateway.renameLocalEntry)
          throw new Error(t('sftp.ui.unsupportedLocalRename'))
        await props.gateway.renameLocalEntry(pane.localDirectory.token, state.entry.name, name)
      } else {
        if (!pane.selectedSessionId || !props.gateway.renameRemoteEntry)
          throw new Error(t('sftp.ui.unsupportedRemoteRename'))
        await props.gateway.renameRemoteEntry(pane.selectedSessionId, state.entry.key, name)
      }
    }
    state.open = false
    refresh(pane)
  } catch (error) {
    state.error = sftpErrorMessage(error, t('sftp.ui.operationFailed'))
  } finally {
    state.saving = false
  }
}
async function renameEntry(pane: SftpWorkspacePaneState, entry: SftpBrowserEntry): Promise<void> {
  openNameDialog(pane, 'rename', entry)
}
async function chmodEntry(pane: SftpWorkspacePaneState, entry: SftpBrowserEntry): Promise<void> {
  const digits = modeDigitsFromEntry(entry.mode)
  if (!digits) {
    toast.error(t('sftp.ui.permissionInvalid'), { description: t('sftp.ui.permissionMissing') })
    return
  }
  permissionBits.value = bitsFromDigits(digits)
  permissionDialog.value = {
    open: true,
    pane,
    entry,
    mode: digits,
    uid: entry.uid ?? 0,
    gid: entry.gid ?? 0,
    users: [],
    groups: [],
    principalsLoading: pane.kind === 'remote',
    saving: false,
    error: '',
  }
  if (pane.kind !== 'remote' || !pane.selectedSessionId || !props.gateway.listRemotePrincipals) return
  try {
    const principals = await props.gateway.listRemotePrincipals(pane.selectedSessionId)
    if (!permissionDialog.value.open || permissionDialog.value.entry?.key !== entry.key) return
    permissionDialog.value.users = ensurePrincipal(principals.users, entry.uid ?? 0)
    permissionDialog.value.groups = ensurePrincipal(principals.groups, entry.gid ?? 0)
  } catch (error) {
    permissionDialog.value.error = sftpErrorMessage(error, t('sftp.ui.principalsLoadFailed'))
    permissionDialog.value.users = ensurePrincipal([], entry.uid ?? 0)
    permissionDialog.value.groups = ensurePrincipal([], entry.gid ?? 0)
  } finally {
    permissionDialog.value.principalsLoading = false
  }
}

function ensurePrincipal(items: Array<{ id: number; name: string }>, id: number): Array<{ id: number; name: string }> {
  return items.some((item) => item.id === id) ? items : [{ id, name: String(id) }, ...items]
}
async function submitPermissionDialog(): Promise<void> {
  const state = permissionDialog.value
  const pane = state.pane
  const entry = state.entry
  if (!pane || !entry) return
  state.saving = true
  state.error = ''
  try {
    const digits = state.mode.replace(/[^0-7]/g, '')
    if (!/^[0-7]{3,4}$/.test(digits)) {
      state.error = t('sftp.ui.permissionFormat')
      return
    }
    state.mode = digits.slice(-3)
    if (pane.kind === 'local') {
      if (!pane.localDirectory || !props.gateway.chmodLocalEntry)
        throw new Error(t('sftp.ui.unsupportedLocalPermissions'))
      await props.gateway.chmodLocalEntry(pane.localDirectory.token, entry.name, state.mode.trim())
    } else {
      if (!pane.selectedSessionId) throw new Error(t('sftp.ui.unsupportedRemotePermissions'))
      if (props.gateway.setRemoteEntryPermissions) {
        await props.gateway.setRemoteEntryPermissions(
          pane.selectedSessionId,
          entry.key,
          state.mode.trim(),
          state.uid,
          state.gid,
        )
      } else if (props.gateway.chmodRemoteEntry) {
        await props.gateway.chmodRemoteEntry(pane.selectedSessionId, entry.key, state.mode.trim())
      } else {
        throw new Error(t('sftp.ui.unsupportedRemotePermissions'))
      }
    }
    state.open = false
    refresh(pane)
  } catch (error) {
    state.error = sftpErrorMessage(error, t('sftp.ui.permissionInvalid'))
  } finally {
    state.saving = false
  }
}
function syncPermissionMode(): void {
  permissionDialog.value.mode = modeFromBits(permissionBits.value)
}

function syncPermissionBits(value: string | number): void {
  const digits = String(value)
    .replace(/[^0-7]/g, '')
    .slice(-4)
  permissionDialog.value.mode = digits
  permissionBits.value = bitsFromDigits(digits || '000')
}

function entryDisplayPath(pane: SftpWorkspacePaneState | null, entry: SftpBrowserEntry | null): string {
  if (!pane || !entry) return ''
  if (pane.kind === 'remote') return entry.key
  const base = pane.localDirectory?.displayPath?.replace(/\/+$/, '') ?? ''
  if (!base) return entry.name
  return base === '/' ? `/${entry.name}` : `${base}/${entry.name}`
}

function formatInfoDate(value: string | undefined): string {
  return formatDateTime(value, { placeholder: t('sftp.ui.info.unknown') })
}

function formatInfoSize(entry: SftpBrowserEntry | null): string {
  if (!entry) return t('sftp.ui.info.unknown')
  return `${entry.sizeText || t('sftp.ui.info.unknown')}（${new Intl.NumberFormat(locale.value).format(entry.size)} ${t('sftp.ui.bytes')}）`
}

function entryTypeLabel(entry: SftpBrowserEntry | null): string {
  switch (entry?.type) {
    case 'directory':
      return t('sftp.ui.folder')
    case 'symlink':
      return t('sftp.ui.info.symlink')
    case 'other':
      return t('sftp.ui.permissionOther')
    case 'file':
      return t('sftp.ui.file')
    default:
      return t('sftp.ui.info.unknown')
  }
}

function requestDelete(pane: SftpWorkspacePaneState, targets: DeleteTarget[]): void {
  if (targets.length === 0) return
  const localDirectory = pane.kind === 'local' ? pane.localDirectory : null
  const sessionId = pane.kind === 'remote' ? pane.selectedSessionId : null
  if (pane.kind === 'local' && !localDirectory) return
  if (pane.kind === 'remote' && !sessionId) return
  deletePending.value = {
    pane,
    kind: pane.kind,
    localToken: localDirectory?.token ?? null,
    sessionId,
    directory: pane.kind === 'remote' ? pane.remoteDirectory : (localDirectory?.displayPath ?? ''),
    generation: pane.generation,
    targets,
    items: targets.map((target) => target.path),
    hasDirectory: targets.some((target) => target.type === 'directory'),
    busy: false,
  }
}

function deleteEntry(pane: SftpWorkspacePaneState, entry: SftpBrowserEntry): void {
  requestDelete(pane, [{ name: entry.name, path: entryDisplayPath(pane, entry), type: entry.type }])
}
async function copyEntryPath(pane: SftpWorkspacePaneState, entry: SftpBrowserEntry): Promise<void> {
  const value = entryDisplayPath(pane, entry)
  try {
    await writeClipboardText(value)
    toast.success(t('sftp.ui.copyPathSuccess'), { description: value })
  } catch {
    toast.error(t('sftp.ui.copyPathFailed'), { description: t('sftp.ui.copyPathManual') })
  }
}

function notifyUnsupportedEntry(entry: SftpBrowserEntry): void {
  toast.info(t('sftp.ui.openEntryFailed'), {
    description: `${entry.name}：${translateErrorKey('sftp.entry_not_directory', t('sftp.ui.entryNotDirectory'))}`,
  })
}

function showEntryInfo(pane: SftpWorkspacePaneState, entry: SftpBrowserEntry): void {
  infoDialog.value = { open: true, pane, entry }
}

async function loadLocal(pane: SftpWorkspacePaneState, token = '', child = '') {
  const loader = props.gateway.openLocalDirectory
  const seq = ++pane.localRequestSequence
  pane.localLoading = true
  pane.localError = ''
  if (!loader) {
    pane.localError = t('sftp.ui.localBrowseUnavailable')
    pane.localLoading = false
    return
  }
  try {
    const result = await loader(token, child)
    if (!pane.disposed && seq === pane.localRequestSequence) {
      if (pane.localDirectory?.token !== result.token) pane.localSelectedNames = []
      pane.localDirectory = result

      pane.pathDraft = result.displayPath || result.token
    }
  } catch (e) {
    if (!pane.disposed && seq === pane.localRequestSequence) {
      const target = child
        ? `${pane.localDirectory?.displayPath ?? ''}/${child}`
        : (pane.localDirectory?.displayPath ?? '')
      pane.localError = target
        ? `${target}：${sftpErrorMessage(e, t('sftp.ui.readLocalDirectoryFailed'))}`
        : sftpErrorMessage(e, t('sftp.ui.readLocalDirectoryFailed'))
    }
  } finally {
    if (!pane.disposed && seq === pane.localRequestSequence) pane.localLoading = false
  }
}
async function loadRemote(pane: SftpWorkspacePaneState, dir = '/', sessionId = pane.selectedSessionId ?? '') {
  if (!sessionId) return
  const seq = ++pane.remoteRequestSequence
  const gen = pane.generation
  pane.remoteLoading = true
  pane.remoteError = ''
  try {
    const result = await props.gateway.listRemoteDirectory(sessionId, dir)
    if (
      pane.disposed ||
      gen !== pane.generation ||
      seq !== pane.remoteRequestSequence ||
      pane.selectedSessionId !== sessionId
    )
      return
    const resolvedDirectory = result.find((entry) => entry.directory)?.directory || dir

    if (pane.remoteDirectory !== resolvedDirectory) pane.remoteSelectedPaths = []
    pane.remoteDirectory = resolvedDirectory
    pane.pathDraft = resolvedDirectory
    pane.remoteEntries = result
  } catch (e) {
    if (!pane.disposed && gen === pane.generation && seq === pane.remoteRequestSequence) {
      pane.remoteError = `${dir}：${sftpErrorMessage(e, t('sftp.ui.readRemoteDirectoryFailed'))}`
    }
  } finally {
    if (!pane.disposed && seq === pane.remoteRequestSequence) pane.remoteLoading = false
  }
}
async function closePane(pane: SftpWorkspacePaneState) {
  const id = pane.selectedSessionId
  pane.generation++
  pane.selectedSessionId = null
  pane.selectedHostId = null
  pane.remoteEntries = []
  pane.remoteDirectory = '/'
  pane.remoteSelectedPaths = []
  pane.remoteDirectoryFollow = false
  pane.localSelectedNames = []
  if (!id) return
  try {
    await props.gateway.closeConnection?.(id)
  } catch {}
}

function hostSftpPreferences(hostId: string): { showHidden: 'inherit' | 'enabled' | 'disabled'; directory: string } {
  const host = props.hosts?.find((item) => item.id === hostId)
  return {
    showHidden: host?.sftpShowHiddenFiles || 'inherit',
    directory: host?.sftpDefaultDirectory?.trim() || '',
  }
}

function resolveShowHidden(override: 'inherit' | 'enabled' | 'disabled', globalValue: boolean): boolean {
  if (override === 'enabled') return true
  if (override === 'disabled') return false
  return globalValue
}

async function connect(pane: SftpWorkspacePaneState, hostId: string) {
  await closePane(pane)
  pane.kind = 'remote'
  const open = props.gateway.openConnection
  if (!open) {
    pane.connectError = t('sftp.ui.independentConnectionUnavailable')
    toast.error(pane.connectError)
    return
  }
  const gen = ++pane.generation
  pane.selectedHostId = hostId
  pane.connecting = true
  pane.connectError = ''
  try {
    const id = await open(hostId)
    if (pane.disposed || gen !== pane.generation) {
      await props.gateway.closeConnection?.(id)
      return
    }
    pane.selectedSessionId = id

    const preferences = hostSftpPreferences(hostId)
    if (preferences.showHidden !== 'inherit') {
      const globalValue = pane.remoteShowHidden
      pane.remoteShowHidden = resolveShowHidden(preferences.showHidden, globalValue)
    }
    await loadRemote(pane, preferences.directory || '/', id)
  } catch (e) {
    if (!pane.disposed && gen === pane.generation) {
      pane.connectError = sftpErrorMessage(e, t('sftp.ui.connectionFailedToast'))
      toast.error(t('sftp.ui.connectionFailedToast'), { description: pane.connectError })
    }
  } finally {
    if (!pane.disposed && gen === pane.generation) pane.connecting = false
  }
}
function openPicker(side: 'left' | 'right') {
  pickerSide.value = side
  search.value = ''
  pickerOpen.value = true
}
function chooseHost(id: string) {
  pickerOpen.value = false
  void connect(pickerSide.value === 'left' ? left : right, id)
}
function chooseLocal(): void {
  pickerOpen.value = false
  const pane = pickerSide.value === 'left' ? left : right
  void (async () => {
    await closePane(pane)
    pane.kind = 'local'
    await loadLocal(pane)
  })()
}
function goUp(pane: SftpWorkspacePaneState): void {
  if (pane.kind === 'local') {
    if (pane.localDirectory?.parentToken) void loadLocal(pane, pane.localDirectory.parentToken)
    return
  }
  if (pane.remoteDirectory === '/') return
  const parts = pane.remoteDirectory.split('/').filter(Boolean)
  void loadRemote(pane, parts.length > 1 ? `/${parts.slice(0, -1).join('/')}` : '/')
}
function refresh(pane: SftpWorkspacePaneState): void {
  if (pane.kind === 'local') void loadLocal(pane, pane.localDirectory?.token)
  else void loadRemote(pane, pane.remoteDirectory)
}

function deleteSelected(pane: SftpWorkspacePaneState): void {
  if (pane.kind === 'local') {
    const directory = pane.localDirectory
    if (!directory || pane.localSelectedNames.length === 0) return
    requestDelete(
      pane,
      pane.localSelectedNames.map((name) => {
        const source = directory.entries.find((entry) => entry.name === name)
        return {
          name,
          path: entryDisplayPath(pane, { key: name, name, type: 'other', modTime: '', size: 0, sizeText: '' }),

          type: source && !source.isSymlink ? source.type : 'other',
        }
      }),
    )
    return
  }
  if (!pane.selectedSessionId || pane.remoteSelectedPaths.length === 0) return

  requestDelete(
    pane,
    pane.remoteSelectedPaths.map((path) => ({
      name: path.split('/').pop() || path,
      path,

      type: pane.remoteEntries.find((entry) => entry.path === path)?.type,
    })),
  )
}
async function confirmDelete(): Promise<void> {
  const pending = deletePending.value
  if (!pending || pending.busy) return
  pending.busy = true
  const { pane, kind, localToken, sessionId, directory, generation, targets, items } = pending
  try {
    let failed = 0
    for (const target of targets) {
      try {
        if (kind === 'local') {
          if (!localToken || !props.gateway.deleteLocalEntry) throw new Error(t('sftp.ui.unsupportedLocalDelete'))
          await props.gateway.deleteLocalEntry(localToken, target.name)
        } else {
          if (!sessionId) throw new Error(t('sftp.ui.remoteNotConnected'))
          await props.gateway.deleteRemoteEntry(sessionId, target.path)
        }
      } catch {
        failed += 1
      }
    }

    const stillAtSnapshot =
      generation === pane.generation &&
      (kind === 'local'
        ? pane.localDirectory?.token === localToken
        : pane.selectedSessionId === sessionId && pane.remoteDirectory === directory)

    if (stillAtSnapshot) {
      if (kind === 'local') pane.localSelectedNames = []
      else pane.remoteSelectedPaths = []
      refresh(pane)
    }
    if (failed > 0) {
      const label = kind === 'local' ? t('sftp.ui.local') : t('sftp.ui.remote')
      toast.error(t('sftp.ui.partialDeleteFailed', { kind: label }), {
        description: t('sftp.ui.deleteFailedCount', { failed, total: items.length }),
      })
    }
  } finally {
    deletePending.value = null
  }
}
function cancelDelete(): void {
  deletePending.value = null
}
const { downloadEntry, uploadFile, uploadFolder, startDrag, drop } = useSftpWorkspaceTransfers({
  gateway: props.gateway,
  left,
  right,
  transfers: () => props.transfers ?? [],
  t,
  toast,
  refresh,
})
onMounted(() => {
  void loadLocal(left)
  if (props.initialHostId) void connect(right, props.initialHostId)
  void hydrateSftpSettings()
})

async function hydrateSftpSettings(): Promise<void> {
  try {
    const settings = await loadSettingsFromPreferences()
    const showHidden = settings.sftpShowHiddenFiles ?? false
    left.localShowHidden = showHidden
    left.remoteShowHidden = showHidden
    right.localShowHidden = showHidden
    right.remoteShowHidden = showHidden

    const override = props.initialHostId ? hostSftpPreferences(props.initialHostId).showHidden : 'inherit'
    if (override !== 'inherit') right.remoteShowHidden = resolveShowHidden(override, showHidden)
  } catch {}
}
onBeforeUnmount(() => {
  left.disposed = true
  right.disposed = true
  void closePane(left)
  void closePane(right)
})
</script>

<template>
  <div class="sftp-workspace" :aria-label="t('sftp.ui.sftpTransfer')">
    <div class="sftp-workspace-panes">
      <section class="sftp-workspace-pane" :aria-label="t('sftp.ui.leftSftpPanel')">
        <header class="sftp-workspace-pane-header">
          <button class="sftp-workspace-pane-title is-switchable" type="button" @click="openPicker('left')">
            <HostIcon v-if="left.kind === 'remote' && leftHost" :icon="leftHost.icon" :size="18" />
            <Server v-else-if="left.kind === 'remote'" :size="18" aria-hidden="true" />
            <Laptop v-else :size="18" aria-hidden="true" />
            <span>{{ paneTitle(left) }}</span>
          </button>
          <span class="sftp-workspace-pane-status text-xs">{{ paneStatus(left) }}</span>
        </header>
        <SftpBrowserPane
          v-if="left.kind === 'local'"
          :segments="leftSegments"
          :entries="leftLocalEntries"
          :loading="left.localLoading"
          :error="left.localError"
          :available="Boolean(props.gateway.openLocalDirectory)"
          :can-go-up="Boolean(left.localDirectory?.parentToken)"
          :show-hidden="left.localShowHidden"
          :selected-keys="left.localSelectedNames"
          :editable-path="false"
          :read-only-path="left.localDirectory?.displayPath"
          :draggable-entries="true"
          context-kind="local"
          :operations-disabled="deletePending !== null"
          v-model:path-draft="left.pathDraft"
          @open="(e) => void loadLocal(left, left.localDirectory?.token, e.name)"
          @unsupported-entry="notifyUnsupportedEntry"
          @navigate="(p) => void loadLocal(left, p === '__root__' ? '' : p)"
          @retry="refresh(left)"
          @update:show-hidden="left.localShowHidden = $event"
          @select="
            (entry, checked) =>
              (left.localSelectedNames = checked
                ? [...left.localSelectedNames, entry.name]
                : left.localSelectedNames.filter((name) => name !== entry.name))
          "
          @drag-start="(e) => startDrag('left', e)"
          @drop-on-entry="(e) => void drop('left', e)"
          @drop-on-directory="() => void drop('left')"
          @create-file="createFile(left)"
          @create-folder="createFolderForPane(left)"
          @refresh="refresh(left)"
          @copy-path="copyEntryPath(left, $event)"
          @rename="renameEntry(left, $event)"
          @chmod="chmodEntry(left, $event)"
          @info="showEntryInfo(left, $event)"
          @delete-entry="deleteEntry(left, $event)"
        >
          <template #toolbar>
            <IconButton
              :label="t('sftp.ui.parentDirectory')"
              size="icon-sm"
              :disabled="!left.localDirectory?.parentToken"
              @click="goUp(left)"
              ><Undo2 :size="15"
            /></IconButton>
            <IconButton
              :label="t('sftp.ui.refreshDirectory')"
              size="icon-sm"
              :disabled="left.localLoading"
              @click="refresh(left)"
              ><RefreshCw :class="{ spin: left.localLoading }" :size="15"
            /></IconButton>
            <IconButton
              :label="left.localShowHidden ? t('sftp.ui.hideHidden') : t('sftp.ui.showHidden')"
              size="icon-sm"
              :active="left.localShowHidden"
              @click="left.localShowHidden = !left.localShowHidden"
              ><Eye v-if="left.localShowHidden" :size="15" /><EyeOff v-else :size="15"
            /></IconButton>
            <IconButton
              :label="t('sftp.ui.createFolder')"
              size="icon-sm"
              :disabled="!left.localDirectory"
              @click="createFolderForPane(left)"
              ><FolderPlus :size="15"
            /></IconButton>
            <IconButton
              :label="t('sftp.ui.deleteSelected')"
              size="icon-sm"
              :disabled="deletePending !== null || !left.localDirectory || left.localSelectedNames.length === 0"
              @click="deleteSelected(left)"
              danger
              ><Trash2 :size="15"
            /></IconButton>
          </template>
        </SftpBrowserPane>
        <SftpBrowserPane
          v-else
          :segments="leftSegments"
          :entries="leftRemoteEntries"
          :loading="left.remoteLoading"
          :error="left.connectError || left.remoteError"
          :unavailable-title="paneUnavailableTitle(left)"
          :unavailable-hint="paneUnavailableHint(left)"
          :available="Boolean(left.selectedSessionId)"
          :drop-session-id="left.selectedSessionId ?? undefined"
          :drop-directory="left.remoteDirectory"
          :session-id="left.selectedSessionId ?? ''"
          :can-go-up="left.remoteDirectory !== '/'"
          :show-hidden="left.remoteShowHidden"
          :selected-keys="left.remoteSelectedPaths"
          :editable-path="true"
          :draggable-entries="true"
          context-kind="remote"
          :operations-disabled="deletePending !== null"
          v-model:path-draft="left.pathDraft"
          @open="(e) => void loadRemote(left, e.key)"
          @unsupported-entry="notifyUnsupportedEntry"
          @navigate="(p) => void loadRemote(left, p)"
          @retry="refresh(left)"
          @update:show-hidden="left.remoteShowHidden = $event"
          @select="
            (entry, checked) =>
              (left.remoteSelectedPaths = checked
                ? [...left.remoteSelectedPaths, entry.key]
                : left.remoteSelectedPaths.filter((path) => path !== entry.key))
          "
          @drag-start="(e) => startDrag('left', e)"
          @drop-on-entry="(e) => void drop('left', e)"
          @drop-on-directory="() => void drop('left')"
          @create-file="createFile(left)"
          @create-folder="createFolderForPane(left)"
          @refresh="refresh(left)"
          @upload-file="uploadFile(left)"
          @upload-directory="uploadFolder(left)"
          @copy-path="copyEntryPath(left, $event)"
          @rename="renameEntry(left, $event)"
          @chmod="chmodEntry(left, $event)"
          @info="showEntryInfo(left, $event)"
          @delete-entry="deleteEntry(left, $event)"
          @download="(e) => void downloadEntry(left, e)"
          @remote-edit-synced="refresh(left)"
        >
          <template #toolbar>
            <IconButton
              :label="t('sftp.ui.parentDirectory')"
              size="icon-sm"
              :disabled="left.remoteDirectory === '/'"
              @click="goUp(left)"
              ><Undo2 :size="15"
            /></IconButton>
            <IconButton
              :label="t('sftp.ui.refreshDirectory')"
              size="icon-sm"
              :disabled="left.remoteLoading"
              @click="refresh(left)"
              ><RefreshCw :class="{ spin: left.remoteLoading }" :size="15"
            /></IconButton>
            <IconButton
              :label="left.remoteShowHidden ? t('sftp.ui.hideHidden') : t('sftp.ui.showHidden')"
              size="icon-sm"
              :active="left.remoteShowHidden"
              @click="left.remoteShowHidden = !left.remoteShowHidden"
              ><Eye v-if="left.remoteShowHidden" :size="15" /><EyeOff v-else :size="15"
            /></IconButton>
            <IconButton
              :label="t('sftp.ui.uploadFile')"
              size="icon-sm"
              :disabled="!left.selectedSessionId"
              @click="uploadFile(left)"
              ><Upload :size="15"
            /></IconButton>
            <IconButton
              :label="t('sftp.ui.uploadFolder')"
              size="icon-sm"
              :disabled="!left.selectedSessionId"
              @click="uploadFolder(left)"
              ><FolderUp :size="15"
            /></IconButton>
            <IconButton
              :label="t('sftp.ui.createFolder')"
              size="icon-sm"
              :disabled="!left.selectedSessionId"
              @click="createFolderForPane(left)"
              ><FolderPlus :size="15"
            /></IconButton>
            <IconButton
              :label="t('sftp.ui.deleteSelected')"
              size="icon-sm"
              :disabled="deletePending !== null || !left.selectedSessionId || left.remoteSelectedPaths.length === 0"
              @click="deleteSelected(left)"
              danger
              ><Trash2 :size="15"
            /></IconButton>
          </template>
        </SftpBrowserPane>
      </section>
      <section class="sftp-workspace-pane" :aria-label="t('sftp.ui.rightSftpPanel')">
        <header class="sftp-workspace-pane-header">
          <button class="sftp-workspace-pane-title is-switchable" type="button" @click="openPicker('right')">
            <HostIcon v-if="right.kind === 'remote' && rightHost" :icon="rightHost.icon" :size="18" />
            <Server v-else-if="right.kind === 'remote'" :size="18" aria-hidden="true" />
            <Laptop v-else :size="18" aria-hidden="true" />
            <span>{{ paneTitle(right) }}</span>
          </button>
          <span class="sftp-workspace-pane-status text-xs">{{ paneStatus(right) }}</span>
        </header>
        <SftpBrowserPane
          v-if="right.kind === 'local'"
          :segments="rightSegments"
          :entries="rightLocalEntries"
          :loading="right.localLoading"
          :error="right.localError"
          :available="Boolean(props.gateway.openLocalDirectory)"
          :can-go-up="Boolean(right.localDirectory?.parentToken)"
          :show-hidden="right.localShowHidden"
          :selected-keys="right.localSelectedNames"
          :editable-path="false"
          :read-only-path="right.localDirectory?.displayPath"
          :draggable-entries="true"
          context-kind="local"
          :operations-disabled="deletePending !== null"
          v-model:path-draft="right.pathDraft"
          @open="(e) => void loadLocal(right, right.localDirectory?.token, e.name)"
          @unsupported-entry="notifyUnsupportedEntry"
          @navigate="(p) => void loadLocal(right, p === '__root__' ? '' : p)"
          @retry="refresh(right)"
          @update:show-hidden="right.localShowHidden = $event"
          @select="
            (entry, checked) =>
              (right.localSelectedNames = checked
                ? [...right.localSelectedNames, entry.name]
                : right.localSelectedNames.filter((name) => name !== entry.name))
          "
          @drag-start="(e) => startDrag('right', e)"
          @drop-on-entry="(e) => void drop('right', e)"
          @drop-on-directory="() => void drop('right')"
          @create-file="createFile(right)"
          @create-folder="createFolderForPane(right)"
          @refresh="refresh(right)"
          @copy-path="copyEntryPath(right, $event)"
          @rename="renameEntry(right, $event)"
          @chmod="chmodEntry(right, $event)"
          @info="showEntryInfo(right, $event)"
          @delete-entry="deleteEntry(right, $event)"
        >
          <template #toolbar>
            <IconButton
              :label="t('sftp.ui.parentDirectory')"
              size="icon-sm"
              :disabled="!right.localDirectory?.parentToken"
              @click="goUp(right)"
              ><Undo2 :size="15"
            /></IconButton>
            <IconButton
              :label="t('sftp.ui.refreshDirectory')"
              size="icon-sm"
              :disabled="right.localLoading"
              @click="refresh(right)"
              ><RefreshCw :class="{ spin: right.localLoading }" :size="15"
            /></IconButton>
            <IconButton
              :label="right.localShowHidden ? t('sftp.ui.hideHidden') : t('sftp.ui.showHidden')"
              size="icon-sm"
              :active="right.localShowHidden"
              @click="right.localShowHidden = !right.localShowHidden"
              ><Eye v-if="right.localShowHidden" :size="15" /><EyeOff v-else :size="15"
            /></IconButton>
            <IconButton
              :label="t('sftp.ui.createFolder')"
              size="icon-sm"
              :disabled="!right.localDirectory"
              @click="createFolderForPane(right)"
              ><FolderPlus :size="15"
            /></IconButton>
            <IconButton
              :label="t('sftp.ui.deleteSelected')"
              size="icon-sm"
              :disabled="deletePending !== null || !right.localDirectory || right.localSelectedNames.length === 0"
              @click="deleteSelected(right)"
              danger
              ><Trash2 :size="15"
            /></IconButton>
          </template>
        </SftpBrowserPane>
        <SftpBrowserPane
          v-else
          :segments="rightSegments"
          :entries="rightRemoteEntries"
          :loading="right.remoteLoading"
          :error="right.connectError || right.remoteError"
          :unavailable-title="paneUnavailableTitle(right)"
          :unavailable-hint="paneUnavailableHint(right)"
          :available="Boolean(right.selectedSessionId)"
          :drop-session-id="right.selectedSessionId ?? undefined"
          :drop-directory="right.remoteDirectory"
          :session-id="right.selectedSessionId ?? ''"
          :can-go-up="right.remoteDirectory !== '/'"
          :show-hidden="right.remoteShowHidden"
          :selected-keys="right.remoteSelectedPaths"
          :editable-path="true"
          :draggable-entries="true"
          context-kind="remote"
          :operations-disabled="deletePending !== null"
          v-model:path-draft="right.pathDraft"
          @open="(e) => void loadRemote(right, e.key)"
          @unsupported-entry="notifyUnsupportedEntry"
          @navigate="(p) => void loadRemote(right, p)"
          @retry="refresh(right)"
          @update:show-hidden="right.remoteShowHidden = $event"
          @select="
            (entry, checked) =>
              (right.remoteSelectedPaths = checked
                ? [...right.remoteSelectedPaths, entry.key]
                : right.remoteSelectedPaths.filter((path) => path !== entry.key))
          "
          @drag-start="(e) => startDrag('right', e)"
          @drop-on-entry="(e) => void drop('right', e)"
          @drop-on-directory="() => void drop('right')"
          @create-file="createFile(right)"
          @create-folder="createFolderForPane(right)"
          @refresh="refresh(right)"
          @upload-file="uploadFile(right)"
          @upload-directory="uploadFolder(right)"
          @copy-path="copyEntryPath(right, $event)"
          @rename="renameEntry(right, $event)"
          @chmod="chmodEntry(right, $event)"
          @info="showEntryInfo(right, $event)"
          @delete-entry="deleteEntry(right, $event)"
          @download="(e) => void downloadEntry(right, e)"
          @remote-edit-synced="refresh(right)"
        >
          <template #toolbar>
            <IconButton
              :label="t('sftp.ui.parentDirectory')"
              size="icon-sm"
              :disabled="right.remoteDirectory === '/'"
              @click="goUp(right)"
              ><Undo2 :size="15"
            /></IconButton>
            <IconButton
              :label="t('sftp.ui.refreshDirectory')"
              size="icon-sm"
              :disabled="right.remoteLoading"
              @click="refresh(right)"
              ><RefreshCw :class="{ spin: right.remoteLoading }" :size="15"
            /></IconButton>
            <IconButton
              :label="right.remoteShowHidden ? t('sftp.ui.hideHidden') : t('sftp.ui.showHidden')"
              size="icon-sm"
              :active="right.remoteShowHidden"
              @click="right.remoteShowHidden = !right.remoteShowHidden"
              ><Eye v-if="right.remoteShowHidden" :size="15" /><EyeOff v-else :size="15"
            /></IconButton>
            <IconButton
              :label="t('sftp.ui.uploadFile')"
              size="icon-sm"
              :disabled="!right.selectedSessionId"
              @click="uploadFile(right)"
              ><Upload :size="15"
            /></IconButton>
            <IconButton
              :label="t('sftp.ui.uploadFolder')"
              size="icon-sm"
              :disabled="!right.selectedSessionId"
              @click="uploadFolder(right)"
              ><FolderUp :size="15"
            /></IconButton>
            <IconButton
              :label="t('sftp.ui.createFolder')"
              size="icon-sm"
              :disabled="!right.selectedSessionId"
              @click="createFolderForPane(right)"
              ><FolderPlus :size="15"
            /></IconButton>
            <IconButton
              :label="t('sftp.ui.deleteSelected')"
              size="icon-sm"
              :disabled="deletePending !== null || !right.selectedSessionId || right.remoteSelectedPaths.length === 0"
              @click="deleteSelected(right)"
              danger
              ><Trash2 :size="15"
            /></IconButton>
          </template>
        </SftpBrowserPane>
      </section>
    </div>
    <DeleteConfirmDialog
      :open="deletePending !== null"
      :title="t('sftp.ui.confirmDelete')"
      :message="t('sftp.ui.confirmDeleteItems', { count: deletePending?.items.length ?? 0 })"
      :items="deletePending?.items"
      :hint="deletePending?.hasDirectory ? t('sftp.ui.deleteDirectoryWarning') : ''"
      :loading="deletePending?.busy ?? false"
      @close="cancelDelete()"
      @confirm="confirmDelete()"
    />
    <QtermDialog
      :open="nameDialog.open"
      @update:open="(open) => !open && !nameDialog.saving && (nameDialog.open = false)"
    >
      <QtermDialogContent v-if="nameDialog.open">
        <form @submit.prevent="submitNameDialog">
          <QtermDialogHeader :title="nameDialog.title" @close="nameDialog.open = false" />
          <div class="dialog-body">
            <FormField :label="t('sftp.ui.info.name')" for-id="sftp-entry-name">
              <QtermInput
                id="sftp-entry-name"
                v-model="nameDialog.value"
                autofocus
                autocomplete="off"
                :placeholder="t('sftp.ui.enterName')"
              />
            </FormField>
            <QtermFormMessage v-if="nameDialog.error" variant="error">{{ nameDialog.error }}</QtermFormMessage>
          </div>
          <QtermDialogFooter>
            <QtermButton type="button" variant="outline" @click="nameDialog.open = false">{{
              t('sftp.ui.cancel')
            }}</QtermButton>
            <QtermButton type="submit" :disabled="nameDialog.saving">{{
              nameDialog.saving ? t('sftp.ui.saving') : t('sftp.ui.confirm')
            }}</QtermButton>
          </QtermDialogFooter>
        </form>
      </QtermDialogContent>
    </QtermDialog>
    <QtermDialog
      :open="permissionDialog.open"
      @update:open="(open) => !open && !permissionDialog.saving && (permissionDialog.open = false)"
    >
      <QtermDialogContent v-if="permissionDialog.open" size="lg">
        <form @submit.prevent="submitPermissionDialog">
          <QtermDialogHeader
            :title="t('sftp.ui.permissionDialog')"
            :description="permissionDialog.entry?.key ?? permissionDialog.entry?.name"
            @close="permissionDialog.open = false"
          />
          <div class="dialog-body sftp-permission-dialog-body">
            <div class="grid grid-cols-3 gap-3">
              <FormField
                v-if="permissionDialog.pane?.kind === 'remote'"
                :label="t('sftp.ui.permission.user')"
                for-id="sftp-entry-owner"
              >
                <QtermCombobox
                  id="sftp-entry-owner"
                  v-model="permissionDialog.uid"
                  :options="permissionUserOptions"
                  :disabled="permissionDialog.principalsLoading"
                  :aria-label="t('sftp.ui.permission.user')"
                />
              </FormField>
              <FormField
                v-if="permissionDialog.pane?.kind === 'remote'"
                :label="t('sftp.ui.permission.group')"
                for-id="sftp-entry-group"
              >
                <QtermCombobox
                  id="sftp-entry-group"
                  v-model="permissionDialog.gid"
                  :options="permissionGroupOptions"
                  :disabled="permissionDialog.principalsLoading"
                  :aria-label="t('sftp.ui.permission.group')"
                />
              </FormField>
              <FormField :label="t('sftp.ui.octalPermissions')" for-id="sftp-entry-mode">
                <QtermInput
                  id="sftp-entry-mode"
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
              <span class="permission-row-label">{{ t('sftp.ui.permission.user') }}</span
              ><QtermCheckbox
                v-model="permissionBits.ownerRead"
                :aria-label="`${t('sftp.ui.permission.user')}${t('sftp.ui.permission.read')}`"
                @update:model-value="syncPermissionMode"
              /><QtermCheckbox
                v-model="permissionBits.ownerWrite"
                :aria-label="`${t('sftp.ui.permission.user')}${t('sftp.ui.permission.write')}`"
                @update:model-value="syncPermissionMode"
              /><QtermCheckbox
                v-model="permissionBits.ownerExec"
                :aria-label="`${t('sftp.ui.permission.user')}${t('sftp.ui.permission.execute')}`"
                @update:model-value="syncPermissionMode"
              />
              <span class="permission-row-label">{{ t('sftp.ui.permission.group') }}</span
              ><QtermCheckbox
                v-model="permissionBits.groupRead"
                :aria-label="`${t('sftp.ui.permission.group')}${t('sftp.ui.permission.read')}`"
                @update:model-value="syncPermissionMode"
              /><QtermCheckbox
                v-model="permissionBits.groupWrite"
                :aria-label="`${t('sftp.ui.permission.group')}${t('sftp.ui.permission.write')}`"
                @update:model-value="syncPermissionMode"
              /><QtermCheckbox
                v-model="permissionBits.groupExec"
                :aria-label="`${t('sftp.ui.permission.group')}${t('sftp.ui.permission.execute')}`"
                @update:model-value="syncPermissionMode"
              />
              <span class="permission-row-label">{{ t('sftp.ui.permission.other') }}</span
              ><QtermCheckbox
                v-model="permissionBits.otherRead"
                :aria-label="`${t('sftp.ui.permission.other')}${t('sftp.ui.permission.read')}`"
                @update:model-value="syncPermissionMode"
              /><QtermCheckbox
                v-model="permissionBits.otherWrite"
                :aria-label="`${t('sftp.ui.permission.other')}${t('sftp.ui.permission.write')}`"
                @update:model-value="syncPermissionMode"
              /><QtermCheckbox
                v-model="permissionBits.otherExec"
                :aria-label="`${t('sftp.ui.permission.other')}${t('sftp.ui.permission.execute')}`"
                @update:model-value="syncPermissionMode"
              />
            </div>
            <QtermFormMessage v-if="permissionDialog.error" variant="error">{{
              permissionDialog.error
            }}</QtermFormMessage>
          </div>
          <QtermDialogFooter
            ><QtermButton type="button" variant="outline" @click="permissionDialog.open = false">{{
              t('sftp.ui.cancel')
            }}</QtermButton
            ><QtermButton type="submit" :disabled="permissionDialog.saving">{{
              t('sftp.ui.save')
            }}</QtermButton></QtermDialogFooter
          >
        </form>
      </QtermDialogContent>
    </QtermDialog>
    <QtermDialog :open="infoDialog.open" @update:open="(open) => !open && (infoDialog.open = false)">
      <QtermDialogContent v-if="infoDialog.open" size="lg">
        <QtermDialogHeader :title="t('sftp.ui.fileInfo')" @close="infoDialog.open = false" />
        <div class="sftp-info-grid">
          <div>
            <span>{{ t('sftp.ui.info.name') }}</span
            ><span>{{ infoDialog.entry?.name || t('sftp.ui.unknown') }}</span>
          </div>
          <div>
            <span>{{ t('sftp.ui.info.type') }}</span
            ><span>{{ entryTypeLabel(infoDialog.entry) }}</span>
          </div>
          <div>
            <span>{{ t('sftp.ui.info.path') }}</span>
            <span class="sftp-info-path-value">
              <span>{{ entryDisplayPath(infoDialog.pane, infoDialog.entry) || t('sftp.ui.unknown') }}</span>
              <QtermButton
                variant="ghost"
                size="icon-sm"
                type="button"
                :aria-label="t('sftp.ui.copyPath')"
                @click="infoDialog.pane && infoDialog.entry && copyEntryPath(infoDialog.pane, infoDialog.entry)"
                ><Copy :size="14"
              /></QtermButton>
            </span>
          </div>
          <div>
            <span>{{ t('sftp.ui.info.size') }}</span
            ><span>{{ formatInfoSize(infoDialog.entry) }}</span>
          </div>
          <div>
            <span>{{ t('sftp.ui.info.modified') }}</span
            ><span>{{ formatInfoDate(infoDialog.entry?.modTime) }}</span>
          </div>
          <div>
            <span>{{ t('sftp.ui.info.permissions') }}</span
            ><span>{{ infoDialog.entry?.mode || t('sftp.ui.unknown') }}</span>
          </div>
          <div>
            <span>{{ t('sftp.ui.info.symlink') }}</span>
            <span>{{
              infoDialog.entry
                ? (infoDialog.entry.symlink ?? infoDialog.entry.type === 'symlink')
                  ? t('sftp.ui.yes')
                  : t('sftp.ui.no')
                : t('sftp.ui.unknown')
            }}</span>
          </div>
        </div>
      </QtermDialogContent>
    </QtermDialog>
    <QtermDialog :open="pickerOpen" @update:open="pickerOpen = $event">
      <QtermDialogContent v-if="pickerOpen" class="sftp-workspace-session-dialog">
        <QtermDialogHeader
          :title="t('sftp.ui.chooseHostTitle')"
          :description="t('sftp.ui.chooseHostDescription')"
          @close="pickerOpen = false"
        />
        <div class="sftp-workspace-picker-body">
          <div class="form-field sftp-workspace-session-search min-w-0 grid gap-[6px] text-qt-muted">
            <Search :size="15" aria-hidden="true" />
            <QtermInput
              v-model="search"
              :aria-label="t('sftp.ui.searchHost')"
              :placeholder="t('sftp.ui.searchHostPlaceholder')"
              type="search"
            />
          </div>
          <div class="sftp-workspace-session-view">
            <button
              class="sftp-workspace-session-option sftp-workspace-local-option text-xs"
              type="button"
              @click="chooseLocal()"
            >
              <Laptop :size="18" aria-hidden="true" />
              <span
                ><strong>{{ t('sftp.ui.currentDevice') }}</strong></span
              >
              <CheckCircle2
                v-if="pickerSide === 'left' ? left.kind === 'local' : right.kind === 'local'"
                :size="16"
                aria-hidden="true"
              />
            </button>
            <Separator class="sftp-workspace-session-separator" />
            <ScrollArea class="sftp-workspace-session-scroll" type="hover">
              <p v-if="filteredHosts.length === 0" class="sftp-workspace-picker-empty text-xs">
                {{ search.trim() ? t('sftp.ui.noMatchingHost') : t('sftp.ui.noHosts') }}
              </p>
              <div v-else class="sftp-workspace-picker-list">
                <button
                  v-for="host in filteredHosts"
                  :key="host.id"
                  class="sftp-workspace-session-option text-xs"
                  :class="{
                    'is-selected': host.id === (pickerSide === 'left' ? left.selectedHostId : right.selectedHostId),
                  }"
                  type="button"
                  @click="chooseHost(host.id)"
                >
                  <HostIcon :icon="host.icon" :size="18" />
                  <span
                    ><strong>{{ host.name }}</strong
                    ><small class="text-xs">{{ host.address }}</small></span
                  >
                  <CheckCircle2
                    v-if="host.id === (pickerSide === 'left' ? left.selectedHostId : right.selectedHostId)"
                    :size="16"
                    aria-hidden="true"
                  />
                </button>
              </div>
            </ScrollArea>
          </div>
        </div>
      </QtermDialogContent>
    </QtermDialog>
  </div>
</template>

<style scoped>
.sftp-workspace {
  display: flex;
  height: 100%;
  min-height: 0;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  color: var(--qt-text);
}
.sftp-workspace-panes {
  display: grid;
  min-height: 0;
  flex: 1;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 8px;
}
.sftp-workspace-pane {
  display: flex;
  min-width: 0;
  min-height: 0;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--qt-border);
  border-radius: 8px;
}
.sftp-workspace-pane-header {
  display: flex;
  flex: none;
  align-items: center;
  gap: 8px;
  min-height: 40px;
  padding: 0 12px;
  border-bottom: 1px solid var(--qt-border);
}
.sftp-workspace-pane-title {
  display: inline-flex;
  min-width: 0;
  flex: 1;
  align-items: center;
  gap: 7px;
  overflow: hidden;
  border: 0;
  padding: 0;
  color: inherit;
  background: transparent;
  text-align: left;
  white-space: nowrap;
  cursor: pointer;
}
.sftp-workspace-pane-title > span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}
.sftp-workspace-pane-status {
  flex: none;
  color: var(--qt-muted);
}
.sftp-workspace-session-dialog {
  width: min(480px, calc(100vw - 32px));
  max-width: min(480px, calc(100vw - 32px));
  gap: 0;
  padding: 0;
}
.sftp-workspace-picker-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-height: 0;
  padding: 10px 16px 16px;
}
.sftp-workspace-session-view {
  padding: 4px;
  border: 1px solid var(--qt-border);
  border-radius: 6px;
}
.sftp-workspace-local-option {
  margin: 0;
}
.sftp-workspace-session-search {
  position: relative;
}
.sftp-workspace-session-search > svg {
  position: absolute;
  left: 10px;
  top: 50%;
  transform: translateY(-50%);
  color: var(--qt-muted);
  pointer-events: none;
}
.sftp-workspace-session-search :deep(input) {
  padding-left: 32px;
}
.sftp-workspace-session-scroll {
  height: var(--qt-sftp-session-list-height-even, 280px);
  min-height: 0;
}
.sftp-workspace-picker-list {
  display: flex;
  flex-direction: column;
}
.sftp-workspace-local-option {
  margin: 0;
}
.sftp-workspace-session-separator {
  margin: 4px 2px;
}
.sftp-workspace-session-option {
  display: flex;
  width: 100%;
  min-height: 38px;
  align-items: center;
  gap: 8px;
  padding: 5px 7px;
  border: 1px solid transparent;
  border-radius: 5px;
  color: inherit;
  background: transparent;
  text-align: left;
  cursor: pointer;
}
.sftp-workspace-session-option:hover,
.sftp-workspace-session-option.is-selected {
  background: var(--qt-hover);
  border-color: var(--qt-border);
}
.sftp-workspace-session-option > span {
  min-width: 0;
  flex: 1;
}
.sftp-workspace-session-option :deep(.host-logo),
.sftp-workspace-session-option > svg {
  flex: none;
}
.sftp-workspace-session-option strong,
.sftp-workspace-session-option small {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sftp-permission-dialog-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
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
.sftp-workspace-session-option strong {
  font-weight: 400;
}
.sftp-workspace-session-option small {
  margin-top: 2px;
  color: var(--qt-muted);
}
.sftp-workspace-picker-empty {
  padding: 32px 8px;
  color: var(--qt-muted);
  text-align: center;
}
</style>
