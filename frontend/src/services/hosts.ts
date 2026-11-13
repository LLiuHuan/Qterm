import { errorMessage } from './errors'

export type HostSftpDirectoryFollowMode = 'inherit' | 'enabled' | 'disabled'

export type HostConnectionType = 'standard' | 'network'

export type HostTerminalType = 'vt100' | 'vt200' | 'xterm' | 'xterm-256color' | 'ansi' | 'linux'

export function defaultTerminalTypeFor(connectionType: HostConnectionType | undefined): HostTerminalType {
  return connectionType === 'network' ? 'vt100' : 'xterm-256color'
}

export type HostBackspaceMode = 'del' | 'bs'

export type HostTerminalEncoding = 'utf-8' | 'gbk'

export type HostVncRfbPasswordMode = 'auto' | 'same'

export interface HostEnvironmentVariable {
  name: string
  value: string
}
export type HostPasswordSource = 'prompt' | 'saved' | 'none'

export type HostProxyMode = 'none' | 'socks5' | 'http'

export function normalizeProxyMode(value: unknown): HostProxyMode {
  return value === 'socks5' || value === 'http' ? value : 'none'
}

export interface HostDraft {
  icon?: string

  color?: string
  name: string
  protocol?: string
  address: string
  port: number
  username: string
  group: string
  credentialId: string

  passwordSource?: HostPasswordSource

  sshTwoFactorEnabled?: boolean
  sshOtpEntryId?: string
  sshOtpAutoFill?: boolean
  notes?: string

  sshKeepAlive?: boolean
  latencyDetection?: boolean
  autoDetectOSIcon?: boolean

  directoryFollow?: HostSftpDirectoryFollowMode

  sftpShowHiddenFiles?: HostSftpDirectoryFollowMode

  sftpDefaultDirectory?: string

  backspaceMode?: HostBackspaceMode

  encoding?: HostTerminalEncoding

  connectTimeoutSeconds?: number

  postLoginCommand?: string

  postLoginCommandDelaySeconds?: number

  environmentVariables?: HostEnvironmentVariable[]

  serialBaud?: number

  serialDataBits?: number

  serialParity?: string

  serialStopBits?: string

  connectionType?: HostConnectionType

  terminalType?: HostTerminalType

  x11Forwarding?: boolean

  rdpDisplayMode?: string

  rdpWidth?: number
  rdpHeight?: number

  rdpClipboardMode?: string

  rdpClipboardContent?: string

  vncViewOnly?: boolean

  vncShared?: boolean

  vncPixelDepth?: number

  vncScale?: string

  vncAutoReconnect?: boolean

  vncAutoReconnectAttempts?: number

  vncTransport?: 'tcp' | 'wss'

  vncWsPath?: string

  vncAllowInsecureTls?: boolean

  vncRfbPasswordMode?: HostVncRfbPasswordMode

  tags?: string[]

  x11Trusted?: boolean

  jumpHostId?: string

  proxyId?: string

  proxyMode?: HostProxyMode

  proxyHost?: string

  proxyPort?: number

  proxyUsername?: string

  proxyPassword?: string

  clearProxyPassword?: boolean

  proxyPasswordConfigured?: boolean

  password?: string
  clearPassword?: boolean
}

export interface HostRecord extends Omit<
  HostDraft,
  'password' | 'clearPassword' | 'proxyPassword' | 'clearProxyPassword' | 'proxyPasswordConfigured'
> {
  id: string
  groupId: string
  sortOrder: number
  createdAt: string
  updatedAt: string
  passwordConfigured?: boolean
  proxyPasswordConfigured?: boolean
}

type HostProxyFields = Required<Pick<HostDraft, 'proxyMode' | 'proxyHost' | 'proxyPort' | 'proxyUsername'>>

