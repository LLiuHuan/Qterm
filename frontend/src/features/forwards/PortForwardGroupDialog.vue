<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { QtermButton, QtermInput } from '@/components/common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '@/components/common/dialog'
import { useI18n } from '@/locales'

const props = withDefaults(
  defineProps<{
    open: boolean

    mode?: 'create' | 'rename'

    initial?: string

    existing?: string[]
  }>(),
  { mode: 'create', initial: '', existing: () => [] },
)

const emit = defineEmits<{
  'update:open': [open: boolean]
  submit: [name: string]
}>()

const { t } = useI18n()
const name = ref('')
const error = ref('')

watch(
  () => props.open,
  (open) => {
    if (!open) return
    name.value = props.mode === 'rename' ? props.initial : ''
    error.value = ''
  },
  { immediate: true },
)

const title = computed(() =>
  props.mode === 'rename' ? t('settings.portForwards.group.renameTitle') : t('settings.portForwards.group.createTitle'),
)

function submit(): void {
  const value = name.value.trim()
  if (!value) {
    error.value = t('settings.portForwards.group.nameRequired')
    return
  }

  const taken = props.existing.some((item) => item === value && item !== props.initial)
  if (taken) {
    error.value = t('settings.portForwards.group.exists')
    return
  }
  emit('submit', value)
  emit('update:open', false)
}
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => emit('update:open', open)">
    <QtermDialogContent v-if="props.open" data-forward-group-dialog>
      <QtermDialogHeader :title="title" :show-close="false" @close="emit('update:open', false)" />
      <div class="dialog-body">
        <label class="flex flex-col gap-1">
          <span class="text-xs text-qt-muted">{{ t('settings.portForwards.group.name') }}</span>
          <QtermInput
            v-model="name"
            :aria-label="t('settings.portForwards.group.name')"
            :placeholder="t('settings.portForwards.group.namePlaceholder')"
            spellcheck="false"
            @keydown.enter="submit"
          />
        </label>
        <p v-if="error" class="m-0 text-xs text-qt-danger" role="alert">{{ error }}</p>
      </div>
      <QtermDialogFooter>
        <div></div>
        <div class="dialog-footer-actions flex items-center gap-[8px]">
          <QtermButton variant="outline" @click="emit('update:open', false)">{{
            t('shell.common.cancel')
          }}</QtermButton>
          <QtermButton data-forward-group-confirm @click="submit">{{ t('shell.common.confirm') }}</QtermButton>
        </div>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>
