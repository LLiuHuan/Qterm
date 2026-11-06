<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Folder, Pencil, Play, Plus, Route, Square, Trash2 } from '@lucide/vue'
import { QtermEmptyState, QtermSearchField, IconButton } from '@/components/common'
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/common/control-primitives'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from '@/components/common/overlay-controls'
import { useI18n } from '@/locales'
import type { HostRecord } from '@/services/hosts'
import {
  PORT_FORWARD_STATUS_EVENT,
  type PortForwardConfig,
  type PortForwardState,
  type PortForwardStatus,
} from '@/services/forwards'
import PortForwardEditorDialog from './PortForwardEditorDialog.vue'
import {
  mergeForwardGroups,
  forwardDisplayName,
  forwardGroupName,
  forwardHostLabel,
  forwardKindLabelKey,
  forwardSections,
  forwardSummary,
} from './forwardView'
import { usePortForwardActions } from './usePortForwardActions'

const props = withDefaults(
  defineProps<{
    forwards: PortForwardConfig[]
    hosts: HostRecord[]

    activeHostId?: string

    forwardGroups?: string[]

    dark?: boolean
  }>(),
  { activeHostId: '', forwardGroups: () => [], dark: false },
)

const emit = defineEmits<{
  'update:forwards': [value: PortForwardConfig[]]
  'update:forwardGroups': [value: string[]]
}>()
const { t } = useI18n()
const actions = usePortForwardActions({
  forwards: () => props.forwards,
  update: (next) => emit('update:forwards', next),
})

const statuses = ref<Record<string, PortForwardStatus>>({})
const busy = ref<Record<string, boolean>>({})
const query = ref('')
const editorOpen = ref(false)
const editing = ref<PortForwardConfig | null>(null)

const sshHosts = computed(() =>
  props.hosts.filter((host) => !host.protocol || host.protocol.trim().toLocaleLowerCase() === 'ssh'),
)

const groupOptions = computed(() => mergeForwardGroups(props.forwards, props.forwardGroups))

const ungroupedLabel = computed(() => t('shell.forwards.ungrouped'))

const sections = computed(() =>
  forwardSections(props.forwards, {
    hosts: props.hosts,
    kindLabel: (row) => t(forwardKindLabelKey(row.kind)),
    ungroupedLabel: ungroupedLabel.value,
    groups: props.forwardGroups,
    query: query.value,
  }),
)

const collapsed = ref<string[]>([])

const openGroups = computed<string[]>({
  get: () => {
    const all = sections.value.map((section) => section.key)
    if (query.value.trim()) return all
    return all.filter((key) => !collapsed.value.includes(key))
  },
  set: (value) => {
    if (query.value.trim()) return
    const all = sections.value.map((section) => section.key)
    collapsed.value = all.filter((key) => !value.includes(key))
  },
})

function stateOf(id: string): PortForwardState {
  return statuses.value[id]?.state ?? 'stopped'
}

function isActive(id: string): boolean {
  const state = stateOf(id)
  return state === 'running' || state === 'starting' || state === 'reconnecting'
}

function statusLabel(row: PortForwardConfig): string {
  const state = stateOf(row.id)
  const status = statuses.value[row.id]
  if (state === 'running') {
    const actual = status?.listenPort ?? 0
    return actual > 0 && actual !== row.listenPort
      ? `${t('shell.forwards.stateRunning')} · :${actual}`
      : t('shell.forwards.stateRunning')
  }
  if (state === 'reconnecting') return t('shell.forwards.stateReconnecting', { count: status?.retryAttempt ?? 0 })
  if (state === 'starting') return t('shell.forwards.stateStarting')
  if (state === 'error') return status?.error || t('shell.forwards.stateError')
  if (state === 'disconnected') return t('shell.forwards.stateDisconnected')
  return t('shell.forwards.stateStopped')
}

function stateTone(id: string): string {
  const state = stateOf(id)
  if (state === 'running') return 'text-qt-success'
  if (state === 'error') return 'text-qt-danger'
  if (state === 'starting' || state === 'reconnecting') return 'text-qt-warning'
  return 'text-qt-muted'
}

function stateDot(id: string): string {
  const state = stateOf(id)
  if (state === 'running') return 'bg-qt-success'
  if (state === 'error') return 'bg-qt-danger'
  if (state === 'starting' || state === 'reconnecting') return 'bg-qt-warning'
  return 'bg-qt-muted'
}

