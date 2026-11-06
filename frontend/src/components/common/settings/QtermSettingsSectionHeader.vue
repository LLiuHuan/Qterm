<script setup lang="ts">
import type { HTMLAttributes } from 'vue'

defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    label?: string

    description?: string
    class?: HTMLAttributes['class']
  }>(),
  { label: undefined, description: undefined, class: undefined },
)
</script>

<template>
  <header v-bind="$attrs" class="settings-section-header" :class="props.class">
    <div v-if="props.label || props.description || $slots.label || $slots.description">
      <h3 v-if="props.label || $slots.label" class="m-0 text-sm">
        <slot name="label">{{ props.label }}</slot>
      </h3>
      <p v-if="props.description || $slots.description" class="mt-[3px] mb-0 text-xs">
        <slot name="description">{{ props.description }}</slot>
      </p>
    </div>
    <slot v-else />
    <slot name="actions" />
  </header>
</template>
