<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { QtermButton, QtermInput, QtermNumberField, QtermSettingsRow, QtermSwitch } from '@/components/common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '@/components/common/dialog'
import { useI18n } from '@/locales'
import { decodeAppError } from '../../services/errors'
import { useAppLock } from './useAppLock'

const { t } = useI18n()
const {
  enabled,
  autoLockMinutes,
  status,
  lock,
  enable,
  changePassword,
  disable,
  setAutoLockMinutes,
  setTouchIdEnabled,
  refresh,
} = useAppLock()

const touchIdRowVisible = computed(() => enabled.value && status.value.touchIdAvailable)
const touchIdWorking = ref(false)

type DialogMode = 'enable' | 'change' | 'disable'
const dialog = ref<DialogMode | null>(null)
const currentPassword = ref('')
const nextPassword = ref('')
const confirmPassword = ref('')
const recoveryQuestion = ref('')
const recoveryAnswer = ref('')
const minutesDraft = ref(0)
const working = ref(false)
const error = ref('')

const dialogTitle = computed(() => {
  if (dialog.value === 'enable') return t('applock.dialog.enableTitle')
  if (dialog.value === 'change') return t('applock.dialog.changeTitle')
  return t('applock.dialog.disableTitle')
})

function resetDraft(): void {
  currentPassword.value = ''
  nextPassword.value = ''
  confirmPassword.value = ''

  minutesDraft.value = autoLockMinutes.value
  recoveryQuestion.value = ''
  recoveryAnswer.value = ''
  error.value = ''
}

function openDialog(mode: DialogMode): void {
  resetDraft()
  dialog.value = mode
}

function closeDialog(): void {
  dialog.value = null
  resetDraft()
}

async function handleToggle(next: boolean): Promise<void> {
  if (next === enabled.value) return
  if (next) {
    await refresh()
    if (enabled.value) {
      openDialog('change')
      error.value = t('applock.error.alreadyEnabled')
      return
    }
    openDialog('enable')
    return
  }
  openDialog('disable')
}

async function submit(): Promise<void> {
  if (working.value) return
  error.value = ''
  if (dialog.value !== 'disable') {
    if (nextPassword.value.length < 4) {
      error.value = t('applock.error.tooShort')
      return
    }
    if (nextPassword.value !== confirmPassword.value) {
      error.value = t('applock.error.mismatch')
      return
    }

    if ((recoveryQuestion.value.trim() === '') !== (recoveryAnswer.value.trim() === '')) {
      error.value = t('applock.error.incompleteRecovery')
      return
    }
  }
  working.value = true
  try {
    if (dialog.value === 'enable') {
      await enable(nextPassword.value, minutesDraft.value, recoveryQuestion.value, recoveryAnswer.value)
    } else if (dialog.value === 'change') {
      await changePassword(currentPassword.value, nextPassword.value)
    } else if (dialog.value === 'disable') {
      await disable(currentPassword.value)
    }
    closeDialog()
  } catch {
    await refresh().catch(() => undefined)
    if (dialog.value === 'enable' && enabled.value) {
      dialog.value = 'change'
      error.value = t('applock.error.alreadyEnabled')
    } else {
      error.value = t('applock.error.failed')
    }
  } finally {
    working.value = false
  }
}

onMounted(() => {
  void refresh()
})

async function updateTouchId(next: boolean): Promise<void> {
  if (touchIdWorking.value || next === status.value.touchIdEnabled) return
  touchIdWorking.value = true
  error.value = ''
  try {
    await setTouchIdEnabled(next)
  } catch (reason) {
    const appError = decodeAppError(reason)
    if (appError?.messageKey === 'applock.touchIdUnavailable') {
      error.value = t('applock.touchId.unavailable')
    } else if (appError?.messageKey === 'applock.notEnabled') {
      error.value = t('applock.error.notEnabled')
    } else {
      error.value = t('applock.error.failed')
    }

    await refresh().catch(() => undefined)
  } finally {
    touchIdWorking.value = false
  }
}

async function updateAutoLock(minutes: number): Promise<void> {
  if (!Number.isFinite(minutes) || minutes < 0) return
  try {
    await setAutoLockMinutes(minutes)
  } catch {
    error.value = t('applock.error.failed')
  }
}
</script>

