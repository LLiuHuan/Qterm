<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { RdpOpenInput } from '../../../bindings/changeme/internal/model/models'
import type { RdpMouseButton } from '../../services/remotedesktop'
import * as rdpService from '../../services/remotedesktop'
import { mapKeyboardEvent } from './keymap'
import { readServerFps, useRdpSession, type RdpClient } from './useRdpSession'
import { RDP_RENDER_RATE_TICK_MS, createRdpRateWindow, useRdpStream } from './useRdpStream'
import { decodeAppError, errorMessage, translateAppError } from '../../services/errors'
import { readClipboardText, writeClipboardText } from '../../services/clipboard'
import RdpCertificateDialog from './RdpCertificateDialog.vue'
import RdpLogDialog from './RdpLogDialog.vue'
import { SessionOverlay, QtermButton } from '../../components/common'
import { rdpDisplayPreferences } from '../settings/rdpDisplayPreferences'
import { useI18n } from '@/locales'
import { toast } from 'vue-sonner'

const props = defineProps<{
  input?: RdpOpenInput

  hostId?: string

  hostTitle?: string

  engine?: string

  initialSessionId?: string

  initialSessionMode?: 'window' | 'main'
  active?: boolean
}>()

const emit = defineEmits<{
  (event: 'close'): void

  (event: 'connection-failed', message: string): void
  (event: 'opened', sessionId: string): void

  (event: 'open-window'): void
}>()

const client: RdpClient = {
  open: rdpService.openRdpSession,
  close: rdpService.closeRdpSession,
  frame: rdpService.fetchRdpFrame,
  stats: rdpService.fetchRdpStats,
  pointerMove: rdpService.sendRdpPointerMove,
  pointerButton: rdpService.sendRdpPointerButton,
  wheel: rdpService.sendRdpWheel,
  key: rdpService.sendRdpKey,
  text: rdpService.sendRdpText,
  readClipboard: rdpService.readRdpClipboard,
  writeClipboard: rdpService.writeRdpClipboard,
  openHost: rdpService.openRdpHostSession,

  session: rdpService.findRdpSession,
}

const session = useRdpSession({ client })

let disposed = false

const { t } = useI18n()

const stream = useRdpStream({
  acquireTicket: rdpService.fetchRdpStreamTicket,
  onError: handleStreamError,
  onFrameSize: handleStreamFrameSize,
})

const RDP_STREAM_FAILURE_THRESHOLD = 4
let streamFailures = 0

function handleStreamError(message: string, fatal: boolean): void {
  if (session.frameMode.value !== 'stream') {
    return
  }
  if (fatal) {
    fallbackToLegacy(message)
    return
  }
  streamFailures += 1
  if (streamFailures >= RDP_STREAM_FAILURE_THRESHOLD) {
    fallbackToLegacy(message)
  }
}

function handleStreamFrameSize(size: { width: number; height: number }): void {
  session.frameSize.value = size

  session.markConnected()
  streamFailures = 0
}

function startStream(sessionId: string): void {
  if (!sessionId || session.frameMode.value !== 'stream') {
    return
  }
  streamFailures = 0

  stream.attach(canvas.value)
  stream.start(sessionId)
}

function stopStream(): void {
  streamFailures = 0
  stream.stop()
}

function resetLegacyFrameState(): void {
  desktopReady = false
  cursorReady = false
  legacyRateWindow.reset()
  legacyRenderRate.value = 0
}

function fallbackToLegacy(reason: string): void {
  if (session.frameMode.value === 'legacy') {
    return
  }
  stream.stop()
  streamFailures = 0
  session.fallbackToLegacy(reason)
  void nextTick(() => {
    updateCanvasScale()
    redraw()
  })
}

const canvas = ref<HTMLCanvasElement | null>(null)
const canvasWrap = ref<HTMLDivElement | null>(null)
const container = ref<HTMLDivElement | null>(null)
const clipboardHint = ref('')

let statsTimer: ReturnType<typeof setInterval> | null = null

function stopStatsPolling(): void {
  if (statsTimer !== null) {
    clearInterval(statsTimer)
    statsTimer = null
  }
}

const pressedButtons = new Set<number>()

const pendingPassword = ref('')
const passwordRequired = ref(false)

const passwordFormVisible = computed(() => passwordRequired.value && !session.connected.value)
const connectionFailureHandled = ref(false)

const failureText = ref('')

const failureOverlayVisible = computed(() => failureText.value !== '')

const canRetry = computed(() => Boolean(props.hostId || props.input))

let connectAttemptGeneration = 0

function isCurrentConnectAttempt(generation: number): boolean {
  return !disposed && generation === connectAttemptGeneration
}

function beginConnectAttempt(): void {
  connectionFailureHandled.value = false
  failureText.value = ''
  certificateDismissed.value = false
  passwordRequired.value = false
}

const engine = ref(props.engine || 'freerdp')

let lastPointerPosition = { x: 0, y: 0 }

const desktopImage = new Image()
const cursorImage = new Image()
let desktopReady = false
let cursorReady = false

const helperStderr = computed(() => session.stats.value?.helperStderr ?? '')

const sessionError = computed(() => session.stats.value?.error ?? '')

const showHelperLog = ref(false)
const sessionClosed = computed(() => session.stats.value?.closed === true)
const certificatePending = computed(
  () => session.certificateState.value === 'needTrust' || session.certificateState.value === 'mismatch',
)
const structuredPasswordRequired = computed(() => session.error.value?.messageKey === 'rdp.passwordRequired')

const handoverPending = ref(false)

function beginHandover(): void {
  handoverPending.value = true
  session.setHandoverPending(true)
}

function cancelHandover(): void {
  handoverPending.value = false
  session.setHandoverPending(false)
}