function proxyFields(input: HostDraft, fallback?: HostRecord): HostProxyFields {
  const proxyMode = normalizeProxyMode(input.proxyMode ?? fallback?.proxyMode)
  if (proxyMode === 'none') {
    return { proxyMode, proxyHost: '', proxyPort: 0, proxyUsername: '' }
  }
  return {
    proxyMode,
    proxyHost: input.proxyHost ?? fallback?.proxyHost ?? '',
    proxyPort: input.proxyPort ?? fallback?.proxyPort ?? 0,
    proxyUsername: input.proxyUsername ?? fallback?.proxyUsername ?? '',
  }
}

export interface GroupRecord {
  id: string
  name: string
  parentId?: string
  sortOrder: number
  createdAt: string
  updatedAt: string
}

export interface HostGateway {
  readonly persistent: boolean
  list(): Promise<HostRecord[]>
  listGroups(): Promise<GroupRecord[]>
  create(input: HostDraft): Promise<HostRecord>
  update(id: string, input: HostDraft): Promise<HostRecord>

  duplicate(id: string, name: string): Promise<HostRecord>
  remove(id: string): Promise<void>
  createGroup(name: string, parentId?: string): Promise<GroupRecord>
  updateGroup(id: string, name: string): Promise<GroupRecord>
  removeGroup(id: string): Promise<void>
  moveHost(id: string, groupId: string): Promise<void>
  reorderHost(id: string, groupId: string, targetId: string, position: string): Promise<void>
  moveGroup(id: string, parentId: string): Promise<void>
  reorderGroup(id: string, parentId: string, targetId: string, position: string): Promise<void>
}

interface WailsWindow extends Window {
  _wails?: {
    environment?: {
      OS?: string
    }
  }
}

class MemoryHostGateway implements HostGateway {
  readonly persistent = false
  private readonly hosts: HostRecord[] = []
  private readonly groups: GroupRecord[] = [createMemoryGroup('default', '默认分组')]

  async list(): Promise<HostRecord[]> {
    return this.hosts.map((host) => ({ ...host }))
  }

  async listGroups(): Promise<GroupRecord[]> {
    return this.groups.map((group) => ({ ...group }))
  }

  async create(input: HostDraft): Promise<HostRecord> {
    const now = new Date().toISOString()
    const group = input.group.trim() ? this.ensureGroup(input.group) : undefined
    const safeInput = {
      ...input,
      sshKeepAlive: input.sshKeepAlive ?? true,
      latencyDetection: input.latencyDetection ?? true,
      autoDetectOSIcon: input.autoDetectOSIcon ?? true,
      directoryFollow: input.directoryFollow ?? 'inherit',
      sftpShowHiddenFiles: input.sftpShowHiddenFiles ?? 'inherit',
      sftpDefaultDirectory: input.sftpDefaultDirectory ?? '',
      backspaceMode: input.backspaceMode ?? 'del',
      encoding: input.encoding ?? 'utf-8',
      connectTimeoutSeconds: input.connectTimeoutSeconds ?? 15,
      postLoginCommand: input.postLoginCommand ?? '',
      postLoginCommandDelaySeconds: input.postLoginCommandDelaySeconds ?? 0,
      environmentVariables: input.environmentVariables ?? [],
      connectionType: input.connectionType ?? 'standard',
      terminalType: input.terminalType || defaultTerminalTypeFor(input.connectionType),
      x11Forwarding: input.x11Forwarding ?? false,
      rdpDisplayMode: input.rdpDisplayMode ?? 'fit',
      rdpWidth: input.rdpWidth ?? 1280,
      rdpHeight: input.rdpHeight ?? 800,
      rdpClipboardMode: input.rdpClipboardMode ?? 'both',
      rdpClipboardContent: input.rdpClipboardContent ?? 'text',

      vncViewOnly: input.vncViewOnly ?? false,
      vncShared: input.vncShared ?? true,
      vncPixelDepth: input.vncPixelDepth ?? 32,

      vncScale: input.vncScale ?? 'fit',
      vncAutoReconnect: input.vncAutoReconnect ?? false,
      vncAutoReconnectAttempts: input.vncAutoReconnectAttempts ?? 5,

      vncTransport: input.vncTransport ?? 'tcp',
      vncWsPath: input.vncWsPath ?? '/',
      vncAllowInsecureTls: input.vncAllowInsecureTls ?? false,

      vncRfbPasswordMode: input.vncRfbPasswordMode ?? 'auto',
      tags: input.tags ?? [],
      x11Trusted: input.x11Trusted ?? true,

      jumpHostId: input.jumpHostId ?? '',

      proxyId: input.proxyId ?? '',
      ...proxyFields(input),
      passwordSource: input.passwordSource ?? (input.password ? 'saved' : 'none'),
    }
    delete safeInput.password
    delete safeInput.clearPassword
    delete safeInput.proxyPassword
    delete safeInput.clearProxyPassword
    delete safeInput.proxyPasswordConfigured
    const host: HostRecord = {
      ...safeInput,
      id: createID('host'),
      groupId: group?.id || '',
      group: group?.name || '',
      sortOrder: this.nextHostOrder(group?.id || ''),
      createdAt: now,
      updatedAt: now,
      passwordConfigured: !safeInput.credentialId && Boolean(input.password),
      proxyPasswordConfigured: safeInput.proxyMode !== 'none' && Boolean(input.proxyPassword),
    }
    this.hosts.push(host)
    return { ...host }
  }

