import { Stream } from '@wailsio/runtime'

export interface TerminalChannelHandlers {
  onReady(): void
  onOutput(data: Uint8Array, acknowledge: () => void): void

  onZmodemDetected?(direction: number): void
  onError(message: string): void
  onClose(): void
}

export interface TerminalChannel {
  sendInput(data: string): boolean | void

  sendBinary(data: Uint8Array): Promise<void>
  resize(columns: number, rows: number): void
  close(): void
}

const activeChannels = new Map<string, { channel: TerminalChannel; canSend: () => boolean }>()

export function registerTerminalChannel(
  sessionId: string,
  channel: TerminalChannel,
  canSend: () => boolean = () => true,
): void {
  activeChannels.set(sessionId, { channel, canSend })
}

export function unregisterTerminalChannel(sessionId: string, channel: TerminalChannel): void {
  if (activeChannels.get(sessionId)?.channel === channel) activeChannels.delete(sessionId)
}

export function sendTerminalInput(sessionId: string, data: string): boolean {
  const entry = activeChannels.get(sessionId)
  if (!entry || !entry.canSend()) return false
  try {
    return entry.channel.sendInput(data) !== false
  } catch {
    return false
  }
}

const PROTOCOL_VERSION = 1
const OP_BIND = 0x01
const OP_INPUT = 0x02
const OP_RESIZE = 0x03
const OP_ACK = 0x04
const OP_CLOSE = 0x05

const OP_ZMODEM = 0x06
const OP_READY = 0x81
const OP_OUTPUT = 0x82
const OP_ERROR = 0x83

const OP_ZMODEM_OUTPUT = 0x84

const OP_ZMODEM_DETECTED = 0x85
const OP_ZMODEM_ACK = 0x86
const MAX_DATA_SIZE = 32 << 10

const MAX_PENDING_OUTPUT_SIZE = 256 << 10
const MAX_REASON_SIZE = 512
const encoder = new TextEncoder()
const decoder = new TextDecoder()

