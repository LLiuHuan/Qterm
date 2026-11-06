import { computed, onScopeDispose, ref } from 'vue'
import { toast } from 'vue-sonner'
import { hostErrorMessage, type GroupRecord, type HostGateway, type HostRecord } from '@/services/hosts.ts'
import { buildAssetTree, type AssetSortMode } from './assetTree'
import { useI18n } from '@/locales'

export function useAssetState(hostGateway: HostGateway) {
  const { t } = useI18n()
  const hosts = ref<HostRecord[]>([])
  const groups = ref<GroupRecord[]>([])
  const selectedHostId = ref<string | null>(null)
  const selectedGroupId = ref<string | null>(null)
  const searchVisible = ref(false)
  const searchQuery = ref('')
  const sortMode = ref<AssetSortMode>('custom')
  const hostLoadError = ref('')
  const loadingHosts = ref(true)
  let loadVersion = 0
  let disposed = false
  const assetTree = computed(() => buildAssetTree(hosts.value, groups.value, searchQuery.value, sortMode.value))
  const ungroupedHosts = computed(() => assetTree.value.ungroupedHosts)
  const groupedHosts = computed(() => assetTree.value.groupedHosts)
  const rootEntries = computed(() => assetTree.value.rootEntries)

  async function loadHosts(): Promise<void> {
    if (disposed) return
    const version = ++loadVersion
    loadingHosts.value = true
    hostLoadError.value = ''
    try {
      const [hostRecords, groupRecords] = await Promise.all([hostGateway.list(), hostGateway.listGroups()])
      if (disposed || version !== loadVersion) return
      hosts.value = hostRecords
      groups.value = groupRecords
      if (!hosts.value.some((host) => host.id === selectedHostId.value)) selectedHostId.value = null
      if (!groups.value.some((group) => group.id === selectedGroupId.value)) selectedGroupId.value = null
    } catch (error) {
      if (!disposed && version === loadVersion)
        hostLoadError.value = hostErrorMessage(error, t('shell.error.loadHosts'))
    } finally {
      if (!disposed && version === loadVersion) loadingHosts.value = false
    }
  }

  function upsertHost(host: HostRecord): void {
    if (disposed) return

    loadVersion += 1
    loadingHosts.value = false
    const index = hosts.value.findIndex((item) => item.id === host.id)
    if (index >= 0) hosts.value.splice(index, 1, host)
    else hosts.value.push(host)
    selectHost(host.id)
  }

  function selectHost(id: string): void {
    selectedHostId.value = id
    selectedGroupId.value = null
  }

  function selectGroup(id: string): void {
    selectedGroupId.value = id
    selectedHostId.value = null
  }

  function clearAssetSelection(): void {
    selectedHostId.value = null
    selectedGroupId.value = null
  }

  function toggleSort(): void {
    sortMode.value =
      sortMode.value === 'name-asc' ? 'name-desc' : sortMode.value === 'name-desc' ? 'custom' : 'name-asc'
  }

  async function moveAsset(
    id: string,
    groupId: string,
    targetId: string,
    position: 'before' | 'after' | 'inside',
  ): Promise<void> {
    sortMode.value = 'custom'
    try {
      await hostGateway.reorderHost(id, groupId, targetId, position)
    } catch (error) {
      if (!disposed) toast.error(hostErrorMessage(error, t('shell.app.moveHostFailed')))
    } finally {
      await loadHosts()
    }
  }

  async function moveGroup(
    id: string,
    parentId: string,
    targetId: string,
    position: 'before' | 'after' | 'inside',
  ): Promise<void> {
    sortMode.value = 'custom'
    try {
      await hostGateway.reorderGroup(id, parentId, targetId, position)
    } catch (error) {
      if (!disposed) toast.error(hostErrorMessage(error, t('shell.app.moveGroupFailed')))
    } finally {
      await loadHosts()
    }
  }

  onScopeDispose(() => {
    disposed = true
    loadVersion += 1
  })
  return {
    hosts,
    groups,
    selectedHostId,
    selectedGroupId,
    searchVisible,
    searchQuery,
    sortMode,
    hostLoadError,
    loadingHosts,
    ungroupedHosts,
    groupedHosts,
    rootEntries,
    loadHosts,
    upsertHost,
    selectHost,
    selectGroup,
    clearAssetSelection,
    toggleSort,
    moveAsset,
    moveGroup,
  }
}
