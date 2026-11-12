<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { FileText, Info, Play, Power, RotateCw, Server, Square } from '@lucide/vue'
import { QtermEmptyState, QtermSearchField, RefreshIconButton, SegmentedTabs } from '@/components/common'
import { ScrollArea } from '@/components/common/control-primitives'
import QtermLoadMore from '@/components/common/layout/QtermLoadMore.vue'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from '@/components/common/overlay-controls'
import { useI18n } from '@/locales'
import { errorMessage } from '@/services/errors'
import {
  createSessionServiceGateway,
  type ServiceActionName,
  type ServiceDetail,
  type ServiceInfo,
  type ServiceLog,
} from '@/services/sessionServices'
import ServiceDetailDialog from './ServiceDetailDialog.vue'
import ServiceLogsDialog from './ServiceLogsDialog.vue'
import { useSnippetGroupPagination } from '../snippets/useSnippetGroupPagination'

const props = withDefaults(
  defineProps<{
    sessionId?: string
    available?: boolean
  }>(),
  { sessionId: '', available: false },
)

const emit = defineEmits<{
  managerChanged: [manager: string]
}>()

const { t } = useI18n()
const gateway = createSessionServiceGateway()

type Tab = 'all' | 'running' | 'stopped' | 'failed'
const tab = ref<Tab>('all')
const services = ref<ServiceInfo[]>([])
const loading = ref(false)
const error = ref('')
const query = ref('')
const busyUnit = ref('')
let refreshVersion = 0

const detailOpen = ref(false)
const detailService = ref<ServiceInfo | null>(null)
const detail = ref<ServiceDetail | null>(null)
const detailLoading = ref(false)
const detailError = ref('')
let detailVersion = 0

const logsOpen = ref(false)
const logsService = ref<ServiceInfo | null>(null)
const logs = ref<ServiceLog | null>(null)
const logsLoading = ref(false)
const logsError = ref('')
const logsLines = ref(200)
const logsAutoRefresh = ref(false)
const logsFollow = ref(false)
let logsVersion = 0
let logsRequestId = 0
let logsTimer: number | undefined

const activeDetailService = computed(() => {
  if (!detailService.value) return null
  return services.value.find((service) => unitKey(service) === unitKey(detailService.value!)) ?? detailService.value
})

const activeLogsService = computed(() => {
  if (!logsService.value) return null
  return services.value.find((service) => unitKey(service) === unitKey(logsService.value!)) ?? logsService.value
})

const tabs = computed<Array<{ value: Tab; label: string }>>(() => [
  { value: 'all', label: t('shell.services.tabAll') },
  { value: 'running', label: t('shell.services.tabRunning') },
  { value: 'stopped', label: t('shell.services.tabStopped') },
  { value: 'failed', label: t('shell.services.tabFailed') },
])

function stateOf(service: ServiceInfo): 'running' | 'stopped' | 'failed' {
  if (service.activeState === 'failed') return 'failed'
  if (service.activeState === 'active' || service.activeState === 'activating') return 'running'
  return 'stopped'
}

function unitKey(service: ServiceInfo): string {
  return service.scope === 'user' ? `user:${service.name}` : service.name
}

function isTemplateUnit(service: ServiceInfo): boolean {
  return service.name.endsWith('@.service')
}

function actionLabel(service: ServiceInfo, label: string): string {
  if (!isTemplateUnit(service)) return label
  return `${label} · ${t('shell.services.templateHint')}`
}

function matchesQuery(service: ServiceInfo): boolean {
  const keyword = query.value.trim().toLowerCase()
  if (!keyword) return true
  return service.name.toLowerCase().includes(keyword) || service.description.toLowerCase().includes(keyword)
}

function filtered(custom: boolean): ServiceInfo[] {
  return services.value.filter((service) => {
    if (service.custom !== custom) return false
    if (!matchesQuery(service)) return false
    if (tab.value === 'all') return true
    return stateOf(service) === tab.value
  })
}

const customServices = computed(() => filtered(true))
const systemServices = computed(() => filtered(false))

const sections = computed(() => [
  { key: 'custom', label: t('shell.services.custom'), rows: customServices.value },
  { key: 'system', label: t('shell.services.system'), rows: systemServices.value },
])

