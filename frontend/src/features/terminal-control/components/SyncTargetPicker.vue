<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Users } from '@lucide/vue'
import { QtermButton } from '../../../components/common'
import {
  QtermDialog,
  QtermDialogContent,
  QtermDialogFooter,
  QtermDialogHeader,
} from '../../../components/common/dialog'
import { useI18n } from '../../../locales'
import type { SyncTargetPickerMode, TerminalControlSession } from '../types'
import SessionPicker from './SessionPicker.vue'

const props = withDefaults(
  defineProps<{
    open: boolean
    sessions: TerminalControlSession[]
    sourceSessionId?: string
    sourceLabel?: string
    selectedIds?: string[]
    mode?: SyncTargetPickerMode
    busy?: boolean
    error?: string
    resumeOnApply?: boolean
  }>(),
  {
    sourceSessionId: '',
    sourceLabel: '',
    selectedIds: () => [],
    mode: 'create',
    busy: false,
    error: '',
    resumeOnApply: false,
  },
)

const emit = defineEmits<{
  'update:open': [value: boolean]
  submit: [sessionIds: string[]]
  cancel: []
}>()

const { t } = useI18n()
const draftIds = ref<Set<string>>(new Set())

const lockedIds = computed(
  () => new Set(props.mode === 'create' && props.sourceSessionId ? [props.sourceSessionId] : []),
)

const selectableCount = computed(() => {
  const ids = new Set(draftIds.value)
  if (props.mode === 'create' && props.sourceSessionId) ids.add(props.sourceSessionId)
  return ids.size
})

function resetDraft(): void {
  draftIds.value = new Set(props.selectedIds)
  if (props.mode === 'create' && props.sourceSessionId) draftIds.value.add(props.sourceSessionId)
}

function close(): void {
  emit('update:open', false)
  emit('cancel')
}

function updateSelected(ids: string[]): void {
  draftIds.value = new Set(ids)
}

function submit(): void {
  const ids = [...draftIds.value]
  if (props.mode === 'create' && props.sourceSessionId && !ids.includes(props.sourceSessionId))
    ids.unshift(props.sourceSessionId)
  emit('submit', ids)
}

watch(
  () => props.open,
  (open) => {
    if (open) resetDraft()
  },
)
watch(
  () => props.selectedIds,
  () => {
    if (!props.open) resetDraft()
  },
  { deep: true },
)
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(value) => emit('update:open', value)">
    <QtermDialogContent size="lg" class="max-h-[min(680px,calc(100vh-2rem))] overflow-hidden">
      <QtermDialogHeader
        :title="props.mode === 'create' ? t('terminalControl.sync.createTitle') : t('terminalControl.sync.manageTitle')"
        :description="t('terminalControl.sync.description')"
        @close="close"
      >
        <template #icon><Users :size="16" /></template>
      </QtermDialogHeader>

      <div class="flex min-h-0 flex-col gap-3 p-4">
        <p v-if="props.mode === 'manage'" class="m-0 text-xs text-qt-muted">
          {{ t('terminalControl.sync.managePaused') }}
        </p>
        <SessionPicker
          :sessions="props.sessions"
          :selected-ids="[...draftIds]"
          :source-session-id="props.sourceSessionId"
          :locked-ids="[...lockedIds]"
          :disabled="props.busy"
          select-all
          max-height-class="max-h-[min(360px,45vh)]"
          @update:selected-ids="updateSelected"
        />

        <p v-if="props.mode === 'create' && selectableCount < 2" class="m-0 text-xs text-qt-muted">
          {{ t('terminalControl.sync.noOther') }}
        </p>
        <p v-if="props.mode === 'manage' && selectableCount < 2" class="m-0 text-xs text-qt-muted">
          {{ t('terminalControl.sync.tooFewEnds') }}
        </p>
        <p v-if="props.error" class="m-0 text-xs text-qt-danger" role="alert">{{ props.error }}</p>
      </div>

      <QtermDialogFooter>
        <QtermButton variant="ghost" @click="close">{{ t('terminalControl.sync.cancel') }}</QtermButton>
        <QtermButton :disabled="props.busy || (props.mode === 'create' && selectableCount < 2)" @click="submit">
          {{
            t(
              props.mode === 'create'
                ? 'terminalControl.sync.createSubmit'
                : props.resumeOnApply
                  ? 'terminalControl.sync.applyAndResume'
                  : 'terminalControl.sync.apply',
            )
          }}
        </QtermButton>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>
