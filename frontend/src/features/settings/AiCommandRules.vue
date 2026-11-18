<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Download, Pencil, Plus, ShieldAlert, ShieldCheck, Trash2, Upload } from '@lucide/vue'
import {
  IconButton,
  QtermButton,
  QtermEmptyState,
  QtermFormMessage,
  QtermInput,
  QtermSettingsSectionHeader,
  QtermTextarea,
  SegmentedTabs,
} from '../../components/common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { useI18n } from '../../locales'
import { toast } from '../../services/toast'
import {
  MAX_AI_COMMAND_RULES,
  createAICommandRule,
  deleteAICommandRule,
  exportAICommandRules,
  importAICommandRules,
  loadAICommandRules,
  updateAICommandRule,
  validateRulePattern,
  type AICommandRule,
  type AICommandRuleAction,
} from '../../services/aiCommandRules'

const { t } = useI18n()

const rules = ref<AICommandRule[]>([])
const loading = ref(false)
const busy = ref(false)
const error = ref('')

const editorOpen = ref(false)
const editingID = ref('')
const pattern = ref('')
const action = ref<AICommandRuleAction>('confirm')
const note = ref('')

const actionOptions = computed(() => [
  { value: 'confirm', label: t('shell.aiRules.action.confirm') },
  { value: 'deny', label: t('shell.aiRules.action.deny') },
])

const patternInvalid = computed(() => pattern.value.trim() !== '' && !validateRulePattern(pattern.value))
const canSubmit = computed(() => validateRulePattern(pattern.value) && !busy.value)

const limitReached = computed(() => rules.value.length >= MAX_AI_COMMAND_RULES)

function actionLabel(value: AICommandRuleAction): string {
  return value === 'deny' ? t('shell.aiRules.action.deny') : t('shell.aiRules.action.confirm')
}

function actionClass(value: AICommandRuleAction): string {
  return value === 'deny'
    ? 'border-qt-danger/50 bg-qt-danger/10 text-qt-danger'
    : 'border-qt-warning/50 bg-qt-warning/10 text-qt-warning'
}

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    rules.value = await loadAICommandRules()
  } catch (loadError) {
    error.value = describeError(loadError)
  } finally {
    loading.value = false
  }
}

function describeError(cause: unknown): string {
  const message = cause instanceof Error ? cause.message : String(cause ?? '')
  return message.trim() === '' ? t('shell.aiRules.error.generic') : message
}

function resetEditor(): void {
  editingID.value = ''
  pattern.value = ''
  action.value = 'confirm'
  note.value = ''
}

function openCreate(): void {
  if (limitReached.value) {
    toast.error(t('shell.aiRules.error.tooMany'))
    return
  }
  resetEditor()
  editorOpen.value = true
}

function openEdit(rule: AICommandRule): void {
  editingID.value = rule.id
  pattern.value = rule.pattern
  action.value = rule.action
  note.value = rule.note
  editorOpen.value = true
}

async function submit(): Promise<void> {
  if (!canSubmit.value) {
    return
  }
  busy.value = true
  error.value = ''
  const input = { pattern: pattern.value.trim(), action: action.value, note: note.value.trim() }
  try {
    rules.value = editingID.value ? await updateAICommandRule(editingID.value, input) : await createAICommandRule(input)
    editorOpen.value = false
    resetEditor()
  } catch (submitError) {
    error.value = describeError(submitError)
  } finally {
    busy.value = false
  }
}

async function remove(rule: AICommandRule): Promise<void> {
  busy.value = true
  error.value = ''
  try {
    rules.value = await deleteAICommandRule(rule.id)
  } catch (removeError) {
    error.value = describeError(removeError)
  } finally {
    busy.value = false
  }
}

async function importRules(): Promise<void> {
  busy.value = true
  error.value = ''
  try {
    rules.value = await importAICommandRules()
  } catch (importError) {
    error.value = describeError(importError)
  } finally {
    busy.value = false
  }
}

async function exportRules(): Promise<void> {
  busy.value = true
  error.value = ''
  try {
    await exportAICommandRules()
  } catch (exportError) {
    error.value = describeError(exportError)
  } finally {
    busy.value = false
  }
}

onMounted(() => {
  void load()
})
</script>

