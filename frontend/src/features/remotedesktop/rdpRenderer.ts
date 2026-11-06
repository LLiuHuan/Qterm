import { RdpFormat, RdpProtocolError, type RdpRectPayload, type StreamFrame } from './rdpProtocol'

export type RdpUpdateFrame = Extract<StreamFrame, { kind: 'update' }>

export interface RdpRenderStats {
  keyFrame: boolean

  rects: number

  bytes: number

  renderMs: number

  sequence: bigint
}

export class RdpRenderError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'RdpRenderError'
  }
}

export interface RdpFrameRendererOptions {
  createCanvas?: () => HTMLCanvasElement

  now?: () => number
}

interface CopyRect {
  x: number
  y: number
  width: number
  height: number
}

const MIN_CANVAS_SIZE = 1

export class RdpFrameRenderer {
  private readonly createCanvas: () => HTMLCanvasElement
  private readonly now: () => number

  private target: HTMLCanvasElement | null = null
  private targetContext: CanvasRenderingContext2D | null = null
  private layerCanvas: HTMLCanvasElement | null = null
  private layerContext: CanvasRenderingContext2D | null = null

  private width = 0
  private height = 0

  private hasContent = false
  private lastSequence = 0n

  constructor(options: RdpFrameRendererOptions = {}) {
    this.createCanvas =
      options.createCanvas ??
      (() => {
        if (typeof document === 'undefined') {
          throw new RdpRenderError('当前环境没有 document，无法创建画布')
        }
        return document.createElement('canvas')
      })
    this.now = options.now ?? (() => (typeof performance === 'undefined' ? Date.now() : performance.now()))
  }

  attach(target: HTMLCanvasElement): void {
    this.target = target
    this.targetContext = target.getContext('2d')
    if (!this.targetContext) {
      throw new RdpRenderError('无法获取可见画布的 2D 上下文')
    }
    if (this.width > 0 && this.height > 0) {
      this.resizeCanvas(target, this.width, this.height)
    }
  }

  detach(): void {
    this.target = null
    this.targetContext = null
  }

  size(): { width: number; height: number } {
    return { width: this.width, height: this.height }
  }

  sequence(): bigint {
    return this.lastSequence
  }

  needsResize(width: number, height: number): boolean {
    return width !== this.width || height !== this.height
  }

  clear(): void {
    this.hasContent = false
    this.lastSequence = 0n
    this.layerContext?.clearRect(0, 0, Math.max(1, this.width), Math.max(1, this.height))
    this.targetContext?.clearRect(0, 0, Math.max(1, this.width), Math.max(1, this.height))
  }

  layer(): HTMLCanvasElement | null {
    return this.layerCanvas
  }

  async apply(frame: RdpUpdateFrame): Promise<RdpRenderStats> {
    const startedAt = this.now()
    const rects = frame.rects
    let payloadBytes = 0
    for (const rect of rects) {
      payloadBytes += rect.payload.byteLength
    }

    this.ensureLayer(frame.width, frame.height)

    if (frame.keyFrame) {
      this.layerContext?.clearRect(0, 0, this.width, this.height)
    }

    const copies: CopyRect[] = []
    for (const rect of rects) {
      const copy = this.drawRect(rect)
      if (copy) {
        copies.push(copy)
      }
    }

    await this.flushPending()

    this.copyToTarget(copies, frame.keyFrame)
    this.hasContent = true
    this.lastSequence = frame.sequence

    return {
      keyFrame: frame.keyFrame,
      rects: rects.length,
      bytes: payloadBytes,
      renderMs: Math.max(0, this.now() - startedAt),
      sequence: frame.sequence,
    }
  }

  private pending: Array<{ rect: RdpRectPayload; copy: CopyRect }> = []

  private drawRect(rect: RdpRectPayload): CopyRect | null {
    const context = this.layerContext
    if (!context) {
      throw new RdpRenderError('底图层上下文不可用')
    }
    const copy = this.clip(rect.x, rect.y, rect.width, rect.height)
    if (!copy) {
      return null
    }
    if (rect.format === RdpFormat.RGBA) {
      const expected = rect.width * rect.height * 4
      if (rect.payload.byteLength !== expected) {
        throw new RdpRenderError(
          `RGBA 负载长度不符: ${rect.width}×${rect.height} 需要 ${expected} 字节，实际 ${rect.payload.byteLength}`,
        )
      }

      const pixels = new Uint8ClampedArray(rect.payload.byteLength)
      pixels.set(rect.payload)
      context.putImageData(new ImageData(pixels, rect.width, rect.height), rect.x, rect.y)
      return copy
    }
    if (rect.format === RdpFormat.PNG) {
      this.pending.push({ rect, copy })
      return copy
    }
    throw new RdpRenderError(`未知的矩形格式: ${rect.format}`)
  }

