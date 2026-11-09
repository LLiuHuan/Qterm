import { onScopeDispose, ref } from 'vue'
import { toast } from 'vue-sonner'
import type { useCredentialState } from '../../composables/useCredentialState'
import { credentialErrorMessage, type CredentialRecord } from '../../services/credentials'
import {
  hostErrorMessage,
  normalizeProxyMode,
  type HostGateway,
  type HostPasswordSource,
  type HostRecord,
} from '@/services/hosts.ts'
import {
  sessionErrorMessage,
  type SessionDetectIconInput,
  type SessionGateway,
  type SessionTestConnectionInput,
} from '../../services/sessions'
import { decodeAppError } from '../../services/errors'
import type { HostAuthMode } from '../../types/qterm'
import { hostIconPickerOptions } from './builtinIcons'
import { hostToDraft } from './hostDraft'
import {
  hostProtocolAuthModes,
  hostProtocolShows,
  vncUsesWebSocket,
  vncWsPathError,
  vncWsPathErrorKey,
  type HostEditorDraft,
} from './hostProtocols'
import { useI18n } from '@/locales'

interface HostEditorOptions {
  hostGateway: Pick<HostGateway, 'persistent' | 'create' | 'update'>
  sessionGateway: Pick<SessionGateway, 'testConnection' | 'detectIcon'>
  credentials: Pick<
    ReturnType<typeof useCredentialState>,
    | 'credentialGateway'
    | 'credentialOptions'
    | 'credentialVaultStatus'
    | 'credentialOptionsLoading'
    | 'credentialOptionsError'
    | 'securityDialogOpen'
    | 'ensureCredentialStorageReady'
  >
  onSaved: (host: HostRecord, created: boolean) => Promise<void>
}

function iconDisplayName(value: string): string {
  if (!value) return value
  return hostIconPickerOptions.find((option) => option.value === value)?.label ?? value
}

