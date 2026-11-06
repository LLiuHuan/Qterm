import type { RemoteImageContent } from '../../bindings/changeme/internal/model/models'
import { decodeAppError } from './errors'
import { createRuntimeGateway, hasWailsRuntime } from './runtime'

export type RemoteImageSnapshot = RemoteImageContent

export interface RemoteImageResult {
  ok: boolean
  snapshot: RemoteImageSnapshot | null
  errorKey: string
}

export interface PreviewGateway {
  readonly persistent: boolean
  readRemoteImage(sessionID: string, remotePath: string): Promise<RemoteImageSnapshot>
}

class WailsPreviewGateway implements PreviewGateway {
  readonly persistent = true

  async readRemoteImage(sessionID: string, remotePath: string): Promise<RemoteImageSnapshot> {
    const { ReadRemoteImage } = await import('../../bindings/changeme/internal/app/service')
    return await ReadRemoteImage(sessionID, remotePath)
  }
}

class MemoryPreviewGateway implements PreviewGateway {
  readonly persistent = false

  async readRemoteImage(): Promise<RemoteImageSnapshot> {
    return { contentBase64: '', size: 0, mimeType: '', truncated: true, modTime: '' }
  }
}

export function createPreviewGateway(): PreviewGateway {
  return createRuntimeGateway<PreviewGateway>(
    () => new WailsPreviewGateway(),
    () => new MemoryPreviewGateway(),
    shouldUseNativeRuntime,
  )
}

function shouldUseNativeRuntime(): boolean {
  return window.location.protocol === 'wails:' || hasWailsRuntime()
}

let gateway: PreviewGateway | null = null

function currentGateway(): PreviewGateway {
  if (!gateway) gateway = createPreviewGateway()
  return gateway
}

export function __setPreviewGatewayForTest(next: PreviewGateway | null): void {
  gateway = next
}

export async function readRemoteImage(sessionID: string, remotePath: string): Promise<RemoteImageResult> {
  if (!sessionID) return { ok: false, snapshot: null, errorKey: 'session.not_found' }
  try {
    const snapshot = await currentGateway().readRemoteImage(sessionID, remotePath)
    return { ok: true, snapshot, errorKey: '' }
  } catch (error) {
    const appError = decodeAppError(error)
    return { ok: false, snapshot: null, errorKey: appError?.messageKey || 'error.unknown' }
  }
}
