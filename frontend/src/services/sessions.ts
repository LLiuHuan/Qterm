import { errorMessage } from './errors'

import type { CredentialKind as BindingCredentialKind } from '../../bindings/changeme/internal/model/models'
import { hasWailsRuntime } from './runtime'

interface WailsWindow extends Window {
  _wails?: {
    environment?: {
      OS?: string
    }
  }
}

export type SessionState =
  | 'preview'
  | 'connecting'
  | 'awaitingAuth'
  | 'hostKeyPending'
  | 'authenticating'
  | 'connected'
  | 'closing'
  | 'closed'
  | 'failed'

export type SessionKind = 'ssh' | 'telnet' | 'local'

export function sessionKindFromBackend(kind?: string | null): SessionKind {
  if (kind === 'local' || kind === 'telnet') return kind
  return 'ssh'
}

export interface SessionRecord {
  id: string
  hostId: string
  kind: SessionKind
  state: Exclude<SessionState, 'preview'>
  createdAt: string
  updatedAt: string
  latencyMs?: number | null

  attachEpoch?: number

  closeReason?: SessionCloseReason

  exitCode?: number | null
}

export type SessionCloseReason = 'user_closed' | 'remote_eof' | 'orphan_timeout' | 'error'
export interface ResourceSnapshot {
  metricsSupported?: boolean
  loadAverage1?: number | null

  loadAverage5?: number | null
  loadAverage15?: number | null

  sampleWindowMs?: number
  hostname: string
  os: string
  architecture: string
  cpuCores: number
  memoryTotalBytes: number
  memoryAvailableBytes: number

  memoryCachedBytes: number
  uptimeSeconds: number
  cpuUsagePercent: number
  memoryUsagePercent: number
  memoryUsedText: string
  memoryAvailableText: string
  memoryCachedText: string
  memoryTotalText: string
  cpuPerCore: number[]
  networks: {
    name: string
    receiveBytes: number
    transmitBytes: number
    receiveRateBytes: number
    transmitRateBytes: number
    receiveText: string
    transmitText: string
  }[]
  disks: {
    mount: string
    fileSystemType: string
    usedPercent: number
    usedBytes: number
    totalBytes: number
    usedText: string
    totalText: string
  }[]
}

export interface SessionOpenResult {
  session: SessionRecord
  streamToken: string

  warnings?: string[]
}

export interface LocalShellOptions {
  shellPath: string
  workingDirectory: string
}

export const DEFAULT_LOCAL_SHELL_OPTIONS: LocalShellOptions = { shellPath: '', workingDirectory: '' }

export interface SessionTestConnectionInput {
  hostId: string
  protocol: string
  address: string
  port: number
  username: string
  credentialId: string
  kind: 'password' | 'privateKey' | 'privateKeyPassphrase' | ''
  secret: string
}

export interface HostKeyChallenge {
  address: string
  algorithm: string
  sha256Fingerprint: string
  issue: 'unknown' | 'changed'
  trustedFingerprints: string[]

  hopIndex: number

  hopLabel: string
}

export interface HostKeyPending {
  sessionId: string
  challenge: HostKeyChallenge
  challenges: HostKeyChallenge[]
}

function normalizeHostKeyChallenge(challenge: {
  address: string
  algorithm: string
  sha256Fingerprint: string
  issue: string
  trustedFingerprints: string[] | null
  hopIndex: number
  hopLabel: string
}): HostKeyChallenge {
  return {
    address: challenge.address,
    algorithm: challenge.algorithm,
    sha256Fingerprint: challenge.sha256Fingerprint,
    issue: challenge.issue as HostKeyChallenge['issue'],
    trustedFingerprints: challenge.trustedFingerprints ?? [],
    hopIndex: challenge.hopIndex ?? 0,
    hopLabel: challenge.hopLabel ?? '',
  }
}

export interface SessionAuthenticationChallenge {
  requestId?: string
  kind?: 'credential' | 'keyboard-interactive'
  hostId?: string
  hopHostId?: string
  instruction?: string
  questions?: { prompt: string; echo: boolean }[]
  sessionId: string
  address: string
  username: string
}

export interface SessionAuthenticationInput {
  requestId?: string
  answers?: string[]
  secret: string
  credentialId: string
  remember: boolean
}

export interface SessionDetectIconInput {
  hostId: string
  protocol: string
  address: string
  port: number
  username: string
  credentialId: string
  kind: string
  secret: string
}

export async function getSessionWarnings(sessionId: string): Promise<string[]> {
  if (!hasWailsRuntime() || !sessionId) return []
  try {
    const { GetSessionWarnings } = await import('../../bindings/changeme/internal/app/service')
    return ((await GetSessionWarnings(sessionId)) ?? []).map((key) => String(key)).filter((key) => key.length > 0)
  } catch {
    return []
  }
}

