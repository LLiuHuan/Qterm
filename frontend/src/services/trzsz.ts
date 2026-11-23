import { TrzszFilter, type TrzszOptions } from 'trzsz'

export const TRZSZ_MAGIC = '::TRZSZ:TRANSFER:'
const HANDSHAKE_LIMIT = 128
const HANDSHAKE_TIMEOUT = 200
const handshakePattern = /^::TRZSZ:TRANSFER:[SRD]:\d+\.\d+\.\d+(?::\d+){0,2}/

const UPLOAD_READ_CHUNK = 512 * 1024

const DOWNLOAD_WRITE_CHUNK = 512 * 1024

export type TrzszState = 'idle' | 'selecting' | 'sending' | 'receiving'

export interface TrzszHandlers {
  onState: (state: TrzszState, detail?: string) => void
}

export interface TrzszEndpoints {
  sendToServer: (data: Uint8Array) => void

  writeToTerminal: (data: Uint8Array) => void

  columns: () => number
}

interface TrzszInternals {
  detectAndHandleTrzsz: (marker: string) => Promise<void>
  handleTrzszDownloadFiles: (version: string, remoteIsWindows: boolean) => Promise<void>
  handleTrzszUploadFiles: (version: string, directory: boolean, remoteIsWindows: boolean) => Promise<void>
  trzszTransfer: {
    sendAction: (ok: boolean, remoteIsWindows: boolean) => Promise<void>
    recvConfig: () => Promise<{ quiet: boolean; tmux_pane_width: number; overwrite: boolean; directory: boolean }>
    sendFiles: (readers: TrzszFileReaderLike[], progress: unknown) => Promise<string[]>
    recvFiles: (factory: unknown, openSaveFile: OpenSaveFile, progress: unknown) => Promise<string[]>
    clientExit: (message: string) => Promise<void>
  }
  createProgressBar: (quiet: boolean, tmuxPaneWidth: number) => void
  textProgressBar: { showCursor: () => void } | null
}

interface TrzszFileReaderLike {
  getPathId: () => number
  getRelPath: () => string[]
  isDir: () => boolean
  getSize: () => number
  readFile: (buffer: ArrayBuffer) => Promise<Uint8Array>
  closeFile: () => void
}

interface TrzszFileWriterLike {
  getFileName: () => string
  getLocalName: () => string
  isDir: () => boolean
  writeFile: (data: Uint8Array) => Promise<void>
  deleteFile: () => Promise<string>
  closeFile: () => void
}

type OpenSaveFile = (
  factory: unknown,
  name: string,
  directory: boolean,
  overwrite: boolean,
) => Promise<TrzszFileWriterLike>

function toBase64(data: Uint8Array): string {
  let binary = ''
  const step = 0x8000
  for (let index = 0; index < data.length; index += step) {
    binary += String.fromCharCode(...data.subarray(index, index + step))
  }
  return btoa(binary)
}

function fromBase64(encoded: string): Uint8Array {
  const binary = atob(encoded)
  const bytes = new Uint8Array(binary.length)
  for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index)
  return bytes
}

function toBytes(output: string | ArrayBuffer | Uint8Array | Blob): Uint8Array {
  if (typeof output === 'string') return new TextEncoder().encode(output)
  if (output instanceof Uint8Array) return output
  if (output instanceof ArrayBuffer) return new Uint8Array(output)
  return new Uint8Array(0)
}

export class TrzszManager {
  private readonly filter: TrzszFilter
  private readonly endpoints: TrzszEndpoints
  private readonly handlers: TrzszHandlers

  private readonly handshakeDecoder = new TextDecoder('latin1')
  private state: TrzszState = 'idle'
  private outcome: string | undefined

  private lastError = ''
  private tail = new Uint8Array(0)
  private tailTimer: ReturnType<typeof setTimeout> | undefined
  private starting: Uint8Array[] | null = null
  private cancelled = false
  private disposed = false

