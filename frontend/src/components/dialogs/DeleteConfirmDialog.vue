<script setup lang="ts">
import { QtermButton, QtermFormMessage } from '../common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../common/dialog'
import { useI18n } from '@/locales'

const props = withDefaults(
  defineProps<{
    open: boolean
    title: string
    message: string
    hint?: string
    error?: string
    items?: string[]
    loading?: boolean
    confirmText?: string
    confirmingText?: string
    showCloseButton?: boolean
    backgroundEnabled?: boolean
  }>(),
  {
    hint: '',
    error: '',
    items: () => [],
    loading: false,
    confirmText: '',
    confirmingText: '',
    showCloseButton: false,
    backgroundEnabled: false,
  },
)

const emit = defineEmits<{ close: []; confirm: [] }>()
const { t } = useI18n()
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => !open && !props.loading && emit('close')">
    <QtermDialogContent
      v-if="props.open"
      class="delete-confirm-dialog px-4 py-0"
      :show-close-button="props.showCloseButton && !props.loading"
      :close-on-escape="!props.loading"
      @pointer-down-outside="(event) => event.preventDefault()"
    >
      <QtermDialogHeader :title="props.title" :show-close="false" />
      <div class="dialog-body">
        <p class="dialog-message">{{ props.message }}</p>

        <ul v-if="props.items.length" class="delete-confirm-items m-0 p-0 list-none grid gap-[4px] wrap-anywhere">
          <li v-for="item in props.items" :key="item">{{ item }}</li>
        </ul>

        <slot />
        <QtermFormMessage v-if="props.hint">{{ props.hint }}</QtermFormMessage>
        <QtermFormMessage v-if="props.error" variant="error">{{ props.error }}</QtermFormMessage>
      </div>
      <QtermDialogFooter class="dialog-delete-footer delete-confirm-actions">
        <QtermButton variant="outline" :disabled="props.loading" @click="emit('close')">{{
          t('shell.common.cancel')
        }}</QtermButton>
        <QtermButton
          variant="destructive"
          class="danger-button"
          :disabled="props.loading"
          :data-deleting="props.loading ? 'true' : 'false'"
          @click="emit('confirm')"
        >
          {{
            props.loading
              ? props.confirmingText || t('shell.delete.confirming')
              : props.confirmText || t('shell.delete.confirm')
          }}
        </QtermButton>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>
