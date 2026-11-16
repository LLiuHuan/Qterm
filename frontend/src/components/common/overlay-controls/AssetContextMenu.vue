<script setup lang="ts">
import { Check } from '@lucide/vue'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuSub,
  ContextMenuSubContent,
  ContextMenuSubTrigger,
  ContextMenuTrigger,
} from './index'
import type { Component } from 'vue'

export interface AssetMenuAction {
  id: string
  label: string
  destructive?: boolean
  disabled?: boolean

  checked?: boolean
  icon?: Component
  separatorBefore?: boolean

  children?: AssetMenuAction[]
}
defineProps<{ actions: AssetMenuAction[] }>()
const emit = defineEmits<{ action: [id: string] }>()
</script>

<template>
  <ContextMenu>
    <ContextMenuTrigger as-child><slot /></ContextMenuTrigger>
    <ContextMenuContent class="asset-context-menu min-w-40">
      <template v-for="action in actions" :key="action.id">
        <ContextMenuSeparator v-if="action.separatorBefore" />
        <ContextMenuSub v-if="action.children?.length">
          <ContextMenuSubTrigger :disabled="action.disabled">
            <component :is="action.icon" v-if="action.icon" :size="14" aria-hidden="true" />
            {{ action.label }}
          </ContextMenuSubTrigger>
          <ContextMenuSubContent class="asset-context-menu min-w-40">
            <ContextMenuItem
              v-for="child in action.children"
              :key="child.id"
              :disabled="child.disabled"
              :data-checked="child.checked ? '' : undefined"
              :class="child.checked ? 'bg-accent data-disabled:opacity-100' : undefined"
              @select="emit('action', child.id)"
            >
              <component :is="child.icon" v-if="child.icon" :size="14" aria-hidden="true" />
              {{ child.label }}
              <Check v-if="child.checked" class="ml-auto size-3.5" aria-hidden="true" />
            </ContextMenuItem>
          </ContextMenuSubContent>
        </ContextMenuSub>
        <ContextMenuItem
          v-else
          :disabled="action.disabled"
          :variant="action.destructive ? 'destructive' : 'default'"
          @select="emit('action', action.id)"
        >
          <component :is="action.icon" v-if="action.icon" :size="14" aria-hidden="true" />
          {{ action.label }}
        </ContextMenuItem>
      </template>
    </ContextMenuContent>
  </ContextMenu>
</template>