  constructor(endpoints: TrzszEndpoints, handlers: TrzszHandlers) {
    this.endpoints = endpoints
    this.handlers = handlers
    const options: TrzszOptions = {
      writeToTerminal: (output) => {
        if (!this.disposed) this.endpoints.writeToTerminal(toBytes(output))
      },
      sendToServer: (input) => {
        if (this.disposed) return
        const bytes = typeof input === 'string' ? new TextEncoder().encode(input) : input
        this.endpoints.sendToServer(bytes)
      },
      terminalColumns: this.endpoints.columns(),
      maxDataChunkSize: UPLOAD_READ_CHUNK,

      chooseSendFiles: async () => undefined,
      chooseSaveDirectory: async () => undefined,
    }
    this.filter = new TrzszFilter(options)
    const internals = this.internals()
    internals.handleTrzszDownloadFiles = (_version, remoteIsWindows) => this.receiveFiles(internals, remoteIsWindows)
    internals.handleTrzszUploadFiles = (_version, _directory, remoteIsWindows) =>
      this.sendFiles(internals, remoteIsWindows)
  }

  consume(data: Uint8Array): boolean {
    if (this.disposed) return false
    if (this.starting) {
      this.starting.push(data)
      return true
    }
    if (this.filter.isTransferringFiles()) {
      this.filter.processServerOutput(data)
      return true
    }

    clearTimeout(this.tailTimer)
    let combined = data
    if (this.tail.length) {
      combined = new Uint8Array(this.tail.length + data.length)
      combined.set(this.tail)
      combined.set(data, this.tail.length)
      this.tail = new Uint8Array(0)
    } else if (!data.includes(0x3a)) {
      this.endpoints.writeToTerminal(data)
      return false
    }
    const text = this.handshakeDecoder.decode(combined)
    let pending = text.length
    for (
      let start = text.indexOf(TRZSZ_MAGIC);
      start >= 0;
      start = text.indexOf(TRZSZ_MAGIC, start + TRZSZ_MAGIC.length)
    ) {
      const candidate = text.slice(start, start + HANDSHAKE_LIMIT)
      const marker = handshakePattern.exec(candidate)?.[0]
      const suffix = marker ? candidate.slice(marker.length) : ''
      const ending = suffix.startsWith('\x1b\\') ? 2 : (/^\r*\n/.exec(suffix)?.[0].length ?? 0)
      if (marker && ending > 0) {
        if (start > 0) this.endpoints.writeToTerminal(combined.subarray(0, start))

        this.beginTransfer(marker, combined.subarray(start + marker.length + ending))
        return true
      }
      if (candidate.length < HANDSHAKE_LIMIT && !candidate.includes('\n')) pending = Math.min(pending, start)
    }
    for (let length = Math.min(TRZSZ_MAGIC.length - 1, text.length); length > 0; length--) {
      if (text.endsWith(TRZSZ_MAGIC.slice(0, length))) {
        pending = Math.min(pending, text.length - length)
        break
      }
    }
    if (pending > 0) this.endpoints.writeToTerminal(combined.subarray(0, pending))
    if (pending < combined.length) {
      this.tail = combined.slice(pending)
      this.tailTimer = setTimeout(() => {
        const tail = this.tail
        this.tail = new Uint8Array(0)
        if (!this.disposed) this.endpoints.writeToTerminal(tail)
      }, HANDSHAKE_TIMEOUT)
    }
    return false
  }

  consumeInput(text: string): void {
    if (text !== '\x03' || !this.isTransferring()) return
    this.cancelled = true
    this.filter.stopTransferringFiles()
  }

  setColumns(columns: number): void {
    this.filter.setTerminalColumns(columns)
  }

  isTransferring(): boolean {
    return !this.disposed && (this.state !== 'idle' || this.filter.isTransferringFiles())
  }

  get failureReason(): string {
    return this.lastError
  }

  dispose(): void {
    if (this.disposed) return
    this.internals().textProgressBar?.showCursor()
    this.disposed = true
    this.cancelled = true
    clearTimeout(this.tailTimer)
    this.tail = new Uint8Array(0)
    this.starting = null
    this.filter.stopTransferringFiles()
    this.updateState('idle')
  }

