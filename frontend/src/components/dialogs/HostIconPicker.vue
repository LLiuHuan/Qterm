<script setup lang="ts">
import { Image, Trash2, Upload } from '@lucide/vue'
import { computed, onBeforeUnmount, ref } from 'vue'
import { builtinHostIcons, hostIconPickerOptions } from '../../features/hosts/builtinIcons'
import { customHostIconSrc, readCustomHostIcon } from '../../features/hosts/customHostIcon'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '../common/overlay-controls'
import { QtermButton } from '../common'
import { ScrollArea } from '../common/control-primitives'
import DeleteConfirmDialog from './DeleteConfirmDialog.vue'
import { useI18n } from '@/locales'
import { toast } from 'vue-sonner'

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
const customIconSrc = computed(() => customHostIconSrc(props.modelValue))
const removeIconOpen = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
let selectionVersion = 0
let pendingImportVersion = 0
let disposed = false

onBeforeUnmount(() => {
  disposed = true
  selectionVersion += 1
})

function selectIcon(value: string): void {
  selectionVersion += 1
  emit('update:modelValue', value)
}

function requestRemoveIcon(event: Event): void {
  event.preventDefault()
  selectionVersion += 1
  removeIconOpen.value = true
}

function confirmRemoveIcon(): void {
  selectionVersion += 1
  emit('update:modelValue', '')
  removeIconOpen.value = false
}

function requestImportIcon(event: Event): void {
  event.preventDefault()
  pendingImportVersion = ++selectionVersion
  fileInput.value?.click()
}

async function importIcon(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement | null
  const file = input?.files?.[0]
  if (!file) return
  const importVersion = pendingImportVersion
  try {
    const value = await readCustomHostIcon(file)
    if (!disposed && importVersion === selectionVersion) emit('update:modelValue', value)
  } catch {
    if (!disposed && importVersion === selectionVersion) toast.error(t('error.unknown'))
  } finally {
    if (input) input.value = ''
  }
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
        <img
          v-else-if="customIconSrc"
          :src="customIconSrc"
          :alt="t('shell.hostIcon.alt', { label: t('shell.hostIcon.custom') })"
          class="host-icon-image size-[24px] w-[22px] h-[22px] block flex-none object-contain"
        />
        <slot v-else name="fallback"><Image :size="17" aria-hidden="true" /></slot>
      </QtermButton>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="start" class="host-icon-menu">
      <ScrollArea type="hover" :scroll-hide-delay="600" class="host-icon-scroll">
        <div
          class="host-icon-grid grid grid-cols-[repeat(8,_34px)] justify-center gap-[5px] py-1.5"
          role="group"
          :aria-label="t('shell.hostIcon.options')"
        >
          <DropdownMenuItem
            v-for="option in iconOptions"
            :key="option.value"
            class="host-icon-option cursor-pointer!"
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
            class="host-icon-action host-icon-action--danger cursor-pointer! data-[disabled]:cursor-default!"
            :disabled="!props.modelValue"
            :title="t('shell.hostIcon.remove')"
            @select="requestRemoveIcon"
          >
            <Trash2 :size="15" aria-hidden="true" />
            <span class="sr-only">{{ t('shell.hostIcon.remove') }}</span>
          </DropdownMenuItem>
          <DropdownMenuItem
            class="host-icon-action cursor-pointer!"
            :title="t('shell.hostIcon.import')"
            @select="requestImportIcon"
          >
            <Upload :size="15" aria-hidden="true" />
            <span class="sr-only">{{ t('shell.hostIcon.import') }}</span>
          </DropdownMenuItem>
        </div>
      </ScrollArea>
    </DropdownMenuContent>
  </DropdownMenu>
  <input
    ref="fileInput"
    class="hidden"
    type="file"
    accept="image/png,image/jpeg,image/webp,image/gif"
    @change="importIcon"
  />
  <DeleteConfirmDialog
    :open="removeIconOpen"
    :title="t('shell.hostIcon.removeTitle')"
    :message="t('shell.hostIcon.removeMessage')"
    :confirm-text="t('shell.hostIcon.removeConfirm')"
    @close="removeIconOpen = false"
    @confirm="confirmRemoveIcon"
  />
</template>