function openCreate(): void {
  editing.value = null
  editorOpen.value = true
}

function openEdit(row: PortForwardConfig): void {
  editing.value = row
  editorOpen.value = true
}

function handleCreateGroup(name: string): void {
  emit('update:forwardGroups', mergeForwardGroups(props.forwards, [...props.forwardGroups, name]))
}

async function handleSubmit(payload: PortForwardConfig): Promise<void> {
  await actions.save(payload)
  await refreshStatuses()
}

async function handleToggle(row: PortForwardConfig, enabled: boolean): Promise<void> {
  busy.value = { ...busy.value, [row.id]: true }
  try {
    statuses.value = { ...statuses.value, [row.id]: await actions.setEnabled(row, enabled) }
  } catch {
    await refreshStatuses()
  } finally {
    busy.value = { ...busy.value, [row.id]: false }
  }
}

async function handleRemove(row: PortForwardConfig): Promise<void> {
  busy.value = { ...busy.value, [row.id]: true }
  try {
    await actions.remove(row)
  } finally {
    const next = { ...busy.value }
    delete next[row.id]
    busy.value = next
  }
}

let stopStatusEvents: (() => void) | undefined

function applyStatus(detail: PortForwardStatus | undefined | null): void {
  if (!detail?.id) return
  const previous = statuses.value[detail.id]

  if (previous && previous.updatedAt > detail.updatedAt) return
  statuses.value = { ...statuses.value, [detail.id]: detail }
}

async function attachStatusEvents(): Promise<void> {
  if (stopStatusEvents) return
  try {
    const { Events } = await import('@wailsio/runtime')
    stopStatusEvents = Events.On(PORT_FORWARD_STATUS_EVENT, (event: { data?: unknown }) => {
      applyStatus(event?.data as PortForwardStatus | undefined)
    })
  } catch {
    stopStatusEvents = undefined
  }
}

async function refreshStatuses(): Promise<void> {
  const next: Record<string, PortForwardStatus> = {}

  await Promise.all(
    props.forwards.map(async (row) => {
      const status = await actions.gateway.status(row.id).catch(() => null)
      if (status) next[row.id] = status
    }),
  )
  statuses.value = next
}

let pollTimer: number | undefined

watch(
  () => props.forwards.map((row) => row.id).join('|'),
  () => void refreshStatuses(),
)

onMounted(() => {
  void attachStatusEvents()
  void refreshStatuses()

  pollTimer = window.setInterval(() => {
    if (document.visibilityState === 'visible') void refreshStatuses()
  }, 3000)
})

