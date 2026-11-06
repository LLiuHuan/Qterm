<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { Search, X } from '@lucide/vue'
import { cn } from '@/lib/utils'
import QtermButton from '../button/QtermButton.vue'
import QtermInput from '../form-controls/QtermInput.vue'

const props = withDefaults(
  defineProps<{
    modelValue: string
    placeholder: string
    ariaLabel: string
    clearLabel?: string
    class?: HTMLAttributes['class']
  }>(),
  { clearLabel: undefined, class: undefined },
)

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
</script>

<template>
  <label :class="cn('relative flex min-w-0 items-center text-qt-muted', props.class)">
    <Search class="pointer-events-none absolute left-2.5 z-1 size-4" :size="16" aria-hidden="true" />
    <QtermInput
      :model-value="props.modelValue"
      class="qterm-reference-input bg-qt-panel pl-8 pr-9"
      type="search"
      :placeholder="props.placeholder"
      :aria-label="props.ariaLabel"
      autocomplete="off"
      @update:model-value="emit('update:modelValue', String($event))"
    />
    <QtermButton
      v-if="props.modelValue"
      variant="ghost"
      size="icon-xs"
      class="absolute right-1 top-1/2 z-1 -translate-y-1/2"
      type="button"
      :aria-label="props.clearLabel"
      @click="emit('update:modelValue', '')"
    >
      <X :size="16" aria-hidden="true" />
    </QtermButton>
  </label>
</template>
