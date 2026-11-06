import { errorMessage } from './errors'
import { createRuntimeGateway } from './runtime'

export type CredentialKind = 'password' | 'privateKey' | 'privateKeyPassphrase' | 'sshAgent'
export type CredentialVaultState = 'setupRequired' | 'locked' | 'unlocked' | 'recoveryRequired' | 'error'
export type CredentialVaultMode = '' | 'automatic' | 'masterPassword'
export type LocalEncryptionMode = 'system' | 'custom'

export interface CredentialRecord {
  id: string
  name: string
  kind: CredentialKind
  username?: string
  publicKey?: string
  configured: boolean
  createdAt: string
  updatedAt: string
}

export function sortCredentialsByCreatedAtDesc(records: CredentialRecord[]): CredentialRecord[] {
  return [...records].sort((left, right) => right.createdAt.localeCompare(left.createdAt))
}

export interface CredentialVaultStatus {
  state: CredentialVaultState
  mode: CredentialVaultMode
  reason: string
  messageKey: string
  autoUnlockEnabled: boolean
  passwordConfigured: boolean
  hint: string

  encryptionMode?: LocalEncryptionMode
  keychainAvailable?: boolean
  customKeyConfigured?: boolean
  recoveryAvailable?: boolean
}

export interface ChangeEncryptionKeyInput {
  currentKey: string
  newKey: string
  confirmation: string
}

export interface SetupMasterPasswordInput {
  password: string
  confirmation: string
  hint: string
}

export interface ChangeMasterPasswordInput {
  oldPassword: string
  newPassword: string
  hint: string
}

export interface SetAutoUnlockInput {
  enabled: boolean
  password: string
}

export interface ExportCredentialVaultKeyInput {
  path: string
  currentPassword: string
  packagePassword: string
  confirmation: string
}

export interface ImportCredentialVaultKeyInput {
  path: string
  packagePassword: string
  masterPassword: string
  confirmation: string
  hint: string
}

export interface ResetCredentialVaultInput {
  confirmation: string
}

export interface CredentialMetadata {
  username?: string
  publicKey?: string
}

export interface GeneratePrivateKeyInput {
  algorithm: 'ed25519' | 'ecdsa' | 'rsa'
  curve?: 'p-256' | 'p-384' | 'p-521'
  bits?: 2048 | 4096
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

export interface LocalPrivateKey {
  token: string
  name: string
  displayPath: string
  algorithm: string
}

export interface ResourceBundleSelection {
  name: string

  data: string
}

export interface RevealedCredential {
  kind: CredentialKind
  secret: string
  passphrase?: string
  username?: string
  publicKey?: string
}

export interface CredentialGateway {
  readonly persistent: boolean
  status(): Promise<CredentialVaultStatus>
  setupMasterPassword(input: SetupMasterPasswordInput): Promise<CredentialVaultStatus>
  unlockPassword(password: string): Promise<CredentialVaultStatus>
  unlockAutomatically(): Promise<CredentialVaultStatus>
  changeMasterPassword(input: ChangeMasterPasswordInput): Promise<CredentialVaultStatus>
  changeEncryptionKey(input: ChangeEncryptionKeyInput): Promise<CredentialVaultStatus>
  unlockWithCustomKey(key: string): Promise<CredentialVaultStatus>
  setAutoUnlock(input: SetAutoUnlockInput): Promise<CredentialVaultStatus>
  exportRecovery(input: ExportCredentialVaultKeyInput): Promise<void>
  importRecovery(input: ImportCredentialVaultKeyInput): Promise<CredentialVaultStatus>
  resetVault(input: ResetCredentialVaultInput): Promise<CredentialVaultStatus>
  exportResources(password: string): Promise<boolean>
  validateResourceBundle(selection: ResourceBundleSelection, password: string): Promise<boolean>
  importResources(password: string, selection?: ResourceBundleSelection): Promise<boolean>
  lock(): Promise<void>
  list(): Promise<CredentialRecord[]>
  create(
    name: string,
    kind: CredentialKind,
    secret: string,
    passphrase?: string,
    metadata?: CredentialMetadata,
  ): Promise<CredentialRecord>
  rename(id: string, name: string): Promise<CredentialRecord>
  replace(
    id: string,
    kind: CredentialKind,
    secret: string,
    passphrase?: string,
    metadata?: CredentialMetadata,
  ): Promise<CredentialRecord>
  generatePrivateKey?(input: GeneratePrivateKeyInput): Promise<GeneratedPrivateKey>
  listLocalPrivateKeys?(): Promise<LocalPrivateKey[]>