export interface SessionGateway {
  readonly persistent: boolean
  open(hostId: string): Promise<SessionOpenResult>
  rebind(id: string): Promise<SessionOpenResult>

  prepareHandover(id: string): Promise<void>

  openLocal(options?: LocalShellOptions): Promise<SessionOpenResult>
  testConnection(input: SessionTestConnectionInput): Promise<void>

  detectIcon(input: SessionDetectIconInput): Promise<string>
  get(id: string): Promise<SessionRecord>
  readResources?(id: string): Promise<ResourceSnapshot>
  getFailure(id: string): Promise<string>
  close(id: string): Promise<void>
  getHostKeyChallenge(id: string): Promise<HostKeyChallenge>

  getHostKeyPending?(id: string): Promise<HostKeyPending>
  trustHostKey(id: string): Promise<SessionOpenResult>
  getAuthenticationChallenge(id: string): Promise<SessionAuthenticationChallenge>
  submitAuthentication(id: string, input: SessionAuthenticationInput): Promise<void>
}

class MemorySessionGateway implements SessionGateway {
  readonly persistent = false
  private readonly sessions = new Map<string, SessionRecord>()

  private readonly attachEpochs = new Map<string, number>()

  async open(hostId: string): Promise<SessionOpenResult> {
    const now = new Date().toISOString()
    const session: SessionRecord = {
      id: createSessionID(),
      hostId,
      kind: 'ssh',
      state: 'connected',
      createdAt: now,
      updatedAt: now,
      attachEpoch: 1,
    }
    this.sessions.set(session.id, session)
    this.attachEpochs.set(session.id, 1)
    return { session: { ...session }, streamToken: '' }
  }

  async rebind(id: string): Promise<SessionOpenResult> {
    const session = await this.get(id)

    const epoch = (this.attachEpochs.get(id) ?? session.attachEpoch ?? 0) + 1
    this.attachEpochs.set(id, epoch)
    session.attachEpoch = epoch
    return { session, streamToken: '' }
  }

  async prepareHandover(id: string): Promise<void> {
    await this.get(id)
  }

  async openLocal(): Promise<SessionOpenResult> {
    throw new Error('浏览器预览没有本机终端')
  }

  async testConnection(): Promise<void> {
    throw new Error('浏览器预览没有真实 SSH 连接')
  }

  async detectIcon(): Promise<string> {
    throw new Error('浏览器预览没有真实 SSH 连接，无法识别远端系统图标')
  }

  async get(id: string): Promise<SessionRecord> {
    const session = this.sessions.get(id)
    if (!session) throw new Error('会话不存在')
    return { ...session }
  }
  async readResources(): Promise<ResourceSnapshot> {
    throw new Error('浏览器预览没有真实 SSH 资源')
  }

  async close(id: string): Promise<void> {
    this.sessions.delete(id)
  }

  async getFailure(): Promise<string> {
    return '浏览器预览没有真实 SSH 连接错误'
  }

  async getHostKeyChallenge(): Promise<HostKeyChallenge> {
    throw new Error('浏览器预览没有真实主机密钥')
  }

  async trustHostKey(): Promise<SessionOpenResult> {
    throw new Error('浏览器预览不能信任真实主机密钥')
  }

  async getAuthenticationChallenge(): Promise<SessionAuthenticationChallenge> {
    throw new Error('浏览器预览没有真实 SSH 认证')
  }

  async submitAuthentication(): Promise<void> {
    throw new Error('浏览器预览没有真实 SSH 认证')
  }
}

class WailsSessionGateway implements SessionGateway {
  readonly persistent = true

  async open(hostId: string): Promise<SessionOpenResult> {
    const { OpenSession } = await import('../../bindings/changeme/internal/app/service')
    return normalizeOpenResult(await OpenSession(hostId))
  }

  async rebind(id: string): Promise<SessionOpenResult> {
    await waitForNativeRuntime()
    const { RebindSession } = await import('../../bindings/changeme/internal/app/service')
    return normalizeOpenResult(await withRebindTimeout(() => RebindSession(id)))
  }

  async prepareHandover(id: string): Promise<void> {
    const { PrepareSessionHandover } = await import('../../bindings/changeme/internal/app/service')
    await PrepareSessionHandover(id)
  }

  async openLocal(options?: LocalShellOptions): Promise<SessionOpenResult> {
    const { OpenLocalSession } = await import('../../bindings/changeme/internal/app/service')

    const { shellPath, workingDirectory } = options ?? DEFAULT_LOCAL_SHELL_OPTIONS
    return normalizeOpenResult(await OpenLocalSession(shellPath, workingDirectory))
  }

