<script setup lang="ts">
import { Search, X } from '@lucide/vue'
import QtermButton from '../button/QtermButton.vue'
import QtermInput from '../form-controls/QtermInput.vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{
  modelValue: string
  placeholder: string
  searchAriaLabel: string
  clearAriaLabel: string
}>()

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
</script>

<template>
  <header v-bind="$attrs" class="settings-section-header flex-cb gap-3">
    <label class="snippet-panel-search relative flex min-w-0 flex-1 items-center text-qt-muted">
      <Search class="pointer-events-none absolute left-2.5 z-1" :size="16" aria-hidden="true" />
      <QtermInput
        :model-value="props.modelValue"
        class="qterm-reference-input bg-qt-panel pl-8 pr-9"
        type="search"
        :placeholder="props.placeholder"
        :aria-label="props.searchAriaLabel"
        autocomplete="off"
        @update:model-value="emit('update:modelValue', String($event))"
      />
      <QtermButton
        v-if="props.modelValue"
        variant="ghost"
        size="icon-xs"
        class="snippet-panel-search-clear absolute z-1 -translate-y-1/2 hover:border-qt-border"
        type="button"
        :aria-label="props.clearAriaLabel"
        @click="emit('update:modelValue', '')"
      >
        <X :size="16" aria-hidden="true" />
      </QtermButton>
    </label>
    <div v-if="$slots.default" class="flex shrink-0 items-center gap-2 [&_[data-slot=button]]:h-qt-control-height">
      <slot />
    </div>
  </header>
</template>
