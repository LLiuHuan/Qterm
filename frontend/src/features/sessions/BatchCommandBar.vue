<script setup lang="ts">
import { QtermButton, QtermInput } from '../../components/common'
import { computed, ref } from 'vue'
import { FileText, Send, OctagonX } from '@lucide/vue'
import { Separator } from '../../components/common/control-primitives'
import SnippetPickerDialog from './SnippetPickerDialog.vue'
import { sendTerminalInput } from '../../services/terminal'
import type { SessionTab } from '../../types/qterm'
import { useI18n } from '@/locales'

const props = defineProps<{ sessions: SessionTab[] }>()
const command = ref('')
const composing = ref(false)
const snippetPickerOpen = ref(false)
const { t } = useI18n()
const hasReady = computed(() => props.sessions.some((s) => s.state === 'connected' && s.streamState === 'ready'))

function broadcast(data: string): void {
  const seen = new Set<string>()
  for (const session of props.sessions) {
    if (seen.has(session.id)) continue
    seen.add(session.id)
    if (session.state !== 'connected' || session.streamState !== 'ready') continue
    sendTerminalInput(session.id, data)
  }
}
function sendSnippet(content: string): void {
  if (!content.trim()) return
  const data = content.replace(/\r\n|\n/g, '\r')
  broadcast(data.endsWith('\r') ? data : `${data}\r`)
}
function sendCommand(): void {
  if (!command.value.trim() || composing.value) return
  const data = command.value
  broadcast(data.endsWith('\r') ? data : `${data}\r`)
  command.value = ''
}
function onKeydown(event: KeyboardEvent): void {
  if (event.key !== 'Enter' || event.isComposing || composing.value || event.keyCode === 229) return
  event.preventDefault()
  if (!event.repeat) sendCommand()
}
</script>

<template>
  <section class="split-batch-command" :aria-label="t('shell.batch.aria')">
    <Separator class="batch-command-separator" />
    <form class="batch-command-row" @submit.prevent="sendCommand">
      <QtermInput
        v-model="command"
        class="batch-command-input text-xs"
        :placeholder="t('shell.batch.placeholder')"
        :aria-label="t('shell.batch.commandAria')"
        :title="t('shell.batch.sendTitle')"
        autocomplete="off"
        spellcheck="false"
        @compositionstart="composing = true"
        @compositionend="composing = false"
        @keydown="onKeydown"
      />
      <div class="batch-command-actions">
        <QtermButton
          type="button"
          variant="outline"
          class="batch-command-button text-xs"
          @click="snippetPickerOpen = true"
        >
          <FileText :size="14" aria-hidden="true" />{{ t('shell.batch.snippet') }}
        </QtermButton>
        <QtermButton
          type="submit"
          variant="outline"
          class="batch-command-button text-xs"
          :disabled="!command.trim() || !hasReady"
          ><Send :size="14" aria-hidden="true" />{{ t('shell.batch.send') }}</QtermButton
        >
        <QtermButton
          type="button"
          variant="outline"
          class="batch-command-button batch-interrupt text-xs"
          :disabled="!hasReady"
          :aria-label="t('shell.batch.interrupt')"
          @click="broadcast('\u0003')"
        >
          <OctagonX :size="14" aria-hidden="true" />Ctrl + C
        </QtermButton>
      </div>
    </form>
    <SnippetPickerDialog v-model:open="snippetPickerOpen" @select="sendSnippet" />
  </section>
</template>

<style scoped>
.split-batch-command {
  flex: 0 0 auto;
  min-width: 0;
  color: var(--qt-terminal-text);
}
.batch-command-separator[data-slot='separator'] {
  width: auto;
  margin: 8px 8px 0;
  background: var(--qt-border);
}
.batch-command-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 8px;
  align-items: center;
}
.batch-command-input {
  flex: 1 1 180px;
  min-width: 0;
  height: 32px;
}
.batch-command-actions {
  display: flex;
  gap: 8px;
  margin-left: auto;
  flex-wrap: wrap;
}
:global(.batch-command-button[data-slot='button']) {
  height: 32px !important;
  min-height: 32px;
  border-color: color-mix(in srgb, var(--qt-border) 82%, var(--qt-text) 18%);
  color: var(--qt-text);
  white-space: nowrap;
  background: color-mix(in srgb, var(--qt-panel) 22%, transparent);
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 8%);
}
:global(.batch-command-button[data-slot='button']:hover:not(:disabled)) {
  background: color-mix(in srgb, var(--qt-panel) 38%, transparent);
}
.batch-interrupt {
  color: var(--qt-danger);
}
</style>
