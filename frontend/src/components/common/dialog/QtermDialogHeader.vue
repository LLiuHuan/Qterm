<script setup lang="ts">
import { X } from '@lucide/vue'
import QtermButton from '../button/QtermButton.vue'
import { DialogDescription, DialogTitle } from '../../ui/dialog'
import { Separator } from '../../ui/separator'
import { useI18n } from '../../../locales'

withDefaults(
  defineProps<{
    title: string
    description?: string
    badge?: string
    closeDisabled?: boolean
    showClose?: boolean
  }>(),
  { description: '', badge: '', closeDisabled: false, showClose: true },
)

const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
</script>

<template>
  <header
    class="dialog-header relative"
    :class="{ 'dialog-header--with-icon': !!$slots.icon }"
    data-slot="dialog-header"
  >
    <Separator class="dialog-header-separator" />
    <span v-if="$slots.icon" class="dialog-header-icon" data-slot="dialog-header-icon" aria-hidden="true">
      <slot name="icon" />
    </span>
    <div class="dialog-header-content">
      <DialogTitle>{{ title }}</DialogTitle>
      <DialogDescription v-if="description">{{ description }}</DialogDescription>
    </div>
    <span v-if="badge" class="shrink-0 rounded-md bg-qt-hover px-2 py-1 text-xs text-qt-muted">{{ badge }}</span>
    <QtermButton
      v-if="showClose"
      class="hover:border-qt-border"
      variant="ghost"
      size="icon-sm"
      type="button"
      :aria-label="t('shell.app.close')"
      :disabled="closeDisabled"
      @click="emit('close')"
    >
      <X :size="16" aria-hidden="true" />
    </QtermButton>
  </header>
</template>

<style scoped>
.dialog-header--with-icon {
  gap: 10px;
}

.dialog-header-icon {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
}

.dialog-header-content {
  min-width: 0;
}
</style>
