<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import TerminalColorPicker from '@/features/settings/TerminalColorPicker.vue'
import { TAG_COLOR_OPTIONS } from '@/lib/colorTag'

const props = defineProps<{
  modelValue?: string

  label: string

  noneLabel: string

  customLabel: string

  colorLabel: (color: string) => string
}>()

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const current = computed(() => props.modelValue ?? '')
const isCustomColor = (value: string): boolean => !!value && !TAG_COLOR_OPTIONS.includes(value as never)

const customColor = ref(isCustomColor(current.value) ? current.value : '')
watch(current, (value) => {
  if (isCustomColor(value)) customColor.value = value
})

function select(color: string): void {
  emit('update:modelValue', color)
}
</script>

<template>
  <div class="snippet-color-options flex items-center flex-wrap gap-[10px]" role="group" :aria-label="props.label">
    <button
      type="button"
      class="snippet-color-none"
      :aria-label="props.noneLabel"
      :aria-pressed="!current"
      @click="select('')"
    />
    <button
      v-for="option in TAG_COLOR_OPTIONS"
      :key="option"
      type="button"
      class="snippet-color-option"
      :style="{ backgroundColor: option }"
      :aria-label="props.colorLabel(option)"
      :aria-pressed="current === option"
      @click="select(option)"
    />
    <TerminalColorPicker
      :model-value="customColor"
      :selected="isCustomColor(current)"
      :label="props.customLabel"
      @select="select"
      @update:model-value="select"
    />
  </div>
</template>
