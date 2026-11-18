<script setup lang="ts">
import type { HostIconCategory } from '../../features/hosts/builtinIcons'
import { Image, Search, Trash2, Upload, X } from '@lucide/vue'
import { computed, onBeforeUnmount, ref } from 'vue'
import { builtinHostIcons, hostIconPickerGroups } from '../../features/hosts/builtinIcons'
import { customHostIconSrc, readCustomHostIcon } from '../../features/hosts/customHostIcon'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '../common/overlay-controls'
import { QtermButton, QtermInput } from '../common'
import { ScrollArea } from '../common/control-primitives'
import DeleteConfirmDialog from './DeleteConfirmDialog.vue'
import { useI18n } from '@/locales'
import { toast } from '../../services/toast'

const props = defineProps<{
  modelValue?: string
  label?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()
const { t } = useI18n()

const categoryLabelKeys: Record<HostIconCategory, string> = {
  os: 'shell.hostIcon.category.os',
  nas: 'shell.hostIcon.category.nas',
  network: 'shell.hostIcon.category.network',
  cloud: 'shell.hostIcon.category.cloud',
  virt: 'shell.hostIcon.category.virt',
  other: 'shell.hostIcon.category.other',
}

const selectedIcon = computed(() => builtinHostIcons.find((item) => item.value === props.modelValue))
const customIconSrc = computed(() => customHostIconSrc(props.modelValue))
const removeIconOpen = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
const searchInput = ref<InstanceType<typeof QtermInput> | null>(null)
const search = ref('')

const keyword = computed(() => search.value.trim().toLowerCase())
const groups = computed(() => {
  const value = keyword.value
  if (!value) return hostIconPickerGroups
  return hostIconPickerGroups
    .map((group) => ({
      category: group.category,
      icons: group.icons.filter(
        (icon) => icon.label.toLowerCase().includes(value) || icon.value.toLowerCase().includes(value),
      ),
    }))
    .filter((group) => group.icons.length > 0)
})
const hasMatches = computed(() => groups.value.length > 0)

let selectionVersion = 0
let pendingImportVersion = 0
let disposed = false

onBeforeUnmount(() => {
  disposed = true
  selectionVersion += 1
})

function focusSearchInput(): void {
  const root = (searchInput.value as unknown as { $el?: HTMLElement } | null)?.$el
  const element = root instanceof HTMLInputElement ? root : root?.querySelector('input')
  element?.focus()
}

function handleOpenAutoFocus(): void {
  window.setTimeout(() => focusSearchInput(), 0)
}

function handleMenuOpenChange(open: boolean): void {
  if (!open) search.value = ''
}

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
  <DropdownMenu @update:open="handleMenuOpenChange">
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
    <DropdownMenuContent align="start" class="host-icon-menu" @open-auto-focus.prevent="handleOpenAutoFocus">
      <div class="flex min-w-0 items-center gap-[6px] px-0.5 pt-0.5 pb-2">
        <div class="host-icon-search">
          <Search :size="14" class="host-icon-search-icon" aria-hidden="true" />
          <QtermInput
            ref="searchInput"
            v-model="search"
            type="search"
            class="host-icon-search-input"
            data-qterm-composite-input
            :placeholder="t('shell.hostIcon.searchPlaceholder')"
            :aria-label="t('shell.hostIcon.search')"
            @keydown.enter.prevent.stop
          />
          <QtermButton
            v-if="search"
            variant="ghost"
            size="icon-xs"
            type="button"
            class="host-icon-search-clear"
            :aria-label="t('shell.hostIcon.clearSearch')"
            @click="search = ''"
          >
            <X :size="14" aria-hidden="true" />
          </QtermButton>
        </div>

        <div class="flex flex-none items-center gap-[5px]" role="group" :aria-label="t('shell.hostIcon.options')">
          <DropdownMenuItem
            class="host-icon-action host-icon-action--danger cursor-pointer! data-[disabled]:cursor-default!"
            :disabled="!props.modelValue"
            :title="t('shell.hostIcon.remove')"
            :aria-label="t('shell.hostIcon.remove')"
            @select="requestRemoveIcon"
          >
            <Trash2 :size="15" aria-hidden="true" />
            <span class="sr-only">{{ t('shell.hostIcon.remove') }}</span>
          </DropdownMenuItem>
          <DropdownMenuItem
            class="host-icon-action cursor-pointer!"
            :title="t('shell.hostIcon.import')"
            :aria-label="t('shell.hostIcon.import')"
            @select="requestImportIcon"
          >
            <Upload :size="15" aria-hidden="true" />
            <span class="sr-only">{{ t('shell.hostIcon.import') }}</span>
          </DropdownMenuItem>
        </div>
      </div>

      <ScrollArea type="hover" :scroll-hide-delay="600" class="host-icon-scroll">
        <p v-if="!hasMatches" class="host-icon-empty m-0 px-2 py-6 text-center text-xs text-qt-muted">
          {{ t('shell.hostIcon.noMatch') }}
        </p>
        <div
          v-for="group in groups"
          :key="group.category"
          class="host-icon-group"
          role="group"
          :aria-label="t(categoryLabelKeys[group.category])"
        >
          <div class="host-icon-group-label">{{ t(categoryLabelKeys[group.category]) }}</div>
          <div class="host-icon-grid grid grid-cols-[repeat(8,_34px)] justify-center gap-[5px]">
            <DropdownMenuItem
              v-for="option in group.icons"
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
          </div>
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
