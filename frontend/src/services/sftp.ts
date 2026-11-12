import { errorMessage } from './errors'
import type { ConflictAction } from './transfer-conflicts'
import { createRuntimeGateway } from './runtime'

export interface RemoteFileEntry {
  name: string
  path: string
  directory?: string
  type: 'file' | 'directory' | 'symlink' | 'other'

  targetType?: 'file' | 'directory' | 'symlink' | 'other'
  size: number
  sizeText?: string
  mode: string
  uid?: number
  gid?: number
  modTime: string
}

export interface RemotePrincipal {
  id: number
  name: string
}

export interface RemotePrincipals {
  users: RemotePrincipal[]
  groups: RemotePrincipal[]
}

export interface LocalFileEntry {
  name: string
  type: 'file' | 'directory' | 'symlink' | 'other'
  size: number
  sizeText?: string
  modTime: string

  mode?: string

  displayPath?: string

  isSymlink?: boolean
}

export interface LocalDirectorySegment {
  name: string
  token: string
}

export interface LocalDirectoryResult {
  token: string

  displayPath?: string
  name: string
  parentToken: string

  segments?: LocalDirectorySegment[]
  entries: LocalFileEntry[]
}

export type SFTPTransferDirection = 'upload' | 'download'
export type SFTPTransferState =
  'queued' | 'awaitingOverwrite' | 'running' | 'paused' | 'completed' | 'failed' | 'canceled'

export interface SFTPTransferRecord {
  id: string
  sessionId: string
  direction: SFTPTransferDirection
  localName: string
  remotePath: string
  state: SFTPTransferState
  transferred: number
  total: number

  rateBytesPerSecond?: number
  createdAt: string
  updatedAt: string

  stage?: string
}

export interface SFTPTransferSelectionResult {
  canceled: boolean
  transfer: SFTPTransferRecord | null
}
export interface DirectoryUploadResult {
  canceled: boolean
  queued: number
}

export interface DirectoryDownloadResult {
  queued: number
  skipped: number
}
export interface RemoteDirectoryTransferResult {
  queued: number
  skipped: number
}

export interface LocalDirectorySelectionResult {
  canceled: boolean
  directory: LocalDirectoryResult | null
}

export interface SftpSettings {
  autoDirectoryFollow: boolean
  showHiddenFiles: boolean
  maxTransfers: number
  localDirectory: string
  editorPath: string
}
export interface DropUploadResult {
  transfers: SFTPTransferRecord[]
  rejected: string[]

  queued?: number

  fellBack?: boolean

  fallbackReason?: string
}

export interface SftpGateway {
  readonly persistent: boolean
  directoryFollow(sessionId: string, configure: boolean, signal?: AbortSignal): Promise<string>
  createRemoteDirectory(sessionId: string, directory: string, name: string): Promise<void>
  deleteRemoteEntry(sessionId: string, remotePath: string): Promise<void>

  deleteRemoteEntryElevated(sessionId: string, remotePath: string, password?: string): Promise<void>
  createRemoteFile?(sessionId: string, directory: string, name: string): Promise<void>
  renameRemoteEntry?(sessionId: string, remotePath: string, newName: string): Promise<void>
  chmodRemoteEntry?(sessionId: string, remotePath: string, mode: string): Promise<void>
  listRemotePrincipals?(sessionId: string): Promise<RemotePrincipals>
  setRemoteEntryPermissions?(
    sessionId: string,
    remotePath: string,
    mode: string,
    uid: number,
    gid: number,
  ): Promise<void>
  listRemoteDirectory(sessionId: string, directory: string): Promise<RemoteFileEntry[]>
  selectSFTPUploadDirectory(sessionId: string, remoteDirectory: string): Promise<DirectoryUploadResult>
  selectSFTPUploadFile(sessionId: string, remoteDirectory: string): Promise<SFTPTransferSelectionResult>
  selectSFTPDownloadFile(sessionId: string, remotePath: string): Promise<SFTPTransferSelectionResult>
  listSFTPTransfers(): Promise<SFTPTransferRecord[]>
  confirmSFTPTransferOverwrite(id: string): Promise<SFTPTransferRecord>
  resolveSFTPTransferConflict?(id: string, action: ConflictAction): Promise<void>
  retrySFTPTransfer(id: string): Promise<SFTPTransferRecord>

