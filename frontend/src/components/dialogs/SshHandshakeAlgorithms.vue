<script setup lang="ts">
import { computed } from 'vue'
import { X } from '@lucide/vue'
import { QtermButton } from '../common'
import { QtermCombobox, type QtermComboboxOption } from '../common/form-controls'
import { Label } from '../common/control-primitives'
import { useI18n } from '../../locales'

const props = withDefaults(
  defineProps<{
    id?: string
    label: string
    help: string
    modelValue: string[]

    supported: string[]

    deprecated: string[]
    deprecatedHint?: string
    deprecatedTag?: string
    addLabel?: string
    resetLabel?: string
    disabled?: boolean
  }>(),
  {
    id: undefined,
    deprecatedHint: '',
    deprecatedTag: 'deprecated',
    addLabel: '',
    resetLabel: '',
    disabled: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string[]]
}>()

const { t } = useI18n()

const deprecatedSet = computed(() => new Set(props.deprecated))

const groups = computed(() => {
  const selected = new Set(props.modelValue)
  const remaining = (values: string[]) => values.filter((name) => !selected.has(name))
  const supported = remaining(props.supported).map((name) => ({ value: name, label: name }))
  const deprecated = remaining(props.deprecated).map((name) => ({
    value: name,

    label: `${name}（${props.deprecatedTag}）`,
  }))
  const result: Array<{ label?: string; options: QtermComboboxOption[] }> = []
  if (supported.length) result.push({ label: t('shell.hostEditor.sshHandshake.defaultGroup'), options: supported })
  if (deprecated.length) result.push({ label: t('shell.hostEditor.sshHandshake.deprecatedGroup'), options: deprecated })
  return result
})

const hasSelection = computed(() => props.modelValue.length > 0)
const hasDeprecatedSelection = computed(() => props.modelValue.some((name) => deprecatedSet.value.has(name)))

function add(value: string | number): void {
  const name = String(value)
  if (!name || props.modelValue.includes(name)) return
  emit('update:modelValue', [...props.modelValue, name])
}

function remove(name: string): void {
  emit(
    'update:modelValue',
    props.modelValue.filter((item) => item !== name),
  )
}
</script>

<template>
  <div class="form-field mt-3 min-w-0 grid gap-[6px] text-qt-muted">
    <Label :for="props.id">{{ props.label }}</Label>
    <small class="text-xs">{{ props.help }}</small>

    <div v-if="props.modelValue.length" class="flex flex-wrap items-center gap-1.5">
      <span
        v-for="name in props.modelValue"
        :key="name"
        class="inline-flex h-6 items-center gap-1 rounded-full border px-2 font-mono text-xs"
        :class="
          deprecatedSet.has(name)
            ? 'border-qt-warning/50 bg-qt-warning/10 text-qt-warning'
            : 'border-qt-control-border bg-qt-control-muted-bg text-qt-control-muted-text'
        "
      >
        {{ name }}
        <span v-if="deprecatedSet.has(name)" class="text-[10px] uppercase">{{ props.deprecatedTag }}</span>
        <button
          type="button"
          class="inline-flex size-3.5 items-center justify-center rounded-full hover:bg-qt-hover"
          :aria-label="t('shell.hostEditor.sshHandshake.removeAlgorithm', { name })"
          :disabled="props.disabled"
          @click="remove(name)"
        >
          <X :size="10" aria-hidden="true" />
        </button>
      </span>
    </div>
    <small v-else class="text-xs opacity-70">{{ t('shell.hostEditor.sshHandshake.usingDefaults') }}</small>

    <div class="flex flex-wrap items-center gap-2">
      <QtermCombobox
        :id="props.id"
        :model-value="''"
        :groups="groups"
        :placeholder="props.addLabel || t('shell.hostEditor.sshHandshake.addAlgorithm')"
        :aria-label="props.label"
        :disabled="props.disabled || groups.length === 0"
        menu-width="auto"
        class="min-w-[16rem] flex-1"
        @update:model-value="add"
      />
      <QtermButton
        v-if="hasSelection"
        type="button"
        variant="ghost"
        size="sm"
        :disabled="props.disabled"
        @click="emit('update:modelValue', [])"
      >
        {{ props.resetLabel || t('shell.hostEditor.sshHandshake.resetAlgorithms') }}
      </QtermButton>
    </div>

    <p v-if="hasDeprecatedSelection" class="text-xs text-qt-warning" role="status">
      {{ props.deprecatedHint || t('shell.hostEditor.sshHandshake.deprecatedHint') }}
    </p>
  </div>
</template>
