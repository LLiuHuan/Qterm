import { SkipTransfer } from './transfer-conflicts'
import { setZmodemUploadPolicy } from './zmodem-policy'
import Zmodem from 'zmodem.js/src/zmodem_browser.js'

export type ZmodemDirection = 'upload' | 'download'
export type ZmodemState = 'idle' | 'detecting' | 'transferring' | 'saving' | 'complete' | 'error' | 'cancelling'
export interface ZmodemUploadSource {
  overwrite?: boolean
  name: string
  size: number
  read(offset: number, length: number): Promise<Uint8Array>
  close(): Promise<void>
}
export interface ZmodemDownloadTarget {
  write(data: Uint8Array): Promise<void>
  finish(): Promise<void>
  close(): Promise<void>
}
export interface ZmodemProgress {
  direction: ZmodemDirection
  fileName: string
  bytesSent: number
  bytesTotal: number
  percent: number
  state: ZmodemState
}
export type ZmodemEventHandler = {
  onUpload?: (signal: AbortSignal) => Promise<ZmodemUploadSource | null>
  onDestination?: (info: { name: string; size: number }, signal: AbortSignal) => Promise<ZmodemDownloadTarget | null>
  onDrainFailure?: () => void
  onDetect?: (direction: ZmodemDirection) => void
  onProgress?: (progress: ZmodemProgress) => void
  onComplete?: (direction: ZmodemDirection, fileName: string) => void
  onError?: (direction: ZmodemDirection, message: string) => void
  onRetract?: () => void
  onOffer?: (fileInfo: { name: string; size: number }) => Promise<boolean>
}
interface Transfer {
  get_offset(): number
  send(bytes: Uint8Array): void
  end(bytes?: Uint8Array): Promise<void>
}
interface Offer {
  get_details(): { name: string; size: number | null }
  accept(options: { on_input: (bytes: number[]) => void }): Promise<void>
  skip(): Promise<unknown>
}
interface Session {
  type: 'send' | 'receive'
  set_sender(sender: (bytes: number[]) => void): void
  on(event: 'session_end', handler: () => void): void
  on(event: 'garbage', handler: (bytes: number[]) => void): void
  on(event: 'offer', handler: (offer: Offer) => void): void
  consume(bytes: number[]): void
  has_ended(): boolean
  aborted(): boolean
  abort(): void
  start(): Promise<unknown>
  close(): Promise<void>
  send_offer(details: { name: string; size: number }): Promise<Transfer | undefined>
  get_trailing_bytes(): number[]
}
const signature = [42, 42, 24, 66, 48]

const startupMarkers = [
  'rz\r',
  'rz\r\n',
  'rz waiting to receive.',
  'rz waiting to receive.\r\n',
  'rz waiting to receive.\r',
].map((text) => Array.from(new TextEncoder().encode(text)))
const detectionPrefixes = [signature, ...startupMarkers.map((marker) => [...marker, ...signature])]

const maxDownloadSize = 256 * 1024 * 1024

export class ZmodemManager {
  private session: Session | null = null
  private activeOffer: Offer | null = null
  private cancelling = false
  private operation = new AbortController()
  private buffer: number[] = []
  private state_: ZmodemState = 'idle'
  private direction_: ZmodemDirection = 'upload'
  private currentFile = ''
  private downloads = new Set<Promise<void>>()
  private uploading = false
  private outbound = Promise.resolve()
  private generation = 0
  private cancelWaiters = new Set<() => void>()
  private timer: ReturnType<typeof setTimeout> | undefined
  private outputTimer: ReturnType<typeof setTimeout> | undefined

  constructor(
    private sender: (data: Uint8Array) => void | Promise<void>,
    private toTerminal: (data: Uint8Array) => void,
    private handlers: ZmodemEventHandler = {},
  ) {}