  private beginTransfer(marker: string, remaining: Uint8Array): void {
    this.cancelled = false
    this.outcome = undefined
    this.lastError = ''
    this.starting = remaining.length ? [remaining] : []
    this.updateState('selecting')

    const normalizedMarker = marker.split(':').length === 6 ? `${marker}:0` : marker
    const transfer = this.internals().detectAndHandleTrzsz(normalizedMarker)
    queueMicrotask(() => {
      const pending = this.starting
      this.starting = null
      if (this.disposed || !pending) return
      if (this.cancelled) this.filter.stopTransferringFiles()
      if (this.filter.isTransferringFiles()) {
        for (const data of pending) this.filter.processServerOutput(data)
      } else {
        for (const data of pending) this.endpoints.writeToTerminal(data)
      }
    })
    void transfer
      .catch((error: unknown) => {
        this.outcome = 'error'
        this.lastError = error instanceof Error ? error.message : String(error)
      })
      .finally(() => {
        if (!this.disposed) this.updateState('idle', this.cancelled ? 'cancelled' : this.outcome)
      })
  }

  private updateState(state: TrzszState, detail?: string): void {
    if (this.state === state && !detail) return
    this.state = state
    this.handlers.onState(state, detail)
  }

  private internals(): TrzszInternals {
    return this.filter as unknown as TrzszInternals
  }

  private async receiveFiles(filter: TrzszInternals, remoteIsWindows: boolean): Promise<void> {
    const api = await import('../../bindings/changeme/internal/app/service')
    const transfer = filter.trzszTransfer
    let directoryToken = ''
    let stagedToken = ''
    const commitSavedFile = async () => {
      if (!stagedToken) return
      this.checkTransferActive()
      await api.CommitTrzszFile(stagedToken)
      stagedToken = ''
    }
    try {
      const directory = this.cancelled ? null : await api.SelectTrzszDownloadDirectory()
      directoryToken = directory?.token ?? ''
      if (this.cancelled || !directoryToken) {
        this.outcome = 'cancelled'
        await transfer.sendAction(false, remoteIsWindows)
        return
      }
      this.updateState('receiving')
      await transfer.sendAction(true, remoteIsWindows)
      const config = await transfer.recvConfig()
      filter.createProgressBar(config.quiet, config.tmux_pane_width)

      const openSaveFile: OpenSaveFile = async (_factory, name, directoryMode, overwrite) => {
        await commitSavedFile()
        this.checkTransferActive()
        const parsedName = directoryMode ? parseDownloadName(name) : { name, relativePath: name, isDir: false }
        if (parsedName.isDir) {
          await api.CreateTrzszDirectory(directoryToken, parsedName.relativePath)
          return {
            getFileName: () => parsedName.name,
            getLocalName: () => parsedName.name,
            isDir: () => true,
            writeFile: async () => {},
            deleteFile: async () => parsedName.name,
            closeFile: () => {},
          }
        }
        let offset = 0
        const opened = await api.OpenTrzszDownloadFile(directoryToken, parsedName.relativePath, overwrite === true)
        stagedToken = opened.token
        this.checkTransferActive()
        return {
          getFileName: () => parsedName.name,
          getLocalName: () => opened.name,
          isDir: () => false,
          writeFile: async (data) => {
            for (let index = 0; index < data.length; index += DOWNLOAD_WRITE_CHUNK) {
              this.checkTransferActive()
              const slice = data.subarray(index, index + DOWNLOAD_WRITE_CHUNK)
              await api.WriteTrzszFile(opened.token, offset, toBase64(slice))
              offset += slice.length
            }
          },
          deleteFile: async () => {
            await api.AbortTrzszFile(opened.token)
            return opened.name
          },

          closeFile: () => {},
        }
      }

      const names = await transfer.recvFiles({ token: directoryToken }, openSaveFile, filter.textProgressBar)
      await commitSavedFile()
      this.checkTransferActive()
      await transfer.clientExit(`${names.length} file(s) received`)
      this.outcome = 'received'
    } catch (error) {
      if (!this.disposed) this.reportTransferError(error)
    } finally {
      if (stagedToken) await api.AbortTrzszFile(stagedToken).catch(() => {})
      if (directoryToken) await api.ReleaseTrzszDownloadDirectory(directoryToken).catch(() => {})
    }
  }

