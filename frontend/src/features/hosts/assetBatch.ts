import type { HostDraft, HostGateway, HostRecord } from '@/services/hosts.ts'

export type BatchField = 'group' | 'icon' | 'protocol'

export interface BatchPatch {
  group?: string
  icon?: string
  protocol?: string
}

export interface BatchResult {
  done: number
  failed: number
}

export type BatchGateway = Pick<HostGateway, 'update' | 'moveHost'>

const READONLY_KEYS = [
  'id',
  'groupId',
  'sortOrder',
  'createdAt',
  'updatedAt',
  'passwordConfigured',
  'proxyPasswordConfigured',
] as const

function toDraft(host: HostRecord): HostDraft {
  const draft = { ...host } as Record<string, unknown>
  for (const key of READONLY_KEYS) delete draft[key]

  return draft as unknown as HostDraft
}

export async function applyBatchEdit(
  records: HostRecord[],
  patch: BatchPatch,
  gateway: BatchGateway,
  onProgress?: (done: number, failed: number) => void,
): Promise<BatchResult> {
  let done = 0
  let failed = 0
  for (const host of records) {
    try {
      if (patch.group !== undefined) {
        await gateway.moveHost(host.id, patch.group)
      } else {
        const draft = toDraft(host)
        if (patch.icon !== undefined) draft.icon = patch.icon
        if (patch.protocol !== undefined) draft.protocol = patch.protocol
        await gateway.update(host.id, draft)
      }
      done += 1
    } catch {
      failed += 1
    }
    onProgress?.(done, failed)
  }
  return { done, failed }
}
