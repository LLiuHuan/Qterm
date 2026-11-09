<script setup lang="ts">
import { IconButton, QtermButton, QtermInput } from '../common'
import {
  ArrowDownAZ,
  ArrowDownUp,
  ArrowLeft,
  ArrowRightLeft,
  ArrowUpZA,
  ChevronRight,
  CircleHelp,
  Copy,
  Folder,
  FolderClosed,
  FolderPlus,
  Link,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  Server,
  Trash2,
} from '@lucide/vue'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import AssetContextMenu, { type AssetMenuAction } from '../common/overlay-controls/AssetContextMenu.vue'
import AssetDragGhost from './AssetDragGhost.vue'
import HostIcon from '../common/icon/HostIcon.vue'
import { tagColorStyle } from '../../lib/colorTag'
import type { GroupedHosts } from '../../types/qterm'
import type { HostRecord } from '@/services/hosts.ts'
import { compareAsset } from '../../features/hosts/assetTree'
import { useAssetDrag, type AssetDragItem } from '../../features/hosts/assetDrag'
import { ScrollArea } from '../common/control-primitives'
import { hiddenHostTagCount, hostConnectionSummary, hostTags, visibleHostTags } from '@/features/hosts/hostSummary'
import { useI18n } from '@/locales'

const props = defineProps<{
  searchVisible: boolean
  searchQuery: string
  sortMode: 'name-asc' | 'name-desc' | 'custom'
  hosts: HostRecord[]
  ungroupedHosts: HostRecord[]
  groupedHosts: GroupedHosts[]
  loading: boolean
  loadError: string
  selectedHostId: string | null

  selectedGroupId?: string | null
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
  openHost: [host: HostRecord]
  editHost: [host: HostRecord]
  deleteHost: [host: HostRecord]
  editGroup: [group: GroupedHosts]
  deleteGroup: [group: GroupedHosts]
  copyHost: [host: HostRecord, kind: 'ip' | 'id']

  duplicateHost: [host: HostRecord]

  moveAsset: [assetId: string, groupId: string, targetId: string, position: 'before' | 'after' | 'inside']
  moveGroup: [id: string, parentId: string, targetId: string, position: 'before' | 'after' | 'inside']

  toolPanel: [host: HostRecord, tool: 'tunnels' | 'sftp']
  updateSearchQuery: [value: string]
}>()
const { t } = useI18n()
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

function hostColorStyle(host: { color?: string }): Record<string, string> | undefined {
  return tagColorStyle(host.color) as Record<string, string> | undefined
}

const currentGroupId = ref<string | null>(null)

function findGroupPath(groups: GroupedHosts[], id: string, trail: GroupedHosts[] = []): GroupedHosts[] {
  for (const group of groups) {
    const next = [...trail, group]
    if (group.id === id) return next
    const found = findGroupPath(group.children ?? [], id, next)
    if (found.length > 0) return found
  }
  return []
}

const groupPath = computed<GroupedHosts[]>(() =>
  currentGroupId.value ? findGroupPath(props.groupedHosts, currentGroupId.value) : [],
)
const currentGroup = computed(() => groupPath.value.at(-1) ?? null)

watch(groupPath, (path) => {
  if (currentGroupId.value !== null && path.length === 0) currentGroupId.value = null
})

const visibleGroups = computed<GroupedHosts[]>(() => {
  if (currentGroup.value) return currentGroup.value.children ?? []
  return props.groupedHosts.filter(
    (group) => !(group.id === 'default' && !group.items.length && !group.children?.length),
  )
})

function collectHosts(group: GroupedHosts): HostRecord[] {
  return [...group.items, ...(group.children ?? []).flatMap(collectHosts)]
}

const visibleHosts = computed<HostRecord[]>(() => {
  const flattened = currentGroup.value
    ? collectHosts(currentGroup.value)
    : [...props.ungroupedHosts, ...props.groupedHosts.flatMap(collectHosts)]
  return props.sortMode === 'custom'
    ? flattened
    : [...flattened].sort((left, right) => compareAsset(left, right, props.sortMode))
})

const sortIcon = computed(() =>
  props.sortMode === 'name-asc' ? ArrowDownAZ : props.sortMode === 'name-desc' ? ArrowUpZA : ArrowDownUp,
)

const searchActive = computed(() => props.searchQuery.trim() !== '')

const breadcrumbs = computed<Array<{ id: string | null; name: string }>>(() => [
  { id: null, name: t('shell.assetLibrary.home') },
  ...groupPath.value.map((group) => ({ id: group.id, name: group.name })),
])