  async update(id: string, input: HostDraft): Promise<HostRecord> {
    const index = this.hosts.findIndex((host) => host.id === id)
    if (index < 0) throw new Error('主机不存在')

    const group = input.group.trim() ? this.ensureGroup(input.group) : undefined
    const safeInput = {
      ...input,
      sshKeepAlive: input.sshKeepAlive ?? this.hosts[index].sshKeepAlive ?? true,
      latencyDetection: input.latencyDetection ?? this.hosts[index].latencyDetection ?? true,
      autoDetectOSIcon: input.autoDetectOSIcon ?? this.hosts[index].autoDetectOSIcon ?? true,
      directoryFollow: input.directoryFollow ?? this.hosts[index].directoryFollow ?? 'inherit',
      sftpShowHiddenFiles: input.sftpShowHiddenFiles ?? this.hosts[index].sftpShowHiddenFiles ?? 'inherit',
      sftpDefaultDirectory: input.sftpDefaultDirectory ?? this.hosts[index].sftpDefaultDirectory ?? '',
      backspaceMode: input.backspaceMode ?? this.hosts[index].backspaceMode ?? 'del',
      encoding: input.encoding ?? this.hosts[index].encoding ?? 'utf-8',
      connectTimeoutSeconds: input.connectTimeoutSeconds ?? this.hosts[index].connectTimeoutSeconds ?? 15,
      postLoginCommand: input.postLoginCommand ?? this.hosts[index].postLoginCommand ?? '',
      postLoginCommandDelaySeconds:
        input.postLoginCommandDelaySeconds ?? this.hosts[index].postLoginCommandDelaySeconds ?? 0,
      environmentVariables: input.environmentVariables ?? this.hosts[index].environmentVariables ?? [],
      connectionType: input.connectionType ?? this.hosts[index].connectionType ?? 'standard',
      terminalType:
        input.terminalType ||
        this.hosts[index].terminalType ||
        defaultTerminalTypeFor(input.connectionType ?? this.hosts[index].connectionType),
      x11Forwarding: input.x11Forwarding ?? this.hosts[index].x11Forwarding ?? false,
      rdpDisplayMode: input.rdpDisplayMode ?? this.hosts[index].rdpDisplayMode ?? 'fit',
      rdpWidth: input.rdpWidth ?? this.hosts[index].rdpWidth ?? 1280,
      rdpHeight: input.rdpHeight ?? this.hosts[index].rdpHeight ?? 800,
      rdpClipboardMode: input.rdpClipboardMode ?? this.hosts[index].rdpClipboardMode ?? 'both',
      rdpClipboardContent: input.rdpClipboardContent ?? this.hosts[index].rdpClipboardContent ?? 'text',
      vncViewOnly: input.vncViewOnly ?? this.hosts[index].vncViewOnly ?? false,
      vncShared: input.vncShared ?? this.hosts[index].vncShared ?? true,
      vncPixelDepth: input.vncPixelDepth ?? this.hosts[index].vncPixelDepth ?? 32,
      vncScale: input.vncScale ?? this.hosts[index].vncScale ?? 'fit',
      vncAutoReconnect: input.vncAutoReconnect ?? this.hosts[index].vncAutoReconnect ?? false,
      vncAutoReconnectAttempts: input.vncAutoReconnectAttempts ?? this.hosts[index].vncAutoReconnectAttempts ?? 5,

      vncTransport: input.vncTransport ?? this.hosts[index].vncTransport ?? 'tcp',
      vncWsPath: input.vncWsPath ?? this.hosts[index].vncWsPath ?? '/',
      vncAllowInsecureTls: input.vncAllowInsecureTls ?? this.hosts[index].vncAllowInsecureTls ?? false,

      vncRfbPasswordMode: input.vncRfbPasswordMode ?? this.hosts[index].vncRfbPasswordMode ?? 'auto',
      tags: input.tags ?? this.hosts[index].tags ?? [],
      x11Trusted: input.x11Trusted ?? this.hosts[index].x11Trusted ?? true,

      jumpHostId: input.jumpHostId ?? this.hosts[index].jumpHostId ?? '',
      proxyId: input.proxyId ?? this.hosts[index].proxyId ?? '',
      ...proxyFields(input, this.hosts[index]),
      passwordSource: input.passwordSource ?? this.hosts[index].passwordSource ?? 'none',
    }
    delete safeInput.password
    delete safeInput.clearPassword
    delete safeInput.proxyPassword
    delete safeInput.clearProxyPassword
    delete safeInput.proxyPasswordConfigured
    const host: HostRecord = {
      ...this.hosts[index],
      ...safeInput,
      groupId: group?.id || '',
      group: group?.name || '',
      updatedAt: new Date().toISOString(),
      passwordConfigured: safeInput.credentialId
        ? false
        : input.clearPassword
          ? Boolean(input.password)
          : this.hosts[index].passwordConfigured === true,
      proxyPasswordConfigured:
        safeInput.proxyMode === 'none'
          ? false
          : input.clearProxyPassword
            ? Boolean(input.proxyPassword)
            : input.proxyPassword
              ? true
              : this.hosts[index].proxyPasswordConfigured === true,
    }
    this.hosts[index] = host
    return { ...host }
  }