export function useHostEditor({ hostGateway, sessionGateway, credentials, onSaved }: HostEditorOptions) {
  const { t } = useI18n()
  const {
    credentialGateway,
    credentialOptions,
    credentialVaultStatus,
    credentialOptionsLoading,
    credentialOptionsError,
    securityDialogOpen,
    ensureCredentialStorageReady,
  } = credentials
  const hostDialogOpen = ref(false)
  const editingHostId = ref<string | null>(null)
  const formError = ref('')
  const savingHost = ref(false)
  const testingConnection = ref(false)
  const credentialOptionsRequestID = ref(0)
  const authMode = ref<HostAuthMode>('password')
  const directSecret = ref('')

  const originalPasswordConfigured = ref(false)
  const authModeChanged = ref(false)
  const draft = ref<HostEditorDraft>(createEmptyDraft())
  let editorVersion = 0
  let disposed = false

  function preferredAuthMode(protocol: string | undefined | null): HostAuthMode {
    const allowed = hostProtocolAuthModes(protocol)
    return allowed.includes('password') ? 'password' : (allowed[0] ?? 'password')
  }

  function createEmptyDraft(): HostEditorDraft {
    return {
      icon: '',

      color: '',
      name: '',
      protocol: 'ssh',
      address: '',
      port: 22,
      username: '',
      group: '',
      credentialId: '',
      passwordSource: 'saved',
      sshTwoFactorEnabled: false,
      sshOtpEntryId: '',
      sshOtpAutoFill: false,

      vncViewOnly: false,
      vncShared: true,
      vncPixelDepth: 32,
      vncScale: 'fit',
      vncAutoReconnect: false,
      vncAutoReconnectAttempts: 5,

      vncTransport: 'tcp',
      vncWsPath: '',
      vncAllowInsecureTls: false,

      vncRfbPasswordMode: 'auto',
      notes: '',
      tags: [],
      sshKeepAlive: true,
      latencyDetection: true,
      autoDetectOSIcon: true,
      directoryFollow: 'inherit',
      sftpShowHiddenFiles: 'inherit',
      sftpDefaultDirectory: '',
      backspaceMode: 'del',
      encoding: 'utf-8',
      connectTimeoutSeconds: 15,
      postLoginCommand: '',
      postLoginCommandDelaySeconds: 0,
      environmentVariables: [],
      connectionType: 'standard',
      terminalType: 'xterm-256color',
      x11Forwarding: false,
      x11Trusted: true,

      jumpHostId: '',

      proxyId: '',
      proxyMode: 'none',
      proxyHost: '',
      proxyPort: 0,
      proxyUsername: '',
      proxyPassword: '',
      clearProxyPassword: false,
      proxyPasswordConfigured: false,
    }
  }

  function openCreateDialog(): void {
    if (disposed || savingHost.value || testingConnection.value) return
    editorVersion += 1
    draft.value = createEmptyDraft()

    authMode.value = preferredAuthMode(draft.value.protocol)
    directSecret.value = ''
    originalPasswordConfigured.value = false
    authModeChanged.value = false
    editingHostId.value = null
    formError.value = ''
    hostDialogOpen.value = true
    void loadCredentialOptions()
  }

  function openEditHostDialog(host: HostRecord): void {
    if (disposed || savingHost.value || testingConnection.value) return
    editorVersion += 1

    draft.value = hostToDraft(host)
    authMode.value = host.credentialId ? 'credential' : draft.value.passwordSource === 'none' ? 'none' : 'password'

    if (!hostProtocolAuthModes(draft.value.protocol).includes(authMode.value)) {
      authMode.value = preferredAuthMode(draft.value.protocol)
    }
    directSecret.value = ''
    originalPasswordConfigured.value = host.passwordConfigured === true
    authModeChanged.value = false
    editingHostId.value = host.id
    formError.value = ''
    hostDialogOpen.value = true
    void loadCredentialOptions()
  }

  async function loadCredentialOptions(): Promise<void> {
    const requestID = ++credentialOptionsRequestID.value
    const editingID = editingHostId.value
    const credentialID = draft.value.credentialId
    credentialOptionsLoading.value = true
    credentialOptionsError.value = ''
    try {
      const status = await credentialGateway.status()
      if (requestID !== credentialOptionsRequestID.value) return
      const options = status.state === 'unlocked' ? await credentialGateway.list() : []
      if (requestID !== credentialOptionsRequestID.value) return
      credentialVaultStatus.value = status
      credentialOptions.value = options
      if (credentialGateway.persistent && (status.state === 'setupRequired' || status.state === 'recoveryRequired')) {
        securityDialogOpen.value = true
      }
      if (editingID && credentialID && editingHostId.value === editingID && draft.value.credentialId === credentialID) {
        const current = credentialOptions.value.find((item) => item.id === credentialID)
        if (current && !authModeChanged.value) authMode.value = 'credential'
      }
    } catch (error) {
      if (requestID !== credentialOptionsRequestID.value) return
      credentialVaultStatus.value = null
      credentialOptions.value = []
      credentialOptionsError.value = credentialErrorMessage(error, t('shell.error.credentialRead'))
      if (credentialGateway.persistent) securityDialogOpen.value = true
    } finally {
      if (requestID === credentialOptionsRequestID.value) credentialOptionsLoading.value = false
    }
  }

  function credentialKindLabel(credential: CredentialRecord): string {
    const labels = {
      password: t('shell.hostEditor.password'),
      privateKey: t('shell.credential.privateKey'),
      privateKeyPassphrase: t('shell.credential.privateKeyPassphrase'),
      sshAgent: t('shell.credential.sshAgent'),
    }
    return labels[credential.kind]
  }

  function closeHostDialog(): void {
    editorVersion += 1
    credentialOptionsRequestID.value += 1
    credentialOptionsLoading.value = false
    directSecret.value = ''
    hostDialogOpen.value = false
    editingHostId.value = null
    originalPasswordConfigured.value = false
    authModeChanged.value = false
    formError.value = ''
  }

  function setAuthMode(mode: HostAuthMode): void {
    if (mode === 'manual') mode = 'password'
    if (mode !== authMode.value && editingHostId.value) authModeChanged.value = true

    if (mode !== 'password') {
      directSecret.value = ''
    } else if (draft.value.passwordSource === 'none') {
      draft.value.passwordSource = 'saved'
    }

    authMode.value = mode
  }

  function setPasswordSource(source: HostPasswordSource): void {
    if (source !== 'prompt' && source !== 'saved') return
    if (source !== draft.value.passwordSource) {
      authModeChanged.value = true
      directSecret.value = ''
    }
    draft.value.passwordSource = source
  }

  async function saveHost(): Promise<void> {
    if (disposed || savingHost.value || testingConnection.value || !hostDialogOpen.value) return
    const snapshot = { ...draft.value }
    const id = editingHostId.value
    const mode = authMode.value
    const version = editorVersion
    const name = snapshot.name.trim()
    const address = snapshot.address.trim()
    const username = snapshot.username.trim()
    const port = Number(snapshot.port)

    const usernameRequired = snapshot.protocol !== 'telnet' && snapshot.protocol !== 'vnc'
    if (!name || !address || (usernameRequired && !username)) {
      formError.value = t('shell.error.hostRequired')
      return
    }
    if (!Number.isInteger(port) || port < 1 || port > 65535) {
      formError.value = t('shell.error.portRange')
      return
    }

    const connectionVisible = hostProtocolShows(snapshot.protocol, 'connection')

    const proxyId = connectionVisible ? (snapshot.proxyId ?? '').trim() : ''
    const proxyMode =
      connectionVisible && !proxyId ? normalizeProxyMode(snapshot.proxyMode) : normalizeProxyMode('none')
    if (proxyMode !== 'none') {
      if (!(snapshot.proxyHost ?? '').trim()) {
        formError.value = t('shell.error.proxyHostRequired')
        return
      }
      const proxyPort = Number(snapshot.proxyPort)
      if (!Number.isInteger(proxyPort) || proxyPort < 1 || proxyPort > 65535) {
        formError.value = t('shell.error.proxyPortRange')
        return
      }
    }
    const jumpHostId = connectionVisible ? (snapshot.jumpHostId ?? '').trim() : ''

    if (snapshot.protocol === 'vnc' && vncUsesWebSocket(snapshot.vncTransport)) {
      const wsPathError = vncWsPathError(snapshot.vncWsPath)
      if (wsPathError) {
        formError.value = t(vncWsPathErrorKey(wsPathError))
        return
      }
    }

    const authVisible = hostProtocolShows(snapshot.protocol, 'auth')
    const draftSource = snapshot.passwordSource || 'prompt'
    const resolveSource = (): HostPasswordSource => {
      if (draftSource === 'saved' && directSecret.value) return 'saved'
      return draftSource === 'none' ? 'none' : 'prompt'
    }
    const effectiveSource: HostPasswordSource | undefined = authVisible ? draftSource : resolveSource()
    const passwordSource: HostPasswordSource | undefined =
      mode === 'password' ? effectiveSource || 'prompt' : mode === 'none' ? 'none' : undefined
    const secret = mode === 'password' && passwordSource === 'saved' ? directSecret.value : ''
    const preservesExistingSecret =
      mode === 'password' &&
      passwordSource === 'saved' &&
      !!id &&
      originalPasswordConfigured.value &&
      !secret &&
      !authModeChanged.value
    if (hostGateway.persistent && passwordSource === 'saved' && !secret && !preservesExistingSecret) {
      formError.value = t('shell.error.passwordRequired')
      return
    }
    if (hostGateway.persistent && authVisible && mode === 'credential' && !snapshot.credentialId) {
      formError.value = t('shell.error.credentialRequired')
      return
    }
    savingHost.value = true
    formError.value = ''
    try {
      if (secret && !(await ensureCredentialStorageReady())) {
        if (!disposed && version === editorVersion) formError.value = t('shell.error.vaultSetupRequired')
        return
      }

      if (disposed || version !== editorVersion) return
      directSecret.value = ''
      const credentialId = mode === 'credential' ? snapshot.credentialId : ''
      const input = {
        ...snapshot,
        name,
        address,
        port,
        username,
        group: snapshot.group.trim(),
        credentialId,
        passwordSource,
        password: secret,
        clearPassword: passwordSource !== 'saved' || !preservesExistingSecret,

        jumpHostId,
        proxyId,
        proxyMode,
        proxyHost: proxyMode === 'none' ? '' : (snapshot.proxyHost ?? '').trim(),
        proxyPort: proxyMode === 'none' ? 0 : Number(snapshot.proxyPort),
        proxyUsername: proxyMode === 'none' ? '' : (snapshot.proxyUsername ?? '').trim(),
        proxyPassword: proxyMode === 'none' ? '' : (snapshot.proxyPassword ?? ''),
        clearProxyPassword: proxyMode === 'none' ? true : snapshot.clearProxyPassword === true,
      }
      const host = id ? await hostGateway.update(id, input) : await hostGateway.create(input)
      if (disposed) return
      toast.success(id ? t('shell.success.hostUpdated') : t('shell.success.hostCreated'))
      if (version === editorVersion) closeHostDialog()
      await onSaved(host, !id)
    } catch (error) {
      if (!disposed) toast.error(hostErrorMessage(error, t('shell.error.saveHost')))
    } finally {
      savingHost.value = false
    }
  }

  async function testHostConnection(): Promise<void> {
    if (disposed || testingConnection.value || savingHost.value || !hostDialogOpen.value) return
    const version = editorVersion
    formError.value = ''

    const address = draft.value.address.trim()
    const username = draft.value.username.trim()
    const port = Number(draft.value.port)
    if (!address || !username) {
      toast.error(t('shell.error.connectionFields'))
      return
    }
    if (!Number.isInteger(port) || port < 1 || port > 65535) {
      toast.error(t('shell.error.portRange'))
      return
    }

    const input: SessionTestConnectionInput = {
      hostId: editingHostId.value || '',
      protocol: draft.value.protocol || 'ssh',
      address,
      port,
      username,
      credentialId: '',
      kind: '',
      secret: '',
    }
    if (authMode.value === 'credential') {
      if (!draft.value.credentialId) {
        toast.error(t('shell.error.credentialTestRequired'))
        return
      }
      input.credentialId = draft.value.credentialId
    } else if (authMode.value === 'password') {
      if (
        !directSecret.value &&
        (draft.value.passwordSource === 'prompt' || !editingHostId.value || !originalPasswordConfigured.value)
      ) {
        toast.error(t('shell.error.passwordTestRequired'))
        return
      }
      input.kind = 'password'
      input.secret = directSecret.value
    } else {
      toast.error(t('shell.error.testAuthRequired'))
      return
    }
    testingConnection.value = true
    try {
      await sessionGateway.testConnection(input)
      if (!disposed && version === editorVersion) toast.success(t('shell.success.connectionTest'))
    } catch (error) {
      if (!disposed && version === editorVersion) {
        if (decodeAppError(error)?.messageKey === 'session.hostkey_unknown_reachable') {
          toast.warning(sessionErrorMessage(error, t('shell.error.connectionTest')))
        } else {
          toast.error(sessionErrorMessage(error, t('shell.error.connectionTest')))
        }
      }
    } finally {
      testingConnection.value = false
    }
  }

  const detectingIcon = ref(false)

  async function detectHostIcon(): Promise<void> {
    if (disposed || detectingIcon.value || savingHost.value || !hostDialogOpen.value) return
    const version = editorVersion
    formError.value = ''

    const address = draft.value.address.trim()
    const username = draft.value.username.trim()
    const port = Number(draft.value.port)
    if (!address || !username) {
      toast.error(t('shell.error.connectionFields'))
      return
    }
    if (!Number.isInteger(port) || port < 1 || port > 65535) {
      toast.error(t('shell.error.portRange'))
      return
    }

    const input: SessionDetectIconInput = {
      hostId: editingHostId.value || '',
      protocol: draft.value.protocol || 'ssh',
      address,
      port,
      username,
      credentialId: '',
      kind: '',
      secret: '',
    }
    if (authMode.value === 'credential') {
      if (!draft.value.credentialId) {
        toast.error(t('shell.error.credentialTestRequired'))
        return
      }
      input.credentialId = draft.value.credentialId
    } else if (authMode.value === 'password') {
      input.kind = 'password'
      input.secret = directSecret.value
    }

    detectingIcon.value = true
    try {
      const icon = await sessionGateway.detectIcon(input)
      if (disposed || version !== editorVersion) return
      draft.value.icon = icon

      toast.success(t('shell.hostEditor.iconDetect.success', { icon: iconDisplayName(icon) }))
    } catch (error) {
      if (disposed || version !== editorVersion) return
      toast.error(sessionErrorMessage(error, t('shell.hostEditor.iconDetect.failed')))
    } finally {
      detectingIcon.value = false
    }
  }

  onScopeDispose(() => {
    disposed = true
    closeHostDialog()
  })
  return {
    hostDialogOpen,
    editingHostId,
    formError,
    savingHost,
    testingConnection,
    authMode,
    directSecret,
    draft,
    openCreateDialog,
    openEditHostDialog,
    credentialKindLabel,
    closeHostDialog,
    setAuthMode,
    setPasswordSource,
    saveHost,
    testHostConnection,
    detectHostIcon,
    detectingIcon,
  }
}
