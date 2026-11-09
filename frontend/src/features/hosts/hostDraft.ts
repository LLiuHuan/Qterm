import { defaultTerminalTypeFor, normalizeProxyMode, type HostRecord } from '@/services/hosts.ts'
import {
  normalizeVncTransport,
  vncWsPathForInput,
  type HostEditorDraft,
  type HostVncTransportOptions,
} from './hostProtocols'

export function hostToDraft(host: HostRecord): HostEditorDraft {
  const vncFields = host as HostRecord & HostVncTransportOptions
  return {
    icon: host.icon || '',

    color: host.color || '',
    name: host.name,
    protocol: host.protocol || 'ssh',
    address: host.address,
    port: host.port,
    username: host.username,
    group: host.group || '',
    credentialId: host.credentialId,
    passwordSource: host.passwordSource || (host.passwordConfigured ? 'saved' : 'none'),
    sshTwoFactorEnabled: host.sshTwoFactorEnabled ?? false,
    sshOtpEntryId: host.sshOtpEntryId || '',
    sshOtpAutoFill: host.sshOtpAutoFill ?? false,
    notes: host.notes || '',
    tags: [...(host.tags ?? [])],
    sshKeepAlive: host.sshKeepAlive ?? true,
    latencyDetection: host.latencyDetection ?? true,
    autoDetectOSIcon: host.autoDetectOSIcon ?? true,
    directoryFollow: host.directoryFollow || 'inherit',
    sftpShowHiddenFiles: host.sftpShowHiddenFiles || 'inherit',
    sftpDefaultDirectory: host.sftpDefaultDirectory || '',
    backspaceMode: host.backspaceMode || 'del',
    encoding: host.encoding || 'utf-8',
    connectTimeoutSeconds: host.connectTimeoutSeconds || 15,
    postLoginCommand: host.postLoginCommand || '',
    postLoginCommandDelaySeconds: host.postLoginCommandDelaySeconds ?? 0,
    environmentVariables: (host.environmentVariables ?? []).map((item) => ({ ...item })),
    connectionType: host.connectionType || 'standard',
    terminalType: host.terminalType || defaultTerminalTypeFor(host.connectionType),
    x11Forwarding: host.x11Forwarding ?? false,
    x11Trusted: host.x11Trusted ?? true,

    vncViewOnly: host.vncViewOnly ?? false,
    vncShared: host.vncShared ?? true,
    vncPixelDepth: host.vncPixelDepth ?? 32,

    vncScale: host.vncScale ?? 'fit',
    vncAutoReconnect: host.vncAutoReconnect ?? false,

    vncAutoReconnectAttempts: host.vncAutoReconnectAttempts ?? 5,

    vncTransport: normalizeVncTransport(vncFields.vncTransport),
    vncWsPath: vncWsPathForInput(vncFields.vncWsPath),
    vncAllowInsecureTls: vncFields.vncAllowInsecureTls === true,

    vncRfbPasswordMode: host.vncRfbPasswordMode === 'same' ? 'same' : 'auto',

    rdpDisplayMode: host.rdpDisplayMode || 'fit',
    rdpWidth: host.rdpWidth ?? 1280,
    rdpHeight: host.rdpHeight ?? 800,
    rdpClipboardMode: host.rdpClipboardMode || 'both',
    rdpClipboardContent: host.rdpClipboardContent || 'text',

    jumpHostId: host.jumpHostId || '',

    proxyId: host.proxyId || '',
    proxyMode: normalizeProxyMode(host.proxyMode),
    proxyHost: host.proxyHost || '',
    proxyPort: host.proxyPort ?? 0,
    proxyUsername: host.proxyUsername || '',
    proxyPassword: '',
    clearProxyPassword: false,
    proxyPasswordConfigured: host.proxyPasswordConfigured === true,
  }
}
