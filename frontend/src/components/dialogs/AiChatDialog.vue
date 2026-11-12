<script setup lang="ts">
import { QtermDialog, QtermDialogContent, QtermDialogHeader } from '../../components/common/dialog'
import { Sparkles } from '@lucide/vue'
import { useI18n } from '../../locales'
import type { SessionState } from '../../services/sessions'
import AiPanel from '../../features/ai/AiPanel.vue'

const props = defineProps<{
  open: boolean

  terminalContext?: string

  hostId?: string
  sessionId?: string
  sessionTitle?: string
  sessionState?: SessionState

  global?: boolean

  availableHosts?: Array<{ id: string; name: string; address?: string }>
}>()

const emit = defineEmits<{ close: [] }>()

const { t } = useI18n()
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => !open && emit('close')">
    <QtermDialogContent
      v-if="props.open"
      data-ai-chat-dialog
      class="flex h-[min(760px,calc(100vh-2rem))] max-h-[calc(100vh-2rem)] w-[min(760px,calc(100vw-2rem))] max-w-[calc(100vw-2rem)] flex-col overflow-hidden"
      size="3xl"
      :show-close-button="false"
    >
      <QtermDialogHeader
        :title="t('shell.aiChat.title')"
        :description="t('shell.aiChat.description')"
        @close="emit('close')"
      >
        <template #icon>
          <span class="flex size-8 items-center justify-center rounded-lg bg-qt-subtle text-qt-muted">
            <Sparkles :size="16" aria-hidden="true" />
          </span>
        </template>
      </QtermDialogHeader>
      <div class="dialog-body flex min-h-0 flex-1 flex-col gap-0 overflow-hidden p-4">
        <AiPanel
          :terminal-context="props.terminalContext"
          :host-id="props.hostId"
          :session-id="props.sessionId"
          :session-title="props.sessionTitle"
          :session-state="props.global ? 'connected' : (props.sessionState ?? 'closed')"
          :available-hosts="props.availableHosts"
        />
      </div>
    </QtermDialogContent>
  </QtermDialog>
</template>
