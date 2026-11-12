<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { AlertTriangle, ChevronDown, Play, ScrollText, Square, X } from '@lucide/vue'
import { QtermButton, QtermNumberField, QtermSwitch, QtermTextarea } from '../../../components/common'
import ResizeHandle from '../../../components/common/layout/ResizeHandle.vue'
import {
  QtermDialog,
  QtermDialogContent,
  QtermDialogFooter,
  QtermDialogHeader,
} from '../../../components/common/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../../components/common/form-controls'
import { ScrollArea } from '../../../components/common/control-primitives'
import { useI18n } from '../../../locales'
import type {
  BatchLogEntry,
  BatchSendMode,
  BatchSendRequest,
  BatchTargetSnapshot,
  BatchTaskView,
  SyncGroupView,
  TerminalControlSession,
} from '../types'
import SessionPicker from './SessionPicker.vue'

const props = withDefaults(
  defineProps<{
    sessions: TerminalControlSession[]
    currentSessionId: string | null
    syncGroup?: SyncGroupView | null
    task?: BatchTaskView | null
    log?: BatchLogEntry[]
    initialSessionId?: string | null
    busy?: boolean
    error?: string
  }>(),
  { syncGroup: null, task: null, log: () => [], initialSessionId: null, busy: false, error: '' },
)

const emit = defineEmits<{
  start: [request: BatchSendRequest, targets: BatchTargetSnapshot[]]
  cancel: [taskId: string]
  close: []
}>()

const { t } = useI18n()
const content = ref('')
const mode = ref<BatchSendMode>('append_cr')
const repeatCount = ref(1)
const intervalMs = ref(1000)

const selectedIds = ref<string[]>([])
const targetsTouched = ref(false)
const targetPickerOpen = ref(false)
const draftIds = ref<string[]>([])
const logOpen = ref(false)
const conflictOpen = ref(false)
const panelHeight = ref(280)

const clearAfterSend = ref(true)

const running = computed(() => props.busy || props.task?.status === 'running')
const readySessions = computed(() => props.sessions.filter((session) => session.ready))
const selectedTargets = computed(() =>
  selectedIds.value.filter((id) => readySessions.value.some((session) => session.id === id)),
)
const displayTargetIds = computed(() => {
  if (running.value) {
    const frozen = props.task?.sessionIds?.length
      ? props.task.sessionIds
      : (props.task?.results.map((result) => result.sessionId) ?? [])
    if (frozen.length) return frozen
  }
  return selectedTargets.value
})
const selectedSessionRows = computed(() =>
  displayTargetIds.value
    .map((id) => props.sessions.find((session) => session.id === id))
    .filter((session): session is TerminalControlSession => Boolean(session)),
)
const syncOverlap = computed(() => {
  if (props.syncGroup?.status !== 'active') return []
  const members = new Set(props.syncGroup?.sessionIds ?? [])
  return selectedTargets.value.filter((id) => members.has(id))
})
const normalizedContent = computed(() => content.value.replace(/\r\n/g, '\n').replace(/\r/g, '\n'))
const payloadPreview = computed(() => {
  if (mode.value === 'raw') return content.value
  if (mode.value === 'append_cr') return `${content.value}\r`
  const lines = normalizedContent.value.split('\n')
  if (lines.length > 1 && lines.at(-1) === '') lines.pop()
  return lines.map((line) => `${line}\r`).join('')
})
const byteCount = computed(() => new TextEncoder().encode(payloadPreview.value).byteLength)
const canStart = computed(() => !running.value && content.value.length > 0 && selectedTargets.value.length > 0)

const displayLog = computed<BatchLogEntry[]>(() => props.log)

const MODE_MESSAGE_KEY: Record<BatchSendMode, string> = {
  raw: 'raw',
  append_cr: 'cr',
  each_line_cr: 'lines',
}

function modeText(sendMode: BatchSendMode): string {
  return t(`terminalControl.batch.mode.${MODE_MESSAGE_KEY[sendMode]}`)
}

function bytesOf(text: string): number {
  return new TextEncoder().encode(text).byteLength
}

function formatTime(at: number): string {
  return new Date(at).toLocaleTimeString()
}

