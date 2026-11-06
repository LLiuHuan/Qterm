import { toBytes, type VncBinarySocket } from './vncProtocol'

export const KASM_POINTER_EVENT_BYTES = 11

const STANDARD_POINTER_EVENT_BYTES = 6
const EXTENDED_POINTER_EVENT_BYTES = 7

const handles = new WeakMap<object, KasmUpstreamCompatHandle>()

export function encodeKasmPointerEvent(pointerEvent: Uint8Array): Uint8Array {
  const length = pointerEvent.byteLength
  if (length !== STANDARD_POINTER_EVENT_BYTES && length !== EXTENDED_POINTER_EVENT_BYTES) {
    throw new Error(`不是标准 PointerEvent 长度：${length}`)
  }
  const lowBits = pointerEvent[1]
  const extended = length === EXTENDED_POINTER_EVENT_BYTES
  const markerSet = (lowBits & 0x80) !== 0
  if (extended !== markerSet) {
    throw new Error('PointerEvent 的 marker 位与长度不一致')
  }
  const mask = extended ? ((pointerEvent[6] << 7) | (lowBits & 0x7f)) & 0xffff : lowBits & 0x7f
  const x = (pointerEvent[2] << 8) | pointerEvent[3]
  const y = (pointerEvent[4] << 8) | pointerEvent[5]

  const out = new Uint8Array(KASM_POINTER_EVENT_BYTES)
  out[0] = 5
  out[1] = (mask >> 8) & 0xff
  out[2] = mask & 0xff
  out[3] = (x >> 8) & 0xff
  out[4] = x & 0xff
  out[5] = (y >> 8) & 0xff
  out[6] = y & 0xff

  out[7] = 0
  out[8] = 0
  out[9] = 0
  out[10] = 0
  return out
}

function readUint16(bytes: Uint8Array, offset: number): number {
  return (bytes[offset] << 8) | bytes[offset + 1]
}

function readUint32(bytes: Uint8Array, offset: number): number {
  return ((bytes[offset] << 24) | (bytes[offset + 1] << 16) | (bytes[offset + 2] << 8) | bytes[offset + 3]) >>> 0
}

function clientMessageLength(bytes: Uint8Array, offset: number): number | null {
  const remaining = bytes.byteLength - offset
  if (remaining < 1) return null
  switch (bytes[offset]) {
    case 0:
      return 20
    case 2:
      if (remaining < 4) return null
      return 4 + 4 * readUint16(bytes, offset + 2)
    case 3:
      return 10
    case 4:
      return 8
    case 5:
      if (remaining < 2) return null
      return bytes[offset + 1] & 0x80 ? EXTENDED_POINTER_EVENT_BYTES : STANDARD_POINTER_EVENT_BYTES
    case 6: {
      if (remaining < 8) return null
      return 8 + (readUint32(bytes, offset + 4) & 0x7fffffff)
    }
    case 150:
      return 10
    case 248: {
      if (remaining < 9) return null
      return 9 + bytes[offset + 8]
    }
    case 250:
      return 4
    case 251: {
      if (remaining < 8) return null
      return 8 + 16 * bytes[offset + 6]
    }
    case 255:
      return 12
    default:
      return null
  }
}

export function createKasmUpstreamRewriter(): (bytes: Uint8Array) => Uint8Array {
  let pending = 0
  return (bytes: Uint8Array): Uint8Array => {
    const chunks: Uint8Array[] = []
    let offset = 0
    let rewritten = false
    if (pending > 0) {
      const take = Math.min(pending, bytes.byteLength)
      chunks.push(bytes.subarray(0, take))
      pending -= take
      offset = take
    }
    while (offset < bytes.byteLength) {
      const length = clientMessageLength(bytes, offset)
      if (length === null || length <= 0) {
        chunks.push(bytes.subarray(offset))
        break
      }
      if (offset + length > bytes.byteLength) {
        chunks.push(bytes.subarray(offset))
        pending = offset + length - bytes.byteLength
        break
      }
      const message = bytes.subarray(offset, offset + length)
      if (bytes[offset] === 5) {
        chunks.push(encodeKasmPointerEvent(message))
        rewritten = true
      } else {
        chunks.push(message)
      }
      offset += length
    }
    if (!rewritten) return bytes
    const out = new Uint8Array(chunks.reduce((total, chunk) => total + chunk.byteLength, 0))
    let cursor = 0
    for (const chunk of chunks) {
      out.set(chunk, cursor)
      cursor += chunk.byteLength
    }
    return out
  }
}

