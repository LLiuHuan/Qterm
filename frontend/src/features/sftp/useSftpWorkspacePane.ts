import { reactive } from 'vue'
import type { LocalDirectoryResult, RemoteFileEntry, SFTPTransferRecord } from '../../services/sftp'
import { loadSettings } from '../settings/settingsStore'

export type SftpWorkspacePaneKind = 'local' | 'remote'

export interface SftpWorkspacePaneState {
  kind: SftpWorkspacePaneKind
  selectedHostId: string | null
  selectedSessionId: string | null
  connecting: boolean
  connectError: string
  localDirectory: LocalDirectoryResult | null
  localLoading: boolean
  localError: string
  localShowHidden: boolean
  localSelectedNames: string[]
  remoteDirectory: string

  pathDraft: string
  remoteEntries: RemoteFileEntry[]
  remoteLoading: boolean
  remoteError: string
  remoteShowHidden: boolean
  remoteDirectoryFollow: boolean
  remoteSelectedPaths: string[]
  transfers: SFTPTransferRecord[]
  transferError: string
  entryBusy: boolean
  entryError: string
  queueError: string
  generation: number
  localRequestSequence: number
  remoteRequestSequence: number
  transferRequestSequence: number
  disposed: boolean
}

export function useSftpWorkspacePane(kind: SftpWorkspacePaneKind): SftpWorkspacePaneState {
  const showHiddenFiles = loadSettings().sftpShowHiddenFiles ?? false
  return reactive({
    kind,
    selectedHostId: null,
    selectedSessionId: null,
    connecting: false,
    connectError: '',
    localDirectory: null,
    localLoading: false,
    localError: '',
    localShowHidden: showHiddenFiles,
    localSelectedNames: [],
    remoteDirectory: '/',
    pathDraft: '/',
    remoteEntries: [],
    remoteLoading: false,
    remoteError: '',
    remoteShowHidden: showHiddenFiles,
    remoteDirectoryFollow: false,
    remoteSelectedPaths: [],
    transfers: [],
    transferError: '',
    entryBusy: false,
    entryError: '',
    queueError: '',
    generation: 0,
    localRequestSequence: 0,
    remoteRequestSequence: 0,
    transferRequestSequence: 0,
    disposed: false,
  }) as SftpWorkspacePaneState
}
