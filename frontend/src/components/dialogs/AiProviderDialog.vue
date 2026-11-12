<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import type { Component } from 'vue'
import { QtermButton, QtermFormMessage, QtermInput, QtermSecretInput, RefreshIconButton } from '../../components/common'
import { QtermCombobox } from '../../components/common/form-controls'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { useI18n } from '../../locales'
import { toast } from 'vue-sonner'
import {
  aiErrorMessage,
  previewProviderConnection,
  previewProviderModels,
  providerKeyConfigured,
  saveProviderKey,
} from '../../services/ai'
import { translateErrorKey } from '../../services/errors'
import {
  AI_PROVIDER_PRESETS,
  createProviderFromPreset,
  readProviders,
  writeProviders,
  type AIProvider,
} from '../../services/aiProviders'
import { ProviderType } from '../../../bindings/changeme/internal/ai/models'
import { presetLogo } from '../../features/ai/providerLogos'

const props = defineProps<{ open: boolean; provider: AIProvider | null }>()
const emit = defineEmits<{ close: []; saved: [provider: AIProvider] }>()

const { t } = useI18n()

const draft = ref<AIProvider>(createProviderFromPreset(AI_PROVIDER_PRESETS[0]!))
const apiKey = ref('')
const keyConfigured = ref(false)
const busy = ref(false)
const refreshing = ref(false)
const error = ref('')

const testReply = ref('')
const testError = ref('')
const testing = ref(false)
let dialogVersion = 0
let previewVersion = 0
let cancelPreview: (() => void) | undefined
function resetPreview(): void {
  previewVersion += 1
  cancelPreview?.()
  cancelPreview = undefined
  testing.value = false
  refreshing.value = false
  testReply.value = ''
  testError.value = ''
}
function close(): void {
  if (busy.value) return
  resetPreview()
  apiKey.value = ''
  emit('close')
}
onBeforeUnmount(() => {
  dialogVersion += 1
  resetPreview()
  apiKey.value = ''
})

const presetOptions = computed(() =>
  AI_PROVIDER_PRESETS.map((preset) => ({
    value: preset.id,
    label: preset.label,

    ...(presetLogo(preset.id) ? { icon: presetLogo(preset.id) as Component } : {}),
  })),
)

const typeOptions = computed(() => [
  { value: ProviderType.ProviderTypeOpenAI, label: t('settings.ai.providerType.openai') },
  { value: ProviderType.ProviderTypeOpenAIResponse, label: t('settings.ai.providerType.openai-response') },
  { value: ProviderType.ProviderTypeAnthropic, label: t('settings.ai.providerType.anthropic') },
])

watch(
  () => props.open,
  async (open) => {
    const version = ++dialogVersion
    resetPreview()
    apiKey.value = ''
    if (!open) return

    const editing = props.provider
    draft.value = editing
      ? {
          ...editing,
          preset: AI_PROVIDER_PRESETS.some((item) => item.id === editing.preset) ? editing.preset : 'custom',
          models: [...editing.models],
        }
      : createProviderFromPreset(AI_PROVIDER_PRESETS[0]!)
    apiKey.value = ''
    error.value = ''
    busy.value = false
    keyConfigured.value = false
    if (props.provider) {
      const configured = await providerKeyConfigured(props.provider.id)
      if (version === dialogVersion && props.open) keyConfigured.value = configured
    }
  },
)

function previewSignature(): string {
  return JSON.stringify([
    draft.value.id,
    draft.value.type,
    draft.value.baseUrl.trim(),
    draft.value.model,
    apiKey.value.trim(),
  ])
}

watch(previewSignature, resetPreview, { flush: 'sync' })

function applyPreset(presetID: string): void {
  const preset = AI_PROVIDER_PRESETS.find((item) => item.id === presetID)
  if (!preset) return
  if (preset.id === 'custom') {
    draft.value = { ...draft.value, preset: 'custom', name: '', baseUrl: '', models: [], model: '' }
    return
  }
  draft.value = {
    ...draft.value,
    preset: preset.id,
    name: preset.label,
    type: preset.type,
    baseUrl: preset.baseUrl,
    models: [],
    model: '',
  }
}

