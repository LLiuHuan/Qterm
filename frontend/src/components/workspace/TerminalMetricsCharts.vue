<script setup lang="ts">
import { computed } from 'vue'
import { Activity, Cpu, HardDrive, MemoryStick, Microchip, Network } from '@lucide/vue'
import ResourceSparkline from './ResourceSparkline.vue'
import TerminalMetricTooltip from './TerminalMetricTooltip.vue'
import type { ResourceSnapshot } from '../../services/sessions'
import { formatGpuMemory, type GpuMemorySummary } from '../../features/monitor/gpuMemory'
import { networkMetrics } from '../../features/monitor/networkMetrics'
import { formatPercent } from '../../lib/format'
import { useI18n } from '@/locales'

export interface MetricChartSample {
  resource: ResourceSnapshot
  gpu: GpuMemorySummary | null
}
const props = defineProps<{ samples: MetricChartSample[]; cpuValues?: number[] }>()
const { t } = useI18n()
const last = computed(() => props.samples.at(-1))
const finite = (value: number | null | undefined): value is number =>
  value != null && Number.isFinite(value) && value >= 0
function pressure(value: number | undefined, warning = 80, danger = 95): string {
  if (!finite(value)) return 'var(--qt-muted)'
  if (value >= danger) return 'var(--qt-danger)'
  return value >= warning ? 'var(--qt-warning)' : 'var(--qt-success)'
}
const sampled = (sample: MetricChartSample) => (sample.resource.sampleWindowMs ?? 0) > 0
const cpuSeries = computed(() => [
  {
    label: 'CPU',
    color: pressure(last.value?.resource.cpuUsagePercent),
    values:
      props.cpuValues ??
      props.samples
        .filter(sampled)
        .map((sample) => sample.resource.cpuUsagePercent)
        .filter(finite),
  },
])
const capacityItems = computed(() => {
  if (!last.value) return []
  const { resource, gpu } = last.value
  const disk = resource.disks.find((item) => item.mount === '/')
  const items = [
    {
      key: 'memory',
      label: 'Mem',
      icon: MemoryStick,
      value: resource.memoryUsagePercent,
      danger: 90,
      used: resource.memoryUsedText,
      total: resource.memoryTotalText,
    },
    {
      key: 'disk',
      label: 'Disk',
      icon: HardDrive,
      value: disk?.usedPercent,
      danger: 90,
      used: disk?.usedText ?? '—',
      total: disk?.totalText ?? '—',
    },
  ]
  if (gpu)
    items.push({
      key: 'gpu',
      label: 'GPU',
      icon: Microchip,
      value: gpu.usedPercent,
      danger: 95,
      used: formatGpuMemory(gpu.usedMb),
      total: gpu.totalMb > 0 ? formatGpuMemory(gpu.totalMb) : '—',
    })
  return items
})
function capacityValues(key: string): number[] {
  return props.samples
    .map((sample) => {
      if (key === 'memory') return sample.resource.memoryUsagePercent
      if (key === 'gpu') return sample.gpu?.usedPercent
      return sample.resource.disks.find((item) => item.mount === '/')?.usedPercent
    })
    .filter(finite)
}
const cpuSummary = computed(
  () =>
    `CPU ${last.value && sampled(last.value) && finite(last.value.resource.cpuUsagePercent) ? formatPercent(last.value.resource.cpuUsagePercent) : t('shell.toolPanel.collecting')}`,
)
const load = computed(() => last.value?.resource.loadAverage1)
const loadColor = computed(() => {
  const cores = last.value?.resource.cpuCores
  return pressure(finite(load.value) && finite(cores) && cores > 0 ? load.value / cores : undefined, 1, 1.5)
})
const network = computed(() => networkMetrics(last.value?.resource))
</script>

<template>
  <template v-if="last">
    <TerminalMetricTooltip :label="cpuSummary">
      <span
        class="terminal-metric-chart inline-flex h-4 shrink-0 items-center gap-1.5"
        :aria-label="cpuSummary"
        role="img"
      >
        <Cpu class="size-3.5 shrink-0" aria-hidden="true" /> CPU
        <ResourceSparkline
          compact
          :series="cpuSeries"
          :empty-text="t('shell.toolPanel.collecting')"
          :peak-label="t('shell.toolPanel.peakRate')"
          :format-value="formatPercent"
          aria-hidden="true"
        />
      </span>
    </TerminalMetricTooltip>
    <TerminalMetricTooltip
      v-for="item in capacityItems"
      :key="item.key"
      :label="`${item.label} ${finite(item.value) ? formatPercent(item.value) : '—'}`"
    >
      <span class="terminal-metric-chart inline-flex h-4 shrink-0 items-center gap-1.5" :data-metric="item.key">
        <span class="h-3.5 w-px bg-qt-border" aria-hidden="true" />
        <component :is="item.icon" class="size-3.5 shrink-0" aria-hidden="true" />
        <span>{{ item.label }}</span>
        <ResourceSparkline
          compact
          :series="[
            { label: item.label, color: pressure(item.value, 80, item.danger), values: capacityValues(item.key) },
          ]"
          :maximum="100"
          :empty-text="t('shell.toolPanel.collecting')"
          :peak-label="t('shell.toolPanel.peakRate')"
          :format-value="formatPercent"
          role="img"
          :aria-label="`${item.label} ${finite(item.value) ? formatPercent(item.value) : '—'}`"
        />
        <span class="terminal-metrics-detail text-qt-muted">{{ item.used }} / {{ item.total }}</span>
      </span>
    </TerminalMetricTooltip>
    <span class="terminal-metric-chart inline-flex h-4 shrink-0 items-center gap-1.5">
      <span class="h-3.5 w-px bg-qt-border" aria-hidden="true" />
      <Activity class="size-3.5 shrink-0" aria-hidden="true" /> Load
      <span :style="{ color: loadColor }">{{ finite(load) ? load.toFixed(2) : '—' }}</span>
    </span>
    <span
      v-if="last.resource.networks.length"
      class="terminal-metric-chart inline-flex h-4 shrink-0 items-center gap-1.5"
      data-metric="network"
    >
      <span class="h-3.5 w-px bg-qt-border" aria-hidden="true" />
      <Network class="size-3.5 shrink-0" aria-hidden="true" /> Net
      <span class="network-rx text-qt-success">↓ {{ sampled(last) ? network.receiveText : '—' }}</span>
      <span class="network-tx text-qt-info">↑ {{ sampled(last) ? network.transmitText : '—' }}</span>
    </span>
  </template>
</template>
