export const RDP_PROTOCOL_VERSION = 1

export const protocolVersion = RDP_PROTOCOL_VERSION

export const RdpOp = {
  Bind: 0x01,

  Ack: 0x02,

  Drop: 0x03,

  Ready: 0x81,

  Update: 0x82,

  Error: 0x83,
} as const
export type RdpOp = (typeof RdpOp)[keyof typeof RdpOp]

export const RdpFormat = {
  RGBA: 1,

  PNG: 2,
} as const
export type RdpFormat = (typeof RdpFormat)[keyof typeof RdpFormat]

export const RdpErrorCode = {
  Protocol: 1,
  Binding: 2,
  Session: 3,
  Transport: 4,
} as const
export type RdpErrorCode = (typeof RdpErrorCode)[keyof typeof RdpErrorCode]

export const RDP_UPDATE_FLAG_KEY_FRAME = 0x01

export const RDP_FRAME_HEADER_BYTES = 2

export const RDP_BIND_HEADER_BYTES = 6

export const RDP_ACK_FRAME_BYTES = 15

export const RDP_DROP_FRAME_BYTES = 11

export const RDP_ERROR_HEADER_BYTES = 3

export const RDP_READY_FRAME_BYTES = 26

export const RDP_READY_FRAME_BYTES_DOCUMENTED = 28

export const RDP_UPDATE_HEADER_BYTES = 32

export const RDP_RECT_HEADER_BYTES = 16

export const RDP_MAX_RECTS_PER_FRAME = 4096

export const RDP_MAX_BIND_ID_BYTES = 1024

export const RDP_MAX_BIND_TOKEN_BYTES = 128

const UINT64_MAX = 0xffffffffffffffffn

const UINT32_MAX = 0xffffffff

export class RdpProtocolError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'RdpProtocolError'
  }
}

export interface RdpRectPayload {
  x: number
  y: number
  width: number
  height: number
  format: RdpFormat

  payload: Uint8Array
}

export type StreamFrame =
  | { kind: 'ready'; width: number; height: number; batchId: bigint }
  | {
      kind: 'update'
      keyFrame: boolean
      batchId: bigint
      sequence: bigint
      width: number
      height: number
      rects: RdpRectPayload[]
    }
  | { kind: 'error'; code: number; reason: string }

const utf8Encoder = new TextEncoder()

const utf8Decoder = new TextDecoder('utf-8')

function toUint64(value: bigint | number, label: string): bigint {
  if (typeof value === 'bigint') {
    if (value < 0n || value > UINT64_MAX) {
      throw new RangeError(`${label} 超出 u64 范围：${value}`)
    }
    return value
  }

  if (!Number.isSafeInteger(value) || value < 0) {
    throw new RangeError(`${label} 必须是 0 ~ 2^53-1 的安全整数或 bigint，实际 ${value}`)
  }
  return BigInt(value)
}

function toUint32(value: number, label: string): number {
  if (!Number.isInteger(value) || value < 0 || value > UINT32_MAX) {
    throw new RangeError(`${label} 必须是 0 ~ ${UINT32_MAX} 的整数，实际 ${value}`)
  }
  return value
}

function toUint8(value: number, label: string): number {
  if (!Number.isInteger(value) || value < 0 || value > 0xff) {
    throw new RangeError(`${label} 必须是 0 ~ 255 的整数，实际 ${value}`)
  }
  return value
}

function hexOp(op: number): string {
  return `0x${op.toString(16).padStart(2, '0')}`
}

export function encodeBindFrame(sessionId: string, token: string): Uint8Array {
  const idBytes = utf8Encoder.encode(sessionId)
  const tokenBytes = utf8Encoder.encode(token)
  if (idBytes.byteLength < 1 || idBytes.byteLength > RDP_MAX_BIND_ID_BYTES) {
    throw new RdpProtocolError(
      `会话 ID 的 UTF-8 长度非法：${idBytes.byteLength} 字节，必须在 1 ~ ${RDP_MAX_BIND_ID_BYTES} 字节之间`,
    )
  }
  if (tokenBytes.byteLength < 1 || tokenBytes.byteLength > RDP_MAX_BIND_TOKEN_BYTES) {
    throw new RdpProtocolError(
      `绑定令牌的 UTF-8 长度非法：${tokenBytes.byteLength} 字节，必须在 1 ~ ${RDP_MAX_BIND_TOKEN_BYTES} 字节之间`,
    )
  }
  const frame = new Uint8Array(RDP_BIND_HEADER_BYTES + idBytes.byteLength + tokenBytes.byteLength)
  const view = new DataView(frame.buffer)
  frame[0] = RDP_PROTOCOL_VERSION
  frame[1] = RdpOp.Bind
  view.setUint16(2, idBytes.byteLength, true)
  view.setUint16(4, tokenBytes.byteLength, true)
  frame.set(idBytes, RDP_BIND_HEADER_BYTES)
  frame.set(tokenBytes, RDP_BIND_HEADER_BYTES + idBytes.byteLength)
  return frame
}

