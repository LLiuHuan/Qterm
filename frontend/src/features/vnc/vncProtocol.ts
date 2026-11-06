export const VNC_PROTOCOL_VERSION = 1

export const VNC_OP_BIND = 0x01
export const VNC_OP_READY = 0x81
export const VNC_OP_ERROR = 0x83

export const VncErrorCode = {
  protocol: 1,
  binding: 2,
  session: 3,
  transport: 4,
} as const

export type VncErrorCodeValue = (typeof VncErrorCode)[keyof typeof VncErrorCode]

export class VncStreamError extends Error {
  readonly code: VncErrorCodeValue | 0

  constructor(code: VncErrorCodeValue | 0, message: string) {
    super(message)
    this.name = 'VncStreamError'
    this.code = code
  }
}

const bufferedAfterReady = new WeakMap<object, ArrayBuffer[]>()

export function takeBufferedVncBytes(socket: VncBinarySocket): ArrayBuffer[] {
  const list = bufferedAfterReady.get(socket as unknown as object) ?? []
  bufferedAfterReady.delete(socket as unknown as object)
  return list
}

function pushBuffered(socket: VncBinarySocket, payload: ArrayBuffer): void {
  const list = bufferedAfterReady.get(socket as unknown as object) ?? []
  list.push(payload)
  bufferedAfterReady.set(socket as unknown as object, list)
}

function copyBuffer(bytes: Uint8Array): ArrayBuffer {
  const copy = new Uint8Array(bytes.byteLength)
  copy.set(bytes)
  return copy.buffer
}

export interface VncBinarySocket {
  readyState: number
  binaryType: string
  onopen: ((event: Event) => void) | null
  onmessage: ((event: MessageEvent) => void) | null
  onclose: ((event: CloseEvent) => void) | null
  onerror: ((event: Event) => void) | null
  send(data: Uint8Array): void
  close(code?: number, reason?: string): void
}

export interface VncControlFrame {
  op: number
  code?: VncErrorCodeValue
  reason?: string
}

export function toBytes(payload: ArrayBuffer | ArrayBufferView): Uint8Array {
  if (payload instanceof ArrayBuffer) return new Uint8Array(payload)
  return new Uint8Array(payload.buffer, payload.byteOffset, payload.byteLength)
}

export function encodeVncBindFrame(sessionId: string, token: string): Uint8Array {
  const encoder = new TextEncoder()
  const idBytes = encoder.encode(sessionId)
  const tokenBytes = encoder.encode(token)
  const frame = new Uint8Array(6 + idBytes.length + tokenBytes.length)
  const view = new DataView(frame.buffer)
  frame[0] = VNC_PROTOCOL_VERSION
  frame[1] = VNC_OP_BIND
  view.setUint16(2, idBytes.length, true)
  view.setUint16(4, tokenBytes.length, true)
  frame.set(idBytes, 6)
  frame.set(tokenBytes, 6 + idBytes.length)
  return frame
}

export function parseVncControlFrame(payload: ArrayBuffer | ArrayBufferView): VncControlFrame {
  const bytes = toBytes(payload)
  if (bytes.length < 2) {
    throw new VncStreamError(VncErrorCode.protocol, '控制帧过短')
  }
  if (bytes[0] !== VNC_PROTOCOL_VERSION) {
    throw new VncStreamError(VncErrorCode.protocol, `协议版本不匹配：${bytes[0]}`)
  }
  const op = bytes[1]
  if (op === VNC_OP_READY) {
    if (bytes.length < 8) {
      throw new VncStreamError(VncErrorCode.protocol, 'READY 帧长度不足')
    }
    return { op }
  }
  if (op === VNC_OP_ERROR) {
    if (bytes.length < 3) {
      throw new VncStreamError(VncErrorCode.protocol, 'ERROR 帧长度不足')
    }
    const code = bytes[2] as VncErrorCodeValue
    const reason = bytes.length > 3 ? new TextDecoder().decode(bytes.subarray(3)) : ''
    return { op, code, reason }
  }
  throw new VncStreamError(VncErrorCode.protocol, `未知控制帧：0x${op.toString(16)}`)
}

function waitForOpen(socket: VncBinarySocket, timeoutMs: number): Promise<void> {
  const OPEN = 1
  return new Promise((resolve, reject) => {
    if (socket.readyState === OPEN) {
      resolve()
      return
    }
    let settled = false
    const timer = setTimeout(() => {
      if (settled) return
      settled = true
      reject(new VncStreamError(VncErrorCode.transport, '建立字节通道超时'))
    }, timeoutMs)
    const finish = (error: Error | null) => {
      if (settled) return
      settled = true
      clearTimeout(timer)
      if (error) reject(error)
      else resolve()
    }
    const previousOpen = socket.onopen
    const previousError = socket.onerror
    const previousClose = socket.onclose
    socket.onopen = (event) => {
      previousOpen?.call(socket, event)
      finish(null)
    }
    socket.onerror = (event) => {
      previousError?.call(socket, event)
      finish(new VncStreamError(VncErrorCode.transport, '字节通道建立失败'))
    }
    socket.onclose = (event) => {
      previousClose?.call(socket, event)
      finish(new VncStreamError(VncErrorCode.transport, '字节通道被关闭'))
    }
  })
}

export interface BindVncChannelOptions {
  openTimeoutMs?: number

  replyTimeoutMs?: number
}

export async function bindVncChannel(
  socket: VncBinarySocket,
  sessionId: string,
  token: string,
  options: BindVncChannelOptions = {},
): Promise<void> {
  const openTimeoutMs = options.openTimeoutMs ?? 10_000
  const replyTimeoutMs = options.replyTimeoutMs ?? 10_000
  await waitForOpen(socket, openTimeoutMs)

  const reply = new Promise<VncControlFrame>((resolve, reject) => {
    let settled = false
    const timer = setTimeout(() => {
      if (settled) return
      settled = true
      reject(new VncStreamError(VncErrorCode.transport, '等待绑定确认超时'))
    }, replyTimeoutMs)
    socket.onmessage = (event: MessageEvent) => {
      if (settled) {
        pushBuffered(socket, copyBuffer(toBytes(event.data as ArrayBuffer | ArrayBufferView)))
        return
      }
      settled = true
      clearTimeout(timer)
      const bytes = toBytes(event.data as ArrayBuffer | ArrayBufferView)
      try {
        const frame = parseVncControlFrame(bytes)

        if (frame.op === VNC_OP_READY && bytes.byteLength > 8) {
          pushBuffered(socket, copyBuffer(bytes.subarray(8)))
        }
        resolve(frame)
      } catch (error) {
        reject(error as Error)
      }
    }
    socket.onclose = () => {
      if (settled) return
      settled = true
      clearTimeout(timer)
      reject(new VncStreamError(VncErrorCode.transport, '绑定过程中通道被关闭'))
    }
    socket.send(encodeVncBindFrame(sessionId, token))
  })

  const frame = await reply
  if (frame.op === VNC_OP_ERROR) {
    throw new VncStreamError(frame.code ?? 0, frame.reason ?? '绑定失败')
  }
}
