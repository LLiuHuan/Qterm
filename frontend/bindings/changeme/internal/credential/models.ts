// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import * as model$0 from '../model/models.js'
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import * as security$0 from '../security/models.js'

export interface ChangeEncryptionKeyInput {
  currentKey: string
  newKey: string
  confirmation: string
}

export interface ChangeMasterPasswordInput {
  oldPassword: string
  newPassword: string
  hint: string
}

export interface CreateCredentialInput {
  name: string
  kind: model$0.CredentialKind
  secret: string
  passphrase: string
  username?: string
  publicKey?: string
}

export enum CredentialVaultMode {
  $zero = '',

  CredentialVaultModeAutomatic = 'automatic',
  CredentialVaultModeMasterPassword = 'masterPassword',
}

export enum CredentialVaultState {
  $zero = '',

  CredentialVaultStateSetupRequired = 'setupRequired',
  CredentialVaultStateLocked = 'locked',
  CredentialVaultStateUnlocked = 'unlocked',
  CredentialVaultStateRecovery = 'recoveryRequired',
  CredentialVaultStateError = 'error',
}

export interface CredentialVaultStatus {
  state: CredentialVaultState
  mode: CredentialVaultMode
  reason: string

  messageKey: string
  autoUnlockEnabled: boolean
  passwordConfigured: boolean
  hint: string
  encryptionMode?: security$0.EncryptionMode
  keychainAvailable?: boolean
  customKeyConfigured?: boolean
  recoveryAvailable?: boolean
}

export interface ExportCredentialVaultKeyInput {
  path: string
  currentPassword: string
  packagePassword: string
  confirmation: string
}

export interface GeneratePrivateKeyInput {
  algorithm: string
  curve?: string
  bits?: number
  passphrase?: string
  username?: string
  name?: string
}

export interface GeneratedPrivateKey {
  algorithm: string
  privateKey: string
  publicKey: string
  installCommand: string
}

export interface ImportCredentialVaultKeyInput {
  path: string
  packagePassword: string
  masterPassword: string
  confirmation: string
  hint: string
}

export interface ImportLocalPrivateKeyInput {
  token: string
  name: string
  username?: string
  passphrase?: string
}

export interface LocalPrivateKey {
  token: string
  name: string
  displayPath: string
  algorithm: string
}

export interface RenameCredentialInput {
  name: string
}

export interface ReplaceCredentialSecretInput {
  kind: model$0.CredentialKind
  secret: string
  passphrase: string
  username?: string
  publicKey?: string
}

export interface ResetCredentialVaultInput {
  confirmation: string
}

export interface RevealedCredential {
  kind: model$0.CredentialKind
  secret: string
  passphrase?: string
  username?: string
  publicKey?: string
}

export interface SetAutoUnlockInput {
  enabled: boolean
  password: string
}

export interface SetupMasterPasswordInput {
  password: string
  confirmation: string
  hint: string
}
