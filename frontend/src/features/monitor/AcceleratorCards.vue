<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Activity, Cpu } from '@lucide/vue'
import { useI18n } from '@/locales'
import { createMonitorGateway, type AcceleratorInfo } from '@/services/monitor'
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
const devices = ref<AcceleratorInfo[]>([])
const error = ref('')
let timer: number | undefined
let disposed = false

const gpuDevices = computed(() => devices.value.filter((device) => device.kind === 'gpu'))
const npuDevices = computed(() => devices.value.filter((device) => device.kind === 'npu'))

function formatMemory(value: number): string {
  if (!value) return '0 MB'
  return value >= 1024 ? `${(value / 1024).toFixed(1)} GB` : `${Math.round(value)} MB`
}

function barWidth(value: number): string {
  const percent = Number.isFinite(value) ? Math.min(100, Math.max(0, value)) : 0
  return `${percent}%`
}

async function refresh(): Promise<void> {
  if (!props.available || !props.sessionId || disposed) {
    devices.value = []
    return
  }
  try {
    const snapshot = await gateway.accelerators(props.sessionId)
    if (disposed) return
    devices.value = snapshot.devices.filter((device: AcceleratorInfo) => device.kind === 'gpu' || device.kind === 'npu')
    error.value = ''
  } catch (cause) {
    if (disposed) return

    devices.value = []

    if (String(cause).includes('monitor.unsupported')) {
      error.value = ''
      return
    }
    error.value = errorMessage(cause, t('shell.monitor.failed'))
  }
}

function stopTimer(): void {
  if (timer !== undefined) window.clearInterval(timer)
  timer = undefined
}

function startTimer(): void {
  stopTimer()
  if (!props.available || !props.sessionId) return
  timer = window.setInterval(() => void refresh(), 15_000)
}

watch(
  () => [props.sessionId, props.available],
  () => {
    devices.value = []
    error.value = ''
    void refresh()
    startTimer()
  },
)

onMounted(() => {
  void refresh()
  startTimer()
})

onBeforeUnmount(() => {
  disposed = true
  stopTimer()
})
</script>

<template>
  <div v-if="gpuDevices.length" class="resource-card" data-accelerator-card="gpu">
    <div class="resource-card-title mb-[10px] flex items-center gap-[7px] font-normal">
      <Activity :size="15" />{{ t('shell.monitor.tabGpu') }}
    </div>
    <div v-if="error" class="text-xs text-qt-danger">{{ error }}</div>
    <div v-for="(device, index) in gpuDevices" :key="`gpu-${device.index}`" :class="index > 0 ? 'mt-[10px]' : ''">
      <div class="metric-row flex-cb min-h-[24px] gap-3 text-qt-muted">
        <span class="min-w-0 truncate text-xs">{{ device.name }}</span>
        <strong class="shrink-0 font-mono text-xs tabular-nums text-qt-accent">{{ device.utilization }}%</strong>
      </div>
      <div
        class="metric-track h-[5px] overflow-hidden rounded-[999px] [background:color-mix(in_srgb,_var(--qt-muted)_18%,_transparent)]"
      >
        <span class="block h-full rounded-[999px] bg-qt-accent" :style="{ width: barWidth(device.utilization) }" />
      </div>
      <div class="metric-row flex-cb mt-1 min-h-[22px] gap-3 text-[11px] text-qt-muted">
        <span class="min-w-0 truncate">
          {{ t('shell.monitor.memoryUsed') }}
          <span class="font-mono text-qt-text">{{ formatMemory(device.memoryUsedMb) }}</span>
          <template v-if="device.memoryTotalMb > 0"> / {{ formatMemory(device.memoryTotalMb) }}</template>
        </span>
        <span v-if="device.temperatureC > 0" class="shrink-0">
          <Cpu :size="12" aria-hidden="true" class="mr-0.5 inline" />{{ device.temperatureC }}°C
        </span>
      </div>
    </div>
  </div>

  <div v-if="npuDevices.length" class="resource-card" data-accelerator-card="npu">
    <div class="resource-card-title mb-[10px] flex items-center gap-[7px] font-normal">
      <Activity :size="15" />{{ t('shell.monitor.tabNpu') }}
    </div>
    <div v-if="error" class="text-xs text-qt-danger">{{ error }}</div>
    <div v-for="(device, index) in npuDevices" :key="`npu-${device.index}`" :class="index > 0 ? 'mt-[10px]' : ''">
      <div class="metric-row flex-cb min-h-[24px] gap-3 text-qt-muted">
        <span class="min-w-0 truncate text-xs">{{ device.name }}</span>
        <strong class="shrink-0 font-mono text-xs tabular-nums text-qt-accent">{{ device.utilization }}%</strong>
      </div>
      <div
        class="metric-track h-[5px] overflow-hidden rounded-[999px] [background:color-mix(in_srgb,_var(--qt-muted)_18%,_transparent)]"
      >
        <span class="block h-full rounded-[999px] bg-qt-accent" :style="{ width: barWidth(device.utilization) }" />
      </div>
      <div class="metric-row flex-cb mt-1 min-h-[22px] gap-3 text-[11px] text-qt-muted">
        <span class="min-w-0 truncate">
          {{ t('shell.monitor.memoryUsed') }}
          <span class="font-mono text-qt-text">{{ formatMemory(device.memoryUsedMb) }}</span>
          <template v-if="device.memoryTotalMb > 0"> / {{ formatMemory(device.memoryTotalMb) }}</template>
        </span>
        <span v-if="device.temperatureC > 0" class="shrink-0">
          <Cpu :size="12" aria-hidden="true" class="mr-0.5 inline" />{{ device.temperatureC }}°C
        </span>
      </div>
    </div>
  </div>
</template>