  async duplicate(id: string, name: string): Promise<HostRecord> {
    const index = this.hosts.findIndex((host) => host.id === id)
    if (index < 0) throw new Error('主机不存在')
    const source = this.hosts[index]
    const now = new Date().toISOString()

    const host: HostRecord = {
      ...source,
      id: createID('host'),
      name: name.trim() || source.name,
      sortOrder: this.nextHostOrder(source.groupId),
      createdAt: now,
      updatedAt: now,
    }
    this.hosts.push(host)
    return { ...host }
  }

  async remove(id: string): Promise<void> {
    const index = this.hosts.findIndex((host) => host.id === id)
    if (index >= 0) this.hosts.splice(index, 1)
  }

  private nextHostOrder(groupId: string): number {
    const siblings = this.hosts.filter((host) => host.groupId === groupId)
    return siblings.reduce((max, host) => Math.max(max, host.sortOrder), -1) + 1
  }

  async moveHost(id: string, groupId: string): Promise<void> {
    await this.reorderHost(id, groupId, '', 'inside')
  }

  async reorderHost(id: string, groupId: string, targetId: string, position: string): Promise<void> {
    const host = this.hosts.find((item) => item.id === id)
    if (!host) throw new Error('主机不存在')
    if (groupId && !this.groups.some((item) => item.id === groupId)) throw new Error('分组不存在')
    const entries = [
      ...this.groups
        .filter((item) => (item.parentId || '') === groupId)
        .map((value) => ({ kind: 'group' as const, value })),
      ...this.hosts
        .filter((item) => item.groupId === groupId && item.id !== id)
        .map((value) => ({ kind: 'host' as const, value })),
    ].sort((a, b) => a.value.sortOrder - b.value.sortOrder || a.value.name.localeCompare(b.value.name))
    const targetIndex = targetId && position !== 'inside' ? entries.findIndex((item) => item.value.id === targetId) : -1
    const insertAt = targetIndex < 0 ? entries.length : targetIndex + (position === 'after' ? 1 : 0)
    entries.splice(insertAt, 0, { kind: 'host', value: host })
    const group = this.groups.find((item) => item.id === groupId)
    for (const [sortOrder, item] of entries.entries()) {
      item.value.sortOrder = sortOrder
      item.value.updatedAt = new Date().toISOString()
      if (item.kind === 'host') {
        item.value.groupId = groupId
        item.value.group = group?.name || ''
      }
    }
  }