  revealCredential?(id: string): Promise<RevealedCredential>
  importLocalPrivateKey?(input: {
    token: string
    name: string
    username?: string
    passphrase?: string
  }): Promise<CredentialRecord>
  remove(id: string): Promise<void>
}

class MemoryCredentialGateway implements CredentialGateway {
  readonly persistent = false
  private readonly credentials: CredentialRecord[] = []
  private vaultStatus: CredentialVaultStatus = {
    state: 'unlocked',
    mode: 'automatic',
    reason: '',
    messageKey: 'credential.status.preview',
    autoUnlockEnabled: true,
    passwordConfigured: true,
    hint: '',
    encryptionMode: 'system',
    keychainAvailable: true,
    customKeyConfigured: false,
    recoveryAvailable: true,
  }

  private password = 'browser-preview-password'

  async status(): Promise<CredentialVaultStatus> {
    return { ...this.vaultStatus }
  }

  async setupMasterPassword(input: SetupMasterPasswordInput): Promise<CredentialVaultStatus> {
    if (this.vaultStatus.passwordConfigured) {
      throw new Error('主密码已经设置，请直接解锁或修改主密码')
    }
    if (!input.password.trim() || input.password !== input.confirmation) {
      this.vaultStatus = {
        ...this.vaultStatus,
        state: 'error',
        reason: 'invalidPassword',
        messageKey: 'credential.status.passwordConfirmation',
      }
      return { ...this.vaultStatus }
    }
    this.password = input.password
    this.vaultStatus = {
      ...this.vaultStatus,
      state: 'unlocked',
      mode: 'automatic',
      reason: '',
      messageKey: 'credential.status.preview',
      autoUnlockEnabled: true,
      passwordConfigured: true,
      hint: input.hint.trim(),
    }
    return { ...this.vaultStatus }
  }

  async unlockAutomatically(): Promise<CredentialVaultStatus> {
    if (!this.vaultStatus.passwordConfigured || !this.vaultStatus.autoUnlockEnabled) {
      return {
        ...this.vaultStatus,
        state: this.vaultStatus.passwordConfigured ? 'locked' : 'setupRequired',
        mode: this.vaultStatus.passwordConfigured ? 'masterPassword' : '',
        reason: this.vaultStatus.passwordConfigured ? 'autoUnlockDisabled' : 'setupRequired',
        messageKey: this.vaultStatus.passwordConfigured
          ? 'credential.status.autoUnlockDisabled'
          : 'credential.status.setupRequired',
      }
    }
    this.vaultStatus = {
      ...this.vaultStatus,
      state: 'unlocked',
      mode: 'automatic',
      reason: '',
      messageKey: 'credential.status.preview',
    }
    return { ...this.vaultStatus }
  }

  async unlockPassword(password: string): Promise<CredentialVaultStatus> {
    if (!this.vaultStatus.passwordConfigured) {
      return {
        ...this.vaultStatus,
        state: 'setupRequired',
        mode: '',
        reason: 'setupRequired',
        messageKey: 'credential.status.setupRequired',
      }
    }
    this.vaultStatus =
      password === this.password
        ? {
            ...this.vaultStatus,
            state: 'unlocked',
            mode: 'masterPassword',
            reason: '',
            messageKey: 'credential.status.previewEnabled',
          }
        : {
            ...this.vaultStatus,
            state: 'locked',
            mode: 'masterPassword',
            reason: 'invalidPassword',
            messageKey: 'credential.status.incorrectPassword',
          }
    return { ...this.vaultStatus }
  }

  async changeMasterPassword(input: ChangeMasterPasswordInput): Promise<CredentialVaultStatus> {
    if (!this.vaultStatus.passwordConfigured) throw new Error('主密码尚未设置')
    if (input.oldPassword !== this.password) {
      return {
        ...this.vaultStatus,
        reason: 'authenticationFailed',
        messageKey: 'credential.status.incorrectCurrentPassword',
      }
    }
    if (!input.newPassword.trim()) {
      return {
        ...this.vaultStatus,
        state: 'error',
        reason: 'invalidPassword',
        messageKey: 'credential.status.newPasswordRequired',
      }
    }
    this.password = input.newPassword
    this.vaultStatus = {
      ...this.vaultStatus,
      hint: input.hint.trim(),
      passwordConfigured: true,
      reason: '',
    }
    return { ...this.vaultStatus }
  }

