<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { Search, Server } from '@lucide/vue'
import { useI18n } from '../../locales'
import { ScrollArea } from '../../components/ui/scroll-area'

export interface MentionHost {
  id: string
  name: string
  address?: string
}

const props = defineProps<{
  open: boolean
  hosts: MentionHost[]

  selected: string[]
}>()

const emit = defineEmits<{ select: [host: MentionHost]; close: [] }>()
const { t } = useI18n()
const query = ref('')
const inputRef = ref<HTMLInputElement | null>(null)
const rootRef = ref<HTMLElement | null>(null)

const filtered = computed(() => {
  const keyword = query.value.trim().toLowerCase()
  const list = keyword
    ? props.hosts.filter((host) => `${host.name} ${host.address ?? ''}`.toLowerCase().includes(keyword))
    : props.hosts

  return list
})

const ROW_HEIGHT = 28
const MAX_ROWS = 8
const listHeight = computed(() => `${Math.min(filtered.value.length, MAX_ROWS) * ROW_HEIGHT + 8}px`)

function handlePointerDown(event: PointerEvent): void {
  const target = event.target as Node | null
  if (target && rootRef.value?.contains(target)) return
  emit('close')
}

watch(
  () => props.open,
  (open) => {
    if (!open) {
      query.value = ''
      document.removeEventListener('pointerdown', handlePointerDown, true)
      return
    }
    document.addEventListener('pointerdown', handlePointerDown, true)
    void nextTick(() => inputRef.value?.focus())
  },
  { immediate: true },
)

onBeforeUnmount(() => document.removeEventListener('pointerdown', handlePointerDown, true))

function choose(host: MentionHost): void {
  emit('select', host)
}
</script>

<template>
  <div
    v-if="props.open"
    ref="rootRef"
    class="absolute inset-x-2 bottom-full z-20 mb-1 flex max-h-[17rem] flex-col overflow-hidden rounded-lg border border-qt-border bg-qt-panel shadow-lg"
    data-ai-host-mention
  >
    <label class="flex items-center gap-1.5 border-b border-qt-border px-2 py-1.5">
      <Search class="size-3.5 shrink-0 text-qt-muted" aria-hidden="true" />
      <input
        ref="inputRef"
        v-model="query"
        type="text"
        class="min-w-0 flex-1 appearance-none rounded-none border-0 bg-transparent p-0 text-xs text-qt-text shadow-none ring-0 outline-none placeholder:text-qt-muted/70 focus:border-0 focus:outline-none focus:ring-0 focus-visible:outline-none"
        :placeholder="t('shell.aiChat.mention.search')"
        data-ai-host-mention-search
        data-ai-inline-field
        @keydown.esc.prevent="emit('close')"
      />
    </label>

    <ScrollArea
      v-if="filtered.length"
      class="ai-mention-list"
      :style="{ height: listHeight }"
      data-ai-host-mention-list
    >
      <div class="py-1">
        <button
          v-for="host in filtered"
          :key="host.id"
          type="button"
          class="flex h-7 w-full min-w-0 items-center gap-1.5 px-2 text-left text-xs text-qt-text hover:bg-qt-hover"
          :data-ai-host-mention-option="host.id"
          @click="choose(host)"
        >
          <Server class="size-3.5 shrink-0 text-qt-muted" aria-hidden="true" />
          <span class="min-w-0 flex-1 truncate">{{ host.name }}</span>
          <span v-if="host.address" class="shrink-0 truncate font-mono text-qt-muted/80">{{ host.address }}</span>
          <span v-if="props.selected.includes(host.id)" class="shrink-0 text-qt-accent">✓</span>
        </button>
      </div>
    </ScrollArea>
    <p v-else class="m-0 px-2 py-2 text-xs text-qt-muted/70">
      {{ t('shell.aiChat.mention.empty') }}
    </p>
  </div>
</template>

<style scoped>
[data-ai-host-mention-search] {
  border: 0 !important;
  background: transparent !important;
  box-shadow: none !important;
  outline: none !important;
}
</style>
