<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ChevronDown, Cpu, Gauge, MemoryStick } from '@lucide/vue'
import { QtermEmptyState, QtermSearchField } from '@/components/common'
import { ScrollArea } from '@/components/common/control-primitives'
import QtermLoadMore from '@/components/common/layout/QtermLoadMore.vue'
import { useI18n } from '@/locales'
import ProcessDetailDialog from './ProcessDetailDialog.vue'
import { createMonitorGateway, type ProcessAction, type ProcessInfo } from '@/services/monitor'
import { errorMessage } from '@/services/errors'

const props = withDefaults(
  defineProps<{
    sessionId?: string

    available?: boolean
  }>(),
  { sessionId: '', available: false },
)

const { t } = useI18n()
const gateway = createMonitorGateway()

const REFRESH_INTERVAL = 15_000

const PAGE_SIZE = 50

const processes = ref<ProcessInfo[]>([])
const updatedAt = ref(0)
const loading = ref(false)
const error = ref('')
const query = ref('')
const sortKey = ref<'cpu' | 'memory'>('memory')
const sortDesc = ref(true)
const detailProcess = ref<ProcessInfo | null>(null)
const actionBusy = ref(false)
const actionError = ref('')

const visibleLimit = ref(PAGE_SIZE)

const visibleProcesses = computed(() => {
  const keyword = query.value.trim().toLowerCase()
  const filtered = keyword
    ? processes.value.filter(
        (item) =>
          item.command.toLowerCase().includes(keyword) ||
          item.user.toLowerCase().includes(keyword) ||
          String(item.pid).includes(keyword),
      )
    : processes.value
  const sorted = [...filtered].sort((a, b) => {
    const left = sortKey.value === 'cpu' ? a.cpu : a.memory
    const right = sortKey.value === 'cpu' ? b.cpu : b.memory
    return sortDesc.value ? right - left : left - right
  })
  return sorted
})

const pagedProcesses = computed(() =>
  visibleProcesses.value.slice(0, Math.min(visibleLimit.value, visibleProcesses.value.length)),
)

function loadMoreProcesses(): void {
  visibleLimit.value = Math.min(visibleProcesses.value.length, visibleLimit.value + PAGE_SIZE)
}

watch([query, sortKey, sortDesc], () => {
  visibleLimit.value = PAGE_SIZE
})

function toggleSort(key: 'cpu' | 'memory'): void {
  if (sortKey.value === key) {
    sortDesc.value = !sortDesc.value
    return
  }
  sortKey.value = key
  sortDesc.value = true
}

async function refresh(): Promise<void> {
  if (!props.available || !props.sessionId || loading.value) return
  loading.value = true
  error.value = ''
  try {
    const snapshot = await gateway.processes(props.sessionId)
    processes.value = snapshot.processes
    updatedAt.value = snapshot.updatedAt
  } catch (cause) {
    error.value = errorMessage(cause, t('shell.monitor.failed'))
  } finally {
    loading.value = false
  }
}

function formatProcessMemory(kilobytes: number): string {
  if (!Number.isFinite(kilobytes) || kilobytes <= 0) return '0'
  return `${(kilobytes / 1024).toFixed(2)} MB`
}

function processName(command: string): string {
  const firstToken = command.trim().split(/\s+/, 1)[0] ?? ''
  const base = firstToken.split('/').pop() || firstToken

  const unbraced = base.replace(/^\{+/, '').replace(/\}+$/, '')

  return unbraced.replace(/:$/, '') || '—'
}

function processStateClass(state: string): string {
  return state.toUpperCase().startsWith('Z') ? 'bg-qt-danger' : 'bg-qt-muted'
}

function openDetail(process: ProcessInfo): void {
  detailProcess.value = process
  actionError.value = ''
}

function parentProcessName(process: ProcessInfo | null): string {
  if (!process || process.ppid <= 0) return ''
  const parent = processes.value.find((item) => item.pid === process.ppid)
  return parent ? processName(parent.command) : ''
}

