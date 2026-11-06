<script setup lang="ts">
import type { CheckboxRootEmits, CheckboxRootProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { Checkbox } from '../../ui/checkbox'

defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<CheckboxRootProps & { class?: HTMLAttributes['class'] }>(), { class: undefined })
const emit = defineEmits<CheckboxRootEmits>()
</script>

<template>
  <Checkbox
    v-bind="{ ...$attrs, ...props }"
    class="qterm-checkbox"
    :class="props.class"
    @update:model-value="(value) => emit('update:modelValue', value)"
  >
    <slot />
  </Checkbox>
</template>

<style scoped>
.qterm-checkbox[data-state='checked'] {
  color: var(--qt-control-primary-text, var(--primary-foreground));
  background-color: var(--qt-control-primary-bg, var(--primary));
  border-color: var(--qt-control-primary-bg, var(--primary));
}

.qterm-checkbox:focus-visible {
  border-color: var(--qt-control-focus, var(--ring));
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--qt-control-focus, var(--ring)) 20%, transparent);
}
</style>