async function emitConnectionFailure(attemptGeneration?: number, failureMessage = ''): Promise<void> {
  const failureGeneration = attemptGeneration ?? connectAttemptGeneration
  if (
    !isCurrentConnectAttempt(failureGeneration) ||
    connectionFailureHandled.value ||
    certificatePending.value ||
    structuredPasswordRequired.value
  ) {
    return
  }
  connectionFailureHandled.value = true
  const message = failureMessage || session.errorText.value || sessionError.value || t('shell.rdp.connectionFailed')
  stopStatsPolling()

  stopStream()
  clearResizeFallback()
  if (fitResizeTimer !== null) {
    clearTimeout(fitResizeTimer)
    fitResizeTimer = null
  }

  if (!handoverPending.value) {
    await session.close()
  }
  if (!isCurrentConnectAttempt(failureGeneration)) {
    return
  }

  failureText.value = message
  emit('connection-failed', message)
}

async function retryConnection(): Promise<void> {
  await reconnect()
}

function checkConnectionFailure(): void {
  if (
    certificatePending.value ||
    session.certificateProbePending.value ||
    structuredPasswordRequired.value ||
    passwordRequired.value
  ) {
    return
  }
  if (session.status.value === 'error' || session.stats.value?.closed === true) {
    void emitConnectionFailure()
  }
}

watch(
  [
    () => session.status.value,
    () => session.stats.value,
    () => session.error.value,
    () => certificatePending.value,
    () => session.certificateProbePending.value,
  ],
  checkConnectionFailure,
  { flush: 'post' },
)

function redraw(): void {
  const element = canvas.value
  if (!element) {
    return
  }
  if (session.frameMode.value === 'stream') {
    return
  }
  if (desktopReady && desktopImage.naturalWidth > 0) {
    if (element.width !== desktopImage.naturalWidth) {
      element.width = desktopImage.naturalWidth
    }
    if (element.height !== desktopImage.naturalHeight) {
      element.height = desktopImage.naturalHeight
    }
  }
  const context = element.getContext('2d')
  if (!context) {
    return
  }
  if (desktopReady && desktopImage.naturalWidth > 0) {
    context.drawImage(desktopImage, 0, 0)
  }
  if (cursorReady && cursorImage.naturalWidth > 0) {
    const position = session.cursor?.value
    context.drawImage(cursorImage, position?.x ?? 0, position?.y ?? 0)
  }
}

const legacyRateWindow = createRdpRateWindow()
const legacyRenderRate = ref(0)

watch(
  () => session.frameUrl.value,
  (url) => {
    if (!url || session.frameMode.value === 'stream') {
      return
    }
    desktopImage.onload = () => {
      if (session.frameMode.value === 'stream') {
        return
      }
      desktopReady = true
      redraw()
      legacyRateWindow.record()
      legacyRenderRate.value = legacyRateWindow.rate()
    }
    desktopImage.src = url
  },
)

watch(
  () => [session.cursor?.value?.x, session.cursor?.value?.y, session.cursor?.value?.visible] as const,
  () => {
    redraw()
  },
)

watch(
  () => session.cursorUrl.value,
  (url) => {
    if (!url) {
      cursorReady = false
      redraw()
      return
    }
    cursorImage.onload = () => {
      cursorReady = true
      redraw()
    }
    cursorImage.src = url
  },
)

watch(
  () => session.cursor.value,
  () => redraw(),
  { deep: true },
)

function toRemotePosition(event: MouseEvent): { x: number; y: number } {
  const element = canvas.value
  if (!element) {
    return { x: 0, y: 0 }
  }
  const rect = element.getBoundingClientRect()
  const scaleX = element.width / Math.max(rect.width, 1)
  const scaleY = element.height / Math.max(rect.height, 1)
  return {
    x: Math.round((event.clientX - rect.left) * scaleX),
    y: Math.round((event.clientY - rect.top) * scaleY),
  }
}

function buttonFromEvent(event: MouseEvent): RdpMouseButton {
  if (event.button === 1) {
    return 2
  }
  if (event.button === 2) {
    return 1
  }
  return 0
}

function handleMouseMove(event: MouseEvent): void {
  event.preventDefault()
  updateStatusHotZone(event)
  const position = toRemotePosition(event)
  lastPointerPosition = position
  void session.pointerMove(position.x, position.y)
}

function handleMouseDown(event: MouseEvent): void {
  event.preventDefault()
  container.value?.focus()
  pressedButtons.add(event.button)
  const position = toRemotePosition(event)

  void session.pointerMove(position.x, position.y)
  void session.pointerButton(position.x, position.y, buttonFromEvent(event), true)
}

function handleMouseUp(event: MouseEvent): void {
  event.preventDefault()
  pressedButtons.delete(event.button)
  const position = toRemotePosition(event)
  void session.pointerButton(position.x, position.y, buttonFromEvent(event), false)
}

function releasePressedButtons(): void {
  if (pressedButtons.size === 0) {
    return
  }
  const position = lastPointerPosition
  for (const button of pressedButtons) {
    void session.pointerButton(position.x, position.y, buttonFromEvent({ button } as MouseEvent), false)
  }
  pressedButtons.clear()
}

function handleWheel(event: WheelEvent): void {
  event.preventDefault()
  const position = toRemotePosition(event)

  const notches = Math.max(1, Math.min(3, Math.round(Math.abs(event.deltaY) / 100)))
  const direction = event.deltaY < 0 ? notches : -notches
  void session.wheel(position.x, position.y, direction, event.shiftKey)
}

function handleKeyDown(event: KeyboardEvent): void {
  const stroke = mapKeyboardEvent(event)
  if (!stroke) {
    return
  }

  event.preventDefault()
  void session.key(stroke.scanCode, true, stroke.extended)
}

function handlePaneBlur(): void {
  releasePressedButtons()
  releaseModifiers()
}

function releaseModifiers(): void {
  for (const scanCode of [0x1d, 0x2a, 0x38]) {
    void session.key(scanCode, false, false)
  }
}

