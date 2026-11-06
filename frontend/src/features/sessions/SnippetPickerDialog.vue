<script setup lang="ts">
import { QtermInput } from '../../components/common'
import { computed, watch, ref } from 'vue'
import { FileText, Folder, Search, X } from '@lucide/vue'
import {
  QtermDialog,
  QtermDialogContent,
  QtermDialogDescription,
  QtermDialogTitle,
} from '../../components/common/dialog'
import { ScrollArea } from '../../components/common/control-primitives'
import { createSnippetGateway, type Snippet } from '../../services/snippets'
import type { GroupRecord } from '@/services/hosts.ts'
import { useI18n } from '@/locales'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ 'update:open': [value: boolean]; select: [content: string] }>()
const snippets = ref<Snippet[]>([])
const groups = ref<GroupRecord[]>([])
const query = ref('')
const loading = ref(false)
const error = ref('')
const gateway = createSnippetGateway()
const { t } = useI18n()
const filtered = computed(() => {
  const value = query.value.trim().toLocaleLowerCase()
  return snippets.value.filter(
    (snippet) => !value || `${snippet.name}\n${snippet.content}`.toLocaleLowerCase().includes(value),
  )
})
const sections = computed(() => {
  const map = new Map<string, Snippet[]>()
  for (const snippet of filtered.value)
    map.set(snippet.groupId || '', [...(map.get(snippet.groupId || '') ?? []), snippet])
  return [...map.entries()].map(([id, items]) => ({
    id,
    name: groups.value.find((group) => group.id === id)?.name ?? t('shell.snippetPicker.ungrouped'),
    items,
  }))
})
async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    ;[snippets.value, groups.value] = await Promise.all([gateway.list(), gateway.listGroups()])
  } catch {
    error.value = t('shell.snippetPicker.error')
  } finally {
    loading.value = false
  }
}
function select(content: string): void {
  emit('select', content)
  emit('update:open', false)
}
watch(
  () => props.open,
  (open) => {
    if (open) {
      query.value = ''
      void load()
    }
  },
  { immediate: true },
)
</script>

<template>
  <QtermDialog :open="props.open" @update:open="emit('update:open', $event)">
    <QtermDialogContent class="snippet-picker-dialog" :close-on-escape="true">
      <button
        type="button"
        class="snippet-picker-close"
        :aria-label="t('shell.snippetPicker.close')"
        @click="emit('update:open', false)"
      >
        <X :size="15" />
      </button>
      <header class="snippet-picker-header">
        <QtermDialogTitle class="text-base">{{ t('shell.snippetPicker.title') }}</QtermDialogTitle>
        <QtermDialogDescription class="text-xs">{{ t('shell.snippetPicker.description') }}</QtermDialogDescription>
      </header>
      <div class="snippet-picker-search">
        <Search :size="15" aria-hidden="true" /><QtermInput
          v-model="query"
          :aria-label="t('shell.snippetPicker.search')"
          class="text-xs"
          :placeholder="t('shell.snippetPicker.search')"
        />
      </div>
      <ScrollArea class="snippet-picker-list">
        <div class="snippet-picker-list-content">
          <p v-if="loading" class="snippet-picker-empty text-xs">{{ t('shell.snippetPicker.loading') }}</p>
          <p v-else-if="error" class="snippet-picker-empty text-xs">{{ error }}</p>
          <p v-else-if="!sections.length" class="snippet-picker-empty text-xs">{{ t('shell.snippetPicker.empty') }}</p>
          <section
            v-for="section in sections"
            v-else
            :key="section.id || '__ungrouped__'"
            class="snippet-picker-section"
          >
            <h3 class="text-xs"><Folder :size="14" aria-hidden="true" />{{ section.name }}</h3>
            <button
              v-for="snippet in section.items"
              :key="snippet.id"
              type="button"
              class="snippet-picker-item"
              @click="select(snippet.content)"
            >
              <strong class="text-xs"><FileText :size="14" aria-hidden="true" />{{ snippet.name }}</strong>
              <code class="text-xs">{{ snippet.content }}</code>
            </button>
          </section>
        </div>
      </ScrollArea>
    </QtermDialogContent>
  </QtermDialog>