export function encodeAckFrame(batchId: bigint | number, renderMicros: number): Uint8Array {
  const frame = new Uint8Array(RDP_ACK_FRAME_BYTES)
  const view = new DataView(frame.buffer)
  frame[0] = RDP_PROTOCOL_VERSION
  frame[1] = RdpOp.Ack
  view.setBigUint64(2, toUint64(batchId, 'batchId'), true)
  view.setUint32(10, toUint32(renderMicros, 'renderMicros'), true)
  return frame
}

export function encodeDropFrame(batchId: bigint | number, reason: number): Uint8Array {
  const frame = new Uint8Array(RDP_DROP_FRAME_BYTES)
  const view = new DataView(frame.buffer)
  frame[0] = RDP_PROTOCOL_VERSION
  frame[1] = RdpOp.Drop
  view.setBigUint64(2, toUint64(batchId, 'batchId'), true)
  frame[10] = toUint8(reason, 'reason')
  return frame
}

interface FrameSource {
  buffer: ArrayBufferLike
  byteOffset: number
  byteLength: number
}

function resolveFrameSource(data: ArrayBuffer | Uint8Array): FrameSource {
  if (ArrayBuffer.isView(data)) {
    return { buffer: data.buffer, byteOffset: data.byteOffset, byteLength: data.byteLength }
  }
  return { buffer: data, byteOffset: 0, byteLength: data.byteLength }
}

function parseReadyFrame(source: FrameSource, view: DataView): StreamFrame {
  let width: number
  let height: number
  let batchId: bigint
  if (source.byteLength === RDP_READY_FRAME_BYTES) {
    width = view.getUint32(2, true)
    height = view.getUint32(6, true)
    batchId = view.getBigUint64(10, true)
  } else if (source.byteLength === RDP_READY_FRAME_BYTES_DOCUMENTED) {
    width = view.getUint32(4, true)
    height = view.getUint32(8, true)
    batchId = view.getBigUint64(12, true)
  } else {
    throw new RdpProtocolError(
      `READY 帧长度不符：期望 ${RDP_READY_FRAME_BYTES} 字节（Go 当前实现）或 ` +
        `${RDP_READY_FRAME_BYTES_DOCUMENTED} 字节（Go 注释布局），实际 ${source.byteLength} 字节`,
    )
  }
  return { kind: 'ready', width, height, batchId }
}