function childProcessCount(process: ProcessInfo | null): number {
  if (!process) return 0
  return processes.value.reduce((count, item) => count + (item.ppid === process.pid ? 1 : 0), 0)
}

function descendantProcessCount(process: ProcessInfo | null): number {
  if (!process) return 0
  const visited = new Set<number>([process.pid])
  const pending = [process.pid]
  while (pending.length > 0) {
    const parentPid = pending.pop()
    for (const candidate of processes.value) {
      if (candidate.ppid !== parentPid || visited.has(candidate.pid)) continue
      visited.add(candidate.pid)
      pending.push(candidate.pid)
    }
  }
  return visited.size - 1
}

async function runProcessAction(action: ProcessAction): Promise<void> {
  const process = detailProcess.value

  if (!process?.startTime || actionBusy.value) return
  actionBusy.value = true
  actionError.value = ''
  try {
    await gateway.processAction(props.sessionId, process.pid, process.startTime, action)
    detailProcess.value = null
    await refresh()
  } catch (cause) {
    actionError.value = errorMessage(cause, t('shell.monitor.actionFailed'))
  } finally {
    actionBusy.value = false
  }
}

function boundedPercent(value: number): number {
  if (!Number.isFinite(value)) return 0
  return Math.min(100, Math.max(0, value))
}

let timer: number | undefined
function startTimer(): void {
  stopTimer()
  timer = window.setInterval(() => {
    if (document.visibilityState === 'visible') void refresh()
  }, REFRESH_INTERVAL)
}
function stopTimer(): void {
  if (timer !== undefined) window.clearInterval(timer)
  timer = undefined
}

onMounted(() => {
  void refresh()
  startTimer()
})
onBeforeUnmount(stopTimer)