<template>
  <div class="flex flex-col gap-2" data-ai-command-rules>
    <QtermSettingsSectionHeader
      class="flex-cb gap-3"
      :label="t('shell.aiRules.title')"
      :description="t('shell.aiRules.description')"
    >
      <template #actions>
        <div class="flex items-center gap-2">
          <QtermButton
            size="sm"
            class="text-xs"
            type="button"
            :disabled="busy"
            data-ai-rules-import
            @click="importRules"
          >
            <Upload :size="14" />{{ t('shell.aiRules.import') }}
          </QtermButton>
          <QtermButton
            size="sm"
            class="text-xs"
            type="button"
            variant="outline"
            :disabled="busy || rules.length === 0"
            data-ai-rules-export
            @click="exportRules"
          >
            <Download :size="14" />{{ t('shell.aiRules.export') }}
          </QtermButton>
          <QtermButton
            size="sm"
            class="text-xs"
            type="button"
            :disabled="busy"
            data-ai-rules-create
            @click="openCreate"
          >
            <Plus :size="14" />{{ t('shell.aiRules.create') }}
          </QtermButton>
        </div>
      </template>
    </QtermSettingsSectionHeader>

    <QtermFormMessage v-if="error" variant="error">{{ error }}</QtermFormMessage>

    <p class="m-0 text-xs text-qt-muted" data-ai-rules-hint>{{ t('shell.aiRules.matchingHint') }}</p>

    <p v-if="loading" class="m-0 text-xs text-qt-muted">{{ t('shell.aiRules.loading') }}</p>
    <QtermEmptyState
      v-else-if="rules.length === 0"
      variant="outlined"
      :icon="ShieldCheck"
      :title="t('shell.aiRules.emptyTitle')"
      :description="t('shell.aiRules.empty')"
      data-ai-rules-empty
    >
      <QtermButton
        size="sm"
        class="mt-1 text-xs"
        type="button"
        :disabled="busy"
        data-ai-rules-empty-create
        @click="openCreate"
      >
        <Plus :size="14" />{{ t('shell.aiRules.create') }}
      </QtermButton>
    </QtermEmptyState>

    <div v-else class="flex flex-col gap-2" data-ai-rules-list>
      <div
        v-for="rule in rules"
        :key="rule.id"
        class="flex-cb gap-3 rounded-md border border-qt-border px-3 py-2.5"
        data-ai-rules-row
      >
        <div class="flex min-w-0 flex-col gap-1">
          <code class="min-w-0 truncate font-mono text-xs text-qt-text" data-ai-rules-pattern>{{ rule.pattern }}</code>
          <span v-if="rule.note" class="min-w-0 truncate text-xs text-qt-muted">{{ rule.note }}</span>
        </div>
        <div class="flex shrink-0 items-center gap-2">
          <span
            class="inline-flex items-center gap-1 rounded-full border border-solid px-2 py-0.5 text-xs font-medium"
            :class="actionClass(rule.action)"
            data-ai-rules-action
          >
            <ShieldAlert v-if="rule.action === 'deny'" :size="12" />
            <ShieldCheck v-else :size="12" />
            {{ actionLabel(rule.action) }}
          </span>

          <span class="inline-flex items-center">
            <IconButton :label="t('shell.aiRules.edit')" size="icon-sm" :disabled="busy" @click="openEdit(rule)">
              <Pencil :size="14" />
            </IconButton>
            <IconButton :label="t('shell.aiRules.remove')" size="icon-sm" :disabled="busy" @click="remove(rule)">
              <Trash2 :size="14" />
            </IconButton>
          </span>
        </div>
      </div>
    </div>

    <QtermDialog v-model:open="editorOpen">
      <QtermDialogContent data-ai-rules-dialog>
        <QtermDialogHeader
          :title="editingID ? t('shell.aiRules.editTitle') : t('shell.aiRules.createTitle')"
          :description="t('shell.aiRules.dialogHint')"
          @close="editorOpen = false"
        />
        <form class="dialog-body flex flex-col gap-3" @submit.prevent="submit">
          <label class="flex flex-col gap-1 text-xs">
            <span class="text-qt-muted">{{ t('shell.aiRules.field.pattern') }}</span>
            <QtermInput v-model="pattern" name="pattern" placeholder="^rm\s+-rf\s+/" data-ai-rules-input-pattern />
            <span class="text-xs" :class="patternInvalid ? 'text-qt-danger' : 'text-qt-muted'">
              {{ patternInvalid ? t('shell.aiRules.error.invalidPattern') : t('shell.aiRules.field.patternHelp') }}
            </span>
          </label>

          <div class="flex flex-col gap-1 text-xs">
            <span class="text-qt-muted">{{ t('shell.aiRules.field.action') }}</span>
            <SegmentedTabs
              :model-value="action"
              :options="actionOptions"
              :label="t('shell.aiRules.field.action')"
              data-ai-rules-input-action
              @update:model-value="(value) => (action = value as AICommandRuleAction)"
            />
          </div>

          <label class="flex flex-col gap-1 text-xs">
            <span class="text-qt-muted">{{ t('shell.aiRules.field.note') }}</span>
            <QtermTextarea v-model="note" name="note" :rows="2" data-ai-rules-input-note />
          </label>
        </form>
        <QtermDialogFooter>
          <QtermButton variant="outline" type="button" :disabled="busy" @click="editorOpen = false">
            {{ t('shell.aiRules.cancel') }}
          </QtermButton>
          <QtermButton type="button" :disabled="!canSubmit" data-ai-rules-submit @click="submit">
            {{ t('shell.aiRules.save') }}
          </QtermButton>
        </QtermDialogFooter>
      </QtermDialogContent>
    </QtermDialog>
  </div>
</template>
