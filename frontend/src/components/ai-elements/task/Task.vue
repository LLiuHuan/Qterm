<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { ChevronRightIcon } from '@lucide/vue'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { cn } from '@/lib/utils'

withDefaults(
  defineProps<{
    class?: HTMLAttributes['class']
    title?: string
    defaultOpen?: boolean
  }>(),
  { class: undefined, title: '', defaultOpen: true },
)
</script>

<template>
  <Collapsible :default-open="$props.defaultOpen" :class="cn('flex w-full min-w-0 flex-col', $props.class)">
    <CollapsibleTrigger
      class="group/task flex w-fit max-w-full min-w-0 cursor-pointer items-center gap-1 rounded-md text-left text-xs text-qt-muted outline-none transition-colors hover:text-qt-text focus-visible:ring-3 focus-visible:ring-ring/50"
    >
      <slot name="title"
        ><span class="truncate">{{ $props.title }}</span></slot
      >
      <ChevronRightIcon
        class="size-3.5 shrink-0 transition-transform group-data-[state=open]/task:rotate-90"
        aria-hidden="true"
      />
    </CollapsibleTrigger>
    <CollapsibleContent class="mt-1 min-w-0">
      <slot />
    </CollapsibleContent>
  </Collapsible>
</template>
