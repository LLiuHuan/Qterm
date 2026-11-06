import { computed, onBeforeUnmount, ref } from 'vue'
import { Stream } from '@wailsio/runtime'
import type { VncSession, VncStreamTicket } from '../../../bindings/changeme/internal/model/models'
import { decodeAppError, errorMessage, translateErrorKey } from '../../services/errors'

import { readClipboardText, writeClipboardText } from '../../services/clipboard'
import { closeVncSession, fetchVncStreamTicket, openVncHostSession } from '../../services/vnc'
import { VncStreamError, VncErrorCode, bindVncChannel, takeBufferedVncBytes, type VncBinarySocket } from './vncProtocol'

import { installKasmUpstreamCompat, installKasmWheelCompat } from './kasmVncCompat'

import {
  installKasmClipboardPaste,
  installKasmServerMessageCompat,
  markKasmServerMessageInstance,
} from './kasmVncServerMessages'

import { readServerIdentity, resolveKasmDialectGate, type VncServerIdentity } from './kasmVncIdentity'

import { createClipboardAutoSync, type ClipboardAutoSyncController } from './vncClipboardBridge'

export interface VncRfbLike {
  viewOnly: boolean
  scaleViewport: boolean
  clipViewport: boolean
  background: string
  disconnect(): void

  sendCredentials(credentials: { username?: string; password?: string }): void

  clipboardPasteFrom(text: string): void
  addEventListener(type: string, listener: (event: never) => void): void
  removeEventListener(type: string, listener: (event: never) => void): void
}

export interface VncRfbConstructor {
  new (
    target: HTMLElement,
    urlOrChannel: unknown,
    options?: { credentials?: { password?: string; username?: string }; shared?: boolean },
  ): VncRfbLike
}

export type VncPasteResult = 'sent' | 'empty' | 'read-failed' | 'send-failed' | 'not-connected' | 'view-only'

export type VncSessionStatus =
  'idle' | 'connecting' | 'connected' | 'password-required' | 'reconnecting' | 'disconnected' | 'failed'

export interface VncHostPreferences {
  scale: 'fit' | 'actual'

  autoReconnect: boolean

  autoReconnectAttempts: number
}

export const VNC_DEFAULT_PREFERENCES: VncHostPreferences = {
  scale: 'fit',
  autoReconnect: false,
  autoReconnectAttempts: 5,
}

export function readVncHostPreferences(profile: unknown): VncHostPreferences {
  const raw = (profile ?? {}) as {
    vncScale?: unknown
    vncAutoReconnect?: unknown
    vncAutoReconnectAttempts?: unknown
  }
  const attempts = raw.vncAutoReconnectAttempts
  return {
    scale: raw.vncScale === 'actual' ? 'actual' : 'fit',
    autoReconnect: raw.vncAutoReconnect === true,
    autoReconnectAttempts:
      typeof attempts === 'number' && Number.isFinite(attempts)
        ? Math.max(0, Math.floor(attempts))
        : VNC_DEFAULT_PREFERENCES.autoReconnectAttempts,
  }
}

export type VncRfbPasswordMode = 'auto' | 'same'

export interface VncHostAuthFacts {
  basicAuthUsed: boolean

  rfbPasswordRequired: boolean

  passwordAvailable: boolean

  rfbPasswordMode: VncRfbPasswordMode
}

const VNC_DEFAULT_AUTH_FACTS: VncHostAuthFacts = {
  basicAuthUsed: false,
  rfbPasswordRequired: true,
  passwordAvailable: false,
  rfbPasswordMode: 'auto',
}

function normalizeRfbPasswordMode(value: unknown): VncRfbPasswordMode {
  return value === 'same' ? 'same' : 'auto'
}

function usesHttpBasicLayer(source: {
  basicAuthUsed?: boolean
  rfbPasswordRequired?: boolean
  transport?: string
}): boolean {
  return source.basicAuthUsed === true || source.rfbPasswordRequired === false || source.transport === 'wss'
}

