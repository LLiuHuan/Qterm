<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import type { CSSProperties } from 'vue'
import { Fingerprint, LockKeyhole } from '@lucide/vue'
import { QtermButton, QtermInput } from '@/components/common'
import { useI18n } from '@/locales'
import { decodeAppError } from '../../services/errors'
import { useAppLock } from './useAppLock'

const props = defineProps<{ themeStyle?: CSSProperties }>()

const { t } = useI18n()
const { unlock, unlockWithTouchId, resetByRecovery, status, touchIdUnlockAvailable } = useAppLock()

const password = ref('')
const error = ref('')
const working = ref(false)

const recovering = ref(false)
const answer = ref('')
const nextPassword = ref('')
const confirmPassword = ref('')
const recoveryError = ref('')
const hasRecovery = computed(() => status.value.hasRecovery && status.value.recoveryQuestion !== '')
const inputRef = ref<{ focus?: () => void; $el?: HTMLElement } | null>(null)

const touchIdWorking = ref(false)
const touchIdError = ref('')

async function submit(): Promise<void> {
  if (working.value) return
  const value = password.value
  if (!value) {
    error.value = t('applock.screen.empty')
    return
  }
  working.value = true
  error.value = ''
  try {
    const ok = await unlock(value)
    if (!ok) {
      error.value = t('applock.screen.invalid')
      password.value = ''
      await focusInput()
    }
  } catch {
    error.value = t('applock.screen.failed')
  } finally {
    working.value = false
  }
}

async function focusInput(): Promise<void> {
  await nextTick()
  const element = inputRef.value?.$el
  if (element instanceof HTMLElement) {
    element.querySelector('input')?.focus()
  }
}

async function submitTouchId(): Promise<void> {
  if (touchIdWorking.value || working.value) return
  touchIdWorking.value = true
  touchIdError.value = ''
  try {
    const ok = await unlockWithTouchId()

    if (!ok) touchIdError.value = t('applock.screen.touchIdFailed')
  } catch (reason) {
    touchIdError.value = translateTouchIdError(reason)

    await focusInput()
  } finally {
    touchIdWorking.value = false
  }
}

function translateTouchIdError(reason: unknown): string {
  const key = decodeAppError(reason)?.messageKey
  switch (key) {
    case 'applock.touchIdUnavailable':
      return t('applock.screen.touchIdUnavailable')
    case 'applock.touchIdCancelled':
      return t('applock.screen.touchIdCancelled')
    case 'applock.touchIdTimeout':
      return t('applock.screen.touchIdTimeout')
    case 'applock.touchIdDisabled':
      return t('applock.screen.touchIdFailed')
    default:
      return t('applock.screen.touchIdFailed')
  }
}

function openRecovery(): void {
  recovering.value = true
  answer.value = ''
  nextPassword.value = ''
  confirmPassword.value = ''
  recoveryError.value = ''
}

function closeRecovery(): void {
  recovering.value = false
  recoveryError.value = ''
}

async function submitRecovery(): Promise<void> {
  if (working.value) return
  recoveryError.value = ''
  if (nextPassword.value.length < 4) {
    recoveryError.value = t('applock.error.tooShort')
    return
  }
  if (nextPassword.value !== confirmPassword.value) {
    recoveryError.value = t('applock.error.mismatch')
    return
  }
  working.value = true
  try {
    await resetByRecovery(answer.value, nextPassword.value)
    await unlock(nextPassword.value)
    closeRecovery()
  } catch {
    recoveryError.value = t('applock.recovery.wrongAnswer')
  } finally {
    working.value = false
  }
}

onMounted(() => {
  void focusInput()
})
</script>

