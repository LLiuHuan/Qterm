<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { FileText, FolderOpen, Layers, Plus, Terminal, Trash2, X } from '@lucide/vue'
import { QtermButton, QtermEmptyState, QtermTextarea } from '../../components/common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { useI18n } from '../../locales'
import {
  buildContext,
  byteLength,
  AI_CONTEXT_BUDGET_BYTES,
  type AiContextSource,
} from '../../services/aiContextSources'

const props = withDefaults(
  defineProps<{
    open: boolean
    items: AiContextSource[]
    busy?: boolean
    error?: string
  }>(),
  { busy: false, error: '' },
)

const emit = defineEmits<{
  'add-terminal': []
  'add-selection': []
  selection: [text: string]
  files: [files: File[]]
  remove: [id: string]
  clear: []
  close: []
  details: []
}>()

const { t } = useI18n()
const fileInput = ref<HTMLInputElement | null>(null)

const textOpen = ref(false)
const selectionText = ref('')
watch(
  () => props.open,
  (open) => {
    if (!open) {
      textOpen.value = false
      selectionText.value = ''
    }
  },
)

const built = computed(() => buildContext(props.items))
const usedBytes = computed(() => built.value.bytes)
const usagePercent = computed(() => Math.min(100, Math.round((usedBytes.value / AI_CONTEXT_BUDGET_BYTES) * 100)))

function kindLabel(kind: AiContextSource['kind']): string {
  return t(`shell.aiChat.contextSources.kind.${kind}`)
}

function kindIcon(kind: AiContextSource['kind']): typeof Terminal {
  if (kind === 'terminal') return Terminal
  if (kind === 'file') return FileText
  return Layers
}

function formatBytes(value: number): string {
  if (value < 1024) return `${value} B`
  return `${(value / 1024).toFixed(value < 10 * 1024 ? 1 : 0)} KB`
}

function toggleText(): void {
  textOpen.value = !textOpen.value
  emit('add-selection')
}

function openFilePicker(): void {
  if (!props.busy) fileInput.value?.click()
}

