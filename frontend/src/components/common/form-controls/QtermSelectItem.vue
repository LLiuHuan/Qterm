<script setup lang="ts">
import type { SelectItemProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { reactiveOmit } from '@vueuse/core'
import { SelectItem as UiSelectItem } from '../../ui/select'
import { useForwardProps } from 'reka-ui'

type QtermSelectItemProps = SelectItemProps & { class?: HTMLAttributes['class'] }
const props = defineProps<QtermSelectItemProps>()
const delegatedProps = reactiveOmit(props, 'class')
const forwardedProps = useForwardProps(delegatedProps)
</script>

<template>
  <UiSelectItem data-slot="select-item" v-bind="forwardedProps" class="qterm-select-item text-xs" :class="props.class">
    <slot />
  </UiSelectItem>
</template>

<style scoped>
.qterm-select-item {
  min-height: 28px;
  gap: var(--qt-control-gap, 4px);
  border-radius: var(--qt-control-radius, 6px);
  padding: 4px 28px 4px 8px;
  color: var(--qt-control-text, var(--foreground));
}
</style>