  async testConnection(input: SessionTestConnectionInput): Promise<void> {
    const { TestConnection } = await import('../../bindings/changeme/internal/app/service')
    await TestConnection({ ...input, kind: input.kind as BindingCredentialKind })
  }

  async detectIcon(input: SessionDetectIconInput): Promise<string> {
    const { DetectHostIcon } = await import('../../bindings/changeme/internal/app/service')
    return DetectHostIcon({ ...input, kind: input.kind as BindingCredentialKind })
  }

  async get(id: string): Promise<SessionRecord> {
    const { GetSession } = await import('../../bindings/changeme/internal/app/service')
    return GetSession(id) as Promise<SessionRecord>
  }
  async readResources(id: string): Promise<ResourceSnapshot> {
    const { ReadResources } = await import('../../bindings/changeme/internal/app/service')
    return (await ReadResources(id)) as ResourceSnapshot
  }

  async getFailure(id: string): Promise<string> {
    const { GetSessionFailure } = await import('../../bindings/changeme/internal/app/service')
    return GetSessionFailure(id)
  }

  async close(id: string): Promise<void> {
    const { CloseSession } = await import('../../bindings/changeme/internal/app/service')
    await CloseSession(id)
  }

  async getHostKeyPending(id: string): Promise<HostKeyPending> {
    const { GetSessionHostKeyPending } = await import('../../bindings/changeme/internal/app/service')
    const pending = await GetSessionHostKeyPending(id)
    const challenges = (pending.challenges ?? []).map(normalizeHostKeyChallenge)
    const challenge = challenges[0] ?? normalizeHostKeyChallenge(pending.challenge)
    return {
      sessionId: pending.sessionId,
      challenge,

      challenges: challenges.length > 0 ? challenges : [challenge],
    }
  }

  async getHostKeyChallenge(id: string): Promise<HostKeyChallenge> {
    const pending = await this.getHostKeyPending(id)
    return pending.challenge
  }

  async trustHostKey(id: string): Promise<SessionOpenResult> {
    const { TrustSessionHostKey } = await import('../../bindings/changeme/internal/app/service')
    return normalizeOpenResult(await TrustSessionHostKey(id))
  }

  async getAuthenticationChallenge(id: string): Promise<SessionAuthenticationChallenge> {
    const { GetSessionAuthenticationChallenge } = await import('../../bindings/changeme/internal/app/service')
    const challenge = await GetSessionAuthenticationChallenge(id)
    return {
      ...challenge,
      questions: challenge.questions ?? [],
      kind: challenge.kind === 'keyboard-interactive' ? 'keyboard-interactive' : 'credential',
    }
  }

  async submitAuthentication(id: string, input: SessionAuthenticationInput): Promise<void> {
    const { SubmitSessionAuthentication } = await import('../../bindings/changeme/internal/app/service')
    await SubmitSessionAuthentication(id, input)
  }
}

const REBIND_RUNTIME_POLL_MS = 25
const REBIND_RUNTIME_WAIT_MS = 1500
const REBIND_CALL_TIMEOUT_MS = 5000
const REBIND_TIMEOUT_MESSAGE = '接管会话超时，请关闭新窗口后重试。'

type CancellablePromiseLike<T> = PromiseLike<T> & {
  cancel?: () => void | PromiseLike<void>
}

function nativeRuntimeReady(): boolean {
  const runtimeWindow = window as WailsWindow
  return window.location.protocol === 'wails:' || Boolean(runtimeWindow._wails?.environment?.OS)
}

function waitForNativeRuntime(): Promise<void> {
  if (nativeRuntimeReady()) return Promise.resolve()

  return new Promise((resolve) => {
    let settled = false
    const finish = (): void => {
      if (settled) return
      settled = true
      window.clearInterval(pollTimer)
      window.clearTimeout(timeoutTimer)
      resolve()
    }
    const pollTimer = window.setInterval(() => {
      if (nativeRuntimeReady()) finish()
    }, REBIND_RUNTIME_POLL_MS)
    const timeoutTimer = window.setTimeout(finish, REBIND_RUNTIME_WAIT_MS)
  })
}

function cancelPendingRebind(pending: CancellablePromiseLike<unknown>): void {
  if (typeof pending.cancel !== 'function') return
  try {
    const cancellation = pending.cancel()
    if (cancellation && typeof cancellation.then === 'function') {
      void Promise.resolve(cancellation).catch(() => undefined)
    }
  } catch {}
}