  pauseSFTPTransfer?(id: string): Promise<SFTPTransferRecord>

  resumeSFTPTransfer?(id: string): Promise<SFTPTransferRecord>
  cancelSFTPTransfer(id: string): Promise<void>
  uploadDroppedFiles(token: string, remoteDirectory: string): Promise<DropUploadResult>
  cancelDroppedFiles(token: string): Promise<void>

  openLocalDirectory?(token: string, child: string): Promise<LocalDirectoryResult>

  chooseLocalDirectory?(): Promise<LocalDirectorySelectionResult>
  createLocalFile?(token: string, name: string): Promise<void>
  createLocalDirectory?(token: string, name: string): Promise<void>
  renameLocalEntry?(token: string, name: string, newName: string): Promise<void>
  chmodLocalEntry?(token: string, name: string, mode: string): Promise<void>
  deleteLocalEntry?(token: string, name: string): Promise<void>
  queueWorkspaceUpload?(
    sessionId: string,
    token: string,
    name: string,
    remoteDirectory: string,
  ): Promise<SFTPTransferRecord | void>

  queueWorkspaceUploadDirectory?(sessionId: string, token: string, name: string, remoteDirectory: string): Promise<void>
  queueWorkspaceDownload?(sessionId: string, remotePath: string, token: string): Promise<SFTPTransferRecord | void>

  downloadRemoteDirectoryArchive?(sessionId: string, remoteDirectory: string, localDirectory: string): Promise<void>

  enqueueDirectoryArchiveDownload?(
    sessionId: string,
    remoteDirectory: string,
    localDirectory: string,
  ): Promise<SFTPTransferRecord>

  chooseDirectoryPath?(): Promise<string>

  queueWorkspaceDownloadDirectory?(
    sessionId: string,
    remotePath: string,
    token: string,
  ): Promise<DirectoryDownloadResult>

  queueRemoteTransfer?(
    sourceSessionId: string,
    destSessionId: string,
    remotePath: string,
    destDirectory: string,
  ): Promise<SFTPTransferRecord | void>

  queueRemoteTransferDirectory?(
    sourceSessionId: string,
    destSessionId: string,
    remotePath: string,
    destDirectory: string,
  ): Promise<RemoteDirectoryTransferResult>

  openConnection?(hostId: string): Promise<string>

  closeConnection?(sessionId: string): Promise<void>
}

export function sftpErrorMessage(error: unknown, fallback: string): string {
  return errorMessage(error, fallback)
}

export function toTransferRecord(record: SFTPTransferRecord): SFTPTransferRecord {
  return { ...record, stage: record.stage || undefined }
}

export function toTransferRecords(records: SFTPTransferRecord[] | null | undefined): SFTPTransferRecord[] {
  return (records ?? []).map(toTransferRecord)
}

interface WailsWindow extends Window {
  _wails?: { environment?: { OS?: string } }
}

class MemorySftpGateway implements SftpGateway {
  readonly persistent = false

  async directoryFollow(): Promise<string> {
    throw new Error('浏览器预览没有真实 SSH 会话')
  }

  async createRemoteDirectory(): Promise<void> {
    throw new Error('浏览器预览没有真实 SFTP 会话')
  }
  async deleteRemoteEntry(): Promise<void> {
    throw new Error('浏览器预览没有真实 SFTP 会话')
  }
  async deleteRemoteEntryElevated(): Promise<void> {
    throw new Error('浏览器预览没有真实 SFTP 会话')
  }

  async listRemoteDirectory(): Promise<RemoteFileEntry[]> {
    throw new Error('浏览器预览没有真实 SFTP 会话')
  }

  async selectSFTPUploadDirectory(): Promise<DirectoryUploadResult> {
    throw new Error('浏览器预览不能上传真实文件夹')
  }

