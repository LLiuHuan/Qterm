<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import {
  QtermButton,
  QtermFormMessage,
  QtermInput,
  QtermNumberField,
  QtermSecretInput,
  QtermSwitch,
  SegmentedTabs,
} from '../common'
import { Label } from '../common/control-primitives'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../common/dialog'
import { useI18n } from '@/locales'
import {
  proxyErrorMessage,
  type ProxyGateway,
  type ProxyInput,
  type ProxyMode,
  type ProxyRecord,
} from '../../services/proxies'

const props = withDefaults(
  defineProps<{
    open: boolean
    gateway: ProxyGateway
    proxy?: ProxyRecord | null
  }>(),
  { proxy: null },
)

const emit = defineEmits<{
  close: []
  created: [proxy: ProxyRecord]
  updated: [proxy: ProxyRecord]
}>()

const { t } = useI18n()
const saving = ref(false)
const formError = ref('')

const form = reactive<{
  name: string
  mode: ProxyMode
  host: string
  port: number
  username: string
  password: string
  clearAuthentication: boolean
}>({
  name: '',
  mode: 'socks5',
  host: '',
  port: 1080,
  username: '',
  password: '',
  clearAuthentication: false,
})

const editing = computed(() => Boolean(props.proxy?.id))
const formTitle = computed(() =>
  t(editing.value ? 'settings.proxies.dialog.editTitle' : 'settings.proxies.dialog.createTitle'),
)
const existingPasswordConfigured = computed(() => props.proxy?.passwordConfigured === true)

function resetForm(proxy: ProxyRecord | null | undefined): void {
  form.name = proxy?.name ?? ''
  form.mode = proxy?.mode ?? 'socks5'
  form.host = proxy?.host ?? ''
  form.port = proxy?.port ?? 1080
  form.username = proxy?.username ?? ''
  form.password = ''
  form.clearAuthentication = false
  formError.value = ''
}

watch(
  () => [props.open, props.proxy] as const,
  ([open]) => {
    if (open) resetForm(props.proxy)
  },
  { immediate: true },
)

const modeOptions = computed(() => [
  { value: 'socks5', label: t('shell.hostEditor.proxySocks5') },
  { value: 'http', label: t('shell.hostEditor.proxyHttp') },
])

function setPassword(value: string | number): void {
  form.password = String(value)
  if (form.password) form.clearAuthentication = false
}

function close(): void {
  if (saving.value) return
  emit('close')
}

async function submit(): Promise<void> {
  if (saving.value) return
  formError.value = ''

  const password = form.password
  saving.value = true
  const input: ProxyInput = {
    name: form.name.trim(),
    mode: form.mode,
    host: form.host.trim(),
    port: Number(form.port) || 0,
    username: form.username.trim(),

    password,
    clearPassword: form.clearAuthentication,
  }
  try {
    const result = editing.value
      ? await props.gateway.update(props.proxy!.id, input)
      : await props.gateway.create(input)
    if (editing.value) emit('updated', result)
    else emit('created', result)
    emit('close')
  } catch (error) {
    formError.value = proxyErrorMessage(error, t('settings.proxies.error.save'))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => !open && close()">
    <QtermDialogContent
      v-if="props.open"
      class="proxy-editor-dialog text-sm"
      :class="{ 'z-60': true }"
      overlay-class="z-60"
      size="lg"
    >
      <QtermDialogHeader :title="formTitle" :description="t('settings.proxies.dialog.description')" @close="close" />
      <form id="proxy-editor-form" class="dialog-body" data-proxy-editor-form @submit.prevent="submit">
        <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
          <Label for="proxy-name"
            >{{ t('settings.proxies.field.name') }} <span class="required-mark text-qt-danger">*</span></Label
          >
          <QtermInput
            id="proxy-name"
            v-model="form.name"
            autocomplete="off"
            :placeholder="t('settings.proxies.field.namePlaceholder')"
          />
          <QtermFormMessage variant="hint">{{ t('settings.proxies.field.nameHint') }}</QtermFormMessage>
        </div>

        <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
          <Label>{{ t('settings.proxies.field.mode') }}</Label>
          <SegmentedTabs
            :model-value="form.mode"
            :options="modeOptions"
            :label="t('settings.proxies.field.mode')"
            @update:model-value="(value) => (form.mode = String(value) as ProxyMode)"
          />
        </div>

        <div class="form-grid grid grid-cols-2 gap-[10px] form-grid--address grid-cols-[minmax(0,_1fr)_104px]">
          <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
            <Label for="proxy-host">{{ t('settings.proxies.field.host') }}</Label>
            <QtermInput
              id="proxy-host"
              v-model="form.host"
              autocomplete="off"
              autocorrect="off"
              autocapitalize="off"
              spellcheck="false"
              :placeholder="t('settings.proxies.field.hostPlaceholder')"
            />
          </div>
          <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
            <Label for="proxy-port">{{ t('settings.proxies.field.port') }}</Label>
            <QtermNumberField
              id="proxy-port"
              :model-value="form.port || undefined"
              name="proxyPort"
              :min="1"
              :max="65535"
              @update:model-value="(value) => (form.port = value ?? 0)"
            />
          </div>
        </div>

        <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
          <Label for="proxy-username">
            {{ t('settings.proxies.field.username') }}
            <span class="optional-label text-qt-faint font-normal">{{ t('settings.proxies.field.optional') }}</span>
          </Label>
          <QtermInput
            id="proxy-username"
            v-model="form.username"
            class="h-qt-control-height min-h-qt-control-height"
            autocomplete="off"
            autocorrect="off"
            autocapitalize="off"
            spellcheck="false"
            :placeholder="t('settings.proxies.field.usernamePlaceholder')"
          />
          <QtermFormMessage variant="hint">{{ t('settings.proxies.field.usernameHint') }}</QtermFormMessage>
        </div>

        <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
          <Label for="proxy-password">{{ t('settings.proxies.field.password') }}</Label>
          <QtermSecretInput
            id="proxy-password"
            :model-value="form.password"
            input-class="h-qt-control-height min-h-qt-control-height"
            autocomplete="new-password"
            :placeholder="t('settings.proxies.field.passwordPlaceholder')"
            :show-label="t('settings.proxies.field.passwordShow')"
            :hide-label="t('settings.proxies.field.passwordHide')"
            @update:model-value="setPassword"
          />
          <QtermFormMessage v-if="existingPasswordConfigured" variant="hint">
            {{ t('settings.proxies.field.passwordKeepHint') }}
          </QtermFormMessage>
          <label v-if="existingPasswordConfigured" class="settings-toggle-row">
            <span>
              <strong>{{ t('settings.proxies.field.passwordClear') }}</strong>
              <small class="text-xs">{{ t('settings.proxies.field.passwordClearHint') }}</small>
            </span>
            <QtermSwitch v-model="form.clearAuthentication" :aria-label="t('settings.proxies.field.passwordClear')" />
          </label>
        </div>
        <QtermFormMessage v-if="formError" variant="error" data-proxy-form-error>{{ formError }}</QtermFormMessage>
      </form>
      <QtermDialogFooter>
        <div></div>
        <div class="dialog-footer-actions flex items-center gap-[8px]">
          <QtermButton variant="outline" type="button" :disabled="saving" @click="close">
            {{ t('settings.proxies.action.cancel') }}
          </QtermButton>
          <QtermButton type="submit" form="proxy-editor-form" data-proxy-submit :disabled="saving" @click="submit">
            {{ t(saving ? 'settings.proxies.action.saveProgress' : 'settings.proxies.action.save') }}
          </QtermButton>
        </div>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>