function onFileChange(event: Event): void {
  const input = event.target as HTMLInputElement
  const files = input.files ? Array.from(input.files) : []
  input.value = ''
  if (files.length) emit('files', files)
}
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => !open && emit('close')">
    <QtermDialogContent
      v-if="props.open"
      data-ai-context-sources-dialog
      size="lg"
      class="flex! max-h-[min(680px,calc(100vh-2rem))] min-w-0 flex-col overflow-hidden"
    >
      <QtermDialogHeader
        :title="t('shell.aiChat.contextSources.title')"
        :description="t('shell.aiChat.contextSources.description')"
        :show-close="false"
        @close="emit('close')"
      />

      <div class="dialog-body flex! min-h-0 min-w-0 flex-1 flex-col gap-3 overflow-hidden!">
        <div class="flex min-w-0 flex-wrap items-center gap-2" data-ai-context-source-actions>
          <QtermButton
            size="sm"
            variant="outline"
            class="h-8! min-h-8!"
            :disabled="props.busy"
            data-ai-context-add-terminal
            @click="emit('add-terminal')"
          >
            <Terminal class="size-3.5" aria-hidden="true" />
            {{ t('shell.aiChat.contextSources.addTerminal') }}
          </QtermButton>
          <QtermButton
            size="sm"
            variant="outline"
            class="h-8! min-h-8!"
            :disabled="props.busy"
            data-ai-context-add-selection
            @click="toggleText"
          >
            <Plus class="size-3.5" aria-hidden="true" />
            {{ t('shell.aiChat.contextSources.addSelection') }}
          </QtermButton>
          <QtermButton
            size="sm"
            variant="outline"
            class="h-8! min-h-8!"
            :disabled="props.busy"
            data-ai-context-add-file
            @click="openFilePicker"
          >
            <FolderOpen class="size-3.5" aria-hidden="true" />
            {{ t('shell.aiChat.contextSources.addFile') }}
          </QtermButton>
          <input
            ref="fileInput"
            class="hidden"
            type="file"
            multiple
            accept="text/*,.md,.txt,.json,.log,.yaml,.yml,.xml,.csv,.go,.ts,.tsx,.vue,.sh,.py,.sql,.html,.css"
            data-ai-context-file-input
            @change="onFileChange"
          />
        </div>

        <div v-if="textOpen" class="flex min-w-0 flex-col gap-2">
          <QtermTextarea
            v-model="selectionText"
            class="max-h-32 min-h-20"
            :placeholder="t('shell.aiChat.contextSources.textPlaceholder')"
            :aria-label="t('shell.aiChat.contextSources.addSelection')"
            :disabled="props.busy"
            data-ai-context-text
          />
          <QtermButton
            size="sm"
            variant="outline"
            class="self-end"
            :disabled="props.busy || !selectionText.trim()"
            data-ai-context-add-text
            @click="emit('selection', selectionText)"
            >{{ t('shell.aiChat.contextSources.addText') }}</QtermButton
          >
        </div>
        <p v-if="props.error" class="m-0 text-xs leading-5 text-qt-danger" role="alert" data-ai-context-error>
          {{ props.error }}
        </p>

        <div
          class="flex min-w-0 items-center justify-between gap-3 rounded-lg border border-qt-border/70 bg-qt-subtle/30 px-3 py-2 text-xs"
          data-ai-context-budget-summary
        >
          <span class="min-w-0 truncate text-qt-muted">{{ t('shell.aiChat.contextSources.budget') }}</span>
          <span class="shrink-0 tabular-nums text-qt-text"
            >{{ formatBytes(usedBytes) }} / {{ formatBytes(AI_CONTEXT_BUDGET_BYTES) }} ({{ usagePercent }}%)</span
          >
        </div>

        <div v-if="props.items.length" class="min-h-0 flex-1 overflow-y-auto overscroll-contain">
          <div class="flex min-w-0 flex-col gap-2 pe-1">
            <details
              v-for="item in props.items"
              :key="item.id"
              class="min-w-0 rounded-lg border border-qt-border/70"
              :data-ai-context-source="item.id"
            >
              <summary
                class="flex min-w-0 cursor-pointer list-none items-center gap-2 px-3 py-2 text-start [&::-webkit-details-marker]:hidden"
              >
                <component :is="kindIcon(item.kind)" class="size-3.5 shrink-0 text-qt-muted" aria-hidden="true" />
                <span class="min-w-0 flex-1 truncate text-xs text-qt-text" :title="item.label">{{ item.label }}</span>
                <span class="shrink-0 text-[11px] text-qt-muted">{{ kindLabel(item.kind) }}</span>
                <span class="shrink-0 font-mono text-[11px] text-qt-muted">{{
                  formatBytes(byteLength(item.text))
                }}</span>
                <span
                  v-if="item.truncated || built.truncatedSourceIds.includes(item.id)"
                  class="shrink-0 text-[11px] text-qt-warning"
                >
                  {{ t('shell.aiChat.contextSources.truncated') }}
                </span>
                <QtermButton
                  size="icon-sm"
                  variant="ghost"
                  class="size-7! shrink-0"
                  :aria-label="t('shell.aiChat.contextSources.remove')"
                  :title="t('shell.aiChat.contextSources.remove')"
                  :disabled="props.busy"
                  data-ai-context-remove
                  @click.stop.prevent="emit('remove', item.id)"
                >
                  <Trash2 class="size-3.5" aria-hidden="true" />
                </QtermButton>
              </summary>
              <div class="border-t border-qt-border/50 px-3 py-2">
                <pre
                  class="m-0 max-h-32 overflow-auto whitespace-pre-wrap break-words font-mono text-xs leading-5 text-qt-muted"
                  >{{ item.text }}</pre>
              </div>
            </details>
          </div>
        </div>

        <QtermEmptyState
          v-else
          :icon="Layers"
          data-ai-context-sources-empty
          :title="t('shell.aiChat.contextSources.empty')"
          :description="t('shell.aiChat.contextSources.emptyHint')"
        />
      </div>

      <QtermDialogFooter class="flex flex-wrap items-center justify-between gap-2">
        <QtermButton
          v-if="props.items.length"
          size="sm"
          variant="ghost"
          class="h-8! min-h-8! text-qt-danger"
          :disabled="props.busy"
          data-ai-context-clear
          @click="emit('clear')"
        >
          <X class="size-3.5" aria-hidden="true" />
          {{ t('shell.aiChat.contextSources.clear') }}
        </QtermButton>
        <span v-else />
        <div class="flex items-center gap-2">
          <QtermButton
            size="sm"
            variant="outline"
            class="h-8! min-h-8!"
            data-ai-context-details
            @click="emit('details')"
          >
            <Layers class="size-3.5" aria-hidden="true" />
            {{ t('shell.aiChat.contextSources.details') }}
          </QtermButton>
          <QtermButton size="sm" class="h-8! min-h-8!" data-ai-context-close @click="emit('close')">
            {{ t('shell.app.close') }}
          </QtermButton>
        </div>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>