function hostCount(group: GroupedHosts): number {
  return group.items.length + (group.children?.reduce((total, child) => total + hostCount(child), 0) ?? 0)
}

function connectionOf(host: HostRecord): string {
  return hostConnectionSummary(host)
}

function withHostTags(base: string, host: HostRecord): string {
  const tags = hostTags(host)
  return tags.length > 0 ? `${base} · ${tags.join('、')}` : base
}

function hostSubtitle(host: HostRecord): string {
  return connectionOf(host)
}

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  if (target.isContentEditable) return true
  return target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.tagName === 'SELECT'
}

const rootRef = ref<HTMLElement | null>(null)

function isLibraryVisible(): boolean {
  if (!rootRef.value) return false
  for (let node: HTMLElement | null = rootRef.value; node; node = node.parentElement) {
    const style = window.getComputedStyle(node)
    if (style.display === 'none' || style.visibility === 'hidden') return false
  }
  return true
}

function suppressBareBackspace(event: KeyboardEvent): void {
  if (event.key !== 'Backspace' || event.defaultPrevented || event.isComposing) return
  if (event.metaKey || event.ctrlKey || event.altKey) return
  if (isEditableTarget(event.target)) return
  if (!isLibraryVisible()) return
  event.preventDefault()
}

let backspaceGuardBound = false
let disposed = false

onMounted(() => {
  void nextTick(() => {
    if (disposed || backspaceGuardBound) return
    backspaceGuardBound = true
    window.addEventListener('keydown', suppressBareBackspace)
  })
})

onBeforeUnmount(() => {
  disposed = true
  if (!backspaceGuardBound) return
  backspaceGuardBound = false
  window.removeEventListener('keydown', suppressBareBackspace)
})

function hostTitle(host: HostRecord): string {
  return withHostTags(t('shell.asset.hostTitle', { connection: connectionOf(host) }), host)
}

function hostAriaLabel(host: HostRecord): string {
  return withHostTags(t('shell.asset.hostAria', { name: host.name, connection: connectionOf(host) }), host)
}

function selectGroupCard(group: GroupedHosts): void {
  emit('selectGroup', group.id)
}

function openGroup(group: GroupedHosts): void {
  currentGroupId.value = group.id
  emit('selectGroup', group.id)
}

function parentOfAsset(item: AssetDragItem): string {
  if (item.kind === 'group') return findGroupPath(props.groupedHosts, item.id).at(-2)?.id ?? ''
  const groupId = props.hosts.find((host) => host.id === item.id)?.groupId ?? ''
  return groupId === 'default' ? '' : groupId
}

const gridRef = ref<HTMLElement | null>(null)
const assetDrag = useAssetDrag(gridRef, parentOfAsset, (item, drop) => {
  if (item.kind === 'host') emit('moveAsset', item.id, drop.groupId, drop.targetId, drop.position)
  else emit('moveGroup', item.id, drop.groupId, drop.targetId, drop.position)
})

const draggingHost = computed(() => {
  const item = assetDrag.state.item
  if (item?.kind !== 'host') return null
  return props.hosts.find((host) => host.id === item.id) ?? null
})

const groupMoveTargets = computed<AssetMenuAction[]>(() => {
  const rows: AssetMenuAction[] = [{ id: 'move-group:', label: t('shell.asset.moveToRoot'), icon: Folder }]
  const walk = (groups: GroupedHosts[], depth: number): void => {
    for (const group of groups) {
      const isPlaceholder = group.id === 'default'
      if (!isPlaceholder) {
        rows.push({
          id: `move-group:${group.id}`,
          label: `${'· '.repeat(depth)}${group.name}`,
          icon: FolderClosed,
        })
      }
      walk(group.children ?? [], isPlaceholder ? depth : depth + 1)
    }
  }
  walk(props.groupedHosts, 0)
  return rows
})

function goHome(): void {
  currentGroupId.value = null
}

function createHostHere(): void {
  if (currentGroup.value) emit('createHostInGroup', currentGroup.value.id)
  else emit('createHost')
}

function goUp(): void {
  currentGroupId.value = groupPath.value.at(-2)?.id ?? null
}

function openBreadcrumb(id: string | null): void {
  if (id === null) {
    goHome()
    return
  }

  currentGroupId.value = id
}

