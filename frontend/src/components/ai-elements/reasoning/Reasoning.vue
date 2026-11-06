<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { computed, provide, toRef } from 'vue'
import { Collapsible } from '@/components/ui/collapsible'
import { cn } from '@/lib/utils'
import { ReasoningKey } from './context'

const props = withDefaults(
  defineProps<{
    class?: HTMLAttributes['class']
    isStreaming?: boolean
    open?: boolean
    defaultOpen?: boolean
    duration?: number
  }>(),
  { class: undefined, isStreaming: false, open: undefined, defaultOpen: true, duration: undefined },
)

const emit = defineEmits<{ 'update:open': [value: boolean] }>()

provide(ReasoningKey, {
  isStreaming: toRef(props, 'isStreaming'),
  duration: toRef(props, 'duration'),
})

const isOpen = computed({
  get: () => props.open ?? props.defaultOpen,
  set: (value: boolean) => emit('update:open', value),
})
</script>

<template>
  <Collapsible v-model:open="isOpen" :class="cn('flex w-full min-w-0 flex-col', props.class)">
    <slot />
  </Collapsible>
</template>
