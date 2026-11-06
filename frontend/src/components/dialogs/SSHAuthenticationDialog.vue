<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { KeyRound, LockKeyhole } from 'lucide'
import { QtermButton, QtermInput, QtermFormMessage, QtermSecretInput, QtermSwitch, SegmentedTabs } from '../common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../common/dialog'
import { QtermCombobox, type QtermComboboxOption } from '../common/form-controls'
import type { HostRecord } from '../../services/hosts'
import type { CredentialRecord } from '../../services/credentials'
import type { SessionAuthenticationPending } from '../../types/qterm'
import type { SessionAuthenticationInput } from '../../services/sessions'
import type { AuthenticationRetryState } from '../../composables/useSessionState'
import { createOtpGateway, type OtpEntry } from '../../services/otp'
import { useI18n } from '@/locales'

const props = withDefaults(
  defineProps<{
    pending: SessionAuthenticationPending | null
    otpEntries?: OtpEntry[]
    hosts?: HostRecord[]
    credentials: CredentialRecord[]
    credentialLabel: (credential: CredentialRecord) => string
    submitting: boolean

    canRemember?: boolean

    retry?: AuthenticationRetryState | null
  }>(),
  { canRemember: true, otpEntries: () => [], hosts: () => [], retry: null },
)
const emit = defineEmits<{ submit: [input: SessionAuthenticationInput]; cancel: [] }>()
const { t } = useI18n()
const mode = ref<'password' | 'credential'>('password')
const secret = ref('')
const credentialId = ref('')

const credentialOptions = computed<QtermComboboxOption[]>(() =>
  props.credentials.map((credential) => ({
    value: credential.id,
    label: `${credential.name} · ${props.credentialLabel(credential)}`,
    icon: credentialIcon(credential),
  })),
)
const remember = ref(false)
const answers = ref<string[]>([])
const selectedOtp = ref('')
const otpError = ref('')
const fillingOtp = ref(false)
let fillEpoch = 0
let disposed = false

let lastAuthHostId = ''
const otpGateway = createOtpGateway()
const interactive = computed(() => props.pending?.challenge.kind === 'keyboard-interactive')
const questions = computed(() => props.pending?.challenge.questions ?? [])
const otpOptions = computed(() =>
  (props.otpEntries ?? [])
    .filter((entry) => entry.type === 'totp')
    .map((entry) => ({ value: entry.id, label: [entry.name, entry.username].filter(Boolean).join(' · ') })),
)

const dialogVisible = computed(() => !!props.pending || !!props.retry)

const retrying = computed(() => props.retry?.retrying === true)

const exhausted = computed(() => props.retry?.exhausted === true)
const authError = computed(() => props.retry?.error ?? '')
const inputsDisabled = computed(() => props.submitting || retrying.value || exhausted.value)

const errorCleared = ref(false)
const showErrorBorder = computed(() => !!props.retry?.error && !errorCleared.value)
const passwordInputClass = computed(() =>
  showErrorBorder.value
    ? 'h-qt-control-height border-qt-danger focus-visible:border-qt-danger focus-visible:ring-qt-danger/25'
    : 'h-qt-control-height',
)

const summaryHost = computed(() => {
  const hostId = props.pending?.challenge.hostId || props.retry?.hostId || ''
  return props.hosts.find((item) => item.id === hostId) ?? null
})
const instruction = computed(() => props.pending?.challenge.instruction ?? '')
const summaryAddress = computed(() => props.pending?.challenge.address || summaryHost.value?.address || '')
const summaryUsername = computed(() => props.pending?.challenge.username || summaryHost.value?.username || '')

const bodyRoot = ref<HTMLElement | null>(null)

const canSubmit = computed(() =>
  interactive.value
    ? answers.value.length === questions.value.length && answers.value.every((answer) => answer.length > 0)
    : mode.value === 'password'
      ? !!secret.value
      : !!credentialId.value,
)

async function fillOtp(index: number): Promise<void> {
  const entry = props.otpEntries?.find((item) => item.id === selectedOtp.value && item.type === 'totp')
  const pending = props.pending
  if (!entry || !pending || fillingOtp.value || props.submitting) return
  const epoch = ++fillEpoch
  fillingOtp.value = true
  otpError.value = ''
  try {
    const result = await otpGateway.generate(entry)
    if (
      disposed ||
      epoch !== fillEpoch ||
      selectedOtp.value !== entry.id ||
      props.pending?.challenge.requestId !== pending.challenge.requestId ||
      props.pending?.sessionId !== pending.sessionId
    )
      return
    answers.value[index] = result.code
  } catch {
    if (!disposed && epoch === fillEpoch && props.pending?.challenge.requestId === pending.challenge.requestId)
      otpError.value = t('shell.otp.generateError')
  } finally {
    if (epoch === fillEpoch) fillingOtp.value = false
  }
}

