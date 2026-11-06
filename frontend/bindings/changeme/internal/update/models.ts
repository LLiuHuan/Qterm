export interface Asset {
  name: string

  size: number
  url: string
}

export interface Release {
  hasUpdate: boolean

  version: string
  tag: string
  name: string
  notes: string
  pageUrl: string
  publishedAt: string
  asset: Asset
}

export enum State {
  $zero = '',

  StateIdle = 'idle',
  StateChecking = 'checking',
  StateUpToDate = 'up-to-date',
  StateAvailable = 'available',
  StateDownloading = 'downloading',
  StateReady = 'ready',
  StateInstalling = 'installing',
  StateFailed = 'failed',
}

export interface Status {
  state: State

  currentVersion: string

  release?: Release | null

  downloaded: number
  total: number
  percent: number

  messageKey?: string

  safeDetail?: string
}