  async selectSFTPUploadFile(): Promise<SFTPTransferSelectionResult> {
    throw new Error('浏览器预览不能选择真实上传文件')
  }

  async selectSFTPDownloadFile(): Promise<SFTPTransferSelectionResult> {
    throw new Error('浏览器预览不能保存真实远程文件')
  }

  async listSFTPTransfers(): Promise<SFTPTransferRecord[]> {
    return []
  }

  async confirmSFTPTransferOverwrite(): Promise<SFTPTransferRecord> {
    throw new Error('浏览器预览没有真实传输任务')
  }

  async resolveSFTPTransferConflict(): Promise<void> {
    throw new Error('浏览器预览没有真实传输任务')
  }

  async retrySFTPTransfer(): Promise<SFTPTransferRecord> {
    throw new Error('浏览器预览没有真实传输任务')
  }

  async pauseSFTPTransfer(): Promise<SFTPTransferRecord> {
    throw new Error('浏览器预览没有真实传输任务')
  }

  async resumeSFTPTransfer(): Promise<SFTPTransferRecord> {
    throw new Error('浏览器预览没有真实传输任务')
  }

  async cancelSFTPTransfer(): Promise<void> {
    throw new Error('浏览器预览没有真实传输任务')
  }
  async uploadDroppedFiles(): Promise<DropUploadResult> {
    throw new Error('浏览器预览不能上传拖入文件')
  }
  async cancelDroppedFiles(): Promise<void> {
    return
  }
  async queueWorkspaceUploadDirectory(): Promise<void> {
    throw new Error('浏览器预览不能上传真实文件夹')
  }

  async queueRemoteTransfer(): Promise<SFTPTransferRecord | void> {
    throw new Error('浏览器预览不支持远程到远程传输')
  }

  async queueRemoteTransferDirectory(): Promise<RemoteDirectoryTransferResult> {
    throw new Error('浏览器预览不支持远程到远程传输')
  }

  async chooseLocalDirectory(): Promise<LocalDirectorySelectionResult> {
    throw new Error('浏览器预览没有本地目录选择器')
  }

  async queueWorkspaceDownloadDirectory(): Promise<DirectoryDownloadResult> {
    throw new Error('浏览器预览不能下载真实文件夹')
  }

  async openConnection(): Promise<string> {
    throw new Error('浏览器预览没有真实 SSH 连接')
  }
  async closeConnection(): Promise<void> {
    return
  }
}

class WailsSftpGateway implements SftpGateway {
  readonly persistent = true

  async directoryFollow(sessionId: string, configure: boolean, signal?: AbortSignal): Promise<string> {
    const { DirectoryFollow } = await import('../../bindings/changeme/internal/app/service')
    if (signal?.aborted) throw new Error('目录跟随操作已取消')
    const request = DirectoryFollow(sessionId, configure)
    const abort = () => {
      void request.cancel().catch(() => undefined)
    }
    signal?.addEventListener('abort', abort, { once: true })
    try {
      return await request
    } finally {
      signal?.removeEventListener('abort', abort)
    }
  }

  async createRemoteDirectory(sessionId: string, directory: string, name: string): Promise<void> {
    const { CreateRemoteDirectory } = await import('../../bindings/changeme/internal/app/service')
    await CreateRemoteDirectory(sessionId, directory, name)
  }
  async deleteRemoteEntry(sessionId: string, remotePath: string): Promise<void> {
    const { DeleteRemoteEntry } = await import('../../bindings/changeme/internal/app/service')
    await DeleteRemoteEntry(sessionId, remotePath)
  }
  async downloadRemoteDirectoryArchive(
    sessionId: string,
    remoteDirectory: string,
    localDirectory: string,
  ): Promise<void> {
    const { DownloadSFTPDirectoryArchive } = await import('../../bindings/changeme/internal/app/service')
    await DownloadSFTPDirectoryArchive(sessionId, remoteDirectory, localDirectory)
  }
  async enqueueDirectoryArchiveDownload(
    sessionId: string,
    remoteDirectory: string,
    localDirectory: string,
  ): Promise<SFTPTransferRecord> {
    const { EnqueueSFTPDirectoryArchiveDownload } = await import('../../bindings/changeme/internal/app/service')
    return toTransferRecord(
      (await EnqueueSFTPDirectoryArchiveDownload(sessionId, remoteDirectory, localDirectory)) as SFTPTransferRecord,
    )
  }
  async chooseDirectoryPath(): Promise<string> {
    const { SelectSFTPLocalDirectorySetting } = await import('../../bindings/changeme/internal/app/service')
    return (await SelectSFTPLocalDirectorySetting()) ?? ''
  }

