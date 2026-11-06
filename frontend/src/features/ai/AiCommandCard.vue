<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Ban, Check, CheckCircle2, ChevronDown, CircleAlert, Copy, LoaderCircle, Play, Square } from '@lucide/vue'
import { IconButton, QtermButton } from '../../components/common'
import { CodeBlock, CodeBlockContent, CodeBlockToken, useHighlightedCode } from '../../components/common/ai'
import { useI18n } from '../../locales'
import { writeClipboardText } from '../../services/clipboard'
import { toast } from 'vue-sonner'

type CommandRisk = 'readonly' | 'confirm' | 'danger'

const props = defineProps<{
  command: string
  target: string

  policy: string
  risk: CommandRisk
  warning?: string
  awaitingConfirmation: boolean
  running: boolean
  reviewing: boolean
  disabled: boolean

  canRun?: boolean

  output?: string
  resultState?: 'unknown' | 'not_sent'

  outputTarget?: string
  exitCode?: number

  showAnalyze?: boolean

  inlineLines?: number
}>()

const emit = defineEmits<{
  run: []
  confirm: []
  cancel: []
  stop: []
  analyze: []
}>()

const { t } = useI18n()

const outputOpen = ref(false)
const copied = ref(false)
const highlightedLines = useHighlightedCode(() => props.command, 'bash')

const hasOutput = computed(() => props.output !== undefined && (!props.resultState || Boolean(props.output)))

const riskLabel = computed(() => {
  if (props.risk === 'readonly') return t('shell.aiChat.commandCard.riskReadonly')
  if (props.risk === 'danger') return t('shell.aiChat.commandCard.riskDanger')
  return t('shell.aiChat.commandCard.riskConfirm')
})

const riskClass = computed(() => {
  if (props.risk === 'readonly') return 'border-qt-success/40 bg-qt-success/10 text-qt-success'
  if (props.risk === 'danger') return 'border-qt-danger/40 bg-qt-danger/10 text-qt-danger'
  return 'border-qt-accent/40 bg-qt-accent/10 text-qt-accent'
})

const cardBorderClass = computed(() => {
  if (props.resultState) return 'border-l-qt-warning'
  if (props.running || props.reviewing) return 'border-l-qt-accent'
  if (props.exitCode !== undefined) return props.exitCode === 0 ? 'border-l-qt-success' : 'border-l-qt-danger'
  if (props.risk === 'readonly') return 'border-l-qt-success'
  if (props.risk === 'danger') return 'border-l-qt-danger'
  return 'border-l-qt-accent'
})

const exitCodeClass = computed(() => (props.exitCode === 0 ? 'text-qt-success' : 'text-qt-danger'))

