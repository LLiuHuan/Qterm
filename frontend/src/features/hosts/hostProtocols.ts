import type { HostDraft } from '@/services/hosts.ts'
import type { HostAuthMode } from '../../types/qterm'

export type HostFormSection =
  'identity' | 'address' | 'identity-name' | 'auth' | 'connection' | 'group' | 'notes' | 'serial' | 'advanced'

export type HostProtocolId = 'ssh' | 'telnet' | 'rdp' | 'vnc' | 'serial'

export interface HostProtocolDefinition {
  id: HostProtocolId

  label: string

  labelKey?: string

  enabled: boolean

  defaultPort: number

  disabledHintKey?: string

  hintKey?: string

  usernameHintKey?: string

  sections: readonly HostFormSection[]

  authModes?: readonly HostAuthMode[]
}

const COMMON_SECTIONS: readonly HostFormSection[] = ['identity', 'address', 'identity-name', 'auth', 'group', 'notes']

export const HOST_PROTOCOLS: readonly HostProtocolDefinition[] = [
  {
    id: 'ssh',
    label: 'SSH',
    enabled: true,
    defaultPort: 22,
    sections: [...COMMON_SECTIONS, 'connection', 'advanced'],
    authModes: ['none', 'password', 'credential'],
  },
  {
    id: 'telnet',
    label: 'Telnet',
    enabled: true,
    defaultPort: 23,
    hintKey: 'shell.hostEditor.telnetHint',

    sections: ['identity', 'address', 'identity-name', 'group', 'notes', 'advanced'],
    authModes: ['none'],
  },
  {
    id: 'rdp',
    label: 'RDP',
    enabled: true,
    defaultPort: 3389,
    hintKey: 'shell.hostEditor.rdpHint',
    usernameHintKey: 'shell.hostEditor.rdpUsernameHint',

    sections: [...COMMON_SECTIONS, 'connection', 'advanced'],

    authModes: ['password', 'credential'],
  },
  {
    id: 'vnc',
    label: 'VNC',
    enabled: true,
    defaultPort: 5900,
    hintKey: 'shell.hostEditor.vncHint',

    sections: ['identity', 'address', 'identity-name', 'auth', 'connection', 'group', 'notes', 'advanced'],
    usernameHintKey: 'shell.hostEditor.vncUsernameHint',

    authModes: ['none', 'password', 'credential'],
  },
  {
    id: 'serial',
    label: 'Serial',
    labelKey: 'shell.hostEditor.protocolSerial',
    enabled: true,

    defaultPort: 0,
    hintKey: 'shell.hostEditor.serialHint',

    sections: ['identity', 'serial', 'group', 'notes', 'advanced'],
    authModes: ['none'],
  },
]

export function hostProtocolDefinition(id?: string | null): HostProtocolDefinition {
  const found = HOST_PROTOCOLS.find((item) => item.id === id)
  if (found) {
    return found
  }
  return HOST_PROTOCOLS[0]
}

export function hostProtocolShows(id: string | undefined | null, section: HostFormSection): boolean {
  return hostProtocolDefinition(id).sections.includes(section)
}

export function hostProtocolAuthModes(id: string | undefined | null): readonly HostAuthMode[] {
  return hostProtocolDefinition(id).authModes ?? ['none', 'password', 'credential']
}

export function shouldAdoptDefaultPort(currentPort: number | undefined | null, next: HostProtocolDefinition): boolean {
  if (!currentPort || currentPort <= 0) {
    return true
  }
  return HOST_PROTOCOLS.some((item) => item.id !== next.id && item.defaultPort === currentPort)
}

export interface HostVncTransportOptions {
  vncTransport?: VncTransport

  vncWsPath?: string

  vncAllowInsecureTls?: boolean
}

export type HostEditorDraft = HostDraft & HostVncTransportOptions

export type VncTransport = 'tcp' | 'wss'

export const VNC_TRANSPORT_TCP: VncTransport = 'tcp'
export const VNC_TRANSPORT_WSS: VncTransport = 'wss'

export function normalizeVncTransport(value: unknown): VncTransport {
  return value === VNC_TRANSPORT_WSS ? VNC_TRANSPORT_WSS : VNC_TRANSPORT_TCP
}

export function vncUsesWebSocket(value: unknown): boolean {
  return normalizeVncTransport(value) === VNC_TRANSPORT_WSS
}

export function vncWsPathForInput(value: unknown): string {
  const raw = typeof value === 'string' ? value : ''
  return raw === '/' ? '' : raw
}

export type VncWsPathError = 'relative' | 'traversal' | 'whitespace'

const VNC_WS_PATH_ERROR_KEYS: Record<VncWsPathError, string> = {
  relative: 'shell.hostEditor.vnc.wsPathErrorRelative',
  traversal: 'shell.hostEditor.vnc.wsPathErrorTraversal',
  whitespace: 'shell.hostEditor.vnc.wsPathErrorWhitespace',
}

export function vncWsPathError(value: unknown): VncWsPathError | null {
  const raw = (typeof value === 'string' ? value : '').trim()
  if (raw === '') return null
  if (!raw.startsWith('/')) return 'relative'
  if (raw.includes('..')) return 'traversal'
  if (/[ \t\r\n\v\f]/.test(raw)) return 'whitespace'
  return null
}

export function vncWsPathErrorKey(error: VncWsPathError | null): string {
  return error ? VNC_WS_PATH_ERROR_KEYS[error] : ''
}
