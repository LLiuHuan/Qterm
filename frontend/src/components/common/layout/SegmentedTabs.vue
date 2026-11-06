<script setup lang="ts">
import { Tabs, TabsList, TabsTrigger } from '../../ui/tabs'

interface SegmentedTabOption {
  value: string
  label: string
  icon?: string
  disabled?: boolean
  hint?: string

  count?: number
}

const props = withDefaults(
  defineProps<{
    modelValue: string
    options: SegmentedTabOption[]
    label: string
    id?: string
  }>(),
  { id: undefined },
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()
</script>

<template>
  <Tabs
    :id="props.id"
    :model-value="props.modelValue"
    class="qterm-segmented-tabs"
    :aria-label="props.label"
    @update:model-value="(value) => typeof value === 'string' && emit('update:modelValue', value)"
  >
    <TabsList class="qterm-segmented-tabs__list">
      <TabsTrigger
        v-for="option in props.options"
        :key="option.value"
        :value="option.value"
        :disabled="option.disabled"
        :title="option.hint"
        class="qterm-segmented-tabs__trigger"
      >
        <span v-if="option.icon" :class="option.icon" aria-hidden="true" />
        <span>{{ option.label }}</span>

        <span
          v-if="option.count !== undefined"
          data-tab-count
          class="shrink-0 rounded-full bg-qt-hover px-1.5 text-xs leading-4 text-qt-muted"
          >{{ option.count }}</span
        >
      </TabsTrigger>
    </TabsList>
  </Tabs>
</template>
