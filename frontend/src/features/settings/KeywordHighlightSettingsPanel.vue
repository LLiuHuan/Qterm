<script setup lang="ts">
import { QtermCheckbox, QtermSettingsRow, QtermSettingsSectionHeader, QtermSwitch } from '../../components/common'
import { IconButton, QtermButton, QtermInput, QtermTextarea } from '../../components/common'
import { computed, ref } from 'vue'
import { VueDraggable } from 'vue-draggable-plus'
import { Clipboard, GripVertical, Plus, Trash2, Upload } from '@lucide/vue'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { readClipboardText, writeClipboardText } from '../../services/clipboard'
import { toast } from 'vue-sonner'
import type { KeywordHighlightRule } from './settingsStore'
import TerminalColorPicker from './TerminalColorPicker.vue'
import { useI18n } from '@/locales'

const props = defineProps<{
  enabled: boolean
  rules: KeywordHighlightRule[]
}>()

const emit = defineEmits<{
  'update:enabled': [value: boolean]
  'update:rules': [value: KeywordHighlightRule[]]
}>()
const { t } = useI18n()

const importError = ref('')
const importBusy = ref(false)
const importOpen = ref(false)
const importText = ref('')

function copyRules(): void {
  importError.value = ''
  void writeClipboardText(JSON.stringify(props.rules, null, 2))
    .then(() => {
      toast.success(t('settings.keywordHighlight.toast.copied'))
    })
    .catch(() => {
      toast.error(t('settings.keywordHighlight.toast.copyFailed'))
    })
}

async function openImportDialog(): Promise<void> {
  if (importBusy.value) return
  importBusy.value = true
  importError.value = ''
  try {
    importText.value = await readClipboardText()
  } catch {
    importText.value = ''
    importError.value = t('settings.keywordHighlight.error.clipboardRead')
  } finally {
    importOpen.value = true
    importBusy.value = false
  }
}

function applyImport(): void {
  if (importBusy.value) return
  importBusy.value = true
  importError.value = ''
  try {
    const parsed: unknown = JSON.parse(importText.value)
    if (!Array.isArray(parsed)) throw new Error(t('settings.keywordHighlight.error.invalidArray'))
    if (parsed.length > 100) throw new Error(t('settings.keywordHighlight.error.tooMany'))
    const rules: KeywordHighlightRule[] = parsed.map((item, index) => {
      if (!item || typeof item !== 'object')
        throw new Error(t('settings.keywordHighlight.error.invalidRule', { index: index + 1 }))
      const value = item as Record<string, unknown>
      if (typeof value.pattern !== 'string')
        throw new Error(t('settings.keywordHighlight.error.missingPattern', { index: index + 1 }))
      if (typeof value.color !== 'string' || !/^#[\da-f]{6}$/i.test(value.color)) {
        throw new Error(t('settings.keywordHighlight.error.invalidColor', { index: index + 1 }))
      }
      return {
        enabled: typeof value.enabled === 'boolean' ? value.enabled : true,
        pattern: value.pattern.slice(0, 512),
        note: typeof value.note === 'string' ? value.note.trim().slice(0, 256) : '',
        color: value.color.toLowerCase(),
      }
    })
    emit('update:rules', rules)
    importOpen.value = false
  } catch (error) {
    importError.value =
      error instanceof SyntaxError
        ? t('settings.keywordHighlight.error.parseFailed')
        : error instanceof Error
          ? error.message
          : t('settings.keywordHighlight.error.importFailed')
  } finally {
    importBusy.value = false
  }
}

function updateRule(index: number, patch: Partial<KeywordHighlightRule>): void {
  const rules = props.rules.map((rule, itemIndex) => (itemIndex === index ? { ...rule, ...patch } : { ...rule }))
  emit('update:rules', rules)
}

function addRule(): void {
  emit('update:rules', [
    ...props.rules.map((rule) => ({ ...rule })),
    { enabled: true, pattern: '', note: '', color: '#ff0000' },
  ])
}