const { loadMore, reset, visibleCount, visibleItems } = useSnippetGroupPagination(50)

watch([query, tab], () => reset())

function selectTab(value: string): void {
  if (value === 'all' || value === 'running' || value === 'stopped' || value === 'failed') tab.value = value
}

function stateDotTone(service: ServiceInfo): string {
  const kind = stateOf(service)
  if (kind === 'running') return 'bg-qt-success'
  if (kind === 'failed') return 'bg-qt-danger'
  return 'bg-qt-muted'
}

function stateBadgeTone(service: ServiceInfo): string {
  const kind = stateOf(service)
  if (kind === 'running') return 'border-qt-success/40 bg-qt-success/10 text-qt-success'
  if (kind === 'failed') return 'border-qt-danger/40 bg-qt-danger/10 text-qt-danger'
  return 'border-qt-border bg-qt-hover text-qt-muted'
}

function enabledBadgeTone(service: ServiceInfo): string {
  return service.enabledState === 'enabled'
    ? 'border-qt-success/40 bg-qt-success/10 text-qt-success'
    : 'border-qt-border bg-qt-hover text-qt-muted'
}

function enabledLabel(service: ServiceInfo): string {
  if (service.enabledState === 'enabled') return t('shell.services.enabledStateEnabled')
  if (service.enabledState === 'disabled') return t('shell.services.enabledStateDisabled')
  return service.enabledState
}

function stateLabel(service: ServiceInfo): string {
  const kind = stateOf(service)
  if (kind === 'running') return t('shell.services.stateRunning')
  if (kind === 'failed') return t('shell.services.stateFailed')
  return t('shell.services.stateStopped')
}

function isRowActionTarget(target: EventTarget | null): boolean {
  return target instanceof Element && target.closest('button, a, input, select, textarea, [role="menuitem"]') !== null
}

function openDetailFromPointer(event: MouseEvent, service: ServiceInfo): void {
  if (isRowActionTarget(event.target)) return
  openDetail(service)
}

function openDetailFromKeyboard(event: KeyboardEvent, service: ServiceInfo): void {
  if (event.target !== event.currentTarget) return
  event.preventDefault()
  openDetail(service)
}

async function refresh(): Promise<void> {
  if (!props.available || !props.sessionId || loading.value) {
    if (!props.available || !props.sessionId) emit('managerChanged', '')
    return
  }
  const version = ++refreshVersion
  const sessionId = props.sessionId
  loading.value = true
  error.value = ''
  try {
    const snapshot = await gateway.list(sessionId)
    if (version !== refreshVersion || sessionId !== props.sessionId) return
    services.value = snapshot.services
    emit('managerChanged', snapshot.manager)
  } catch (cause) {
    if (version !== refreshVersion) return
    error.value = errorMessage(cause, t('shell.services.failed'))
    emit('managerChanged', '')
  } finally {
    if (version === refreshVersion) loading.value = false
  }
}

async function runAction(service: ServiceInfo, action: ServiceActionName): Promise<boolean> {
  if (!props.sessionId || busyUnit.value || isTemplateUnit(service)) return false
  busyUnit.value = service.name
  error.value = ''
  try {
    await gateway.action(props.sessionId, unitKey(service), action)
    await refresh()
    return true
  } catch (cause) {
    error.value = errorMessage(cause, t('shell.services.actionFailed'))
    return false
  } finally {
    busyUnit.value = ''
  }
}

async function loadDetail(): Promise<void> {
  if (!props.sessionId || !activeDetailService.value) return
  const version = detailVersion
  const sessionId = props.sessionId
  const unit = unitKey(activeDetailService.value)
  detailLoading.value = true
  detailError.value = ''
  try {
    const next = await gateway.detail(sessionId, unit)
    if (version === detailVersion) detail.value = next
  } catch (cause) {
    if (version === detailVersion) detailError.value = errorMessage(cause, t('shell.services.detailFailed'))
  } finally {
    if (version === detailVersion) detailLoading.value = false
  }
}