export function rewriteKasmUpstreamBytes(bytes: Uint8Array): Uint8Array {
  return createKasmUpstreamRewriter()(bytes)
}

export interface KasmUpstreamCompatHandle {
  enable(): void

  sendRaw(data: Uint8Array): void
}

export function installKasmUpstreamCompat(socket: VncBinarySocket): KasmUpstreamCompatHandle {
  const existing = handles.get(socket as unknown as object)
  if (existing) return existing
  const rewrite = createKasmUpstreamRewriter()
  let enabled = false
  const forward = socket.send.bind(socket)
  const handle: KasmUpstreamCompatHandle = {
    enable(): void {
      enabled = true
    },
    sendRaw(data: Uint8Array): void {
      forward(toBytes(data))
    },
  }
  socket.send = (data: Uint8Array): void => {
    const bytes = toBytes(data)
    forward(enabled ? rewrite(bytes) : bytes)
  }
  handles.set(socket as unknown as object, handle)
  return handle
}

const KASM_WHEEL_LINE_HEIGHT = 19

const KASM_SCROLL_LIMIT = 32767

function clampScroll(value: number): number {
  if (!Number.isFinite(value)) return 0
  const rounded = Math.round(value)
  if (rounded > KASM_SCROLL_LIMIT) return KASM_SCROLL_LIMIT
  if (rounded < -KASM_SCROLL_LIMIT) return -KASM_SCROLL_LIMIT
  return rounded
}

export function encodeKasmScrollEvent(deltaX: number, deltaY: number): Uint8Array | null {
  const scrollX = clampScroll(deltaX)
  const scrollY = clampScroll(deltaY)
  if (scrollX === 0 && scrollY === 0) return null
  const frame = new Uint8Array(KASM_POINTER_EVENT_BYTES)
  frame[0] = 5

  frame[1] = 0
  frame[2] = 0

  frame[3] = 0
  frame[4] = 0
  frame[5] = 0
  frame[6] = 0
  frame[7] = (scrollX >> 8) & 0xff
  frame[8] = scrollX & 0xff
  frame[9] = (scrollY >> 8) & 0xff
  frame[10] = scrollY & 0xff
  return frame
}

interface KasmWheelTarget {
  viewOnly?: boolean

  _rfbConnectionState?: string

  _canvas?: unknown
}

function isOverCanvas(instance: KasmWheelTarget, target: EventTarget | null): boolean {
  const canvas = instance._canvas
  if (typeof Node !== 'undefined' && canvas instanceof Node) {
    return target instanceof Node && canvas.contains(target)
  }
  return typeof HTMLCanvasElement !== 'undefined' && target instanceof HTMLCanvasElement
}

export function installKasmWheelCompat(
  instance: KasmWheelTarget,
  container: HTMLElement,
  compat: Pick<KasmUpstreamCompatHandle, 'sendRaw'>,
): () => void {
  const onWheel = (event: WheelEvent): void => {
    if (instance.viewOnly) return
    const state = instance._rfbConnectionState
    if (typeof state === 'string' && state !== 'connected') return
    if (!isOverCanvas(instance, event.target)) return

    const scale = event.deltaMode === 0 ? 1 : KASM_WHEEL_LINE_HEIGHT
    const frame = encodeKasmScrollEvent(event.deltaX * scale, event.deltaY * scale)
    if (!frame) return

    event.preventDefault()
    event.stopPropagation()
    try {
      compat.sendRaw(frame)
    } catch {}
  }
  container.addEventListener('wheel', onWheel, { capture: true, passive: false })
  return () => container.removeEventListener('wheel', onWheel, { capture: true })
}
