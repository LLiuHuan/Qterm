import { ref, watch } from 'vue'
import type { ExternalToast } from 'vue-sonner'
import { claimSession, claimTransfer } from '../../services/transferOwnership'
import { sftpErrorMessage, type SFTPTransferRecord, type SftpGateway } from '../../services/sftp'
import type { SftpBrowserEntry } from './SftpBrowserPane.vue'
import type { SftpWorkspacePaneState } from './useSftpWorkspacePane'

type Translate = (key: string, params?: Record<string, string | number>) => string

type WorkspaceToast = {
  error: (message: string, options?: ExternalToast) => unknown
  success: (message: string, options?: ExternalToast) => unknown
  info: (message: string, options?: ExternalToast) => unknown
}

export interface UseSftpWorkspaceTransfersOptions {
  gateway: SftpGateway
  left: SftpWorkspacePaneState
  right: SftpWorkspacePaneState

  transfers: () => SFTPTransferRecord[]
  t: Translate
  toast: WorkspaceToast
  refresh: (pane: SftpWorkspacePaneState) => void
}

export function useSftpWorkspaceTransfers(options: UseSftpWorkspaceTransfersOptions) {
  const { gateway, left, right, t, toast, refresh } = options
  const dragged = ref<{ side: 'left' | 'right'; entry: SftpBrowserEntry } | null>(null)

  function localDownloadTarget(pane: SftpWorkspacePaneState) {
    const other = pane === left ? right : left
    if (other.kind === 'local' && other.localDirectory) return other.localDirectory
    return null
  }

  function claimQueuedTransfer(sessionId: string, transfer: SFTPTransferRecord | void): void {
    if (transfer?.id) claimTransfer(transfer.id)
    else claimSession(sessionId)
  }

  function claimQueuedDirectory(sessionId: string, queued: number): void {
    if (queued > 0) claimSession(sessionId)
  }

  async function downloadEntry(pane: SftpWorkspacePaneState, entry: SftpBrowserEntry): Promise<void> {
    if (pane.kind !== 'remote' || !pane.selectedSessionId) return
    if (entry.type !== 'directory') {
      if (!gateway.selectSFTPDownloadFile) return
      try {
        const result = await gateway.selectSFTPDownloadFile(pane.selectedSessionId, entry.key)
        if (!result.canceled) claimQueuedTransfer(pane.selectedSessionId, result.transfer ?? undefined)
      } catch (error) {
        toast.error(t('sftp.ui.downloadFailed'), {
          description: sftpErrorMessage(error, t('sftp.ui.downloadSelectedFailed')),
        })
      }
      return
    }
    const target = localDownloadTarget(pane)
    if (!target) {
      toast.error(t('sftp.ui.downloadFolderFailed'), { description: t('sftp.ui.downloadFolderHint') })
      return
    }
    if (!gateway.queueWorkspaceDownloadDirectory) {
      toast.error(t('sftp.ui.downloadFolderUnavailable'), { description: t('sftp.ui.downloadFolderUnavailable') })
      return
    }
    try {
      const result = await gateway.queueWorkspaceDownloadDirectory(pane.selectedSessionId, entry.key, target.token)
      claimQueuedDirectory(pane.selectedSessionId, result.queued)
      const description =
        result.skipped > 0
          ? t('sftp.ui.queueFilesSkipped', { queued: result.queued, skipped: result.skipped })
          : t('sftp.ui.queueFiles', { queued: result.queued })
      toast.success(t('sftp.ui.folderDownloadCreated'), { description })
    } catch (error) {
      toast.error(t('sftp.ui.downloadFailed'), {
        description: sftpErrorMessage(error, t('sftp.ui.downloadFolderFailed')),
      })
    }
  }

  async function uploadFile(pane: SftpWorkspacePaneState): Promise<void> {
    if (pane.kind !== 'remote' || !pane.selectedSessionId) return
    try {
      const result = await gateway.selectSFTPUploadFile(pane.selectedSessionId, pane.remoteDirectory)
      if (!result.canceled) claimQueuedTransfer(pane.selectedSessionId, result.transfer ?? undefined)
      refresh(pane)
    } catch (error) {
      toast.error(t('sftp.ui.uploadFileFailed'), {
        description: sftpErrorMessage(error, t('sftp.ui.uploadSelectedFailed')),
      })
    }
  }

  async function uploadFolder(pane: SftpWorkspacePaneState): Promise<void> {
    if (pane.kind !== 'remote' || !pane.selectedSessionId) return
    try {
      const result = await gateway.selectSFTPUploadDirectory(pane.selectedSessionId, pane.remoteDirectory)
      if (!result.canceled) claimQueuedDirectory(pane.selectedSessionId, result.queued)
      refresh(pane)
    } catch (error) {
      toast.error(t('sftp.ui.uploadFolderFailed'), {
        description: sftpErrorMessage(error, t('sftp.ui.uploadFolderSelectedFailed')),
      })
    }
  }

  function paneFor(side: 'left' | 'right') {
    return side === 'left' ? left : right
  }

  function startDrag(side: 'left' | 'right', entry: SftpBrowserEntry): void {
    dragged.value = { side, entry }
  }

  async function drop(side: 'left' | 'right', entry?: SftpBrowserEntry): Promise<void> {
    const source = dragged.value
    dragged.value = null
    if (!source || source.side === side) return
    const from = paneFor(source.side)
    const to = paneFor(side)
    if (from.kind === 'local' && to.kind === 'remote') {
      if (!to.selectedSessionId || !from.localDirectory) return
      const remoteDirectory = (entry?.type === 'directory' ? entry.key : '') || to.remoteDirectory
      try {
        if (source.entry.type === 'directory') {
          if (!gateway.queueWorkspaceUploadDirectory) {
            toast.error(t('sftp.ui.uploadFolderUnavailable'), { description: t('sftp.ui.uploadFolderUnsupported') })
            return
          }
          await gateway.queueWorkspaceUploadDirectory(
            to.selectedSessionId,
            from.localDirectory.token,
            source.entry.name,
            remoteDirectory,
          )

          claimSession(to.selectedSessionId)
        } else {
          if (!gateway.queueWorkspaceUpload) {
            toast.error(t('sftp.ui.dragUploadUnavailable'), { description: t('sftp.ui.dragUploadUnsupported') })
            return
          }
          const transfer = await gateway.queueWorkspaceUpload(
            to.selectedSessionId,
            from.localDirectory.token,
            source.entry.name,
            remoteDirectory,
          )
          claimQueuedTransfer(to.selectedSessionId, transfer)
        }
        toast.success(t('sftp.ui.transferTaskCreated'), {
          description: t('sftp.ui.entryQueued', { name: source.entry.name }),
        })
        refresh(to)
      } catch (error) {
        toast.error(t('sftp.ui.uploadFailed'), {
          description: sftpErrorMessage(error, t('sftp.ui.uploadSelectedFailed')),
        })
      }
    } else if (from.kind === 'remote' && to.kind === 'local') {
      if (!from.selectedSessionId || !to.localDirectory) return
      const downloadFile = gateway.queueWorkspaceDownload
      const downloadDirectory = gateway.queueWorkspaceDownloadDirectory
      if (!downloadFile) {
        toast.error(t('sftp.ui.dragDownloadUnavailable'), { description: t('sftp.ui.dragDownloadUnsupported') })
        return
      }
      try {
        let token = to.localDirectory.token
        if (entry?.type === 'directory') {
          const child = await gateway.openLocalDirectory?.(to.localDirectory.token, entry.name)
          if (!child) {
            toast.error(t('sftp.ui.downloadFailed'), { description: t('sftp.ui.targetFolderUnavailable') })
            return
          }
          token = child.token
        }
        if (source.entry.type === 'directory') {
          if (!downloadDirectory) {
            toast.error(t('sftp.ui.downloadFolderUnavailable'), { description: t('sftp.ui.downloadFolderUnavailable') })
            return
          }
          const result = await downloadDirectory(from.selectedSessionId, source.entry.key, token)
          claimQueuedDirectory(from.selectedSessionId, result.queued)
          const description =
            result.skipped > 0
              ? t('sftp.ui.queueFilesSkipped', { queued: result.queued, skipped: result.skipped })
              : t('sftp.ui.queueFiles', { queued: result.queued })
          toast.success(t('sftp.ui.folderDownloadCreated'), { description })
        } else {
          const transfer = await downloadFile(from.selectedSessionId, source.entry.key, token)
          claimQueuedTransfer(from.selectedSessionId, transfer)
          toast.success(t('sftp.ui.transferTaskCreated'), {
            description: t('sftp.ui.entryQueued', { name: source.entry.name }),
          })
        }
        if (entry?.type === 'directory') refresh(to)
      } catch (error) {
        toast.error(t('sftp.ui.downloadFailed'), {
          description: sftpErrorMessage(error, t('sftp.ui.downloadSelectedFailed')),
        })
      }
    } else if (from.kind === 'remote' && to.kind === 'remote') {
      if (!from.selectedSessionId || !to.selectedSessionId) return
      const destDirectory = (entry?.type === 'directory' ? entry.key : '') || to.remoteDirectory
      try {
        if (source.entry.type === 'directory') {
          if (!gateway.queueRemoteTransferDirectory) {
            toast.error(t('sftp.ui.remoteTransferUnavailable'), {
              description: t('sftp.ui.remoteFolderTransferUnsupported'),
            })
            return
          }
          const result = await gateway.queueRemoteTransferDirectory(
            from.selectedSessionId,
            to.selectedSessionId,
            source.entry.key,
            destDirectory,
          )

          claimQueuedDirectory(to.selectedSessionId, result.queued)
          const description =
            result.skipped > 0
              ? t('sftp.ui.queueTransferFilesSkipped', { queued: result.queued, skipped: result.skipped })
              : t('sftp.ui.queueTransferFiles', { queued: result.queued })
          toast.success(t('sftp.ui.folderTransferCreated'), { description })
        } else {
          if (!gateway.queueRemoteTransfer) {
            toast.error(t('sftp.ui.remoteTransferUnavailable'), { description: t('sftp.ui.remoteTransferFileFailed') })
            return
          }
          const transfer = await gateway.queueRemoteTransfer(
            from.selectedSessionId,
            to.selectedSessionId,
            source.entry.key,
            destDirectory,
          )

          claimQueuedTransfer(to.selectedSessionId, transfer)
          toast.success(t('sftp.ui.transferTaskCreated'), {
            description: t('sftp.ui.entryQueued', { name: source.entry.name }),
          })
        }
      } catch (e) {
        toast.error(t('sftp.ui.remoteTransferFailed'), {
          description: sftpErrorMessage(e, t('sftp.ui.remoteTransferFileFailed')),
        })
      }
    }
  }

  const handledCompletedTransfers = new Set<string>()

  const transfersBaseline = Date.now()
  watch(options.transfers, (records) => {
    for (const record of records) {
      if (record.state !== 'completed' || handledCompletedTransfers.has(record.id)) continue
      const finishedAt = Date.parse(record.updatedAt)
      if (!Number.isFinite(finishedAt) || finishedAt < transfersBaseline) {
        handledCompletedTransfers.add(record.id)
        continue
      }
      handledCompletedTransfers.add(record.id)

      for (const pane of [left, right]) {
        if (pane.kind === 'remote' && pane.selectedSessionId && pane.selectedSessionId === record.sessionId) {
          refresh(pane)
        }
      }

      if (record.direction === 'download') {
        for (const pane of [left, right]) {
          if (pane.kind === 'local') refresh(pane)
        }
      }
    }
    if (handledCompletedTransfers.size > 0) {
      const known = new Set(records.map((record) => record.id))
      for (const id of [...handledCompletedTransfers]) {
        if (!known.has(id)) handledCompletedTransfers.delete(id)
      }
    }
  })

  return {
    downloadEntry,
    uploadFile,
    uploadFolder,
    startDrag,
    drop,
  }
}
