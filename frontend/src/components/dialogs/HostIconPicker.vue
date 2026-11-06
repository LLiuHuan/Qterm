<script setup lang="ts">
import { Image, Trash2, Upload } from '@lucide/vue'
import { computed, ref } from 'vue'
import { builtinHostIcons, hostIconPickerOptions } from '../../features/hosts/builtinIcons'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '../common/overlay-controls'
import { QtermButton } from '../common'
import DeleteConfirmDialog from './DeleteConfirmDialog.vue'
import { useI18n } from '@/locales'

const props = defineProps<{
  modelValue?: string
  label?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()
const { t } = useI18n()

const iconOptions = hostIconPickerOptions

const selectedIcon = computed(() => builtinHostIcons.find((item) => item.value === props.modelValue))
const removeIconOpen = ref(false)

function selectIcon(value: string): void {
  emit('update:modelValue', value)
}

function requestRemoveIcon(event: Event): void {
  event.preventDefault()
  removeIconOpen.value = true
}

function confirmRemoveIcon(): void {
  emit('update:modelValue', '')
  removeIconOpen.value = false
}
</script>

<template>
  <DropdownMenu>
    <DropdownMenuTrigger as-child>
      <QtermButton
        variant="outline"
        size="icon"
        type="button"
        class="host-icon-trigger"
        :class="{ 'is-empty': !props.modelValue }"
        :aria-label="props.label || t('shell.hostIcon.select')"
      >
        <img
          v-if="selectedIcon"
          :src="selectedIcon.src"
          :alt="t('shell.hostIcon.alt', { label: selectedIcon.label })"
          class="host-icon-image size-[24px] w-[22px] h-[22px] block flex-none object-contain"
          :class="{ 'qterm-theme-aware-logo': selectedIcon.themeAware }"
        />
        <slot v-else name="fallback"><Image :size="17" aria-hidden="true" /></slot>
      </QtermButton>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="start" class="host-icon-menu">
      <div
        class="host-icon-grid grid grid-cols-[repeat(8,_34px)] justify-center gap-[5px]"
        role="group"
        :aria-label="t('shell.hostIcon.options')"
      >
        <DropdownMenuItem
          v-for="option in iconOptions"
          :key="option.value"
          class="host-icon-option"
          :class="{ 'is-selected': props.modelValue === option.value }"
          :title="option.label"
          @select="selectIcon(option.value)"
        >
          <img
            :src="option.src"
            :alt="t('shell.hostIcon.alt', { label: option.label })"
            class="host-icon-image w-[22px] h-[22px] block flex-none object-contain"
            :class="{ 'qterm-theme-aware-logo': option.themeAware }"
          />
          <span class="sr-only">{{ option.label }}</span>
        </DropdownMenuItem>
        <DropdownMenuItem
          class="host-icon-action host-icon-action--danger"
          :disabled="!props.modelValue"
          :title="t('shell.hostIcon.remove')"
          @select="requestRemoveIcon"
        >
          <Trash2 :size="15" aria-hidden="true" />
          <span class="sr-only">{{ t('shell.hostIcon.remove') }}</span>
        </DropdownMenuItem>
        <DropdownMenuItem class="host-icon-action" disabled :title="t('shell.hostIcon.importDisabled')">
          <Upload :size="15" aria-hidden="true" />
          <span class="sr-only">{{ t('shell.hostIcon.import') }}</span>
        </DropdownMenuItem>
      </div>
    </DropdownMenuContent>
  </DropdownMenu>
  <DeleteConfirmDialog
    :open="removeIconOpen"
    :title="t('shell.hostIcon.removeTitle')"
    :message="t('shell.hostIcon.removeMessage')"
    :confirm-text="t('shell.hostIcon.removeConfirm')"
    @close="removeIconOpen = false"
    @confirm="confirmRemoveIcon"
  />
</template>
