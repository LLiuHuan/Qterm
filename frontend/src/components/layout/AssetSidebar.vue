<script setup lang="ts">
import { QtermButton, QtermInput } from '../common'
import {
  ArrowDownAZ,
  ArrowDownUp,
  ArrowUpZA,
  CircleDot,
  CircleHelp,
  FolderPlus,
  Plus,
  RefreshCw,
  Search,
  Server,
  X,
} from '@lucide/vue'
import { computed, nextTick, ref, watch } from 'vue'
import { useAssetDrag, type AssetDragItem } from '../../features/hosts/assetDrag'
import { IconButton } from '../common'
import AssetDragGhost from './AssetDragGhost.vue'
import AssetHostRow from './AssetHostRow.vue'
import AssetTreeGroup from './AssetTreeGroup.vue'
import type { GroupedHosts } from '../../types/qterm'
import type { HostRecord } from '@/services/hosts.ts'
import { ScrollArea } from '../common/control-primitives'
import { useI18n } from '@/locales'

const props = defineProps<{
  searchVisible: boolean
  searchQuery: string
  sortMode: 'name-asc' | 'name-desc' | 'custom'
  hosts: HostRecord[]
  ungroupedHosts: HostRecord[]
  groupedHosts: GroupedHosts[]
  rootEntries: Array<{ kind: 'host'; value: HostRecord } | { kind: 'group'; value: GroupedHosts }>
  loading: boolean
  loadError: string
  selectedHostId: string | null
  selectedGroupId?: string | null
  isDark: boolean
  showHostAddress: boolean
}>()

const emit = defineEmits<{
  createHost: []
  createHostInGroup: [groupId: string]
  createSubgroup: [groupId: string]
  createGroup: []
  toggleSearch: []
  toggleSort: []
  refresh: []
  selectHost: [hostId: string]
  selectGroup: [groupId: string]
  clearSelection: []
  copyHost: [host: HostRecord, kind: 'ip' | 'id']

  duplicateHost: [host: HostRecord]

  toolPanel: [host: HostRecord, tool: 'tunnels' | 'sftp']
  moveAsset: [assetId: string, groupId: string, targetId: string, position: 'before' | 'after' | 'inside']
  moveGroup: [id: string, parentId: string, targetId: string, position: 'before' | 'after' | 'inside']
  openHost: [host: HostRecord]
  editHost: [host: HostRecord]
  deleteHost: [host: HostRecord]
  editGroup: [group: GroupedHosts]
  deleteGroup: [group: GroupedHosts]
  updateSearchQuery: [value: string]
}>()
const { t } = useI18n()

function containsHost(group: GroupedHosts): boolean {
  return group.items.length > 0 || (group.children?.some(containsHost) ?? false)
}
const hasVisibleHosts = computed(() => props.ungroupedHosts.length > 0 || props.groupedHosts.some(containsHost))
const visibleRootEntries = computed(() =>
  props.rootEntries.filter(
    (entry) =>
      !(
        entry.kind === 'group' &&
        entry.value.id === 'default' &&
        !entry.value.items.length &&
        !entry.value.children?.length
      ),
  ),
)
const scrollAreaRef = ref<InstanceType<typeof ScrollArea> | null>(null)
const rootListRef = ref<HTMLElement | null>(null)

const searchFieldRef = ref<HTMLLabelElement | null>(null)

watch(
  () => props.searchVisible,
  async (visible) => {
    if (!visible) return
    await nextTick()
    if (props.searchVisible) searchFieldRef.value?.querySelector('input')?.focus()
  },
  { immediate: true, flush: 'post' },
)

function clearSearchQuery(event?: MouseEvent): void {
  event?.preventDefault()
  event?.stopPropagation()
  emit('updateSearchQuery', '')

  searchFieldRef.value?.querySelector<HTMLInputElement>('input')?.focus()
}
function findAsset(item: AssetDragItem): HTMLElement | null {
  return (
    Array.from(document.querySelectorAll<HTMLElement>('[data-asset-kind][data-asset-id]')).find(
      (node) => node.dataset.assetKind === item.kind && node.dataset.assetId === item.id,
    ) ?? null
  )
}
function parentOf(item: AssetDragItem): string {
  return findAsset(item)?.parentElement?.closest<HTMLElement>('.asset-group')?.dataset.assetId ?? ''
}
const assetDrag = useAssetDrag(rootListRef, parentOf, (item, drop) => {
  if (item.kind === 'host') emit('moveAsset', item.id, drop.groupId, drop.targetId, drop.position)
  else emit('moveGroup', item.id, drop.groupId, drop.targetId, drop.position)
})