  async deleteRemoteEntryElevated(sessionId: string, remotePath: string, password = ''): Promise<void> {
    const { RemoveSFTPEntryElevated } = await import('../../bindings/changeme/internal/app/service')
    await RemoveSFTPEntryElevated(sessionId, remotePath, password)
  }
  async createRemoteFile(sessionId: string, directory: string, name: string): Promise<void> {
    const { CreateRemoteFile } = await import('../../bindings/changeme/internal/app/service')
    await CreateRemoteFile(sessionId, directory, name)
  }
  async renameRemoteEntry(sessionId: string, remotePath: string, newName: string): Promise<void> {
    const { RenameRemoteEntry } = await import('../../bindings/changeme/internal/app/service')
    await RenameRemoteEntry(sessionId, remotePath, newName)
  }
  async chmodRemoteEntry(sessionId: string, remotePath: string, mode: string): Promise<void> {
    const { ChmodRemoteEntry } = await import('../../bindings/changeme/internal/app/service')
    await ChmodRemoteEntry(sessionId, remotePath, mode)
  }
  async listRemotePrincipals(sessionId: string): Promise<RemotePrincipals> {
    const { ListRemotePrincipals } = await import('../../bindings/changeme/internal/app/service')
    const result = await ListRemotePrincipals(sessionId)
    return { users: result.users ?? [], groups: result.groups ?? [] }
  }
  async setRemoteEntryPermissions(
    sessionId: string,
    remotePath: string,
    mode: string,
    uid: number,
    gid: number,
  ): Promise<void> {
    const { SetRemoteEntryPermissions } = await import('../../bindings/changeme/internal/app/service')
    await SetRemoteEntryPermissions(sessionId, remotePath, mode, uid, gid)
  }
  async listRemoteDirectory(sessionId: string, directory: string): Promise<RemoteFileEntry[]> {
    const { ListRemoteDirectory } = await import('../../bindings/changeme/internal/app/service')
    return ((await ListRemoteDirectory(sessionId, directory)) ?? []) as RemoteFileEntry[]
  }

  async selectSFTPUploadDirectory(sessionId: string, remoteDirectory: string): Promise<DirectoryUploadResult> {
    const { SelectSFTPUploadDirectory } = await import('../../bindings/changeme/internal/app/service')
    return await SelectSFTPUploadDirectory(sessionId, remoteDirectory)
  }

  async selectSFTPUploadFile(sessionId: string, remoteDirectory: string): Promise<SFTPTransferSelectionResult> {
    const { SelectSFTPUploadFile } = await import('../../bindings/changeme/internal/app/service')
    return (await SelectSFTPUploadFile(sessionId, remoteDirectory)) as SFTPTransferSelectionResult
  }

  async selectSFTPDownloadFile(sessionId: string, remotePath: string): Promise<SFTPTransferSelectionResult> {
    const { SelectSFTPDownloadFile } = await import('../../bindings/changeme/internal/app/service')
    return (await SelectSFTPDownloadFile(sessionId, remotePath)) as SFTPTransferSelectionResult
  }

  async listSFTPTransfers(): Promise<SFTPTransferRecord[]> {
    const { ListSFTPTransfers } = await import('../../bindings/changeme/internal/app/service')
    return toTransferRecords((await ListSFTPTransfers()) as SFTPTransferRecord[] | null)
  }

