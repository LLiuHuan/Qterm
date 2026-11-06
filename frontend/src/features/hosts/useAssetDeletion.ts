import { onScopeDispose, ref } from 'vue'
import { hostErrorMessage, type HostGateway, type HostRecord } from '@/services/hosts.ts'
import type { PendingDelete } from '../../types/qterm'
import { useI18n } from '@/locales'

interface AssetDeletionOptions {
  hostGateway: Pick<HostGateway, 'remove' | 'removeGroup'>
  closeHostSessions: (hostId: string) => Promise<void>
  onDeleted: () => Promise<void>
}

export function useAssetDeletion({ hostGateway, closeHostSessions, onDeleted }: AssetDeletionOptions) {
  const { t } = useI18n()
  const pendingDelete = ref<PendingDelete | null>(null)
  const deleteError = ref('')
  const deletingAsset = ref(false)
  let disposed = false

  function requestDeleteHost(host: HostRecord): void {
    if (disposed || deletingAsset.value) return
    pendingDelete.value = { kind: 'host', id: host.id, name: host.name }
    deleteError.value = ''
  }

  function requestDeleteGroup(group: { id: string; name: string }): void {
    if (disposed || deletingAsset.value) return
    pendingDelete.value = { kind: 'group', id: group.id, name: group.name }
    deleteError.value = ''
  }

  function closeDeleteDialog(): void {
    if (deletingAsset.value) return
    pendingDelete.value = null
    deleteError.value = ''
  }

  async function confirmDelete(): Promise<void> {
    const target = pendingDelete.value
    if (disposed || !target || deletingAsset.value) return
    deletingAsset.value = true
    deleteError.value = ''
    try {
      if (target.kind === 'host') {
        await closeHostSessions(target.id)
        if (disposed) return
        await hostGateway.remove(target.id)
      } else {
        await hostGateway.removeGroup(target.id)
      }
      if (disposed) return
      pendingDelete.value = null
      await onDeleted()
    } catch (error) {
      if (!disposed) deleteError.value = hostErrorMessage(error, t('shell.error.deleteAsset'))
    } finally {
      deletingAsset.value = false
    }
  }

  onScopeDispose(() => {
    disposed = true
    pendingDelete.value = null
  })
  return {
    pendingDelete,
    deleteError,
    deletingAsset,
    requestDeleteHost,
    requestDeleteGroup,
    closeDeleteDialog,
    confirmDelete,
  }
}