function removeRule(index: number): void {
  emit(
    'update:rules',
    props.rules.filter((_, itemIndex) => itemIndex !== index).map((rule) => ({ ...rule })),
  )
}

const draggableRules = computed({
  get: () => props.rules,
  set: (value: KeywordHighlightRule[]) =>
    emit(
      'update:rules',
      value.map((rule) => ({ ...rule })),
    ),
})

interface PreviewSegment {
  text: string
  color?: string
}

function previewSegments(rules: KeywordHighlightRule[]): PreviewSegment[] {
  const sample = 'ERROR at 192.168.1.100'
  const matches: Array<{ start: number; end: number; color: string; priority: number }> = []
  rules.forEach((rule, priority) => {
    if (!rule.enabled || !isValidPattern(rule.pattern)) return
    try {
      for (const match of sample.matchAll(new RegExp(rule.pattern, 'gu'))) {
        const start = match.index ?? 0
        if (match[0]) matches.push({ start, end: start + match[0].length, color: rule.color, priority })
      }
    } catch {}
  })
  const boundaries = [...new Set([0, sample.length, ...matches.flatMap((match) => [match.start, match.end])])].sort(
    (a, b) => a - b,
  )
  const segments: PreviewSegment[] = []
  for (let index = 0; index < boundaries.length - 1; index += 1) {
    const start = boundaries[index]!
    const end = boundaries[index + 1]!
    if (start === end) continue
    const winner = matches
      .filter((match) => match.start <= start && match.end >= end)
      .sort((a, b) => b.priority - a.priority)[0]
    const previous = segments[segments.length - 1]
    const color = winner?.color
    if (previous && previous.color === color) previous.text += sample.slice(start, end)
    else segments.push({ text: sample.slice(start, end), color })
  }
  return segments.length ? segments : [{ text: sample }]
}

function isValidPattern(pattern: string): boolean {
  if (!pattern) return false
  try {
    new RegExp(pattern, 'u')
    return true
  } catch {
    return false
  }
}

const previewRules = computed(() => props.rules)
</script>