  async moveGroup(id: string, parentId: string): Promise<void> {
    await this.reorderGroup(id, parentId, '', 'inside')
  }

  async reorderGroup(id: string, parentId: string, targetId: string, position: string): Promise<void> {
    const group = this.groups.find((item) => item.id === id)
    if (!group) throw new Error('分组不存在')
    if (id === 'default') throw new Error('默认分组不能移动')
    if (parentId && !this.groups.some((item) => item.id === parentId)) throw new Error('分组不存在')
    if (parentId === id || this.isDescendant(parentId, id)) throw new Error('不能将分组移动到自身或子分组下')
    const entries = [
      ...this.groups
        .filter((item) => (item.parentId || '') === parentId && item.id !== id)
        .map((value) => ({ kind: 'group' as const, value })),
      ...this.hosts.filter((item) => item.groupId === parentId).map((value) => ({ kind: 'host' as const, value })),
    ].sort((a, b) => a.value.sortOrder - b.value.sortOrder || a.value.name.localeCompare(b.value.name))
    const targetIndex = targetId && position !== 'inside' ? entries.findIndex((item) => item.value.id === targetId) : -1
    const insertAt = targetIndex < 0 ? entries.length : targetIndex + (position === 'after' ? 1 : 0)
    entries.splice(insertAt, 0, { kind: 'group', value: group })
    for (const [sortOrder, item] of entries.entries()) {
      item.value.sortOrder = sortOrder
      item.value.updatedAt = new Date().toISOString()
      if (item.kind === 'group') item.value.parentId = parentId || undefined
    }
  }

  async createGroup(name: string, parentId?: string): Promise<GroupRecord> {
    const normalizedName = requireGroupName(name)
    if (this.findGroup(normalizedName)) throw new Error('分组名称已存在')
    const group = createMemoryGroup(createID('group'), normalizedName)
    group.parentId = parentId || undefined
    group.sortOrder = this.groups.filter((item) => (item.parentId || '') === (parentId || '')).length
    this.groups.push(group)
    return { ...group }
  }

  async updateGroup(id: string, name: string): Promise<GroupRecord> {
    if (id === 'default') throw new Error('默认分组不能重命名')
    const index = this.groups.findIndex((group) => group.id === id)
    if (index < 0) throw new Error('分组不存在')
    const normalizedName = requireGroupName(name)
    const conflict = this.findGroup(normalizedName)
    if (conflict && conflict.id !== id) throw new Error('分组名称已存在')
    this.groups[index] = { ...this.groups[index], name: normalizedName, updatedAt: new Date().toISOString() }
    for (const host of this.hosts) if (host.groupId === id) host.group = normalizedName
    return { ...this.groups[index] }
  }

