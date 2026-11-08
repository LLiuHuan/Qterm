<script setup lang="ts">
import type { Component, HTMLAttributes } from 'vue'
import { computed, ref, watch } from 'vue'
import { Check, ChevronDown } from '@lucide/vue'
import {
  Combobox,
  ComboboxAnchor,
  ComboboxEmpty,
  ComboboxGroup,
  ComboboxLabel,
  ComboboxInput,
  ComboboxItem,
  ComboboxItemIndicator,
  ComboboxList,
  ComboboxTrigger,
  ComboboxViewport,
} from '../../ui/combobox'
import { useI18n } from '../../../locales'

export interface QtermComboboxOption {
  value: string | number
  label: string
  disabled?: boolean

  icon?: Component

  searchText?: string

  style?: string
}

export interface QtermComboboxGroup {
  label?: string
  options: QtermComboboxOption[]
}

const props = withDefaults(
  defineProps<{
    modelValue: string | number | null | undefined

    options?: QtermComboboxOption[]
    groups?: QtermComboboxGroup[]
    placeholder?: string

    emptyText?: string

    align?: 'start' | 'center' | 'end'

    openOnFocus?: boolean
    disabled?: boolean
    ariaLabel?: string
    id?: string

    filter?: boolean

    search?: string

    open?: boolean

    menuWidth?: 'trigger' | 'auto'

    menuClass?: HTMLAttributes['class']
    class?: HTMLAttributes['class']
  }>(),
  {
    options: () => [],
    groups: () => [],
    placeholder: '',
    emptyText: '',
    disabled: false,
    ariaLabel: undefined,
    id: undefined,
    filter: true,
    search: undefined,
    open: undefined,
    menuWidth: 'trigger',
    menuClass: undefined,
    class: undefined,
    align: 'start',
    openOnFocus: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string | number]
  'update:search': [value: string]
  'update:open': [value: boolean]

  scroll: [event: Event]
}>()

const { t } = useI18n()

const surface = ref<HTMLElement | null>(null)

const focused = ref(false)

const menuOpen = ref(false)

const selectedLabel = computed(() => {
  const value = props.modelValue
  if (value === null || value === undefined || value === '') return ''
  const matched = allOptions.value.find((option) => option.value === value)
  return matched ? matched.label : String(value)
})

function setSearchText(text: string): void {
  if (internalSearch.value !== text) internalSearch.value = text
  emit('update:search', '')
}

function surfaceHasFocus(): boolean {
  const active = document.activeElement
  if (!active || active === document.body) return false
  if (surface.value?.contains(active)) return true
  return active instanceof HTMLElement && active.closest('[data-slot="combobox-content"]') !== null
}

function focusSurface(): void {
  if (props.disabled) return
  const input = surface.value?.querySelector('input')
  if (input && document.activeElement !== input) input.focus()
}

function onInputClick(event: MouseEvent): void {
  if (event.detail === 0) {
    event.preventDefault()
    event.stopPropagation()
  }
}

function onInputPointerDown(event: PointerEvent): void {
  if (event.clientX === 0 && event.clientY === 0) {
    event.preventDefault()
    event.stopPropagation()
  }
}

const renderedGroups = computed<QtermComboboxGroup[]>(() =>
  props.groups.length > 0 ? props.groups : props.options.length > 0 ? [{ options: props.options }] : [],
)
const allOptions = computed<QtermComboboxOption[]>(() => renderedGroups.value.flatMap((group) => group.options))

function labelOf(value: unknown): string {
  const matched = allOptions.value.find((option) => option.value === value)
  if (matched) return matched.label
  return value === null || value === undefined ? '' : String(value)
}

const internalSearch = ref('')
const searchModel = computed({
  get: () => (props.search === undefined ? internalSearch.value : props.search),
  set: (value: string | undefined) => {
    const text = value ?? ''
    if (props.search === undefined) internalSearch.value = text

    emit('update:search', text === labelOf(props.modelValue) ? '' : text)
  },
})

watch(focused, (value) => {
  if (!value && menuOpen.value) return
  setSearchText(value ? '' : selectedLabel.value)
})

