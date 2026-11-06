import { ref } from 'vue'
import { toast } from 'vue-sonner'
import {
  createCredentialGateway,
  credentialErrorMessage,
  sortCredentialsByCreatedAtDesc,
  type ChangeMasterPasswordInput,
  type ChangeEncryptionKeyInput,
  type CredentialRecord,
  type CredentialVaultStatus,
  type ExportCredentialVaultKeyInput,
  type ImportCredentialVaultKeyInput,
  type ResetCredentialVaultInput,
  type SetAutoUnlockInput,
  type SetupMasterPasswordInput,
} from '../services/credentials'
import { credentialStatusMessage } from '../services/errors'
import { hideSettingsWindow, isSettingsWindowMode } from '../services/sessionWindows'
import { useI18n } from '../locales'

export function useCredentialState() {
  const { t } = useI18n()
  const credentialGateway = createCredentialGateway()
  const credentialOptions = ref<CredentialRecord[]>([])
  const credentialVaultStatus = ref<CredentialVaultStatus | null>(null)
  const credentialOptionsLoading = ref(false)
  const credentialOptionsError = ref('')
  const securityDialogOpen = ref(false)
  const securityWorking = ref(false)
  const securityError = ref('')

  async function openSecuritySettings(): Promise<void> {
    securityDialogOpen.value = true
    securityError.value = ''
    await refreshSecurityStatus()
  }

  function closeSecuritySettings(): void {
    const state = credentialVaultStatus.value?.state
    if (state === 'setupRequired' || state === 'recoveryRequired') return
    securityDialogOpen.value = false
    securityError.value = ''

    if (isSettingsWindowMode()) void hideSettingsWindow()
  }

  async function refreshSecurityStatus(): Promise<void> {
    securityWorking.value = true
    securityError.value = ''
    try {
      const status = await credentialGateway.status()
      credentialVaultStatus.value = status
      credentialOptions.value =
        status.state === 'unlocked' ? sortCredentialsByCreatedAtDesc(await credentialGateway.list()) : []
      if (
        credentialGateway.persistent &&
        ['setupRequired', 'recoveryRequired', 'locked', 'error'].includes(status.state)
      ) {
        securityDialogOpen.value = true
      }
    } catch (error) {
      credentialVaultStatus.value = null
      credentialOptions.value = []
      securityError.value = credentialErrorMessage(error, t('shell.error.securityStatus'))
      if (credentialGateway.persistent) securityDialogOpen.value = true
    } finally {
      securityWorking.value = false
    }
  }

  async function reloadCredentialStateAfterFailure(): Promise<void> {
    try {
      const status = await credentialGateway.status()
      credentialVaultStatus.value = status
      credentialOptions.value =
        status.state === 'unlocked' ? sortCredentialsByCreatedAtDesc(await credentialGateway.list()) : []
      if (credentialGateway.persistent && status.state !== 'unlocked') securityDialogOpen.value = true
    } catch {
      credentialVaultStatus.value = null
      credentialOptions.value = []
      if (credentialGateway.persistent) securityDialogOpen.value = true
    }
  }

  async function setupMasterPassword(input: SetupMasterPasswordInput): Promise<void> {
    securityWorking.value = true
    securityError.value = ''
    try {
      const status = await credentialGateway.setupMasterPassword(input)
      credentialVaultStatus.value = status
      if (status.state === 'unlocked') {
        credentialOptions.value = sortCredentialsByCreatedAtDesc(await credentialGateway.list())
        securityDialogOpen.value = false
      } else {
        credentialOptions.value = []
        securityDialogOpen.value = true
        securityError.value = credentialStatusMessage(status, t('shell.error.setupMaster'))
      }
    } catch (error) {
      securityError.value = credentialErrorMessage(error, t('shell.error.setupMaster'))
      await reloadCredentialStateAfterFailure()
    } finally {
      securityWorking.value = false
    }
  }

  async function unlockWithPassword(password: string): Promise<void> {
    securityWorking.value = true
    securityError.value = ''
    try {
      const status = await credentialGateway.unlockPassword(password)
      credentialVaultStatus.value = status
      if (status.state === 'unlocked') {
        credentialOptions.value = sortCredentialsByCreatedAtDesc(await credentialGateway.list())
        securityDialogOpen.value = false
      } else {
        credentialOptions.value = []
        securityDialogOpen.value = true
        securityError.value = credentialStatusMessage(status, t('shell.error.unlockMaster'))
      }
    } catch (error) {
      securityError.value = credentialErrorMessage(error, t('shell.error.unlockMaster'))
      await reloadCredentialStateAfterFailure()
    } finally {
      securityWorking.value = false
    }
  }

  async function unlockAutomatically(): Promise<void> {
    securityWorking.value = true
    securityError.value = ''
    try {
      const status = await credentialGateway.unlockAutomatically()
      credentialVaultStatus.value = status
      if (status.state === 'unlocked') {
        credentialOptions.value = sortCredentialsByCreatedAtDesc(await credentialGateway.list())
        securityDialogOpen.value = false
      } else {
        credentialOptions.value = []
        securityDialogOpen.value = true
        securityError.value = credentialStatusMessage(status, t('shell.error.autoUnlock'))
      }
    } catch (error) {
      securityError.value = credentialErrorMessage(error, t('shell.error.autoUnlock'))
      await reloadCredentialStateAfterFailure()
    } finally {
      securityWorking.value = false
    }
  }

  async function changeMasterPassword(input: ChangeMasterPasswordInput): Promise<void> {
    securityWorking.value = true
    securityError.value = ''
    try {
      const status = await credentialGateway.changeMasterPassword(input)
      credentialVaultStatus.value = status
      if (status.state !== 'unlocked') {
        credentialOptions.value = []
        securityDialogOpen.value = true
      }
      if (status.reason) securityError.value = credentialStatusMessage(status, t('shell.error.changeMaster'))
    } catch (error) {
      securityError.value = credentialErrorMessage(error, t('shell.error.changeMaster'))
      await reloadCredentialStateAfterFailure()
    } finally {
      securityWorking.value = false
    }
  }

  async function changeEncryptionKey(input: ChangeEncryptionKeyInput): Promise<void> {
    securityWorking.value = true
    securityError.value = ''
    try {
      const status = await credentialGateway.changeEncryptionKey(input)
      credentialVaultStatus.value = status
      if (status.state === 'unlocked') {
        credentialOptions.value = sortCredentialsByCreatedAtDesc(await credentialGateway.list())
        securityDialogOpen.value = false
      } else {
        credentialOptions.value = []
        securityDialogOpen.value = true
        securityError.value = credentialStatusMessage(status, t('shell.error.changeEncryptionKey'))
      }
      if (status.reason) securityError.value = credentialStatusMessage(status, t('shell.error.changeEncryptionKey'))
    } catch (error) {
      securityError.value = credentialErrorMessage(error, t('shell.error.changeEncryptionKey'))
      await reloadCredentialStateAfterFailure()
    } finally {
      securityWorking.value = false
    }
  }

  async function unlockWithCustomKey(key: string): Promise<void> {
    securityWorking.value = true
    securityError.value = ''
    try {
      const status = await credentialGateway.unlockWithCustomKey(key)
      credentialVaultStatus.value = status
      if (status.state === 'unlocked') {
        credentialOptions.value = sortCredentialsByCreatedAtDesc(await credentialGateway.list())
        securityDialogOpen.value = false
      } else {
        credentialOptions.value = []
        securityDialogOpen.value = true
        securityError.value = credentialStatusMessage(status, t('shell.error.unlockCustomKey'))
      }
    } catch (error) {
      securityError.value = credentialErrorMessage(error, t('shell.error.unlockCustomKey'))
      await reloadCredentialStateAfterFailure()
    } finally {
      securityWorking.value = false
    }
  }

  async function setAutoUnlock(input: SetAutoUnlockInput): Promise<void> {
    securityWorking.value = true
    securityError.value = ''
    try {
      const status = await credentialGateway.setAutoUnlock(input)
      credentialVaultStatus.value = status
      if (status.state === 'unlocked')
        credentialOptions.value = sortCredentialsByCreatedAtDesc(await credentialGateway.list())
      else {
        credentialOptions.value = []
        securityDialogOpen.value = true
      }
      if (status.reason) securityError.value = credentialStatusMessage(status, t('shell.error.autoUnlockSetting'))
    } catch (error) {
      securityError.value = credentialErrorMessage(error, t('shell.error.autoUnlockSetting'))
      await reloadCredentialStateAfterFailure()
    } finally {
      securityWorking.value = false
    }
  }

  async function lockCredentialVault(): Promise<void> {
    securityWorking.value = true
    securityError.value = ''
    try {
      await credentialGateway.lock()
      credentialOptions.value = []
      credentialVaultStatus.value = await credentialGateway.status()
      securityDialogOpen.value = true
    } catch (error) {
      credentialOptions.value = []
      securityError.value = credentialErrorMessage(error, t('shell.error.lockVault'))
      await reloadCredentialStateAfterFailure()
      securityDialogOpen.value = true
    } finally {
      securityWorking.value = false
    }
  }

  async function exportRecovery(input: ExportCredentialVaultKeyInput): Promise<void> {
    securityWorking.value = true
    securityError.value = ''
    try {
      await credentialGateway.exportRecovery(input)
      toast.success(t('shell.success.recoveryExported'))
    } catch (error) {
      securityError.value = credentialErrorMessage(error, t('shell.error.recoveryExport'))
    } finally {
      securityWorking.value = false
    }
  }

  async function importRecovery(input: ImportCredentialVaultKeyInput): Promise<void> {
    securityWorking.value = true
    securityError.value = ''
    try {
      if (credentialVaultStatus.value?.state === 'unlocked') {
        await credentialGateway.lock()
        credentialOptions.value = []
      }
      const status = await credentialGateway.importRecovery(input)
      credentialVaultStatus.value = status
      if (status.state === 'unlocked') {
        credentialOptions.value = sortCredentialsByCreatedAtDesc(await credentialGateway.list())
        toast.success(t('shell.success.recoveryImported'))
        securityDialogOpen.value = false
      } else {
        credentialOptions.value = []
        securityError.value = credentialStatusMessage(status, t('shell.error.recoveryImport'))
      }
    } catch (error) {
      securityError.value = credentialErrorMessage(error, t('shell.error.recoveryImport'))
      await reloadCredentialStateAfterFailure()
    } finally {
      securityWorking.value = false
    }
  }

  async function resetVault(input: ResetCredentialVaultInput): Promise<void> {
    securityWorking.value = true
    securityError.value = ''
    try {
      const status = await credentialGateway.resetVault(input)
      credentialVaultStatus.value = status
      credentialOptions.value = []
      if (status.state === 'setupRequired') toast.success(t('shell.success.vaultReset'))
      else securityError.value = credentialStatusMessage(status, t('shell.error.vaultReset'))
      securityDialogOpen.value = true
    } catch (error) {
      securityError.value = credentialErrorMessage(error, t('shell.error.vaultReset'))
      await reloadCredentialStateAfterFailure()
    } finally {
      securityWorking.value = false
    }
  }

  async function ensureCredentialStorageReady(): Promise<boolean> {
    credentialOptionsError.value = ''
    try {
      let status = credentialVaultStatus.value ?? (await credentialGateway.status())
      if (status.state !== 'unlocked') {
        if (['setupRequired', 'recoveryRequired', 'error'].includes(status.state) || !status.autoUnlockEnabled) {
          securityDialogOpen.value = true
          return false
        }
        status = await credentialGateway.unlockAutomatically()
      }
      credentialVaultStatus.value = status
      if (status.state !== 'unlocked') {
        if (credentialGateway.persistent) securityDialogOpen.value = true
        return false
      }
      credentialOptions.value = sortCredentialsByCreatedAtDesc(await credentialGateway.list())
      return true
    } catch (error) {
      credentialOptionsError.value = credentialErrorMessage(error, t('shell.error.vaultUnavailable'))
      return false
    }
  }

  async function refreshCredentials(): Promise<void> {
    credentialOptionsLoading.value = true
    credentialOptionsError.value = ''
    try {
      const status = await credentialGateway.status()
      credentialVaultStatus.value = status
      credentialOptions.value =
        status.state === 'unlocked' ? sortCredentialsByCreatedAtDesc(await credentialGateway.list()) : []
    } catch (error) {
      credentialOptions.value = []
      credentialOptionsError.value = credentialErrorMessage(error, t('shell.error.credentialsRead'))
    } finally {
      credentialOptionsLoading.value = false
    }
  }

  async function createCredential(name: string, kind: 'password' | 'privateKey', secret: string, passphrase = '') {
    const credential = await credentialGateway.create(name, kind, secret, passphrase)
    credentialOptions.value = sortCredentialsByCreatedAtDesc([...credentialOptions.value, credential])
    return credential
  }

  async function updateCredential(
    credential: CredentialRecord,
    name: string,
    secret: string,
    passphrase = '',
  ): Promise<void> {
    let updated = credential
    if (name.trim() !== credential.name) updated = await credentialGateway.rename(credential.id, name)
    if (secret) updated = await credentialGateway.replace(credential.id, credential.kind, secret, passphrase)
    credentialOptions.value = sortCredentialsByCreatedAtDesc(
      credentialOptions.value.map((item) => (item.id === updated.id ? updated : item)),
    )
  }

  async function removeCredential(id: string): Promise<void> {
    await credentialGateway.remove(id)
    credentialOptions.value = credentialOptions.value.filter((item) => item.id !== id)
  }

  return {
    credentialGateway,
    credentialOptions,
    credentialVaultStatus,
    credentialOptionsLoading,
    credentialOptionsError,
    securityDialogOpen,
    securityWorking,
    securityError,
    openSecuritySettings,
    closeSecuritySettings,
    refreshSecurityStatus,
    setupMasterPassword,
    unlockWithPassword,
    unlockAutomatically,
    changeMasterPassword,
    changeEncryptionKey,
    unlockWithCustomKey,
    setAutoUnlock,
    lockCredentialVault,
    exportRecovery,
    importRecovery,
    resetVault,
    ensureCredentialStorageReady,
    refreshCredentials,
    createCredential,
    updateCredential,
    removeCredential,
  }
}
