<script setup lang="ts">
import { ChevronRight, Folder } from '@lucide/vue'
import { AccordionTrigger } from '@/components/common/control-primitives'

const props = withDefaults(
  defineProps<{
    title: string

    count?: number

    folder?: boolean

    titleClass?: string
  }>(),
  { count: undefined, folder: true, titleClass: '' },
)
</script>

<template>
  <header class="settings-library-row group flex h-9 min-w-0 items-center gap-1 px-3">
    <AccordionTrigger
      class="flex min-w-0 flex-1 items-center justify-start gap-1.5 px-0 py-0 text-left text-sm font-medium hover:no-underline"
    >
      <template #icon><span class="hidden" /></template>
      <ChevronRight
        :size="14"
        class="shrink-0 text-qt-muted transition-transform group-aria-expanded/accordion-trigger:rotate-90"
        aria-hidden="true"
      />
      <Folder v-if="props.folder" :size="13" class="shrink-0 text-qt-muted" aria-hidden="true" />
      <strong class="min-w-0 truncate text-sm leading-5" :class="props.titleClass">{{ props.title }}</strong>
      <span v-if="props.count !== undefined" class="shrink-0 text-[11px] text-qt-muted">（{{ props.count }}）</span>
    </AccordionTrigger>

    <div
      class="flex shrink-0 items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100"
    >
      <slot name="actions" />
    </div>
  </header>
</template>

<style scoped>
.settings-library-row > :deep(h3) {
  flex: 1;
  min-width: 0;
}
</style>