  async confirmSFTPTransferOverwrite(id: string): Promise<SFTPTransferRecord> {
    const { ConfirmSFTPTransferOverwrite } = await import('../../bindings/changeme/internal/app/service')
    return toTransferRecord((await ConfirmSFTPTransferOverwrite(id)) as SFTPTransferRecord)
  }

  async resolveSFTPTransferConflict(id: string, action: ConflictAction): Promise<void> {
    const { ResolveSFTPTransferConflict } = await import('../../bindings/changeme/internal/app/service')
    await ResolveSFTPTransferConflict(id, action)
  }

  async retrySFTPTransfer(id: string): Promise<SFTPTransferRecord> {
    const { RetrySFTPTransfer } = await import('../../bindings/changeme/internal/app/service')
    return toTransferRecord((await RetrySFTPTransfer(id)) as SFTPTransferRecord)
  }

  async pauseSFTPTransfer(id: string): Promise<SFTPTransferRecord> {
    const { PauseSFTPTransfer } = await import('../../bindings/changeme/internal/app/service')
    return toTransferRecord((await PauseSFTPTransfer(id)) as SFTPTransferRecord)
  }

  async resumeSFTPTransfer(id: string): Promise<SFTPTransferRecord> {
    const { ResumeSFTPTransfer } = await import('../../bindings/changeme/internal/app/service')
    return toTransferRecord((await ResumeSFTPTransfer(id)) as SFTPTransferRecord)
  }

  async cancelSFTPTransfer(id: string): Promise<void> {
    const { CancelSFTPTransfer } = await import('../../bindings/changeme/internal/app/service')
    await CancelSFTPTransfer(id)
  }
  async uploadDroppedFiles(token: string, remoteDirectory: string): Promise<DropUploadResult> {
    const { UploadDroppedFiles } = await import('../../bindings/changeme/internal/app/service')
    return (await UploadDroppedFiles(token, remoteDirectory)) as DropUploadResult
  }
  async cancelDroppedFiles(token: string): Promise<void> {
    const { CancelDroppedFiles } = await import('../../bindings/changeme/internal/app/service')
    await CancelDroppedFiles(token)
  }

  async openLocalDirectory(token: string, child: string): Promise<LocalDirectoryResult> {
    const { OpenSFTPLocalDirectory } = await import('../../bindings/changeme/internal/app/service')
    return (await OpenSFTPLocalDirectory(token, child)) as LocalDirectoryResult
  }

  async chooseLocalDirectory(): Promise<LocalDirectorySelectionResult> {
    const { SelectSFTPLocalDirectory } = await import('../../bindings/changeme/internal/app/service')
    return (await SelectSFTPLocalDirectory()) as LocalDirectorySelectionResult
  }
  async createLocalFile(token: string, name: string): Promise<void> {
    const { CreateLocalFile } = await import('../../bindings/changeme/internal/app/service')
    await CreateLocalFile(token, name)
  }
  async createLocalDirectory(token: string, name: string): Promise<void> {
    const { CreateLocalDirectory } = await import('../../bindings/changeme/internal/app/service')
    await CreateLocalDirectory(token, name)
  }
  async renameLocalEntry(token: string, name: string, newName: string): Promise<void> {
    const { RenameLocalEntry } = await import('../../bindings/changeme/internal/app/service')
    await RenameLocalEntry(token, name, newName)
  }
  async chmodLocalEntry(token: string, name: string, mode: string): Promise<void> {
    const { ChmodLocalEntry } = await import('../../bindings/changeme/internal/app/service')
    await ChmodLocalEntry(token, name, mode)
  }
  async deleteLocalEntry(token: string, name: string): Promise<void> {
    const { DeleteLocalEntry } = await import('../../bindings/changeme/internal/app/service')
    await DeleteLocalEntry(token, name)
  }

  async queueWorkspaceUpload(
    sessionId: string,
    token: string,
    name: string,
    remoteDirectory: string,
  ): Promise<SFTPTransferRecord | void> {
    const { QueueSFTPWorkspaceUpload } = await import('../../bindings/changeme/internal/app/service')
    return (await QueueSFTPWorkspaceUpload(sessionId, token, name, remoteDirectory)) as SFTPTransferRecord
  }