export function connectTerminalChannel(
  sessionId: string,
  streamToken: string,
  columns: number,
  rows: number,
  handlers: TerminalChannelHandlers,
): TerminalChannel {
  const socket = Stream('terminal')
  socket.binaryType = 'arraybuffer'
  let ready = false
  let closing = false
  const pendingBinary: Array<{
    size: number
    resolve(): void
    reject(error: Error): void
    timer: ReturnType<typeof setTimeout>
  }> = []

  const pendingOutput: Uint8Array[] = []
  let pendingOutputBytes = 0
  const rejectBinary = () => {
    for (const pending of pendingBinary.splice(0)) {
      clearTimeout(pending.timer)
      pending.reject(new Error('终端数据通道已关闭'))
    }
  }

  const fail = (message: string) => {
    if (closing) return
    closing = true
    rejectBinary()
    handlers.onError(message)
    socket.close()
  }

  const deliverOutput = (output: Uint8Array) => {
    let acknowledged = false
    try {
      handlers.onOutput(output, () => {
        if (acknowledged || closing) return
        acknowledged = true
        try {
          socket.send(createUint32Frame(OP_ACK, output.byteLength))
        } catch {
          fail('无法确认终端输出')
        }
      })
    } catch {
      fail('终端无法处理远程输出')
    }
  }

  const queuePendingOutput = (output: Uint8Array): boolean => {
    if (ready) {
      deliverOutput(output)
      return true
    }
    if (pendingOutputBytes + output.byteLength > MAX_PENDING_OUTPUT_SIZE) return false
    pendingOutput.push(output)
    pendingOutputBytes += output.byteLength
    return true
  }

  const flushPendingOutput = () => {
    for (const output of pendingOutput.splice(0)) {
      pendingOutputBytes -= output.byteLength
      deliverOutput(output)
    }
  }

  socket.onopen = () => {
    if (import.meta.env.DEV) {
      console.info('[terminal-channel] socket open', { sessionId, columns, rows })
    }
    try {
      socket.send(createBindFrame(sessionId, streamToken, columns, rows))
      if (import.meta.env.DEV) console.info('[terminal-channel] bind frame sent', { sessionId })
    } catch (error) {
      console.warn('[terminal-channel] bind frame failed', { sessionId, error })
      fail('无法绑定终端数据通道')
    }
  }
  socket.onclose = (event: CloseEvent) => {
    if (import.meta.env.DEV) {
      console.info('[terminal-channel] socket closed', {
        sessionId,
        ready,
        code: event.code,
        reason: event.reason,
        wasClean: event.wasClean,
      })
    }
  }
  socket.onerror = () => {
    if (import.meta.env.DEV) console.warn('[terminal-channel] socket error', { sessionId })
  }
  socket.onmessage = (event: MessageEvent) => {
    if (!(event.data instanceof ArrayBuffer)) {
      fail('终端返回了不受支持的数据格式')
      return
    }
    const frame = new Uint8Array(event.data)
    if (frame.length < 2 || frame[0] !== PROTOCOL_VERSION) {
      fail('终端协议版本不兼容')
      return
    }
    const payload = frame.subarray(2)
    switch (frame[1]) {
      case OP_READY:
        if (payload.length !== 0 || ready) {
          fail('终端就绪帧无效')
          return
        }
        ready = true
        handlers.onReady()

        flushPendingOutput()
        break
      case OP_OUTPUT: {
        if (payload.length === 0 || payload.length > MAX_DATA_SIZE) {
          fail('终端输出帧无效')
          return
        }
        if (!queuePendingOutput(payload.slice())) {
          fail('终端在就绪前发送了过多输出')
        }
        break
      }
      case OP_ZMODEM_ACK: {
        const pending = pendingBinary[0]
        if (
          !ready ||
          payload.length !== 4 ||
          !pending ||
          new DataView(payload.buffer, payload.byteOffset, 4).getUint32(0) !== pending.size
        ) {
          fail('ZMODEM 写入确认无效')
          return
        }
        pendingBinary.shift()
        clearTimeout(pending.timer)
        pending.resolve()
        break
      }
      case OP_ZMODEM_DETECTED:
        if (!ready || payload.length !== 1) {
          fail('ZMODEM 检测帧无效')
          return
        }
        handlers.onZmodemDetected?.(payload[0])
        break
      case OP_ZMODEM_OUTPUT: {
        if (payload.length === 0 || payload.length > MAX_DATA_SIZE) {
          fail('ZMODEM 输出帧无效')
          return
        }

        if (!queuePendingOutput(payload.slice())) {
          fail('终端在就绪前发送了过多输出')
        }
        break
      }
      case OP_ERROR:
        if (payload.length < 1 || payload.length > 1 + MAX_REASON_SIZE) {
          fail('终端错误帧无效')
          return
        }
        fail(decoder.decode(payload.subarray(1)) || '终端数据通道发生错误')
        break
      case OP_CLOSE:
        if (payload.length > MAX_REASON_SIZE) {
          fail('终端关闭帧无效')
          return
        }
        closing = true
        rejectBinary()
        handlers.onClose()
        socket.close()
        break
      default:
        fail('终端返回了未知数据帧')
    }
  }
  socket.onerror = () => fail('终端数据通道连接失败')
  socket.onclose = () => {
    if (!closing) {
      closing = true
      rejectBinary()
      handlers.onClose()
    }
  }

  return {
    sendInput(data: string) {
      if (!ready || closing) return false
      const value = encoder.encode(data)
      try {
        for (let offset = 0; offset < value.length; offset += MAX_DATA_SIZE) {
          const chunk = value.subarray(offset, offset + MAX_DATA_SIZE)
          socket.send(createDataFrame(OP_INPUT, chunk))
        }
        return true
      } catch {
        fail('无法发送终端输入')
        return false
      }
    },

    sendBinary(data: Uint8Array) {
      if (!ready || closing) return Promise.reject(new Error('终端数据通道未就绪'))
      const writes: Promise<void>[] = []
      for (let offset = 0; offset < data.length; offset += MAX_DATA_SIZE) {
        const chunk = data.subarray(offset, offset + MAX_DATA_SIZE)
        writes.push(
          new Promise<void>((resolve, reject) => {
            pendingBinary.push({
              size: chunk.length,
              resolve,
              reject,
              timer: setTimeout(() => fail('ZMODEM 写入确认超时'), 30000),
            })
            try {
              socket.send(createDataFrame(OP_ZMODEM, chunk))
            } catch {
              fail('无法发送 ZMODEM 数据')
            }
          }),
        )
        if (closing) break
      }
      return Promise.all(writes).then(() => {})
    },
    resize(nextColumns: number, nextRows: number) {
      if (!ready || closing) return
      try {
        socket.send(createResizeFrame(nextColumns, nextRows))
      } catch {
        fail('无法调整远程终端尺寸')
      }
    },
    close() {
      if (closing) return
      closing = true
      rejectBinary()
      try {
        if (socket.readyState === socket.OPEN) socket.send(new Uint8Array([PROTOCOL_VERSION, OP_CLOSE]))
      } catch {}
      socket.close()
    },
  }
}
function createBindFrame(sessionId: string, streamToken: string, columns: number, rows: number): Uint8Array {
  const id = encoder.encode(sessionId)
  const token = encoder.encode(streamToken)
  if (id.length === 0 || id.length > 1024 || token.length === 0 || token.length > 128) {
    throw new Error('invalid terminal binding')
  }
  const frame = new Uint8Array(10 + id.length + token.length)
  const view = new DataView(frame.buffer)
  frame[0] = PROTOCOL_VERSION
  frame[1] = OP_BIND
  view.setUint16(2, requireDimension(columns))
  view.setUint16(4, requireDimension(rows))
  view.setUint16(6, id.length)
  view.setUint16(8, token.length)
  frame.set(id, 10)
  frame.set(token, 10 + id.length)
  return frame
}

function createDataFrame(op: number, payload: Uint8Array): Uint8Array {
  const frame = new Uint8Array(2 + payload.length)
  frame[0] = PROTOCOL_VERSION
  frame[1] = op
  frame.set(payload, 2)
  return frame
}

function createResizeFrame(columns: number, rows: number): Uint8Array {
  const frame = new Uint8Array(6)
  const view = new DataView(frame.buffer)
  frame[0] = PROTOCOL_VERSION
  frame[1] = OP_RESIZE
  view.setUint16(2, requireDimension(columns))
  view.setUint16(4, requireDimension(rows))
  return frame
}

function createUint32Frame(op: number, value: number): Uint8Array {
  if (!Number.isSafeInteger(value) || value < 1 || value > 0xffffffff) throw new Error('invalid frame value')
  const frame = new Uint8Array(6)
  frame[0] = PROTOCOL_VERSION
  frame[1] = op
  new DataView(frame.buffer).setUint32(2, value)
  return frame
}

function requireDimension(value: number): number {
  if (!Number.isInteger(value) || value < 1 || value > 1000) throw new Error('invalid terminal dimension')
  return value
}
