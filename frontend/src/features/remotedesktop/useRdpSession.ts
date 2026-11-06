import { computed, onBeforeUnmount, ref, shallowRef, type Ref } from 'vue'
import type { RdpFrame, RdpOpenInput, RdpSession, RdpStats } from '../../../bindings/changeme/internal/model/models'
import { decodeAppError, translateAppError, type AppError } from '../../services/errors'
import type { RdpMouseButton } from '../../services/remotedesktop'

export type RdpStatus = 'idle' | 'connecting' | 'connected' | 'error'

export const RDP_FRAME_INTERVAL_MS = 50

export type RdpFrameMode = 'stream' | 'legacy'

export const RDP_LEGACY_FRAMES_KEY = 'qterm.rdp.legacyFrames'

export function legacyFramesForced(): boolean {
  try {
    return localStorage.getItem(RDP_LEGACY_FRAMES_KEY) === '1'
  } catch {
    return false
  }
}

interface RdpStreamStatsFields {
  serverFPS?: number
}

export function readServerFps(stats: RdpStats | null): number {
  const value = (stats as unknown as RdpStreamStatsFields | null)?.serverFPS
  return typeof value === 'number' && Number.isFinite(value) && value > 0 ? value : 0
}

export interface RdpClient {
  open(input: RdpOpenInput): Promise<RdpSession>
  close(id: string): Promise<void>
  frame(id: string): Promise<RdpFrame>
  stats(id: string): Promise<RdpStats>
  pointerMove(id: string, x: number, y: number): Promise<void>
  pointerButton(id: string, x: number, y: number, button: RdpMouseButton, pressed: boolean): Promise<void>
  wheel(id: string, x: number, y: number, notches: number, horizontal: boolean): Promise<void>
  key(id: string, scanCode: number, pressed: boolean, extended: boolean): Promise<void>
  text(id: string, text: string): Promise<void>
  readClipboard(id: string): Promise<string>
  writeClipboard(id: string, text: string): Promise<void>

  openHost?(hostId: string, width: number, height: number, password: string, engine: string): Promise<RdpSession>

  session?(id: string): Promise<RdpSession | null>
}

export function pngDataUrl(base64: string | null | undefined): string {
  if (!base64) {
    return ''
  }
  return `data:image/png;base64,${base64}`
}

export interface UseRdpSessionOptions {
  client: RdpClient

  intervalMs?: number

  frameMode?: RdpFrameMode
}