<template>
  <div
    class="settings-card border border-solid border-qt-border rounded-[8px] [background:color-mix(in_srgb,_var(--qt-panel)_32%,_transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
  >
    <QtermSettingsRow
      class="settings-switch-row c-p"
      :label="t('applock.enable.label')"
      :description="t('applock.enable.help')"
    >
      <QtermSwitch
        id="settings-app-lock-enabled"
        :model-value="enabled"
        :aria-label="t('applock.enable.label')"
        @update:model-value="handleToggle"
      />
    </QtermSettingsRow>

    <template v-if="enabled">
      <QtermSettingsRow :label="t('applock.autoLock.label')" :description="t('applock.autoLock.help')">
        <QtermNumberField
          id="settings-app-lock-autolock"
          class="w-32"
          :model-value="autoLockMinutes"
          :min="0"
          :max="1440"
          :step="1"
          :aria-label="t('applock.autoLock.label')"
          :decrease-label="t('applock.autoLock.decrease')"
          :increase-label="t('applock.autoLock.increase')"
          @update:model-value="(value) => value !== undefined && updateAutoLock(value)"
        />
      </QtermSettingsRow>

      <QtermSettingsRow
        v-if="touchIdRowVisible"
        class="settings-switch-row c-p"
        :label="t('applock.touchId.label')"
        :description="t('applock.touchId.help')"
      >
        <QtermSwitch
          id="settings-app-lock-touch-id"
          :model-value="status.touchIdEnabled"
          :aria-label="t('applock.touchId.label')"
          @update:model-value="updateTouchId"
        />
      </QtermSettingsRow>

      <QtermSettingsRow :label="t('applock.actions.label')" :description="t('applock.actions.help')">
        <div class="flex flex-wrap items-center gap-2">
          <QtermButton size="sm" variant="outline" type="button" @click="lock()">
            {{ t('applock.actions.lockNow') }}
          </QtermButton>
          <QtermButton size="sm" variant="outline" type="button" @click="openDialog('change')">
            {{ t('applock.actions.changePassword') }}
          </QtermButton>
          <QtermButton size="sm" variant="ghost" type="button" @click="openDialog('disable')">
            {{ t('applock.actions.disable') }}
          </QtermButton>
        </div>
      </QtermSettingsRow>
    </template>
  </div>

  <QtermDialog :open="dialog !== null" @update:open="(open) => !open && closeDialog()">
    <QtermDialogContent v-if="dialog">
      <QtermDialogHeader :title="dialogTitle" :close-disabled="working" @close="closeDialog" />
      <div class="dialog-body flex-col">
        <QtermInput
          v-if="dialog !== 'enable'"
          id="applock-current-password"
          v-model="currentPassword"
          type="password"
          autocomplete="current-password"
          :aria-label="t('applock.dialog.currentPassword')"
          :placeholder="t('applock.dialog.currentPassword')"
          :disabled="working"
        />
        <template v-if="dialog !== 'disable'">
          <QtermInput
            id="applock-next-password"
            v-model="nextPassword"
            type="password"
            autocomplete="new-password"
            :aria-label="t('applock.dialog.newPassword')"
            :placeholder="t('applock.dialog.newPassword')"
            :disabled="working"
          />
          <QtermInput
            id="applock-confirm-password"
            v-model="confirmPassword"
            type="password"
            autocomplete="new-password"
            :aria-label="t('applock.dialog.confirmPassword')"
            :placeholder="t('applock.dialog.confirmPassword')"
            :disabled="working"
          />
        </template>
        <template v-if="dialog === 'enable'">
          <p class="m-0 text-xs text-qt-muted">{{ t('applock.dialog.recoveryHint') }}</p>
          <QtermInput
            id="applock-recovery-question"
            v-model="recoveryQuestion"
            :aria-label="t('applock.dialog.recoveryQuestion')"
            :placeholder="t('applock.dialog.recoveryQuestion')"
            :disabled="working"
          />
          <QtermInput
            id="applock-recovery-answer"
            v-model="recoveryAnswer"
            :aria-label="t('applock.dialog.recoveryAnswer')"
            :placeholder="t('applock.dialog.recoveryAnswer')"
            :disabled="working"
          />
        </template>
        <div v-if="dialog === 'enable'" class="flex-cb gap-3">
          <span class="text-xs">{{ t('applock.autoLock.label') }}</span>
          <QtermNumberField
            class="w-32"
            :model-value="minutesDraft"
            :min="0"
            :max="1440"
            :step="1"
            :aria-label="t('applock.autoLock.label')"
            :decrease-label="t('applock.autoLock.decrease')"
            :increase-label="t('applock.autoLock.increase')"
            @update:model-value="(value) => value !== undefined && (minutesDraft = value)"
          />
        </div>
        <p v-if="error" class="m-0 text-xs text-qt-danger" role="alert">{{ error }}</p>
      </div>
      <QtermDialogFooter>
        <QtermButton variant="outline" type="button" :disabled="working" @click="closeDialog">
          {{ t('applock.dialog.cancel') }}
        </QtermButton>
        <QtermButton :disabled="working" type="button" @click="submit">
          {{ t('applock.dialog.confirm') }}
        </QtermButton>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>