const draggingHost = computed(() => {
  const item = assetDrag.state.item
  if (item?.kind !== 'host') return null
  return props.hosts.find((host) => host.id === item.id) ?? null
})

function forwardCopy(host: HostRecord, kind: 'ip' | 'id'): void {
  emit('copyHost', host, kind)
}

function forwardDuplicate(host: HostRecord): void {
  emit('duplicateHost', host)
}
</script>

<template>
  <aside class="sidebar" :aria-label="t('shell.asset.aria')">
    <div class="sidebar-header">
      <div class="sidebar-title min-w-0 flex items-center gap-[7px] font-medium whitespace-nowrap">
        <Server :size="16" aria-hidden="true" />
        <span>{{ t('shell.asset.title') }}</span>
      </div>
      <div class="sidebar-header-actions min-w-0 ml-auto flex items-center">
        <IconButton :label="t('shell.asset.addHost')" @click="emit('createHost')">
          <Plus :size="16" aria-hidden="true" />
        </IconButton>
        <IconButton :label="t('shell.asset.createGroup')" @click="emit('createGroup')">
          <FolderPlus :size="16" aria-hidden="true" />
        </IconButton>
      </div>
    </div>

    <div
      class="sidebar-tools row-[2] min-h-[2.625rem] gap-[0.125rem] pt-[0.3125rem] pr-[0.4375rem] pb-[0.3125rem] pl-[0.4375rem]"
    >
      <IconButton
        :label="t('shell.asset.searchHosts')"
        :active="props.searchVisible"
        :pressed="props.searchVisible"
        @click="emit('toggleSearch')"
      >
        <Search :size="16" aria-hidden="true" />
      </IconButton>
      <IconButton
        :label="t('shell.asset.toggleSort')"
        :active="props.sortMode !== 'custom'"
        :pressed="props.sortMode !== 'custom'"
        @click="emit('toggleSort')"
      >
        <component
          :is="props.sortMode === 'name-asc' ? ArrowDownAZ : props.sortMode === 'name-desc' ? ArrowUpZA : ArrowDownUp"
          :size="16"
          aria-hidden="true"
        />
      </IconButton>
      <IconButton :label="t('shell.asset.filterDisabled')" disabled>
        <CircleDot :size="16" aria-hidden="true" />
      </IconButton>
      <div class="flex-1"></div>
      <IconButton :label="t('shell.asset.refresh')" @click="emit('refresh')">
        <RefreshCw :size="16" aria-hidden="true" />
      </IconButton>
    </div>

    <label
      ref="searchFieldRef"
      v-if="props.searchVisible"
      class="search-field row-[3] relative min-w-0 block mt-0 mr-[8px] mb-[5px] ml-[8px]"
    >
      <Search class="search-field-icon" :size="15" aria-hidden="true" />
      <span class="sr-only">{{ t('shell.asset.searchLabel') }}</span>
      <QtermInput
        :model-value="props.searchQuery"
        class="search-input qterm-reference-input"
        :class="{ 'pr-8': props.searchQuery !== '' }"
        type="search"
        :placeholder="t('shell.asset.searchPlaceholder')"
        autofocus
        @update:model-value="(value) => emit('updateSearchQuery', String(value))"
      />

      <span
        v-if="props.searchQuery !== ''"
        class="asset-search-clear absolute top-1/2 right-1 z-1 flex -translate-y-1/2"
      >
        <IconButton size="icon-xs" :label="t('shell.asset.clearSearch')" @click="clearSearchQuery">
          <X :size="14" aria-hidden="true" />
        </IconButton>
      </span>
    </label>

    <div
      class="asset-list row-start-4 min-h-0 p-0 overflow-hidden relative"
      :class="{ 'asset-list--empty': visibleRootEntries.length === 0 }"
      @click.self="emit('clearSelection')"
    >
      <ScrollArea ref="scrollAreaRef" class="h-full" type="hover" :scroll-hide-delay="600">
        <div class="asset-scroll-content px-[7px] pt-1 pb-[10px]">
          <div
            v-if="props.loading"
            class="asset-empty text-qt-muted asset-empty--fill flex min-h-[180px] flex-col items-center justify-center gap-2 px-3 py-[26px] text-center"
            aria-live="polite"
          >
            <RefreshCw class="spin" :size="22" aria-hidden="true" />
            <p class="m-0">{{ t('shell.asset.loading') }}</p>
          </div>
          <div
            v-else-if="props.loadError"
            class="asset-empty text-qt-muted asset-empty--fill flex min-h-[180px] flex-col items-center justify-center gap-2 px-3 py-[26px] text-center"
            role="alert"
          >
            <CircleHelp :size="22" aria-hidden="true" />
            <p class="m-0">{{ props.loadError }}</p>
            <QtermButton variant="link" size="sm" type="button" @click="emit('refresh')">{{
              t('shell.asset.retry')
            }}</QtermButton>
          </div>

          <div
            v-else-if="props.hosts.length === 0 && visibleRootEntries.length === 0"
            class="asset-empty text-qt-muted asset-empty--fill flex min-h-28 flex-col items-center justify-center gap-2 px-3 py-[14px] text-center"
          >
            <Server :size="22" aria-hidden="true" />
            <p class="m-0">{{ t('shell.asset.empty') }}</p>
            <QtermButton variant="link" size="sm" type="button" @click="emit('createHost')">{{
              t('shell.asset.addFirst')
            }}</QtermButton>
          </div>

          <div
            v-else-if="props.hosts.length > 0 && !hasVisibleHosts && props.searchQuery.trim().length > 0"
            class="asset-empty text-qt-muted asset-empty--fill flex min-h-[180px] flex-col items-center justify-center gap-2 px-3 py-[26px] text-center"
          >
            <Search :size="22" aria-hidden="true" />
            <p class="m-0">{{ t('shell.asset.noMatch') }}</p>
          </div>

          <div
            v-if="!props.loading && !props.loadError && visibleRootEntries.length > 0"
            ref="rootListRef"
            class="relative"
            data-asset-parent=""
            @pointerdown.capture="assetDrag.start"
          >
            <div
              v-if="assetDrag.state.active && assetDrag.state.line"
              class="absolute h-[2px] rounded-[1px] bg-qt-accent pointer-events-none z-5"
              :style="{
                top: `${assetDrag.state.line.top}px`,
                left: `${assetDrag.state.line.left}px`,
                width: `${assetDrag.state.line.width}px`,
              }"
            ></div>
            <template v-for="entry in visibleRootEntries" :key="`${entry.kind}-${entry.value.id}`">
              <AssetTreeGroup
                v-if="entry.kind === 'group'"
                :group="entry.value"
                :selected-host-id="props.selectedHostId"
                :selected-group-id="props.selectedGroupId"
                :show-address="props.showHostAddress"
                @select-host="emit('selectHost', $event)"
                @open-host="emit('openHost', $event)"
                @edit-host="emit('editHost', $event)"
                @delete-host="emit('deleteHost', $event)"
                @select-group="emit('selectGroup', $event)"
                @create-host-in-group="emit('createHostInGroup', $event)"
                @create-subgroup="emit('createSubgroup', $event)"
                @edit-group="emit('editGroup', $event)"
                @delete-group="emit('deleteGroup', $event)"
                @copy-host="forwardCopy"
                @duplicate-host="forwardDuplicate"
                @tool-panel="(host, tool) => emit('toolPanel', host, tool)"
                @move-asset="(...args) => emit('moveAsset', ...args)"
                @move-group="(...args) => emit('moveGroup', ...args)"
              />
              <AssetHostRow
                v-else
                :host="entry.value"
                :selected="props.selectedHostId === entry.value.id"
                :show-address="props.showHostAddress"
                @select="emit('selectHost', $event)"
                @open="emit('openHost', $event)"
                @edit="emit('editHost', $event)"
                @delete="emit('deleteHost', $event)"
                @copy="forwardCopy"
                @duplicate="forwardDuplicate"
                @tool-panel="(host, tool) => emit('toolPanel', host, tool)"
              />
            </template>
          </div>
        </div>
      </ScrollArea>
    </div>

    <AssetDragGhost
      v-if="assetDrag.state.active || assetDrag.state.returnTo"
      :kind="assetDrag.state.item?.kind ?? 'host'"
      :host="draggingHost"
      :label="assetDrag.state.label"
      :valid="assetDrag.state.valid"
      :x="assetDrag.state.x"
      :y="assetDrag.state.y"
      :anchor-x="assetDrag.state.anchorX"
      :anchor-y="assetDrag.state.anchorY"
      :source-width="assetDrag.state.sourceWidth"
      :return-to="assetDrag.state.returnTo"
      @returned="assetDrag.clearReturn()"
    />
  </aside>
</template>
