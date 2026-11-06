import { Stream } from '@wailsio/runtime'
import {
  RDP_PROTOCOL_VERSION,
  RdpErrorCode,
  encodeAckFrame,
  encodeBindFrame,
  encodeDropFrame,
  parseStreamFrame,
  type StreamFrame,
} from './rdpProtocol'
import type { RdpUpdateFrame } from './rdpRenderer'

export type RdpStreamState = 'idle' | 'connecting' | 'ready' | 'stopped' | 'failed'

export interface RdpStreamTicket {
  streamName: string
  token: string
  protocolVersion: number
}

export interface RdpBinarySocket {
  readyState: number
  readonly OPEN: number
  binaryType: string
  onopen: ((event: Event) => void) | null
  onmessage: ((event: MessageEvent) => void) | null
  onclose: ((event: CloseEvent) => void) | null
  onerror: ((event: Event) => void) | null
  bufferedAmount: number
  send(data: Uint8Array): void
  close(code?: number, reason?: string): void
}

export interface RdpFrameStreamOptions {
  sessionId: string

  acquireTicket: (sessionId: string) => Promise<RdpStreamTicket>

  onReady?: (info: { width: number; height: number; batchId: bigint }) => void

  onUpdate: (update: RdpUpdateFrame) => Promise<void> | void

  onError?: (error: { code: number; reason: string; fatal: boolean }) => void

  onStateChange?: (state: RdpStreamState) => void

  openSocket?: (name: string) => RdpBinarySocket

  now?: () => number

  retryDelayMs?: number
  maxRetryDelayMs?: number

  skipReconnectWhenHidden?: boolean
}

const DEFAULT_RETRY_DELAY_MS = 400
const DEFAULT_MAX_RETRY_DELAY_MS = 4000

const DROP_REASON_DECODE_FAILED = 1

function pageVisible(): boolean {
  if (typeof document === 'undefined') {
    return true
  }
  return document.visibilityState !== 'hidden'
}

export class RdpFrameStream {
  private readonly options: RdpFrameStreamOptions
  private readonly retryDelayMs: number
  private readonly maxRetryDelayMs: number
  private readonly now: () => number

  private socket: RdpBinarySocket | null = null
  private currentState: RdpStreamState = 'idle'
  private timer: ReturnType<typeof setTimeout> | null = null
  private retryDelay = 0
  private generation = 0
  private stopped = false
  private fatal = false
  private visibilityBound = false

  private appliedBatches = 0
  private reconnectCount = 0

  constructor(options: RdpFrameStreamOptions) {
    this.options = options
    this.retryDelayMs = options.retryDelayMs ?? DEFAULT_RETRY_DELAY_MS
    this.maxRetryDelayMs = options.maxRetryDelayMs ?? DEFAULT_MAX_RETRY_DELAY_MS
    this.now = options.now ?? (() => (typeof performance === 'undefined' ? Date.now() : performance.now()))
  }

  get state(): RdpStreamState {
    return this.currentState
  }

  get batches(): number {
    return this.appliedBatches
  }

  get reconnects(): number {
    return this.reconnectCount
  }

  start(): void {
    if (this.stopped) {
      return
    }
    this.bindVisibility()
    this.generation += 1
    void this.connect(this.generation)
  }

  stop(): void {
    this.stopped = true
    this.generation += 1
    this.clearTimer()
    this.unbindVisibility()
    this.closeSocket()
    this.setState('stopped')
  }

  private closeSocket(): void {
    const socket = this.socket
    this.socket = null
    if (!socket) {
      return
    }
    socket.onopen = null
    socket.onmessage = null
    socket.onclose = null
    socket.onerror = null
    try {
      socket.close()
    } catch {}
  }

  private async connect(generation: number): Promise<void> {
    if (this.stopped || generation !== this.generation) {
      return
    }
    this.setState('connecting')
    let ticket: RdpStreamTicket
    try {
      ticket = await this.options.acquireTicket(this.options.sessionId)
    } catch (error) {
      this.reportError(RdpErrorCode.Binding, describeError(error), true)
      return
    }
    if (this.stopped || generation !== this.generation) {
      return
    }
    if (ticket.protocolVersion !== RDP_PROTOCOL_VERSION) {
      this.reportError(
        RdpErrorCode.Protocol,
        `前后端画面协议版本不一致：前端 ${RDP_PROTOCOL_VERSION}，后端 ${ticket.protocolVersion}`,
        true,
      )
      return
    }

    this.closeSocket()
    const openSocket = this.options.openSocket ?? ((name: string) => Stream(name) as unknown as RdpBinarySocket)
    let socket: RdpBinarySocket
    try {
      socket = openSocket(ticket.streamName)
    } catch (error) {
      this.scheduleReconnect(generation, describeError(error))
      return
    }
    socket.binaryType = 'arraybuffer'
    this.socket = socket

    socket.onopen = () => {
      if (generation !== this.generation) {
        return
      }
      try {
        socket.send(encodeBindFrame(this.options.sessionId, ticket.token))
      } catch (error) {
        this.handleDisconnect(generation, describeError(error))
      }
    }
    socket.onmessage = (event: MessageEvent) => {
      if (generation !== this.generation) {
        return
      }
      void this.handleMessage(socket, event, generation)
    }
    socket.onerror = () => {}
    socket.onclose = () => {
      if (generation !== this.generation) {
        return
      }
      this.handleDisconnect(generation, '画面通道已关闭')
    }
  }

