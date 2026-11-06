<script setup lang="ts">
import { ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuSeparator, ContextMenuTrigger } from './index'
import type { Component } from 'vue'

export interface AssetMenuAction {
  id: string
  label: string
  destructive?: boolean
  disabled?: boolean
  icon?: Component
  separatorBefore?: boolean
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
        <ContextMenuItem
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
