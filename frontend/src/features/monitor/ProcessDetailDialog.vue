<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { Check, Copy, Skull, Square } from '@lucide/vue'
import { QtermButton, QtermFormMessage, QtermInfoList, type QtermInfoItem } from '@/components/common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '@/components/common/dialog'
import { ScrollArea } from '@/components/common/control-primitives'
import DeleteConfirmDialog from '@/components/dialogs/DeleteConfirmDialog.vue'
import { writeClipboardText } from '@/services/clipboard'
import { useI18n } from '@/locales'
import type { ProcessAction, ProcessInfo } from '@/services/monitor'

const props = withDefaults(
  defineProps<{
    open: boolean
    process: ProcessInfo | null
    parentName?: string
    childCount?: number
    descendantCount?: number
    actionBusy?: boolean
    actionError?: string
  }>(),
  { parentName: '', childCount: 0, descendantCount: 0, actionBusy: false, actionError: '' },
)

const emit = defineEmits<{
  close: []
  action: [payload: { action: ProcessAction }]
}>()

const { t } = useI18n()
const pendingAction = ref<ProcessAction | null>(null)
const copied = ref<'pid' | 'command' | null>(null)
let copiedTimer: number | undefined

const processName = computed(() => {
  const command = props.process?.command?.trim() ?? ''
  const firstToken = command.split(/\s+/, 1)[0] ?? ''
  return firstToken.split('/').pop() || firstToken || t('shell.monitor.detailUnknownProcess')
})

const processDescription = computed(() => {
  if (!props.process) return ''
  return `PID ${props.process.pid} · ${props.process.user || '—'}`
})

const details = computed<QtermInfoItem[]>(() => {
  const process = props.process
  if (!process) return []
  return [
    { label: t('shell.monitor.detailPid'), value: String(process.pid), mono: true },
    {
      label: t('shell.monitor.detailParent'),
      value: process.ppid > 0 ? `${props.parentName || '—'} (${process.ppid})` : '—',
    },
    { label: t('shell.monitor.detailUser'), value: process.user },
    { label: t('shell.monitor.detailState'), value: process.state },
    { label: t('shell.monitor.detailStartTime'), value: process.startTime },
    { label: t('shell.monitor.detailElapsed'), value: process.elapsed },
    { label: t('shell.monitor.detailTty'), value: process.tty, mono: true },
    {
      label: t('shell.monitor.detailPriority'),
      value: process.startTime && Number.isFinite(process.priority) ? String(process.priority) : '',
    },
    { label: t('shell.monitor.detailNice'), value: process.startTime ? String(process.nice) : '' },
    {
      label: t('shell.monitor.detailCpuUsage'),
      value: process.metricsAvailable ? `${process.cpu.toFixed(1)}%` : '',
    },
    { label: t('shell.monitor.detailCpuTime'), value: process.cpuTime, mono: true },
    {
      label: t('shell.monitor.detailResidentMemory'),
      value: process.metricsAvailable ? `${formatMemory(process.rssKb)} (${process.memory.toFixed(1)}%)` : '',
    },
    {
      label: t('shell.monitor.detailVirtualMemory'),
      value: process.startTime ? formatMemory(process.vszKb) : '',
    },
    { label: t('shell.monitor.detailChildren'), value: String(props.childCount ?? 0) },
    { label: t('shell.monitor.detailDescendants'), value: String(props.descendantCount ?? 0) },
  ]
})

const detailColumns = computed(() => {
  const middle = Math.ceil(details.value.length / 2)
  return [details.value.slice(0, middle), details.value.slice(middle)]
})

const canAct = computed(() => Boolean(props.process?.startTime) && !props.actionBusy)

const pendingTitle = computed(() =>
  pendingAction.value === 'kill' ? t('shell.monitor.confirmKillTitle') : t('shell.monitor.confirmTerminateTitle'),
)

const pendingMessage = computed(() => {
  if (!props.process) return ''
  const name = processName.value
  return pendingAction.value === 'kill'
    ? t('shell.monitor.confirmKillMessage', { pid: props.process.pid, name })
    : t('shell.monitor.confirmTerminateMessage', { pid: props.process.pid, name })
})

function formatMemory(kilobytes: number): string {
  if (!Number.isFinite(kilobytes)) return ''
  if (kilobytes <= 0) return '0 B'
  if (kilobytes >= 1024 * 1024) return `${(kilobytes / 1024 / 1024).toFixed(2)} GiB`
  return `${(kilobytes / 1024).toFixed(2)} MB`
}

function stopCopiedTimer(): void {
  if (copiedTimer !== undefined) {
    window.clearTimeout(copiedTimer)
    copiedTimer = undefined
  }
}