function handleKeyUp(event: KeyboardEvent): void {
  const stroke = mapKeyboardEvent(event)
  if (!stroke) {
    return
  }
  event.preventDefault()
  void session.key(stroke.scanCode, false, stroke.extended)
}

const clipboardBusy = ref(false)

let lastPushedToRemote = ''
let lastWrittenToLocal = ''

let autoSyncWarned = false

function resetClipboardSyncState(): void {
  lastPushedToRemote = ''
  lastWrittenToLocal = ''
  autoSyncWarned = false
}

watch(() => session.sessionId.value, resetClipboardSyncState)

const RDP_CLIPBOARD_MAX_BYTES = 1 << 20

const clipboardMode = ref('both')
const clipboardContent = ref('text')

const canPasteToRemote = computed(() => clipboardMode.value === 'both' || clipboardMode.value === 'local-to-remote')

const canCopyFromRemote = computed(() => clipboardMode.value === 'both' || clipboardMode.value === 'remote-to-local')

function clipboardByteLength(text: string): number {
  return new TextEncoder().encode(text).length
}

function clipboardTargetSession(): string {
  const id = session.sessionId.value
  if (!id || sessionClosed.value || session.status.value === 'error') {
    toast.warning(t('shell.rdp.clipboard.noSession'))
    return ''
  }
  return id
}

async function pasteLocalClipboard(): Promise<void> {
  if (clipboardBusy.value) {
    return
  }
  if (!clipboardTargetSession()) {
    return
  }
  if (!canPasteToRemote.value) {
    toast.warning(t('shell.rdp.clipboard.localToRemoteBlocked'))
    return
  }
  clipboardBusy.value = true
  try {
    const text = await readClipboardText()
    if (!text) {
      clipboardHint.value = t('shell.rdp.clipboard.localEmpty')
      toast.warning(t('shell.rdp.clipboard.localEmpty'))
      return
    }
    try {
      await session.pushClipboard(text)

      lastPushedToRemote = text
      const count = t('shell.rdp.clipboard.charCount', { count: text.length })
      clipboardHint.value = `${t('shell.rdp.clipboard.pushed')}（${count}）`
      toast.success(t('shell.rdp.clipboard.pushed'), { description: count })
    } catch (error) {
      if (clipboardByteLength(text) > RDP_CLIPBOARD_MAX_BYTES) {
        const detail = t('shell.rdp.clipboard.tooLarge', { count: clipboardByteLength(text) })
        clipboardHint.value = `${t('shell.rdp.clipboard.pushFailed')}：${detail}`
        toast.error(t('shell.rdp.clipboard.pushFailed'), { description: detail })
        return
      }
      try {
        await session.text(text)
      } catch (fallbackError) {
        const detail = errorMessage(fallbackError, t('shell.rdp.clipboard.typeFallbackFailed'))
        clipboardHint.value = `${t('shell.rdp.clipboard.typeFallbackFailed')}：${detail}`
        toast.error(t('shell.rdp.clipboard.typeFallbackFailed'), { description: detail })
        return
      }
      const detail = errorMessage(error, t('shell.rdp.clipboard.pushFailed'))
      clipboardHint.value = `${t('shell.rdp.clipboard.pushedFallback')}：${detail}`
      toast.warning(t('shell.rdp.clipboard.pushedFallback'), { description: detail })
    }
  } catch (error) {
    const detail = errorMessage(error, t('shell.rdp.clipboard.readFailed'))
    clipboardHint.value = detail
    toast.error(t('shell.rdp.clipboard.readFailed'), { description: detail })
  } finally {
    clipboardBusy.value = false
  }
}

async function copyRemoteClipboard(): Promise<void> {
  if (clipboardBusy.value) {
    return
  }
  const id = clipboardTargetSession()
  if (!id) {
    return
  }
  clipboardBusy.value = true
  try {
    const before = session.stats.value?.clipboardSequence ?? 0
    try {
      await rdpService.refreshRdpClipboard(id)
    } catch (error) {
      const detail = errorMessage(error, t('shell.rdp.clipboard.refreshFailed'))
      clipboardHint.value = `${t('shell.rdp.clipboard.refreshFailed')}：${detail}`
      toast.error(t('shell.rdp.clipboard.refreshFailed'), { description: detail })
      return
    }

    const snapshot = await rdpService.fetchRdpStats(id)
    const after = snapshot.clipboardSequence ?? 0
    const text = snapshot.clipboardText ?? ''
    if (after <= before) {
      clipboardHint.value = t('shell.rdp.clipboard.stale')
      toast.warning(t('shell.rdp.clipboard.stale'))
      return
    }
    if (!text) {
      clipboardHint.value = t('shell.rdp.clipboard.remoteEmpty')
      toast.info(t('shell.rdp.clipboard.remoteEmpty'))
      return
    }
    await writeClipboardText(text)
    lastWrittenToLocal = text
    const message = t('shell.rdp.clipboard.copied', { count: text.length })
    clipboardHint.value = message
    toast.success(message)
  } catch (error) {
    const detail = errorMessage(error, t('shell.rdp.clipboard.copyFailed'))
    clipboardHint.value = detail
    toast.error(t('shell.rdp.clipboard.copyFailed'), { description: detail })
  } finally {
    clipboardBusy.value = false
  }
}

watch(
  () => session.stats.value?.clipboardSequence ?? 0,
  () => {
    if (!canCopyFromRemote.value) {
      return
    }
    const text = session.stats.value?.clipboardText ?? ''
    if (!text) {
      return
    }
    if (text === lastPushedToRemote || text === lastWrittenToLocal) {
      return
    }
    void writeClipboardText(text)
      .then(() => {
        lastWrittenToLocal = text
      })
      .catch(() => {
        if (autoSyncWarned) {
          return
        }
        autoSyncWarned = true
        clipboardHint.value = t('shell.rdp.clipboard.autoSyncFailed')
        toast.error(t('shell.rdp.clipboard.autoSyncFailed'))
      })
  },
)