  async changeEncryptionKey(input: ChangeEncryptionKeyInput): Promise<CredentialVaultStatus> {
    if (!input.newKey.trim() || input.newKey !== input.confirmation) {
      return {
        ...this.vaultStatus,
        reason: 'invalidEncryptionKey',
        messageKey: 'credential.status.encryptionKeyInvalid',
      }
    }
    if (this.vaultStatus.encryptionMode === 'custom' && input.currentKey !== this.password) {
      return {
        ...this.vaultStatus,
        reason: 'authenticationFailed',
        messageKey: 'credential.status.incorrectCurrentEncryptionKey',
      }
    }
    this.password = input.newKey
    this.vaultStatus = {
      ...this.vaultStatus,
      encryptionMode: 'custom',
      customKeyConfigured: true,
      state: 'unlocked',
      mode: 'automatic',
      reason: '',
      messageKey: 'credential.status.preview',
    }
    return { ...this.vaultStatus }
  }

  async unlockWithCustomKey(key: string): Promise<CredentialVaultStatus> {
    if (!this.vaultStatus.customKeyConfigured) return { ...this.vaultStatus, reason: 'customKeyUnavailable' }
    if (key !== this.password) {
      return {
        ...this.vaultStatus,
        state: 'locked',
        reason: 'authenticationFailed',
        messageKey: 'credential.status.incorrectCurrentEncryptionKey',
      }
    }
    this.vaultStatus = {
      ...this.vaultStatus,
      state: 'unlocked',
      mode: 'masterPassword',
      reason: '',
      messageKey: 'credential.status.preview',
    }
    return { ...this.vaultStatus }
  }

  async setAutoUnlock(input: SetAutoUnlockInput): Promise<CredentialVaultStatus> {
    if (!this.vaultStatus.passwordConfigured) throw new Error('主密码尚未设置')
    if (input.password !== this.password) {
      return {
        ...this.vaultStatus,
        reason: 'authenticationFailed',
        messageKey: 'credential.status.incorrectCurrentPassword',
      }
    }
    this.vaultStatus = {
      ...this.vaultStatus,
      autoUnlockEnabled: input.enabled,
      state: this.vaultStatus.state === 'unlocked' ? 'unlocked' : 'locked',
      mode: input.enabled ? 'automatic' : 'masterPassword',
      reason: '',
      messageKey: input.enabled ? 'credential.status.preview' : 'credential.status.nextStartPassword',
    }
    return { ...this.vaultStatus }
  }

  async exportRecovery(input: ExportCredentialVaultKeyInput): Promise<void> {
    if (this.vaultStatus.state !== 'unlocked') throw new Error('请先解锁本地密码保存')
    if (input.currentPassword !== this.password) throw new Error('credential.status.incorrectCurrentPassword')
    if (!input.path.trim() || !input.packagePassword || input.packagePassword !== input.confirmation) {
      throw new Error('请填写有效的恢复包路径和密码')
    }
  }

  async importRecovery(input: ImportCredentialVaultKeyInput): Promise<CredentialVaultStatus> {
    if (!['locked', 'recoveryRequired', 'error'].includes(this.vaultStatus.state)) {
      return {
        ...this.vaultStatus,
        reason: 'recoveryNotAllowed',
        messageKey: 'credential.status.importUnavailable',
      }
    }
    if (!input.path.trim() || !input.packagePassword || !input.masterPassword) {
      return {
        ...this.vaultStatus,
        state: 'error',
        reason: 'invalidRecoveryPackage',
        messageKey: 'credential.status.recoveryInputMissing',
      }
    }
    if (input.masterPassword !== input.confirmation) {
      return {
        ...this.vaultStatus,
        state: 'error',
        reason: 'invalidPassword',
        messageKey: 'credential.status.newPasswordConfirmation',
      }
    }
    this.password = input.masterPassword
    this.vaultStatus = {
      ...this.vaultStatus,
      state: 'unlocked',
      mode: 'automatic',
      autoUnlockEnabled: true,
      passwordConfigured: true,
      hint: input.hint.trim(),
      reason: '',
      messageKey: 'credential.status.preview',
    }
    return { ...this.vaultStatus }
  }

