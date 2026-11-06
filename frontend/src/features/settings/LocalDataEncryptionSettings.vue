<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  QtermButton,
  QtermFormMessage,
  QtermInput,
  QtermSecretInput,
  QtermSettingsSectionHeader,
} from '@/components/common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '@/components/common/dialog'
import { Label } from '@/components/common/control-primitives'
import { useI18n } from '@/locales'
import type {
  ChangeEncryptionKeyInput,
  CredentialVaultStatus,
  ExportCredentialVaultKeyInput,
  ImportCredentialVaultKeyInput,
  ResetCredentialVaultInput,
} from '@/services/credentials'
import { hasNativeFileDialogs, pickRecoveryPackage, pickRecoveryPackageDestination } from '@/services/file-dialogs'

const props = withDefaults(
  defineProps<{
    status: CredentialVaultStatus | null
    working: boolean
    error?: string
    showHeader?: boolean
    showRecoveryActions?: boolean
  }>(),
  { error: '', showHeader: true, showRecoveryActions: true },
)

const emit = defineEmits<{
  changeEncryptionKey: [input: ChangeEncryptionKeyInput]
  exportRecovery: [input: ExportCredentialVaultKeyInput]
  importRecovery: [input: ImportCredentialVaultKeyInput]
  unlockCustomKey: [key: string]
  resetVault: [input: ResetCredentialVaultInput]
}>()

const { t } = useI18n()
const dialogOpen = ref(false)
const exportDialogOpen = ref(false)
const importDialogOpen = ref(false)
const unlockDialogOpen = ref(false)
const resetDialogOpen = ref(false)
const currentKey = ref('')
const newKey = ref('')
const confirmation = ref('')
const exportPath = ref('')
const exportPackagePassword = ref('')
const exportConfirmation = ref('')
const importPath = ref('')
const importPackagePassword = ref('')
const customUnlockKey = ref('')
const resetConfirmation = ref('')
const choosingPath = ref<'import' | 'export' | null>(null)
const formError = ref('')

const available = computed(() => props.status?.encryptionMode !== undefined)
const isCustom = computed(() => props.status?.encryptionMode === 'custom')
const isUnlocked = computed(() => props.status?.state === 'unlocked')
const isCustomLocked = computed(() => isCustom.value && !isUnlocked.value)
const isResettable = computed(() => ['locked', 'recoveryRequired', 'error'].includes(props.status?.state ?? ''))
const modeTitle = computed(() => t(isCustom.value ? 'security.core.customMode' : 'security.core.systemMode'))
const modeDescription = computed(() =>
  t(isCustom.value ? 'security.core.customModeDescription' : 'security.core.systemModeDescription'),
)

watch(dialogOpen, (open) => {
  if (open) return
  currentKey.value = ''
  newKey.value = ''
  confirmation.value = ''
  formError.value = ''
})

watch(exportDialogOpen, (open) => {
  if (open) return
  exportPath.value = ''
  exportPackagePassword.value = ''
  exportConfirmation.value = ''
  choosingPath.value = null
  formError.value = ''
})

watch(importDialogOpen, (open) => {
  if (open) return
  importPath.value = ''
  importPackagePassword.value = ''
  choosingPath.value = null
  formError.value = ''
})

watch(unlockDialogOpen, (open) => {
  if (open) return
  customUnlockKey.value = ''
  formError.value = ''
})

watch(resetDialogOpen, (open) => {
  if (open) return
  resetConfirmation.value = ''
  formError.value = ''
})

function openDialog(): void {
  if (props.working || !isUnlocked.value) return
  formError.value = ''
  dialogOpen.value = true
}

function closeDialog(): void {
  if (props.working) return
  dialogOpen.value = false
}

function openExportDialog(): void {
  if (props.working || !isUnlocked.value) return
  formError.value = ''
  exportDialogOpen.value = true
}

function closeExportDialog(): void {
  if (props.working) return
  exportDialogOpen.value = false
}

function openImportDialog(): void {
  if (props.working) return
  formError.value = ''
  importDialogOpen.value = true
}

function closeImportDialog(): void {
  if (props.working) return
  importDialogOpen.value = false
}

function openUnlockDialog(): void {
  if (props.working || !isCustomLocked.value) return
  formError.value = ''
  unlockDialogOpen.value = true
}