function start(): void {
  if (!content.value || !selectedTargets.value.length || running.value) return
  if (syncOverlap.value.length && !conflictOpen.value) {
    conflictOpen.value = true
    return
  }
  emit(
    'start',
    {
      sessionIds: [...selectedTargets.value],
      content: content.value,
      sendMode: mode.value,
      repeatCount: Math.max(1, Math.min(1000, Math.round(repeatCount.value || 1))),
      intervalMs: Math.max(0, Math.min(86400000, Math.round(intervalMs.value || 0))),
      pauseSync: syncOverlap.value.length > 0,
    },
    selectedSessionRows.value.map((session) => ({ id: session.id, title: session.title })),
  )
  conflictOpen.value = false

  if (clearAfterSend.value) content.value = ''
}

function cancelTask(): void {
  if (props.task?.id) emit('cancel', props.task.id)
}

function openTargetPicker(): void {
  if (running.value) return
  draftIds.value = [...selectedIds.value]
  targetPickerOpen.value = true
}

function closeTargetPicker(): void {
  targetPickerOpen.value = false
}

function confirmTargetPicker(): void {
  selectedIds.value = [...draftIds.value]
  targetsTouched.value = true
  targetPickerOpen.value = false
}

function resizePanel(delta: number): void {
  const viewportHeight = typeof window === 'undefined' ? 900 : window.innerHeight
  panelHeight.value = Math.max(220, Math.min(Math.round(viewportHeight * 0.65), panelHeight.value - delta))
}

watch(
  [() => props.initialSessionId, () => props.currentSessionId, () => props.sessions],
  () => {
    if (running.value || targetsTouched.value || selectedIds.value.length) return
    const id = props.initialSessionId || props.currentSessionId
    if (id && props.sessions.some((session) => session.id === id && session.ready)) selectedIds.value = [id]
  },
  { immediate: true },
)
watch(running, (value) => {
  if (!value) conflictOpen.value = false
})
</script>

