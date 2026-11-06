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
}>()

const VIEWBOX_WIDTH = 300
const VIEWBOX_HEIGHT = 84
const CHART_LEFT = 4
const CHART_RIGHT = VIEWBOX_WIDTH - 4
const CHART_TOP = 6
const CHART_BOTTOM = VIEWBOX_HEIGHT - 6

const maxValue = computed(() => {
  const values = props.series.flatMap((item) => item.values.map((value) => Math.max(0, value)))
  return Math.max(1, ...values)
})

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

const peakText = computed(() => props.peakLabel.replace('{value}', props.formatValue(maxValue.value)))
</script>

<template>
  <div class="grid gap-1">
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