  async removeGroup(id: string): Promise<void> {
    if (id === 'default') throw new Error('默认分组不能删除')
    if (this.hosts.some((host) => host.groupId === id) || this.groups.some((group) => group.parentId === id))
      throw new Error('请先移动或删除分组内的主机和子分组')
    const index = this.groups.findIndex((group) => group.id === id)
    if (index < 0) throw new Error('分组不存在')
    this.groups.splice(index, 1)
  }

  private isDescendant(candidateID: string, ancestorID: string): boolean {
    let current = this.groups.find((group) => group.id === candidateID)
    const visited = new Set<string>()
    while (current?.parentId) {
      if (current.parentId === ancestorID) return true
      if (visited.has(current.parentId)) return true
      visited.add(current.parentId)
      current = this.groups.find((group) => group.id === current?.parentId)
    }
    return false
  }

  private ensureGroup(name: string): GroupRecord {
    const normalizedName = name.trim() || '默认分组'
    const existing = this.findGroup(normalizedName)
    if (existing) return existing
    const group = createMemoryGroup(createID('group'), normalizedName)
    this.groups.push(group)
    return group
  }

  private findGroup(name: string): GroupRecord | undefined {
    const normalizedName = name.trim().toLocaleLowerCase()
    return this.groups.find((group) => group.name.toLocaleLowerCase() === normalizedName)
  }
}

function toRecord(host: HostRecord): HostRecord {
  return {
    ...host,
    icon: host.icon || '',
    color: host.color || '',
    protocol: host.protocol || 'ssh',
    serialBaud: host.serialBaud || 115200,
    serialDataBits: host.serialDataBits || 8,
    serialParity: host.serialParity || 'none',
    serialStopBits: host.serialStopBits || '1',
    notes: host.notes || '',
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
    environmentVariables: host.environmentVariables ?? [],
    connectionType: host.connectionType || 'standard',
    terminalType: host.terminalType || defaultTerminalTypeFor(host.connectionType),
    x11Forwarding: host.x11Forwarding ?? false,
    x11Trusted: host.x11Trusted ?? true,
    rdpClipboardMode: host.rdpClipboardMode || 'both',
    rdpClipboardContent: host.rdpClipboardContent || 'text',
    vncViewOnly: host.vncViewOnly ?? false,
    vncShared: host.vncShared ?? true,
    vncPixelDepth: host.vncPixelDepth || 32,

    vncScale: host.vncScale || 'fit',
    vncAutoReconnect: host.vncAutoReconnect ?? false,
    vncAutoReconnectAttempts: host.vncAutoReconnectAttempts ?? 5,

    vncTransport: host.vncTransport || 'tcp',
    vncWsPath: host.vncWsPath || '/',
    vncAllowInsecureTls: host.vncAllowInsecureTls ?? false,

    vncRfbPasswordMode: host.vncRfbPasswordMode || 'auto',

    jumpHostId: host.jumpHostId || '',

    proxyId: host.proxyId || '',
    proxyMode: normalizeProxyMode(host.proxyMode),
    proxyHost: host.proxyHost || '',

    proxyPort: host.proxyPort ?? 0,
    proxyUsername: host.proxyUsername || '',
    proxyPasswordConfigured: host.proxyPasswordConfigured === true,
    tags: host.tags ?? [],
    passwordSource: host.passwordSource || (host.passwordConfigured ? 'saved' : 'none'),
    sortOrder: Number(host.sortOrder ?? 0),
  }
}

class WailsHostGateway implements HostGateway {
  readonly persistent = true