onBeforeUnmount(() => {
  stopStatusEvents?.()
  stopStatusEvents = undefined
  if (pollTimer !== undefined) window.clearInterval(pollTimer)
  pollTimer = undefined
})
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <div class="flex items-center gap-1.5 px-2 pt-2 pb-2">
      <QtermSearchField
        v-model="query"
        class="min-w-0 flex-1"
        v-bind="{ ariaLabel: t('shell.forwards.search') }"
        :placeholder="t('shell.forwards.search')"
        :clear-label="t('shell.forwards.clearSearch')"
      />
      <IconButton size="icon" :label="t('shell.forwards.add')" @click="openCreate">
        <Plus :size="16" aria-hidden="true" />
      </IconButton>
    </div>

    <div v-if="props.forwards.length === 0" class="flex-1">
      <QtermEmptyState
        :icon="Route"
        :title="t('shell.toolPanel.noTunnels')"
        :description="t('shell.forwards.emptyHint')"
      />
    </div>

    <div v-else-if="sections.length === 0" class="flex-1">
      <QtermEmptyState :icon="Route" :title="t('shell.forwards.noMatch')" />
    </div>

    <div v-else class="min-h-0 flex-1 overflow-y-auto px-2 pb-2">
      <Accordion v-model="openGroups" type="multiple" class="flex flex-col gap-2">
        <AccordionItem
          v-for="section in sections"
          :key="section.key"
          :value="section.key"
          class="forward-panel-group overflow-hidden rounded-[8px] border border-qt-border"
        >
          <AccordionTrigger class="w-full rounded-none border-0 px-2.5 py-2 text-sm hover:no-underline">
            <span class="flex min-w-0 items-center gap-1.5 text-qt-muted">
              <Folder v-if="section.name !== ungroupedLabel" :size="13" class="shrink-0" aria-hidden="true" />
              <span class="min-w-0 [overflow-wrap:anywhere] text-qt-text">{{ section.name }}</span>
              <small class="text-xs font-normal text-qt-muted">（{{ section.rows.length }}）</small>
            </span>
          </AccordionTrigger>
          <AccordionContent class="pb-1">
            <ContextMenu v-for="row in section.rows" :key="row.id">
              <ContextMenuTrigger as-child>
                <article
                  class="mx-1 flex min-w-0 cursor-default items-start gap-2.5 rounded-md px-1.5 py-2 text-xs transition-colors [&:not(:first-child)]:mt-0.5 hover:bg-qt-hover focus-visible:bg-qt-hover focus-visible:outline-none"
                  tabindex="0"
                  :data-forward-row="row.id"
                  :data-forward-group="forwardGroupName(row)"
                  @dblclick="openEdit(row)"
                  @keydown.enter="openEdit(row)"
                >
                  <Route :size="15" class="mt-0.5 shrink-0 text-qt-muted" aria-hidden="true" />
                  <div class="flex min-w-0 flex-1 flex-col gap-1">
                    <div class="flex min-w-0 flex-wrap items-center gap-1.5">
                      <span class="min-w-0 [overflow-wrap:anywhere] text-sm font-medium text-qt-text">{{
                        forwardDisplayName(row)
                      }}</span>
                      <span class="shrink-0 rounded border border-qt-border px-1 text-[10px] text-qt-muted">{{
                        t(forwardKindLabelKey(row.kind))
                      }}</span>
                    </div>
                    <code class="block min-w-0 [overflow-wrap:anywhere] font-mono text-[11px] text-qt-muted">{{
                      forwardSummary(row)
                    }}</code>
                    <div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-0.5 text-[11px]">
                      <span class="flex min-w-0 items-center gap-1" :class="stateTone(row.id)">
                        <span class="size-1.5 shrink-0 rounded-full" :class="stateDot(row.id)" aria-hidden="true" />
                        <span class="min-w-0 [overflow-wrap:anywhere]">{{ statusLabel(row) }}</span>
                      </span>
                      <span class="min-w-0 [overflow-wrap:anywhere] text-qt-muted">{{
                        forwardHostLabel(row, props.hosts)
                      }}</span>
                    </div>
                  </div>
                </article>
              </ContextMenuTrigger>
              <ContextMenuContent class="min-w-44">
                <ContextMenuItem
                  v-if="isActive(row.id)"
                  :disabled="busy[row.id] === true"
                  :aria-label="t('shell.forwards.action.stop')"
                  @select="handleToggle(row, false)"
                >
                  <Square :size="14" aria-hidden="true" />{{ t('shell.forwards.action.stop') }}
                </ContextMenuItem>
                <ContextMenuItem
                  v-else
                  :disabled="busy[row.id] === true"
                  :aria-label="t('shell.forwards.action.start')"
                  @select="handleToggle(row, true)"
                >
                  <Play :size="14" aria-hidden="true" />{{ t('shell.forwards.action.start') }}
                </ContextMenuItem>
                <ContextMenuSeparator />
                <ContextMenuItem :aria-label="t('shell.forwards.edit')" @select="openEdit(row)">
                  <Pencil :size="14" aria-hidden="true" />{{ t('shell.forwards.edit') }}
                </ContextMenuItem>
                <ContextMenuItem
                  :disabled="busy[row.id] === true"
                  :aria-label="t('shell.forwards.remove')"
                  @select="handleRemove(row)"
                >
                  <Trash2 :size="14" aria-hidden="true" />{{ t('shell.forwards.remove') }}
                </ContextMenuItem>
              </ContextMenuContent>
            </ContextMenu>
          </AccordionContent>
        </AccordionItem>
      </Accordion>
    </div>

    <PortForwardEditorDialog
      :open="editorOpen"
      :config="editing"
      :hosts="sshHosts"
      :active-host-id="props.activeHostId"
      :groups="groupOptions"
      @update:open="(open) => (editorOpen = open)"
      @submit="handleSubmit"
      @create-group="handleCreateGroup"
    />
  </div>
</template>
