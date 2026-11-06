<script setup lang="ts">
import { ref, watch } from 'vue'
import type { SessionState } from '../../services/sessions'
import AiConversation from './AiConversation.vue'
import { useI18n } from '../../locales'
import { useAILaunch } from '../../services/aiAgentLaunch'

const props = defineProps<{
  hostId?: string
  sessionId?: string
  sessionTitle?: string
  sessionState?: SessionState
  terminalContext?: string
}>()

const { t } = useI18n()

const AGENT_MODE_KEY = 'qterm.ai.agent-mode.v1'

function readMode(): 'ask' | 'agent' {
  return window.localStorage.getItem(AGENT_MODE_KEY) === 'agent' ? 'agent' : 'ask'
}

const mode = ref<'ask' | 'agent'>(readMode())
const launch = useAILaunch()

function setMode(value: string): void {
  mode.value = value === 'agent' ? 'agent' : 'ask'
  window.localStorage.setItem(AGENT_MODE_KEY, mode.value)
}

watch(
  launch,
  (value) => {
    if (value?.mode === 'agent' && mode.value !== 'agent') setMode('agent')
  },
  { immediate: true },
)
</script>

<template>
  <section class="flex h-full min-h-0 min-w-0 flex-col" :aria-label="t('shell.aiChat.title')">
    <AiConversation
      :active="true"
      :terminal-context="props.terminalContext"
      :host-id="props.hostId"
      :session-id="props.sessionId"
      :session-title="props.sessionTitle"
      :session-state="props.sessionState ?? 'closed'"
      :chat-mode="mode"
      @update:mode="setMode"
    />
  </section>
</template>
