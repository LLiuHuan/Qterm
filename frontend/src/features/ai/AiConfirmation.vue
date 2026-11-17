<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Ban, Check, CircleAlert, Copy, Play } from '@lucide/vue'
import {
  CodeBlock,
  CodeBlockContent,
  CodeBlockLine,
  CodeBlockToken,
  useHighlightedCode,
} from '../../components/common/ai'
import { IconButton, QtermButton } from '../../components/common'
import { useI18n } from '../../locales'
import { writeClipboardText } from '../../services/clipboard'
import { toast } from '../../services/toast'

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

const highlightedLines = useHighlightedCode(() => props.command, 'bash')
const copied = ref(false)

const isSingleLine = computed(() => !props.command.includes('\n'))

async function copyCommand(): Promise<void> {
  try {
    await writeClipboardText(props.command)
    copied.value = true
    toast.success(t('shell.aiChat.copied'))
  } catch {
    toast.error(t('shell.aiChat.copyFailed'))
  }
}

watch(
  () => props.command,
  () => {
    copied.value = false
  },
)
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
    <div class="min-w-0 overflow-hidden">
      <CodeBlock size="sm" class="min-w-0 rounded-none border-0 bg-transparent">
        <CodeBlockContent v-if="isSingleLine" class="px-2 py-1.5 text-xs leading-5" :wrap="false">
          <span v-for="(line, index) in highlightedLines" :key="index" class="block min-h-5"
            ><CodeBlockToken v-for="(token, tokenIndex) in line" :key="tokenIndex" :token="token"
          /></span>
        </CodeBlockContent>
        <CodeBlockContent
          v-else
          line-numbers
          class="max-h-32 overflow-auto px-2 py-1.5 text-xs leading-5"
          :wrap="false"
        >
          <CodeBlockLine v-for="(line, index) in highlightedLines" :key="index">
            <CodeBlockToken v-for="(token, tokenIndex) in line" :key="tokenIndex" :token="token" />
          </CodeBlockLine>
        </CodeBlockContent>
      </CodeBlock>
    </div>

    <div class="flex min-w-0 items-center gap-2">
      <span class="shrink-0 text-xs text-qt-muted">
        {{ t('shell.aiChat.confirmation.risk') }}：<span class="text-qt-text">{{ riskLabel }}</span>
      </span>
      <IconButton
        :label="t(copied ? 'shell.aiChat.copied' : 'shell.aiChat.copyCommand')"
        size="icon-sm"
        class="ms-auto size-6! min-h-6! shrink-0 border-0 bg-transparent shadow-none hover:bg-transparent"
        @click="copyCommand"
      >
        <Check v-if="copied" class="size-3.5" aria-hidden="true" />
        <Copy v-else class="size-3.5" aria-hidden="true" />
      </IconButton>
    </div>
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