async function close(): Promise<void> {
  connectAttemptGeneration += 1
  stopStatsPolling()

  stopStream()
  await session.close()
  emit('close')
}

function startStatsPolling(): void {
  stopStatsPolling()
  void session.refreshStats()
  statsTimer = setInterval(() => {
    void session.refreshStats()
  }, 1000)
}

async function connectByHost(password = ''): Promise<void> {
  const hostId = props.hostId
  if (!hostId || disposed) {
    return
  }
  const attemptGeneration = ++connectAttemptGeneration

  stopStatsPolling()
  stopStream()
  resetLegacyFrameState()
  beginConnectAttempt()

  await nextTick()
  if (!isCurrentConnectAttempt(attemptGeneration)) {
    return
  }
  await new Promise<void>((resolve) => {
    if (typeof requestAnimationFrame !== 'function') {
      resolve()
      return
    }
    requestAnimationFrame(() => requestAnimationFrame(() => resolve()))
  })
  if (!isCurrentConnectAttempt(attemptGeneration)) {
    return
  }

  await waitForLayoutBox()
  if (!isCurrentConnectAttempt(attemptGeneration)) {
    return
  }
  const desktop = currentDesktopSize()

  lastFitSize = desktop
  traceFit(
    `连接 请求=${desktop.width}×${desktop.height} 实测=${measuredSizeText()} 密码行=${passwordFormVisible.value ? '显示' : '隐藏'}`,
  )
  const opened = await session.openHostSession(hostId, desktop.width, desktop.height, password, engine.value)
  if (!isCurrentConnectAttempt(attemptGeneration)) {
    return
  }
  if (opened) {
    dynamicResizeFailures = 0
    connectedAt = Date.now()
  }
  if (opened) {
    passwordRequired.value = false
    pendingPassword.value = ''
    emit('opened', opened.id)
    startStatsPolling()

    startStream(opened.id)
    return
  }

  if (structuredPasswordRequired.value) {
    passwordRequired.value = true
    return
  }
  await emitConnectionFailure(attemptGeneration)
}

async function attachBySession(sessionId: string): Promise<void> {
  const attemptGeneration = ++connectAttemptGeneration

  stopStatsPolling()
  stopStream()
  resetLegacyFrameState()
  beginConnectAttempt()
  beginHandover()
  const attached = await session.attach(sessionId)
  if (!isCurrentConnectAttempt(attemptGeneration)) {
    return
  }
  if (!attached) {
    if (props.initialSessionMode === 'main') {
      await rdpService.cancelRdpReturnToMain(sessionId).catch(() => undefined)
    }
    await emitConnectionFailure(attemptGeneration)
    return
  }
  try {
    if (props.initialSessionMode === 'main') {
      await rdpService.completeRdpReturnToMain(attached.id)
    } else {
      await rdpService.adoptRdpWindowSession(attached.id)
    }
  } catch (error) {
    if (props.initialSessionMode === 'main') {
      await rdpService.cancelRdpReturnToMain(attached.id).catch(() => undefined)
    }

    const decoded = decodeAppError(error)
    const message = decoded
      ? translateAppError(decoded, t('shell.rdp.windowHandoverFailed'))
      : error instanceof Error && error.message.trim()
        ? error.message
        : t('shell.rdp.windowHandoverFailed')
    await emitConnectionFailure(attemptGeneration, message)
    return
  }

  cancelHandover()
  emit('opened', attached.id)
  startStatsPolling()
  startStream(attached.id)
}

const certificateDismissed = ref(false)

const displayMode = ref<'fit' | 'fixed'>('fit')
const fixedSize = ref({ width: 1280, height: 800 })
const canvasScale = ref(1)
let fitResizeTimer: ReturnType<typeof setTimeout> | null = null
let lastFitSize = { width: 0, height: 0 }

let pendingResize: { width: number; height: number } | null = null
let resizeFallbackTimer: ReturnType<typeof setTimeout> | null = null

let dynamicResizeFailures = 0

let connectedAt = 0

let connectStartedAt = 0

const connectElapsedSeconds = ref(0)

const hasRenderedPicture = computed(() =>
  session.frameMode.value === 'stream' ? stream.appliedBatches.value > 0 : session.firstFrameReceived.value,
)

const showConnectingOverlay = computed(
  () => !hasRenderedPicture.value && !sessionClosed.value && session.status.value !== 'error',
)

const RDP_CONNECTING_MIN_VISIBLE_MS = 2000
const connectingOverlayShownAt = ref(0)
const connectingOverlayLinger = ref(false)
let connectingOverlayTimer: ReturnType<typeof setTimeout> | null = null

const connectingOverlayVisible = computed(() => showConnectingOverlay.value || connectingOverlayLinger.value)

watch(
  showConnectingOverlay,
  (visible) => {
    if (connectingOverlayTimer !== null) {
      clearTimeout(connectingOverlayTimer)
      connectingOverlayTimer = null
    }
    if (visible) {
      connectingOverlayShownAt.value = Date.now()
      connectingOverlayLinger.value = false
      return
    }
    const elapsed = Date.now() - connectingOverlayShownAt.value
    if (connectingOverlayShownAt.value === 0 || elapsed >= RDP_CONNECTING_MIN_VISIBLE_MS) {
      connectingOverlayLinger.value = false
      return
    }
    connectingOverlayLinger.value = true
    connectingOverlayTimer = setTimeout(() => {
      connectingOverlayTimer = null
      connectingOverlayLinger.value = false
    }, RDP_CONNECTING_MIN_VISIBLE_MS - elapsed)
  },
  { immediate: true },
)

watch(showConnectingOverlay, (visible) => {
  traceFit(
    `加载态${visible ? '显示' : '隐藏'}｜画面=${hasRenderedPicture.value}｜通道批次=${stream.appliedBatches.value}` +
      `｜首帧标记=${session.firstFrameReceived.value}｜模式=${session.frameMode.value}｜状态=${session.status.value}` +
      `｜后端已关闭=${sessionClosed.value}｜有错误文本=${session.errorText.value ? '是' : '否'}`,
  )
})