function credentialIcon(credential: CredentialRecord) {
  return credential.kind === 'password' ? LockKeyhole : KeyRound
}

watch(
  () => `${props.pending?.sessionId ?? ''}:${props.pending?.challenge.requestId ?? ''}`,
  () => {
    ++fillEpoch
    fillingOtp.value = false

    const nextHostId = props.pending?.challenge.hostId || props.retry?.hostId || ''
    if (nextHostId !== lastAuthHostId) {
      mode.value = 'password'
      secret.value = ''
      credentialId.value = ''
      remember.value = false
    }
    lastAuthHostId = nextHostId
    answers.value = questions.value.map(() => '')
    selectedOtp.value = ''
    otpError.value = ''
    const challenge = props.pending?.challenge
    const host = props.hosts.find((item) => item.id === (challenge?.hopHostId || challenge?.hostId))
    if (!interactive.value || !host?.sshTwoFactorEnabled) return
    const entry = props.otpEntries.find((item) => item.id === host.sshOtpEntryId && item.type === 'totp')
    if (!entry) return
    selectedOtp.value = entry.id
    if (!host.sshOtpAutoFill) return

    const index = questions.value.findIndex((question) =>
      /^(?:verification code|authentication code|authenticator code|one[ -]time (?:password|code)|otp(?: code)?|totp(?: code)?|验证码|动态验证码|动态口令)\s*[:：]?$/i.test(
        question.prompt.trim(),
      ),
    )
    if (index >= 0) void fillOtp(index)
  },
  { immediate: true },
)

function focusPasswordInput(): void {
  void nextTick(() => {
    const element = bodyRoot.value?.querySelector<HTMLInputElement>('#ssh-auth-password')
    element?.focus()
    element?.select()
  })
}

watch(
  () => `${props.pending?.sessionId ?? ''}:${props.pending?.challenge.requestId ?? ''}`,
  () => {
    if (props.pending && !interactive.value) focusPasswordInput()
  },
)

watch(
  () => props.retry?.attempt,
  () => {
    errorCleared.value = false
  },
)
watch([secret, credentialId], () => {
  if (props.retry?.error) errorCleared.value = true
})

onBeforeUnmount(() => {
  disposed = true
  ++fillEpoch
})

function submit(): void {
  if (!canSubmit.value || props.submitting || fillingOtp.value) return
  if (interactive.value) {
    emit('submit', {
      secret: '',
      credentialId: '',
      remember: false,
      requestId: props.pending?.challenge.requestId,
      answers: [...answers.value],
    })
    answers.value = questions.value.map(() => '')
    return
  }
  if (mode.value === 'credential') {
    if (!credentialId.value) return
    emit('submit', {
      secret: '',
      credentialId: credentialId.value,
      remember: remember.value,
      requestId: props.pending?.challenge.requestId,
    })
    return
  }
  if (!secret.value) return
  emit('submit', {
    secret: secret.value,
    credentialId: '',
    remember: remember.value,
    requestId: props.pending?.challenge.requestId,
  })
}
</script>

