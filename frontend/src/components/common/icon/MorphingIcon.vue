<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, shallowRef, watch } from 'vue'
import { canonicalD, createMorph, restAttrs, type Morph } from 'morphicons/dom'
import type { IconInput, MorphOptions, SpringPreset } from 'morphicons/vue'
import { simplifyMorphPath } from './simplifyMorphPath'

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

const path = shallowRef<SVGPathElement>()

const initialD = canonicalD(props.icon)
const initialPaint = restAttrs(props.icon)
const svgStrokeWidth = computed(() =>
  props.absoluteStrokeWidth ? (Number(props.strokeWidth) * 24) / Number(props.size) : props.strokeWidth,
)
const gridStrokeWidth = computed(() => Number(svgStrokeWidth.value))
let morph: Morph | undefined

onMounted(() => {
  const element = path.value
  if (!element) return

  morph = createMorph(
    { setAttribute: (name, value) => element.setAttribute(name, name === 'd' ? simplifyMorphPath(value) : value) },
    props.icon,
    { strokeWidth: gridStrokeWidth.value, reducedMotion: props.reducedMotion },
  )
})
watch(
  () => props.icon,
  (icon) => {
    if (!morph) return

    morph.reducedMotion = props.reducedMotion
    morph.strokeWidth = gridStrokeWidth.value
    morph.morphTo(icon, props.spring)
  },
)
watch(gridStrokeWidth, (value) => {
  if (morph) morph.strokeWidth = value
})
watch(
  () => props.reducedMotion,
  (value) => {
    if (morph) morph.reducedMotion = value
  },
)
onBeforeUnmount(() => {
  morph?.destroy()
  morph = undefined
})
</script>

<template>
  <svg
    xmlns="http://www.w3.org/2000/svg"
    :width="props.size"
    :height="props.size"
    viewBox="0 0 24 24"
    fill="none"
    :stroke="props.color"
    :color="props.color"
    :stroke-width="svgStrokeWidth"
    stroke-linecap="round"
    stroke-linejoin="round"
    :role="props.label ? 'img' : undefined"
    :aria-hidden="props.label ? undefined : 'true'"
    v-bind="$attrs"
  >
    <title v-if="props.label">{{ props.label }}</title>
    <path ref="path" :d="initialD" v-bind="initialPaint" />
  </svg>
</template>
