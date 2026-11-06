export const KASM_MSG_BINARY_CLIPBOARD = 180

const MAX_BINARY_CLIPBOARD_BYTES = 64 * 1024 * 1024

export type KasmBinaryClipboardParse =
  { kind: 'ok'; total: number; text: string } | { kind: 'incomplete' } | { kind: 'unknown' }

function readUint32(bytes: Uint8Array, offset: number): number {
  return ((bytes[offset] << 24) | (bytes[offset + 1] << 16) | (bytes[offset + 2] << 8) | bytes[offset + 3]) >>> 0
}

function decodeMime(bytes: Uint8Array, offset: number, length: number): string {
  let out = ''
  for (let i = 0; i < length; i++) out += String.fromCharCode(bytes[offset + i])
  return out
}

function isTextMime(mime: string): boolean {
  const lower = mime.trim().toLowerCase()
  return (
    lower.startsWith('text/') || lower === 'utf8_string' || lower === 'string' || lower === 'text' || lower === 'utf-8'
  )
}

export function parseKasmBinaryClipboard(bytes: Uint8Array): KasmBinaryClipboardParse {
  if (bytes.byteLength < 1 || bytes[0] !== KASM_MSG_BINARY_CLIPBOARD) return { kind: 'unknown' }
  if (bytes.byteLength < 2) return { kind: 'incomplete' }
  const count = bytes[1]
  let offset = 2
  let text = ''
  for (let index = 0; index < count; index++) {
    if (offset + 5 > bytes.byteLength) return { kind: 'incomplete' }
    offset += 4
    const mimeLength = bytes[offset]
    offset += 1
    if (offset + mimeLength + 4 > bytes.byteLength) return { kind: 'incomplete' }
    const mime = decodeMime(bytes, offset, mimeLength)
    offset += mimeLength
    const dataLength = readUint32(bytes, offset)
    offset += 4
    if (dataLength > MAX_BINARY_CLIPBOARD_BYTES) return { kind: 'unknown' }
    if (offset + dataLength > bytes.byteLength) return { kind: 'incomplete' }

    if (!text && dataLength > 0 && isTextMime(mime)) {
      text = new TextDecoder('utf-8', { fatal: false }).decode(bytes.subarray(offset, offset + dataLength))
    }
    offset += dataLength
  }
  return { kind: 'ok', total: offset, text }
}

interface RfbReceiveQueue {
  rQlen(): number
  rQpeekBytes(len: number, copy?: boolean): Uint8Array
  rQskipBytes(len: number): void
}

interface RfbInternals {
  _normalMsg?: (this: RfbInternals) => boolean

  _FBU?: { rects?: unknown }

  _sock?: RfbReceiveQueue
  _viewOnly?: boolean
  dispatchEvent?: (event: Event) => boolean
}

const NORMAL_MESSAGE_METHOD_CANDIDATES = ['_normalMsg', '_handleNormalMessage'] as const

const RECEIVE_QUEUE_PATH_CANDIDATES = ['_sock', '_sock._recvQ', '_recvQ'] as const

function pickMethod(source: object | undefined, names: readonly string[]): ((this: RfbInternals) => boolean) | null {
  if (!source) return null
  for (const name of names) {
    const value = (source as Record<string, unknown>)[name]
    if (typeof value === 'function') return value as (this: RfbInternals) => boolean
  }
  return null
}

function looksLikeReceiveQueue(value: unknown): value is RfbReceiveQueue {
  if (!value || typeof value !== 'object') return false
  const queue = value as Record<string, unknown>
  return (
    typeof queue.rQlen === 'function' &&
    typeof queue.rQpeekBytes === 'function' &&
    typeof queue.rQskipBytes === 'function'
  )
}