watch(
  [() => props.modelValue, () => menuOpen.value],
  ([, open]) => {
    if (focused.value && !open && !surfaceHasFocus()) {
      focused.value = false
      return
    }
    if (!open) {
      if (focused.value) setSearchText('')
      else setSearchText(selectedLabel.value)
    } else if (!focused.value) {
      setSearchText(selectedLabel.value)
    }
  },
  { flush: 'post' },
)

watch(selectedLabel, (label) => {
  if (!focused.value) setSearchText(label)
})

setSearchText(selectedLabel.value)
</script>

<template>
  <Combobox
    class="flex-1"
    :model-value="props.modelValue ?? undefined"
    :open="props.open"
    :disabled="props.disabled"
    :ignore-filter="props.filter === false"
    :open-on-click="true"
    :open-on-focus="props.openOnFocus"
    :reset-search-term-on-blur="false"
    :reset-search-term-on-select="false"
    @update:model-value="(value) => emit('update:modelValue', value as string | number)"
    @update:open="
      (value) => {
        menuOpen = value
        emit('update:open', value)
      }
    "
  >
    <ComboboxAnchor as-child>
      <div
        ref="surface"
        class="qterm-combobox relative flex w-full min-w-0 items-center"
        :class="props.class"
        @click="focusSurface"
      >
        <ComboboxInput
          :id="props.id"
          v-model="searchModel"
          class="qterm-combobox-input bg-transparent! pr-7 text-xs text-qt-text! shadow-none!"
          :class="selectedLabel !== '' && !focused ? 'placeholder:text-qt-text!' : 'placeholder:text-qt-muted!'"
          :display-value="(value: unknown) => labelOf(value)"
          :placeholder="selectedLabel || props.placeholder"
          :aria-label="props.ariaLabel"
          :disabled="props.disabled"
          aria-haspopup="listbox"
          @pointerdown="onInputPointerDown"
          @click="onInputClick"
          @focus="focused = true"
          @blur="focused = false"
          autocomplete="off"
          autocorrect="off"
          autocapitalize="off"
          :spellcheck="false"
        />

        <ComboboxTrigger as-child>
          <button
            type="button"
            tabindex="-1"
            class="qterm-combobox-chevron pointer-events-none absolute inset-y-0 end-0 flex w-7 items-center justify-center text-qt-muted"
            :aria-label="props.ariaLabel"
            :disabled="props.disabled"
          >
            <ChevronDown :size="14" aria-hidden="true" />
          </button>
        </ComboboxTrigger>
      </div>
    </ComboboxAnchor>

    <ComboboxList
      :align="props.align"
      class="qterm-form-menu text-xs qterm-select-content--no-animation"
      :class="[props.menuWidth === 'trigger' ? 'qterm-select-content--trigger-width' : '', props.menuClass]"
    >
      <ComboboxEmpty class="text-xs">{{ props.emptyText || t('shell.common.noMatchingOption') }}</ComboboxEmpty>
      <ComboboxViewport @scroll="(event: Event) => emit('scroll', event)">
        <ComboboxGroup v-for="(group, index) in renderedGroups" :key="group.label ?? index">
          <ComboboxLabel v-if="group.label">{{ group.label }}</ComboboxLabel>
          <ComboboxItem
            v-for="option in group.options"
            :key="option.value"
            :value="option.value"
            :text-value="option.searchText ?? option.label"
            :disabled="option.disabled"
            class="qterm-combobox-item py-1 pr-7 pl-2 text-xs"
          >
            <component :is="option.icon" v-if="option.icon" :size="14" class="flex-none" aria-hidden="true" />
            <span class="truncate" :style="option.style">{{ option.label }}</span>
            <ComboboxItemIndicator class="ms-auto flex-none">
              <Check :size="14" aria-hidden="true" />
            </ComboboxItemIndicator>
          </ComboboxItem>
        </ComboboxGroup>
      </ComboboxViewport>
      <slot name="footer" />
    </ComboboxList>
  </Combobox>
</template>