watch([() => session.sessionId.value, () => hasRenderedPicture.value], ([, rendered]) => {
  if (rendered) {
    connectStartedAt = 0
    connectElapsedSeconds.value = 0
    return
  }
  connectStartedAt = Date.now()
  connectElapsedSeconds.value = 0
})

const fitTrace = ref<string[]>([])
function traceFit(message: string): void {
  const stamp = new Date().toLocaleTimeString('zh-CN', { hour12: false })
  fitTrace.value = [...fitTrace.value.slice(-49), `${stamp} ${message}`]
}

const logActionHint = computed(() =>
  [clipboardHint.value, ...(fitTrace.value.length > 0 ? ['— 适配追踪（临时诊断）—', ...fitTrace.value] : [])]
    .filter(Boolean)
    .join('\n'),
)

function measuredSizeText(): string {
  const size = measureFitSize()
  return `${size.width}×${size.height}`
}

function measureFitSize(): { width: number; height: number } {
  const element = canvasWrap.value
  if (!element) {
    return { width: 1280, height: 800 }
  }

  if (element.clientWidth <= 0 || element.clientHeight <= 0) {
    return lastFitSize.width > 0 && lastFitSize.height > 0 ? lastFitSize : { width: 1280, height: 800 }
  }
  const clamp = (value: number, min: number, max: number): number =>
    Math.max(min, Math.min(max, Math.floor(value / 2) * 2))
  return {
    width: clamp(element.clientWidth, 800, 3840),
    height: clamp(element.clientHeight, 600, 2160),
  }
}

function paneHasLayoutBox(): boolean {
  const element = canvasWrap.value
  return Boolean(element && element.clientWidth > 0 && element.clientHeight > 0)
}

async function waitForLayoutBox(timeoutMs = 1500): Promise<boolean> {
  if (paneHasLayoutBox() || !environmentHasLayout()) {
    return true
  }
  const start = Date.now()
  while (!disposed && Date.now() - start < timeoutMs) {
    await new Promise<void>((resolve) => {
      let settled = false
      const finish = (): void => {
        if (settled) return
        settled = true
        resolve()
      }
      if (typeof requestAnimationFrame === 'function') {
        requestAnimationFrame(finish)
      }
      setTimeout(finish, 32)
    })
    if (paneHasLayoutBox()) {
      traceFit(`等待布局盒 ${Date.now() - start}ms 后可用`)
      return true
    }
  }
  traceFit(`等待布局盒超时 ${timeoutMs}ms：只能用默认尺寸连接`)
  return false
}

function environmentHasLayout(): boolean {
  if (typeof document === 'undefined') {
    return false
  }
  const body = document.body
  return Boolean(body && (body.clientWidth > 0 || body.clientHeight > 0))
}

function currentDesktopSize(): { width: number; height: number } {
  if (displayMode.value === 'fixed') {
    return { width: fixedSize.value.width, height: fixedSize.value.height }
  }
  return measureFitSize()
}

function remoteDesktopSize(): { width: number; height: number } {
  if (session.frameMode.value === 'stream') {
    const size = session.frameSize.value
    if (size.width > 0 && size.height > 0) {
      return size
    }
  }
  const stats = session.stats.value
  return { width: stats?.width ?? 0, height: stats?.height ?? 0 }
}

function updateCanvasScale(): void {
  const element = canvasWrap.value
  const desktop = remoteDesktopSize()
  if (!element || desktop.width <= 0 || desktop.height <= 0) {
    canvasScale.value = 1
    return
  }
  const availableWidth = element.clientWidth
  const availableHeight = element.clientHeight

  if (availableWidth <= 0 || availableHeight <= 0) {
    return
  }

  const scale = Math.min(1, availableWidth / desktop.width, availableHeight / desktop.height)
  canvasScale.value = Math.max(0.25, scale)
}

watch(
  () => session.stats.value,
  () => {
    const stats = session.stats.value
    if (pendingResize && stats) {
      if (stats.width === pendingResize.width && stats.height === pendingResize.height) {
        clearResizeFallback()
        dynamicResizeFailures = 0
      } else if (stats.resizeUnsupported) {
        clearResizeFallback()
        dynamicResizeFailures += 1
        void reconnect()
      }
    }
    scheduleFitReconnect()
  },
)

const statusLabel = computed(() => {
  if (sessionClosed.value) return '已结束'
  switch (session.status.value) {
    case 'connecting':
      return '连接中'
    case 'connected':
      return '已连接'
    case 'error':
      return '连接出错'
    default:
      return '未连接'
  }
})

const logDialogOpen = ref(false)

const statusColor = computed(() => {
  if (session.errorText.value || sessionError.value || sessionClosed.value) {
    return 'text-red-500'
  }
  if (session.connected.value) {
    return 'text-emerald-500'
  }
  return 'text-amber-400'
})

const statusTone = computed<'idle' | 'connected' | 'error'>(() => {
  if (session.errorText.value || sessionError.value || sessionClosed.value) return 'error'
  if (session.connected.value) return 'connected'
  return 'idle'
})

const statusHiddenByPointer = ref(false)
const STATUS_HOT_ZONE_WIDTH = 180
const STATUS_HOT_ZONE_HEIGHT = 48

function updateStatusHotZone(event: MouseEvent): void {
  const rect = container.value?.getBoundingClientRect()
  if (!rect) return
  statusHiddenByPointer.value =
    event.clientX >= rect.right - STATUS_HOT_ZONE_WIDTH && event.clientY <= rect.top + STATUS_HOT_ZONE_HEIGHT
}

function handlePaneMouseLeave(): void {
  statusHiddenByPointer.value = false
  releasePressedButtons()
}

