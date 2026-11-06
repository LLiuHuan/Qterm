<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { computed } from 'vue'
import { ChevronRightIcon } from '@lucide/vue'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { CodeBlock, CodeBlockContent } from '@/components/ai-elements/code-block'
import { cn } from '@/lib/utils'
import { useI18n } from '@/locales'

const props = withDefaults(
  defineProps<{
    class?: HTMLAttributes['class']
    title: string
    command: string
    state: string
    output?: string
    exitCode?: number
    defaultOpen?: boolean
  }>(),
  { class: undefined, output: undefined, exitCode: undefined, defaultOpen: false },
)

const { t } = useI18n()
const tone = computed(() => {
  if (props.state === 'completed') return 'bg-qt-success'
  if (props.state === 'failed' || props.state === 'rejected' || props.state === 'timed_out') return 'bg-qt-danger'
  if (props.state === 'running') return 'bg-qt-accent'
  return 'bg-qt-muted'
})
const stateLabel = computed(() => {
  if (props.state === 'completed') return t('shell.aiChat.agent.completed')
  if (props.state === 'running') return t('shell.aiChat.agent.running')
  if (props.state === 'failed') return t('shell.aiChat.agent.failed')
  if (props.state === 'rejected') return t('shell.aiChat.agent.rejected')
  if (props.state === 'cancelled') return t('shell.aiChat.agent.cancelled')
  if (props.state === 'timed_out') return t('shell.aiChat.agent.timedOut')
  return t('shell.aiChat.agent.waiting')
})
</script>

<template>
  <Collapsible :default-open="props.defaultOpen" :class="cn('flex w-full min-w-0 flex-col', props.class)">
    <CollapsibleTrigger
      class="group/tool flex w-fit max-w-full min-w-0 cursor-pointer items-center gap-1 rounded-md text-left text-xs text-qt-muted outline-none transition-colors hover:text-qt-text focus-visible:ring-3 focus-visible:ring-ring/50"
    >
      <span class="size-1.5 shrink-0 rounded-full" :class="tone" aria-hidden="true" />
      <span class="truncate">&gt;_ {{ t('shell.aiChat.agent.stepCommand') }} · {{ props.title }}</span>
      <span class="shrink-0 text-qt-muted/70">{{ stateLabel }}</span>
      <ChevronRightIcon
        class="size-3.5 shrink-0 transition-transform group-data-[state=open]/tool:rotate-90"
        aria-hidden="true"
      />
    </CollapsibleTrigger>
    <CollapsibleContent class="mt-1 min-w-0">
      <CodeBlock size="sm">
        <CodeBlockContent language="bash">{{ props.command }}</CodeBlockContent>
      </CodeBlock>
      <pre
        v-if="props.output"
        class="m-0 mt-1 max-h-48 overflow-auto rounded-md border-l-2 bg-qt-subtle/30 px-2 py-1.5 font-mono text-xs leading-5 whitespace-pre text-qt-muted"
        :class="props.state === 'completed' ? 'border-l-qt-success/60' : 'border-l-qt-danger/60'"
        >{{ props.output }}</pre>
      <span v-if="props.exitCode !== undefined" class="mt-1 block text-xs text-qt-muted">
        {{ t('shell.aiChat.exitCode', { code: props.exitCode }) }}
      </span>
    </CollapsibleContent>
  </Collapsible>
</template>
