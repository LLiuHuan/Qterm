import { createRuntimeGateway, hasWailsRuntime } from './runtime'

export type UpdateState =
  'idle' | 'checking' | 'up-to-date' | 'no-release' | 'available' | 'downloading' | 'ready' | 'installing' | 'failed'

export interface UpdateAsset {
  name: string
  size: number
  url: string
}

export interface UpdateRelease {
  hasUpdate: boolean

  noRelease: boolean
  version: string
  tag: string
  name: string
  notes: string
  pageUrl: string
  publishedAt: string
  asset: UpdateAsset
}

export interface UpdateStatus {
  state: UpdateState
  currentVersion: string
  release?: UpdateRelease
  downloaded: number
  total: number
  percent: number
  messageKey?: string
  safeDetail?: string
}

export interface UpdateGateway {
  status(): Promise<UpdateStatus>
  check(currentVersion: string): Promise<UpdateRelease>
  download(): Promise<void>
  install(): Promise<void>

  subscribe(listener: (status: UpdateStatus) => void): () => void
}

const EMPTY_STATUS: UpdateStatus = {
  state: 'idle',
  currentVersion: '',
  downloaded: 0,
  total: 0,
  percent: 0,
}

function toNumber(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

function toStringValue(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function normalizeRelease(value: unknown): UpdateRelease | undefined {
  if (!value || typeof value !== 'object') return undefined
  const source = value as Record<string, unknown>
  const asset = (source.asset ?? {}) as Record<string, unknown>
  return {
    hasUpdate: source.hasUpdate === true,
    noRelease: source.noRelease === true,
    version: toStringValue(source.version),
    tag: toStringValue(source.tag),
    name: toStringValue(source.name),
    notes: toStringValue(source.notes),
    pageUrl: toStringValue(source.pageUrl),
    publishedAt: toStringValue(source.publishedAt),
    asset: {
      name: toStringValue(asset.name),
      size: toNumber(asset.size),
      url: toStringValue(asset.url),
    },
  }
}

export function normalizeStatus(value: unknown): UpdateStatus {
  if (!value || typeof value !== 'object') return { ...EMPTY_STATUS }
  const source = value as Record<string, unknown>
  const state = toStringValue(source.state) as UpdateState
  const percent = toNumber(source.percent)
  return {
    state: state || 'idle',
    currentVersion: toStringValue(source.currentVersion),
    release: normalizeRelease(source.release),
    downloaded: toNumber(source.downloaded),
    total: toNumber(source.total),
    percent: Math.min(100, Math.max(0, percent)),
    messageKey: toStringValue(source.messageKey) || undefined,
    safeDetail: toStringValue(source.safeDetail) || undefined,
  }
}

const wailsGateway: UpdateGateway = {
  async status() {
    const { UpdateStatus } = await import('../../bindings/changeme/internal/app/service')
    return normalizeStatus(await UpdateStatus())
  },
  async check(currentVersion: string) {
    const { UpdateCheck } = await import('../../bindings/changeme/internal/app/service')
    const release = normalizeRelease(await UpdateCheck(currentVersion))
    if (!release) throw new Error('update.unavailable')
    return release
  },
  async download() {
    const { UpdateDownload } = await import('../../bindings/changeme/internal/app/service')
    await UpdateDownload()
  },
  async install() {
    const { UpdateInstall } = await import('../../bindings/changeme/internal/app/service')
    await UpdateInstall()
  },
  subscribe(listener) {
    if (!hasWailsRuntime()) return () => {}
    let stop: (() => void) | undefined
    let cancelled = false
    void import('@wailsio/runtime').then(({ Events }) => {
      if (cancelled) return
      stop = Events.On('update:status', (event: { data?: unknown }) => {
        listener(normalizeStatus(event?.data))
      })
    })
    return () => {
      cancelled = true
      stop?.()
    }
  },
}

const memoryGateway: UpdateGateway = {
  async status() {
    return { ...EMPTY_STATUS }
  },
  async check() {
    throw new Error('update.unavailable')
  },
  async download() {},
  async install() {},
  subscribe() {
    return () => {}
  },
}

export function updatesAvailable(): boolean {
  return hasWailsRuntime()
}

export const updates: UpdateGateway = createRuntimeGateway(
  () => wailsGateway,
  () => memoryGateway,
)
