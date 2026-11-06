import { computed, onBeforeUnmount, onMounted, ref, type Ref } from 'vue'
import { claimSession, claimTransfer } from '../../services/transferOwnership'
import {
  sftpErrorMessage,
  type DirectoryUploadResult,
  type RemoteFileEntry,
  type SftpGateway,
  type SFTPTransferRecord,
  type SFTPTransferSelectionResult,
  type SFTPTransferState,
} from '../../services/sftp'
import type { TransferProgressItem } from '../../components/common/feedback/TransferProgressPanel.vue'
import {
  transferEtaText,
  transferQueueAction,
  transferSecondaryAction,
  transferSpeedText,
} from '../../services/transferQueue'

type Translate = (key: string, params?: Record<string, string | number>) => string

export interface UseSftpPanelTransfersOptions {
  gateway: () => SftpGateway
  sessionId: () => string
  available: () => boolean
  directory: Ref<string>

  sessionGeneration: () => number
  isCurrentSessionOperation: (generation: number, sessionId: string) => boolean
  loadDirectory: (directory: string) => Promise<void>
  t: Translate
}

export function useSftpPanelTransfers(options: UseSftpPanelTransfersOptions) {
  const transferError = ref('')
  const transfers = ref<SFTPTransferRecord[]>([])
  const legacyOverwriteTransfer = ref<SFTPTransferRecord | null>(null)
  const transfersLoading = ref(false)
  const filePickerBusy = ref(false)
  let transferRequestSequence = 0
  let transferTimer: ReturnType<typeof setInterval> | undefined

  const completedTransferIds = new Set<string>()
  let transfersInitialized = false

  const dismissedTransferIds = ref<Set<string>>(new Set())
  const sessionTransfers = computed(() =>
    transfers.value
      .filter(
        (transfer) =>
          transfer.sessionId === options.sessionId() &&
          transfer.state !== 'completed' &&
          transfer.state !== 'canceled' &&
          !dismissedTransferIds.value.has(transfer.id),
      )
      .reverse(),
  )
  const progressItems = computed<TransferProgressItem[]>(() =>
    sessionTransfers.value.map((transfer) => ({
      id: transfer.id,
      name: transfer.localName,
      direction: transfer.direction,
      progress: transferProgress(transfer),
      label: transferStateLabel(transfer.state),
      stage: transfer.stage,
      speed: transfer.state === 'running' ? transferSpeedText(transfer.rateBytesPerSecond) : undefined,
      eta:
        transfer.state === 'running'
          ? transferEtaText(transfer.total - transfer.transferred, transfer.rateBytesPerSecond)
          : undefined,
      action: transferAction(transfer),
      secondaryAction: transferSecondaryAction(transfer),
    })),
  )

  function registerTransferOwnership(
    sessionID: string,
    result: DirectoryUploadResult | SFTPTransferSelectionResult,
  ): void {
    if ('transfer' in result) {
      if (result.transfer) claimTransfer(result.transfer.id)
      else claimSession(sessionID)
      return
    }
    if (result.queued > 0) claimSession(sessionID)
  }

  async function selectUpload(folder = false): Promise<void> {
    const sessionID = options.sessionId()
    const generation = options.sessionGeneration()
    if (!options.available() || !sessionID || filePickerBusy.value) return
    const remoteDirectory = options.directory.value
    filePickerBusy.value = true
    transferError.value = ''
    try {
      const result = folder
        ? await options.gateway().selectSFTPUploadDirectory(sessionID, remoteDirectory)
        : await options.gateway().selectSFTPUploadFile(sessionID, remoteDirectory)

      if (!result.canceled) registerTransferOwnership(sessionID, result)
      if (!options.isCurrentSessionOperation(generation, sessionID)) return
      if (!result.canceled) {
        await refreshTransfers()
        if (folder) await options.loadDirectory(remoteDirectory)
      }
    } catch (reason) {
      if (!options.isCurrentSessionOperation(generation, sessionID)) return
      transferError.value = sftpErrorMessage(reason, options.t('sftp.ui.uploadSelectedFailed'))
      await refreshTransfers()
    } finally {
      filePickerBusy.value = false
    }
  }

  async function selectDownload(entry: Pick<RemoteFileEntry, 'path' | 'type'>): Promise<void> {
    const sessionID = options.sessionId()
    const generation = options.sessionGeneration()
    if (!options.available() || !sessionID || entry.type !== 'file' || filePickerBusy.value) return
    const remotePath = entry.path
    filePickerBusy.value = true
    transferError.value = ''
    try {
      const result = await options.gateway().selectSFTPDownloadFile(sessionID, remotePath)
      if (!result.canceled) registerTransferOwnership(sessionID, result)
      if (!options.isCurrentSessionOperation(generation, sessionID)) return
      if (!result.canceled) await refreshTransfers()
    } catch (reason) {
      if (!options.isCurrentSessionOperation(generation, sessionID)) return
      transferError.value = sftpErrorMessage(reason, options.t('sftp.ui.downloadSelectedFailed'))
    } finally {
      filePickerBusy.value = false
    }
  }

  async function refreshTransfers(): Promise<void> {
    if (!options.available() || transfersLoading.value) return
    const sessionID = options.sessionId()
    const generation = options.sessionGeneration()
    const sequence = ++transferRequestSequence
    transfersLoading.value = true
    try {
      const result = await options.gateway().listSFTPTransfers()
      if (!options.isCurrentSessionOperation(generation, sessionID) || sequence !== transferRequestSequence) return
      transfers.value = result
      legacyOverwriteTransfer.value =
        result.find((transfer) => transfer.sessionId === sessionID && transfer.state === 'awaitingOverwrite') ?? null
      let completedNow = false
      for (const transfer of result) {
        if (transfer.sessionId !== sessionID || transfer.state !== 'completed') continue
        if (completedTransferIds.has(transfer.id)) continue
        completedTransferIds.add(transfer.id)

        if (transfersInitialized) completedNow = true
      }
      if (!transfersInitialized) transfersInitialized = true
      else if (completedNow) void options.loadDirectory(options.directory.value)
    } catch (reason) {
      if (!options.isCurrentSessionOperation(generation, sessionID) || sequence !== transferRequestSequence) return
      transferError.value = sftpErrorMessage(reason, options.t('sftp.ui.operationFailed'))
    } finally {
      if (sequence === transferRequestSequence) transfersLoading.value = false
    }
  }

  async function runTransferAction(action: () => Promise<unknown>): Promise<void> {
    transferError.value = ''
    try {
      await action()
      await refreshTransfers()
    } catch (reason) {
      transferError.value = sftpErrorMessage(reason, options.t('sftp.ui.operationFailed'))
    }
  }

  async function retryTransfer(transfer: SFTPTransferRecord): Promise<void> {
    await runTransferAction(() => options.gateway().retrySFTPTransfer(transfer.id))
  }

  async function cancelTransfer(transfer: SFTPTransferRecord): Promise<void> {
    await runTransferAction(() => options.gateway().cancelSFTPTransfer(transfer.id))
  }

  function transferAction(transfer: SFTPTransferRecord): TransferProgressItem['action'] {
    return transferQueueAction(transfer, options.gateway())
  }

  async function pauseTransfer(transfer: SFTPTransferRecord): Promise<void> {
    const gateway = options.gateway()
    if (!gateway.pauseSFTPTransfer) return
    await runTransferAction(() => gateway.pauseSFTPTransfer!(transfer.id))
  }

  async function resumeTransfer(transfer: SFTPTransferRecord): Promise<void> {
    const gateway = options.gateway()
    if (!gateway.resumeSFTPTransfer) return
    await runTransferAction(() => gateway.resumeSFTPTransfer!(transfer.id))
  }

  function handleTransferAction(id: string, action?: TransferProgressItem['action']): void {
    const transfer = sessionTransfers.value.find((item) => item.id === id)
    if (!transfer) return

    switch (action ?? transferAction(transfer)) {
      case 'retry':
        void retryTransfer(transfer)
        return
      case 'pause':
        void pauseTransfer(transfer)
        return
      case 'resume':
        void resumeTransfer(transfer)
        return
      default:
        void cancelTransfer(transfer)
    }
  }

  function dismissTransfer(id: string): void {
    const next = new Set(dismissedTransferIds.value)
    next.add(id)
    dismissedTransferIds.value = next
  }

  function resetForSession(): void {
    transferRequestSequence += 1
    transfers.value = []
    legacyOverwriteTransfer.value = null
    transfersLoading.value = false
    transferError.value = ''
    completedTransferIds.clear()
    transfersInitialized = false
  }

  function transferProgress(transfer: SFTPTransferRecord): number {
    if (transfer.total <= 0) return transfer.state === 'completed' ? 100 : 0
    return Math.min(100, Math.round((transfer.transferred / transfer.total) * 100))
  }

  function transferStateLabel(state: SFTPTransferState): string {
    const labels: Record<SFTPTransferState, string> = {
      queued: options.t('sftp.ui.transfer.queued'),
      awaitingOverwrite: options.t('sftp.ui.transfer.awaitingOverwrite'),
      running: options.t('sftp.ui.transfer.running'),
      paused: options.t('sftp.ui.transfer.paused'),
      completed: options.t('sftp.ui.transfer.completed'),
      failed: options.t('sftp.ui.transfer.failed'),
      canceled: options.t('sftp.ui.transfer.canceled'),
    }
    return labels[state]
  }

  onMounted(() => {
    transferTimer = setInterval(() => {
      if (options.available() && options.sessionId()) void refreshTransfers()
    }, 800)
  })

  onBeforeUnmount(() => {
    transferRequestSequence += 1
    if (transferTimer) clearInterval(transferTimer)
  })

  return {
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
  }
}
