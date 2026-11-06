<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RefreshCw } from '@lucide/vue'
import { QtermButton, QtermCheckbox } from '@/components/common'
import { QtermDialog, QtermDialogContent, QtermDialogHeader } from '@/components/common/dialog'
import { ScrollArea } from '@/components/common/control-primitives'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/common/form-controls'
import { useI18n } from '@/locales'
import type { ServiceInfo, ServiceLog } from '@/services/sessionServices'

const LOG_LINE_OPTIONS = [50, 100, 200, 500, 1000] as const

const props = withDefaults(
  defineProps<{
    open: boolean
    service: ServiceInfo | null
    logs: ServiceLog | null
    loading?: boolean
    lines: number
    autoRefresh: boolean
    follow: boolean
    error?: string
  }>(),
  { loading: false, error: '' },
)

const emit = defineEmits<{
  close: []
  refresh: []
  'update:lines': [value: number]
  'update:auto-refresh': [value: boolean]
  'update:follow': [value: boolean]
}>()

const { t } = useI18n()
const logsScrollArea = ref<{ viewport?: HTMLElement | { value: HTMLElement | null } } | null>(null)
let logsViewport: HTMLElement | null = null

function getViewport(): HTMLElement | null {
  const raw = logsScrollArea.value?.viewport
  if (!raw) return null
  if (raw instanceof HTMLElement) return raw
  return raw.value ?? null
}

function scrollLogsToBottom(): void {
  const viewport = getViewport()
  if (viewport) viewport.scrollTop = viewport.scrollHeight
}

function onLogsScroll(event: Event): void {
  const viewport = event.currentTarget as HTMLElement | null
  if (!viewport || !props.follow) return
  const distance = viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight
  if (distance > 24) emit('update:follow', false)
}

function unbindLogsViewport(): void {
  logsViewport?.removeEventListener('scroll', onLogsScroll)
  logsViewport = null
}

function bindLogsViewport(): void {
  unbindLogsViewport()
  const viewport = getViewport()
  if (!viewport) return
  logsViewport = viewport
  viewport.addEventListener('scroll', onLogsScroll, { passive: true })
}

function selectLines(value: string): void {
  const lines = Number(value)
  if (LOG_LINE_OPTIONS.includes(lines as (typeof LOG_LINE_OPTIONS)[number])) emit('update:lines', lines)
}

function toggleAutoRefresh(value: boolean | 'indeterminate'): void {
  emit('update:auto-refresh', value === true)
}

function toggleFollow(): void {
  emit('update:follow', !props.follow)
}

watch(
  () => props.logs?.text,
  () => {
    if (props.follow) void nextTick(() => scrollLogsToBottom())
  },
)

watch(
  () => props.follow,
  (enabled) => {
    if (enabled && props.open) void nextTick(scrollLogsToBottom)
  },
)

watch(
  () => props.open,
  (open) => {
    if (!open) {
      unbindLogsViewport()
      return
    }
    void nextTick(() => {
      bindLogsViewport()
      if (props.follow) scrollLogsToBottom()
    })
  },
)

onMounted(() => {
  if (props.open) void nextTick(() => bindLogsViewport())
})

onBeforeUnmount(() => {
  unbindLogsViewport()
})
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => !open && emit('close')">
    <QtermDialogContent
      v-if="props.open && props.service"
      size="3xl"
      close-on-escape
      class="min-h-0 max-h-[calc(100vh-2rem)]"
    >
      <QtermDialogHeader
        :title="props.service.name"
        :description="props.service.description || undefined"
        :badge="t('shell.services.logs')"
        @close="emit('close')"
      />

      <div class="flex w-full min-w-0 max-w-full min-h-0 flex-col gap-2 overflow-y-auto px-4 py-3">
        <div class="flex flex-wrap items-center gap-2 text-xs">
          <label class="inline-flex items-center gap-1.5 text-qt-muted">
            <span>{{ t('shell.services.logLines') }}</span>
            <Select :model-value="String(props.lines)" @update:model-value="(value) => selectLines(String(value))">
              <SelectTrigger size="sm" class="w-20" :aria-label="t('shell.services.logLines')"
                ><SelectValue
              /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="option in LOG_LINE_OPTIONS" :key="option" :value="String(option)">{{
                  option
                }}</SelectItem>
              </SelectContent>
            </Select>
          </label>
          <label class="inline-flex items-center gap-1.5 text-qt-muted">
            <QtermCheckbox
              :model-value="props.autoRefresh"
              :aria-label="t('shell.services.autoRefresh')"
              @update:model-value="toggleAutoRefresh"
            />
            <span>{{ t('shell.services.autoRefresh') }}</span>
          </label>
          <QtermButton
            size="icon-sm"
            variant="ghost"
            type="button"
            :aria-label="t('shell.services.refresh')"
            :disabled="props.loading"
            @click="emit('refresh')"
          >
            <RefreshCw :size="14" aria-hidden="true" />
          </QtermButton>
          <QtermButton
            class="ml-auto"
            size="sm"
            :variant="props.follow ? 'default' : 'outline'"
            type="button"
            @click="toggleFollow"
          >
            {{ props.follow ? t('shell.services.stopFollowing') : t('shell.services.followLogs') }}
          </QtermButton>
        </div>

        <p v-if="props.error && props.logs" class="m-0 text-xs text-qt-danger">{{ props.error }}</p>

        <ScrollArea
          ref="logsScrollArea"
          class="logs-scroll h-[min(55vh,480px)] min-h-48 w-full min-w-0 max-w-full rounded-md border border-qt-border"
          type="hover"
          :scroll-hide-delay="600"
          @scroll="onLogsScroll"
        >
          <div
            v-if="props.loading && !props.logs"
            class="flex min-h-48 items-center justify-center p-4 text-xs text-qt-muted"
          >
            {{ t('shell.services.loading') }}
          </div>
          <div
            v-else-if="props.error && !props.logs"
            class="flex min-h-48 flex-col items-center justify-center gap-3 p-4 text-center text-xs"
          >
            <p class="m-0 text-qt-danger">{{ props.error }}</p>
            <QtermButton size="sm" variant="outline" type="button" @click="emit('refresh')">
              <RefreshCw :size="14" aria-hidden="true" />{{ t('shell.services.refresh') }}
            </QtermButton>
          </div>
          <div
            v-else-if="!props.logs?.text"
            class="flex min-h-48 items-center justify-center p-4 text-xs text-qt-muted"
          >
            {{ t('shell.services.emptyLogs') }}
          </div>
          <pre
            v-else
            class="m-0 min-h-48 w-full min-w-0 max-w-full whitespace-pre-wrap break-words [overflow-wrap:anywhere] p-3 font-mono text-[11px] leading-5 text-qt-text"
            >{{ props.logs.text }}</pre>
        </ScrollArea>

        <p v-if="props.logs?.truncated" class="m-0 text-[11px] text-qt-warning">
          {{ t('shell.services.truncated') }}
        </p>
      </div>
    </QtermDialogContent>
  </QtermDialog>
</template>

<style scoped>
.logs-scroll :deep([data-slot='scroll-area-viewport'] > div) {
  display: block !important;
  width: 100%;
  min-width: 0;
  max-width: 100%;
}
</style>
