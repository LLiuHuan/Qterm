<script setup lang="ts">
import { computed, ref } from 'vue'
import { Check, Search } from '@lucide/vue'
import { QtermButton, QtermCheckbox, QtermInput } from '../../../components/common'
import { ScrollArea } from '../../../components/common/control-primitives'
import { useI18n } from '../../../locales'
import type { TerminalControlSession } from '../types'

const props = withDefaults(
  defineProps<{
    sessions: TerminalControlSession[]
    selectedIds: string[]
    sourceSessionId?: string
    lockedIds?: string[]
    disabled?: boolean
    maxHeightClass?: string

    searchClear?: boolean

    selectAll?: boolean
  }>(),
  {
    sourceSessionId: '',
    lockedIds: () => [],
    disabled: false,
    maxHeightClass: 'max-h-48',
    searchClear: true,
    selectAll: false,
  },
)

const emit = defineEmits<{ 'update:selectedIds': [ids: string[]] }>()
const { t } = useI18n()
const search = ref('')
const locked = computed(() => new Set(props.lockedIds ?? []))
const selected = computed(() => new Set(props.selectedIds))

const visibleSessions = computed(() => {
  const query = search.value.trim().toLocaleLowerCase()
  return props.sessions.filter((session) => {
    if (!query) return true
    const haystack = [session.title, session.address, session.username, session.tabTitle].filter(Boolean).join(' ')
    return haystack.toLocaleLowerCase().includes(query)
  })
})

const selectableVisible = computed(() =>
  visibleSessions.value.filter((session) => session.ready && !locked.value.has(session.id)),
)
const selectedVisibleCount = computed(
  () => selectableVisible.value.filter((session) => selected.value.has(session.id)).length,
)
const allVisibleSelected = computed(
  () => selectableVisible.value.length > 0 && selectedVisibleCount.value === selectableVisible.value.length,
)
const someVisibleSelected = computed(() => selectedVisibleCount.value > 0 && !allVisibleSelected.value)

function toggle(session: TerminalControlSession, value: boolean | 'indeterminate'): void {
  if (props.disabled || !session.ready || locked.value.has(session.id)) return
  const next = new Set(selected.value)
  if (value === true) next.add(session.id)
  else next.delete(session.id)
  emit('update:selectedIds', [...next])
}

function clearSearch(): void {
  search.value = ''
}

function toggleAllVisible(value: boolean | 'indeterminate'): void {
  if (props.disabled) return
  const next = new Set(selected.value)
  const select = value === true
  for (const session of selectableVisible.value) {
    if (select) next.add(session.id)
    else next.delete(session.id)
  }
  emit('update:selectedIds', [...next])
}
</script>

<template>
  <div class="flex min-h-0 flex-col gap-2">
    <div class="flex items-center gap-2">
      <div class="relative w-full">
        <Search class="pointer-events-none absolute left-2 top-1/2 size-3.5 -translate-y-1/2 text-qt-muted" />
        <QtermInput v-model="search" class="h-8 pl-7 text-xs" :placeholder="t('terminalControl.sync.search')" />
      </div>
      <QtermButton v-if="props.searchClear && search" class="shrink-0" size="xs" variant="ghost" @click="clearSearch">{{
        t('terminalControl.sync.cancel')
      }}</QtermButton>
    </div>

    <div class="flex min-h-0 flex-col rounded-md border border-qt-border" :class="props.maxHeightClass">
      <div
        v-if="props.selectAll"
        class="flex shrink-0 items-center gap-2 border-b border-qt-border px-3 py-1.5"
        data-picker-select-all-row
      >
        <QtermCheckbox
          :model-value="allVisibleSelected ? true : someVisibleSelected ? 'indeterminate' : false"
          :disabled="props.disabled || !selectableVisible.length"
          :aria-label="t('terminalControl.batch.selectAll')"
          data-picker-select-all
          @update:model-value="toggleAllVisible"
        />
        <span class="ml-auto text-xs text-qt-muted">{{
          t('terminalControl.batch.selectedCount', { count: props.selectedIds.length })
        }}</span>
      </div>
      <ScrollArea type="hover" :scroll-hide-delay="600" class="min-h-0 flex-1">
        <div v-if="!visibleSessions.length" class="p-5 text-center text-xs text-qt-muted">
          {{ t('terminalControl.sync.empty') }}
        </div>
        <label
          v-for="session in visibleSessions"
          :key="session.id"
          class="flex cursor-pointer items-center gap-2 border-b border-qt-border/60 px-3 py-2 last:border-b-0 hover:bg-qt-hover"
          :class="{ 'cursor-not-allowed opacity-55': props.disabled || !session.ready || locked.has(session.id) }"
        >
          <QtermCheckbox
            :model-value="selected.has(session.id)"
            :disabled="props.disabled || !session.ready || locked.has(session.id)"
            :aria-label="session.title"
            @update:model-value="(value) => toggle(session, value)"
          />
          <span class="min-w-0 flex-1">
            <span class="flex items-center gap-1 text-xs text-qt-text">
              <span class="truncate">{{ session.title }}</span>
              <Check v-if="locked.has(session.id)" :size="13" class="shrink-0 text-qt-accent" aria-hidden="true" />
            </span>
            <span class="block truncate text-xs text-qt-muted">
              {{ [session.username, session.address].filter(Boolean).join('@') || session.address || '—' }}
              <span v-if="session.tabTitle"> · {{ session.tabTitle }}</span>
            </span>
          </span>
          <span v-if="!session.ready" class="shrink-0 text-xs text-qt-muted">{{
            t('terminalControl.sync.notReady')
          }}</span>
        </label>
      </ScrollArea>
    </div>
  </div>
</template>
