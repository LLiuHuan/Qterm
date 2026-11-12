<script setup lang="ts">
import { computed, ref } from 'vue'
import { Ban, CircleAlert, PencilLine } from '@lucide/vue'
import { QtermButton } from '../../components/common'
import { useI18n } from '../../locales'
import type { AgentStep } from '../../services/aiAgent'
import AiDiffPreview from './AiDiffPreview.vue'

const props = defineProps<{
  step: AgentStep
  disabled?: boolean

  rememberable?: boolean
}>()

const emit = defineEmits<{ confirm: [remember: boolean]; cancel: [] }>()
const { t } = useI18n()
const remember = ref(false)

interface WritePreview {
  path?: string
  oldText?: string
  newText?: string
  created?: boolean
}

interface TransferPreview {
  direction?: 'upload' | 'download'
  localPath?: string
  remotePath?: string
  size?: number
}

const preview = computed<WritePreview | null>(() => {
  const data = props.step.preview?.data
  if (!data || typeof data !== 'object') return null
  return data as WritePreview
})

const transfer = computed<TransferPreview | null>(() => {
  if (props.step.preview?.kind !== 'sftp_transfer') return null
  const data = props.step.preview?.data
  if (!data || typeof data !== 'object') return null
  return data as TransferPreview
})

const transferSize = computed(() => {
  const bytes = transfer.value?.size ?? 0
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GiB`
})

const title = computed(() => {
  if (props.step.kind === 'sftp_write_file') return t('shell.aiChat.confirmation.writeTitle')
  if (transfer.value) {
    return transfer.value.direction === 'download'
      ? t('shell.aiChat.confirmation.downloadTitle')
      : t('shell.aiChat.confirmation.uploadTitle')
  }
  return t('shell.aiChat.confirmation.title')
})
</script>

<template>
  <div
    class="flex min-w-0 flex-col gap-2 rounded-lg border border-qt-warning/40 bg-qt-warning/5 p-3"
    data-ai-tool-confirmation
  >
    <p class="m-0 flex items-center gap-1.5 text-xs font-medium text-qt-warning">
      <CircleAlert class="size-3.5 shrink-0" aria-hidden="true" />
      {{ title }}
    </p>
    <template v-if="transfer">
      <p class="m-0 flex min-w-0 items-center gap-1.5 text-xs text-qt-muted">
        <PencilLine class="size-3.5 shrink-0" aria-hidden="true" />
        <span class="shrink-0">{{ t('shell.aiChat.confirmation.transferSize') }}</span>
        <span class="font-mono tabular-nums text-qt-text">{{ transferSize }}</span>
      </p>
      <p class="m-0 flex min-w-0 items-center gap-1.5 text-xs text-qt-muted" data-ai-transfer-local>
        <span class="shrink-0">{{ t('shell.aiChat.confirmation.localPath') }}</span>
        <span class="truncate font-mono text-qt-text">{{ transfer.localPath }}</span>
      </p>
      <p class="m-0 flex min-w-0 items-center gap-1.5 text-xs text-qt-muted" data-ai-transfer-remote>
        <span class="shrink-0">{{ t('shell.aiChat.confirmation.remotePath') }}</span>
        <span class="truncate font-mono text-qt-text">{{ transfer.remotePath }}</span>
      </p>
    </template>
    <p v-else-if="preview?.path" class="m-0 flex min-w-0 items-center gap-1.5 text-xs text-qt-muted">
      <PencilLine class="size-3.5 shrink-0" aria-hidden="true" />
      <span class="truncate font-mono text-qt-text">{{ preview.path }}</span>
    </p>
    <AiDiffPreview
      v-if="preview && !transfer"
      :old-text="preview.oldText ?? ''"
      :new-text="preview.newText ?? ''"
      :created="preview.created"
    />
    <p v-else-if="!transfer" class="m-0 text-xs text-qt-muted/70">
      {{ t('shell.aiChat.confirmation.noPreview') }}
    </p>
    <p class="m-0 text-xs text-qt-muted">
      {{ t('shell.aiChat.confirmation.risk') }}：<span class="text-qt-text">{{
        t('shell.aiChat.commandCard.riskConfirm')
      }}</span>
    </p>
    <label v-if="props.rememberable" class="flex w-fit cursor-pointer items-center gap-1.5 text-xs text-qt-muted">
      <input v-model="remember" type="checkbox" class="size-3.5 accent-[var(--qt-accent)]" data-ai-tool-remember />
      <span>{{ t('shell.aiChat.confirmation.remember') }}</span>
    </label>
    <div class="flex items-center justify-end gap-2">
      <QtermButton
        size="sm"
        variant="outline"
        class="h-7! min-h-7! text-xs"
        :disabled="props.disabled"
        data-ai-tool-reject
        @click="emit('cancel')"
      >
        <Ban class="size-3.5" aria-hidden="true" />
        {{ t('shell.aiChat.commandCard.skip') }}
      </QtermButton>
      <QtermButton
        size="sm"
        class="h-7! min-h-7! text-xs"
        :disabled="props.disabled"
        data-ai-tool-approve
        @click="emit('confirm', remember)"
      >
        <PencilLine class="size-3.5" aria-hidden="true" />
        {{ t('shell.aiChat.confirmation.apply') }}
      </QtermButton>
    </div>
  </div>
</template>