function withRebindTimeout<T>(invoke: () => CancellablePromiseLike<T> | PromiseLike<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    let settled = false
    let pending: CancellablePromiseLike<T> | undefined
    const timeoutTimer = window.setTimeout(() => {
      if (settled) return
      settled = true
      if (pending) cancelPendingRebind(pending)
      reject(new Error(REBIND_TIMEOUT_MESSAGE))
    }, REBIND_CALL_TIMEOUT_MS)
    const finish = (callback: () => void): void => {
      if (settled) return
      settled = true
      window.clearTimeout(timeoutTimer)
      callback()
    }

    try {
      pending = invoke() as CancellablePromiseLike<T>
      Promise.resolve(pending).then(
        (value) => finish(() => resolve(value)),
        (error: unknown) => finish(() => reject(error)),
      )
    } catch (error) {
      finish(() => reject(error))
    }
  })
}

class DeferredSessionGateway implements SessionGateway {
  private readonly memory = new MemorySessionGateway()
  private readonly wails = new WailsSessionGateway()

  private get target(): SessionGateway {
    return shouldUseWailsGateway() ? this.wails : this.memory
  }

  get persistent(): boolean {
    return this.target.persistent
  }
  open(hostId: string): Promise<SessionOpenResult> {
    return this.target.open(hostId)
  }
  rebind(id: string): Promise<SessionOpenResult> {
    return this.target.rebind(id)
  }
  prepareHandover(id: string): Promise<void> {
    return this.target.prepareHandover(id)
  }
  openLocal(options?: LocalShellOptions): Promise<SessionOpenResult> {
    return this.target.openLocal(options)
  }
  testConnection(input: SessionTestConnectionInput): Promise<void> {
    return this.target.testConnection(input)
  }
  detectIcon(input: SessionDetectIconInput): Promise<string> {
    return this.target.detectIcon(input)
  }
  get(id: string): Promise<SessionRecord> {
    return this.target.get(id)
  }
  readResources(id: string): Promise<ResourceSnapshot> {
    return this.target.readResources!(id)
  }
  getFailure(id: string): Promise<string> {
    return this.target.getFailure(id)
  }
  close(id: string): Promise<void> {
    return this.target.close(id)
  }
  getHostKeyChallenge(id: string): Promise<HostKeyChallenge> {
    return this.target.getHostKeyChallenge(id)
  }
  async getHostKeyPending(id: string): Promise<HostKeyPending> {
    if (this.target.getHostKeyPending) return this.target.getHostKeyPending(id)

    const challenge = await this.target.getHostKeyChallenge(id)
    return { sessionId: id, challenge, challenges: [challenge] }
  }
  trustHostKey(id: string): Promise<SessionOpenResult> {
    return this.target.trustHostKey(id)
  }
  getAuthenticationChallenge(id: string): Promise<SessionAuthenticationChallenge> {
    return this.target.getAuthenticationChallenge(id)
  }
  submitAuthentication(id: string, input: SessionAuthenticationInput): Promise<void> {
    return this.target.submitAuthentication(id, input)
  }
}

export function createSessionGateway(): SessionGateway {
  return new DeferredSessionGateway()
}

export function sessionErrorMessage(error: unknown, fallback: string): string {
  return errorMessage(error, fallback)
}

function shouldUseWailsGateway(): boolean {
  const runtimeWindow = window as WailsWindow
  const query = new URLSearchParams(window.location.search)
  return Boolean(
    window.location.protocol === 'wails:' ||
    runtimeWindow._wails?.environment?.OS ||
    query.has('openHost') ||
    query.has('openSession') ||
    query.get('openLocal') === '1',
  )
}

function normalizeOpenResult(result: {
  session: Omit<SessionRecord, 'state' | 'kind' | 'closeReason' | 'exitCode'> & {
    state: string
    kind: string

    closeReason?: string

    exitCode?: number | null
  }
  streamToken: string
  warnings?: string[] | null
}): SessionOpenResult {
  return {
    session: {
      ...result.session,

      kind: sessionKindFromBackend(result.session.kind),
      state: result.session.state as SessionRecord['state'],
      closeReason: result.session.closeReason as SessionCloseReason | undefined,
      exitCode: typeof result.session.exitCode === 'number' ? result.session.exitCode : undefined,
    },
    streamToken: result.streamToken,

    warnings: (result.warnings ?? []).filter((key): key is string => typeof key === 'string' && key.length > 0),
  }
}

function createSessionID(): string {
  const randomID = globalThis.crypto?.randomUUID?.()
  return `preview-${randomID ?? `${Date.now()}-${Math.random().toString(16).slice(2)}`}`
}
