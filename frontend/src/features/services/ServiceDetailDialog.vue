<script setup lang="ts">
import { computed } from 'vue'
import { Play, Power, RefreshCw, RotateCw, Square } from '@lucide/vue'
import { QtermButton, QtermInfoList, type QtermInfoItem } from '@/components/common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '@/components/common/dialog'
import { useI18n } from '@/locales'
import type { ServiceActionName, ServiceDetail, ServiceInfo } from '@/services/sessionServices'

const props = withDefaults(
  defineProps<{
    open: boolean
    service: ServiceInfo | null
    detail: ServiceDetail | null
    loading?: boolean
    actionBusy?: boolean
    error?: string
  }>(),
  { loading: false, actionBusy: false, error: '' },
)

const emit = defineEmits<{
  close: []
  retry: []
  action: [value: ServiceActionName]
}>()

const { t } = useI18n()

const statusLabel = computed(() => {
  if (!props.service) return ''
  if (props.service.activeState === 'failed') return t('shell.services.stateFailed')
  if (props.service.activeState === 'active' || props.service.activeState === 'activating') {
    return t('shell.services.stateRunning')
  }
  return t('shell.services.stateStopped')
})

const actionDisabled = computed(() => props.actionBusy || !props.service || props.service.name.endsWith('@.service'))

const statusRows = computed(() => [
  { key: 'LoadState', label: t('shell.services.detailLoadState') },
  { key: 'ActiveState', label: t('shell.services.detailActiveState') },
  { key: 'UnitFileState', label: t('shell.services.detailStartup') },
  { key: 'MainPID', label: t('shell.services.detailMainPid') },
  { key: 'TasksCurrent', label: t('shell.services.detailTasks') },
  { key: 'NRestarts', label: t('shell.services.detailRestarts') },
  { key: 'ExecMainStatus', label: t('shell.services.detailExitStatus') },
  { key: 'ExecMainStartTimestamp', label: t('shell.services.detailStartTime') },
  { key: 'ExecMainExitTimestamp', label: t('shell.services.detailStopTime') },
  { key: 'MemoryCurrent', label: t('shell.services.detailMemory') },
  { key: 'FragmentPath', label: t('shell.services.detailUnitFile') },
])

function fieldValue(key: string): string {
  const value = props.detail?.fields?.[key]?.trim() ?? ''
  if (!value || value === '[not set]') return '-'
  if (key === 'MemoryCurrent') {
    const bytes = Number(value)
    if (Number.isFinite(bytes) && bytes >= 0) {
      if (bytes >= 1024 * 1024 * 1024) return (bytes / (1024 * 1024 * 1024)).toFixed(2) + ' GB'
      if (bytes >= 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(2) + ' MB'
      if (bytes >= 1024) return (bytes / 1024).toFixed(1) + ' KB'
      return bytes + ' B'
    }
  }
  return value
}

const MONO_FIELDS = new Set(['MainPID', 'FragmentPath'])
const detailItems = computed<QtermInfoItem[]>(() =>
  statusRows.value.map((row) => ({
    label: row.label,
    value: fieldValue(row.key),
    mono: MONO_FIELDS.has(row.key),
  })),
)

const detailColumns = computed(() => {
  const middle = Math.ceil(detailItems.value.length / 2)
  return [detailItems.value.slice(0, middle), detailItems.value.slice(middle)]
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
        :badge="statusLabel"
        @close="emit('close')"
      />

      <div class="flex w-full min-w-0 max-w-full min-h-0 flex-col gap-2 overflow-y-auto px-4 py-3">
        <div v-if="props.loading" class="py-8 text-center text-xs text-qt-muted">
          {{ t('shell.services.loadingDetail') }}
        </div>
        <div v-else-if="!props.detail" class="rounded-md border border-qt-danger/30 px-3 py-4 text-xs">
          <p class="m-0 text-qt-danger">{{ props.error || t('shell.services.detailFailed') }}</p>
          <QtermButton class="mt-3" size="sm" variant="outline" type="button" @click="emit('retry')">
            <RefreshCw :size="14" aria-hidden="true" />{{ t('shell.services.refresh') }}
          </QtermButton>
        </div>
        <template v-else>
          <p v-if="props.error" class="m-0 text-xs text-qt-danger">{{ props.error }}</p>
          <div class="grid min-w-0 grid-cols-1 gap-2 sm:grid-cols-2">
            <QtermInfoList v-for="(column, index) in detailColumns" :key="index" :items="column" />
          </div>
        </template>
      </div>

      <QtermDialogFooter class="flex flex-wrap items-center gap-2">
        <QtermButton
          v-if="props.service.activeState === 'active' || props.service.activeState === 'activating'"
          variant="outline"
          type="button"
          class="text-qt-danger"
          :disabled="actionDisabled"
          @click="emit('action', 'stop')"
        >
          <Square :size="14" aria-hidden="true" />{{ t('shell.services.stop') }}
        </QtermButton>
        <QtermButton v-else variant="outline" type="button" :disabled="actionDisabled" @click="emit('action', 'start')">
          <Play :size="14" aria-hidden="true" />{{ t('shell.services.start') }}
        </QtermButton>
        <QtermButton variant="outline" type="button" :disabled="actionDisabled" @click="emit('action', 'restart')">
          <RotateCw :size="14" aria-hidden="true" />{{ t('shell.services.restart') }}
        </QtermButton>
        <QtermButton
          variant="outline"
          type="button"
          :disabled="actionDisabled"
          @click="emit('action', props.service.enabledState === 'enabled' ? 'disable' : 'enable')"
        >
          <Power :size="14" aria-hidden="true" />
          {{ props.service.enabledState === 'enabled' ? t('shell.services.disable') : t('shell.services.enable') }}
        </QtermButton>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>