  private async flushPending(): Promise<void> {
    if (this.pending.length === 0) {
      return
    }
    const pending = this.pending
    this.pending = []
    for (const item of pending) {
      let bitmap: ImageBitmap
      try {
        const blob = new Blob([item.rect.payload as BlobPart], { type: 'image/png' })
        bitmap = await createImageBitmap(blob)
      } catch (error) {
        throw new RdpRenderError(`PNG 解码失败: ${error instanceof Error ? error.message : String(error)}`)
      }
      try {
        this.layerContext?.drawImage(bitmap, item.rect.x, item.rect.y)
      } finally {
        bitmap.close()
      }
    }
  }

  private ensureLayer(width: number, height: number): void {
    const safeWidth = Math.max(MIN_CANVAS_SIZE, Math.floor(width))
    const safeHeight = Math.max(MIN_CANVAS_SIZE, Math.floor(height))
    if (this.layerCanvas && this.width === safeWidth && this.height === safeHeight) {
      if (this.target && (this.target.width !== safeWidth || this.target.height !== safeHeight)) {
        this.resizeCanvas(this.target, safeWidth, safeHeight)

        this.hasContent = false
      }
      return
    }
    this.width = safeWidth
    this.height = safeHeight
    const layer = this.createCanvas()
    this.resizeCanvas(layer, safeWidth, safeHeight)
    this.layerCanvas = layer
    this.layerContext = layer.getContext('2d')
    if (!this.layerContext) {
      throw new RdpRenderError('无法获取底图层的 2D 上下文')
    }
    if (this.target) {
      this.resizeCanvas(this.target, safeWidth, safeHeight)
    }
    this.hasContent = false
  }

  private resizeCanvas(canvas: HTMLCanvasElement, width: number, height: number): void {
    canvas.width = width
    canvas.height = height
  }

  private clip(x: number, y: number, width: number, height: number): CopyRect | null {
    if (width <= 0 || height <= 0) {
      return null
    }
    const left = Math.max(0, x)
    const top = Math.max(0, y)
    const right = Math.min(this.width, x + width)
    const bottom = Math.min(this.height, y + height)
    if (right <= left || bottom <= top) {
      return null
    }
    return { x: left, y: top, width: right - left, height: bottom - top }
  }

  private copyToTarget(copies: CopyRect[], keyFrame: boolean): void {
    const context = this.targetContext
    if (!context || copies.length === 0) {
      return
    }
    if (!this.hasContent || keyFrame) {
      context.clearRect(0, 0, this.width, this.height)
      context.drawImage(this.layerCanvas as HTMLCanvasElement, 0, 0)
      return
    }
    const batches = this.mergeCopies(copies)
    for (const batch of batches) {
      context.drawImage(
        this.layerCanvas as HTMLCanvasElement,
        batch.x,
        batch.y,
        batch.width,
        batch.height,
        batch.x,
        batch.y,
        batch.width,
        batch.height,
      )
    }
  }

  private mergeCopies(copies: CopyRect[]): CopyRect[] {
    if (copies.length <= 1) {
      return copies
    }
    const union = copies.reduce(
      (accumulator, item) => ({
        x: Math.min(accumulator.x, item.x),
        y: Math.min(accumulator.y, item.y),
        right: Math.max(accumulator.right, item.x + item.width),
        bottom: Math.max(accumulator.bottom, item.y + item.height),
      }),
      { x: copies[0].x, y: copies[0].y, right: copies[0].x + copies[0].width, bottom: copies[0].y + copies[0].height },
    )
    const unionArea = (union.right - union.x) * (union.bottom - union.y)
    const sumArea = copies.reduce((total, item) => total + item.width * item.height, 0)
    if (unionArea <= sumArea * 1.5) {
      return [{ x: union.x, y: union.y, width: union.right - union.x, height: union.bottom - union.y }]
    }
    return copies
  }
}

export function isRdpRenderFailure(error: unknown): boolean {
  return error instanceof RdpRenderError || error instanceof RdpProtocolError
}