function usesEmptyRfbPassword(source: {
  transport?: unknown
  rfbPasswordMode?: unknown
  rfbPasswordRequired?: unknown
  basicAuthUsed?: unknown
}): boolean {
  const wss = source.transport === 'wss' || source.basicAuthUsed === true || source.rfbPasswordRequired === false
  return wss && normalizeRfbPasswordMode(source.rfbPasswordMode) === 'auto'
}

function readHostAuthFacts(source: unknown): VncHostAuthFacts {
  const raw = (source ?? {}) as Partial<VncHostAuthFacts> & { transport?: unknown }
  const basicAuthUsed = raw.basicAuthUsed === true
  const rfbPasswordMode = normalizeRfbPasswordMode(raw.rfbPasswordMode)

  const wss = basicAuthUsed || raw.transport === 'wss' || raw.rfbPasswordRequired === false
  return {
    basicAuthUsed,

    rfbPasswordRequired: !(wss && rfbPasswordMode === 'auto'),
    passwordAvailable: raw.passwordAvailable === true,
    rfbPasswordMode,
  }
}

export interface UseVncSessionOptions {
  hostId: () => string

  loadRfb?: () => Promise<VncRfbConstructor>

  createStream?: (streamName: string) => VncBinarySocket

  openSession?: (hostId: string) => Promise<{
    session: VncSession
    ticket: VncStreamTicket

    username?: string
    password: string

    basicAuthUsed?: boolean

    transport?: string

    rfbPasswordMode?: string

    serverSoftware?: string
    serverVersion?: string
    serverBanner?: string
    identitySource?: string
  }>

  closeSession?: (sessionId: string) => Promise<void>

  streamTicket?: (sessionId: string) => Promise<VncStreamTicket>

  handshakeTimeoutMs?: number

  readClipboard?: () => Promise<string>

  writeClipboard?: (text: string) => Promise<void>

  loadHostPreferences?: (hostId: string) => Promise<VncHostPreferences & Partial<VncHostAuthFacts>>

  reconnectBaseDelayMs?: number

  reconnectMaxDelayMs?: number

  sleep?: (ms: number) => Promise<void>
}

async function defaultLoadRfb(): Promise<VncRfbConstructor> {
  const module = (await import('@novnc/novnc')) as { default: VncRfbConstructor }
  return module.default
}

export function vncStreamErrorKey(error: unknown): string {
  if (error instanceof VncStreamError) {
    switch (error.code) {
      case VncErrorCode.binding:
        return 'vnc.streamUnavailable'
      case VncErrorCode.session:
        return 'vnc.sessionNotFound'
      case VncErrorCode.transport:
      case VncErrorCode.protocol:
        return 'vnc.unreachable'
      default:
        return 'vnc.openFailed'
    }
  }
  return 'vnc.openFailed'
}

const VNC_NON_RETRYABLE_ERROR_KEYS = new Set<string>([
  'vnc.basicAuthRejected',
  'vnc.tlsCertificate',
  'vnc.handshakeRejected',

  'vnc.credentialNotPassword',
  'host.credential_locked',
  'credential.locked',
  'host.credential_not_ready',
])

const VNC_RETRYABLE_ERROR_KEYS = new Set<string>([
  'vnc.unreachable',
  'vnc.openFailed',
  'vnc.streamUnavailable',
  'vnc.sessionNotFound',
])

function isRetryableVncFailure(error: unknown): boolean {
  const errorKey = error instanceof VncStreamError ? vncStreamErrorKey(error) : decodeAppError(error)?.messageKey
  if (!errorKey) return true
  if (VNC_NON_RETRYABLE_ERROR_KEYS.has(errorKey)) return false
  return VNC_RETRYABLE_ERROR_KEYS.has(errorKey)
}