<template>
  <section
    class="terminal-batch-panel relative flex min-h-0 shrink-0 flex-col border-t border-qt-border text-xs text-qt-text max-h-[65vh]"
    :style="{ height: `${panelHeight}px`, maxHeight: '65%' }"
    data-terminal-batch-panel
  >
    <ResizeHandle orientation="horizontal" :label="t('terminalControl.batch.height')" @resize="resizePanel" />
    <header class="flex min-h-9 shrink-0 items-center gap-2 border-b border-qt-border px-3 py-1.5">
      <span class="font-medium">{{ t('terminalControl.batch.title') }}</span>
      <span v-if="running" class="text-qt-muted"
        >·
        {{
          t('terminalControl.batch.running', {
            round: props.task?.completedRounds ?? 0,
            total: props.task?.repeatCount ?? 0,
          })
        }}</span
      >
      <span class="ml-auto flex shrink-0 items-center gap-1">
        <QtermButton
          size="xs"
          variant="ghost"
          :disabled="!props.task && !props.log.length"
          data-batch-log-open
          @click="logOpen = true"
        >
          <ScrollText :size="13" aria-hidden="true" />{{ t('terminalControl.batch.log') }}
        </QtermButton>
        <QtermButton v-if="running" size="xs" variant="destructive" @click="cancelTask">
          <Square :size="13" aria-hidden="true" />{{ t('terminalControl.batch.stop') }}
        </QtermButton>
        <QtermButton
          size="icon-xs"
          variant="ghost"
          :aria-label="t('terminalControl.batch.close')"
          @click="emit('close')"
        >
          <X :size="14" aria-hidden="true" />
        </QtermButton>
      </span>
    </header>

    <ScrollArea type="hover" :scroll-hide-delay="600" class="min-h-0 flex-1">
      <div class="p-3">
        <div class="flex flex-col gap-3">
          <div class="flex min-w-0 flex-col gap-1" data-batch-targets>
            <span class="text-qt-muted">{{ t('terminalControl.batch.targets') }}</span>
            <button
              type="button"
              class="flex min-h-8 w-full flex-wrap items-center gap-1.5 rounded-md border border-qt-control-border px-1.5 py-1 text-left text-xs hover:bg-qt-hover disabled:cursor-not-allowed disabled:opacity-55"
              :disabled="running"
              :aria-label="t('terminalControl.batch.targets')"
              data-batch-target-open
              @click="openTargetPicker"
            >
              <span
                v-for="session in selectedSessionRows"
                :key="session.id"
                class="inline-flex max-w-44 items-center truncate rounded border border-qt-border bg-qt-hover px-1.5 py-0.5 text-xs"
                :title="session.title"
                data-batch-target-tag
                >{{ session.title }}</span
              >
              <span v-if="!selectedSessionRows.length" class="flex items-center gap-1 text-qt-muted">
                {{ t('terminalControl.batch.selectTargets') }}
              </span>
              <ChevronDown v-else :size="13" class="ml-auto shrink-0 text-qt-muted" aria-hidden="true" />
            </button>
          </div>

          <div class="grid grid-cols-2 gap-2 md:grid-cols-3">
            <label class="flex min-w-0 flex-col gap-1 text-qt-muted">
              {{ t('terminalControl.batch.sendMode') }}
              <Select v-model="mode" :disabled="running">
                <SelectTrigger class="w-full" size="sm" :aria-label="t('terminalControl.batch.sendMode')"
                  ><SelectValue
                /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="raw">{{ t('terminalControl.batch.mode.raw') }}</SelectItem>
                  <SelectItem value="append_cr">{{ t('terminalControl.batch.mode.cr') }}</SelectItem>
                  <SelectItem value="each_line_cr">{{ t('terminalControl.batch.mode.lines') }}</SelectItem>
                </SelectContent>
              </Select>
            </label>
            <label class="flex min-w-0 flex-col gap-1 text-qt-muted">
              {{ t('terminalControl.batch.repeat') }}
              <QtermNumberField
                v-model="repeatCount"
                class="w-full"
                :min="1"
                :max="1000"
                :disabled="running"
                :aria-label="t('terminalControl.batch.repeat')"
              />
            </label>

            <label class="flex min-w-0 flex-col gap-1 text-qt-muted">
              {{ t('terminalControl.batch.interval') }}
              <QtermNumberField
                v-model="intervalMs"
                class="w-full"
                :min="0"
                :max="86400000"
                :disabled="running || repeatCount <= 1"
                :aria-label="t('terminalControl.batch.interval')"
              />
            </label>
          </div>

          <div class="flex flex-col gap-1">
            <div class="flex flex-wrap items-center gap-2">
              <span class="text-qt-muted">{{ t('terminalControl.batch.content') }}</span>
              <label class="ml-auto flex items-center gap-1.5 text-qt-muted">
                <QtermSwitch
                  :model-value="clearAfterSend"
                  size="sm"
                  data-batch-clear-after-send
                  :aria-label="t('terminalControl.batch.clearAfterSend')"
                  @update:model-value="(value) => (clearAfterSend = value === true)"
                />
                {{ t('terminalControl.batch.clearAfterSend') }}
              </label>
            </div>
            <QtermTextarea
              v-model="content"
              :disabled="running"
              class="min-h-24 w-full resize-y"
              :placeholder="t('terminalControl.batch.placeholder')"
              autocomplete="off"
              autocorrect="off"
              autocapitalize="off"
              spellcheck="false"
            />
          </div>

          <div class="flex flex-wrap items-center gap-2 text-xs text-qt-muted">
            <span>{{ t('terminalControl.batch.bytes', { count: byteCount }) }}</span>
            <span v-if="mode === 'append_cr'">{{ t('terminalControl.batch.crHint') }}</span>
            <span v-else-if="mode === 'each_line_cr'">{{ t('terminalControl.batch.lineHint') }}</span>
          </div>

          <p v-if="mode === 'append_cr' && /[\r\n]$/.test(content)" class="m-0 text-xs text-qt-danger">
            {{ t('terminalControl.batch.endlineWarning') }}
          </p>
          <p class="m-0 text-xs text-qt-muted">{{ t('terminalControl.batch.deliveryHint') }}</p>
          <div
            v-if="conflictOpen"
            class="flex flex-wrap items-center gap-2 rounded-md border border-qt-warning/50 bg-qt-warning/10 p-2 text-xs"
          >
            <AlertTriangle :size="14" class="shrink-0 text-qt-warning" aria-hidden="true" />
            <span class="min-w-0 flex-1"
              ><strong>{{ t('terminalControl.batch.conflictTitle') }}</strong>
              {{ t('terminalControl.batch.conflictText') }}</span
            >
            <QtermButton size="xs" @click="start">{{ t('terminalControl.batch.pauseAndSend') }}</QtermButton>
            <QtermButton size="xs" variant="ghost" @click="conflictOpen = false">{{
              t('terminalControl.batch.cancelSend')
            }}</QtermButton>
          </div>
        </div>
      </div>
    </ScrollArea>

    <footer class="flex shrink-0 items-center justify-end gap-2 border-t border-qt-border px-3 py-2">
      <span v-if="!content" class="mr-auto text-xs text-qt-muted">{{ t('terminalControl.batch.noContent') }}</span>
      <span v-else-if="!selectedTargets.length" class="mr-auto text-xs text-qt-muted">{{
        t('terminalControl.batch.noTargets')
      }}</span>
      <p v-if="props.error" class="mr-auto m-0 text-xs text-qt-danger" role="alert">{{ props.error }}</p>
      <QtermButton :disabled="!canStart" @click="start"
        ><Play :size="14" aria-hidden="true" />{{ t('terminalControl.batch.start') }}</QtermButton
      >
    </footer>

    <QtermDialog :open="targetPickerOpen" @update:open="(open) => !open && closeTargetPicker()">
      <QtermDialogContent
        v-if="targetPickerOpen"
        size="lg"
        class="max-h-[min(680px,calc(100vh-2rem))] overflow-hidden"
        data-batch-target-picker
      >
        <QtermDialogHeader
          :title="t('terminalControl.batch.pickerTitle')"
          :show-close="false"
          @close="closeTargetPicker"
        />
        <div class="dialog-body flex flex-col">
          <SessionPicker
            :sessions="props.sessions"
            :selected-ids="draftIds"
            :source-session-id="props.initialSessionId ?? props.currentSessionId ?? ''"
            :search-clear="false"
            select-all
            max-height-class="max-h-64"
            @update:selected-ids="(ids) => (draftIds = ids)"
          />
        </div>
        <QtermDialogFooter>
          <div></div>
          <div class="dialog-footer-actions flex items-center gap-[8px]">
            <QtermButton variant="outline" @click="closeTargetPicker">{{
              t('terminalControl.batch.cancelSend')
            }}</QtermButton>
            <QtermButton @click="confirmTargetPicker">{{ t('terminalControl.batch.confirm') }}</QtermButton>
          </div>
        </QtermDialogFooter>
      </QtermDialogContent>
    </QtermDialog>

    <QtermDialog :open="logOpen" @update:open="(open) => !open && (logOpen = false)">
      <QtermDialogContent v-if="logOpen" class="w-[min(600px,calc(100vw-48px))]" data-terminal-batch-log>
        <QtermDialogHeader :title="t('terminalControl.batch.log')" :show-close="false" @close="logOpen = false" />
        <div class="dialog-body flex flex-col gap-3">
          <p v-if="!displayLog.length" class="m-0 text-xs text-qt-muted">
            {{ t('terminalControl.batch.logEmpty') }}
          </p>

          <ScrollArea v-else type="hover" :scroll-hide-delay="600" class="max-h-[55vh]">
            <div class="flex flex-col gap-3 pr-3">
              <div
                v-for="entry in displayLog"
                :key="entry.id"
                class="flex flex-col gap-1.5 rounded-md border border-qt-border p-2"
                data-batch-log-entry
              >
                <div class="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-qt-muted">
                  <span v-if="entry.at">{{ formatTime(entry.at) }}</span>
                  <span>{{ modeText(entry.sendMode) }}</span>
                  <span>{{ t('terminalControl.batch.repeat') }} {{ entry.repeatCount }}</span>
                  <span>{{ t('terminalControl.batch.bytes', { count: bytesOf(entry.content) }) }}</span>
                  <span class="ml-auto">{{ t(`terminalControl.batch.status.${entry.status}`) }}</span>
                </div>
                <div class="flex flex-wrap gap-1">
                  <span
                    v-for="target in entry.targets"
                    :key="target.id"
                    class="inline-flex max-w-40 truncate rounded border border-qt-border bg-qt-hover px-1.5 py-0.5 text-xs"
                    :title="target.title || target.id"
                    >{{ target.title || target.id }}</span
                  >
                </div>
                <pre
                  class="m-0 max-h-32 overflow-auto whitespace-pre-wrap break-words rounded bg-qt-hover p-2 text-xs"
                  data-batch-log-content
                  >{{ entry.content || '—' }}</pre>
              </div>
            </div>
          </ScrollArea>
        </div>
        <QtermDialogFooter>
          <div></div>
          <div class="dialog-footer-actions flex items-center gap-[8px]">
            <QtermButton v-if="running" variant="destructive" @click="cancelTask">
              <Square :size="13" aria-hidden="true" />{{ t('terminalControl.batch.stop') }}
            </QtermButton>
            <QtermButton variant="outline" @click="logOpen = false">{{
              t('terminalControl.batch.dismiss')
            }}</QtermButton>
          </div>
        </QtermDialogFooter>
      </QtermDialogContent>
    </QtermDialog>
  </section>
</template>