function updateOutputOpen(event: Event): void {
  outputOpen.value = (event.currentTarget as HTMLDetailsElement).open
}

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
  <article
    class="min-w-0 overflow-hidden rounded-lg border border-qt-border/60 border-l-[3px] bg-qt-subtle/25"
    :class="cardBorderClass"
    data-ai-command-card
  >
    <header class="flex min-w-0 items-center gap-2 border-b border-qt-border/40 px-3 py-2 text-xs text-qt-muted">
      <span class="shrink-0 font-medium uppercase tracking-[0.12em]">{{ t('shell.aiChat.commandCard.shell') }}</span>
      <span v-if="props.target" class="min-w-0 truncate">
        {{ t('shell.aiChat.commandCard.target') }}: {{ props.target }}
      </span>
      <span
        class="ms-auto max-w-[48%] truncate rounded-full border px-2 py-0.5 text-xs font-medium"
        :class="riskClass"
        :aria-label="`${t('shell.aiChat.commandCard.localAssessment')}: ${riskLabel}`"
      >
        {{ riskLabel }}
      </span>
    </header>

    <div class="flex min-w-0 items-start gap-1 border-b border-qt-border/40">
      <CodeBlock size="sm" class="min-w-0 flex-1 rounded-none border-0 bg-transparent">
        <CodeBlockContent class="max-h-32 overflow-auto px-3 py-2 text-xs leading-5" :wrap="false">
          <span v-for="(line, index) in highlightedLines" :key="index" class="block min-h-5"
            ><CodeBlockToken v-for="(token, tokenIndex) in line" :key="tokenIndex" :token="token"
          /></span>
        </CodeBlockContent>
      </CodeBlock>
      <IconButton
        :label="t(copied ? 'shell.aiChat.copied' : 'shell.aiChat.copyCommand')"
        size="icon-sm"
        class="me-2 mt-1 size-7! min-h-7! shrink-0"
        @click="copyCommand"
        ><Check v-if="copied" class="size-3.5" aria-hidden="true" /><Copy v-else class="size-3.5" aria-hidden="true"
      /></IconButton>
    </div>

    <p
      v-if="props.resultState"
      class="m-0 flex gap-2 border-b border-qt-border/40 px-3 py-2 text-xs leading-5 text-qt-warning"
      role="status"
      data-ai-command-result-warning
    >
      <CircleAlert class="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
      <span>{{
        t(props.resultState === 'unknown' ? 'shell.aiChat.commandOutcomeUnknown' : 'shell.aiChat.commandNotSent')
      }}</span>
    </p>
    <div class="space-y-1.5 px-3 py-2 text-xs leading-5 text-qt-muted">
      <div class="grid grid-cols-[max-content_minmax(0,1fr)] gap-x-2 gap-y-1">
        <span class="font-medium text-qt-text/75">{{ t('shell.aiChat.commandCard.policyLabel') }}</span>
        <span class="min-w-0 break-words">{{ props.policy }}</span>
        <span class="font-medium text-qt-text/75">{{ t('shell.aiChat.commandCard.localAssessment') }}</span>
        <span class="min-w-0 break-words">{{ riskLabel }}</span>
      </div>
      <p v-if="props.warning" class="m-0 flex items-start gap-1.5 text-qt-danger">
        <CircleAlert class="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
        <span class="min-w-0 break-words">{{ props.warning }}</span>
      </p>
    </div>

    <div
      v-if="props.awaitingConfirmation || props.running || props.reviewing || props.canRun !== false"
      class="flex flex-wrap items-center gap-2 border-t border-qt-border/40 px-3 py-2"
    >
      <template v-if="props.reviewing">
        <QtermButton size="sm" variant="outline" class="h-7! min-h-7!" disabled>
          <LoaderCircle class="size-3.5 animate-spin" aria-hidden="true" />
          {{ t('shell.aiChat.reviewing') }}
        </QtermButton>
      </template>
      <template v-else-if="props.running">
        <QtermButton size="sm" variant="outline" class="h-7! min-h-7!" @click="emit('stop')">
          <Square class="size-3.5" aria-hidden="true" />
          {{ t('shell.aiChat.stop') }}
        </QtermButton>
      </template>
      <template v-else-if="props.awaitingConfirmation">
        <QtermButton
          size="sm"
          variant="outline"
          class="h-7! min-h-7!"
          :disabled="props.disabled"
          @click="emit('cancel')"
        >
          <Ban class="size-3.5" aria-hidden="true" />
          {{ t('shell.aiChat.commandCard.skip') }}
        </QtermButton>
        <QtermButton size="sm" class="h-7! min-h-7!" :disabled="props.disabled" @click="emit('confirm')">
          <CheckCircle2 class="size-3.5" aria-hidden="true" />
          {{ t('shell.aiChat.runCommand') }}
        </QtermButton>
      </template>
      <QtermButton
        v-else-if="props.canRun !== false"
        size="sm"
        class="h-7! min-h-7!"
        :disabled="props.disabled"
        data-ai-rerun-command
        @click="emit('run')"
      >
        <Play class="size-3.5" aria-hidden="true" />
        {{ t('shell.aiChat.runCommand') }}
      </QtermButton>
    </div>

    <details v-if="hasOutput" :open="outputOpen" class="border-t border-qt-border/40" @toggle="updateOutputOpen">
      <summary
        class="flex cursor-pointer list-none items-center gap-1.5 px-3 py-2 text-xs text-qt-muted transition-colors [&::-webkit-details-marker]:hidden hover:bg-qt-hover/60 hover:text-qt-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-qt-accent/50"
      >
        <ChevronDown class="size-3.5 transition-transform" :class="outputOpen ? 'rotate-180' : ''" aria-hidden="true" />
        <span>{{
          outputOpen ? t('shell.aiChat.commandCard.collapseOutput') : t('shell.aiChat.commandCard.expandOutput')
        }}</span>
        <span v-if="props.exitCode !== undefined" class="ms-auto font-medium tabular-nums" :class="exitCodeClass">
          {{ t('shell.aiChat.exitCode', { code: props.exitCode }) }}
        </span>
      </summary>
      <div class="border-t border-qt-border/40">
        <p v-if="props.outputTarget" class="m-0 truncate px-3 pt-2 text-xs text-qt-muted" :title="props.outputTarget">
          {{ t('shell.aiChat.targetSession', { name: props.outputTarget }) }}
        </p>
        <pre
          class="m-0 max-h-48 overflow-auto bg-qt-subtle/30 px-3 py-2 font-mono text-xs leading-5 whitespace-pre text-qt-muted"
          :style="props.inlineLines ? { maxHeight: `${props.inlineLines * 1.25}rem` } : undefined"
          >{{ props.output || t('shell.aiChat.notAvailable') }}</pre>
        <div class="flex items-center justify-end px-3 py-2">
          <QtermButton
            v-if="props.showAnalyze !== false"
            size="sm"
            variant="outline"
            class="h-7! min-h-7!"
            :disabled="!props.output"
            @click="emit('analyze')"
          >
            {{ t('shell.aiChat.analyzeOutput') }}
          </QtermButton>
        </div>
      </div>
    </details>
  </article>
</template>
