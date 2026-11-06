<script setup lang="ts">
import { MorphIcon, type IconInput, type MorphOptions, type SpringPreset } from 'morphicons/vue'

defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    icon: IconInput
    size?: string | number
    strokeWidth?: string | number
    absoluteStrokeWidth?: boolean
    color?: string
    label?: string
    spring?: SpringPreset | MorphOptions
    reducedMotion?: 'never' | 'user' | 'always'
  }>(),
  {
    size: 16,
    strokeWidth: 2,
    absoluteStrokeWidth: false,
    color: 'currentColor',
    label: undefined,
    spring: undefined,
    reducedMotion: 'user',
  },
)
</script>

<template>
  <MorphIcon
    v-bind="$attrs"
    :icon="props.icon"
    :size="props.size"
    :stroke-width="props.strokeWidth"
    :absolute-stroke-width="props.absoluteStrokeWidth"
    :color="props.color"
    :label="props.label"
    :spring="props.spring"
    :reduced-motion="props.reducedMotion"
  />
</template>
