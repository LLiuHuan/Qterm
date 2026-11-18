<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { QtermSettingsRow, QtermSettingsSectionHeader, QtermSwitch } from '../../components/common'
import { useI18n } from '../../locales'
import { toast } from '../../services/toast'
import {
  DEFAULT_AI_CONTEXT_SETTINGS,
  MAX_AI_CUSTOM_PROMPT_LENGTH,
  loadAIContextSettings,
  saveAIContextSettings,
} from '../../services/aiContext'

const { t } = useI18n()

const SAVE_DEBOUNCE_MS = 600

const SAVED_FLASH_MS = 2000

const customPrompt = ref('')
const includeHostNote = ref(false)
const includeSnippetIndex = ref(false)
const loading = ref(false)
const saving = ref(false)
const savedVisible = ref(false)

const promptLength = computed(() => customPrompt.value.length)

let baseline = { ...DEFAULT_AI_CONTEXT_SETTINGS }

let ready = false
let saveTimer: ReturnType<typeof setTimeout> | null = null
let flashTimer: ReturnType<typeof setTimeout> | null = null

const dirty = computed(
  () =>
    customPrompt.value.trim() !== baseline.customPrompt ||
    includeHostNote.value !== baseline.includeHostNote ||
    includeSnippetIndex.value !== baseline.includeSnippetIndex,
)

function applySettings(settings: typeof DEFAULT_AI_CONTEXT_SETTINGS): void {
  customPrompt.value = settings.customPrompt
  includeHostNote.value = settings.includeHostNote
  includeSnippetIndex.value = settings.includeSnippetIndex
  baseline = {
    customPrompt: settings.customPrompt.trim(),
    includeHostNote: settings.includeHostNote,
    includeSnippetIndex: settings.includeSnippetIndex,
  }
}

function flashSaved(): void {
  savedVisible.value = true
  if (flashTimer) clearTimeout(flashTimer)
  flashTimer = setTimeout(() => {
    savedVisible.value = false
    flashTimer = null
  }, SAVED_FLASH_MS)
}

async function save(): Promise<void> {
  if (saving.value || !dirty.value) return
  saving.value = true
  try {
    applySettings(
      await saveAIContextSettings({
        customPrompt: customPrompt.value.trim(),
        includeHostNote: includeHostNote.value,
        includeSnippetIndex: includeSnippetIndex.value,
      }),
    )
    flashSaved()
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : t('shell.aiContext.saveFailed'))
  } finally {
    saving.value = false
  }
}

watch([customPrompt, includeHostNote, includeSnippetIndex], () => {
  if (!ready) return
  if (saveTimer) clearTimeout(saveTimer)
  saveTimer = setTimeout(() => {
    saveTimer = null
    void save()
  }, SAVE_DEBOUNCE_MS)
})

async function load(): Promise<void> {
  loading.value = true
  try {
    applySettings(await loadAIContextSettings())
  } finally {
    loading.value = false

    void Promise.resolve().then(() => {
      ready = true
    })
  }
}

onMounted(() => {
  void load()
})

onBeforeUnmount(() => {
  if (saveTimer) {
    clearTimeout(saveTimer)
    saveTimer = null
    void save()
  }
  if (flashTimer) {
    clearTimeout(flashTimer)
    flashTimer = null
  }
})
</script>

<template>
  <div class="flex flex-col gap-4" data-ai-context-settings>
    <QtermSettingsSectionHeader
      class="settings-subsection-header flex-cb gap-3 mt-2"
      :label="t('shell.aiContext.title')"
      :description="t('shell.aiContext.description')"
    >
      <template v-if="savedVisible" #actions>
        <span class="text-xs text-qt-muted" data-ai-context-saved>{{ t('shell.aiContext.saved') }}</span>
      </template>
    </QtermSettingsSectionHeader>

    <div
      class="settings-card border border-solid border-qt-border rounded-xl [background:color-mix(in_srgb,var(--qt-panel)_32%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
    >
      <div class="px-3 py-3" data-ai-context-prompt>
        <label class="text-sm font-medium text-qt-text" for="ai-custom-prompt">
          {{ t('shell.aiContext.customPromptLabel') }}
        </label>
        <p class="mt-1 text-xs text-qt-muted">{{ t('shell.aiContext.customPromptHelp') }}</p>

        <div class="relative mt-2">
          <textarea
            id="ai-custom-prompt"
            v-model="customPrompt"
            class="block min-h-24 w-full resize-y break-words overflow-x-hidden rounded-md border border-qt-border bg-transparent pt-2 pb-7 px-3 font-mono text-xs leading-5 text-qt-text outline-none focus-visible:border-qt-accent"
            data-qt-thin-scrollbar
            :maxlength="MAX_AI_CUSTOM_PROMPT_LENGTH"
            :placeholder="t('shell.aiContext.customPromptPlaceholder')"
            data-ai-context-prompt-input
          />
          <span
            class="pointer-events-none absolute end-[22px] bottom-1.5 rounded-full bg-qt-panel/92 px-1.5 py-0.5 text-[11px] tabular-nums text-qt-muted backdrop-blur-sm"
            data-ai-context-counter
          >
            {{ promptLength }} / {{ MAX_AI_CUSTOM_PROMPT_LENGTH }}
          </span>
        </div>
      </div>

      <QtermSettingsRow :label="t('shell.aiContext.hostNoteLabel')" :description="t('shell.aiContext.hostNoteHelp')">
        <QtermSwitch v-model="includeHostNote" data-ai-context-host-note />
      </QtermSettingsRow>

      <QtermSettingsRow
        :label="t('shell.aiContext.snippetIndexLabel')"
        :description="t('shell.aiContext.snippetIndexHelp')"
      >
        <QtermSwitch v-model="includeSnippetIndex" data-ai-context-snippet-index />
      </QtermSettingsRow>
    </div>
  </div>
</template>
