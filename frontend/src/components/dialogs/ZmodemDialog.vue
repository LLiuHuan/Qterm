<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import TransferProgressPanel, { type TransferProgressItem } from '../common/feedback/TransferProgressPanel.vue'
import type { ZmodemDirection } from '../../services/zmodem'
import { useI18n } from '@/locales'

interface ZmodemTransfer {
  sessionId: string
  direction: ZmodemDirection
  fileName: string
  fileSize?: number
  progress: number
  state: 'detected' | 'transferring' | 'saving' | 'complete' | 'error' | 'retracted'
  message?: string
}

const transfersMap = ref<Record<string, ZmodemTransfer>>({})
const timers = new Map<string, ReturnType<typeof setTimeout>>()
const { t } = useI18n()
function transferLabel(transfer: ZmodemTransfer): string {
  if (transfer.direction === 'upload') {
    if (transfer.state === 'detected') return t('shell.zmodem.waitingSelect')
    if (transfer.state === 'transferring') return t('shell.zmodem.uploading')
    if (transfer.state === 'complete') return t('shell.zmodem.uploaded')
  }
  if (transfer.state === 'detected') return t('shell.zmodem.waitingReceive')
  if (transfer.state === 'transferring') return t('shell.zmodem.downloading')
  if (transfer.state === 'saving') return t('shell.zmodem.saving')
  if (transfer.state === 'complete') return t('shell.zmodem.saved')
  if (transfer.state === 'error') return t('shell.zmodem.failed')
  return t('shell.zmodem.cancelled')
}
const progressItems = computed<TransferProgressItem[]>(() =>
  Object.values(transfersMap.value)
    .filter((transfer) => !['detected', 'retracted'].includes(transfer.state))
    .map((transfer) => ({
      id: transfer.sessionId,
      name: transfer.fileName || t('shell.zmodem.waitingFile'),
      direction: transfer.direction,
      progress: transfer.progress,
      label: transfer.message || transferLabel(transfer),
      completed: transfer.state === 'complete',
      action: ['detected', 'transferring', 'saving'].includes(transfer.state) ? 'cancel' : 'dismiss',
    })),
)

function clearTimer(sessionId: string): void {
  clearTimeout(timers.get(sessionId))
  timers.delete(sessionId)
}

function onDownloadOffer(sessionId: string, name: string, size: number): void {
  clearTimer(sessionId)
  transfersMap.value[sessionId] = {
    sessionId,
    direction: 'download',
    fileName: name,
    fileSize: size,
    progress: 0,
    state: 'detected',
  }
}

function onProgress(info: {
  sessionId: string
  direction: ZmodemDirection
  state: string
  fileName?: string
  progress?: number
  message?: string
}): void {
  const states = ['detected', 'transferring', 'saving', 'complete', 'error', 'retracted']
  if (!states.includes(info.state)) return
  const existing = transfersMap.value[info.sessionId]
  clearTimer(info.sessionId)
  if (info.state === 'retracted') {
    delete transfersMap.value[info.sessionId]
    return
  }
  const transfer: ZmodemTransfer = {
    ...existing,
    sessionId: info.sessionId,
    direction: info.direction,
    fileName: info.fileName ?? existing?.fileName ?? '',
    progress: info.state === 'complete' ? 100 : (info.progress ?? existing?.progress ?? 0),
    state: info.state as ZmodemTransfer['state'],
    message: info.message,
  }
  transfersMap.value[info.sessionId] = transfer
  if (['complete', 'error', 'retracted'].includes(info.state)) {
    if (info.state !== 'error')
      timers.set(
        info.sessionId,
        setTimeout(() => {
          delete transfersMap.value[info.sessionId]
          timers.delete(info.sessionId)
        }, 3000),
      )
  }
}

function removeTransfer(key: string): void {
  const transfer = transfersMap.value[key]
  if (transfer && ['detected', 'transferring', 'saving'].includes(transfer.state)) {
    window.dispatchEvent(new CustomEvent('qterm:zmodem-cancel', { detail: { sessionId: transfer.sessionId } }))
  }
  clearTimer(key)
  delete transfersMap.value[key]
}

onBeforeUnmount(() => {
  for (const timer of timers.values()) clearTimeout(timer)
  timers.clear()
})
defineExpose({ onDownloadOffer, onProgress, removeTransfer })
</script>

<template>
  <TransferProgressPanel :items="progressItems" @action="removeTransfer" />
</template>