function groupActions(group: GroupedHosts): AssetMenuAction[] {
  const forbidden = new Set(findGroupPath(props.groupedHosts, group.id).map((node) => node.id))
  const collect = (node: GroupedHosts): void => {
    forbidden.add(node.id)
    for (const child of node.children ?? []) collect(child)
  }
  collect(group)
  return [
    { id: 'add-host', label: t('shell.asset.addHostInGroup'), icon: Plus },
    { id: 'create-subgroup', label: t('shell.asset.createSubgroup'), icon: FolderPlus },
    {
      id: 'move-group',
      label: t('shell.asset.moveToGroup'),
      icon: FolderClosed,
      children: groupMoveTargets.value.filter((row) => !forbidden.has(row.id.slice('move-group:'.length))),
    },
    {
      id: 'edit',
      label: t('shell.asset.editGroup'),
      icon: Pencil,
      separatorBefore: true,
      disabled: group.id === 'default',
    },
    {
      id: 'delete',
      label: t('shell.asset.deleteGroup'),
      icon: Trash2,
      destructive: true,
      disabled: group.id === 'default',
    },
  ]
}

function handleGroupAction(group: GroupedHosts, id: string): void {
  if (id === 'add-host') emit('createHostInGroup', group.id)
  else if (id === 'create-subgroup') emit('createSubgroup', group.id)
  else if (id === 'edit') emit('editGroup', group)
  else if (id === 'delete') emit('deleteGroup', group)
  else if (id.startsWith('move-group:')) emit('moveGroup', group.id, id.slice('move-group:'.length), '', 'inside')
}

function sshCapable(host: HostRecord): boolean {
  const protocol = (host.protocol ?? '').trim().toLowerCase()
  return protocol === '' || protocol === 'ssh'
}

function hostActionsFor(host: HostRecord): AssetMenuAction[] {
  const ssh = sshCapable(host)
  return [
    { id: 'open', label: t('shell.asset.connect'), icon: Link },
    { id: 'tunnel', label: t('shell.asset.tunnel'), icon: ArrowRightLeft, disabled: !ssh },
    { id: 'sftp', label: t('shell.asset.sftp'), icon: Folder, disabled: !ssh },
    {
      id: 'move-group',
      label: t('shell.asset.moveToGroup'),
      icon: FolderClosed,
      children: groupMoveTargets.value,
    },
    { id: 'copy-ip', label: t('shell.asset.copyIp'), icon: Copy, separatorBefore: true },
    { id: 'copy-id', label: t('shell.asset.copyId'), icon: Copy },
    { id: 'edit', label: t('shell.asset.edit'), icon: Pencil, separatorBefore: true },
    { id: 'copy', label: t('shell.asset.copy'), icon: Copy },
    { id: 'delete', label: t('shell.asset.delete'), icon: Trash2, destructive: true },
  ]
}

function handleHostAction(host: HostRecord, id: string): void {
  if (id === 'tunnel') emit('toolPanel', host, 'tunnels')
  else if (id === 'sftp') emit('toolPanel', host, 'sftp')
  else if (id === 'open') emit('openHost', host)
  else if (id === 'edit') emit('editHost', host)
  else if (id === 'delete') emit('deleteHost', host)
  else if (id === 'copy-ip') emit('copyHost', host, 'ip')
  else if (id === 'copy-id') emit('copyHost', host, 'id')
  else if (id === 'copy') emit('duplicateHost', host)
  else if (id.startsWith('move-group:')) emit('moveAsset', host.id, id.slice('move-group:'.length), '', 'inside')
}
</script>

