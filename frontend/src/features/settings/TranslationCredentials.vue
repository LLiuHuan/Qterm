<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Check, Save, Trash2 } from '@lucide/vue'
import { toast } from 'vue-sonner'
import { QtermButton, QtermInput, QtermSecretInput, QtermSettingsRow } from '../../components/common'
import {
  createTranslateCredentialGateway,
  type TranslateCredentialProvider,
  type TranslateProviderCredentialStatus,
} from '../../services/translate'
import { useI18n } from '@/locales'

const props = defineProps<{ provider: TranslateCredentialProvider }>()
const { t } = useI18n()

const gateway = createTranslateCredentialGateway()
const statuses = ref<TranslateProviderCredentialStatus[]>([])
const loading = ref(false)
const saving = ref(false)
const requestSerial = ref(0)
const apiKey = ref('')
const appId = ref('')
const appKey = ref('')

const providerLabel = computed(() => {
  switch (props.provider) {
    case 'deepl':
      return t('terminal.core.translateProviderDeepL')
    case 'baidu':
      return t('terminal.core.translateProviderBaidu')
    case 'ali':
      return t('terminal.core.translateProviderAli')
    case 'youdao':
      return t('terminal.core.translateProviderYoudao')
    default:
      return props.provider
  }
})

const currentStatus = computed(() => statuses.value.find((item) => item.provider === props.provider))
const configured = computed(() => currentStatus.value?.configured === true)
const hasInput = computed(() => {
  if (props.provider === 'deepl') return apiKey.value.trim().length > 0
  return appId.value.trim().length > 0 && appKey.value.trim().length > 0
})

function clearInputs(): void {
  apiKey.value = ''
  appId.value = ''
  appKey.value = ''
}

async function loadStatus(): Promise<void> {
  const provider = props.provider
  const serial = ++requestSerial.value
  clearInputs()
  saving.value = false
  loading.value = true
  try {
    const next = await gateway.list()
    if (serial === requestSerial.value && provider === props.provider) statuses.value = next
  } catch {
    if (serial === requestSerial.value && provider === props.provider) {
      statuses.value = []
      toast.error(t('terminal.core.translateCredentialStatusReadFailed'))
    }
  } finally {
    if (serial === requestSerial.value && provider === props.provider) loading.value = false
  }
}

async function save(): Promise<void> {
  if (!hasInput.value || saving.value) return
  const provider = props.provider
  const label = providerLabel.value
  const serial = ++requestSerial.value
  const input = {
    provider,
    apiKey: provider === 'deepl' ? apiKey.value : undefined,
    appId: provider === 'deepl' ? undefined : appId.value,
    appKey: provider === 'deepl' ? undefined : appKey.value,
  }
  loading.value = false
  saving.value = true
  try {
    await gateway.save(input)
    if (serial !== requestSerial.value || provider !== props.provider) return
    statuses.value = statuses.value.filter((item) => item.provider !== provider).concat({ provider, configured: true })
    clearInputs()
    toast.success(t('terminal.core.translateCredentialSaved', { provider: label }))
  } catch {
    if (serial === requestSerial.value && provider === props.provider) {
      toast.error(t('terminal.core.translateCredentialSaveFailed', { provider: label }))
    }
  } finally {
    if (serial === requestSerial.value && provider === props.provider) saving.value = false
  }
}

async function remove(): Promise<void> {
  if (!configured.value || saving.value) return
  const provider = props.provider
  const label = providerLabel.value
  const serial = ++requestSerial.value
  loading.value = false
  saving.value = true
  try {
    await gateway.remove(provider)
    if (serial !== requestSerial.value || provider !== props.provider) return
    statuses.value = statuses.value.filter((item) => item.provider !== provider).concat({ provider, configured: false })
    clearInputs()
    toast.success(t('terminal.core.translateCredentialCleared', { provider: label }))
  } catch {
    if (serial === requestSerial.value && provider === props.provider) {
      toast.error(t('terminal.core.translateCredentialClearFailed', { provider: label }))
    }
  } finally {
    if (serial === requestSerial.value && provider === props.provider) saving.value = false
  }
}

watch(() => props.provider, loadStatus, { immediate: true })
</script>

<template>
  <QtermSettingsRow
    :label="t('terminal.core.translateCredentialTitle', { provider: providerLabel })"
    :description="
      configured
        ? t('terminal.core.translateCredentialConfiguredHelp')
        : t('terminal.core.translateCredentialNotConfiguredHelp')
    "
  >
    <span class="inline-flex items-center gap-1 text-xs text-qt-muted" :aria-busy="loading">
      <Check v-if="configured" :size="14" aria-hidden="true" />
      {{
        loading
          ? t('terminal.core.translateCredentialLoading')
          : configured
            ? t('terminal.core.translateCredentialConfigured')
            : t('terminal.core.translateCredentialNotConfigured')
      }}
    </span>
  </QtermSettingsRow>

  <QtermSettingsRow
    v-if="props.provider === 'deepl'"
    layout="stacked"
    :label="t('terminal.core.translateCredentialApiKey')"
  >
    <QtermSecretInput
      id="translation-deepl-api-key"
      class="w-full"
      input-class="font-mono text-xs"
      :model-value="apiKey"
      :placeholder="t('terminal.core.translateCredentialApiKeyPlaceholder', { provider: providerLabel })"
      autocomplete="off"
      @update:model-value="(value) => (apiKey = value)"
    />
  </QtermSettingsRow>

  <template v-else>
    <QtermSettingsRow layout="stacked" :label="t('terminal.core.translateCredentialAppId')">
      <QtermInput
        :id="`translation-${props.provider}-app-id`"
        class="w-full font-mono text-xs"
        :model-value="appId"
        :placeholder="
          props.provider === 'ali'
            ? t('terminal.core.translateCredentialAliAppIdPlaceholder')
            : t('terminal.core.translateCredentialAppIdPlaceholder', { provider: providerLabel })
        "
        autocomplete="off"
        spellcheck="false"
        @update:model-value="(value) => (appId = String(value))"
      />
    </QtermSettingsRow>
    <QtermSettingsRow layout="stacked" :label="t('terminal.core.translateCredentialAppKey')">
      <QtermSecretInput
        :id="`translation-${props.provider}-app-key`"
        class="w-full"
        input-class="font-mono text-xs"
        :model-value="appKey"
        :placeholder="
          props.provider === 'ali'
            ? t('terminal.core.translateCredentialAliAppKeyPlaceholder')
            : t('terminal.core.translateCredentialAppKeyPlaceholder', { provider: providerLabel })
        "
        autocomplete="off"
        @update:model-value="(value) => (appKey = value)"
      />
    </QtermSettingsRow>
  </template>

  <div class="flex items-center justify-end gap-2 px-[14px] py-[10px]">
    <QtermButton variant="outline" size="sm" type="button" :disabled="!hasInput || saving" @click="save">
      <Save :size="14" aria-hidden="true" />
      {{ t('terminal.core.translateCredentialSave') }}
    </QtermButton>
    <QtermButton variant="ghost" size="sm" type="button" :disabled="!configured || saving" @click="remove">
      <Trash2 :size="14" aria-hidden="true" />
      {{ t('terminal.core.translateCredentialClear') }}
    </QtermButton>
  </div>
</template>
