import { TrzszFilter, type TrzszOptions } from 'trzsz'

export const TRZSZ_MAGIC = '::TRZSZ:TRANSFER:'

const IPC_CHUNK = 512 * 1024

export type TrzszState = 'idle' | 'sending' | 'receiving'

export interface TrzszHandlers {
  onState: (state: TrzszState, detail?: string) => void
}

export interface TrzszEndpoints {
  sendToServer: (data: Uint8Array) => void

  writeToTerminal: (data: Uint8Array) => void

  columns: () => number
}

interface TrzszInternals {
  trzszTransfer: {
    sendAction: (ok: boolean, tmux: boolean) => Promise<void>
    recvConfig: () => Promise<{ quiet: boolean; tmux_pane_width: number; overwrite: boolean; directory: boolean }>
    sendFiles: (readers: TrzszFileReaderLike[], progress: unknown) => Promise<string[]>
    recvFiles: (factory: unknown, openSaveFile: OpenSaveFile, progress: unknown) => Promise<string[]>
    clientExit: (message: string) => Promise<void>
  }
  createProgressBar: (quiet: boolean, tmuxPaneWidth: number) => void
  textProgressBar: unknown
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
  private state: TrzszState = 'idle'

  constructor(endpoints: TrzszEndpoints, handlers: TrzszHandlers) {
    this.endpoints = endpoints
    this.handlers = handlers
    const options: TrzszOptions = {
      writeToTerminal: (output) => this.endpoints.writeToTerminal(toBytes(output)),
      sendToServer: (input) => {
        const bytes = typeof input === 'string' ? new TextEncoder().encode(input) : input
        this.endpoints.sendToServer(bytes)
      },
      terminalColumns: this.endpoints.columns(),

      chooseSendFiles: async () => undefined,
      chooseSaveDirectory: async () => undefined,
    }
    this.filter = new TrzszFilter(options)
    this.installDownloadHandler()
    this.installUploadHandler()
  }

  consume(data: Uint8Array): boolean {
    this.filter.processServerOutput(data)
    return this.filter.isTransferringFiles()
  }

  consumeInput(text: string): void {
    this.filter.processTerminalInput(text)
  }

  setColumns(columns: number): void {
    this.filter.setTerminalColumns(columns)
  }

  isTransferring(): boolean {
    return this.filter.isTransferringFiles()
  }

  private updateState(state: TrzszState, detail?: string): void {
    if (this.state === state && !detail) return
    this.state = state
    this.handlers.onState(state, detail)
  }

  private internals(): TrzszInternals & { filter: TrzszFilter } {
    return this.filter as unknown as TrzszInternals & { filter: TrzszFilter }
  }