  consume(data: Uint8Array): void {
    clearTimeout(this.outputTimer)
    const session = this.session
    if (session) {
      try {
        this.touch()
        if (session.type === 'send') {
          let offset = 0
          while (offset < data.length && !session.has_ended()) session.consume([data[offset++]!])
          if (offset < data.length) this.consume(data.slice(offset))
        } else session.consume(Array.from(data))
        if (session.has_ended() && !session.aborted() && session.type === 'receive') {
          this.consume(new Uint8Array(session.get_trailing_bytes()))
        }
      } catch {
        if (this.session === session && !this.cancelling) this.fail('ZMODEM 协议失败，请重试传输')
      }
      return
    }

    this.buffer.push(...data)
    while (this.buffer.length) {
      const index = this.buffer.findIndex((_, i) => signature.every((b, j) => this.buffer[i + j] === b))
      if (index < 0) {
        let keep = 0
        for (const prefix of detectionPrefixes) {
          for (let size = Math.min(prefix.length - 1, this.buffer.length); size > keep; size--) {
            if (prefix.slice(0, size).every((byte, i) => this.buffer[this.buffer.length - size + i] === byte)) {
              keep = size
              break
            }
          }
        }
        this.output(this.buffer.splice(0, this.buffer.length - keep))

        if (keep) this.armOutputFlush()
        return
      }

      if (this.buffer.length - index < 19) {
        this.armOutputFlush()
        return
      }
      const headerLength = [10, 138].includes(this.buffer[index + 18]!) ? 19 : 20
      if (this.buffer.length - index < headerLength) {
        this.armOutputFlush()
        return
      }
      const header = this.buffer.slice(index, index + headerLength)
      let detected: Session | undefined
      try {
        detected = Zmodem.Session.parse(header) as Session | undefined
      } catch {}
      if (!detected) {
        this.output(this.buffer.splice(0, index + 1))
        continue
      }
      const prefix = this.buffer.splice(0, index)
      const marker = startupMarkers.find(
        (candidate) =>
          candidate.length <= prefix.length &&
          candidate.every((byte, i) => prefix[prefix.length - candidate.length + i] === byte),
      )
      if (marker) prefix.splice(prefix.length - marker.length)
      this.output(prefix)
      this.buffer.splice(0, headerLength)
      if (this.buffer[0] === 17) this.buffer.shift()
      const remaining = new Uint8Array(this.buffer)
      this.buffer = []
      try {
        this.begin(detected)
        if (remaining.length) this.consume(remaining)
      } catch {
        this.fail('ZMODEM 握手失败，请重试')
      }
      return
    }
  }

  private output(bytes: number[]): void {
    if (bytes.length) this.toTerminal(new Uint8Array(bytes))
  }

  private touch(): void {
    clearTimeout(this.timer)
    this.timer = setTimeout(
      () => {
        if (this.cancelling) this.handlers.onDrainFailure?.()
        this.fail('ZMODEM 传输超时，请重新连接终端')
      },
      this.cancelling ? 10_000 : 120_000,
    )
  }

  private begin(session: Session): void {
    const generation = ++this.generation
    this.operation = new AbortController()
    this.cancelling = false
    this.activeOffer = null
    this.session = session
    this.currentFile = ''
    this.direction_ = session.type === 'send' ? 'upload' : 'download'
    this.state_ = 'transferring'
    this.touch()
    this.outbound = Promise.resolve()
    session.set_sender((bytes) => {
      this.outbound = Promise.all([this.outbound, this.sender(new Uint8Array(bytes))]).then(() => {})
      void this.outbound.catch(() => {
        if (generation === this.generation) this.fail('ZMODEM 数据发送失败，请重新连接终端')
      })
    })
    session.on('garbage', () => {})
    session.on('session_end', () => {
      if (this.session !== session) return
      if (this.cancelling) {
        this.session = null
        this.activeOffer = null
        this.cancelling = false
        clearTimeout(this.timer)
        this.state_ = 'idle'
        this.handlers.onRetract?.()
        return
      }
      if (session.aborted()) {
        this.fail('远端已取消 ZMODEM 传输')
        return
      }
      this.session = null
      clearTimeout(this.timer)

      void Promise.all(this.downloads).then(() => {
        if (generation !== this.generation) return
        this.state_ = 'complete'
        this.handlers.onComplete?.(this.direction_, this.currentFile)
      })
    })
    if (session.type === 'receive') {
      session.on('offer', (offer) => {
        this.activeOffer = offer
        if (this.cancelling) {
          void offer.skip()
          return
        }
        const download = this.receiveFile(session, offer, generation).catch(() => {
          if (generation === this.generation) this.fail('ZMODEM 下载失败，请重试')
        })
        this.downloads.add(download)
        void download.finally(() => this.downloads.delete(download))
      })
    }
    this.handlers.onDetect?.(this.direction_)
    if (session.type === 'send' && this.handlers.onUpload) void this.chooseUpload(generation)
    if (session.type === 'receive') {
      try {
        void session.start().catch(() => this.fail('ZMODEM 下载握手失败'))
      } catch {
        this.fail('ZMODEM 下载握手失败')
      }
    }
  }

