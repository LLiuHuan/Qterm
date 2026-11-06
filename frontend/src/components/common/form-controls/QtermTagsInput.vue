<script setup lang="ts">
import { X } from '@lucide/vue'
import { useI18n } from '@/locales'
import { type AcceptableInputValue } from 'reka-ui'
import {
  TagsInput,
  TagsInputInput,
  TagsInputItem,
  TagsInputItemDelete,
  TagsInputItemText,
} from '@/components/ui/tags-input'

const props = withDefaults(
  defineProps<{
    modelValue?: string[]
    id?: string

    max?: number
    disabled?: boolean
    placeholder?: string

    ariaLabel?: string

    maxLength?: number
  }>(),
  {
    modelValue: () => [],

    id: undefined,
    placeholder: '',
    ariaLabel: '',
    max: 12,
    maxLength: 24,
    disabled: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string[]]
}>()

const { t } = useI18n()

function nextValue(value: AcceptableInputValue[]): void {
  const trimmed = value
    .map((item) => String(item).trim())
    .filter((item) => item !== '')
    .map((item) => (item.length > props.maxLength ? item.slice(0, props.maxLength) : item))
  emit('update:modelValue', trimmed)
}
</script>

<template>
  <TagsInput
    :id="props.id"
    :model-value="props.modelValue"
    :max="props.max"
    :disabled="props.disabled"
    :add-on-paste="true"
    :add-on-blur="true"
    :duplicate="false"
    :aria-label="props.ariaLabel || t('shell.hostEditor.tags')"
    class="tags-input-shell min-h-8 w-full rounded-(--qt-control-radius) border border-qt-control-border bg-(--qt-control-input-bg) px-2.5 py-1 text-xs text-qt-control-text"
    @update:model-value="nextValue"
  >
    <TagsInputItem
      v-for="tag in props.modelValue"
      :key="tag"
      :value="tag"
      class="h-5 bg-qt-control-muted-bg text-xs text-qt-control-muted-text"
    >
      <TagsInputItemText />
      <TagsInputItemDelete :aria-label="t('shell.hostEditor.tagsRemove', { tag })">
        <X :size="12" aria-hidden="true" />
      </TagsInputItemDelete>
    </TagsInputItem>

    <TagsInputInput
      :placeholder="props.modelValue.length === 0 ? props.placeholder : ''"
      :maxlength="props.maxLength"
      class="h-6 border-0 bg-transparent! p-0 text-xs text-qt-control-text shadow-none!"
    />
  </TagsInput>
</template>

<style scoped>
.tags-input-shell:focus-within {
  border-color: var(--qt-control-focus, var(--ring));
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--qt-control-focus, var(--ring)) 20%, transparent);
}

.tags-input-shell :deep(input) {
  font: inherit;
  color: inherit;
  background: transparent;
  border: 0;
  outline: none;
}

.tags-input-shell :deep(input)::placeholder {
  color: var(--qt-muted, var(--muted-foreground));
}
</style>
