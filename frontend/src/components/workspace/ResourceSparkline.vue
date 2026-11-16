<script setup lang="ts">
import { computed } from 'vue'

export interface SparklineSeries {
  label: string

  color: string
  values: number[]
}

const props = defineProps<{
  series: SparklineSeries[]

  emptyText: string

  peakLabel: string

  formatValue: (value: number) => string

  caption?: string

  compact?: boolean

  maximum?: number
}>()

const VIEWBOX_WIDTH = 300
const VIEWBOX_HEIGHT = 84
const CHART_LEFT = 4
const CHART_RIGHT = VIEWBOX_WIDTH - 4
const CHART_TOP = 6
const CHART_BOTTOM = VIEWBOX_HEIGHT - 6

const peakValue = computed(() => {
  const values = props.series.flatMap((item) => item.values.filter((value) => Number.isFinite(value) && value >= 0))
  return Math.max(0, ...values)
})

const maxValue = computed(() => Math.max(props.maximum ?? 1, 1, peakValue.value))

const pointCount = computed(() => Math.max(...props.series.map((item) => item.values.length), 0))

const hasChart = computed(() => pointCount.value >= 2)

function toPolyline(values: number[]): string {
  const width = CHART_RIGHT - CHART_LEFT
  const height = CHART_BOTTOM - CHART_TOP
  const lastIndex = Math.max(values.length - 1, 1)
  return values
    .map((value, index) => {
      const x = CHART_LEFT + (index / lastIndex) * width
      const y = CHART_BOTTOM - (Math.max(0, value) / maxValue.value) * height
      return `${x.toFixed(2)},${y.toFixed(2)}`
    })
    .join(' ')
}

const peakText = computed(() => props.peakLabel.replace('{value}', props.formatValue(peakValue.value)))
</script>

<template>
  <span
    v-if="compact"
    class="inline-flex h-3.5 w-8 shrink-0 items-center justify-center"
    :title="!hasChart ? emptyText : undefined"
  >
    <svg
      v-if="hasChart"
      class="h-full w-full"
      :viewBox="`0 0 ${VIEWBOX_WIDTH} ${VIEWBOX_HEIGHT}`"
      preserveAspectRatio="none"
      role="img"
      :aria-label="series.map((item) => item.label).join(' / ')"
    >
      <polyline
        v-for="item in series"
        :key="item.label"
        :points="toPolyline(item.values)"
        fill="none"
        :stroke="item.color"
        stroke-width="1.5"
        stroke-linejoin="round"
        stroke-linecap="round"
        vector-effect="non-scaling-stroke"
      />
    </svg>
    <span v-else class="text-xs text-qt-muted" :aria-label="emptyText">—</span>
  </span>
  <div v-else class="grid gap-1">
    <div class="flex items-center justify-between gap-2 text-[0.625rem] text-qt-muted">
      <div class="flex items-center gap-2">
        <span v-for="item in series" :key="item.label" class="inline-flex items-center gap-1">
          <span class="size-1.5 rounded-full" :style="{ background: item.color }" />{{ item.label }}
        </span>
      </div>
      <span v-if="hasChart" class="font-mono tabular-nums">{{ peakText }}</span>
    </div>
    <div class="rounded-md border border-qt-border px-2 py-1.5">
      <div v-if="!hasChart" class="flex h-[76px] items-center justify-center text-[0.6875rem] text-qt-muted">
        {{ emptyText }}
      </div>
      <svg
        v-else
        class="h-[76px] w-full"
        :viewBox="`0 0 ${VIEWBOX_WIDTH} ${VIEWBOX_HEIGHT}`"
        preserveAspectRatio="none"
        role="img"
        :aria-label="series.map((item) => item.label).join(' / ')"
      >
        <polyline
          v-for="item in series"
          :key="item.label"
          :points="toPolyline(item.values)"
          fill="none"
          :stroke="item.color"
          stroke-width="1.5"
          stroke-linejoin="round"
          stroke-linecap="round"
          vector-effect="non-scaling-stroke"
        />
      </svg>
    </div>
    <p v-if="caption" class="text-right text-[0.625rem] text-qt-muted">{{ caption }}</p>
  </div>
</template>