  async resetVault(input: ResetCredentialVaultInput): Promise<CredentialVaultStatus> {
    if (input.confirmation !== 'RESET QTERM VAULT') throw new Error('重置确认短语不正确')
    if (!['locked', 'recoveryRequired', 'error'].includes(this.vaultStatus.state)) {
      throw new Error('当前状态不允许重新初始化保险库')
    }
    this.credentials.splice(0, this.credentials.length)
    this.password = ''
    this.vaultStatus = {
      state: 'setupRequired',
      mode: '',
      reason: 'reset',
      messageKey: 'credential.status.resetComplete',
      autoUnlockEnabled: false,
      passwordConfigured: false,
      hint: '',
    }
    return { ...this.vaultStatus }
  }

  async exportResources(password: string): Promise<boolean> {
    void password
    return true
  }

  async validateResourceBundle(selection: ResourceBundleSelection, password: string): Promise<boolean> {
    void selection
    void password
    return true
  }

  async importResources(password: string, selection?: ResourceBundleSelection): Promise<boolean> {
    void password
    void selection
    return true
  }

  async lock(): Promise<void> {
    if (!this.vaultStatus.passwordConfigured) return
    this.vaultStatus = {
      ...this.vaultStatus,
      state: 'locked',
      mode: this.vaultStatus.mode,
      reason: 'locked',
      messageKey: 'credential.status.storageDisabled',
    }
  }

  async list(): Promise<CredentialRecord[]> {
    this.requireUnlocked()
    return this.credentials.map((credential) => ({ ...credential }))
  }

  async create(
    name: string,
    kind: CredentialKind,
    secret: string,
    passphrase = '',
    metadata?: CredentialMetadata,
  ): Promise<CredentialRecord> {
    void passphrase
    void metadata
    this.requireUnlocked()
    requireSecret(secret)
    const now = new Date().toISOString()
    const credential: CredentialRecord = {
      id: createCredentialID(),
      name: requireName(name),
      kind,
      configured: true,
      createdAt: now,
      updatedAt: now,
    }
    this.credentials.push(credential)
    return { ...credential }
  }

  async rename(id: string, name: string): Promise<CredentialRecord> {
    const credential = this.requireCredential(id)
    credential.name = requireName(name)
    credential.updatedAt = new Date().toISOString()
    return { ...credential }
  }

  async replace(
    id: string,
    kind: CredentialKind,
    secret: string,
    passphrase = '',
    metadata?: CredentialMetadata,
  ): Promise<CredentialRecord> {
    void passphrase
    void metadata
    const credential = this.requireCredential(id)
    requireSecret(secret)
    if (credential.kind !== kind) throw new Error('认证信息类型不能更改')
    credential.updatedAt = new Date().toISOString()
    return { ...credential }
  }

  async remove(id: string): Promise<void> {
    this.requireUnlocked()
    const index = this.credentials.findIndex((credential) => credential.id === id)
    if (index < 0) throw new Error('认证信息不存在')
    this.credentials.splice(index, 1)
  }

  private requireUnlocked(): void {
    if (this.vaultStatus.state !== 'unlocked') throw new Error('本地认证信息当前不可用')
  }

  private requireCredential(id: string): CredentialRecord {
    this.requireUnlocked()
    const credential = this.credentials.find((item) => item.id === id)
    if (!credential) throw new Error('认证信息不存在')
    return credential
  }
}

class WailsCredentialGateway implements CredentialGateway {
  readonly persistent = true

  private async bindings(): Promise<CredentialBindings> {
    return (await import('../../bindings/changeme/internal/app/service')) as unknown as CredentialBindings
  }

  async status(): Promise<CredentialVaultStatus> {
    const bindings = await this.bindings()
    return (await bindings.CredentialStatus()) as CredentialVaultStatus
  }

  async setupMasterPassword(input: SetupMasterPasswordInput): Promise<CredentialVaultStatus> {
    const bindings = await this.bindings()
    return (await bindings.SetupMasterPassword(input)) as CredentialVaultStatus
  }

  async unlockPassword(password: string): Promise<CredentialVaultStatus> {
    const bindings = await this.bindings()
    return (await bindings.UnlockWithPassword(password)) as CredentialVaultStatus
  }

  async unlockAutomatically(): Promise<CredentialVaultStatus> {
    const bindings = await this.bindings()
    return (await bindings.UnlockAutomatically()) as CredentialVaultStatus
  }

