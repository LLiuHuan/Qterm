export type AssetKind = 'host' | 'group' | 'snippet' | 'snippetGroup' | 'credential' | 'proxy' | 'credentialPromptRule'

export type AssetAction = 'create' | 'update' | 'delete' | 'invalidate'

export interface AssetChange {
  kind: AssetKind
  action: AssetAction

  record?: unknown

  id?: string

  cascadedIds?: string[]

  seq?: number

  origin: string
}

export const ASSET_CHANGED_EVENT = 'qterm:asset-changed'

export const ASSET_CHANGED_BRIDGE_EVENT = 'qterm:asset-changed-bridge'

const ORIGIN = 'window'

let nextSeq = 1
let lastSentSeq = 0

export async function emitAssetChange(change: Omit<AssetChange, 'origin' | 'seq'>): Promise<void> {
  const payload: AssetChange = { ...change, origin: ORIGIN, seq: nextSeq++ }
  lastSentSeq = payload.seq ?? 0
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent<AssetChange>(ASSET_CHANGED_EVENT, { detail: payload }))
  }
  try {
    const { Events } = await import('@wailsio/runtime')
    const { hasWailsRuntime } = await import('./runtime')
    if (!hasWailsRuntime()) return
    await Events.Emit(ASSET_CHANGED_BRIDGE_EVENT, payload)
  } catch {}
}

export function isSelfEcho(change: Pick<AssetChange, 'origin' | 'seq'>): boolean {
  return change.origin === ORIGIN && change.seq === lastSentSeq
}

export function connectAssetChanged(handler: (change: AssetChange) => void): () => void {
  if (typeof window === 'undefined') return () => {}

  const dispatch = (change: AssetChange | undefined, viaBridge: boolean): void => {
    if (!change || typeof change !== 'object' || !change.kind || !change.action) return
    if (viaBridge && isSelfEcho(change)) return
    handler(change)
  }
  const onPage = (event: Event): void => dispatch((event as CustomEvent<AssetChange>).detail, false)

  window.addEventListener(ASSET_CHANGED_EVENT, onPage)
  let disposeBridge: (() => void) | undefined
  void (async () => {
    try {
      const { Events } = await import('@wailsio/runtime')
      const { hasWailsRuntime } = await import('./runtime')
      if (!hasWailsRuntime()) return
      disposeBridge = Events.On(ASSET_CHANGED_BRIDGE_EVENT, (event: { data?: AssetChange }) => {
        dispatch(event?.data, true)
      })
    } catch {}
  })()

  return () => {
    window.removeEventListener(ASSET_CHANGED_EVENT, onPage)
    disposeBridge?.()
  }
}
