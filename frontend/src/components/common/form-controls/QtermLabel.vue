<script setup lang="ts">
import type { LabelProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { Label as UiLabel } from '../../ui/label'

const props = defineProps<LabelProps & { class?: HTMLAttributes['class'] }>()

const INTERACTIVE_TARGETS = '[data-slot="select-trigger"], .qterm-combobox'

function onClick(event: MouseEvent): void {
  const targetID = props.for
  if (!targetID || typeof document === 'undefined') return
  const target = document.getElementById(targetID)
  if (target?.matches(INTERACTIVE_TARGETS) || target?.closest(INTERACTIVE_TARGETS)) {
    event.preventDefault()
  }
}
</script>

<template>
  <UiLabel v-bind="props" :class="props.class" @click="onClick">
    <slot />
  </UiLabel>
</template>
