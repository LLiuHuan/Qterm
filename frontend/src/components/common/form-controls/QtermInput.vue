<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { Input } from '../../ui/input'

defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    modelValue?: string | number
    defaultValue?: string | number
    class?: HTMLAttributes['class']
  }>(),
  { modelValue: undefined, defaultValue: undefined, class: undefined },
)

const emit = defineEmits<{ 'update:modelValue': [value: string | number] }>()
</script>

<template>
  <Input
    v-bind="$attrs"
    class="qterm-input text-xs"
    :model-value="props.modelValue"
    :default-value="props.defaultValue"
    :class="props.class"
    @update:model-value="(value) => emit('update:modelValue', value)"
  />
</template>

<style scoped>
.qterm-input {
  min-height: var(--qt-control-height, 32px);
  border-radius: var(--qt-control-radius, 6px);
  font-family: inherit;
  color: var(--qt-control-text, var(--foreground));
  border-color: var(--qt-control-border, var(--border));
  background-color: var(--qt-control-input-bg, transparent);
}

.qterm-input:focus-visible {
  border-color: var(--qt-focus);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--qt-focus) 16%, transparent);
}
</style>
