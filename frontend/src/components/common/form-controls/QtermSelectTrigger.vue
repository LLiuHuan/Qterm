<script setup lang="ts">
import type { SelectTriggerProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { useForwardProps } from 'reka-ui'
import { SelectTrigger as UiSelectTrigger } from '../../ui/select'
import { reactiveOmit } from '@vueuse/core'

type QtermSelectTriggerProps = SelectTriggerProps & {
  class?: HTMLAttributes['class']
  size?: 'sm' | 'default'
}

const props = withDefaults(defineProps<QtermSelectTriggerProps>(), { size: 'default', class: undefined })
const delegatedProps = reactiveOmit(props, 'class', 'size')
const forwardedProps = useForwardProps(delegatedProps)

function onTriggerClick(event: MouseEvent): void {
  if (event.detail === 0 && event.clientX === 0 && event.clientY === 0) {
    event.preventDefault()
    event.stopPropagation()
  }
}

function onTriggerPointerDown(event: PointerEvent): void {
  if (event.clientX === 0 && event.clientY === 0) {
    event.preventDefault()
    event.stopPropagation()
  }
}
</script>

<template>
  <UiSelectTrigger
    data-slot="select-trigger"
    :data-size="size"
    @pointerdown.capture="onTriggerPointerDown"
    @click.capture="onTriggerClick"
    v-bind="forwardedProps"
    class="qterm-select-trigger text-xs"
    :class="props.class"
  >
    <slot />
  </UiSelectTrigger>
</template>

<style scoped>
.qterm-select-trigger {
  min-height: var(--qt-control-height, 32px);
  height: var(--qt-control-height, 32px);
  gap: var(--qt-control-gap, 4px);
  border-radius: var(--qt-control-radius, 6px);
  border-color: var(--qt-control-border, var(--border));
  background: var(--qt-control-input-bg, transparent);
  color: var(--qt-control-text, var(--foreground));
}

.qterm-select-trigger[data-size='sm'] {
  min-height: var(--qt-button-height-sm, 28px);
  height: var(--qt-button-height-sm, 28px);
}

.qterm-select-trigger:focus-visible {
  border-color: var(--qt-control-focus, var(--ring));
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--qt-control-focus, var(--ring)) 25%, transparent);
}
</style>
