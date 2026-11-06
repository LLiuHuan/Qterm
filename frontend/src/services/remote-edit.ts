import type { LocalEdit, RemoteFileContent, RemoteFileWriteResult } from '../../bindings/changeme/internal/model/models'
import { createRuntimeGateway, hasWailsRuntime } from './runtime'

export const REMOTE_EDIT_CHANGED_EVENT = 'remote-edit:changed'

export type RemoteEditState = 'watching' | 'syncing' | 'synced' | 'error' | 'stopped'

export type LocalEditSnapshot = LocalEdit

export type RemoteFileContentSnapshot = RemoteFileContent

export type RemoteFileWriteSnapshot = RemoteFileWriteResult

export interface RemoteEditChangedPayload {
  editID: string
  state: string
  lastError: string
  hasUnsyncedChanges?: boolean
}

export interface RemoteEditGateway {
  readonly persistent: boolean
  listLocalEdits(): Promise<LocalEditSnapshot[]>
  openLocalEdit(sessionID: string, remotePath: string): Promise<LocalEditSnapshot>
  syncLocalEdit(editID: string): Promise<LocalEditSnapshot>
  discardLocalEdit(editID: string): Promise<void>
  readRemoteFileText(sessionID: string, remotePath: string): Promise<RemoteFileContentSnapshot>
  writeRemoteFileText(
    sessionID: string,
    remotePath: string,
    contentBase64: string,
    expectedHash: string,
  ): Promise<RemoteFileWriteSnapshot>
}

const EMPTY_CONTENT: RemoteFileContentSnapshot = {
  contentBase64: '',
  size: 0,
  binary: false,
  truncated: false,
  hash: '',
  modTime: '',
}

class WailsRemoteEditGateway implements RemoteEditGateway {
  readonly persistent = true

  async listLocalEdits(): Promise<LocalEditSnapshot[]> {
    const { ListLocalEdits } = await import('../../bindings/changeme/internal/app/service')
    return (await ListLocalEdits()) ?? []
  }

  async openLocalEdit(sessionID: string, remotePath: string): Promise<LocalEditSnapshot> {
    const { OpenLocalEdit } = await import('../../bindings/changeme/internal/app/service')
    return await OpenLocalEdit(sessionID, remotePath)
  }

  async syncLocalEdit(editID: string): Promise<LocalEditSnapshot> {
    const { SyncLocalEdit } = await import('../../bindings/changeme/internal/app/service')
    return await SyncLocalEdit(editID)
  }

  async discardLocalEdit(editID: string): Promise<void> {
    const { DiscardLocalEdit } = await import('../../bindings/changeme/internal/app/service')
    await DiscardLocalEdit(editID)
  }

  async readRemoteFileText(sessionID: string, remotePath: string): Promise<RemoteFileContentSnapshot> {
    const { ReadRemoteFileText } = await import('../../bindings/changeme/internal/app/service')
    return await ReadRemoteFileText(sessionID, remotePath)
  }

  async writeRemoteFileText(
    sessionID: string,
    remotePath: string,
    contentBase64: string,
    expectedHash: string,
  ): Promise<RemoteFileWriteSnapshot> {
    const { WriteRemoteFileText } = await import('../../bindings/changeme/internal/app/service')
    return await WriteRemoteFileText(sessionID, remotePath, contentBase64, expectedHash)
  }
}

class MemoryRemoteEditGateway implements RemoteEditGateway {
  readonly persistent = false
  private readonly edits = new Map<string, LocalEditSnapshot>()

  async listLocalEdits(): Promise<LocalEditSnapshot[]> {
    return [...this.edits.values()]
  }

  async openLocalEdit(): Promise<LocalEditSnapshot> {
    throw new Error('remote edit is unavailable in browser preview')
  }

  async syncLocalEdit(editID: string): Promise<LocalEditSnapshot> {
    const edit = this.edits.get(editID)
    if (!edit) throw new Error('remote edit is unavailable in browser preview')
    return edit
  }

  async discardLocalEdit(editID: string): Promise<void> {
    this.edits.delete(editID)
  }

  async readRemoteFileText(): Promise<RemoteFileContentSnapshot> {
    return { ...EMPTY_CONTENT, truncated: true }
  }

  async writeRemoteFileText(): Promise<RemoteFileWriteSnapshot> {
    throw new Error('remote edit is unavailable in browser preview')
  }
}

export function createRemoteEditGateway(): RemoteEditGateway {
  return createRuntimeGateway<RemoteEditGateway>(
    () => new WailsRemoteEditGateway(),
    () => new MemoryRemoteEditGateway(),
    shouldUseNativeRuntime,
  )
}

function shouldUseNativeRuntime(): boolean {
  return window.location.protocol === 'wails:' || hasWailsRuntime()
}
