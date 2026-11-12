<script setup lang="ts">
import { computed, ref } from 'vue'
import { Check, ChevronRightIcon, Copy, FileText, FolderOpen, Server } from '@lucide/vue'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '../../components/common/control-primitives'
import { Tool } from '../../components/ai-elements/tool'
import { IconButton } from '../../components/common'
import { useI18n } from '../../locales'
import { writeClipboardText } from '../../services/clipboard'
import { StepKind, type AgentStep } from '../../services/aiAgent'
import { toast } from 'vue-sonner'

const props = defineProps<{
  step: AgentStep
}>()

const { t } = useI18n()
const copied = ref(false)

const isCommand = computed(() => props.step.kind === StepKind.StepKindCommand)

const title = computed(() => {
  switch (props.step.kind) {
    case StepKind.StepKindListHosts:
      return t('shell.aiChat.tool.listHosts')
    case StepKind.StepKindGetHost:
      return t('shell.aiChat.tool.getHost')
    case StepKind.StepKindSftpList:
      return t('shell.aiChat.tool.sftpList')
    case StepKind.StepKindSftpReadFile:
      return t('shell.aiChat.tool.sftpReadFile')
    case StepKind.StepKindSftpWriteFile:
      return t('shell.aiChat.tool.sftpWriteFile')
    case StepKind.StepKindLocalListDir:
      return t('shell.aiChat.tool.localListDir')
    case StepKind.StepKindLocalReadFile:
      return t('shell.aiChat.tool.localReadFile')
    case StepKind.StepKindWebFetch:
      return t('shell.aiChat.tool.webFetch')
    default:
      return props.step.title || props.step.kind
  }
})

const detail = computed(() => {
  const args = props.step.args as
    { query?: string; limit?: number; host_id?: string; path?: string; size?: number; url?: string } | undefined
  switch (props.step.kind) {
    case StepKind.StepKindListHosts: {
      const parts: string[] = []
      if (args?.query) parts.push(`query=${args.query}`)
      if (args?.limit) parts.push(`limit=${args.limit}`)
      return parts.join(' ')
    }
    case StepKind.StepKindGetHost:
      return args?.host_id ?? ''
    case StepKind.StepKindSftpList:
    case StepKind.StepKindSftpReadFile:
    case StepKind.StepKindSftpWriteFile:
    case StepKind.StepKindLocalListDir:
    case StepKind.StepKindLocalReadFile: {
      const path = args?.path ?? ''
      const size = (args as { size?: number } | undefined)?.size
      return size === undefined ? path : `${path} · ${size} B`
    }
    case StepKind.StepKindWebFetch: {
      const url = (args as { url?: string } | undefined)?.url ?? ''
      return url
    }
    default:
      return ''
  }
})

const icon = computed(() => {
  switch (props.step.kind) {
    case StepKind.StepKindSftpList:
      return FolderOpen
    case StepKind.StepKindSftpReadFile:
      return FileText
    default:
      return Server
  }
})

const tone = computed(() => {
  if (props.step.state === 'completed') return 'bg-qt-success'
  if (props.step.state === 'failed' || props.step.state === 'rejected' || props.step.state === 'timed_out')
    return 'bg-qt-danger'
  if (props.step.state === 'running') return 'bg-qt-accent'
  return 'bg-qt-muted'
})

const stateLabel = computed(() => {
  switch (props.step.state) {
    case 'completed':
      return t('shell.aiChat.agent.completed')
    case 'running':
      return t('shell.aiChat.agent.running')
    case 'failed':
      return t('shell.aiChat.agent.failed')
    case 'rejected':
      return t('shell.aiChat.agent.rejected')
    case 'cancelled':
      return t('shell.aiChat.agent.cancelled')
    case 'timed_out':
      return t('shell.aiChat.agent.timedOut')
    default:
      return t('shell.aiChat.agent.waiting')
  }
})

async function copyOutput(): Promise<void> {
  if (!props.step.observation) return
  await writeClipboardText(props.step.observation)
  copied.value = true
  window.setTimeout(() => {
    copied.value = false
  }, 1500)
  toast.success(t('shell.aiChat.tool.copied'))
}
</script>

<template>
  <Tool
    v-if="isCommand"
    :title="props.step.title || props.step.command || props.step.kind"
    :command="props.step.command || props.step.title || ''"
    :state="props.step.state"
    :output="props.step.observation ?? undefined"
    :exit-code="props.step.exitCode ?? undefined"
  />
  <Collapsible v-else class="flex w-full min-w-0 flex-col" data-ai-tool-card>
    <CollapsibleTrigger
      class="group/tool flex w-fit max-w-full min-w-0 cursor-pointer items-center gap-1 rounded-md text-left text-xs text-qt-muted outline-none transition-colors hover:text-qt-text focus-visible:ring-3 focus-visible:ring-ring/50"
    >
      <span class="size-1.5 shrink-0 rounded-full" :class="tone" aria-hidden="true" />
      <component :is="icon" class="size-3.5 shrink-0" aria-hidden="true" />
      <span class="shrink-0">{{ title }}</span>
      <span v-if="detail" class="truncate font-mono text-qt-muted/80">{{ detail }}</span>
      <span class="shrink-0 text-qt-muted/70">{{ stateLabel }}</span>
      <ChevronRightIcon
        class="size-3.5 shrink-0 transition-transform group-data-[state=open]/tool:rotate-90"
        aria-hidden="true"
      />
    </CollapsibleTrigger>
    <CollapsibleContent class="mt-1 min-w-0">
      <template v-if="props.step.observation">
        <pre
          data-ai-tool-output
          class="m-0 max-h-48 overflow-auto rounded-md border-l-2 bg-qt-subtle/30 px-2 py-1.5 font-mono text-xs leading-5 whitespace-pre-wrap break-words text-qt-muted"
          :class="props.step.state === 'completed' ? 'border-l-qt-success/60' : 'border-l-qt-danger/60'"
          >{{ props.step.observation }}</pre>
        <div class="mt-1 flex items-center gap-2">
          <IconButton :label="t('shell.aiChat.tool.copyOutput')" data-ai-tool-copy @click="copyOutput">
            <Check v-if="copied" class="size-3.5" />
            <Copy v-else class="size-3.5" />
          </IconButton>
          <span v-if="props.step.observationTruncated" class="text-xs text-qt-muted/70">
            {{ t('shell.aiChat.tool.truncated') }}
          </span>
        </div>
      </template>
      <p v-else class="m-0 text-xs text-qt-muted/70">{{ t('shell.aiChat.tool.noOutput') }}</p>
    </CollapsibleContent>
  </Collapsible>
</template>
