<script setup lang="ts">
import DeleteConfirmDialog from './DeleteConfirmDialog.vue'
import type { PendingDelete } from '../../types/qterm'
import { useI18n } from '@/locales'

const props = defineProps<{ target: PendingDelete | null; deleting: boolean; error: string }>()
defineEmits<{ close: []; confirm: [] }>()
const { t } = useI18n()
</script>

<template>
  <DeleteConfirmDialog
    :open="!!props.target"
    :title="
      t('shell.delete.confirm') + (props.target?.kind === 'host' ? t('shell.asset.host') : t('shell.asset.group'))
    "
    :message="t('shell.asset.deleteMessage', { name: props.target?.name ?? '' })"
    :hint="props.target?.kind === 'group' ? t('shell.asset.emptyGroupHint') : ''"
    :error="props.error"
    :loading="props.deleting"
    :confirm-text="t('shell.delete.confirm')"
    @close="$emit('close')"
    @confirm="$emit('confirm')"
  />
</template>