const statusTooltip = computed(() => {
  const parts: string[] = [statusLabel.value]
  const stats = session.stats.value
  if (stats?.engine) parts.push(`引擎 ${stats.engine}`)
  if (stats) parts.push(`远端桌面 ${stats.width}×${stats.height}`)
  if (session.frameMode.value === 'stream') {
    parts.push(`画面通道 ${streamStateText.value || t('shell.rdp.stream.connecting')}`)
    parts.push(`已确认批次 ${stream.appliedBatches.value}`)
    if (stream.keyFrames.value > 0) parts.push(`关键帧 ${stream.keyFrames.value}`)
    if (stream.reconnects.value > 0) parts.push(`通道重连 ${stream.reconnects.value} 次`)
    parts.push(`最近一批绘制 ${stream.lastRenderMs.value.toFixed(1)} ms`)
  } else {
    parts.push(`累计更新 ${session.sequence.value} 帧`)
  }

  parts.push(`${t('shell.rdp.rate.serverLabel')} ${serverFpsText.value}：${t('shell.rdp.rate.serverTooltip')}`)
  parts.push(`${t('shell.rdp.rate.renderLabel')} ${renderRateText.value}：${renderRateTooltip.value}`)
  const reason = session.errorText.value || sessionError.value || session.fallbackReason.value
  if (reason) parts.push(reason)

  if (session.connected.value && !hasAnyFrame.value && !sessionClosed.value) {
    parts.push(t('shell.rdp.noFrameHint'))
  }
  return parts.join('｜')
})

function openLog(): void {
  logDialogOpen.value = true
}

const streamStateText = computed(() => {
  if (session.status.value === 'idle') {
    return ''
  }
  if (session.frameMode.value === 'legacy') {
    if (session.fallbackReason.value) {
      return t('shell.rdp.stream.fallback')
    }
    return t('shell.rdp.stream.legacyMode')
  }
  switch (stream.state.value) {
    case 'ready':
      return t('shell.rdp.stream.connected')
    case 'connecting':
      return t('shell.rdp.stream.connecting')
    case 'failed':
    case 'stopped':
      return t('shell.rdp.stream.fallback')
    default:
      return ''
  }
})

const SERVER_FPS_STALE_MS = 3000
const statsTick = ref(0)
let statsSeenAt = 0

watch(
  () => session.stats.value,
  () => {
    statsSeenAt = Date.now()
    statsTick.value += 1
  },
)

const serverFps = computed(() => {
  void statsTick.value
  if (statsSeenAt === 0 || Date.now() - statsSeenAt > SERVER_FPS_STALE_MS) {
    return 0
  }
  return readServerFps(session.stats.value)
})

const renderRate = computed(() =>
  session.frameMode.value === 'stream' ? stream.renderRate.value : legacyRenderRate.value,
)

const serverFpsText = computed(() => `${serverFps.value.toFixed(1)}/s`)
const renderRateText = computed(() => `${renderRate.value.toFixed(1)}/s`)

const liveResolutionText = computed(() => {
  const size = session.frameSize.value
  if (size.width <= 0 || size.height <= 0) return ''
  return `${size.width}×${size.height}`
})

const rateReadoutText = computed(() => `${serverFpsText.value} / ${renderRateText.value}`)

const rateReadoutTooltip = computed(
  () =>
    `${t('shell.rdp.rate.serverLabel')} ${serverFpsText.value}：${t('shell.rdp.rate.serverTooltip')}｜` +
    `${t('shell.rdp.rate.renderLabel')} ${renderRateText.value}：${renderRateTooltip.value}`,
)

const resolutionTooltip = computed(() => `${t('shell.rdp.resolution.tooltip')} ${liveResolutionText.value}`.trim())

const renderRateTooltip = computed(() =>
  session.frameMode.value === 'stream' ? t('shell.rdp.rate.renderTooltip') : t('shell.rdp.rate.legacyRenderTooltip'),
)

const hasAnyFrame = computed(() =>
  session.frameMode.value === 'stream' ? stream.appliedBatches.value > 0 : session.sequence.value > 0,
)

let rateTimer: ReturnType<typeof setInterval> | null = null

function startRateTimer(): void {
  if (rateTimer !== null) {
    return
  }
  rateTimer = setInterval(() => {
    statsTick.value += 1
    legacyRenderRate.value = legacyRateWindow.rate()
    if (showConnectingOverlay.value && connectStartedAt > 0) {
      connectElapsedSeconds.value = Math.max(0, Math.round((Date.now() - connectStartedAt) / 1000))
    }
  }, RDP_RENDER_RATE_TICK_MS)
}

function stopRateTimer(): void {
  if (rateTimer !== null) {
    clearInterval(rateTimer)
    rateTimer = null
  }
}

watch(
  () => session.frameMode.value,
  (mode) => {
    if (mode === 'stream') {
      startStream(session.sessionId.value)
      return
    }
    stopStream()
    void nextTick(() => {
      updateCanvasScale()
      redraw()
    })
  },
)

watch(
  () => [
    session.stats.value?.width,
    session.stats.value?.height,
    session.frameSize.value.width,
    session.frameSize.value.height,
    session.sequence.value,
  ],
  () => {
    void nextTick(() => updateCanvasScale())
  },
)

const fitGraceMs = computed(() => Math.max(0, rdpDisplayPreferences.fitGraceSeconds) * 1000)

const resizeDebounceMs = 1000