</template>

<style scoped>
:global(.snippet-picker-dialog[data-slot='dialog-content']) {
  width: min(960px, calc(100vw - 32px)) !important;
  max-height: var(--qt-snippet-picker-height-even, 640px);
  max-width: min(960px, calc(100vw - 32px)) !important;
  gap: 8px;
  padding: 18px;
  color: var(--qt-text);

  background: color-mix(in srgb, var(--qt-panel) 94%, transparent);
  border: 1px solid var(--qt-border);
  box-shadow: 0 18px 56px rgb(24 24 27 / 24%);
}
.snippet-picker-header {
  display: grid;
  gap: 5px;
  padding-right: 32px;
}
.snippet-picker-header h2 {
  color: var(--qt-text);
  font-weight: 600;
}
.snippet-picker-header p {
  margin: 0;
  color: var(--qt-muted);
}
.snippet-picker-close {
  position: absolute;
  top: 10px;
  right: 10px;
  display: grid;
  width: 26px;
  height: 26px;
  place-items: center;
  border: 1px solid var(--qt-border);
  border-radius: 5px;
  color: var(--qt-muted);
  background: transparent;
  transition:
    color 150ms ease,
    border-color 150ms ease,
    background-color 150ms ease,
    transform 150ms ease;
}
.snippet-picker-close:hover {
  border-color: color-mix(in srgb, var(--qt-accent) 38%, var(--qt-border));
  color: var(--qt-text);
  background: var(--qt-hover);
  transform: rotate(90deg);
}
.snippet-picker-close:active {
  transform: rotate(90deg) scale(0.9);
}
.snippet-picker-search {
  display: flex;
  align-items: center;
  gap: 0;
  height: 32px;
  margin: 0.75rem 0 0.25rem;
  padding: 0 10px;
  border: 1px solid var(--qt-border);
  border-radius: 6px;
  background: color-mix(in srgb, var(--qt-panel) 24%, transparent);
}
.snippet-picker-search:focus-within {
  border-color: var(--qt-focus);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--qt-focus) 16%, transparent);
}
.snippet-picker-search :deep([data-slot='input']),
.snippet-picker-search :deep([data-slot='input']:focus),
.snippet-picker-search :deep([data-slot='input']:focus-visible) {
  height: 30px;
  padding-left: 6px !important;
  border: 0 !important;
  outline: 0 !important;
  line-height: 1.4;
  background: transparent !important;
  box-shadow: none !important;
}
.snippet-picker-list {
  height: var(--qt-snippet-picker-list-height-even, 420px);
}
.snippet-picker-list-content {
  padding: 2px 12px 8px 2px;
}
.snippet-picker-section h3 {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 6px 2px;
  color: var(--qt-muted);
  font-weight: 500;
}
.snippet-picker-item {
  display: grid;
  box-sizing: border-box;
  width: 100%;
  gap: 5px;
  margin: 0 0 7px;
  padding: 9px 12px;
  border: 0;
  border-radius: 5px;
  text-align: left;
  border: 1px solid color-mix(in srgb, var(--qt-border) 72%, transparent);
  color: var(--qt-text);
  background: color-mix(in srgb, var(--qt-panel) 48%, transparent);
}
.snippet-picker-item:hover,
.snippet-picker-item:focus-visible {
  border-color: color-mix(in srgb, var(--qt-accent) 42%, var(--qt-border));
  background: color-mix(in srgb, var(--qt-accent) 10%, var(--qt-panel));
  outline: none;
}
.snippet-picker-item strong {
  display: flex;
  align-items: center;
  gap: 7px;
  font-weight: 500;
}
.snippet-picker-item code {
  overflow: hidden;
  color: var(--qt-muted);
  line-height: 1.4;
  font-family: var(--qt-font-mono);
  text-overflow: ellipsis;
  white-space: nowrap;
}
.snippet-picker-empty {
  padding: 24px 8px;
  color: var(--qt-muted);
  text-align: center;
}
</style>
