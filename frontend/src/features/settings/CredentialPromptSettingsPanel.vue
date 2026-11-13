<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import { Loader2, Pencil, Plus, Trash2 } from '@lucide/vue'
import {
  IconButton,
  QtermButton,
  QtermEmptyState,
  QtermFormMessage,
  QtermInput,
  QtermSwitch,
  SegmentedTabs,
} from '../../components/common'
import QtermSearchToolbar from '../../components/common/layout/QtermSearchToolbar.vue'
import QtermLoadMore from '../../components/common/layout/QtermLoadMore.vue'
import { Label, ScrollArea } from '../../components/common/control-primitives'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import DeleteConfirmDialog from '../../components/dialogs/DeleteConfirmDialog.vue'
import { errorMessage } from '@/services/errors.ts'
import type {
  CredentialPromptRuleDraft,
  CredentialPromptRuleGateway,
  CredentialPromptRuleRecord,
} from '@/services/credentialPromptRules.ts'
import { isValidPromptPattern, SUDO_PASSWORD_PROMPT_PATTERN } from '../sessions/credentialAutofill'
import { useI18n } from '@/locales'

const props = defineProps<{ gateway: CredentialPromptRuleGateway }>()
const emit = defineEmits<{ changed: [] }>()
const { t } = useI18n()

const rules = ref<CredentialPromptRuleRecord[]>([])
const searchQuery = ref('')

const loading = ref(true)
const saving = ref(false)
const error = ref('')

const loadError = ref('')
const editorOpen = ref(false)
const editingID = ref<string | null>(null)
const deleteTarget = ref<CredentialPromptRuleRecord | null>(null)
const deleteSaving = ref(false)
const deleteError = ref('')
const draft = ref<CredentialPromptRuleDraft>({ kind: 'password', pattern: '', enabled: true })
const RULE_PAGE_SIZE = 20
const visibleRuleCount = ref(RULE_PAGE_SIZE)

const draftInvalid = computed(() => draft.value.pattern.trim().length > 0 && !isValidPromptPattern(draft.value.pattern))

const loadErrorDetail = computed(() =>
  loadError.value === t('settings.credentialPrompt.error.load') ? '' : loadError.value,
)
const canSave = computed(() => draft.value.pattern.trim().length > 0 && !draftInvalid.value && !saving.value)
const kindOptions = computed(() => [
  { value: 'password', label: t('settings.credentialPrompt.kind.password') },
  { value: 'username', label: t('settings.credentialPrompt.kind.username') },
])

function kindLabel(kind: CredentialPromptRuleRecord['kind']): string {
  return t(kind === 'username' ? 'settings.credentialPrompt.kind.username' : 'settings.credentialPrompt.kind.password')
}

const filteredRules = computed(() => {
  const query = searchQuery.value.trim().toLocaleLowerCase()
  if (!query) return rules.value
  return rules.value.filter((rule) => `${kindLabel(rule.kind)}\n${rule.pattern}`.toLocaleLowerCase().includes(query))
})
const visibleRules = computed(() => filteredRules.value.slice(0, visibleRuleCount.value))

function loadMoreRules(): void {
  visibleRuleCount.value = Math.min(filteredRules.value.length, visibleRuleCount.value + RULE_PAGE_SIZE)
}

async function loadRules(): Promise<void> {
  loading.value = true
  error.value = ''
  loadError.value = ''
  try {
    rules.value = await props.gateway.list()
    visibleRuleCount.value = RULE_PAGE_SIZE
  } catch (cause) {
    loadError.value = errorMessage(cause, t('settings.credentialPrompt.error.load'))
  } finally {
    loading.value = false
  }
}

function resetDraft(): void {
  editingID.value = null
  draft.value = { kind: 'password', pattern: '', enabled: true }
}

function useSudoExample(): void {
  draft.value = { ...draft.value, kind: 'password', pattern: SUDO_PASSWORD_PROMPT_PATTERN }
  error.value = ''
}

function openCreateDialog(): void {
  if (saving.value) return
  resetDraft()
  error.value = ''
  deleteTarget.value = null
  deleteError.value = ''
  editorOpen.value = true
}

function closeEditor(): void {
  if (saving.value) return
  editorOpen.value = false
  resetDraft()
  error.value = ''
}

function startEdit(rule: CredentialPromptRuleRecord): void {
  if (saving.value) return
  editingID.value = rule.id
  deleteTarget.value = null
  deleteError.value = ''
  error.value = ''
  draft.value = { kind: rule.kind, pattern: rule.pattern, enabled: rule.enabled }
  editorOpen.value = true
}