  private installDownloadHandler(): void {
    const updateState = (state: TrzszState, detail?: string) => this.updateState(state, detail)
    const handler = async function (this: TrzszInternals, _version: string, tmux: boolean): Promise<void> {
      const api = await import('../../bindings/changeme/internal/app/service')
      const directory = await api.SelectTrzszDownloadDirectory()
      if (!directory.token) {
        await this.trzszTransfer.sendAction(false, tmux)
        return
      }
      updateState('receiving')
      await this.trzszTransfer.sendAction(true, tmux)
      const config = await this.trzszTransfer.recvConfig()
      this.createProgressBar(config.quiet, config.tmux_pane_width)
      const created: { token: string; close: () => Promise<void>; abort: () => Promise<void> }[] = []
      const openSaveFile: OpenSaveFile = async (_factory, name, _directory, overwrite) => {
        const parsedName = parseDownloadName(name)
        if (parsedName?.isDir) {
          await api.CreateTrzszDirectory(directory.token, parsedName.relativePath)
          return {
            getFileName: () => parsedName.name,
            getLocalName: () => parsedName.name,
            isDir: () => true,
            writeFile: async () => {},
            deleteFile: async () => parsedName.name,
            closeFile: () => {},
          }
        }
        const relativePath = parsedName?.relativePath ?? name
        let offset = 0
        const opened = await api.OpenTrzszDownloadFile(directory.token, relativePath, overwrite === true)
        const entry = {
          token: opened.token,
          close: async () => {
            await api.CommitTrzszFile(opened.token)
          },
          abort: async () => {
            await api.AbortTrzszFile(opened.token)
          },
        }
        created.push(entry)
        return {
          getFileName: () => parsedName?.name ?? name,
          getLocalName: () => opened.name,
          isDir: () => false,
          writeFile: async (data) => {
            for (let index = 0; index < data.length; index += IPC_CHUNK) {
              const slice = data.subarray(index, Math.min(index + IPC_CHUNK, data.length))
              await api.WriteTrzszFile(opened.token, offset, toBase64(slice))
              offset += slice.length
            }
          },
          deleteFile: async () => {
            await entry.abort()
            return opened.name
          },
          closeFile: () => {},
        }
      }
      try {
        const names = await this.trzszTransfer.recvFiles({ token: directory.token }, openSaveFile, this.textProgressBar)

        for (const entry of created) await entry.close()
        await this.trzszTransfer.clientExit(`${names.length} file(s) received`)
        updateState('idle', 'received')
      } catch (error) {
        for (const entry of created) await entry.abort().catch(() => {})
        await this.trzszTransfer.clientExit(`receive failed: ${errorMessage(error)}`).catch(() => {})
        updateState('idle', 'error')
        throw error
      }
    }
    const target = this.filter as unknown as Record<string, unknown>
    target.handleTrzszDownloadFiles = handler
  }

  private installUploadHandler(): void {
    const updateState = (state: TrzszState, detail?: string) => this.updateState(state, detail)
    const handler = async function (this: TrzszInternals, _directory: boolean, tmux: boolean): Promise<void> {
      const api = await import('../../bindings/changeme/internal/app/service')
      const files = await api.SelectTrzszUploads()
      if (!files || files.length === 0) {
        await this.trzszTransfer.sendAction(false, tmux)
        return
      }
      updateState('sending')
      const readers = files.map((file, index) => createUploadReader(api, file, index))
      try {
        await this.trzszTransfer.sendAction(true, tmux)
        const config = await this.trzszTransfer.recvConfig()
        this.createProgressBar(config.quiet, config.tmux_pane_width)
        const names = await this.trzszTransfer.sendFiles(readers, this.textProgressBar)
        await this.trzszTransfer.clientExit(`${names.length} file(s) sent`)
        updateState('idle', 'sent')
      } catch (error) {
        updateState('idle', 'error')
        throw error
      } finally {
        for (const reader of readers) reader.closeFile()
      }
    }
    const target = this.filter as unknown as Record<string, unknown>
    target.handleTrzszUploadFiles = handler
  }
}

function parseDownloadName(name: string): { name: string; relativePath: string; isDir: boolean } | null {
  if (!name.startsWith('{')) return null
  try {
    const parsed = JSON.parse(name) as { path_name?: unknown; is_dir?: unknown }
    if (!Array.isArray(parsed.path_name) || typeof parsed.is_dir !== 'boolean') return null
    const segments = parsed.path_name.filter(
      (segment): segment is string => typeof segment === 'string' && segment !== '',
    )
    if (!segments.length) return null
    return { name: segments[segments.length - 1], relativePath: segments.join('/'), isDir: parsed.is_dir }
  } catch {
    return null
  }
}

type AppApi = typeof import('../../bindings/changeme/internal/app/service')

function createUploadReader(
  api: AppApi,
  file: { token: string; name: string; size: number },
  index: number,
): TrzszFileReaderLike & { closeFile: () => void } {
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
      const remaining = file.size - offset
      if (remaining <= 0) return new Uint8Array(0)
      const length = Math.min(buffer.byteLength, remaining, IPC_CHUNK)
      const encoded = await api.ReadZmodemFile(file.token, offset, length)
      const bytes = fromBase64(encoded ?? '')
      offset += bytes.length
      return bytes
    },
    closeFile: release,
  }
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error)
}
