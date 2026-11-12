<script setup lang="ts">
import type { ButtonVariants } from '../../ui/button'
import { onBeforeUnmount, ref, watch } from 'vue'
import { RefreshCw } from '@lucide/vue'
import IconButton from './IconButton.vue'

const MIN_SPIN_MS = 900

const props = withDefaults(
  defineProps<{
    label: string

    spinning?: boolean

    mode?: 'status' | 'action'
    size?: ButtonVariants['size']
    disabled?: boolean

    iconClass?: string
  }>(),
  {
    spinning: false,
    mode: 'status',
    size: 'icon-sm',
    disabled: false,
    iconClass: 'size-3.5',
  },
)

const rendering = ref(false)
let startedAt = 0
let stopTimer: ReturnType<typeof setTimeout> | undefined
let actionTimer: ReturnType<typeof setTimeout> | undefined

function startSpin(): void {
  startedAt = Date.now()
  rendering.value = true
}

function settle(): void {
  const remaining = MIN_SPIN_MS - (Date.now() - startedAt)
  if (remaining <= 0) {
    rendering.value = false
    return
  }
  if (stopTimer !== undefined) clearTimeout(stopTimer)
  stopTimer = setTimeout(() => {
    stopTimer = undefined
    rendering.value = false
  }, remaining)
}

watch(
  () => props.spinning,
  (spinning) => {
    if (stopTimer !== undefined) {
      clearTimeout(stopTimer)
      stopTimer = undefined
    }
    if (props.mode !== 'status') return
    if (spinning) startSpin()
    else settle()
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  if (stopTimer !== undefined) clearTimeout(stopTimer)
  if (actionTimer !== undefined) clearTimeout(actionTimer)
})

defineOptions({ inheritAttrs: false })

const emit = defineEmits<{ click: [event: MouseEvent] }>()

function handleClick(event: MouseEvent): void {
  if (props.mode === 'action' && !props.disabled) {
    rendering.value = true
    if (actionTimer !== undefined) clearTimeout(actionTimer)
    actionTimer = setTimeout(() => {
      actionTimer = undefined
      rendering.value = false
    }, MIN_SPIN_MS)
  }
  emit('click', event)
}
</script>

<template>
  <IconButton
    v-bind="$attrs"
    :label="props.label"
    :size="props.size"
    :disabled="props.disabled"
    :aria-busy="props.spinning || undefined"
    @click="handleClick"
  >
    <RefreshCw
      :class="[
        props.iconClass,
        rendering ? 'animate-[qterm-spin_900ms_linear_infinite] motion-reduce:animate-none' : '',
      ]"
      aria-hidden="true"
    />
  </IconButton>
</template>
