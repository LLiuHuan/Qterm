<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { computed, ref } from 'vue'
import { Eye, EyeOff } from 'lucide'
import QtermButton from '../button/QtermButton.vue'
import MorphingIcon from '../icon/MorphingIcon.vue'
import QtermInput from './QtermInput.vue'
import { useI18n } from '../../../locales'

defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    id: string

    modelValue?: string

    name?: string
    placeholder?: string
    disabled?: boolean

    ariaLabel?: string

    autocomplete?: string

    autofocus?: boolean

    invalid?: boolean

    class?: HTMLAttributes['class']

    inputClass?: HTMLAttributes['class']

    showLabel?: string

    hideLabel?: string
  }>(),
  {
    modelValue: '',
    name: undefined,
    placeholder: undefined,
    disabled: false,
    ariaLabel: undefined,
    autocomplete: 'off',
    autofocus: false,
    class: undefined,
    inputClass: undefined,
    showLabel: undefined,
    hideLabel: undefined,
  },
)

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const { t } = useI18n()

const revealed = ref(false)

const showLabelText = computed(() => props.showLabel ?? t('shell.hostEditor.showPassword'))
const hideLabelText = computed(() => props.hideLabel ?? t('shell.hostEditor.hidePassword'))

const toggleLabel = computed(() => (revealed.value ? hideLabelText.value : showLabelText.value))

const inputLabel = computed(() => props.ariaLabel)

function toggleReveal(): void {
  revealed.value = !revealed.value
}
</script>

<template>
  <div v-bind="$attrs" class="password-input-group min-w-0 relative" :class="props.class">
    <QtermInput
      :id="props.id"
      :model-value="props.modelValue"
      :name="props.name"
      :type="revealed ? 'text' : 'password'"
      :placeholder="props.placeholder"
      :disabled="props.disabled"
      :aria-label="inputLabel"
      :aria-invalid="props.invalid ? 'true' : undefined"
      :class="props.inputClass"
      :autocomplete="props.autocomplete"
      autocorrect="off"
      autocapitalize="off"
      :spellcheck="false"
      :autofocus="props.autofocus || undefined"
      @update:model-value="(value) => emit('update:modelValue', String(value))"
    />

    <QtermButton
      variant="ghost"
      size="icon-sm"
      type="button"
      :disabled="props.disabled"
      :aria-label="toggleLabel"
      @click="toggleReveal"
    >
      <MorphingIcon :icon="revealed ? EyeOff : Eye" :size="15" reduced-motion="user" aria-hidden="true" />
    </QtermButton>
  </div>
</template>
