<script setup lang="ts">
import { computed } from 'vue'
import { CircleAlert, Play, Ban } from '@lucide/vue'
import { CodeBlock, CodeBlockContent } from '../../components/common/ai'
import { IconButton, QtermButton } from '../../components/common'
import { useI18n } from '../../locales'
import { writeClipboardText } from '../../services/clipboard'
import { toast } from 'vue-sonner'
import { Copy } from '@lucide/vue'

const props = defineProps<{
  command: string
  risk: 'readonly' | 'confirm' | 'danger'
  warning?: string
  disabled?: boolean
}>()

const emit = defineEmits<{ confirm: []; cancel: [] }>()
const { t } = useI18n()

const riskLabel = computed(() => {
  if (props.risk === 'readonly') return t('shell.aiChat.commandCard.riskReadonly')
  if (props.risk === 'danger') return t('shell.aiChat.commandCard.riskDanger')
  return t('shell.aiChat.commandCard.riskConfirm')
})

async function copyCommand(): Promise<void> {
  try {
    await writeClipboardText(props.command)
    toast.success(t('shell.aiChat.copied'))
  } catch {
    toast.error(t('shell.aiChat.copyFailed'))
  }
}
</script>

<template>
  <div
    class="flex min-w-0 flex-col gap-2 rounded-lg border border-qt-warning/40 bg-qt-warning/5 p-3"
    data-ai-confirmation
  >
    <p class="m-0 flex items-center gap-1.5 text-xs font-medium text-qt-warning">
      <CircleAlert class="size-3.5 shrink-0" aria-hidden="true" />
      {{ t('shell.aiChat.confirmation.title') }}
    </p>
    <div class="flex min-w-0 items-start gap-1">
      <CodeBlock size="sm" class="min-w-0 flex-1 rounded-none border-0 bg-transparent">
        <CodeBlockContent class="max-h-32 overflow-auto px-2 py-1.5 text-xs leading-5" :wrap="false">{{
          props.command
        }}</CodeBlockContent>
      </CodeBlock>
      <IconButton
        :label="t('shell.aiChat.copyCommand')"
        size="icon-sm"
        class="me-1 mt-1 border-0 bg-transparent shadow-none hover:bg-transparent"
        @click="copyCommand"
      >
        <Copy class="size-3.5" aria-hidden="true" />
      </IconButton>
    </div>
    <p class="m-0 text-xs text-qt-muted">
      {{ t('shell.aiChat.confirmation.risk') }}：<span class="text-qt-text">{{ riskLabel }}</span>
    </p>
    <p v-if="props.warning" class="m-0 text-xs text-qt-danger">{{ props.warning }}</p>
    <div class="flex items-center justify-end gap-2">
      <QtermButton
        size="sm"
        variant="outline"
        class="h-7! min-h-7! text-xs"
        :disabled="props.disabled"
        @click="emit('cancel')"
      >
        <Ban class="size-3.5" aria-hidden="true" />
        {{ t('shell.aiChat.commandCard.skip') }}
      </QtermButton>
      <QtermButton size="sm" class="h-7! min-h-7! text-xs" :disabled="props.disabled" @click="emit('confirm')">
        <Play class="size-3.5" aria-hidden="true" />
        {{ t('shell.aiChat.runCommand') }}
      </QtermButton>
    </div>
  </div>
</template>