<template>
  <Teleport to="body">
    <div
      class="qterm-app-lock fixed inset-0 z-[2147483647] flex-cc [background:color-mix(in_srgb,_var(--qt-theme-6,_var(--qt-app))_88%,_transparent)] [-webkit-backdrop-filter:blur(10px)_saturate(120%)] [backdrop-filter:blur(10px)_saturate(120%)]"
      role="dialog"
      aria-modal="true"
      :style="props.themeStyle"
    >
      <form v-if="recovering" class="qterm-app-lock__card flex w-80 flex-col gap-4" @submit.prevent="submitRecovery">
        <div class="flex flex-col items-center gap-2 text-center">
          <span
            class="qterm-app-lock__icon inline-flex-cc text-(--qt-text, var(--foreground)) opacity-90 text-(--qt-text, var(--foreground)) text-(--qt-text, var(--foreground))"
            aria-hidden="true"
            ><LockKeyhole :size="22" stroke-width="1.6"
          /></span>
          <h2 class="m-0 text-base font-medium">{{ t('applock.recovery.title') }}</h2>
          <p class="m-0 text-xs text-qt-muted">{{ status.recoveryQuestion }}</p>
        </div>
        <QtermInput
          id="qterm-app-lock-answer"
          v-model="answer"
          :aria-label="t('applock.recovery.answer')"
          :placeholder="t('applock.recovery.answer')"
          :disabled="working"
        />
        <QtermInput
          id="qterm-app-lock-new-password"
          v-model="nextPassword"
          type="password"
          autocomplete="new-password"
          :aria-label="t('applock.recovery.newPassword')"
          :placeholder="t('applock.recovery.newPassword')"
          :disabled="working"
        />
        <QtermInput
          id="qterm-app-lock-confirm-password"
          v-model="confirmPassword"
          type="password"
          autocomplete="new-password"
          :aria-label="t('applock.dialog.confirmPassword')"
          :placeholder="t('applock.dialog.confirmPassword')"
          :disabled="working"
        />
        <p v-if="recoveryError" class="qterm-app-lock__error m-0 text-xs text-qt-danger" role="alert">
          {{ recoveryError }}
        </p>
        <QtermButton type="submit" :disabled="working">{{ t('applock.recovery.submit') }}</QtermButton>
        <QtermButton type="button" variant="ghost" :disabled="working" @click="closeRecovery">
          {{ t('applock.recovery.back') }}
        </QtermButton>
      </form>

      <form v-else class="qterm-app-lock__card flex w-80 flex-col gap-4" @submit.prevent="submit">
        <div class="flex flex-col items-center gap-2 text-center">
          <span
            class="qterm-app-lock__icon inline-flex-cc text-(--qt-text, var(--foreground)) opacity-90 text-(--qt-text, var(--foreground)) text-(--qt-text, var(--foreground))"
            aria-hidden="true"
            ><LockKeyhole :size="20" stroke-width="1.75"
          /></span>
          <h2 class="m-0 text-base font-medium">{{ t('applock.screen.title') }}</h2>
          <p class="m-0 text-xs text-qt-muted">{{ t('applock.screen.description') }}</p>
        </div>
        <QtermInput
          id="qterm-app-lock-password"
          ref="inputRef"
          v-model="password"
          type="password"
          autocomplete="current-password"
          :aria-label="t('applock.screen.password')"
          :placeholder="t('applock.screen.password')"
          :disabled="working"
        />
        <p v-if="error" class="qterm-app-lock__error m-0 text-xs text-qt-danger" role="alert">{{ error }}</p>
        <QtermButton type="submit" :disabled="working">{{ t('applock.screen.unlock') }}</QtermButton>

        <template v-if="touchIdUnlockAvailable">
          <QtermButton type="button" variant="outline" :disabled="working || touchIdWorking" @click="submitTouchId">
            <Fingerprint :size="15" stroke-width="1.75" aria-hidden="true" />
            {{ touchIdWorking ? t('applock.screen.touchIdWaiting') : t('applock.screen.touchIdUnlock') }}
          </QtermButton>
          <p v-if="touchIdError" class="qterm-app-lock__error m-0 text-xs text-qt-danger" role="alert">
            {{ touchIdError }}
          </p>
        </template>
        <button
          v-if="hasRecovery"
          type="button"
          class="qterm-app-lock__link m-0 text-center text-[11px] leading-4"
          @click="openRecovery"
        >
          {{ t('applock.screen.forgot') }}
        </button>
      </form>
    </div>
  </Teleport>
</template>
