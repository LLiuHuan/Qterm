<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { computed } from 'vue'
import { ChevronRightIcon, BrainIcon } from '@lucide/vue'
import { CollapsibleTrigger } from '@/components/ui/collapsible'
import { cn } from '@/lib/utils'
import { useReasoningContext } from './context'

const props = defineProps<{ class?: HTMLAttributes['class']; label: string; preview?: string }>()
const { duration } = useReasoningContext()

const previewText = computed(() => {
  const raw = (props.preview ?? '').replace(/\s+/g, ' ').trim()
  return raw === '' ? '' : raw
})
</script>

<template>
  <CollapsibleTrigger
    :class="
      cn(
        'group/reasoning flex max-w-full min-w-0 cursor-pointer items-center gap-1 rounded-md text-left text-xs text-muted-foreground outline-none transition-colors hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50',
        previewText ? 'w-full' : 'w-fit',
        props.class,
      )
    "
  >
    <BrainIcon class="size-3.5 shrink-0" aria-hidden="true" />
    <span class="truncate" :class="previewText ? 'shrink-0' : ''">{{ props.label }}</span>
    <span
      v-if="previewText"
      data-reasoning-preview
      class="min-w-0 flex-1 truncate text-muted-foreground/70 group-data-[state=open]/reasoning:hidden"
      >{{ previewText }}</span
    >
    <span v-if="duration !== undefined" class="shrink-0 text-muted-foreground/70">{{ duration }}s</span>
    <ChevronRightIcon
      class="size-3.5 shrink-0 transition-transform group-data-[state=open]/reasoning:rotate-90"
      aria-hidden="true"
    />
  </CollapsibleTrigger>
</template>