function closeUnlockDialog(): void {
  if (props.working) return
  unlockDialogOpen.value = false
}

function openResetDialog(): void {
  if (props.working || !isResettable.value) return
  formError.value = ''
  resetDialogOpen.value = true
}

function closeResetDialog(): void {
  if (props.working) return
  resetDialogOpen.value = false
}

async function chooseExportPath(): Promise<void> {
  choosingPath.value = 'export'
  try {
    const path = await pickRecoveryPackageDestination()
    if (path) exportPath.value = path
  } catch {
    formError.value = t('security.core.error.saveFile')
  } finally {
    choosingPath.value = null
  }
}

async function chooseImportPath(): Promise<void> {
  choosingPath.value = 'import'
  try {
    const path = await pickRecoveryPackage()
    if (path) importPath.value = path
  } catch {
    formError.value = t('security.core.error.chooseFile')
  } finally {
    choosingPath.value = null
  }
}

function submit(): void {
  formError.value = ''
  if (isCustom.value && !currentKey.value) {
    formError.value = t('security.core.error.currentEncryptionKey')
    return
  }
  if (newKey.value.length < 8) {
    formError.value = t('security.core.error.encryptionKeyShort')
    return
  }
  if (newKey.value !== confirmation.value) {
    formError.value = t('security.core.error.encryptionKeyMismatch')
    return
  }
  emit('changeEncryptionKey', {
    currentKey: currentKey.value,
    newKey: newKey.value,
    confirmation: confirmation.value,
  })
  dialogOpen.value = false
}

function submitExportRecovery(): void {
  formError.value = ''
  if (!exportPath.value.trim() || !exportPackagePassword.value) {
    formError.value = t('security.core.error.exportFieldsNew')
    return
  }
  if (exportPackagePassword.value.length < 8) {
    formError.value = t('security.core.error.packagePasswordShort')
    return
  }
  if (exportPackagePassword.value !== exportConfirmation.value) {
    formError.value = t('security.core.error.packagePasswordMismatch')
    return
  }
  emit('exportRecovery', {
    path: exportPath.value.trim(),
    currentPassword: '',
    packagePassword: exportPackagePassword.value,
    confirmation: exportConfirmation.value,
  })
  exportDialogOpen.value = false
}

function submitImportRecovery(): void {
  formError.value = ''
  if (!importPath.value.trim() || !importPackagePassword.value) {
    formError.value = t('security.core.error.importFieldsNew')
    return
  }
  if (importPackagePassword.value.length < 8) {
    formError.value = t('security.core.error.packagePasswordShort')
    return
  }
  emit('importRecovery', {
    path: importPath.value.trim(),
    packagePassword: importPackagePassword.value,
    masterPassword: '',
    confirmation: '',
    hint: '',
  })
  importDialogOpen.value = false
}

function submitUnlockCustomKey(): void {
  formError.value = ''
  if (!customUnlockKey.value) {
    formError.value = t('security.core.error.customKeyRequired')
    return
  }
  emit('unlockCustomKey', customUnlockKey.value)
  unlockDialogOpen.value = false
}

function submitResetVault(): void {
  formError.value = ''
  if (resetConfirmation.value !== 'RESET QTERM VAULT') {
    formError.value = t('security.core.error.confirmPhrase')
    return
  }
  emit('resetVault', { confirmation: resetConfirmation.value })
  resetDialogOpen.value = false
}
</script>

