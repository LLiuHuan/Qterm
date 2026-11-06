<script setup lang="ts">
import type { ComboboxLabelProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { ComboboxLabel } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps<ComboboxLabelProps & { class?: HTMLAttributes['class'] }>()
</script>

<template>
  <ComboboxLabel data-slot="combobox-label" :class="cn('text-muted-foreground px-1.5 py-1 text-xs', props.class)">
    <slot />
  </ComboboxLabel>
</template>