<template>
  <section
    class="settings-panel keyword-highlight-settings-panel flex flex-col gap-4"
    :aria-label="t('settings.keywordHighlight.panel.aria')"
  >
    <div
      class="settings-card border border-solid border-qt-border rounded-xl [background:color-mix(in_srgb,var(--qt-panel)_32%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
    >
      <QtermSettingsRow
        class="settings-switch-row c-p"
        :label="t('settings.keywordHighlight.enabled.title')"
        :description="t('settings.keywordHighlight.enabled.description')"
      >
        <QtermSwitch
          id="settings-keyword-highlight-enabled"
          :model-value="props.enabled"
          :aria-label="t('settings.keywordHighlight.enabled.aria')"
          @update:model-value="emit('update:enabled', $event === true)"
        />
      </QtermSettingsRow>
    </div>

    <QtermSettingsSectionHeader
      class="settings-subsection-header mt-2"
      :label="t('settings.keywordHighlight.preview.title')"
      :description="t('settings.keywordHighlight.preview.description')"
    />
    <div class="keyword-highlight-preview text-sm" :aria-label="t('settings.keywordHighlight.preview.aria')">
      <span>$ npm run build</span>
      <span>WARN retrying connection to api-gateway</span>
      <span>
        <template v-for="(segment, segmentIndex) in previewSegments(previewRules)" :key="segmentIndex">
          <b v-if="segment.color" :style="{ color: segment.color }">{{ segment.text }}</b>
          <span v-else>{{ segment.text }}</span>
        </template>
      </span>
      <span>deploy success: health check passed</span>
    </div>

    <QtermSettingsSectionHeader
      class="settings-subsection-header mt-2 keyword-highlight-rules-header"
      :label="t('settings.keywordHighlight.rules.title', { count: props.rules.length })"
      :description="t('settings.keywordHighlight.rules.description')"
    >
      <template #actions>
        <div class="keyword-highlight-rule-actions">
          <QtermButton variant="outline" size="sm" type="button" class="text-xs" @click="copyRules"
            ><Clipboard :size="14" />{{ t('settings.keywordHighlight.action.copy') }}</QtermButton
          >
          <QtermButton
            variant="outline"
            size="sm"
            type="button"
            class="text-xs"
            :disabled="importBusy"
            @click="openImportDialog"
            ><Upload :size="14" />{{ t('settings.keywordHighlight.action.import') }}</QtermButton
          >
          <QtermButton
            variant="outline"
            size="sm"
            type="button"
            class="text-xs"
            :disabled="props.rules.length >= 100"
            @click="addRule"
            ><Plus :size="14" />{{ t('settings.keywordHighlight.action.add') }}</QtermButton
          >
        </div>
      </template>
    </QtermSettingsSectionHeader>

    <div
      class="keyword-highlight-rules settings-card border border-solid border-qt-border rounded-xl [background:color-mix(in_srgb,var(--qt-panel)_32%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
    >
      <div class="keyword-highlight-rule-head text-xs" aria-hidden="true">
        <span></span><span>{{ t('settings.keywordHighlight.table.enabled') }}</span
        ><span>{{ t('settings.keywordHighlight.table.pattern') }}</span
        ><span>{{ t('settings.keywordHighlight.table.note') }}</span
        ><span>{{ t('settings.keywordHighlight.table.color') }}</span
        ><span>{{ t('settings.keywordHighlight.table.actions') }}</span>
      </div>
      <div v-if="props.rules.length === 0" class="keyword-highlight-empty text-sm">
        {{ t('settings.keywordHighlight.empty') }}
      </div>
      <VueDraggable
        v-model="draggableRules"
        class="keyword-highlight-rule-list"
        tag="div"
        handle=".keyword-highlight-drag-handle"
        draggable=".keyword-highlight-rule-row"
        direction="vertical"
        :animation="300"
        easing="cubic-bezier(0.2, 0.8, 0.2, 1)"
        ghost-class="keyword-highlight-rule-ghost"
        chosen-class="keyword-highlight-rule-chosen"
        drag-class="keyword-highlight-rule-drag"
        fallback-class="keyword-highlight-rule-dragging"
        :force-fallback="true"
        :fallback-on-body="true"
        :fallback-tolerance="4"
        :swap-threshold="0.65"
        :invert-swap="true"
        :inverted-swap-threshold="0.3"
      >
        <div v-for="(rule, index) in draggableRules" :key="index" class="keyword-highlight-rule-row">
          <GripVertical class="keyword-highlight-drag-handle" :size="15" aria-hidden="true" />
          <QtermCheckbox
            :model-value="rule.enabled"
            :aria-label="t('settings.keywordHighlight.rule.enabled', { index: index + 1 })"
            @update:model-value="updateRule(index, { enabled: $event === true })"
          />
          <QtermInput
            :model-value="rule.pattern"
            class="keyword-highlight-rule-pattern"
            :class="{ 'is-invalid': rule.pattern.length > 0 && !isValidPattern(rule.pattern) }"
            :aria-label="t('settings.keywordHighlight.rule.pattern', { index: index + 1 })"
            :placeholder="t('settings.keywordHighlight.rule.patternPlaceholder')"
            @update:model-value="updateRule(index, { pattern: String($event) })"
          />
          <QtermInput
            :model-value="rule.note"
            :aria-label="t('settings.keywordHighlight.rule.note', { index: index + 1 })"
            :placeholder="t('settings.keywordHighlight.rule.notePlaceholder')"
            @update:model-value="updateRule(index, { note: String($event) })"
          />
          <div class="keyword-highlight-color-field">
            <TerminalColorPicker
              :model-value="rule.color"
              :label="t('settings.keywordHighlight.rule.color', { index: index + 1 })"
              @update:model-value="updateRule(index, { color: $event })"
            />
            <QtermInput
              :model-value="rule.color"
              class="keyword-highlight-color-text"
              :aria-label="t('settings.keywordHighlight.rule.hexColor', { index: index + 1 })"
              @update:model-value="updateRule(index, { color: String($event) })"
            />
          </div>
          <IconButton
            danger
            size="icon-sm"
            class="keyword-highlight-remove-button"
            :label="t('settings.keywordHighlight.rule.delete', { index: index + 1 })"
            @click="removeRule(index)"
          >
            <Trash2 :size="14" />
          </IconButton>
        </div>
      </VueDraggable>
    </div>
    <p v-if="importError && !importOpen" class="dialog-error text-xs" role="alert">{{ importError }}</p>
  </section>

  <QtermDialog :open="importOpen" @update:open="(open) => (importOpen = open)">
    <QtermDialogContent
      class="keyword-highlight-import-dialog w-[min(820px,calc(100vw-2rem))]"
      size="2xl"
      :show-close-button="true"
    >
      <QtermDialogHeader
        :title="t('settings.keywordHighlight.dialog.title')"
        :description="t('settings.keywordHighlight.dialog.description')"
        :show-close="false"
      />
      <div class="dialog-body keyword-highlight-import-body">
        <QtermTextarea
          v-model="importText"
          rows="14"
          spellcheck="false"
          :aria-label="t('settings.keywordHighlight.dialog.content')"
          class="keyword-highlight-import-textarea text-xs"
          :class="{ 'is-invalid': Boolean(importError) }"
          :aria-invalid="Boolean(importError)"
        />
        <p class="keyword-highlight-import-hint text-xs">{{ t('settings.keywordHighlight.dialog.hint') }}</p>
        <p v-if="importError" class="dialog-error text-xs" role="alert">{{ importError }}</p>
      </div>
      <QtermDialogFooter>
        <QtermButton variant="outline" type="button" @click="importOpen = false">{{
          t('settings.keywordHighlight.action.cancel')
        }}</QtermButton>
        <QtermButton type="button" :disabled="importBusy" @click="applyImport">{{
          t('settings.keywordHighlight.action.apply')
        }}</QtermButton>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>