async function refreshModels(): Promise<void> {
  if (refreshing.value) return
  refreshing.value = true
  error.value = ''
  const version = ++previewVersion
  try {
    const models = await previewProviderModels({
      providerId: draft.value.id,
      type: draft.value.type,
      baseUrl: draft.value.baseUrl,
      model: draft.value.model,
      draftKey: apiKey.value.trim(),
    })
    if (version !== previewVersion || !props.open) return
    const merged = Array.from(new Set([...models, ...draft.value.models]))
    draft.value = { ...draft.value, models: merged }

    if (merged.length && (!draft.value.model || !merged.includes(draft.value.model))) {
      draft.value = { ...draft.value, model: merged[0]! }
    }
    toast.success(t('settings.ai.modelsRefreshed', { count: models.length }))
  } catch (cause) {
    if (version !== previewVersion || !props.open) return
    error.value = aiErrorMessage(cause, t('settings.ai.modelsFailed'))
  } finally {
    if (version === previewVersion) refreshing.value = false
  }
}

function addModelRow(): void {
  draft.value = { ...draft.value, models: [...draft.value.models, ''] }
}

function updateModelRow(index: number, value: string): void {
  const models = draft.value.models.map((item, position) => (position === index ? value : item))

  const previous = draft.value.models[index]
  const model = draft.value.model === previous ? value : draft.value.model
  draft.value = { ...draft.value, models, model }
}

function removeModelRow(index: number): void {
  const removed = draft.value.models[index]
  const models = draft.value.models.filter((_, position) => position !== index)
  draft.value = { ...draft.value, models, model: draft.value.model === removed ? (models[0] ?? '') : draft.value.model }
}

const selectableModels = computed(() => {
  const unique = draft.value.models.map((item) => item.trim()).filter(Boolean)
  return Array.from(new Set(unique))
})

async function testConnection(): Promise<void> {
  if (testing.value) return
  testing.value = true
  testReply.value = ''
  testError.value = ''
  const version = ++previewVersion
  try {
    const request = previewProviderConnection({
      providerId: draft.value.id,
      type: draft.value.type,
      baseUrl: draft.value.baseUrl,
      model: draft.value.model,
      draftKey: apiKey.value.trim(),
    })
    cancelPreview = request.cancel
    const outcome = await request.promise
    if (version !== previewVersion || !props.open) return
    if (outcome.errorKey) {
      const base = translateErrorKey(outcome.errorKey, t('settings.ai.testFailed'))
      throw new Error(outcome.errorDetail ? `${base}（${outcome.errorDetail}）` : base)
    }
    testReply.value = outcome.reply.slice(0, 200) || t('settings.ai.testEmpty')
  } catch (cause) {
    if (version !== previewVersion || !props.open) return
    testError.value = aiErrorMessage(cause, t('settings.ai.testFailed'))
  } finally {
    if (version === previewVersion) {
      testing.value = false
      cancelPreview = undefined
    }
  }
}