watch(
  () => [props.sessionId, props.available],
  () => void refresh(),
)
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <div v-if="!props.available" class="flex-1">
      <QtermEmptyState
        :icon="Gauge"
        :title="t('shell.monitor.noSession')"
        :description="t('shell.monitor.noSessionHint')"
      />
    </div>

    <template v-else>
      <div class="flex min-h-0 flex-1 flex-col">
        <div class="shrink-0 space-y-2 px-2 pb-2 pt-2">
          <QtermSearchField
            v-model="query"
            v-bind="{ ariaLabel: t('shell.monitor.search') }"
            :placeholder="t('shell.monitor.search')"
            :clear-label="t('shell.toolPanel.clearSearch')"
          />
          <div class="flex min-w-0 items-center gap-1">
            <button
              type="button"
              class="inline-flex min-w-0 items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm transition-colors"
              :class="sortKey === 'memory' ? 'bg-qt-info/10 text-qt-info' : 'text-qt-info hover:bg-qt-info/10'"
              :aria-pressed="sortKey === 'memory'"
              :aria-label="t('shell.monitor.memory')"
              @click="toggleSort('memory')"
            >
              <MemoryStick :size="16" aria-hidden="true" />
              <span class="truncate">{{ t('shell.monitor.memory') }}</span>
              <ChevronDown
                v-if="sortKey === 'memory'"
                :size="16"
                aria-hidden="true"
                :class="{ 'rotate-180': !sortDesc }"
              />
            </button>
            <button
              type="button"
              class="inline-flex min-w-0 items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm transition-colors"
              :class="sortKey === 'cpu' ? 'bg-qt-warning/10 text-qt-warning' : 'text-qt-warning hover:bg-qt-warning/10'"
              :aria-pressed="sortKey === 'cpu'"
              aria-label="CPU"
              @click="toggleSort('cpu')"
            >
              <Cpu :size="16" aria-hidden="true" />
              <span>CPU</span>
              <ChevronDown
                v-if="sortKey === 'cpu'"
                :size="16"
                aria-hidden="true"
                :class="{ 'rotate-180': !sortDesc }"
              />
            </button>
            <span class="ml-auto shrink-0 px-1 text-sm text-qt-muted">{{
              t('shell.monitor.count', { count: visibleProcesses.length })
            }}</span>
            <span v-if="loading" class="sr-only">{{ t('shell.monitor.loading') }}</span>
          </div>
        </div>
        <div v-if="error" class="px-2 pb-2 text-xs text-qt-danger">{{ error }}</div>
        <ScrollArea
          class="monitor-scroll min-h-0 min-w-0 w-full max-w-full flex-1 overflow-hidden"
          type="hover"
          :scroll-hide-delay="600"
        >
          <div class="monitor-scroll-content min-h-full w-full min-w-0 max-w-full space-y-2 px-2 pb-2">
            <article
              v-for="item in pagedProcesses"
              :key="item.pid"
              class="min-w-0 cursor-pointer rounded-[10px] border border-qt-border px-3 py-2.5 transition-colors hover:border-qt-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-qt-accent/50"
              role="button"
              tabindex="0"
              :title="item.command"
              @dblclick="openDetail(item)"
              @keydown.enter="openDetail(item)"
              @keydown.space.prevent="openDetail(item)"
            >
              <div class="flex min-w-0 items-center gap-2">
                <span
                  class="size-2.5 shrink-0 rounded-full"
                  :class="processStateClass(item.state)"
                  aria-hidden="true"
                />
                <span class="min-w-0 flex-1 truncate text-sm font-medium text-qt-text">{{
                  processName(item.command)
                }}</span>
                <span class="shrink-0 font-mono text-xs tabular-nums text-qt-muted">PID {{ item.pid }}</span>
              </div>
              <div class="mt-1 truncate text-xs text-qt-muted">{{ item.user }} · {{ item.startTime || '—' }}</div>

              <div class="mt-1.5 flex min-w-0 items-center justify-between gap-2 text-[11px]">
                <span class="min-w-0 truncate text-qt-info">
                  {{ t('shell.monitor.memory') }}
                  <span class="font-mono tabular-nums">{{ formatProcessMemory(item.rssKb) }}</span>
                  ·
                  <span class="font-mono tabular-nums">{{ item.memory.toFixed(1) }}%</span>
                </span>
                <span class="shrink-0 font-mono tabular-nums text-qt-warning">CPU {{ item.cpu.toFixed(1) }}%</span>
              </div>

              <div
                class="mt-1 h-1 overflow-hidden rounded-full bg-qt-muted/20"
                role="img"
                :title="`${t('shell.monitor.memory')} ${item.memory.toFixed(1)}%`"
                :aria-label="`${t('shell.monitor.memory')} ${item.memory.toFixed(1)}%`"
              >
                <span
                  class="block h-full rounded-full bg-qt-info/70 transition-[width]"
                  :style="{ width: `${boundedPercent(item.memory)}%` }"
                />
              </div>
            </article>
            <div v-if="visibleProcesses.length === 0" class="px-2 py-6 text-center text-xs text-qt-muted">
              {{ t('shell.monitor.emptyProcesses') }}
            </div>

            <QtermLoadMore
              :visible="pagedProcesses.length"
              :total="visibleProcesses.length"
              :load-more-label="t('shell.common.loadMore')"
              @load-more="loadMoreProcesses"
            />
          </div>
        </ScrollArea>
      </div>
    </template>
  </div>

  <ProcessDetailDialog
    :open="detailProcess !== null"
    :process="detailProcess"
    :parent-name="parentProcessName(detailProcess)"
    :child-count="childProcessCount(detailProcess)"
    :descendant-count="descendantProcessCount(detailProcess)"
    :action-busy="actionBusy"
    :action-error="actionError"
    @close="detailProcess = null"
    @action="runProcessAction($event.action)"
  />
</template>

<style scoped>
.monitor-scroll :deep([data-slot='scroll-area-viewport'] > div) {
  width: 100%;
  min-width: 0;
  max-width: 100%;
}
</style>