<template>
  <QtermDialog :open="dialogVisible" @update:open="(open) => !open && !props.submitting && !retrying && emit('cancel')">
    <QtermDialogContent
      ref="bodyRoot"
      v-if="dialogVisible"
      class="ssh-auth-dialog"
      size="md"
      :aria-busy="props.submitting"
    >
      <QtermDialogHeader
        :title="t('shell.authDialog.title')"
        :description="t('shell.authDialog.description')"
        :close-disabled="props.submitting || retrying"
        @close="emit('cancel')"
      />
      <div ref="bodyRoot" class="dialog-body ssh-auth-body">
        <dl class="ssh-auth-summary">
          <div>
            <dt>{{ t('shell.authDialog.host') }}</dt>
            <dd>
              <code>{{ summaryAddress }}</code>
            </dd>
          </div>
          <div>
            <dt>{{ t('shell.authDialog.username') }}</dt>
            <dd>
              <code>{{ summaryUsername }}</code>
            </dd>
          </div>
        </dl>

        <QtermFormMessage v-if="authError" variant="error" role="alert" data-testid="ssh-auth-error">{{
          authError
        }}</QtermFormMessage>
        <p v-if="retrying" class="m-0 text-xs text-qt-muted" aria-live="polite">
          {{ t('shell.authDialog.verifying') }}
        </p>
        <template v-if="exhausted">
          <p class="m-0 text-sm text-qt-muted">{{ t('shell.authDialog.exhaustedHint') }}</p>
        </template>
        <template v-else-if="interactive">
          <p v-if="instruction" class="whitespace-pre-wrap break-words text-sm text-qt-muted">
            {{ instruction }}
          </p>
          <div v-if="otpOptions.length && questions.length" class="grid gap-2 rounded-md border-d p-3">
            <label class="text-xs text-qt-muted">{{ t('shell.otp.selectForAuth') }}</label>
            <QtermCombobox
              v-model="selectedOtp"
              :options="otpOptions"
              :aria-label="t('shell.otp.selectForAuth')"
              :placeholder="t('shell.otp.selectForAuth')"
            />
            <p class="text-xs text-qt-muted">{{ t('shell.otp.fillHint') }}</p>
          </div>
          <div v-for="(question, index) in questions" :key="index" class="grid min-w-0 gap-2">
            <label :for="`ssh-auth-answer-${index}`" class="whitespace-pre-wrap break-words text-sm">{{
              question.prompt || t('shell.otp.response')
            }}</label>
            <QtermInput
              v-if="question.echo"
              :id="`ssh-auth-answer-${index}`"
              v-model="answers[index]"
              autocomplete="off"
              :disabled="inputsDisabled"
            />
            <QtermSecretInput
              v-else
              :id="`ssh-auth-answer-${index}`"
              v-model="answers[index]"
              autocomplete="off"
              :disabled="inputsDisabled"
            />
            <QtermButton
              v-if="otpOptions.length"
              class="justify-self-start"
              size="sm"
              variant="outline"
              :disabled="!selectedOtp || fillingOtp || inputsDisabled"
              @click="fillOtp(index)"
              >{{ t('shell.otp.fillCode') }}</QtermButton
            >
          </div>
          <QtermFormMessage v-if="otpError" variant="error">{{ otpError }}</QtermFormMessage>
        </template>

        <template v-if="!exhausted">
          <SegmentedTabs
            v-if="!interactive"
            :model-value="mode"
            :options="[
              { value: 'password', label: t('shell.authDialog.password') },
              { value: 'credential', label: t('shell.authDialog.credential') },
            ]"
            :label="t('shell.authDialog.method')"
            @update:model-value="(value) => (mode = value as 'password' | 'credential')"
          />
          <div v-if="!interactive && mode === 'password'" class="form-field min-w-0 grid gap-[6px] text-qt-muted">
            <label class="text-sm font-medium" for="ssh-auth-password">{{ t('shell.hostEditor.password') }}</label>

            <QtermSecretInput
              id="ssh-auth-password"
              v-model="secret"
              :input-class="passwordInputClass"
              :invalid="showErrorBorder"
              autocomplete="current-password"
              autofocus
              :disabled="inputsDisabled"
              :placeholder="t('shell.authDialog.passwordPlaceholder')"
            />
          </div>
          <div v-else-if="!interactive" class="form-field min-w-0 grid gap-[6px] text-qt-muted">
            <label class="text-sm font-medium" for="ssh-auth-credential">{{
              t('shell.authDialog.credentialField')
            }}</label>
            <QtermCombobox
              id="ssh-auth-credential"
              v-model="credentialId"
              :class="showErrorBorder ? '[&_[data-slot=input-group]]:border-qt-danger' : ''"
              :options="credentialOptions"
              :disabled="!props.credentials.length || inputsDisabled"
              :placeholder="t('shell.authDialog.selectCredential')"
              :aria-label="t('shell.authDialog.credentialField')"
              :empty-text="t('settings.credentials.empty.matching')"
            />
            <QtermFormMessage v-if="!props.credentials.length" variant="hint">
              {{ t('shell.authDialog.noCredentials') }}
            </QtermFormMessage>
          </div>
          <label v-if="props.canRemember && !interactive" class="settings-toggle-row ssh-auth-remember">
            <span
              ><strong>{{ t('shell.authDialog.remember') }}</strong
              ><small class="text-xs">{{ t('shell.authDialog.rememberHint') }}</small></span
            >
            <QtermSwitch v-model="remember" :aria-label="t('shell.authDialog.remember')" />
          </label>
        </template>
      </div>
      <QtermDialogFooter>
        <QtermButton variant="outline" :disabled="props.submitting" @click="emit('cancel')">{{
          t('shell.authDialog.cancel')
        }}</QtermButton>
        <QtermButton
          v-if="!exhausted"
          type="button"
          :disabled="inputsDisabled || fillingOtp || !canSubmit"
          @click="submit"
        >
          {{
            props.submitting
              ? t('shell.common.connecting')
              : retrying
                ? t('shell.authDialog.verifying')
                : t('shell.authDialog.submit')
          }}
        </QtermButton>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>

<style scoped>
.ssh-auth-dialog {
  width: min(500px, calc(100vw - 32px));
  max-width: min(500px, calc(100vw - 32px)) !important;
}
.ssh-auth-summary {
  display: grid;
  gap: 0.65rem;
  margin: 0;
  padding: 0.75rem;
  border: 1px solid var(--qt-border);
  border-radius: 0.6rem;
  background: var(--qt-subtle);
}
.ssh-auth-summary div {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
}
.ssh-auth-summary dt {
  color: var(--qt-muted);
}
.ssh-auth-summary dd {
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}
.ssh-auth-remember {
  margin-top: 0.5rem;
}
</style>
