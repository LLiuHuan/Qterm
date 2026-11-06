import { getCurrentInstance, onBeforeUnmount, ref, type Ref } from 'vue'
import { fetchRdpStreamTicket } from '../../services/remotedesktop'
import { RdpFrameRenderer, type RdpFrameRendererOptions, type RdpUpdateFrame } from './rdpRenderer'
import { RdpFrameStream, type RdpFrameStreamOptions, type RdpStreamState, type RdpStreamTicket } from './rdpStream'

export const RDP_RENDER_RATE_WINDOW_MS = 2000

export const RDP_RENDER_RATE_TICK_MS = 250

export interface RdpRateWindow {
  record(): void

  rate(): number

  reset(): void
}

export function createRdpRateWindow(options: { windowMs?: number; now?: () => number } = {}): RdpRateWindow {
  const windowMs = options.windowMs ?? RDP_RENDER_RATE_WINDOW_MS
  const now = options.now ?? defaultNow
  let samples: number[] = []
  return {
    record(): void {
      const at = now()
      samples = samples.filter((sample) => at - sample < windowMs)
      samples.push(at)
    },
    rate(): number {
      const at = now()
      samples = samples.filter((sample) => at - sample < windowMs)
      return samples.length / (windowMs / 1000)
    },
    reset(): void {
      samples = []
    },
  }
}

export interface UseRdpStreamOptions {
  acquireTicket?: (sessionId: string) => Promise<RdpStreamTicket>

  createStream?: (options: RdpFrameStreamOptions) => RdpFrameStream

  createRenderer?: (options?: RdpFrameRendererOptions) => RdpFrameRenderer

  onError?: (message: string, fatal: boolean) => void

  onFrameSize?: (size: { width: number; height: number }) => void

  now?: () => number
}

export function useRdpStream(options: UseRdpStreamOptions = {}) {
  const now = options.now ?? defaultNow
  const acquireTicket = options.acquireTicket ?? ((sessionId: string) => fetchRdpStreamTicket(sessionId))
  const createStream =
    options.createStream ?? ((streamOptions: RdpFrameStreamOptions) => new RdpFrameStream(streamOptions))
  const createRenderer =
    options.createRenderer ?? ((rendererOptions?: RdpFrameRendererOptions) => new RdpFrameRenderer(rendererOptions))

  const rateWindow = createRdpRateWindow({ now })

  const state = ref<RdpStreamState>('idle')
  const started = ref(false)
  const renderRate = ref(0)
  const lastRenderMs = ref(0)
  const appliedBatches = ref(0)
  const keyFrames = ref(0)
  const reconnects = ref(0)
  const lastError = ref('')

  let stream: RdpFrameStream | null = null
  let renderer: RdpFrameRenderer | null = null
  let ticker: ReturnType<typeof setInterval> | null = null
  let attachedCanvas: HTMLCanvasElement | null = null
  let lastSize = { width: 0, height: 0 }

  let fatal = false

  function ensureRenderer(): RdpFrameRenderer {
    if (!renderer) {
      renderer = createRenderer({ now })
      if (attachedCanvas) {
        renderer.attach(attachedCanvas)
      }
    }
    return renderer
  }

  function noteSize(width: number, height: number, force: boolean): void {
    const safeWidth = Math.max(0, Math.floor(width))
    const safeHeight = Math.max(0, Math.floor(height))
    if (safeWidth <= 0 || safeHeight <= 0) {
      return
    }
    if (!force && safeWidth === lastSize.width && safeHeight === lastSize.height) {
      return
    }
    lastSize = { width: safeWidth, height: safeHeight }
    options.onFrameSize?.({ width: safeWidth, height: safeHeight })
  }

  function report(message: string, isFatal: boolean): void {
    lastError.value = message
    if (isFatal) {
      fatal = true
    }
    options.onError?.(message, isFatal)
  }

  async function handleUpdate(update: RdpUpdateFrame): Promise<void> {
    const target = renderer ?? ensureRenderer()
    const stats = await target.apply(update)
    lastRenderMs.value = stats.renderMs
    appliedBatches.value += 1
    if (stats.keyFrame) {
      keyFrames.value += 1
    }
    rateWindow.record()

    renderRate.value = rateWindow.rate()
    lastError.value = ''
    noteSize(update.width, update.height, false)
  }

  function handleError(error: { code: number; reason: string; fatal: boolean }): void {
    report(error.reason, error.fatal)
    if (error.fatal) {
      stopTicker()
    }
  }

  function handleStateChange(next: RdpStreamState): void {
    if (fatal && next === 'stopped') {
      return
    }
    state.value = next
    if (stream) {
      reconnects.value = stream.reconnects
    }
  }

  function stopTicker(): void {
    if (ticker !== null) {
      clearInterval(ticker)
      ticker = null
    }
  }

  function startTicker(): void {
    stopTicker()
    ticker = setInterval(() => {
      renderRate.value = rateWindow.rate()
      if (stream) {
        reconnects.value = stream.reconnects
      }
    }, RDP_RENDER_RATE_TICK_MS)
  }

  function resetCounters(): void {
    appliedBatches.value = 0
    keyFrames.value = 0
    reconnects.value = 0
    lastRenderMs.value = 0
    lastError.value = ''
    renderRate.value = 0
    lastSize = { width: 0, height: 0 }
    rateWindow.reset()
  }

  function bindCanvas(): void {
    if (!attachedCanvas) {
      return
    }
    try {
      ensureRenderer().attach(attachedCanvas)
    } catch (error) {
      report(describeError(error), false)
    }
  }

  function attach(canvas: HTMLCanvasElement | null): void {
    attachedCanvas = canvas
    if (!canvas) {
      renderer?.detach()
      return
    }
    bindCanvas()
  }

  function start(sessionId: string): void {
    if (!sessionId) {
      return
    }
    stop()
    fatal = false
    resetCounters()
    bindCanvas()
    started.value = true
    const instance = createStream({
      sessionId,
      acquireTicket,
      onReady: (info) => {
        lastError.value = ''
        noteSize(info.width, info.height, true)
      },
      onUpdate: handleUpdate,
      onError: handleError,
      onStateChange: handleStateChange,
      now,
    })
    stream = instance
    startTicker()
    instance.start()
  }

  function stop(): void {
    stopTicker()
    const instance = stream
    stream = null
    if (instance) {
      instance.stop()
    }
    renderer?.detach()
    started.value = false
    fatal = false
    state.value = 'stopped'
    rateWindow.reset()
    renderRate.value = 0
  }

  if (getCurrentInstance()) {
    onBeforeUnmount(() => stop())
  }

  return {
    state: state as Ref<RdpStreamState>,
    started,
    attach,
    start,
    stop,
    renderRate,
    lastRenderMs,
    appliedBatches,
    keyFrames,
    reconnects,
    lastError,
  }
}

export type RdpStreamController = ReturnType<typeof useRdpStream>

function defaultNow(): number {
  return typeof performance === 'undefined' ? Date.now() : performance.now()
}

function describeError(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }
  if (typeof error === 'string') {
    return error
  }
  return '画面通道异常'
}
