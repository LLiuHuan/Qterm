<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'
import { Separator } from '@/components/ui/separator'

const props = defineProps<{
  class?: HTMLAttributes['class']
}>()
</script>

<template>
  <div data-slot="dialog-header" :class="cn('gap-2 flex flex-col', props.class)">
    <slot />
    <Separator class="dialog-header-separator" />
  </div>
</template>