function parseUpdateFrame(source: FrameSource, view: DataView): StreamFrame {
  if (source.byteLength < RDP_UPDATE_HEADER_BYTES) {
    throw new RdpProtocolError(`UPDATE 帧头截断：需要 ${RDP_UPDATE_HEADER_BYTES} 字节，实际 ${source.byteLength} 字节`)
  }

  const keyFrame = (view.getUint8(2) & RDP_UPDATE_FLAG_KEY_FRAME) !== 0
  const batchId = view.getBigUint64(4, true)
  const sequence = view.getBigUint64(12, true)
  const width = view.getUint32(20, true)
  const height = view.getUint32(24, true)
  const rectCount = view.getUint16(28, true)
  if (rectCount > RDP_MAX_RECTS_PER_FRAME) {
    throw new RdpProtocolError(
      `单帧矩形过多：${rectCount}，上限 ${RDP_MAX_RECTS_PER_FRAME}（超过说明上游脏矩形策略失效，拒绝按对端声明分配内存）`,
    )
  }
  const payloadStart = RDP_UPDATE_HEADER_BYTES + rectCount * RDP_RECT_HEADER_BYTES
  if (payloadStart > source.byteLength) {
    throw new RdpProtocolError(
      `UPDATE 帧截断：${rectCount} 个矩形描述符需要 ${payloadStart} 字节，实际只有 ${source.byteLength} 字节`,
    )
  }

  const rects: RdpRectPayload[] = []
  let cursor = payloadStart
  for (let index = 0; index < rectCount; index += 1) {
    const descriptor = RDP_UPDATE_HEADER_BYTES + index * RDP_RECT_HEADER_BYTES
    const rawFormat = view.getUint8(descriptor + 8)
    if (rawFormat !== RdpFormat.RGBA && rawFormat !== RdpFormat.PNG) {
      throw new RdpProtocolError(
        `矩形 ${index} 的 format 未知：${rawFormat}（只定义了 ${RdpFormat.RGBA}=RGBA8888 / ${RdpFormat.PNG}=PNG）`,
      )
    }
    const payloadLen = view.getUint32(descriptor + 12, true)
    if (cursor + payloadLen > source.byteLength) {
      throw new RdpProtocolError(
        `矩形 ${index} 的 payloadLen 越界：声明 ${payloadLen} 字节，从偏移 ${cursor} 起超出帧长 ${source.byteLength} 字节`,
      )
    }
    rects.push({
      x: view.getUint16(descriptor, true),
      y: view.getUint16(descriptor + 2, true),
      width: view.getUint16(descriptor + 4, true),
      height: view.getUint16(descriptor + 6, true),
      format: rawFormat,
      payload: new Uint8Array(source.buffer, source.byteOffset + cursor, payloadLen),
    })
    cursor += payloadLen
  }
  if (cursor !== source.byteLength) {
    throw new RdpProtocolError(
      `UPDATE 帧尾部有 ${source.byteLength - cursor} 字节多余数据：描述符声明的负载总长与帧长不一致`,
    )
  }
  return { kind: 'update', keyFrame, batchId, sequence, width, height, rects }
}

function parseErrorFrame(source: FrameSource, view: DataView): StreamFrame {
  if (source.byteLength < RDP_ERROR_HEADER_BYTES) {
    throw new RdpProtocolError(
      `ERROR 帧截断：至少需要 ${RDP_ERROR_HEADER_BYTES} 字节（版本|op|code），实际 ${source.byteLength} 字节`,
    )
  }
  return {
    kind: 'error',
    code: view.getUint8(2),

    reason: utf8Decoder.decode(
      new Uint8Array(
        source.buffer,
        source.byteOffset + RDP_ERROR_HEADER_BYTES,
        source.byteLength - RDP_ERROR_HEADER_BYTES,
      ),
    ),
  }
}

export function parseStreamFrame(data: ArrayBuffer | Uint8Array): StreamFrame {
  const source = resolveFrameSource(data)
  if (source.byteLength < RDP_FRAME_HEADER_BYTES) {
    throw new RdpProtocolError(
      `帧长度不足：读取协议版本与 op 至少需要 ${RDP_FRAME_HEADER_BYTES} 字节，实际 ${source.byteLength} 字节`,
    )
  }

  const view = new DataView(source.buffer, source.byteOffset, source.byteLength)
  const version = view.getUint8(0)
  if (version !== RDP_PROTOCOL_VERSION) {
    throw new RdpProtocolError(
      `协议版本不符：收到 ${version}，期望 ${RDP_PROTOCOL_VERSION}（rdpProtocol.ts 与 stream_protocol.go 必须同步升级）`,
    )
  }
  const op = view.getUint8(1)
  switch (op) {
    case RdpOp.Ready:
      return parseReadyFrame(source, view)
    case RdpOp.Update:
      return parseUpdateFrame(source, view)
    case RdpOp.Error:
      return parseErrorFrame(source, view)
    case RdpOp.Bind:
    case RdpOp.Ack:
    case RdpOp.Drop:
      throw new RdpProtocolError(`op 方向错误：${hexOp(op)} 是客户端→服务端的帧，前端不应收到`)
    default:
      throw new RdpProtocolError(
        `未知 op：${hexOp(op)}（服务端→客户端只定义了 ${hexOp(RdpOp.Ready)} READY / ${hexOp(RdpOp.Update)} UPDATE / ${hexOp(RdpOp.Error)} ERROR）`,
      )
  }
}