  async list(): Promise<HostRecord[]> {
    const { ListHosts } = await import('../../bindings/changeme/internal/app/service')
    const hosts = await ListHosts()
    return (hosts ?? []).map((host) => toRecord(host as HostRecord))
  }

  async listGroups(): Promise<GroupRecord[]> {
    const { ListGroups } = await import('../../bindings/changeme/internal/app/service')
    return (await ListGroups()) ?? []
  }

  async create(input: HostDraft): Promise<HostRecord> {
    const { CreateHost } = await import('../../bindings/changeme/internal/app/service')

    const payload = {
      ...input,
      icon: input.icon || '',

      color: input.color || '',
      protocol: input.protocol || 'ssh',
      serialBaud: input.serialBaud ?? 0,
      serialDataBits: input.serialDataBits ?? 8,
      serialParity: input.serialParity ?? 'none',
      serialStopBits: input.serialStopBits ?? '1',
      vncScale: input.vncScale ?? 'fit',
      vncAutoReconnect: input.vncAutoReconnect ?? false,
      vncAutoReconnectAttempts: input.vncAutoReconnectAttempts ?? 5,

      vncTransport: input.vncTransport ?? 'tcp',
      vncWsPath: input.vncWsPath ?? '/',
      vncAllowInsecureTls: input.vncAllowInsecureTls ?? false,

      vncRfbPasswordMode: input.vncRfbPasswordMode ?? 'auto',

      jumpHostId: input.jumpHostId ?? '',

      proxyId: input.proxyId ?? '',
      ...proxyFields(input),
    } as Parameters<typeof CreateHost>[0]

    return toRecord((await CreateHost(payload)) as HostRecord)
  }

  async update(id: string, input: HostDraft): Promise<HostRecord> {
    const { UpdateHost } = await import('../../bindings/changeme/internal/app/service')

    const payload = {
      ...input,
      icon: input.icon || '',

      color: input.color || '',
      protocol: input.protocol || 'ssh',
      serialBaud: input.serialBaud ?? 0,
      serialDataBits: input.serialDataBits ?? 8,
      serialParity: input.serialParity ?? 'none',
      serialStopBits: input.serialStopBits ?? '1',
      vncScale: input.vncScale ?? 'fit',
      vncAutoReconnect: input.vncAutoReconnect ?? false,
      vncAutoReconnectAttempts: input.vncAutoReconnectAttempts ?? 5,
      vncTransport: input.vncTransport ?? 'tcp',
      vncWsPath: input.vncWsPath ?? '/',
      vncAllowInsecureTls: input.vncAllowInsecureTls ?? false,
      vncRfbPasswordMode: input.vncRfbPasswordMode ?? 'auto',

      jumpHostId: input.jumpHostId ?? '',
      proxyId: input.proxyId ?? '',
      ...proxyFields(input),
    } as Parameters<typeof UpdateHost>[1]
    return toRecord((await UpdateHost(id, payload)) as HostRecord)
  }

  async duplicate(id: string, name: string): Promise<HostRecord> {
    const { DuplicateHost } = await import('../../bindings/changeme/internal/app/service')

    return toRecord((await DuplicateHost(id, name)) as HostRecord)
  }

  async remove(id: string): Promise<void> {
    const { DeleteHost } = await import('../../bindings/changeme/internal/app/service')
    await DeleteHost(id)
  }

  async createGroup(name: string, parentId?: string): Promise<GroupRecord> {
    const { CreateGroup } = await import('../../bindings/changeme/internal/app/service')
    return CreateGroup({ name, parentId: parentId || '' }) as Promise<GroupRecord>
  }

  async updateGroup(id: string, name: string): Promise<GroupRecord> {
    const { UpdateGroup } = await import('../../bindings/changeme/internal/app/service')
    return UpdateGroup(id, { name })
  }

  async removeGroup(id: string): Promise<void> {
    const { DeleteGroup } = await import('../../bindings/changeme/internal/app/service')
    await DeleteGroup(id)
  }