<template>
  <div v-if="available" class="flex flex-col gap-3.5">
    <QtermSettingsSectionHeader
      v-if="props.showHeader"
      :label="t('security.core.localEncryption')"
      :description="t('security.core.localEncryptionDescription')"
    />

    <div
      class="settings-card flex items-center gap-3 px-4 py-3.5 border border-solid border-qt-border rounded-[8px] [background:color-mix(in_srgb,_var(--qt-panel)_32%,_transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
      aria-live="polite"
    >
      <div class="min-w-0 grow">
        <strong class="block text-sm font-medium">{{ modeTitle }}</strong>
        <small class="mt-1 block text-xs leading-5 text-qt-muted">{{ modeDescription }}</small>
        <small v-if="props.status?.keychainAvailable === false" class="mt-1 block text-xs leading-5 text-qt-warning">
          {{ t('security.core.keychainUnavailable') }}
        </small>
      </div>
      <div class="flex shrink-0 flex-wrap items-center justify-end gap-2">
        <QtermButton
          v-if="props.showRecoveryActions"
          variant="outline"
          type="button"
          :disabled="props.working"
          @click="openImportDialog"
        >
          {{ t('security.core.importRecovery') }}
        </QtermButton>
        <QtermButton
          v-if="props.showRecoveryActions && isUnlocked"
          variant="outline"
          type="button"
          :disabled="props.working"
          @click="openExportDialog"
        >
          {{ t('security.core.exportRecovery') }}
        </QtermButton>
        <QtermButton
          v-if="isCustomLocked"
          variant="outline"
          type="button"
          :disabled="props.working"
          @click="openUnlockDialog"
        >
          {{ t('security.core.unlockData') }}
        </QtermButton>
        <QtermButton v-if="isUnlocked" variant="outline" type="button" :disabled="props.working" @click="openDialog">
          {{ t(isCustom ? 'security.core.changeEncryptionKey' : 'security.core.setCustomKey') }}
        </QtermButton>
        <QtermButton
          v-if="isResettable"
          variant="destructive"
          type="button"
          :disabled="props.working"
          @click="openResetDialog"
        >
          {{ t('security.core.reinitialize') }}
        </QtermButton>
      </div>
    </div>
    <QtermFormMessage v-if="props.error" variant="error">{{ props.error }}</QtermFormMessage>

    <QtermDialog :open="dialogOpen" @update:open="(open) => !open && closeDialog()">
      <QtermDialogContent v-if="dialogOpen" class="local-encryption-dialog z-60 text-sm" overlay-class="z-60" size="md">
        <QtermDialogHeader
          :title="t(isCustom ? 'security.core.changeEncryptionKey' : 'security.core.setCustomKey')"
          :description="t('security.core.rekeyDescription')"
          :close-disabled="props.working"
          @close="closeDialog"
        />

        <form id="local-encryption-key-form" class="dialog-body" @submit.prevent="submit">
          <div v-if="isCustom" class="form-field min-w-0 grid gap-[6px] text-qt-muted">
            <Label for="current-encryption-key">{{ t('security.core.currentEncryptionKey') }}</Label>
            <QtermSecretInput
              id="current-encryption-key"
              v-model="currentKey"
              autocomplete="current-password"
              autofocus
            />
          </div>
          <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
            <Label for="new-encryption-key">{{ t('security.core.newEncryptionKey') }}</Label>
            <QtermSecretInput
              id="new-encryption-key"
              v-model="newKey"
              autocomplete="new-password"
              :autofocus="!isCustom"
              :placeholder="t('security.core.packagePasswordPlaceholder')"
            />
          </div>
          <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
            <Label for="encryption-key-confirmation">{{ t('security.core.confirmEncryptionKey') }}</Label>
            <QtermSecretInput
              id="encryption-key-confirmation"
              v-model="confirmation"
              autocomplete="new-password"
              :placeholder="t('security.core.confirmEncryptionKey')"
            />
          </div>
          <QtermFormMessage variant="hint">{{ t('security.core.rekeyHint') }}</QtermFormMessage>
          <QtermFormMessage v-if="formError" variant="error">{{ formError }}</QtermFormMessage>
        </form>

        <QtermDialogFooter>
          <div class="dialog-footer-actions flex items-center gap-[8px] ml-auto">
            <QtermButton variant="outline" type="button" :disabled="props.working" @click="closeDialog">
              {{ t('security.core.cancel') }}
            </QtermButton>
            <QtermButton type="submit" form="local-encryption-key-form" :disabled="props.working">
              {{ props.working ? t('security.core.saving') : t('security.core.save') }}
            </QtermButton>
          </div>
        </QtermDialogFooter>
      </QtermDialogContent>
    </QtermDialog>

    <QtermDialog :open="exportDialogOpen" @update:open="(open) => !open && closeExportDialog()">
      <QtermDialogContent
        v-if="exportDialogOpen"
        class="local-encryption-dialog z-60 text-sm"
        overlay-class="z-60"
        size="md"
      >
        <QtermDialogHeader
          :title="t('security.core.exportRecovery')"
          :description="t('security.core.recoveryHint')"
          :close-disabled="props.working"
          @close="closeExportDialog"
        />

        <form id="local-encryption-export-form" class="dialog-body" @submit.prevent="submitExportRecovery">
          <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
            <Label for="local-encryption-export-path">{{ t('security.core.savePath') }}</Label>
            <div class="flex items-center gap-2">
              <QtermInput
                id="local-encryption-export-path"
                v-model="exportPath"
                class="min-w-0 flex-1"
                autocomplete="off"
                :placeholder="t('security.core.pathPlaceholder')"
                autofocus
              />
              <QtermButton
                v-if="hasNativeFileDialogs()"
                class="shrink-0"
                variant="outline"
                type="button"
                :disabled="props.working || choosingPath !== null"
                @click="chooseExportPath"
              >
                {{ choosingPath === 'export' ? t('security.core.opening') : t('security.core.chooseLocation') }}
              </QtermButton>
            </div>
          </div>
          <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
            <Label for="local-encryption-export-password">{{ t('security.core.packagePassword') }}</Label>
            <QtermSecretInput
              id="local-encryption-export-password"
              v-model="exportPackagePassword"
              autocomplete="new-password"
              :placeholder="t('security.core.packagePasswordPlaceholder')"
            />
          </div>
          <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
            <Label for="local-encryption-export-confirmation">{{ t('security.core.packagePasswordConfirm') }}</Label>
            <QtermSecretInput
              id="local-encryption-export-confirmation"
              v-model="exportConfirmation"
              autocomplete="new-password"
              :placeholder="t('security.core.packagePasswordConfirmPlaceholder')"
            />
          </div>
          <QtermFormMessage v-if="formError" variant="error">{{ formError }}</QtermFormMessage>
        </form>

        <QtermDialogFooter>
          <div class="dialog-footer-actions flex items-center gap-[8px] ml-auto">
            <QtermButton variant="outline" type="button" :disabled="props.working" @click="closeExportDialog">
              {{ t('security.core.cancel') }}
            </QtermButton>
            <QtermButton type="submit" form="local-encryption-export-form" :disabled="props.working">
              {{ props.working ? t('security.core.exporting') : t('security.core.exportRecovery') }}
            </QtermButton>
          </div>
        </QtermDialogFooter>
      </QtermDialogContent>
    </QtermDialog>

    <QtermDialog :open="importDialogOpen" @update:open="(open) => !open && closeImportDialog()">
      <QtermDialogContent
        v-if="importDialogOpen"
        class="local-encryption-dialog z-60 text-sm"
        overlay-class="z-60"
        size="md"
      >
        <QtermDialogHeader
          :title="t('security.core.importRecovery')"
          :description="t('security.core.recoveryRestoreHint')"
          :close-disabled="props.working"
          @close="closeImportDialog"
        />

        <form id="local-encryption-import-form" class="dialog-body" @submit.prevent="submitImportRecovery">
          <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
            <Label for="local-encryption-import-path">{{ t('security.core.recoveryPath') }}</Label>
            <div class="flex items-center gap-2">
              <QtermInput
                id="local-encryption-import-path"
                v-model="importPath"
                class="min-w-0 flex-1"
                autocomplete="off"
                :placeholder="t('security.core.pathPlaceholder')"
                autofocus
              />
              <QtermButton
                v-if="hasNativeFileDialogs()"
                class="shrink-0"
                variant="outline"
                type="button"
                :disabled="props.working || choosingPath !== null"
                @click="chooseImportPath"
              >
                {{ choosingPath === 'import' ? t('security.core.opening') : t('security.core.chooseFile') }}
              </QtermButton>
            </div>
          </div>
          <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
            <Label for="local-encryption-import-password">{{ t('security.core.packagePassword') }}</Label>
            <QtermSecretInput
              id="local-encryption-import-password"
              v-model="importPackagePassword"
              autocomplete="current-password"
              :placeholder="t('security.core.packagePasswordExportPlaceholder')"
            />
          </div>
          <QtermFormMessage variant="hint">{{ t('security.core.recoveryDataHint') }}</QtermFormMessage>
          <QtermFormMessage v-if="formError" variant="error">{{ formError }}</QtermFormMessage>
        </form>

        <QtermDialogFooter>
          <div class="dialog-footer-actions flex items-center gap-[8px] ml-auto">
            <QtermButton variant="outline" type="button" :disabled="props.working" @click="closeImportDialog">
              {{ t('security.core.cancel') }}
            </QtermButton>
            <QtermButton type="submit" form="local-encryption-import-form" :disabled="props.working">
              {{ props.working ? t('security.core.importing') : t('security.core.importAndUnlock') }}
            </QtermButton>
          </div>
        </QtermDialogFooter>
      </QtermDialogContent>
    </QtermDialog>

    <QtermDialog :open="unlockDialogOpen" @update:open="(open) => !open && closeUnlockDialog()">
      <QtermDialogContent
        v-if="unlockDialogOpen"
        class="local-encryption-dialog z-60 text-sm"
        overlay-class="z-60"
        size="md"
      >
        <QtermDialogHeader
          :title="t('security.core.unlockData')"
          :description="t('security.core.unlockDescription')"
          :close-disabled="props.working"
          @close="closeUnlockDialog"
        />

        <form id="local-encryption-unlock-form" class="dialog-body" @submit.prevent="submitUnlockCustomKey">
          <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
            <Label for="local-encryption-unlock-key">{{ t('security.core.customKey') }}</Label>
            <QtermSecretInput
              id="local-encryption-unlock-key"
              v-model="customUnlockKey"
              autocomplete="current-password"
              :placeholder="t('security.core.customKeyPlaceholder')"
              autofocus
            />
          </div>
          <QtermFormMessage v-if="formError" variant="error">{{ formError }}</QtermFormMessage>
        </form>

        <QtermDialogFooter>
          <div class="dialog-footer-actions flex items-center gap-[8px] ml-auto">
            <QtermButton variant="outline" type="button" :disabled="props.working" @click="closeUnlockDialog">
              {{ t('security.core.cancel') }}
            </QtermButton>
            <QtermButton type="submit" form="local-encryption-unlock-form" :disabled="props.working">
              {{ props.working ? t('security.core.unlocking') : t('security.core.unlockData') }}
            </QtermButton>
          </div>
        </QtermDialogFooter>
      </QtermDialogContent>
    </QtermDialog>

    <QtermDialog :open="resetDialogOpen" @update:open="(open) => !open && closeResetDialog()">
      <QtermDialogContent
        v-if="resetDialogOpen"
        class="local-encryption-dialog z-60 text-sm"
        overlay-class="z-60"
        size="md"
      >
        <QtermDialogHeader
          :title="t('security.core.resetVault')"
          :description="t('security.core.resetDescription')"
          :close-disabled="props.working"
          @close="closeResetDialog"
        />

        <form id="local-encryption-reset-form" class="dialog-body" @submit.prevent="submitResetVault">
          <QtermFormMessage variant="hint">{{ t('security.core.resetHint') }}</QtermFormMessage>
          <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
            <Label for="local-encryption-reset-confirmation">{{ t('security.core.confirmPhrase') }}</Label>
            <QtermInput
              id="local-encryption-reset-confirmation"
              v-model="resetConfirmation"
              autocomplete="off"
              placeholder="RESET QTERM VAULT"
              autofocus
            />
          </div>
          <QtermFormMessage variant="hint">{{
            t('security.core.resetContinue', { phrase: 'RESET QTERM VAULT' })
          }}</QtermFormMessage>
          <QtermFormMessage v-if="formError" variant="error">{{ formError }}</QtermFormMessage>
        </form>

        <QtermDialogFooter>
          <div class="dialog-footer-actions flex items-center gap-[8px] ml-auto">
            <QtermButton variant="outline" type="button" :disabled="props.working" @click="closeResetDialog">
              {{ t('security.core.cancel') }}
            </QtermButton>
            <QtermButton
              variant="destructive"
              type="submit"
              form="local-encryption-reset-form"
              :disabled="props.working"
            >
              {{ props.working ? t('security.core.resetting') : t('security.core.reinitialize') }}
            </QtermButton>
          </div>
        </QtermDialogFooter>
      </QtermDialogContent>
    </QtermDialog>
  </div>
</template>