  async changeMasterPassword(input: ChangeMasterPasswordInput): Promise<CredentialVaultStatus> {
    const bindings = await this.bindings()
    return (await bindings.ChangeMasterPassword(input)) as CredentialVaultStatus
  }

  async changeEncryptionKey(input: ChangeEncryptionKeyInput): Promise<CredentialVaultStatus> {
    const bindings = await this.bindings()
    return (await bindings.ChangeEncryptionKey(input)) as CredentialVaultStatus
  }

  async unlockWithCustomKey(key: string): Promise<CredentialVaultStatus> {
    const bindings = await this.bindings()
    return (await bindings.UnlockWithCustomKey(key)) as CredentialVaultStatus
  }

  async setAutoUnlock(input: SetAutoUnlockInput): Promise<CredentialVaultStatus> {
    const bindings = await this.bindings()
    return (await bindings.SetAutoUnlock(input)) as CredentialVaultStatus
  }

  async exportRecovery(input: ExportCredentialVaultKeyInput): Promise<void> {
    const bindings = await this.bindings()
    await bindings.ExportCredentialVaultKey(input)
  }

  async importRecovery(input: ImportCredentialVaultKeyInput): Promise<CredentialVaultStatus> {
    const bindings = await this.bindings()
    return (await bindings.ImportCredentialVaultKey(input)) as CredentialVaultStatus
  }

  async resetVault(input: ResetCredentialVaultInput): Promise<CredentialVaultStatus> {
    const bindings = await this.bindings()
    return (await bindings.ResetCredentialVault(input)) as CredentialVaultStatus
  }

  async exportResources(password: string): Promise<boolean> {
    const bindings = await this.bindings()
    return await bindings.ExportResourceBundle(password)
  }

  async validateResourceBundle(selection: ResourceBundleSelection, password: string): Promise<boolean> {
    const bindings = await this.bindings()
    if (!bindings.ValidateResourceBundleData) throw new Error('asset.import.unsupported')
    return Boolean(await bindings.ValidateResourceBundleData(selection.data, password))
  }

  async importResources(password: string, selection?: ResourceBundleSelection): Promise<boolean> {
    const bindings = await this.bindings()
    if (selection) {
      if (!bindings.ImportResourceBundleData) throw new Error('asset.import.unsupported')
      return await bindings.ImportResourceBundleData(selection.data, password)
    }
    return await bindings.ImportResourceBundle(password)
  }

  async lock(): Promise<void> {
    const bindings = await this.bindings()
    await bindings.LockCredentialVault()
  }

  async list(): Promise<CredentialRecord[]> {
    const bindings = await this.bindings()
    return ((await bindings.ListCredentials()) ?? []) as CredentialRecord[]
  }

  async create(
    name: string,
    kind: CredentialKind,
    secret: string,
    passphrase = '',
    metadata?: CredentialMetadata,
  ): Promise<CredentialRecord> {
    const bindings = await this.bindings()
    return (await bindings.CreateCredential({
      name,
      kind: kind as BindingCredentialKind,
      secret,
      passphrase,
      username: metadata?.username,
      publicKey: metadata?.publicKey,
    })) as CredentialRecord
  }

  async rename(id: string, name: string): Promise<CredentialRecord> {
    const bindings = await this.bindings()
    return (await bindings.RenameCredential(id, { name })) as CredentialRecord
  }

  async replace(
    id: string,
    kind: CredentialKind,
    secret: string,
    passphrase = '',
    metadata?: CredentialMetadata,
  ): Promise<CredentialRecord> {
    const bindings = await this.bindings()
    return (await bindings.ReplaceCredentialSecret(id, {
      kind: kind as BindingCredentialKind,
      secret,
      passphrase,
      username: metadata?.username,
      publicKey: metadata?.publicKey,
    })) as CredentialRecord
  }

  async generatePrivateKey(input: GeneratePrivateKeyInput): Promise<GeneratedPrivateKey> {
    const bindings = await this.bindings()
    if (!bindings.GeneratePrivateKey) throw new Error('credential.keyGeneration.unsupported')
    return (await bindings.GeneratePrivateKey(input)) as GeneratedPrivateKey
  }