function pickReceiveQueue(rfb: RfbInternals): RfbReceiveQueue | null {
  for (const path of RECEIVE_QUEUE_PATH_CANDIDATES) {
    let current: unknown = rfb
    for (const segment of path.split('.')) {
      if (!current || typeof current !== 'object') {
        current = undefined
        break
      }
      current = (current as Record<string, unknown>)[segment]
    }
    if (looksLikeReceiveQueue(current)) {
      detectedQueuePaths.set(rfb as unknown as object, path)
      return current
    }
  }
  return null
}

const detectedQueuePaths = new WeakMap<object, string>()

function pickFramebufferRects(rfb: RfbInternals): number | null {
  const framebuffer = rfb._FBU
  if (!framebuffer || typeof framebuffer !== 'object') return null
  const rects = (framebuffer as { rects?: unknown }).rects
  return typeof rects === 'number' && Number.isFinite(rects) ? rects : null
}

const kasmInstances = new WeakSet<object>()

const patchedPrototypes = new WeakMap<object, RfbInternals>()

const installFailures = new WeakMap<object, KasmServerMessageInstallReason>()

function consumeKasmServerMessage(rfb: RfbInternals): boolean | null {
  const rects = pickFramebufferRects(rfb)
  if (rects === null) return null
  const sock = pickReceiveQueue(rfb)
  if (!sock) return null

  if (rects > 0) return null
  const available = sock.rQlen()
  if (available < 1) return null
  const head = sock.rQpeekBytes(available, false)
  if (head[0] !== KASM_MSG_BINARY_CLIPBOARD) return null
  if (available < 2) return false
  const parsed = parseKasmBinaryClipboard(head)
  if (parsed.kind === 'incomplete') return false
  if (parsed.kind === 'unknown') return null
  sock.rQskipBytes(parsed.total)
  if (parsed.text && rfb._viewOnly !== true && typeof rfb.dispatchEvent === 'function') {
    try {
      rfb.dispatchEvent(new CustomEvent('clipboard', { detail: { text: parsed.text } }))
    } catch {}
  }
  return true
}

export type KasmServerMessageInstallReason = 'installed' | 'method-missing' | 'self-check-failed' | 'unknown'

export interface KasmServerMessageInstallResult {
  installed: boolean

  reason: KasmServerMessageInstallReason

  selfCheck: KasmServerMessageSelfCheck
}

export interface KasmServerMessageSelfCheck {
  ok: boolean

  total: number

  queuePath: string

  detail: string
}

const SELF_CHECK_TEXT = 'QTERM_SELF_CHECK'

export function buildSelfCheckBinaryClipboard(text: string = SELF_CHECK_TEXT): Uint8Array {
  const mime = new TextEncoder().encode('text/plain')
  const data = new TextEncoder().encode(text)
  const frame = new Uint8Array(2 + 4 + 1 + mime.byteLength + 4 + data.byteLength)
  let offset = 0
  frame[offset++] = KASM_MSG_BINARY_CLIPBOARD
  frame[offset++] = 1

  frame[offset++] = 0
  frame[offset++] = 0
  frame[offset++] = 0
  frame[offset++] = 2
  frame[offset++] = mime.byteLength
  frame.set(mime, offset)
  offset += mime.byteLength
  frame[offset++] = (data.byteLength >>> 24) & 0xff
  frame[offset++] = (data.byteLength >>> 16) & 0xff
  frame[offset++] = (data.byteLength >>> 8) & 0xff
  frame[offset++] = data.byteLength & 0xff
  frame.set(data, offset)
  return frame
}

interface SelfCheckInstance extends RfbInternals, RfbReceiveQueue {
  events: string[]
  data: Uint8Array
  cursor: number
}

function makeSelfCheckInstance(bytes: Uint8Array): SelfCheckInstance {
  const instance = {
    data: bytes,
    cursor: 0,
    events: [] as string[],
    _FBU: { rects: 0 },
    _viewOnly: false,
    dispatchEvent: (event: Event) => {
      const detail = (event as CustomEvent<{ text?: string }>).detail
      instance.events.push(detail?.text ?? '')
      return true
    },
    rQlen(): number {
      return instance.data.byteLength - instance.cursor
    },
    rQpeekBytes(length: number, copy = true): Uint8Array {
      const slice = instance.data.subarray(instance.cursor, instance.cursor + length)
      return copy ? slice.slice() : slice
    },
    rQskipBytes(length: number): void {
      instance.cursor += length
    },
  }

  ;(instance as unknown as RfbInternals)._sock = instance as unknown as RfbReceiveQueue
  return instance as unknown as SelfCheckInstance
}