export function useVncSession(options: UseVncSessionOptions) {
  const status = ref<VncSessionStatus>('idle')
  const errorText = ref('')
  const desktopName = ref('')
  const session = ref<VncSession | null>(null)
  const viewOnly = ref(false)

  const passwordRequired = ref(false)

  const preferences = ref<VncHostPreferences>({ ...VNC_DEFAULT_PREFERENCES })

  const reconnectAttempt = ref(0)

  const reconnectLimit = ref(0)

  const kasmCompatAlert = ref<{
    reason: string
    detail: string
    software: string
    version: string
    dialectVerdict: string
  } | null>(null)

  const serverIdentity = ref<VncServerIdentity>({ software: '', version: '', banner: '', source: '' })

  type AttemptOutcome = 'done' | 'retry'

  let rfb: VncRfbLike | null = null

  let kasmWheelDispose: (() => void) | null = null
  let target: HTMLElement | null = null
  let closed = false
  let handshakeTimer: ReturnType<typeof setTimeout> | null = null

  let preferencesHostId = ''

  let hostAuth: VncHostAuthFacts = { ...VNC_DEFAULT_AUTH_FACTS }

  let retryToken = 0

  const retrying = ref(false)

  let attemptOutcome: ((outcome: AttemptOutcome) => void) | null = null

  let sleepRelease: (() => void) | null = null

  function defaultSleep(ms: number): Promise<void> {
    return new Promise<void>((resolve) => {
      const timer = setTimeout(() => {
        sleepRelease = null
        resolve()
      }, ms)
      sleepRelease = () => {
        clearTimeout(timer)
        sleepRelease = null
        resolve()
      }
    })
  }

  function settleAttempt(outcome: AttemptOutcome): void {
    const resolve = attemptOutcome
    attemptOutcome = null
    resolve?.(outcome)
  }

  function waitForAttempt(): Promise<AttemptOutcome> {
    return new Promise<AttemptOutcome>((resolve) => {
      attemptOutcome = resolve
    })
  }

  function stopAutoReconnect(): void {
    retryToken += 1
    retrying.value = false
    const release = sleepRelease
    sleepRelease = null
    release?.()
    settleAttempt('retry')
    reconnectAttempt.value = 0
    reconnectLimit.value = 0
  }

  function armHandshakeWatchdog(): void {
    clearHandshakeWatchdog()
    handshakeTimer = setTimeout(() => {
      handshakeTimer = null
      if (closed || status.value === 'connected' || status.value === 'password-required') return
      status.value = 'failed'
      errorText.value = translateErrorKey('vnc.openFailed')
      settleAttempt('retry')
      startAutoReconnect()
    }, handshakeTimeoutMs)
  }

  function clearHandshakeWatchdog(): void {
    if (handshakeTimer !== null) {
      clearTimeout(handshakeTimer)
      handshakeTimer = null
    }
  }

  const loadRfb = options.loadRfb ?? defaultLoadRfb
  const createStream =
    options.createStream ?? ((streamName: string) => Stream(streamName) as unknown as VncBinarySocket)
  const openSession = options.openSession ?? openVncHostSession
  const closeSession = options.closeSession ?? closeVncSession
  const streamTicket = options.streamTicket ?? fetchVncStreamTicket
  const handshakeTimeoutMs = options.handshakeTimeoutMs ?? 20_000
  const readClipboard = options.readClipboard ?? readClipboardText
  const writeClipboard = options.writeClipboard ?? writeClipboardText
  const loadHostPreferences = options.loadHostPreferences ?? (async () => ({ ...VNC_DEFAULT_PREFERENCES }))
  const reconnectBaseDelayMs = options.reconnectBaseDelayMs ?? 1_000
  const reconnectMaxDelayMs = options.reconnectMaxDelayMs ?? 8_000
  const sleep = options.sleep ?? defaultSleep

  async function ensurePreferences(hostId: string): Promise<VncHostPreferences> {
    if (preferencesHostId === hostId) return preferences.value
    try {
      const loaded = await loadHostPreferences(hostId)

      preferences.value = {
        scale: loaded.scale,
        autoReconnect: loaded.autoReconnect,
        autoReconnectAttempts: loaded.autoReconnectAttempts,
      }
      hostAuth = readHostAuthFacts(loaded)
    } catch {
      preferences.value = { ...VNC_DEFAULT_PREFERENCES }
      hostAuth = { ...VNC_DEFAULT_AUTH_FACTS }
    }
    preferencesHostId = hostId
    return preferences.value
  }

  async function releaseSession(): Promise<void> {
    clearHandshakeWatchdog()
    const disposeWheel = kasmWheelDispose
    kasmWheelDispose = null
    disposeWheel?.()
    const current = rfb
    rfb = null
    if (current) {
      try {
        current.disconnect()
      } catch {}
    }
    const currentSession = session.value
    session.value = null
    if (currentSession) {
      try {
        await closeSession(currentSession.id)
      } catch {}
    }
  }

  async function disconnect(): Promise<void> {
    closed = true
    stopAutoReconnect()
    status.value = 'disconnected'
    await releaseSession()
  }

  function startAutoReconnect(): void {
    if (closed || retrying.value) return
    const current = preferences.value
    const limit = Math.floor(current.autoReconnectAttempts)
    if (!current.autoReconnect || limit <= 0) return
    retrying.value = true
    const token = ++retryToken
    void runAutoReconnect(token, limit)
  }

  async function runAutoReconnect(token: number, limit: number): Promise<void> {
    try {
      while (!closed && token === retryToken && reconnectAttempt.value < limit) {
        reconnectAttempt.value += 1
        reconnectLimit.value = limit
        status.value = 'reconnecting'
        await sleep(Math.min(reconnectBaseDelayMs * 2 ** (reconnectAttempt.value - 1), reconnectMaxDelayMs))
        if (closed || token !== retryToken) return

        await releaseSession()
        if (closed || token !== retryToken) return
        const element = target
        if (!element) return
        const outcome = waitForAttempt()
        await connect(element)
        const settled = await outcome
        if (closed || token !== retryToken) return
        if (settled === 'done') {
          reconnectAttempt.value = 0
          reconnectLimit.value = 0
          return
        }
      }

      if (!closed && token === retryToken && reconnectAttempt.value >= limit) {
        if (status.value !== 'failed') status.value = 'failed'
        if (!errorText.value) errorText.value = translateErrorKey('vnc.openFailed')
      }
    } finally {
      if (token === retryToken) retrying.value = false
    }
  }

  async function connect(element: HTMLElement): Promise<void> {
    target = element
    closed = false
    status.value = 'connecting'
    errorText.value = ''
    passwordRequired.value = false
    desktopName.value = ''
    try {
      await ensurePreferences(options.hostId())
      if (closed) return

      if (usesHttpBasicLayer(hostAuth) && !hostAuth.passwordAvailable) {
        preferencesHostId = ''
        status.value = 'failed'
        errorText.value = translateErrorKey('vnc.basicAuthPasswordRequired')

        settleAttempt('done')
        return
      }

      const RfbConstructor = await loadRfb()
      if (closed) return

      const opened = await openSession(options.hostId())
      if (closed) {
        await closeSession(opened.session.id).catch(() => undefined)
        return
      }
      session.value = opened.session

      viewOnly.value = opened.session.viewOnly

      const socket = createStream(opened.ticket.streamName)
      await bindVncChannel(socket, opened.session.id, opened.ticket.token)
      if (closed) return

      const emptyRfbPassword = usesEmptyRfbPassword({
        transport: typeof opened.transport === 'string' ? opened.transport : undefined,
        basicAuthUsed: opened.basicAuthUsed === true,
        rfbPasswordMode: opened.rfbPasswordMode,
      })
      const credentials: { username?: string; password?: string } = {}

      if (opened.username) credentials.username = opened.username
      if (opened.password) credentials.password = opened.password

      if (emptyRfbPassword) credentials.password = ''

      const identity = readServerIdentity({
        serverSoftware: opened.serverSoftware,
        serverVersion: opened.serverVersion,
        serverBanner: opened.serverBanner,
        identitySource: opened.identitySource,
      })
      serverIdentity.value = identity
      const dialectGate = resolveKasmDialectGate(identity, emptyRfbPassword || usesEmptyRfbPassword(hostAuth))
      const kasmCompat = dialectGate.install ? installKasmUpstreamCompat(socket) : null

      if (kasmCompat) {
        const installResult = installKasmServerMessageCompat(RfbConstructor, (failure) => {
          kasmCompatAlert.value = {
            reason: failure.reason,
            detail: failure.selfCheck.detail,
            software: identity.software,
            version: identity.version,
            dialectVerdict: dialectGate.dialectVerdict,
          }

          console.warn(
            `[vnc] KasmVNC 二进制剪贴板适配未安装（${failure.reason}）：${failure.selfCheck.detail}；` +
              `服务端身份=${identity.software || '未知'}/${identity.version || '未知'}；` +
              `远端复制文本时连接可能断开，请核对 noVNC 版本（package.json 里为精确版本）。`,
          )
        })
        if (installResult.installed) {
          console.info(
            `[vnc] KasmVNC 二进制剪贴板适配已安装并自检通过（合成帧 ${installResult.selfCheck.total} 字节，` +
              `队列路径 ${installResult.selfCheck.queuePath || 'n/a'}）。`,
          )
        }
      }
      const instance = new RfbConstructor(element, socket, {
        credentials: Object.keys(credentials).length > 0 ? credentials : undefined,
        shared: opened.session.shared,
      })
      rfb = instance

      if (kasmCompat) markKasmServerMessageInstance(instance)

      if (kasmCompat) installKasmClipboardPaste(instance, kasmCompat)

      if (kasmCompat) kasmWheelDispose = installKasmWheelCompat(instance, element, kasmCompat)

      for (const chunk of takeBufferedVncBytes(socket)) {
        socket.onmessage?.({ data: chunk } as MessageEvent)
      }

      armHandshakeWatchdog()
      instance.viewOnly = opened.session.viewOnly

      instance.addEventListener('connect', () => {
        if (rfb !== instance) return

        kasmCompat?.enable()
        clearHandshakeWatchdog()
        status.value = 'connected'
        settleAttempt('done')
      })
      instance.addEventListener('desktopname', (event: never) => {
        if (rfb !== instance) return
        const detail = (event as unknown as { detail?: { name?: string } }).detail
        if (detail?.name) desktopName.value = detail.name
      })
      instance.addEventListener('disconnect', (event: never) => {
        if (rfb !== instance) return
        const detail = (event as unknown as { detail?: { clean?: boolean } }).detail
        if (status.value === 'failed') return
        status.value = 'disconnected'
        errorText.value = detail?.clean === false ? translateErrorKey('vnc.openFailed') : ''

        settleAttempt('retry')
        startAutoReconnect()
      })

      instance.addEventListener('clipboard', (event: never) => {
        if (rfb !== instance) return
        const detail = (event as unknown as { detail?: { text?: string } }).detail
        const text = detail?.text ?? ''
        if (!text) return
        void writeClipboard(text).catch(() => undefined)
      })
      instance.addEventListener('credentialsrequired', () => {
        if (rfb !== instance) return
        clearHandshakeWatchdog()
        if (emptyRfbPassword) {
          status.value = 'failed'
          errorText.value = translateErrorKey('vnc.wssRfbPasswordRequired')
          settleAttempt('done')
          return
        }
        passwordRequired.value = true
        status.value = 'password-required'

        settleAttempt('done')
      })
      instance.addEventListener('securityfailure', (event: never) => {
        if (rfb !== instance) return
        clearHandshakeWatchdog()
        const detail = (event as unknown as { detail?: { reason?: string } }).detail
        status.value = 'failed'
        errorText.value = detail?.reason || translateErrorKey('vnc.openFailed')

        settleAttempt('done')
      })
    } catch (error) {
      status.value = 'failed'
      errorText.value =
        error instanceof VncStreamError
          ? translateErrorKey(vncStreamErrorKey(error))
          : errorMessage(error, translateErrorKey('vnc.openFailed'))

      if (isRetryableVncFailure(error)) {
        settleAttempt('retry')
        startAutoReconnect()
      } else {
        settleAttempt('done')
      }
    }
  }

  function submitPassword(password: string): void {
    if (!rfb) return
    if (!password) return
    rfb.sendCredentials({ password })
    passwordRequired.value = false

    status.value = 'connecting'
  }

  async function pasteToRemote(): Promise<VncPasteResult> {
    if (!rfb || closed) return 'not-connected'

    if (viewOnly.value || rfb.viewOnly) return 'view-only'

    let text: string
    try {
      text = await readClipboard()
    } catch {
      return 'read-failed'
    }
    if (!text) return 'empty'
    return sendClipboardText(text)
  }

  async function sendClipboardText(text: string): Promise<VncPasteResult> {
    if (!rfb || closed) return 'not-connected'
    if (!text) return 'empty'

    if (viewOnly.value || rfb.viewOnly) return 'view-only'
    try {
      rfb.clipboardPasteFrom(text)
    } catch {
      return 'send-failed'
    }
    return 'sent'
  }

  function setViewOnly(value: boolean): void {
    viewOnly.value = value
    if (rfb) rfb.viewOnly = value
  }

  const clipboardAutoSyncVisible = ref(false)

  const clipboardAutoSync: ClipboardAutoSyncController = createClipboardAutoSync({
    readClipboard,
    isVisible: () => clipboardAutoSyncVisible.value,

    send: (text) => sendClipboardText(text),
  })

  function setClipboardAutoSyncVisible(visible: boolean): void {
    clipboardAutoSyncVisible.value = visible
  }

  function startClipboardAutoSync(): void {
    clipboardAutoSync.start()
  }

  function stopClipboardAutoSync(): void {
    clipboardAutoSync.stop()
  }

  function fitToContainer(enabled: boolean): void {
    if (!rfb) return
    rfb.scaleViewport = enabled
    rfb.clipViewport = !enabled
  }

  onBeforeUnmount(() => {
    clipboardAutoSync.stop()
    void disconnect()
  })

  const sessionId = computed(() => session.value?.id ?? '')

  const statusTone = computed<'idle' | 'connected' | 'error'>(() => {
    if (status.value === 'connected') return 'connected'
    if (status.value === 'failed') return 'error'
    return 'idle'
  })

  const preferredFit = computed(() => preferences.value.scale === 'fit')

  const reconnecting = computed(() => retrying.value && reconnectAttempt.value > 0)

  const reconnectExhausted = computed(
    () => !retrying.value && reconnectLimit.value > 0 && reconnectAttempt.value >= reconnectLimit.value,
  )

  return {
    status,
    sessionId,
    statusTone,
    errorText,
    desktopName,
    session,
    viewOnly,
    passwordRequired,

    preferences,
    preferredFit,

    reconnecting,
    reconnectAttempt,
    reconnectLimit,
    reconnectExhausted,
    connect,
    disconnect,
    pasteToRemote,
    sendClipboardText,

    serverIdentity,
    kasmCompatAlert,

    clipboardAutoSyncVisible,
    setClipboardAutoSyncVisible,
    startClipboardAutoSync,
    stopClipboardAutoSync,
    submitPassword,
    setViewOnly,
    fitToContainer,

    hasRfb: () => rfb !== null,
    targetElement: () => target,
    acquireTicket: streamTicket,
  }
}
