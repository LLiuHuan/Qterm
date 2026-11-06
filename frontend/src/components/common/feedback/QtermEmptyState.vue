<script setup lang="ts">
import type { Component, HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

type EmptyStateVariant = 'plain' | 'outlined'

const VARIANT_CLASS: Record<EmptyStateVariant, string> = {
  plain: '',
  outlined: 'rounded-lg border border-dashed border-qt-border',
}

const props = withDefaults(
  defineProps<{
    icon?: Component

    iconClass?: string

    title?: string

    description?: string

    iconSize?: number

    role?: 'alert' | 'status'

    busy?: boolean

    variant?: EmptyStateVariant
    class?: HTMLAttributes['class']
  }>(),
  {
    icon: undefined,
    iconClass: undefined,
    title: '',
    description: '',
    iconSize: 28,
    role: undefined,
    busy: false,
    variant: 'plain',
    class: undefined,
  },
)
</script>

<template>
  <div
    class="qterm-empty-state min-h-[220px] flex flex-col items-center justify-center gap-[8px] text-qt-muted text-center"
    :class="cn(VARIANT_CLASS[props.variant], props.class)"
    :role="props.role"
    :aria-live="props.busy ? 'polite' : undefined"
    :aria-busy="props.busy ? 'true' : undefined"
  >
    <slot name="icon">
      <component
        :is="props.icon"
        v-if="props.icon"
        :size="props.iconSize"
        :class="props.iconClass"
        aria-hidden="true"
      />
    </slot>
    <strong v-if="props.title" class="text-sm">{{ props.title }}</strong>
    <span v-if="props.description" class="text-xs">{{ props.description }}</span>
    <slot />
  </div>
</template>