  async listLocalPrivateKeys(): Promise<LocalPrivateKey[]> {
    const bindings = await this.bindings()
    if (!bindings.ListLocalPrivateKeys) throw new Error('credential.localKeyScan.unsupported')
    return ((await bindings.ListLocalPrivateKeys()) ?? []) as LocalPrivateKey[]
  }

  async revealCredential(id: string): Promise<RevealedCredential> {
    const { RevealCredential } = await import('../../bindings/changeme/internal/app/service')
    const revealed = await RevealCredential(id)
    return {
      kind: (revealed?.kind ?? 'password') as CredentialKind,
      secret: revealed?.secret ?? '',
      passphrase: revealed?.passphrase ?? '',
      username: revealed?.username ?? '',
      publicKey: revealed?.publicKey ?? '',
    }
  }

  async importLocalPrivateKey(input: {
    token: string
    name: string
    username?: string
    passphrase?: string
  }): Promise<CredentialRecord> {
    const bindings = await this.bindings()
    if (!bindings.ImportLocalPrivateKey) throw new Error('credential.localKeyImport.unsupported')
    return (await bindings.ImportLocalPrivateKey(input)) as CredentialRecord
  }

  async remove(id: string): Promise<void> {
    const bindings = await this.bindings()
    await bindings.DeleteCredential(id)
  }
}

interface CredentialBindings {
  CredentialStatus(): PromiseLike<unknown>
  SetupMasterPassword(input: SetupMasterPasswordInput): PromiseLike<unknown>
  UnlockWithPassword(password: string): PromiseLike<unknown>
  UnlockAutomatically(): PromiseLike<unknown>
  ChangeMasterPassword(input: ChangeMasterPasswordInput): PromiseLike<unknown>
  ChangeEncryptionKey(input: ChangeEncryptionKeyInput): PromiseLike<unknown>
  UnlockWithCustomKey(key: string): PromiseLike<unknown>
  SetAutoUnlock(input: SetAutoUnlockInput): PromiseLike<unknown>
  ExportCredentialVaultKey(input: ExportCredentialVaultKeyInput): PromiseLike<void>
  ImportCredentialVaultKey(input: ImportCredentialVaultKeyInput): PromiseLike<unknown>
  ResetCredentialVault(input: ResetCredentialVaultInput): PromiseLike<unknown>
  ExportResourceBundle(password: string): PromiseLike<boolean>
  ValidateResourceBundleData?(data: string | null, password: string): PromiseLike<boolean>
  ImportResourceBundle(password: string): PromiseLike<boolean>
  ImportResourceBundleData?(data: string | null, password: string): PromiseLike<boolean>
  LockCredentialVault(): PromiseLike<void>
  ListCredentials(): PromiseLike<unknown[] | null>
  CreateCredential(input: {
    name: string
    kind: BindingCredentialKind
    secret: string
    passphrase: string
    username?: string
    publicKey?: string
  }): PromiseLike<unknown>
  RenameCredential(id: string, input: { name: string }): PromiseLike<unknown>
  ReplaceCredentialSecret(
    id: string,
    input: {
      kind: BindingCredentialKind
      secret: string
      passphrase: string
      username?: string
      publicKey?: string
    },
  ): PromiseLike<unknown>
  GeneratePrivateKey?(input: GeneratePrivateKeyInput): PromiseLike<unknown>
  ListLocalPrivateKeys?(): PromiseLike<unknown[] | null>
  ImportLocalPrivateKey?(input: {
    token: string
    name: string
    username?: string
    passphrase?: string
  }): PromiseLike<unknown>
  DeleteCredential(id: string): PromiseLike<void>
}

export function createCredentialGateway(): CredentialGateway {
  return createRuntimeGateway<CredentialGateway>(
    () => new WailsCredentialGateway(),
    () => new MemoryCredentialGateway(),
  )
}

export function credentialErrorMessage(error: unknown, fallback: string): string {
  return errorMessage(error, fallback)
}

function requireName(name: string): string {
  const normalized = name.trim()
  if (!normalized) throw new Error('认证信息名称不能为空')
  return normalized
}

function requireSecret(secret: string): void {
  if (!secret) throw new Error('认证信息内容不能为空或过大')
}

function createCredentialID(): string {
  const randomID = globalThis.crypto?.randomUUID?.()
  return `credential-${randomID ?? `${Date.now()}-${Math.random().toString(16).slice(2)}`}`
}
import type { CredentialKind as BindingCredentialKind } from '../../bindings/changeme/internal/model/models'