  async queueWorkspaceDownloadDirectory(
    sessionId: string,
    remotePath: string,
    token: string,
  ): Promise<DirectoryDownloadResult> {
    const { QueueSFTPWorkspaceDownloadDirectory } = await import('../../bindings/changeme/internal/app/service')
    return (await QueueSFTPWorkspaceDownloadDirectory(sessionId, remotePath, token)) as DirectoryDownloadResult
  }

  async queueWorkspaceDownload(
    sessionId: string,
    remotePath: string,
    token: string,
  ): Promise<SFTPTransferRecord | void> {
    const { QueueSFTPWorkspaceDownload } = await import('../../bindings/changeme/internal/app/service')
    return (await QueueSFTPWorkspaceDownload(sessionId, remotePath, token)) as SFTPTransferRecord
  }

  async queueRemoteTransfer(
    sourceSessionId: string,
    destSessionId: string,
    remotePath: string,
    destDirectory: string,
  ): Promise<SFTPTransferRecord | void> {
    const { QueueSFTPRemoteTransfer } = await import('../../bindings/changeme/internal/app/service')
    return (await QueueSFTPRemoteTransfer(
      sourceSessionId,
      destSessionId,
      remotePath,
      destDirectory,
    )) as SFTPTransferRecord
  }

  async queueRemoteTransferDirectory(
    sourceSessionId: string,
    destSessionId: string,
    remotePath: string,
    destDirectory: string,
  ): Promise<RemoteDirectoryTransferResult> {
    const { QueueSFTPRemoteTransferDirectory } = await import('../../bindings/changeme/internal/app/service')
    return (await QueueSFTPRemoteTransferDirectory(
      sourceSessionId,
      destSessionId,
      remotePath,
      destDirectory,
    )) as RemoteDirectoryTransferResult
  }

  async queueWorkspaceUploadDirectory(
    sessionId: string,
    token: string,
    name: string,
    remoteDirectory: string,
  ): Promise<void> {
    const { QueueSFTPWorkspaceUploadDirectory } = await import('../../bindings/changeme/internal/app/service')
    await QueueSFTPWorkspaceUploadDirectory(sessionId, token, name, remoteDirectory)
  }

  async openConnection(hostId: string): Promise<string> {
    const { OpenSftpConnection } = await import('../../bindings/changeme/internal/app/service')
    const session = await OpenSftpConnection(hostId)
    return session.id
  }

  async closeConnection(sessionId: string): Promise<void> {
    const { CloseSession } = await import('../../bindings/changeme/internal/app/service')
    await CloseSession(sessionId)
  }
}

export async function selectSFTPLocalDirectorySetting(): Promise<string> {
  const service =
    (await import('../../bindings/changeme/internal/app/service')) as typeof import('../../bindings/changeme/internal/app/service') & {
      SelectSFTPLocalDirectorySetting: () => Promise<string>
    }
  return service.SelectSFTPLocalDirectorySetting()
}

export async function selectSFTPEditor(): Promise<string> {
  const service =
    (await import('../../bindings/changeme/internal/app/service')) as typeof import('../../bindings/changeme/internal/app/service') & {
      SelectSFTPEditor: () => Promise<string>
    }
  return service.SelectSFTPEditor()
}

export function createSftpGateway(): SftpGateway {
  return createRuntimeGateway<SftpGateway>(
    () => new WailsSftpGateway(),
    () => new MemorySftpGateway(),
    shouldUseNativeRuntime,
  )
}

function shouldUseNativeRuntime(): boolean {
  const runtimeWindow = window as WailsWindow
  const query = new URLSearchParams(window.location.search)
  return Boolean(
    window.location.protocol === 'wails:' ||
    runtimeWindow._wails?.environment?.OS ||
    query.has('openSession') ||
    query.has('openHost') ||
    query.has('sftpSession') ||
    query.get('openLocal') === '1',
  )
}