<style scoped>
.keyword-highlight-preview {
  display: grid;
  gap: 2px;
  padding: 16px 18px;
  overflow: auto;
  border: 1px solid var(--qt-border);
  border-radius: 10px;
  background: var(--qt-theme-7) !important;
  color: var(--qt-text);
  font-family: var(--qt-font-mono);
  line-height: 1.65;
}

.keyword-highlight-rules-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 12px;
}

.keyword-highlight-rule-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 6px;
}

.keyword-highlight-rule-actions :deep([data-slot='button']) {
  gap: 5px;
}

.keyword-highlight-rules {
  overflow: hidden;
  padding: 0 10px;
}

.keyword-highlight-rule-list {
  display: flex;
  flex-direction: column;
}

.keyword-highlight-rule-head,
.keyword-highlight-rule-row {
  display: grid;
  grid-template-columns: 20px 42px minmax(140px, 1.3fr) minmax(110px, 1fr) minmax(150px, 1fr) 30px;
  align-items: center;
  gap: 8px;
}

.keyword-highlight-rule-head {
  min-height: 38px;
  border-bottom: 1px solid var(--qt-border);
  color: var(--qt-muted);
}

.keyword-highlight-rule-row {
  min-height: 52px;
  border-bottom: 1px solid color-mix(in srgb, var(--qt-border) 70%, transparent);
  transition:
    background-color 120ms ease,
    opacity 120ms ease;
}

.keyword-highlight-rule-row:last-child {
  border-bottom: 0;
}

.keyword-highlight-drag-handle {
  color: var(--qt-muted);
  cursor: grab;
  transition:
    color 120ms ease,
    transform 120ms ease;
}

.keyword-highlight-drag-handle:hover {
  color: var(--qt-text);
  transform: scale(1.08);
}