export function useRdpSession(options: UseRdpSessionOptions) {
  const status = ref<RdpStatus>('idle')
  const errorText = ref('')

  const errorState = ref<AppError | null>(null)
  const info = ref<RdpSession | null>(null)
  const stats = ref<RdpStats | null>(null)

  const frameMode = ref<RdpFrameMode>(options.frameMode ?? (legacyFramesForced() ? 'legacy' : 'stream'))

  const fallbackReason = ref('')

  const frameUrl = shallowRef('')
  const cursorUrl = shallowRef('')
  const frameSize = ref({ width: 0, height: 0 })
  const cursor = ref({ visible: false, x: 0, y: 0, width: 0, height: 0 })
  const sequence = ref(0)
  const firstFrameReceived = ref(false)

  const certificateProbePending = ref(false)

  let timer: ReturnType<typeof setTimeout> | null = null

  let pollGeneration = 0

  let consecutiveFailures = 0

  let closedProbes = 0
  let statsInFlight = false

  let lifecycleGeneration = 0

  let handoverPending = false

  const sessionId = computed(() => info.value?.id ?? '')

  const engineLabel = computed(() => info.value?.engine ?? '')

  const certificateState = computed(() => stats.value?.certificateState ?? '')
  const certificateFingerprint = computed(() => stats.value?.certificateFingerprint ?? '')
  const certificateSubject = computed(() => stats.value?.certificateSubject ?? '')
  const certificateIssuer = computed(() => stats.value?.certificateIssuer ?? '')
  const connected = computed(() => status.value === 'connected')

  function handlePollFailure(error: unknown): void {
    const decoded = decodeAppError(error)
    const message = decoded
      ? translateAppError(decoded, '远程桌面连接出错')
      : error instanceof Error
        ? error.message
        : String(error)
    errorText.value = message
    errorState.value = decoded ?? null
    consecutiveFailures += 1
    const terminal = decoded?.code === 'NOT_FOUND' || decoded?.code === 'SESSION_CLOSED'
    if (terminal || consecutiveFailures >= 3) {
      stopPolling()
      status.value = 'error'
    }
  }

  function stopPolling(): void {
    pollGeneration += 1
    if (timer !== null) {
      clearTimeout(timer)
      timer = null
    }
  }

  function schedulePolling(id: string, generation: number, delayMs?: number): void {
    if (generation !== pollGeneration) {
      return
    }
    timer = setTimeout(
      () => {
        if (generation !== pollGeneration) {
          return
        }
        void (async () => {
          const startedAt = Date.now()
          try {
            await pollOnce(id)
          } finally {
            if (generation === pollGeneration) {
              const interval = options.intervalMs ?? RDP_FRAME_INTERVAL_MS
              schedulePolling(id, generation, Math.max(0, interval - (Date.now() - startedAt)))
            }
          }
        })()
      },
      delayMs ?? options.intervalMs ?? RDP_FRAME_INTERVAL_MS,
    )
  }

  async function pollOnce(id: string): Promise<void> {
    try {
      const frame = await options.client.frame(id)
      if (info.value?.id !== id) {
        return
      }
      consecutiveFailures = 0
      if (frame.changed) {
        frameUrl.value = pngDataUrl(frame.png)
        sequence.value = frame.sequence
        if (frame.png) {
          firstFrameReceived.value = true
          status.value = 'connected'
        }
      }
      if (frame.width > 0 && frame.height > 0) {
        frameSize.value = { width: frame.width, height: frame.height }
      }
      cursor.value = {
        visible: frame.cursorVisible,
        x: frame.cursorX,
        y: frame.cursorY,
        width: frame.cursorWidth,
        height: frame.cursorHeight,
      }
      if (frame.cursorVisible && frame.cursorPng) {
        cursorUrl.value = pngDataUrl(frame.cursorPng)
      } else if (!frame.cursorVisible) {
        cursorUrl.value = ''
      }
    } catch (error) {
      if (info.value?.id === id) {
        handlePollFailure(error)
      }
    }
  }

  function startFramePolling(): void {
    stopPolling()
    if (frameMode.value !== 'legacy') {
      return
    }
    const id = info.value?.id
    if (!id) {
      return
    }

    schedulePolling(id, pollGeneration)
  }

  function setFrameMode(mode: RdpFrameMode): void {
    if (frameMode.value === mode) {
      return
    }
    frameMode.value = mode
    if (mode === 'legacy') {
      startFramePolling()
      return
    }
    stopPolling()
    fallbackReason.value = ''
  }

  function fallbackToLegacy(reason = ''): void {
    if (frameMode.value === 'legacy') {
      return
    }
    fallbackReason.value = reason
    setFrameMode('legacy')
  }

  function markConnected(): void {
    firstFrameReceived.value = true
    if (status.value === 'connecting') {
      status.value = 'connected'
    }
  }

  function startSession(session: RdpSession): RdpSession {
    consecutiveFailures = 0
    closedProbes = 0
    certificateProbePending.value = false
    errorState.value = null
    errorText.value = ''
    stats.value = null
    firstFrameReceived.value = false
    info.value = session

    status.value = 'connecting'
    frameSize.value = { width: session.width, height: session.height }
    startFramePolling()
    return session
  }

  function failSession(error: unknown): null {
    stopPolling()
    status.value = 'error'
    const decoded = decodeAppError(error)
    errorState.value = decoded ?? null
    errorText.value = decoded
      ? translateAppError(decoded, '远程桌面连接出错')
      : error instanceof Error
        ? error.message
        : String(error)
    return null
  }

  async function open(input: RdpOpenInput): Promise<RdpSession | null> {
    const generation = ++lifecycleGeneration
    await clearCurrentSession()
    if (generation !== lifecycleGeneration) {
      return null
    }
    status.value = 'connecting'
    errorText.value = ''
    errorState.value = null
    try {
      const opened = await options.client.open(input)
      if (generation !== lifecycleGeneration) {
        await options.client.close(opened.id).catch(() => undefined)
        return null
      }
      return startSession(opened)
    } catch (error) {
      if (generation !== lifecycleGeneration) {
        return null
      }
      return failSession(error)
    }
  }

  async function openHostSession(
    hostId: string,
    width = 1280,
    height = 800,
    password = '',
    engine = '',
  ): Promise<RdpSession | null> {
    const generation = ++lifecycleGeneration
    await clearCurrentSession()
    if (generation !== lifecycleGeneration) {
      return null
    }
    if (!options.client.openHost) {
      return failSession(new Error('当前环境不支持按主机打开远程桌面'))
    }
    status.value = 'connecting'
    errorText.value = ''
    errorState.value = null
    try {
      const opened = await options.client.openHost(hostId, width, height, password, engine)
      if (generation !== lifecycleGeneration) {
        await options.client.close(opened.id).catch(() => undefined)
        return null
      }
      return startSession(opened)
    } catch (error) {
      if (generation !== lifecycleGeneration) {
        return null
      }
      return failSession(error)
    }
  }

  async function attach(sessionId: string): Promise<RdpSession | null> {
    const id = sessionId.trim()
    const generation = ++lifecycleGeneration
    await clearCurrentSession()
    if (generation !== lifecycleGeneration) {
      return null
    }
    if (!id) {
      return failSession(new Error('缺少远程桌面会话标识，无法接管'))
    }
    if (!options.client.session) {
      return failSession(new Error('当前环境不支持接管已打开的远程桌面会话'))
    }
    status.value = 'connecting'
    errorText.value = ''
    errorState.value = null
    try {
      const existing = await options.client.session(id)
      if (generation !== lifecycleGeneration) {
        return null
      }
      if (!existing) {
        return failSession(new Error('该远程桌面会话已经结束，请重新连接'))
      }
      return startSession(existing)
    } catch (error) {
      if (generation !== lifecycleGeneration) {
        return null
      }
      return failSession(error)
    }
  }

  async function clearCurrentSession(closeBackend = true): Promise<void> {
    stopPolling()
    const id = info.value?.id
    info.value = null
    status.value = 'idle'
    errorState.value = null
    errorText.value = ''
    stats.value = null
    certificateProbePending.value = false
    frameUrl.value = ''
    cursorUrl.value = ''
    firstFrameReceived.value = false
    if (id && closeBackend) {
      await options.client.close(id).catch(() => undefined)
    }
  }

  async function close(): Promise<void> {
    lifecycleGeneration += 1
    await clearCurrentSession()
  }

  function setHandoverPending(pending: boolean): void {
    handoverPending = pending
  }

  async function refreshStats(): Promise<void> {
    const id = info.value?.id

    if (!id || (status.value === 'error' && closedProbes >= 5) || statsInFlight) {
      return
    }
    statsInFlight = true
    try {
      const nextStats = await options.client.stats(id)
      if (info.value?.id !== id) {
        return
      }
      stats.value = nextStats
      consecutiveFailures = 0

      if (stats.value.closed) {
        status.value = 'error'
        errorText.value = stats.value.error || '远程桌面会话已结束'

        closedProbes += 1
        const certificateSettled =
          stats.value.certificateState === 'needTrust' || stats.value.certificateState === 'mismatch'
        certificateProbePending.value = !certificateSettled && closedProbes < 5
        if (certificateSettled || closedProbes >= 5) {
          stopPolling()
        }
      } else {
        closedProbes = 0
        certificateProbePending.value = false
      }
    } catch (error) {
      if (info.value?.id === id) {
        handlePollFailure(error)
      }
    } finally {
      statsInFlight = false
    }
  }

  async function pointerMove(x: number, y: number): Promise<void> {
    const id = info.value?.id
    if (id) {
      await options.client.pointerMove(id, x, y)
    }
  }

  async function pointerButton(x: number, y: number, button: RdpMouseButton, pressed: boolean): Promise<void> {
    const id = info.value?.id
    if (id) {
      await options.client.pointerButton(id, x, y, button, pressed)
    }
  }

  async function wheel(x: number, y: number, notches: number, horizontal = false): Promise<void> {
    const id = info.value?.id
    if (id) {
      await options.client.wheel(id, x, y, notches, horizontal)
    }
  }

  async function key(scanCode: number, pressed: boolean, extended: boolean): Promise<void> {
    const id = info.value?.id
    if (id) {
      await options.client.key(id, scanCode, pressed, extended)
    }
  }

  async function text(value: string): Promise<void> {
    const id = info.value?.id
    if (id) {
      await options.client.text(id, value)
    }
  }

  async function pushClipboard(value: string): Promise<void> {
    const id = info.value?.id
    if (id) {
      await options.client.writeClipboard(id, value)
    }
  }

  async function pullClipboard(): Promise<string> {
    const id = info.value?.id
    if (!id) {
      return ''
    }
    return options.client.readClipboard(id)
  }

  onBeforeUnmount(() => {
    lifecycleGeneration += 1

    void clearCurrentSession(!handoverPending)
  })

  return {
    status,
    errorText,
    error: errorState,
    info,
    stats,
    sessionId,
    engineLabel,
    certificateState,
    certificateFingerprint,
    certificateSubject,
    certificateIssuer,
    connected,
    frameUrl,
    cursorUrl,
    frameSize,
    cursor,
    sequence,
    firstFrameReceived,
    certificateProbePending,
    frameMode,
    fallbackReason,
    open,
    openHostSession,
    attach,
    setHandoverPending,
    close,
    refreshStats,
    setFrameMode,
    fallbackToLegacy,
    markConnected,
    pointerMove,
    pointerButton,
    wheel,
    key,
    text,
    pushClipboard,
    pullClipboard,
  }
}

export type RdpSessionController = ReturnType<typeof useRdpSession>
export type { RdpFrame, RdpStats, RdpSession }
export type { Ref }
