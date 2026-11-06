<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'
import { toast } from 'vue-sonner'
import { requestTransferConflict } from '../../services/transfer-conflicts'
import { sftpErrorMessage, type SftpGateway, type SFTPTransferRecord } from '../../services/sftp'
import {
  claimConflictAnswer,
  currentOwnership,
  refreshSharedOwnership,
  renewConflictAnswer,
  shouldHandleConflict,
} from '../../services/transferOwnership'
import { useI18n } from '@/locales'
const props = defineProps<{
  gateway: SftpGateway

  primaryWindow: boolean
}>()
const { t } = useI18n()
let timer: ReturnType<typeof setInterval> | undefined
let loading = false
let disposed = false
let active: { id: string; controller: AbortController } | undefined

const failedIds = new Set<string>()

function conflictAgeMs(record: SFTPTransferRecord): number {
  const started = Date.parse(record.updatedAt)
  if (Number.isNaN(started)) return Number.POSITIVE_INFINITY
  return Math.max(0, Date.now() - started)
}

async function refresh(): Promise<void> {
  if (loading || disposed) return

  if (!props.gateway.resolveSFTPTransferConflict) return
  loading = true
  try {
    refreshSharedOwnership()
    const records = await props.gateway.listSFTPTransfers()
    if (disposed) return

    if (active) renewConflictAnswer(active.id)
    for (const id of [...failedIds]) {
      if (!records.some((record) => record.id === id)) failedIds.delete(id)
    }
    if (active && !records.some((record) => record.id === active?.id && record.state === 'awaitingOverwrite')) {
      active.controller.abort()
      active = undefined
    }
    if (active) return
    const owned = currentOwnership()
    const record = records.find(
      (item) =>
        item.state === 'awaitingOverwrite' &&
        !failedIds.has(item.id) &&
        shouldHandleConflict(item, owned, {
          primaryWindow: props.primaryWindow,
          conflictAgeMs: conflictAgeMs(item),
        }),
    )
    if (!record) return

    if (!claimConflictAnswer(record.id)) return
    const request = { id: record.id, controller: new AbortController() }
    active = request
    void (async () => {
      try {
        const name = record.direction === 'upload' ? record.remotePath.split('/').pop()! : record.localName
        const action = await requestTransferConflict(name, request.controller.signal)
        if (!request.controller.signal.aborted) {
          await props.gateway.resolveSFTPTransferConflict?.(record.id, action)
        }
      } catch (error) {
        failedIds.add(record.id)
        if (!disposed) toast.error(sftpErrorMessage(error, t('sftp.ui.conflictResolveFailed')))
      } finally {
        if (active === request) active = undefined
      }
    })()
  } catch {
  } finally {
    loading = false
  }
}
onMounted(() => {
  void refresh()
  timer = setInterval(() => void refresh(), 800)
})
onBeforeUnmount(() => {
  disposed = true
  clearInterval(timer)
  active?.controller.abort()
})
</script>
<template><span hidden /></template>