<template>
  <section ref="rootRef" class="flex h-full min-h-0 min-w-0 flex-col">
    <div class="flex flex-none items-center gap-2 px-3 py-2">
      <IconButton :label="t('shell.assetLibrary.back')" :disabled="groupPath.length === 0" @click="goUp()">
        <ArrowLeft :size="16" aria-hidden="true" />
      </IconButton>
      <nav
        class="flex min-w-0 flex-1 items-center gap-1 overflow-hidden text-sm"
        :aria-label="t('shell.assetLibrary.breadcrumbAria')"
      >
        <template v-for="(item, index) in breadcrumbs" :key="item.id ?? 'root'">
          <ChevronRight v-if="index > 0" :size="14" class="flex-none text-qt-muted" aria-hidden="true" />
          <span
            v-if="index === breadcrumbs.length - 1"
            class="min-w-0 truncate px-1.5 py-0.5 text-qt-muted"
            aria-current="page"
            >{{ item.name }}</span
          >
          <button
            v-else
            type="button"
            class="max-w-48 flex-none truncate rounded px-1.5 py-0.5 hover:bg-qt-hover"
            @click="openBreadcrumb(item.id)"
          >
            {{ item.name }}
          </button>
        </template>
      </nav>

      <label ref="searchFieldRef" v-if="props.searchVisible" class="relative flex w-56 flex-none items-center">
        <Search
          :size="14"
          class="pointer-events-none absolute top-1/2 left-2.5 -translate-y-1/2 text-qt-muted"
          aria-hidden="true"
        />
        <span class="sr-only">{{ t('shell.asset.searchLabel') }}</span>
        <QtermInput
          :model-value="props.searchQuery"
          class="pl-8"
          type="search"
          :placeholder="t('shell.asset.searchPlaceholder')"
          autofocus
          @update:model-value="(value) => emit('updateSearchQuery', String(value))"
        />
      </label>

      <div
        class="asset-library-tools flex flex-none items-center gap-1"
        role="toolbar"
        :aria-label="t('shell.assetLibrary.toolbar')"
      >
        <IconButton :label="t('shell.asset.addHost')" @click="createHostHere">
          <Plus :size="16" aria-hidden="true" />
        </IconButton>
        <IconButton :label="t('shell.asset.createGroup')" @click="emit('createGroup')">
          <FolderPlus :size="16" aria-hidden="true" />
        </IconButton>
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
          <component :is="sortIcon" :size="16" aria-hidden="true" />
        </IconButton>
        <IconButton :label="t('shell.asset.refresh')" @click="emit('refresh')">
          <RefreshCw :size="16" aria-hidden="true" />
        </IconButton>
      </div>
    </div>

    <div class="asset-library-divider h-px flex-none bg-qt-border" aria-hidden="true" tabindex="-1"></div>

    <div
      v-if="props.loading"
      class="flex min-h-0 grow flex-col items-center justify-center gap-2 px-4 py-10 text-center text-qt-muted"
      aria-live="polite"
    >
      <RefreshCw class="spin" :size="22" aria-hidden="true" />
      <p class="m-0 text-sm">{{ t('shell.asset.loading') }}</p>
    </div>

    <div
      v-else-if="props.loadError"
      class="flex min-h-0 grow flex-col items-center justify-center gap-2 px-4 py-10 text-center text-qt-muted"
      role="alert"
    >
      <CircleHelp :size="22" aria-hidden="true" />
      <p class="m-0 text-sm">{{ props.loadError }}</p>
      <QtermButton variant="link" size="sm" type="button" @click="emit('refresh')">{{
        t('shell.asset.retry')
      }}</QtermButton>
    </div>

    <div
      v-else-if="props.hosts.length === 0 && !visibleGroups.length"
      class="flex min-h-0 grow flex-col items-center justify-center gap-2 px-4 py-10 text-center text-qt-muted"
    >
      <Server :size="22" aria-hidden="true" />
      <p class="m-0 text-sm">{{ t('shell.asset.empty') }}</p>
      <QtermButton variant="link" size="sm" type="button" @click="emit('createHost')">{{
        t('shell.asset.addFirst')
      }}</QtermButton>
    </div>

    <div
      v-else-if="props.hosts.length > 0 && !visibleHosts.length && searchActive"
      class="flex min-h-0 grow flex-col items-center justify-center gap-2 px-4 py-10 text-center text-qt-muted"
    >
      <Search :size="22" aria-hidden="true" />
      <p class="m-0 text-sm">{{ t('shell.asset.noMatch') }}</p>
    </div>

    <div
      v-else-if="!visibleGroups.length && !visibleHosts.length"
      class="flex min-h-0 grow flex-col items-center justify-center gap-2 px-4 py-10 text-center text-qt-muted"
    >
      <Server :size="22" aria-hidden="true" />
      <p class="m-0 text-sm">{{ t('shell.assetLibrary.emptyHosts') }}</p>
      <QtermButton v-if="currentGroup" variant="link" size="sm" type="button" @click="createHostHere()">{{
        t('shell.asset.addHost')
      }}</QtermButton>
    </div>

    <ScrollArea v-else type="hover" :scroll-hide-delay="600" class="min-h-0 grow">
      <div
        ref="gridRef"
        class="relative flex flex-col gap-5 p-4"
        :class="{ 'asset-drag-active': assetDrag.state.active }"
        @pointerdown.capture="assetDrag.start"
      >
        <section v-if="visibleGroups.length" class="min-w-0">
          <h2 class="m-0 text-sm">{{ t('shell.assetLibrary.groups') }}</h2>
          <div class="mt-2 grid grid-cols-[repeat(auto-fill,minmax(16rem,1fr))] gap-2">
            <AssetContextMenu
              v-for="group in visibleGroups"
              :key="group.id"
              :actions="groupActions(group)"
              @action="(id) => handleGroupAction(group, id)"
            >
              <button
                type="button"
                data-asset-card
                :data-asset-kind="'group'"
                :data-asset-id="group.id"
                :data-asset-label="group.name"
                class="flex w-full min-w-0 items-center gap-3 rounded-lg border-d bg-[var(--qt-asset-library-card-bg,var(--qt-panel))] p-3 text-left hover:bg-[var(--qt-asset-library-card-hover,var(--qt-hover))] focus-visible:ring-2 focus-visible:ring-qt-focus"
                :class="{
                  'is-selected bg-[var(--qt-asset-library-card-hover,var(--qt-hover))]':
                    props.selectedGroupId === group.id,
                }"
                :aria-label="t('shell.assetLibrary.groupAria', { name: group.name })"
                @click="selectGroupCard(group)"
                @dblclick="openGroup(group)"
              >
                <span class="grid size-10 flex-none place-items-center rounded-lg border-d bg-qt-subtle text-qt-info">
                  <FolderClosed :size="20" aria-hidden="true" />
                </span>
                <span class="min-w-0">
                  <span class="block truncate text-sm">{{ group.name }}</span>
                  <span class="block truncate text-xs text-qt-muted">{{
                    t('shell.assetLibrary.hostCount', { count: hostCount(group) })
                  }}</span>
                </span>
              </button>
            </AssetContextMenu>
          </div>
        </section>

        <section class="min-w-0">
          <h2 class="m-0 text-sm">{{ t('shell.assetLibrary.hosts') }}</h2>
          <div v-if="visibleHosts.length" class="mt-2 grid grid-cols-[repeat(auto-fill,minmax(16rem,1fr))] gap-2">
            <AssetContextMenu
              v-for="host in visibleHosts"
              :key="host.id"
              :actions="hostActionsFor(host)"
              @action="(id) => handleHostAction(host, id)"
            >
              <button
                type="button"
                data-asset-card
                :data-asset-kind="'host'"
                :data-asset-id="host.id"
                :data-asset-label="host.name"
                class="host-color-card relative flex w-full min-w-0 items-center gap-3 rounded-lg border-d bg-[var(--qt-asset-library-card-bg,var(--qt-panel))] p-3 text-left hover:bg-[var(--qt-asset-library-card-hover,var(--qt-hover))] focus-visible:ring-2 focus-visible:ring-qt-focus"
                :class="{
                  'bg-[var(--qt-asset-library-card-hover,var(--qt-hover))]': props.selectedHostId === host.id,
                  'has-host-color': !!hostColorStyle(host),
                  'is-selected': props.selectedHostId === host.id,
                }"
                :style="hostColorStyle(host)"
                :aria-label="hostAriaLabel(host)"
                :title="hostTitle(host)"
                @click="emit('selectHost', host.id)"
                @dblclick="emit('openHost', host)"
              >
                <span
                  class="host-color-tile grid size-10 flex-none place-items-center rounded-lg border-d bg-qt-subtle"
                >
                  <HostIcon :icon="host.icon" :protocol="host.protocol" :size="22" />
                </span>
                <span class="min-w-0">
                  <span class="block truncate text-sm">{{ host.name }}</span>
                  <span v-if="props.showHostAddress !== false" class="block truncate text-xs text-qt-muted">{{
                    hostSubtitle(host)
                  }}</span>

                  <span v-if="hostTags(host).length > 0" class="mt-1 flex flex-wrap items-center gap-1">
                    <span
                      v-for="tag in visibleHostTags(host)"
                      :key="tag"
                      class="max-w-24 truncate rounded bg-qt-control-muted-bg px-1.5 py-0.5 text-[11px] leading-4 text-qt-control-muted-text"
                      >{{ tag }}</span
                    >
                    <span v-if="hiddenHostTagCount(host) > 0" class="text-[11px] leading-4 text-qt-muted"
                      >+{{ hiddenHostTagCount(host) }}</span
                    >
                  </span>
                </span>
              </button>
            </AssetContextMenu>
          </div>

          <div
            v-else-if="!currentGroup && !props.hosts.length"
            class="flex flex-col items-center gap-2 py-8 text-center text-qt-muted"
          >
            <Server :size="22" aria-hidden="true" />
            <p class="m-0 text-sm">{{ t('shell.asset.empty') }}</p>
            <QtermButton variant="link" size="sm" type="button" @click="emit('createHost')">{{
              t('shell.asset.addFirst')
            }}</QtermButton>
          </div>
          <p v-else-if="currentGroup" class="mt-2 text-xs text-qt-muted">
            {{ t('shell.assetLibrary.emptyHosts') }}
          </p>
        </section>
      </div>
    </ScrollArea>

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
  </section>
</template>
