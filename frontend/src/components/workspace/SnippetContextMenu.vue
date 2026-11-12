<script setup lang="ts">
import { Copy, Pencil, Trash2 } from '@lucide/vue'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from '../common/overlay-controls'
import { useI18n } from '@/locales'

defineProps<{ canEdit: boolean }>()
const emit = defineEmits<{ copy: []; edit: []; remove: [] }>()
const { t } = useI18n()
</script>

<template>
  <ContextMenu>
    <ContextMenuTrigger as-child><slot /></ContextMenuTrigger>
    <ContextMenuContent class="asset-context-menu min-w-36">
      <ContextMenuItem @select="emit('copy')"
        ><Copy :size="14" />{{ t('shell.toolPanel.snippetCopy') }}</ContextMenuItem
      >
      <ContextMenuItem :disabled="!canEdit" @select="emit('edit')"
        ><Pencil :size="14" />{{ t('shell.toolPanel.snippetEdit') }}</ContextMenuItem
      >
      <ContextMenuSeparator />
      <ContextMenuItem variant="destructive" @select="emit('remove')">
        <Trash2 :size="14" />{{ t('shell.toolPanel.snippetDelete') }}
      </ContextMenuItem>
    </ContextMenuContent>
  </ContextMenu>
</template>