.keyword-highlight-rule-row :deep(.keyword-highlight-remove-button) {
  transition:
    color 120ms ease,
    background-color 120ms ease,
    transform 120ms ease;
}

.keyword-highlight-rule-row :deep(.keyword-highlight-remove-button:hover) {
  color: var(--qt-danger);
  background: color-mix(in srgb, var(--qt-danger) 12%, transparent);
  transform: scale(1.06);
}

.keyword-highlight-rule-row :deep(.keyword-highlight-remove-button:focus-visible) {
  color: var(--qt-danger);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--qt-danger) 28%, transparent);
}

.keyword-highlight-rule-row :deep([data-slot='input']) {
  min-width: 0;
}

.keyword-highlight-rule-pattern :deep([data-slot='input']) {
  font-family: var(--qt-font-mono);
}

.keyword-highlight-rule-pattern.is-invalid :deep([data-slot='input']) {
  border-color: var(--qt-danger);
}

.keyword-highlight-color-field {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 6px;
}

.keyword-highlight-color-text {
  min-width: 0;
  flex: 1;
}

.keyword-highlight-empty {
  padding: 28px 12px;
  color: var(--qt-muted);
  text-align: center;
}

.keyword-highlight-import-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.keyword-highlight-import-textarea {
  min-height: 260px;
  font-family: var(--qt-font-mono);
  line-height: 1.55;
  white-space: pre;
  scrollbar-width: thin;
  scrollbar-color: color-mix(in srgb, var(--qt-muted) 28%, transparent) transparent;
}

.keyword-highlight-import-textarea::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

.keyword-highlight-import-textarea::-webkit-scrollbar-track {
  background: transparent;
}

.keyword-highlight-import-textarea::-webkit-scrollbar-thumb {
  border: 2px solid transparent;
  border-radius: 999px;
  background: color-mix(in srgb, var(--qt-muted) 28%, transparent);
  background-clip: padding-box;
}

.keyword-highlight-import-textarea::-webkit-scrollbar-thumb:hover {
  background: color-mix(in srgb, var(--qt-text) 42%, transparent);
  background-clip: padding-box;
}

.keyword-highlight-import-textarea::-webkit-scrollbar-corner {
  background: transparent;
}

.keyword-highlight-import-hint {
  margin: 0;
  color: var(--qt-muted);
}

@media (max-width: 900px) {
  .keyword-highlight-rules-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .keyword-highlight-rule-head {
    display: none;
  }

  .keyword-highlight-rule-row {
    grid-template-columns: 20px 30px minmax(0, 1fr) 30px;
    gap: 8px;
    padding: 8px 0;
  }

  .keyword-highlight-rule-row > :nth-child(3),
  .keyword-highlight-rule-row > :nth-child(4),
  .keyword-highlight-rule-row > :nth-child(5) {
    grid-column: 3;
  }

  .keyword-highlight-rule-row > :nth-child(6) {
    grid-column: 4;
    grid-row: 1;
  }
}

:global(.keyword-highlight-rule-ghost) {
  display: grid !important;
  min-height: 52px !important;
  opacity: 0.42 !important;
  background: color-mix(in srgb, var(--qt-accent) 14%, var(--qt-panel)) !important;
  border: 1px dashed var(--qt-accent) !important;
  visibility: visible !important;
}

:global(.keyword-highlight-rule-chosen) {
  background: color-mix(in srgb, var(--qt-accent) 8%, transparent) !important;
}

:global(.keyword-highlight-rule-drag) {
  cursor: grabbing !important;
  box-shadow: 0 8px 22px rgb(0 0 0 / 18%) !important;
}

:global(.keyword-highlight-rule-dragging) {
  cursor: grabbing !important;
  box-shadow: 0 10px 24px rgb(0 0 0 / 22%) !important;
  border: 1px solid color-mix(in srgb, var(--qt-accent) 45%, var(--qt-border)) !important;
}
</style>