function openDetail(service: ServiceInfo): void {
  if (!props.sessionId) return
  detailVersion += 1
  detailService.value = service
  detail.value = null
  detailLoading.value = false
  detailError.value = ''
  detailOpen.value = true
  void loadDetail()
}

function closeDetail(): void {
  detailVersion += 1
  detailOpen.value = false
  detailService.value = null
  detail.value = null
  detailLoading.value = false
  detailError.value = ''
}

async function runDetailAction(action: ServiceActionName): Promise<void> {
  const service = activeDetailService.value
  if (!service) return
  if (!(await runAction(service, action))) {
    detailError.value = error.value
    return
  }
  if (detailOpen.value) await loadDetail()
}

async function loadLogs(force = false): Promise<void> {
  if (!props.sessionId || !activeLogsService.value || (logsLoading.value && !force)) return
  const requestId = ++logsRequestId
  const version = logsVersion
  const sessionId = props.sessionId
  const unit = unitKey(activeLogsService.value)
  const lines = logsLines.value
  logsLoading.value = true
  logsError.value = ''
  try {
    const next = await gateway.logs(sessionId, unit, lines)
    if (version === logsVersion && requestId === logsRequestId) logs.value = next
  } catch (cause) {
    if (version === logsVersion && requestId === logsRequestId) {
      logsError.value = errorMessage(cause, t('shell.services.logsFailed'))
    }
  } finally {
    if (version === logsVersion && requestId === logsRequestId) logsLoading.value = false
  }
}

function stopLogsTimer(): void {
  if (logsTimer !== undefined) window.clearInterval(logsTimer)
  logsTimer = undefined
}

function syncLogsTimer(): void {
  stopLogsTimer()
  if (!logsOpen.value || (!logsAutoRefresh.value && !logsFollow.value)) return

  logsTimer = window.setInterval(() => {
    if (document.visibilityState === 'visible' && logsOpen.value) void loadLogs()
  }, 3000)
}

function openLogs(service: ServiceInfo): void {
  if (!props.sessionId) return
  logsVersion += 1
  logsRequestId += 1
  logsService.value = service
  logs.value = null
  logsLoading.value = false
  logsError.value = ''
  logsAutoRefresh.value = false
  logsFollow.value = false
  logsOpen.value = true
  void loadLogs()
}

function closeLogs(): void {
  logsVersion += 1
  logsRequestId += 1
  logsOpen.value = false
  logsAutoRefresh.value = false
  logsFollow.value = false
  logsService.value = null
  logs.value = null
  logsLoading.value = false
  logsError.value = ''
}

function setLogsLines(lines: number): void {
  logsLines.value = lines
  if (logsOpen.value) void loadLogs(true)
}

function setLogsAutoRefresh(enabled: boolean): void {
  logsAutoRefresh.value = enabled
}

function setLogsFollow(enabled: boolean): void {
  logsFollow.value = enabled

  if (enabled) logsAutoRefresh.value = true
}

let timer: number | undefined
onMounted(() => {
  void refresh()

  timer = window.setInterval(() => {
    if (document.visibilityState === 'visible') void refresh()
  }, 30_000)
})
onBeforeUnmount(() => {
  if (timer !== undefined) window.clearInterval(timer)
  timer = undefined
  stopLogsTimer()
})
watch(
  () => [props.sessionId, props.available],
  () => {
    refreshVersion += 1
    loading.value = false
    services.value = []
    reset()
    emit('managerChanged', '')
    closeDetail()
    closeLogs()
    void refresh()
  },
)
watch([logsOpen, logsAutoRefresh, logsFollow], syncLogsTimer)
</script>

