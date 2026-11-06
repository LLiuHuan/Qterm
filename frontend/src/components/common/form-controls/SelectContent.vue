<script setup lang="ts">
import type { SelectContentProps, SelectContentEmits } from 'reka-ui'
import { useForwardPropsEmits } from 'reka-ui'
import { SelectContent } from '../../ui/select'
const props = withDefaults(defineProps<SelectContentProps & { onViewportScroll?: (event: Event) => void }>(), {
  position: 'popper',
  align: 'start',
  onViewportScroll: undefined,
})
const emits = defineEmits<SelectContentEmits>()
const forwarded = useForwardPropsEmits(props, emits)
</script>
<template>
  <SelectContent
    v-bind="forwarded"
    class="qterm-form-menu text-xs qterm-select-content--trigger-width qterm-select-content--no-animation"
  >
    <template v-if="$slots.header" #header><slot name="header" /></template>
    <slot />
  </SelectContent>
</template>