export function selfCheckKasmServerMessageCompat(prototype: object): KasmServerMessageSelfCheck {
  const frame = buildSelfCheckBinaryClipboard()
  if (!pickMethod(prototype as RfbInternals, NORMAL_MESSAGE_METHOD_CANDIDATES)) {
    return {
      ok: false,
      total: frame.byteLength,
      queuePath: '',
      detail: `分发点方法不存在（试过 ${NORMAL_MESSAGE_METHOD_CANDIDATES.join(' / ')}）`,
    }
  }

  const prototypeInternals = prototype as RfbInternals
  const prototypeQueue = pickReceiveQueue(prototypeInternals)
  const socketCandidate = prototypeInternals._sock
  if (!prototypeQueue && socketCandidate !== undefined && socketCandidate !== null) {
    return {
      ok: false,
      total: frame.byteLength,
      queuePath: '',
      detail:
        '接收队列形状探测失败：`_sock` 存在但不是接收队列（noVNC 可能改造了队列的形状/路径，' +
        '候选：_sock / _sock._recvQ / _recvQ）',
    }
  }

  const dispatchPoint = pickMethod(prototype as RfbInternals, NORMAL_MESSAGE_METHOD_CANDIDATES)
  const fresh = makeSelfCheckInstance(frame)
  markKasmServerMessageInstance(fresh)
  const before = fresh.rQlen()
  let handled: boolean | null
  try {
    handled = dispatchPoint?.call(fresh as unknown as RfbInternals) ?? null
  } catch (error) {
    return {
      ok: false,
      total: frame.byteLength,
      queuePath: detectedQueuePaths.get(fresh as unknown as object) ?? '',
      detail: `分发点抛错：${error instanceof Error ? error.message : String(error)}`,
    }
  }
  const remaining = fresh.rQlen()
  const queuePath = detectedQueuePaths.get(fresh as unknown as object) ?? ''
  if (handled !== true) {
    return { ok: false, total: frame.byteLength, queuePath, detail: `补丁未接管（返回 ${String(handled)}）` }
  }
  if (remaining !== 0 || before !== frame.byteLength) {
    return {
      ok: false,
      total: frame.byteLength,
      queuePath,
      detail: `合成帧未被整条消费（剩余 ${remaining}/${before} 字节）`,
    }
  }
  if (fresh.events.length !== 1 || fresh.events[0] !== SELF_CHECK_TEXT) {
    return {
      ok: false,
      total: frame.byteLength,
      queuePath,
      detail: `clipboard 事件未按预期派发（${JSON.stringify(fresh.events)}）`,
    }
  }
  return { ok: true, total: frame.byteLength, queuePath, detail: '' }
}