<template>
  <div class="flex h-full min-h-0 flex-col overflow-hidden">
    <div class="services-tabs w-full shrink-0 border-b border-qt-border px-2 py-1.5">
      <SegmentedTabs
        :model-value="tab"
        :options="tabs"
        :label="t('shell.services.title')"
        @update:model-value="selectTab"
      />
    </div>

    <div class="flex min-w-0 items-center gap-1 px-2 py-2">
      <QtermSearchField
        v-model="query"
        class="min-w-0 flex-1"
        v-bind="{ ariaLabel: t('shell.services.search') }"
        :placeholder="t('shell.services.search')"
        :clear-label="t('shell.toolPanel.clearSearch')"
      />

      <RefreshIconButton
        class="shrink-0"
        :label="t('shell.services.refresh')"
        :spinning="loading || !!busyUnit"
        :disabled="loading || !!busyUnit"
        @click="refresh"
      />
    </div>

    <div v-if="!props.available" class="flex-1">
      <QtermEmptyState
        :icon="Server"
        :title="t('shell.monitor.noSession')"
        :description="t('shell.services.noSessionHint')"
      />
    </div>

    <ScrollArea
      v-else
      class="min-h-0 min-w-0 w-full max-w-full flex-1 overflow-hidden"
      type="hover"
      :scroll-hide-delay="600"
    >
      <p v-if="error" class="px-2 pb-2 text-xs text-qt-danger">{{ error }}</p>

      <section
        v-for="section in sections"
        :key="section.key"
        :data-service-group="section.key"
        class="mx-2 mt-2 overflow-hidden rounded-[8px] border border-qt-border"
      >
        <header class="flex min-w-0 items-center gap-2 px-2.5 py-2">
          <span class="truncate text-sm font-medium text-qt-text">{{ section.label }}</span>
          <small class="shrink-0 rounded-full bg-qt-hover px-1.5 text-xs leading-4 text-qt-muted">{{
            section.rows.length
          }}</small>
        </header>

        <ContextMenu v-for="service in visibleItems(section.key, section.rows, true)" :key="unitKey(service)">
          <ContextMenuTrigger as-child>
            <article
              class="mx-1 flex min-w-0 cursor-pointer items-start gap-2 rounded-md px-1.5 py-2 transition-colors [&:not(:first-child)]:mt-0.5 hover:bg-qt-hover focus-visible:bg-qt-hover focus-visible:outline-none"
              tabindex="0"
              @dblclick="openDetailFromPointer($event, service)"
              @keydown.enter="openDetailFromKeyboard($event, service)"
            >
              <span class="mt-1 size-2 shrink-0 rounded-full" :class="stateDotTone(service)" aria-hidden="true" />
              <div class="flex min-w-0 flex-1 flex-col gap-0.5">
                <span
                  v-if="service.scope === 'user'"
                  class="mr-1 flex-none rounded bg-qt-hover px-1 py-0.5 text-[10px] uppercase text-qt-muted"
                  data-service-scope="user"
                  >user</span
                >
                <span class="block min-w-0 truncate text-xs text-qt-text" :title="service.name">{{
                  service.name
                }}</span>
                <span
                  v-if="service.description"
                  class="block min-w-0 truncate text-xs text-qt-muted"
                  :title="service.description"
                >
                  {{ service.description }}
                </span>

                <span class="mt-0.5 flex min-w-0 flex-wrap items-center gap-1">
                  <span class="shrink-0 rounded-full border px-1.5 text-xs leading-4" :class="stateBadgeTone(service)">
                    {{ stateLabel(service) }}
                  </span>
                  <span
                    v-if="service.enabledState"
                    class="shrink-0 rounded-full border px-1.5 text-xs leading-4"
                    :class="enabledBadgeTone(service)"
                  >
                    {{ enabledLabel(service) }}
                  </span>
                </span>
              </div>
            </article>
          </ContextMenuTrigger>

          <ContextMenuContent class="min-w-40">
            <ContextMenuItem
              :disabled="isTemplateUnit(service) || stateOf(service) === 'running'"
              :aria-label="actionLabel(service, t('shell.services.start'))"
              :title="isTemplateUnit(service) ? t('shell.services.templateHint') : undefined"
              @select="runAction(service, 'start')"
            >
              <Play :size="14" aria-hidden="true" />{{ t('shell.services.start') }}
            </ContextMenuItem>
            <ContextMenuItem
              :disabled="isTemplateUnit(service) || stateOf(service) !== 'running'"
              :aria-label="actionLabel(service, t('shell.services.stop'))"
              :title="isTemplateUnit(service) ? t('shell.services.templateHint') : undefined"
              @select="runAction(service, 'stop')"
            >
              <Square :size="14" aria-hidden="true" />{{ t('shell.services.stop') }}
            </ContextMenuItem>
            <ContextMenuItem
              :disabled="isTemplateUnit(service)"
              :aria-label="actionLabel(service, t('shell.services.restart'))"
              :title="isTemplateUnit(service) ? t('shell.services.templateHint') : undefined"
              @select="runAction(service, 'restart')"
            >
              <RotateCw :size="14" aria-hidden="true" />{{ t('shell.services.restart') }}
            </ContextMenuItem>
            <ContextMenuSeparator />
            <ContextMenuItem
              v-if="service.enabledState === 'enabled'"
              :disabled="isTemplateUnit(service)"
              :aria-label="actionLabel(service, t('shell.services.disable'))"
              :title="isTemplateUnit(service) ? t('shell.services.templateHint') : undefined"
              @select="runAction(service, 'disable')"
            >
              <Power :size="14" aria-hidden="true" />{{ t('shell.services.disable') }}
            </ContextMenuItem>
            <ContextMenuItem
              v-else
              :disabled="isTemplateUnit(service)"
              :aria-label="actionLabel(service, t('shell.services.enable'))"
              :title="isTemplateUnit(service) ? t('shell.services.templateHint') : undefined"
              @select="runAction(service, 'enable')"
            >
              <Power :size="14" aria-hidden="true" />{{ t('shell.services.enable') }}
            </ContextMenuItem>
            <ContextMenuSeparator />
            <ContextMenuItem @select="openDetail(service)">
              <Info :size="14" aria-hidden="true" />{{ t('shell.services.details') }}
            </ContextMenuItem>
            <ContextMenuItem @select="openLogs(service)">
              <FileText :size="14" aria-hidden="true" />{{ t('shell.services.logs') }}
            </ContextMenuItem>
          </ContextMenuContent>
        </ContextMenu>

        <div v-if="section.rows.length === 0" class="px-2.5 pb-2 text-xs text-qt-muted">
          {{ t('shell.services.empty') }}
        </div>
        <QtermLoadMore
          v-else
          class="px-2.5 pb-1"
          :visible="visibleCount(section.key, section.rows.length, true)"
          :total="section.rows.length"
          :load-more-label="t('shell.common.loadMore')"
          @load-more="loadMore(section.key, section.rows.length)"
        />
      </section>
    </ScrollArea>

    <ServiceDetailDialog
      :open="detailOpen"
      :service="activeDetailService"
      :detail="detail"
      :loading="detailLoading"
      :action-busy="!!busyUnit"
      :error="detailError"
      @close="closeDetail"
      @retry="loadDetail"
      @action="runDetailAction"
    />

    <ServiceLogsDialog
      :open="logsOpen"
      :service="activeLogsService"
      :logs="logs"
      :loading="logsLoading"
      :lines="logsLines"
      :auto-refresh="logsAutoRefresh"
      :follow="logsFollow"
      :error="logsError"
      @close="closeLogs"
      @refresh="loadLogs(true)"
      @update:lines="setLogsLines"
      @update:auto-refresh="setLogsAutoRefresh"
      @update:follow="setLogsFollow"
    />
  </div>
</template>

<style scoped>
.services-tabs :deep(.qterm-segmented-tabs__list) {
  background: color-mix(in srgb, var(--qt-panel) 16%, transparent);
}

.services-tabs :deep(.qterm-segmented-tabs__trigger)[data-state='active'] {
  background: color-mix(in srgb, var(--qt-panel) 46%, transparent);
  border: 1px solid color-mix(in srgb, var(--qt-text) 18%, transparent);
}

.services-tabs :deep(.qterm-segmented-tabs),
.services-tabs :deep(.qterm-segmented-tabs__list) {
  width: 100%;
  min-width: 0;
}

.services-tabs :deep(.qterm-segmented-tabs__list) {
  justify-content: flex-start;
  overflow-x: auto;
  scrollbar-width: none;
}

.services-tabs :deep(.qterm-segmented-tabs__list::-webkit-scrollbar) {
  display: none;
}

.services-tabs :deep(.qterm-segmented-tabs__trigger) {
  flex: 1 0 auto;
  min-width: max-content;
}
</style>