  private async chooseUpload(generation: number): Promise<void> {
    let source: ZmodemUploadSource | null = null
    try {
      source = await this.handlers.onUpload!(this.operation.signal)
      if (generation !== this.generation) return
      if (!source) {
        this.abort()
        return
      }
      await this.sendFile(source)
    } catch {
      if (generation === this.generation) this.fail('无法选择或读取上传文件，请重试')
    } finally {
      try {
        await source?.close()
      } catch {
        this.handlers.onError?.('upload', '无法关闭上传文件')
      }
    }
  }

  private progress(
    name: string,
    bytes: number,
    total: number,
    state: 'transferring' | 'saving' = 'transferring',
  ): void {
    this.handlers.onProgress?.({
      direction: this.direction_,
      fileName: name,
      bytesSent: bytes,
      bytesTotal: total,
      percent: total > 0 ? Math.min(this.direction_ === 'download' ? 99 : 100, Math.round((bytes / total) * 100)) : 0,
      state,
    })
  }

  private wait<T>(operation: Promise<T>): Promise<T> {
    return new Promise<T>((resolve, reject) => {
      const cancel = () => reject(new Error('传输已取消'))
      this.cancelWaiters.add(cancel)
      operation.then(resolve, reject).finally(() => this.cancelWaiters.delete(cancel))
    })
  }

  async sendFile(file: File | ZmodemUploadSource): Promise<void> {
    const session = this.session
    if (!session || session.type !== 'send' || this.uploading) throw new Error('ZMODEM 会话未就绪')
    const generation = this.generation
    this.uploading = true
    this.currentFile = file.name
    this.progress(file.name, 0, file.size)
    try {
      setZmodemUploadPolicy(session, 'overwrite' in file && file.overwrite === true)
      const transfer = await this.wait(session.send_offer({ name: file.name, size: file.size }))
      if (!transfer) throw new Error('远端拒绝接收文件')

      while (transfer.get_offset() < file.size) {
        const offset = transfer.get_offset()
        const bytes = await this.wait(
          'read' in file
            ? file.read(offset, Math.min(8192, file.size - offset))
            : new Promise<Uint8Array>((resolve, reject) => {
                const reader = new FileReader()
                reader.onload = () => resolve(new Uint8Array(reader.result as ArrayBuffer))
                reader.onerror = () => reject(new Error('读取文件失败'))
                reader.readAsArrayBuffer(file.slice(offset, offset + 8192))
              }),
        )
        if (generation !== this.generation) throw new Error('传输已取消')
        transfer.send(bytes)

        await this.wait(this.outbound)
        if (generation !== this.generation) throw new Error('传输已取消')
        this.progress(file.name, transfer.get_offset(), file.size)
        this.touch()
        await this.wait(new Promise<void>((resolve) => setTimeout(resolve, 0)))
      }
      await this.wait(transfer.end())
      await this.wait(session.close())
    } catch (error) {
      if (generation === this.generation) this.fail('ZMODEM 上传失败或远端拒绝接收，请重试')
      throw error
    } finally {
      if (generation === this.generation) this.uploading = false
    }
  }

