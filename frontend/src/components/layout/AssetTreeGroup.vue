<script setup lang="ts">
import { QtermButton } from '../common'
import { ChevronDown, FolderPlus, Pencil, Plus, Trash2 } from '@lucide/vue'
import { computed, ref } from 'vue'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '../common/control-primitives'
import AssetContextMenu from '../common/overlay-controls/AssetContextMenu.vue'
import AssetHostRow from './AssetHostRow.vue'
import type { GroupedHosts } from '../../types/qterm'
import type { HostRecord } from '@/services/hosts.ts'
import { useI18n } from '@/locales'

const props = defineProps<{
  group: GroupedHosts
  selectedHostId: string | null
  selectedGroupId?: string | null
  showAddress?: boolean
}>()
const emit = defineEmits<{
  selectHost: [string]
  openHost: [HostRecord]
  editHost: [HostRecord]
  deleteHost: [HostRecord]
  selectGroup: [string]
  createHostInGroup: [string]
  createSubgroup: [string]
  editGroup: [GroupedHosts]
  deleteGroup: [GroupedHosts]
  copyHost: [HostRecord, 'ip' | 'id']
  duplicateHost: [HostRecord]

  toolPanel: [HostRecord, 'tunnels' | 'sftp']
  moveAsset: [string, string, string, 'before' | 'after' | 'inside']
  moveGroup: [string, string, string, 'before' | 'after' | 'inside']
}>()
const { t } = useI18n()

const open = ref(true)
const actions = computed(() => [
  { id: 'add-host', label: t('shell.asset.addHostInGroup'), icon: Plus },
  { id: 'create-subgroup', label: t('shell.asset.createSubgroup'), icon: FolderPlus },
  {
    id: 'edit',
    label: t('shell.asset.editGroup'),
    icon: Pencil,
    separatorBefore: true,
    disabled: props.group.id === 'default',
  },
  {
    id: 'delete',
    label: t('shell.asset.deleteGroup'),
    icon: Trash2,
    destructive: true,
    disabled: props.group.id === 'default',
  },
])
function forwardCopy(host: HostRecord, kind: 'ip' | 'id'): void {
  emit('copyHost', host, kind)
}

function forwardDuplicate(host: HostRecord): void {
  emit('duplicateHost', host)
}

function hostCount(group: GroupedHosts): number {
  return group.items.length + (group.children?.reduce((total, child) => total + hostCount(child), 0) ?? 0)
}
</script>

<template>
  <Collapsible v-model:open="open" class="asset-group" data-asset-kind="group" :data-asset-id="props.group.id">
    <div class="relative">
      <AssetContextMenu
        :actions="actions"
        @action="
          (id) =>
            id === 'add-host'
              ? emit('createHostInGroup', props.group.id)
              : id === 'create-subgroup'
                ? emit('createSubgroup', props.group.id)
                : id === 'edit'
                  ? emit('editGroup', props.group)
                  : emit('deleteGroup', props.group)
        "
      >
        <CollapsibleTrigger as-child
          ><QtermButton
            variant="ghost"
            class="asset-group-title"
            @asset-drag-expand="open = true"
            :class="{ 'is-selected': props.selectedGroupId === props.group.id }"
            :aria-label="t('shell.asset.groupAria', { name: props.group.name })"
            @click="emit('selectGroup', props.group.id)"
            ><ChevronDown :size="14" class="group-chevron flex-none size-[18px]" :class="{ 'is-closed': !open }" /><span
              class="min-w-0 flex-1 truncate text-sm"
              >{{ props.group.name }}</span
            ><span class="text-qt-muted font-normal">{{ hostCount(props.group) }}</span></QtermButton
          ></CollapsibleTrigger
        >
      </AssetContextMenu>
    </div>
    <CollapsibleContent v-if="props.group.entries?.length && open" class="asset-group-children relative pl-[18px]">
      <div class="asset-host-list" :data-asset-parent="props.group.id">
        <template v-for="entry in props.group.entries" :key="`${entry.kind}-${entry.value.id}`">
          <AssetHostRow
            v-if="entry.kind === 'host'"
            :host="entry.value"
            :selected="props.selectedHostId === entry.value.id"
            :show-address="props.showAddress"
            @select="emit('selectHost', $event)"
            @open="emit('openHost', $event)"
            @edit="emit('editHost', $event)"
            @delete="emit('deleteHost', $event)"
            @copy="forwardCopy"
            @duplicate="forwardDuplicate"
            @tool-panel="(host, tool) => emit('toolPanel', host, tool)"
          />
          <AssetTreeGroup
            v-else
            :group="entry.value"
            :selected-host-id="props.selectedHostId"
            :selected-group-id="props.selectedGroupId"
            :show-address="props.showAddress"
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
        </template>
      </div>
    </CollapsibleContent>
  </Collapsible>
</template>
