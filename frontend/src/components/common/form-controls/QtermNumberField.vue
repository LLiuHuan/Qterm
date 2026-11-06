<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { computed } from 'vue'
import {
  NumberField,
  NumberFieldContent,
  NumberFieldDecrement,
  NumberFieldIncrement,
  NumberFieldInput,
} from '../../ui/number-field'
import { useI18n } from '../../../locales'

const props = withDefaults(
  defineProps<{
    modelValue?: number
    min?: number
    max?: number
    step?: number
    disabled?: boolean

    showControls?: boolean

    stepSnapping?: boolean

    integerOnly?: boolean
    id?: string
    name?: string
    ariaLabel?: string
    ariaLabelledby?: string

    decreaseLabel?: string

    increaseLabel?: string
    class?: HTMLAttributes['class']
  }>(),
  {
    modelValue: undefined,
    min: undefined,
    max: undefined,
    step: 1,
    disabled: false,
    showControls: true,
    stepSnapping: true,
    integerOnly: true,
    id: undefined,
    name: undefined,
    ariaLabel: undefined,
    ariaLabelledby: undefined,
    decreaseLabel: undefined,
    increaseLabel: undefined,
    class: undefined,
  },
)

const emit = defineEmits<{ 'update:modelValue': [value: number | undefined] }>()

const { t } = useI18n()

const decreaseLabel = computed(() => props.decreaseLabel ?? t('control.numberField.decrease'))
const increaseLabel = computed(() => props.increaseLabel ?? t('control.numberField.increase'))

const formatOptions = computed<Intl.NumberFormatOptions>(() => ({
  useGrouping: false,
  minimumFractionDigits: 0,
  maximumFractionDigits: props.integerOnly ? 0 : 3,
}))
</script>

<template>
  <NumberField
    :id="props.id"
    :name="props.name"
    :model-value="props.modelValue"
    :min="props.min"
    :max="props.max"
    :step="props.step"
    :step-snapping="props.stepSnapping"
    :disabled="props.disabled"
    :format-options="formatOptions"
    :class="['qterm-number-field', props.class]"
    @update:model-value="(value) => emit('update:modelValue', value ?? undefined)"
  >
    <NumberFieldContent class="qterm-number-field__content">
      <NumberFieldDecrement
        v-if="props.showControls"
        class="qterm-number-field__step qterm-number-field__step--decrement"
        :aria-label="decreaseLabel"
      />
      <NumberFieldInput
        class="qterm-number-field__input text-xs"
        :class="{ 'qterm-number-field__input--with-controls': props.showControls }"
        :aria-label="props.ariaLabel"
        :aria-labelledby="props.ariaLabelledby"
      />
      <NumberFieldIncrement
        v-if="props.showControls"
        class="qterm-number-field__step qterm-number-field__step--increment"
        :aria-label="increaseLabel"
      />
    </NumberFieldContent>
  </NumberField>
</template>

<style scoped>
.qterm-number-field {
  font-family: inherit;
}

.qterm-number-field__input {
  min-height: var(--qt-control-height, 32px);
  border-radius: var(--qt-control-radius, 6px);
  font-family: inherit;
}

.qterm-number-field__content .qterm-number-field__input--with-controls {
  padding-left: 26px;
  padding-right: 26px;
}

.qterm-number-field__step {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  padding: 0;
  border: 0;
  border-radius: var(--qt-control-radius, 6px);
  background: transparent;
  color: var(--qt-control-muted-text, var(--muted-foreground));
  cursor: pointer;
  transition:
    background-color 0.15s ease,
    color 0.15s ease;
}

.qterm-number-field__step--decrement {
  left: 2px;
}

.qterm-number-field__step--increment {
  right: 2px;
}

.qterm-number-field__step:hover:not(:disabled) {
  background: var(--qt-hover, var(--accent));
  color: var(--qt-control-text, var(--accent-foreground));
}

.qterm-number-field__step:disabled {
  cursor: not-allowed;
}
</style>
