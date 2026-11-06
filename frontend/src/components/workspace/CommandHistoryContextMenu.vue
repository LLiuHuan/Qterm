<script setup lang="ts">
import { Plus, Trash2 } from '@lucide/vue'
import { ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuTrigger } from '../common/overlay-controls'
import { useI18n } from '@/locales'

defineProps<{ canDelete: boolean }>()
const emit = defineEmits<{ addSnippet: []; remove: [] }>()
const { t } = useI18n()
</script>

<template>
  <ContextMenu>
    <ContextMenuTrigger as-child><slot /></ContextMenuTrigger>
    <ContextMenuContent class="asset-context-menu min-w-44">
      <ContextMenuItem @select="emit('addSnippet')"
        ><Plus :size="14" />{{ t('shell.commandMenu.addSnippet') }}</ContextMenuItem
      >
      <ContextMenuItem variant="destructive" :disabled="!canDelete" @select="emit('remove')">
        <Trash2 :size="14" />{{ t('shell.commandMenu.delete') }}
      </ContextMenuItem>
    </ContextMenuContent>
  </ContextMenu>
</template>
