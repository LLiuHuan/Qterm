<script setup lang="ts">
import type { SwitchRootEmits, SwitchRootProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { Switch } from '../../ui/switch'

defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<SwitchRootProps & { class?: HTMLAttributes['class']; size?: 'sm' | 'default' }>(),
  { class: undefined, size: 'default' },
)
const emit = defineEmits<SwitchRootEmits>()
</script>

<template>
  <Switch
    v-bind="{ ...$attrs, ...props }"
    class="qterm-switch"
    :class="props.class"
    :size="props.size"
    @update:model-value="(value) => emit('update:modelValue', value)"
  />
</template>

<style scoped>
.qterm-switch[data-state='checked'] {
  background-color: var(--qt-control-primary-bg, var(--primary));
  border-color: var(--qt-control-primary-bg, var(--primary));
}

.qterm-switch[data-state='unchecked'] {
  background-color: var(--qt-control-input-bg, transparent);
  border-color: var(--qt-control-border, var(--border));
}

.qterm-switch:focus-visible {
  border-color: var(--qt-control-focus, var(--ring));
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--qt-control-focus, var(--ring)) 20%, transparent);
}
</style>