async function copy(value: string, field: 'pid' | 'command'): Promise<void> {
  if (!value || props.actionBusy) return
  try {
    await writeClipboardText(value)
    copied.value = field
    stopCopiedTimer()
    copiedTimer = window.setTimeout(() => {
      copied.value = null
      copiedTimer = undefined
    }, 1600)
  } catch {
    copied.value = null
  }
}

function requestAction(action: ProcessAction): void {
  if (!canAct.value) return
  pendingAction.value = action
}

function confirmAction(): void {
  const action = pendingAction.value
  pendingAction.value = null
  if (action && props.process?.startTime) emit('action', { action })
}

onBeforeUnmount(stopCopiedTimer)
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => !open && !props.actionBusy && emit('close')">
    <QtermDialogContent
      v-if="props.open && props.process"
      size="2xl"
      :close-on-escape="!props.actionBusy"
      class="process-detail-dialog"
      @pointer-down-outside="(event) => event.preventDefault()"
    >
      <QtermDialogHeader
        :title="processName"
        :description="processDescription"
        :badge="props.process.state"
        :close-disabled="props.actionBusy"
        @close="emit('close')"
      />
      <div class="min-h-0 px-4">
        <ScrollArea
          class="max-h-[min(62vh,620px)] w-full min-w-0 max-w-full overflow-hidden [&>[data-reka-scroll-area-viewport]]:max-h-[min(62vh,620px)]"
          type="scroll"
          :scroll-hide-delay="600"
        >
          <div class="flex flex-col gap-4 py-3">
            <section class="grid gap-2">
              <h3 class="m-0 text-sm font-medium text-qt-text">{{ t('shell.monitor.detailSection') }}</h3>
              <div class="grid gap-2 sm:grid-cols-2">
                <QtermInfoList v-for="(column, index) in detailColumns" :key="index" :items="column" />
              </div>
            </section>

            <section class="grid gap-2">
              <div class="flex-cb gap-2">
                <h3 class="m-0 text-sm font-medium text-qt-text">{{ t('shell.monitor.detailCommand') }}</h3>
                <QtermButton
                  variant="ghost"
                  size="sm"
                  type="button"
                  :aria-label="
                    copied === 'command' ? t('shell.monitor.detailCopied') : t('shell.monitor.detailCopyCommand')
                  "
                  @click="copy(props.process.command, 'command')"
                >
                  <Check v-if="copied === 'command'" :size="14" aria-hidden="true" />
                  <Copy v-else :size="14" aria-hidden="true" />
                  {{ copied === 'command' ? t('shell.monitor.detailCopied') : t('shell.monitor.detailCopyCommand') }}
                </QtermButton>
              </div>

              <pre
                class="m-0 min-w-0 max-h-[min(30vh,220px)] overflow-auto whitespace-pre-wrap break-all rounded-lg border border-qt-border px-3 py-2 font-mono text-xs text-qt-text"
                >{{ props.process.command || '—' }}</pre>
            </section>

            <QtermFormMessage v-if="props.actionError" variant="error">{{ props.actionError }}</QtermFormMessage>
            <p v-if="!props.process.startTime" class="m-0 text-xs text-qt-muted">
              {{ t('shell.monitor.detailActionUnavailable') }}
            </p>
          </div>
        </ScrollArea>
      </div>

      <QtermDialogFooter>
        <QtermButton
          variant="outline"
          size="sm"
          type="button"
          :disabled="props.actionBusy"
          @click="copy(String(props.process.pid), 'pid')"
        >
          <Check v-if="copied === 'pid'" :size="14" aria-hidden="true" />
          <Copy v-else :size="14" aria-hidden="true" />
          {{ copied === 'pid' ? t('shell.monitor.detailCopied') : t('shell.monitor.detailCopyPid') }}
        </QtermButton>
        <div class="ml-auto flex items-center gap-2">
          <QtermButton
            variant="outline"
            size="sm"
            type="button"
            class="text-qt-danger"
            :disabled="!canAct"
            @click="requestAction('terminate')"
          >
            <Square :size="14" aria-hidden="true" />
            {{ t('shell.monitor.terminate') }}
          </QtermButton>
          <QtermButton variant="destructive" size="sm" type="button" :disabled="!canAct" @click="requestAction('kill')">
            <Skull :size="14" aria-hidden="true" />
            {{ t('shell.monitor.kill') }}
          </QtermButton>
        </div>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>

  <DeleteConfirmDialog
    :open="pendingAction !== null"
    :title="pendingTitle"
    :message="pendingMessage"
    :confirm-text="pendingAction === 'kill' ? t('shell.monitor.kill') : t('shell.monitor.terminate')"
    :confirming-text="t('shell.monitor.actionRunning')"
    :loading="props.actionBusy"
    @close="pendingAction = null"
    @confirm="confirmAction"
  />
</template>