export function installKasmServerMessageCompat(
  RfbConstructor: { prototype: object },
  onFailure?: (result: KasmServerMessageInstallResult) => void,
): KasmServerMessageInstallResult {
  const prototype = RfbConstructor.prototype
  const alreadyPatched = patchedPrototypes.get(prototype)
  if (alreadyPatched) {
    const reason = installFailures.get(prototype)
    if (reason && reason !== 'installed') {
      const result: KasmServerMessageInstallResult = {
        installed: false,
        reason,
        selfCheck: { ok: false, total: 0, queuePath: '', detail: '该 noVNC 原型此前已判定为补丁不可用' },
      }
      notifyFailure(onFailure, result)
      return result
    }
    return {
      installed: true,
      reason: 'installed',
      selfCheck: {
        ok: true,
        total: buildSelfCheckBinaryClipboard().byteLength,
        queuePath: '',
        detail: '（本次会话复用此前自检通过的补丁）',
      },
    }
  }
  const original = pickMethod(prototype as RfbInternals, NORMAL_MESSAGE_METHOD_CANDIDATES)
  if (!original) {
    patchedPrototypes.set(prototype, prototype as RfbInternals)
    installFailures.set(prototype, 'method-missing')
    const result: KasmServerMessageInstallResult = {
      installed: false,
      reason: 'method-missing',
      selfCheck: {
        ok: false,
        total: 0,
        queuePath: '',
        detail: `noVNC 的 RFB 原型上没有找到消息分发点（试过：${NORMAL_MESSAGE_METHOD_CANDIDATES.join(' / ')}）`,
      },
    }
    notifyFailure(onFailure, result)
    return result
  }
  ;(prototype as RfbInternals)._normalMsg = function patchedNormalMsg(this: RfbInternals): boolean {
    if (kasmInstances.has(this as unknown as object)) {
      try {
        const consumed = consumeKasmServerMessage(this)
        if (consumed !== null) return consumed
      } catch {}
    }
    return original.call(this)
  }
  patchedPrototypes.set(prototype, prototype as RfbInternals)

  const selfCheck = selfCheckKasmServerMessageCompat(prototype)
  if (!selfCheck.ok) {
    installFailures.set(prototype, 'self-check-failed')
    const result: KasmServerMessageInstallResult = { installed: false, reason: 'self-check-failed', selfCheck }
    notifyFailure(onFailure, result)
    return result
  }
  installFailures.set(prototype, 'installed')
  return { installed: true, reason: 'installed', selfCheck }
}

function notifyFailure(
  onFailure: ((result: KasmServerMessageInstallResult) => void) | undefined,
  result: KasmServerMessageInstallResult,
): void {
  if (!onFailure) return
  try {
    onFailure(result)
  } catch {}
}

export function markKasmServerMessageInstance(instance: object): void {
  kasmInstances.add(instance)
}

const KASM_PASTE_TEXT_MIME = 'text/plain'

const KASM_PASTE_MAX_BYTES = 16 * 1024 * 1024

export function encodeKasmBinaryClipboard(text: string, mime: string = KASM_PASTE_TEXT_MIME): Uint8Array {
  const data = new TextEncoder().encode(text)
  if (data.byteLength > KASM_PASTE_MAX_BYTES) {
    throw new Error(`clipboard too large: ${data.byteLength}`)
  }
  const mimeBytes = new Uint8Array(mime.length)
  for (let i = 0; i < mime.length; i++) mimeBytes[i] = mime.charCodeAt(i) & 0xff
  if (mimeBytes.byteLength > 255) throw new Error('clipboard mime too long')
  const frame = new Uint8Array(3 + mimeBytes.byteLength + 4 + data.byteLength)
  frame[0] = KASM_MSG_BINARY_CLIPBOARD
  frame[1] = 1
  frame[2] = mimeBytes.byteLength
  frame.set(mimeBytes, 3)
  const lengthOffset = 3 + mimeBytes.byteLength
  frame[lengthOffset] = (data.byteLength >>> 24) & 0xff
  frame[lengthOffset + 1] = (data.byteLength >>> 16) & 0xff
  frame[lengthOffset + 2] = (data.byteLength >>> 8) & 0xff
  frame[lengthOffset + 3] = data.byteLength & 0xff
  frame.set(data, lengthOffset + 4)
  return frame
}

interface KasmPasteTarget {
  clipboardPasteFrom?(text: string): void
  _rfbConnectionState?: string
}

export function installKasmClipboardPaste(target: KasmPasteTarget, compat: { sendRaw(data: Uint8Array): void }): void {
  target.clipboardPasteFrom = (text: string): void => {
    if (typeof text !== 'string' || text.length === 0) return
    const state = target._rfbConnectionState
    if (typeof state === 'string' && state !== 'connected') return
    compat.sendRaw(encodeKasmBinaryClipboard(text))
  }
}
