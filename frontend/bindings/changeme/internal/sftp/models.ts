// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import * as model$0 from '../model/models.js'

export interface DirectoryDownloadResult {
  queued: number
  skipped: number
}

export interface DirectoryUploadResult {
  canceled: boolean
  queued: number
}

export interface DropUploadResult {
  transfers: model$0.SFTPTransfer[] | null
  rejected: string[] | null
  queued: number
  fellBack: boolean
  fallbackReason?: string
}

export interface LocalDirectory {
  token: string
  displayPath: string
  name: string
  parentToken: string
  segments: LocalDirectorySegment[] | null
  entries: LocalDirectoryEntry[] | null
}

export interface LocalDirectoryEntry {
  name: string
  type: LocalFileType
  size: number
  sizeText: string
  modTime: string
  mode: string
  displayPath: string
  isSymlink: boolean
}

export interface LocalDirectorySegment {
  name: string
  token: string
}

export enum LocalFileType {
  $zero = '',

  LocalFileTypeFile = 'file',
  LocalFileTypeDirectory = 'directory',
  LocalFileTypeOther = 'other',
}

export interface RemoteDirectoryTransferResult {
  queued: number
  skipped: number
}

export interface SFTPLocalDirectorySelectionResult {
  canceled: boolean
  directory: LocalDirectory | null
}

export interface SFTPTransferSelectionResult {
  canceled: boolean
  transfer: model$0.SFTPTransfer | null
}

export interface Settings {
  autoDirectoryFollow: boolean
  showHiddenFiles: boolean
  maxTransfers: number
  localDirectory: string
  editorPath: string

  dragFolderUploadMode: string

  transferDiagnostics?: TransferDiagnostics | null
}

export interface TransferDiagnostics {
  activeConnections: number

  desiredConnections: number

  blockedReason: string

  recorded: boolean
}