  private async sendFiles(filter: TrzszInternals, remoteIsWindows: boolean): Promise<void> {
    const api = await import('../../bindings/changeme/internal/app/service')
    const transfer = filter.trzszTransfer
    let readers: TrzszFileReaderLike[] = []
    try {
      const files = this.cancelled ? [] : ((await api.SelectTrzszUploads()) ?? [])
      readers = files.map((file, index) => createUploadReader(api, file, index, () => this.checkTransferActive()))
      if (this.cancelled || readers.length === 0) {
        this.outcome = 'cancelled'
        await transfer.sendAction(false, remoteIsWindows)
        return
      }
      this.updateState('sending')
      await transfer.sendAction(true, remoteIsWindows)
      const config = await transfer.recvConfig()

      if (config.overwrite && new Set(files.map((file) => file.name)).size !== readers.length) {
        throw new Error('Duplicate upload names')
      }
      filter.createProgressBar(config.quiet, config.tmux_pane_width)
      const names = await transfer.sendFiles(readers, filter.textProgressBar)
      this.checkTransferActive()
      await transfer.clientExit(`${names.length} file(s) sent`)
      this.outcome = 'sent'
    } catch (error) {
      if (!this.disposed) this.reportTransferError(error)
    } finally {
      for (const reader of readers) reader.closeFile()
    }
  }

  private checkTransferActive(): void {
    if (this.cancelled) throw new Error('Transfer cancelled')
  }

  private reportTransferError(error: unknown): never {
    this.outcome = 'error'
    this.lastError = error instanceof Error ? error.message : String(error)
    if (error instanceof Error && error.name === 'TrzszError') throw error

    const safe = new Error(this.cancelled ? 'Transfer cancelled' : 'Qterm could not complete the file transfer')
    safe.stack = undefined
    throw safe
  }
}

function parseDownloadName(name: string): { name: string; relativePath: string; isDir: boolean } {
  const parsed = JSON.parse(name) as { path_name?: unknown; is_dir?: unknown } | null
  if (!parsed || !Array.isArray(parsed.path_name) || typeof parsed.is_dir !== 'boolean') {
    throw new Error('Invalid directory entry')
  }
  const segments = parsed.path_name as unknown[]
  if (
    !segments.length ||
    !segments.every(
      (segment): segment is string => typeof segment === 'string' && segment !== '' && !/[\\/]/.test(segment),
    )
  ) {
    throw new Error('Invalid directory path')
  }
  return { name: segments[segments.length - 1]!, relativePath: segments.join('/'), isDir: parsed.is_dir }
}

type AppApi = typeof import('../../bindings/changeme/internal/app/service')

function createUploadReader(
  api: AppApi,
  file: { token: string; name: string; size: number },
  index: number,
  checkTransferActive: () => void,
): TrzszFileReaderLike {
  let offset = 0
  let released = false
  const release = () => {
    if (released) return
    released = true
    void api.ReleaseZmodemFile(file.token).catch(() => {})
  }
  return {
    getPathId: () => index,
    getRelPath: () => [file.name],
    isDir: () => false,
    getSize: () => file.size,
    readFile: async (buffer) => {
      checkTransferActive()
      const remaining = file.size - offset
      if (remaining <= 0) return new Uint8Array(0)
      const length = Math.min(buffer.byteLength, remaining, UPLOAD_READ_CHUNK)
      const encoded = await api.ReadZmodemFile(file.token, offset, length)
      checkTransferActive()
      const bytes = fromBase64(encoded ?? '')
      if (bytes.length !== length) throw new Error('Incomplete file read')
      offset += bytes.length
      return bytes
    },
    closeFile: release,
  }
}