async function applyFitSize(next: { width: number; height: number }): Promise<void> {
  lastFitSize = next
  const id = session.info.value?.id
  if (!id || dynamicResizeFailures >= 2) {
    void reconnect()
    return
  }

  if (Date.now() - connectedAt < fitGraceMs.value) {
    const baseline = measureFitSize()
    traceFit(
      `适配(宽限期内 ${Math.round(fitGraceMs.value / 1000)}s) 只重设基线=${baseline.width}×${baseline.height} 原基线=${next.width}×${next.height}`,
    )
    lastFitSize = baseline
    return
  }
  traceFit(
    `适配 请求=${next.width}×${next.height} 远端当前=${remoteDesktopSize().width}×${remoteDesktopSize().height} 距连接=${Math.round((Date.now() - connectedAt) / 1000)}s`,
  )
  try {
    await rdpService.resizeRdpSession(id, next.width, next.height)
  } catch {
    traceFit('适配失败：请求发不出去 → 重连')
    void reconnect()
    return
  }
  pendingResize = next
  if (resizeFallbackTimer !== null) {
    clearTimeout(resizeFallbackTimer)
  }
  resizeFallbackTimer = setTimeout(() => {
    resizeFallbackTimer = null
    if (!pendingResize) {
      return
    }
    pendingResize = null
    dynamicResizeFailures += 1
    traceFit(`适配超时 3s 未确认 ${next.width}×${next.height} → 重连`)
    void reconnect()
  }, 3000)
}

function clearResizeFallback(): void {
  if (resizeFallbackTimer !== null) {
    clearTimeout(resizeFallbackTimer)
    resizeFallbackTimer = null
  }
  pendingResize = null
}

function scheduleFitReconnect(): void {
  if (displayMode.value !== 'fit' || session.status.value !== 'connected' || !session.firstFrameReceived.value) {
    return
  }

  if (!paneHasLayoutBox()) {
    return
  }
  const next = measureFitSize()

  const changed = Math.abs(next.width - lastFitSize.width) >= 32 || Math.abs(next.height - lastFitSize.height) >= 32
  if (!changed) {
    return
  }
  traceFit(`复检命中 实测=${next.width}×${next.height} 基线=${lastFitSize.width}×${lastFitSize.height}`)
  if (fitResizeTimer !== null) {
    clearTimeout(fitResizeTimer)
  }
  fitResizeTimer = setTimeout(() => {
    fitResizeTimer = null
    void applyFitSize(next)
  }, resizeDebounceMs)
}
const certificateDialogOpen = computed(() => !certificateDismissed.value && certificatePending.value)
const certificateBusy = ref(false)

async function confirmCertificate(): Promise<void> {
  if (!props.hostId) {
    return
  }
  certificateBusy.value = true
  try {
    await rdpService.trustRdpCertificate(props.hostId, session.certificateFingerprint.value)
    await connectByHost()
  } finally {
    certificateBusy.value = false
  }
}

function cancelCertificate(): void {
  connectAttemptGeneration += 1
  certificateDismissed.value = true
  stopStream()
  void session.close()
}

async function reconnect(): Promise<void> {
  if (disposed) {
    return
  }
  if (props.hostId) {
    await connectByHost()
    return
  }
  if (props.input) {
    const attemptGeneration = ++connectAttemptGeneration
    beginConnectAttempt()
    stopStream()
    resetLegacyFrameState()
    const opened = await session.open({ ...props.input, engine: engine.value })
    if (!isCurrentConnectAttempt(attemptGeneration)) {
      return
    }
    if (opened) {
      startStatsPolling()
      emit('opened', opened.id)
      startStream(opened.id)
      return
    }
    await emitConnectionFailure(attemptGeneration)
  }
}

async function submitPendingPassword(): Promise<void> {
  await connectByHost(pendingPassword.value)
}

onMounted(async () => {
  if (props.hostId) {
    try {
      const profile = await rdpService.fetchRdpHostProfile(props.hostId)
      if (disposed) {
        return
      }

      if (typeof profile?.clipboardMode === 'string' && profile.clipboardMode) {
        clipboardMode.value = profile.clipboardMode
      }
      if (typeof profile?.clipboardContent === 'string' && profile.clipboardContent) {
        clipboardContent.value = profile.clipboardContent
      }
      if (profile?.displayMode === 'fixed') {
        displayMode.value = 'fixed'
        fixedSize.value = {
          width: profile.width && profile.width > 0 ? profile.width : 1280,
          height: profile.height && profile.height > 0 ? profile.height : 800,
        }
      }
    } catch {}
  }

  if (disposed) {
    return
  }

  watch(
    () => session.status.value,
    (value) => {
      if (value === 'idle') {
        stopStatsPolling()
      }
    },
  )

  const handedOverSessionId = props.initialSessionId?.trim()
  if (handedOverSessionId) {
    await attachBySession(handedOverSessionId)
    return
  }
  if (props.hostId) {
    await connectByHost()
    return
  }
  if (!props.input) {
    return
  }
  const attemptGeneration = ++connectAttemptGeneration
  stopStream()
  resetLegacyFrameState()
  const opened = await session.open({ ...props.input, engine: engine.value })
  if (!isCurrentConnectAttempt(attemptGeneration)) {
    return
  }
  if (!opened) {
    await emitConnectionFailure(attemptGeneration)
    return
  }
  emit('opened', opened.id)
  startStatsPolling()
  startStream(opened.id)
})

defineExpose({
  openLog,
  reconnect,
  disconnect: close,

  pasteToRemote: pasteLocalClipboard,

  copyFromRemote: copyRemoteClipboard,

  canPasteToRemote,
  canCopyFromRemote,
  showHelperLog,
  helperStderr,
  sessionError,
  sessionClosed,
  connected: session.connected,
  lastErrorText: session.errorText,
  statusTone,
  statusTooltip,

  sessionId: session.sessionId,

  beginHandover,

  cancelHandover,
})

let resizeObserver: ResizeObserver | null = null
if (typeof ResizeObserver === 'function') {
  resizeObserver = new ResizeObserver(() => {
    updateCanvasScale()
    scheduleFitReconnect()
  })
}

onMounted(() => {
  if (resizeObserver && container.value) {
    resizeObserver.observe(container.value)
  }

  startRateTimer()
})