async function saveDraft(): Promise<void> {
  if (!canSave.value) return
  const pattern = draft.value.pattern.trim()
  if (!pattern) {
    error.value = t('settings.credentialPrompt.patternRequired')
    return
  }
  if (!isValidPromptPattern(pattern)) {
    error.value = t('settings.credentialPrompt.patternInvalid')
    return
  }
  saving.value = true
  error.value = ''
  const input: CredentialPromptRuleDraft = {
    kind: draft.value.kind,
    pattern,

    enabled: editingID.value ? draft.value.enabled : true,
  }
  try {
    if (editingID.value) {
      await props.gateway.update(editingID.value, input)
      toast.success(t('settings.credentialPrompt.toast.updated'))
    } else {
      await props.gateway.create(input)
      toast.success(t('settings.credentialPrompt.toast.created'))
    }
    editorOpen.value = false
    resetDraft()
    await loadRules()
    emit('changed')
  } catch (saveError) {
    error.value = errorMessage(saveError, t('settings.credentialPrompt.error.save'))
  } finally {
    saving.value = false
  }
}

async function toggleEnabled(rule: CredentialPromptRuleRecord, enabled: boolean): Promise<void> {
  try {
    const updated = await props.gateway.update(rule.id, {
      kind: rule.kind,
      pattern: rule.pattern,
      enabled,
    })
    rules.value = rules.value.map((item) => (item.id === rule.id ? updated : item))
    emit('changed')
  } catch (toggleError) {
    error.value = errorMessage(toggleError, t('settings.credentialPrompt.error.save'))
  }
}

function requestRemove(rule: CredentialPromptRuleRecord): void {
  deleteTarget.value = rule
  deleteError.value = ''
}

function closeDelete(): void {
  if (deleteSaving.value) return
  deleteTarget.value = null
  deleteError.value = ''
}

async function removeRule(): Promise<void> {
  const target = deleteTarget.value
  if (!target || deleteSaving.value) return
  deleteSaving.value = true
  deleteError.value = ''
  try {
    await props.gateway.remove(target.id)
    if (editingID.value === target.id) {
      editorOpen.value = false
      resetDraft()
    }
    deleteTarget.value = null
    toast.success(t('settings.credentialPrompt.toast.removed'))
    await loadRules()
    emit('changed')
  } catch (removeError) {
    deleteError.value = errorMessage(removeError, t('settings.credentialPrompt.error.remove'))
  } finally {
    deleteSaving.value = false
  }
}

onMounted(() => {
  void loadRules()
})

watch(searchQuery, () => {
  visibleRuleCount.value = RULE_PAGE_SIZE
})
</script>