async function save(): Promise<void> {
  if (busy.value || testing.value || refreshing.value) return
  if (!draft.value.name.trim() || !draft.value.baseUrl.trim() || !draft.value.model) {
    error.value = t('settings.ai.requiredFields')
    return
  }
  const version = dialogVersion
  const saved: AIProvider = {
    ...draft.value,
    name: draft.value.name.trim(),
    baseUrl: draft.value.baseUrl.trim(),
    models: [...draft.value.models],
  }
  const key = apiKey.value.trim()
  busy.value = true
  error.value = ''
  try {
    if (key) await saveProviderKey(saved.id, key)

    if (version !== dialogVersion || !props.open) return
    const existing = readProviders()
    if (!existing.some((provider) => provider.id === saved.id) && existing.length === 0) {
      writeProviders([...existing, saved])
    }
    emit('saved', saved)
  } catch (cause) {
    if (version === dialogVersion && props.open) error.value = aiErrorMessage(cause, t('settings.ai.saveFailed'))
  } finally {
    if (version === dialogVersion) busy.value = false
  }
}
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => !open && close()">
    <QtermDialogContent v-if="props.open" data-ai-provider-dialog class="max-w-[560px]">
      <QtermDialogHeader
        :title="props.provider ? t('settings.ai.editProvider') : t('settings.ai.addProvider')"
        :description="t('settings.ai.providerDialogHint')"
        :show-close="false"
      />
      <form class="dialog-body flex flex-col gap-3" @submit.prevent="save">
        <fieldset :disabled="busy" class="m-0 flex min-w-0 flex-col gap-3 border-0 p-0">
          <div class="grid grid-cols-2 gap-3">
            <label class="flex flex-col gap-1 text-xs text-qt-muted">
              {{ t('settings.ai.providerName') }}
              <QtermInput
                v-model="draft.name"
                :placeholder="t('settings.ai.providerNamePlaceholder')"
                autocomplete="off"
              />
            </label>
            <label class="flex flex-col gap-1 text-xs text-qt-muted">
              {{ t('settings.ai.preset') }}
              <QtermCombobox
                :model-value="draft.preset"
                :options="presetOptions"
                v-bind="{ ariaLabel: t('settings.ai.preset') }"
                @update:model-value="applyPreset(String($event))"
              />
            </label>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <label class="flex flex-col gap-1 text-xs text-qt-muted">
              {{ t('settings.ai.providerTypeLabel') }}
              <QtermCombobox
                :model-value="draft.type"
                :options="typeOptions"
                v-bind="{ ariaLabel: t('settings.ai.providerTypeLabel') }"
                @update:model-value="draft = { ...draft, type: String($event) as AIProvider['type'] }"
              />
            </label>
            <label class="flex flex-col gap-1 text-xs text-qt-muted">
              {{ t('settings.ai.currentModel') }}
              <QtermCombobox
                :model-value="draft.model"
                :options="selectableModels.map((model) => ({ value: model, label: model }))"
                :placeholder="t('settings.ai.modelPlaceholder')"
                v-bind="{ ariaLabel: t('settings.ai.currentModel') }"
                @update:model-value="draft = { ...draft, model: String($event) }"
              />
            </label>
          </div>
          <label class="flex flex-col gap-1 text-xs text-qt-muted">
            {{ t('settings.ai.baseUrlLabel') }}
            <QtermInput
              v-model="draft.baseUrl"
              :placeholder="t('settings.ai.baseUrlPlaceholder')"
              autocomplete="off"
              spellcheck="false"
            />
            <span class="text-[11px] text-qt-muted">{{ t('settings.ai.baseUrlHelp') }}</span>
          </label>
          <label class="flex flex-col gap-1 text-xs text-qt-muted">
            {{ t('settings.ai.apiKeyLabel') }}
            <QtermSecretInput
              id="settings-ai-provider-key"
              v-model="apiKey"
              :placeholder="keyConfigured ? t('settings.ai.apiKeyConfigured') : t('settings.ai.apiKeyPlaceholder')"
              autocomplete="off"
            />
            <span class="text-xs leading-4 text-qt-muted" data-ai-key-draft-notice>
              {{ t('settings.ai.keyImmediateSaveNotice') }}
            </span>
          </label>

          <div class="flex flex-col gap-1 text-xs text-qt-muted">
            <span class="flex items-center gap-2">
              {{ t('settings.ai.modelList') }}
              <RefreshIconButton
                class="ms-auto"
                :label="t('settings.ai.refreshModels')"
                :spinning="refreshing"
                :disabled="refreshing || testing || busy"
                data-ai-model-refresh
                @click="refreshModels"
              />
            </span>
            <div class="mt-[6px] flex flex-col gap-[8px]">
              <div
                v-for="(model, index) in draft.models"
                :key="index"
                class="flex items-center gap-[8px]"
                data-ai-model-row
              >
                <QtermInput
                  :model-value="model"
                  autocomplete="off"
                  autocorrect="off"
                  autocapitalize="off"
                  spellcheck="false"
                  :placeholder="t('settings.ai.modelPlaceholder')"
                  :aria-label="t('settings.ai.modelName')"
                  @update:model-value="(value) => updateModelRow(index, String(value))"
                />
                <QtermButton
                  variant="outline"
                  type="button"
                  class="h-8! min-h-8! text-xs"
                  :aria-label="t('settings.ai.removeModelRow')"
                  @click="removeModelRow(index)"
                >
                  {{ t('settings.ai.removeModelRow') }}
                </QtermButton>
              </div>
              <QtermButton variant="outline" type="button" class="text-xs" @click="addModelRow">
                {{ t('settings.ai.addModel') }}
              </QtermButton>
            </div>
          </div>
          <p
            v-if="testReply"
            class="m-0 rounded-md border-d bg-qt-subtle px-3 py-2 text-xs text-qt-text"
            data-ai-test-reply
          >
            {{ t('settings.ai.testReply', { reply: testReply }) }}
          </p>
          <QtermFormMessage v-if="testError" :message="testError" tone="error" />
          <QtermFormMessage v-if="error" :message="error" tone="error" />
        </fieldset>
      </form>
      <QtermDialogFooter>
        <div></div>
        <div class="dialog-footer-actions flex items-center gap-[8px]">
          <QtermButton variant="outline" :disabled="busy" @click="close">{{ t('settings.ai.cancel') }}</QtermButton>
          <QtermButton
            variant="outline"
            data-ai-provider-test
            :disabled="busy || testing || refreshing"
            @click="testConnection"
            >{{ testing ? t('settings.ai.testing') : t('settings.ai.test') }}</QtermButton
          >
          <QtermButton data-ai-provider-save :disabled="busy || testing || refreshing" @click="save">{{
            t('settings.ai.save')
          }}</QtermButton>
        </div>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>