onBeforeUnmount(() => {
  disposed = true
  connectAttemptGeneration += 1
  resizeObserver?.disconnect()
  resizeObserver = null
  if (fitResizeTimer !== null) {
    clearTimeout(fitResizeTimer)
    fitResizeTimer = null
  }
  clearResizeFallback()
  if (connectingOverlayTimer !== null) {
    clearTimeout(connectingOverlayTimer)
    connectingOverlayTimer = null
  }
  stopStatsPolling()

  stopStream()
  stopRateTimer()
  legacyRateWindow.reset()

  resetClipboardSyncState()

  if (!handoverPending.value) {
    void session.close()
  }
})
</script>

<template>
  <div
    ref="container"
    class="relative flex h-full w-full flex-col overflow-hidden rounded-md border border-qt-theme-divider bg-black outline-none select-none"
    tabindex="0"
    @mousemove="handleMouseMove"
    @mousedown="handleMouseDown"
    @mouseup="handleMouseUp"
    @mouseleave="handlePaneMouseLeave"
    @blur="handlePaneBlur"
    @wheel="handleWheel"
    @keydown="handleKeyDown"
    @keyup="handleKeyUp"
    @contextmenu.prevent
  >
    <div class="relative min-h-0 w-full flex-1 overflow-hidden bg-black">
      <div ref="canvasWrap" class="absolute inset-0 overflow-hidden">
        <canvas
          ref="canvas"
          :style="{ transform: `scale(${canvasScale})`, transformOrigin: 'top left' }"
          class="block max-h-full max-w-full"
        />
      </div>

      <SessionOverlay
        v-if="connectingOverlayVisible || failureOverlayVisible"
        class="pointer-events-none"
        surface="canvas"
        :data-rdp-failure="failureOverlayVisible ? '' : undefined"
        :tone="failureOverlayVisible ? 'error' : 'info'"
        :busy="!failureOverlayVisible"
        :title="t(failureOverlayVisible ? 'shell.rdp.connectionFailed' : 'shell.rdp.connecting')"
      >
        <template v-if="failureOverlayVisible">
          <span class="text-xs leading-5 break-words text-qt-overlay-muted" data-rdp-failure-reason>
            {{ failureText }}
          </span>
        </template>
        <template v-else>
          <span v-if="connectElapsedSeconds > 0" class="text-xs text-qt-overlay-muted">
            {{ t('shell.rdp.connectingElapsed', { seconds: connectElapsedSeconds }) }}
          </span>
          <span class="text-[11px] leading-4 text-qt-overlay-muted">
            {{
              connectElapsedSeconds >= rdpDisplayPreferences.hostProbeTimeoutSeconds + 1
                ? t('shell.rdp.connectingNegotiating')
                : t('shell.rdp.connectingHint')
            }}
          </span>
        </template>
        <template #actions>
          <template v-if="failureOverlayVisible">
            <QtermButton v-if="canRetry" class="pointer-events-auto" data-rdp-retry @click="retryConnection">
              {{ t('shell.rdp.retryConnect') }}
            </QtermButton>
            <QtermButton
              variant="outline"
              class="pointer-events-auto border-qt-overlay-border bg-qt-overlay-surface text-qt-overlay-fg"
              data-rdp-close
              @click="emit('close')"
            >
              {{ t('shell.rdp.closePane') }}
            </QtermButton>
          </template>
          <QtermButton
            v-else
            variant="outline"
            class="pointer-events-auto border-qt-overlay-border bg-qt-overlay-surface text-qt-overlay-fg"
            @click="emit('close')"
          >
            {{ t('shell.rdp.cancelConnect') }}
          </QtermButton>
        </template>
      </SessionOverlay>

      <form
        v-if="passwordFormVisible"
        class="absolute inset-x-0 bottom-0 z-20 flex-cc gap-2 bg-amber-500/15 px-2 py-1 text-xs text-amber-100"
        @submit.prevent="submitPendingPassword"
      >
        <span>该主机未保存密码：</span>
        <input
          v-model="pendingPassword"
          class="rounded bg-black/30 px-2 py-0.5 text-white"
          placeholder="输入本次连接密码"
          type="password"
          autocomplete="off"
          autocorrect="off"
          autocapitalize="off"
          spellcheck="false"
        />
        <button class="rounded bg-sky-600 px-2 py-0.5 hover:bg-sky-500" type="submit">连接</button>
      </form>

      <div
        class="pointer-events-none absolute top-2 right-2 z-10 flex items-center gap-2 rounded-md bg-black/45 px-2 py-1 text-[11px] backdrop-blur-sm transition-opacity duration-150"
        :class="statusHiddenByPointer ? 'opacity-0' : 'opacity-100'"
        :title="statusTooltip"
        data-rdp-status
      >
        <span class="text-base leading-none" :class="statusColor">●</span>
        <span
          v-if="rdpDisplayPreferences.showResolution && liveResolutionText"
          class="pointer-events-auto text-white/70"
          :title="resolutionTooltip"
          data-rdp-resolution
        >
          {{ liveResolutionText }}
        </span>
        <span
          v-if="rdpDisplayPreferences.showFps && session.connected.value"
          class="pointer-events-auto text-white/70"
          :title="rateReadoutTooltip"
          data-rdp-fps
        >
          {{ rateReadoutText }}
        </span>
      </div>
    </div>
  </div>

  <RdpLogDialog
    :open="logDialogOpen"
    :host-title="props.hostTitle || props.hostId || ''"
    :state-text="statusTooltip"
    :stderr-text="helperStderr"
    :action-hint="logActionHint"
    @close="logDialogOpen = false"
  />

  <RdpCertificateDialog
    :open="certificateDialogOpen"
    :state="session.certificateState.value"
    :host-title="props.hostTitle || props.hostId || ''"
    :fingerprint="session.certificateFingerprint.value"
    :subject="session.certificateSubject.value"
    :issuer="session.certificateIssuer.value"
    :busy="certificateBusy"
    @confirm="confirmCertificate"
    @cancel="cancelCertificate"
  />
</template>