  private async handleMessage(socket: RdpBinarySocket, event: MessageEvent, generation: number): Promise<void> {
    let frame: StreamFrame
    try {
      frame = parseStreamFrame(event.data as ArrayBuffer | Uint8Array)
    } catch (error) {
      this.reportError(RdpErrorCode.Protocol, describeError(error), false)
      this.handleDisconnect(generation, describeError(error))
      return
    }
    if (frame.kind === 'ready') {
      this.options.onReady?.({ width: frame.width, height: frame.height, batchId: frame.batchId })
      this.retryDelay = 0
      this.setState('ready')
      return
    }
    if (frame.kind === 'error') {
      const fatal = frame.code === RdpErrorCode.Binding || frame.code === RdpErrorCode.Session
      this.reportError(frame.code, frame.reason, fatal)
      if (fatal) {
        this.stop()
      } else {
        this.handleDisconnect(generation, frame.reason)
      }
      return
    }

    const startedAt = this.now()
    try {
      await this.options.onUpdate(frame)
    } catch (error) {
      try {
        socket.send(encodeDropFrame(frame.batchId, DROP_REASON_DECODE_FAILED))
      } catch {}
      this.reportError(RdpErrorCode.Protocol, describeError(error), false)
      return
    }
    if (this.stopped || generation !== this.generation) {
      return
    }
    const renderMicros = Math.max(0, Math.round((this.now() - startedAt) * 1000))
    try {
      socket.send(encodeAckFrame(frame.batchId, renderMicros))
      this.appliedBatches += 1
    } catch (error) {
      this.handleDisconnect(generation, describeError(error))
    }
  }

  private handleDisconnect(generation: number, reason: string): void {
    if (this.stopped || generation !== this.generation) {
      return
    }
    this.closeSocket()
    if (this.fatal) {
      return
    }
    if (this.skipReconnect()) {
      this.setState('connecting')
      return
    }
    this.scheduleReconnect(generation, reason)
  }

  private skipReconnect(): boolean {
    const skip = this.options.skipReconnectWhenHidden ?? true
    return skip && !pageVisible()
  }

  private scheduleReconnect(generation: number, reason: string): void {
    if (this.stopped || generation !== this.generation || this.timer !== null) {
      return
    }
    this.reconnectCount += 1
    this.retryDelay = this.retryDelay === 0 ? this.retryDelayMs : Math.min(this.retryDelay * 2, this.maxRetryDelayMs)
    this.reportError(RdpErrorCode.Transport, reason, false)
    this.setState('connecting')
    this.timer = setTimeout(() => {
      this.timer = null
      const next = this.generation
      void this.connect(next)
    }, this.retryDelay)
  }

  private clearTimer(): void {
    if (this.timer !== null) {
      clearTimeout(this.timer)
      this.timer = null
    }
  }

  private reportError(code: number, reason: string, fatal: boolean): void {
    if (fatal) {
      this.fatal = true
    }
    this.options.onError?.({ code, reason, fatal })
    if (fatal) {
      this.setState('failed')
    }
  }

  private setState(state: RdpStreamState): void {
    if (this.currentState === state) {
      return
    }
    this.currentState = state
    this.options.onStateChange?.(state)
  }

  private bindVisibility(): void {
    if (this.visibilityBound || typeof document === 'undefined') {
      return
    }
    this.visibilityBound = true
    document.addEventListener('visibilitychange', this.onVisibilityChange)
  }

  private unbindVisibility(): void {
    if (!this.visibilityBound || typeof document === 'undefined') {
      return
    }
    this.visibilityBound = false
    document.removeEventListener('visibilitychange', this.onVisibilityChange)
  }

  private readonly onVisibilityChange = (): void => {
    if (this.stopped || this.fatal || !pageVisible()) {
      return
    }
    const socket = this.socket
    if (socket && socket.readyState === socket.OPEN) {
      return
    }

    this.clearTimer()
    this.generation += 1
    this.retryDelay = 0
    void this.connect(this.generation)
  }
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