<template>
  <section class="flex h-full min-h-0 flex-col gap-4" :aria-label="t('settings.credentialPrompt.panel.aria')">
    <QtermSearchToolbar
      v-model="searchQuery"
      :placeholder="t('settings.credentialPrompt.search.placeholder')"
      :search-aria-label="t('settings.credentialPrompt.search.aria')"
      :clear-aria-label="t('settings.credentialPrompt.search.clear')"
    >
      <QtermButton size="sm" type="button" @click="openCreateDialog">
        <Plus :size="14" aria-hidden="true" /> {{ t('settings.credentialPrompt.add') }}
      </QtermButton>
    </QtermSearchToolbar>

    <QtermFormMessage v-if="error && !editorOpen && !deleteTarget" variant="error">{{ error }}</QtermFormMessage>

    <QtermEmptyState
      v-if="loading"
      variant="outlined"
      role="status"
      busy
      :icon="Loader2"
      icon-class="spin"
      :description="t('settings.credentialPrompt.loading')"
    />

    <QtermEmptyState
      v-else-if="loadError"
      variant="outlined"
      role="alert"
      :title="t('settings.credentialPrompt.error.load')"
      :description="loadErrorDetail"
    />

    <QtermEmptyState v-else-if="!rules.length" variant="outlined" :description="t('settings.credentialPrompt.empty')">
      <template #icon>
        <div class="grid size-[34px] place-items-center rounded-[9px] bg-qt-subtle">
          <Plus :size="18" />
        </div>
      </template>
    </QtermEmptyState>
    <QtermEmptyState
      v-else-if="!filteredRules.length"
      variant="outlined"
      :description="t('settings.credentialPrompt.empty.matching')"
    />
    <ScrollArea v-else type="hover" :scroll-hide-delay="600" class="min-h-0 flex-1">
      <div class="pr-2 pb-1">
        <ul class="m-0 flex list-none flex-col gap-2 p-0">
          <li
            v-for="rule in visibleRules"
            :key="rule.id"
            class="settings-card flex-cb gap-4 bg-qt-subtle px-3 py-2.5 border border-solid border-qt-border rounded-[8px] [background:color-mix(in_srgb,_var(--qt-panel)_32%,_transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
          >
            <span class="grid min-w-0 flex-1 gap-1">
              <strong class="truncate text-sm font-semibold">{{ kindLabel(rule.kind) }}</strong>
              <code
                class="block min-w-0 overflow-hidden text-ellipsis whitespace-nowrap bg-transparent p-0 font-mono text-xs leading-5 text-qt-muted"
                :title="rule.pattern"
              >
                {{ rule.pattern }}
              </code>
            </span>
            <span class="inline-flex shrink-0 items-center justify-end gap-1.5">
              <span class="text-xs" :class="rule.enabled ? 'text-qt-text' : 'text-qt-muted'">
                {{ t(rule.enabled ? 'settings.credentialPrompt.enabled' : 'settings.credentialPrompt.disabled') }}
              </span>
              <QtermSwitch
                :model-value="rule.enabled"
                :aria-label="t(rule.enabled ? 'settings.credentialPrompt.disable' : 'settings.credentialPrompt.enable')"
                @update:model-value="(value) => void toggleEnabled(rule, value)"
              />
              <QtermButton
                variant="ghost"
                size="icon-sm"
                type="button"
                :aria-label="t('settings.credentialPrompt.edit')"
                @click="startEdit(rule)"
              >
                <Pencil :size="14" aria-hidden="true" />
              </QtermButton>
              <IconButton
                danger
                size="icon-sm"
                :label="t('settings.credentialPrompt.remove')"
                @click="requestRemove(rule)"
              >
                <Trash2 :size="14" aria-hidden="true" />
              </IconButton>
            </span>
          </li>
        </ul>
        <QtermLoadMore
          :visible="visibleRules.length"
          :total="filteredRules.length"
          :load-more-label="t('settings.credentialPrompt.pagination.loadMore')"
          @load-more="loadMoreRules"
        />
      </div>
    </ScrollArea>

    <QtermDialog :open="editorOpen" @update:open="(open) => !open && closeEditor()">
      <QtermDialogContent v-if="editorOpen" class="qterm-command-dialog w-[calc(100vw-2rem)] !max-w-[480px]">
        <form @submit.prevent="saveDraft">
          <QtermDialogHeader
            :title="
              t(
                editingID
                  ? 'settings.credentialPrompt.dialog.editTitle'
                  : 'settings.credentialPrompt.dialog.createTitle',
              )
            "
            :description="t('settings.credentialPrompt.dialog.description')"
            :close-disabled="saving"
            @close="closeEditor"
          />
          <div class="dialog-body flex-col">
            <div class="flex flex-col gap-1.5">
              <SegmentedTabs
                :model-value="draft.kind"
                :options="kindOptions"
                :label="t('settings.credentialPrompt.kind')"
                @update:model-value="(value) => (draft.kind = value === 'username' ? 'username' : 'password')"
              />
            </div>
            <label class="flex flex-col gap-1.5" for="credential-prompt-pattern">
              <Label for="settings-field-label"
                >{{ t('settings.credentialPrompt.pattern') }} <span class="required-mark text-qt-danger">*</span></Label
              >
              <QtermInput
                id="credential-prompt-pattern"
                v-model="draft.pattern"
                autocomplete="off"
                autocorrect="off"
                autocapitalize="off"
                spellcheck="false"
                :placeholder="t('settings.credentialPrompt.patternPlaceholder')"
                autofocus
              />
              <QtermFormMessage v-if="draftInvalid" variant="error">{{
                t('settings.credentialPrompt.patternInvalid')
              }}</QtermFormMessage>
              <QtermFormMessage v-else>{{ t('settings.credentialPrompt.patternHint') }}</QtermFormMessage>
            </label>
            <div class="flex items-center">
              <QtermButton variant="outline" size="sm" type="button" :disabled="saving" @click="useSudoExample">
                {{ t('settings.credentialPrompt.useSudoExample') }}
              </QtermButton>
            </div>
            <QtermFormMessage v-if="error" variant="error">{{ error }}</QtermFormMessage>
          </div>
          <QtermDialogFooter>
            <QtermButton variant="outline" type="button" :disabled="saving" @click="closeEditor">
              {{ t('shell.common.cancel') }}
            </QtermButton>
            <QtermButton type="submit" :disabled="!canSave">
              {{ t(saving ? 'settings.credentialPrompt.saving' : 'settings.credentialPrompt.save') }}
            </QtermButton>
          </QtermDialogFooter>
        </form>
      </QtermDialogContent>
    </QtermDialog>

    <DeleteConfirmDialog
      :open="!!deleteTarget"
      :title="t('settings.credentialPrompt.deleteTitle')"
      :message="t('settings.credentialPrompt.deleteMessage', { pattern: deleteTarget?.pattern ?? '' })"
      :error="deleteError"
      :loading="deleteSaving"
      :confirm-text="t('settings.credentialPrompt.removeConfirm')"
      :confirming-text="t('settings.credentialPrompt.removing')"
      @close="closeDelete"
      @confirm="removeRule"
    />
  </section>
</template>
