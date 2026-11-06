<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { CollapsibleContent } from '@/components/ui/collapsible'
import { cn } from '@/lib/utils'

const props = defineProps<{ class?: HTMLAttributes['class'] }>()
</script>

<template>
  <CollapsibleContent :class="cn('mt-1 min-w-0 text-xs text-muted-foreground', props.class)">
    <slot />
  </CollapsibleContent>
</template>
