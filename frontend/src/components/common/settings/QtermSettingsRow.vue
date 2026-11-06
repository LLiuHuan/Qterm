<script setup lang="ts">
import { computed } from 'vue'
import type { HTMLAttributes } from 'vue'

defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    label?: string

    description?: string

    forId?: string

    as?: 'div' | 'label'

    layout?: 'inline' | 'stacked'
    class?: HTMLAttributes['class']
  }>(),
  {
    label: undefined,
    description: undefined,
    forId: undefined,
    as: 'div',
    layout: 'inline',
    class: undefined,
  },
)

const isStacked = computed(() => props.layout === 'stacked')
</script>

<template>
  <component
    :is="props.as"
    v-bind="$attrs"
    class="settings-card-row flex-cb gap-4 pt-3 pr-3.5 pb-3 pl-3.5 border-b border-b-qt-divider"
    :class="[props.class, isStacked ? 'flex-col' : undefined]"
    :for="props.as === 'label' ? props.forId : undefined"
  >
    <div
      v-if="props.label || props.description || $slots.label || $slots.description"
      :class="isStacked ? 'w-full' : undefined"
    >
      <label
        v-if="props.as !== 'label' && props.forId && (props.label || $slots.label)"
        :for="props.forId"
        class="block text-sm font-medium"
      >
        <slot name="label">{{ props.label }}</slot>
      </label>
      <strong v-else-if="props.label || $slots.label" class="text-sm">
        <slot name="label">{{ props.label }}</slot>
      </strong>
      <small v-if="props.description || $slots.description" class="text-xs">
        <slot name="description">{{ props.description }}</slot>
      </small>
    </div>
    <div v-if="isStacked" class="w-full">
      <slot name="control"><slot /></slot>
    </div>
    <template v-else>
      <slot name="control"><slot /></slot>
    </template>
  </component>
</template>
