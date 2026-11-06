import { shallowRef } from 'vue'

export type ConflictAction = 'overwrite' | 'copy' | 'stop' | 'cancel'
interface ConflictRequest {
  name: string
  resolve: (action: ConflictAction) => void
}

export const activeTransferConflict = shallowRef<ConflictRequest | null>(null)
const pending: ConflictRequest[] = []
export function requestTransferConflict(name: string, signal: AbortSignal): Promise<ConflictAction> {
  if (signal.aborted) return Promise.resolve('cancel')
  return new Promise((resolve) => {
    const request: ConflictRequest = { name, resolve: finish }
    function finish(action: ConflictAction): void {
      signal.removeEventListener('abort', abort)
      const index = pending.indexOf(request)
      if (index >= 0) pending.splice(index, 1)
      if (activeTransferConflict.value === request) activeTransferConflict.value = pending.shift() ?? null
      resolve(action)
    }
    const abort = () => finish('cancel')
    signal.addEventListener('abort', abort, { once: true })
    if (activeTransferConflict.value) pending.push(request)
    else activeTransferConflict.value = request
  })
}

export function transferCopyName(name: string, number: number): string {
  const dot = name.lastIndexOf('.')
  return dot > 0 ? `${name.slice(0, dot)}${number}${name.slice(dot)}` : `${name}${number}`
}

export class SkipTransfer extends Error {}

export type RemoteDirectorySource = 'terminal' | 'rz'
interface RemoteDirectoryRequest {
  source: RemoteDirectorySource
  resolve: (directory: string | null) => void
}
export const remoteDirectoryRequest = shallowRef<RemoteDirectoryRequest | null>(null)
export function requestRemoteDirectory(signal: AbortSignal, source: RemoteDirectorySource): Promise<string | null> {
  if (signal.aborted) return Promise.resolve(null)
  return new Promise((resolve) => {
    const abort = () => finish(null)
    const request: RemoteDirectoryRequest = { source, resolve: finish }
    function finish(directory: string | null): void {
      signal.removeEventListener('abort', abort)
      if (remoteDirectoryRequest.value === request) remoteDirectoryRequest.value = null
      resolve(directory)
    }
    signal.addEventListener('abort', abort, { once: true })
    remoteDirectoryRequest.value = request
  })
}
