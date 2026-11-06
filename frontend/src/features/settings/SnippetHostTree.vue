<script setup lang="ts">
import { QtermCheckbox } from '../../components/common'
import { QtermButton } from '../../components/common'
import { computed } from 'vue'
import { ChevronDown } from '@lucide/vue'
import { Label } from '../../components/common/control-primitives'
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from '../../components/common/control-primitives'
import type { GroupRecord, HostRecord } from '@/services/hosts.ts'
import { useI18n } from '@/locales'

const props = withDefaults(
  defineProps<{
    groups: GroupRecord[]
    hosts: HostRecord[]
    selected: string[]
    parentId?: string
  }>(),
  { parentId: '' },
)
const emit = defineEmits<{ select: [ids: string[], checked: boolean] }>()
const { t } = useI18n()
const children = computed(() =>
  props.groups.filter((g) => (g.parentId || '') === props.parentId && descendants(g.id).length > 0),
)
const directHosts = computed(() => props.hosts.filter((h) => (h.groupId || '') === props.parentId))
function descendants(id: string): string[] {
  const groupIDs = new Set([id])

  let changed = true
  while (changed) {
    changed = false
    for (const group of props.groups) {
      if (group.parentId && groupIDs.has(group.parentId) && !groupIDs.has(group.id)) {
        groupIDs.add(group.id)
        changed = true
      }
    }
  }
  return props.hosts.filter((h) => groupIDs.has(h.groupId)).map((h) => h.id)
}
function state(id: string): boolean | 'indeterminate' {
  const ids = descendants(id)
  const count = ids.filter((value) => props.selected.includes(value)).length
  return count === 0 ? false : count === ids.length ? true : 'indeterminate'
}
</script>
<template>
  <div class="snippet-tree">
    <Collapsible v-for="group in children" :key="group.id" :default-open="true">
      <div
        class="snippet-tree-row text-sm flex items-center gap-[8px] min-h-[34px] pt-[4px] pr-[10px] pb-[4px] pl-[10px]"
        :data-group-id="group.id"
      >
        <QtermCheckbox
          :id="`snippet-asset-${group.id}`"
          :model-value="state(group.id)"
          :data-indeterminate="state(group.id) === 'indeterminate' ? 'true' : undefined"
          :disabled="!descendants(group.id).length"
          @update:model-value="(value) => emit('select', descendants(group.id), value === true)"
        />
        <Label :for="`snippet-asset-${group.id}`" class="text-sm"
          >{{ group.name
          }}<small class="text-xs" v-if="descendants(group.id).length"
            >（{{ descendants(group.id).length }}）</small
          ></Label
        >
        <CollapsibleTrigger as-child>
          <QtermButton
            type="button"
            variant="ghost"
            size="icon"
            :aria-label="t('settings.snippetTree.toggle', { name: group.name })"
          >
            <ChevronDown :size="16" />
          </QtermButton>
        </CollapsibleTrigger>
      </div>
      <CollapsibleContent class="snippet-tree-children">
        <SnippetHostTree
          :groups="groups"
          :hosts="hosts"
          :selected="selected"
          :parent-id="group.id"
          @select="(ids, checked) => emit('select', ids, checked)"
        />
      </CollapsibleContent>
    </Collapsible>
    <div
      v-for="host in directHosts"
      :key="host.id"
      class="snippet-tree-row snippet-tree-host text-sm flex items-center gap-[8px] min-h-[34px] pt-[4px] pr-[10px] pb-[4px] pl-[10px]"
    >
      <QtermCheckbox
        :id="`snippet-host-${host.id}`"
        :model-value="selected.includes(host.id)"
        @update:model-value="(value) => emit('select', [host.id], value === true)"
      />
      <Label :for="`snippet-host-${host.id}`" class="snippet-host-copy flex flex-col items-start gap-[2px] min-w-0"
        ><span>{{ host.name }}</span
        ><small class="text-xs">{{ host.address }}</small></Label
      >
    </div>
  </div>
</template>