  private async receiveFile(session: Session, offer: Offer, generation: number): Promise<void> {
    const details = offer.get_details()

    const name =
      Array.from(details.name.split(/[\\/]/).pop() ?? '')
        .filter((char) => char.charCodeAt(0) >= 32 && char.charCodeAt(0) !== 127)
        .join('') || 'download'
    const size = details.size ?? 0
    this.currentFile = name
    if (size > maxDownloadSize) {
      this.fail('文件超过 256 MiB，请使用 SFTP 下载')
      return
    }
    let target: ZmodemDownloadTarget | null = null
    let writes = Promise.resolve()
    try {
      const accepted = await this.handlers.onOffer?.({ name, size })
      if (generation !== this.generation || this.session !== session) return
      if (accepted === false) {
        this.abort()
        return
      }
      if (this.handlers.onDestination) {
        try {
          target = await this.handlers.onDestination({ name, size }, this.operation.signal)
        } catch (error) {
          if (error instanceof SkipTransfer) {
            await offer.skip()
            return
          }
          throw error
        }
        if (generation !== this.generation) return
        if (!target) {
          this.abort()
          return
        }
      }
      if (!target) throw new Error('未选择保存位置')
      this.progress(name, 0, size)
      const destination = target
      let received = 0
      let written = 0
      let saving = false
      let writeFailed = false
      let pending = new Uint8Array(32768)
      let pendingSize = 0
      const flush = () => {
        if (!pendingSize) return
        const chunk = pending.subarray(0, pendingSize)
        pending = new Uint8Array(32768)
        pendingSize = 0

        writes = writes.then(async () => {
          if (generation !== this.generation || writeFailed) return
          try {
            await destination.write(chunk)
            written += chunk.length
            if (generation === this.generation) this.progress(name, written, size, saving ? 'saving' : 'transferring')
          } catch {
            writeFailed = true
          }
        })
      }
      await this.wait(
        offer.accept({
          on_input: (bytes) => {
            if (this.cancelling || generation !== this.generation) return
            received += bytes.length
            if (received > maxDownloadSize) throw new Error('下载超出内存限制')
            if (writeFailed) throw new Error('下载写入失败')
            for (let offset = 0; offset < bytes.length;) {
              const count = Math.min(pending.length - pendingSize, bytes.length - offset)
              pending.set(bytes.slice(offset, offset + count), pendingSize)
              pendingSize += count
              offset += count
              if (pendingSize === pending.length) flush()
            }
          },
        }),
      )
      if (generation !== this.generation) return
      if (this.activeOffer === offer) this.activeOffer = null
      saving = true
      this.progress(name, written, size, 'saving')
      flush()
      await writes
      if (generation !== this.generation) return
      if (writeFailed) throw new Error('下载写入失败')
      await target.finish()
    } finally {
      await writes
      await target?.close()
    }
  }

  getState(): ZmodemState {
    return this.state_
  }
  getDirection(): ZmodemDirection {
    return this.direction_
  }

  private stop(): void {
    ++this.generation
    this.operation.abort()
    this.cancelling = false
    this.activeOffer = null
    clearTimeout(this.outputTimer)
    clearTimeout(this.timer)
    const session = this.session
    this.session = null
    for (const cancel of this.cancelWaiters) cancel()
    this.cancelWaiters.clear()
    this.downloads.clear()
    this.uploading = false
    this.flushBufferedOutput()
    if (session && !session.has_ended()) {
      try {
        session.abort()
      } catch {}
    }
  }

  private armOutputFlush(): void {
    clearTimeout(this.outputTimer)
    this.outputTimer = setTimeout(() => {
      this.outputTimer = undefined
      this.flushBufferedOutput()
    }, 200)
  }

  private flushBufferedOutput(): void {
    if (!this.buffer.length) return

    const pending = this.buffer.splice(0)
    try {
      this.output(pending)
    } catch {}
  }

  private fail(message: string): void {
    this.stop()
    this.state_ = 'error'
    this.handlers.onError?.(this.direction_, message)
  }

  abort(): void {
    if (this.cancelling) return
    if (this.session?.type === 'receive' && !this.session.has_ended()) {
      this.cancelling = true
      this.state_ = 'cancelling'
      ++this.generation
      this.operation.abort()
      for (const cancel of this.cancelWaiters) cancel()
      this.cancelWaiters.clear()
      this.downloads.clear()
      try {
        void this.activeOffer?.skip()
      } catch {}
      this.touch()
      return
    }
    const active = this.session !== null || this.state_ === 'transferring'
    this.stop()
    this.state_ = 'idle'
    if (active) this.handlers.onRetract?.()
  }

  reset(): void {
    const active = this.session !== null || this.state_ === 'transferring'
    this.stop()
    this.state_ = 'idle'
    if (active) this.handlers.onRetract?.()
  }
}