  async moveHost(id: string, groupId: string): Promise<void> {
    const { MoveHost } = await import('../../bindings/changeme/internal/app/service')
    await MoveHost(id, groupId)
  }
  async reorderHost(id: string, groupId: string, targetId: string, position: string): Promise<void> {
    const { ReorderHost } = await import('../../bindings/changeme/internal/app/service')
    await ReorderHost(id, groupId, targetId, position)
  }
  async moveGroup(id: string, parentId: string): Promise<void> {
    const { MoveGroup } = await import('../../bindings/changeme/internal/app/service')
    await MoveGroup(id, parentId)
  }
  async reorderGroup(id: string, parentId: string, targetId: string, position: string): Promise<void> {
    const { ReorderGroup } = await import('../../bindings/changeme/internal/app/service')
    await ReorderGroup(id, parentId, targetId, position)
  }
}

class DeferredHostGateway implements HostGateway {
  private readonly memory = new MemoryHostGateway()
  private readonly wails = new WailsHostGateway()

  private get target(): HostGateway {
    return shouldUseWailsGateway() ? this.wails : this.memory
  }

  get persistent(): boolean {
    return this.target.persistent
  }
  list(): Promise<HostRecord[]> {
    return this.target.list()
  }
  listGroups(): Promise<GroupRecord[]> {
    return this.target.listGroups()
  }
  create(input: HostDraft): Promise<HostRecord> {
    return this.target.create(input)
  }
  update(id: string, input: HostDraft): Promise<HostRecord> {
    return this.target.update(id, input)
  }
  duplicate(id: string, name: string): Promise<HostRecord> {
    return this.target.duplicate(id, name)
  }
  remove(id: string): Promise<void> {
    return this.target.remove(id)
  }
  createGroup(name: string, parentId?: string): Promise<GroupRecord> {
    return this.target.createGroup(name, parentId)
  }
  updateGroup(id: string, name: string): Promise<GroupRecord> {
    return this.target.updateGroup(id, name)
  }
  removeGroup(id: string): Promise<void> {
    return this.target.removeGroup(id)
  }
  moveHost(id: string, groupId: string): Promise<void> {
    return this.target.moveHost(id, groupId)
  }
  reorderHost(id: string, groupId: string, targetId: string, position: string): Promise<void> {
    return this.target.reorderHost(id, groupId, targetId, position)
  }
  moveGroup(id: string, parentId: string): Promise<void> {
    return this.target.moveGroup(id, parentId)
  }
  reorderGroup(id: string, parentId: string, targetId: string, position: string): Promise<void> {
    return this.target.reorderGroup(id, parentId, targetId, position)
  }
}

export function createHostGateway(): HostGateway {
  return new DeferredHostGateway()
}

export function hostErrorMessage(error: unknown, fallback: string): string {
  return errorMessage(error, fallback)
}

function shouldUseWailsGateway(): boolean {
  const runtimeWindow = window as WailsWindow
  const query = new URLSearchParams(window.location.search)
  return Boolean(
    window.location.protocol === 'wails:' ||
    runtimeWindow._wails?.environment?.OS ||
    query.get('newWindow') === '1' ||
    query.has('openHost') ||
    query.has('openSession') ||
    query.get('openLocal') === '1',
  )
}

function createID(prefix: string): string {
  const randomID = globalThis.crypto?.randomUUID?.()
  return `${prefix}-${randomID ?? `${Date.now()}-${Math.random().toString(16).slice(2)}`}`
}

function createMemoryGroup(id: string, name: string): GroupRecord {
  const now = new Date().toISOString()
  return {
    id,
    name,
    sortOrder: 0,
    createdAt: now,
    updatedAt: now,
  }
}

function requireGroupName(name: string): string {
  const normalizedName = name.trim()
  if (!normalizedName) throw new Error('请填写分组名称')
  return normalizedName
}
