import { ref, computed, onBeforeUnmount, onMounted } from 'vue'
import type { SftpGateway, SFTPTransferRecord } from '../services/sftp'
import { listenSharedTransferClaims, refreshSharedOwnership } from '../services/transferOwnership'
import { translate } from '../locales'
import {
  transferEtaText,
  transferQueueAction,
  transferSecondaryAction,
  transferSpeedText,
  type TransferQueueAction,
} from '../services/transferQueue'

export interface TransferProgressItem {
  id: string

  sessionId: string
  name: string
  direction: 'upload' | 'download'
  progress: number
  label: string

  stage?: string
  completed?: boolean
  action?: TransferQueueAction

  secondaryAction?: TransferQueueAction
  speed?: string
  eta?: string
}

interface TransferSpeedSample {
  at: number
  bytes: number
}

export function useTransferState(gateway: SftpGateway) {
  const records = ref<SFTPTransferRecord[]>([])
  const speedSamples = ref<Record<string, TransferSpeedSample[]>>({})
  const lastProgressAt = ref<Record<string, number>>({})
  const speeds = ref<Record<string, number>>({})
  const speedWindowMs = 4500
  const speedStaleMs = 4500

  const dismissedIds = ref<Set<string>>(new Set())
  const error = ref('')
  let timer: ReturnType<typeof setInterval> | undefined
  let refreshInFlight = false

  const items = computed<TransferProgressItem[]>(() => {
    return records.value
      .filter((r) => r.state !== 'completed' && r.state !== 'canceled' && !dismissedIds.value.has(r.id))
      .map((r) => {
        const rawPct = r.total > 0 ? (r.transferred / r.total) * 100 : 0
        const boundedPct = Math.min(100, Math.max(0, rawPct))

        const visiblePct = r.state === 'running' ? Math.min(99.9, boundedPct) : boundedPct
        const pct = Math.round(visiblePct * 10) / 10
        const speed = speeds.value[r.id] || 0

        return {
          id: r.id,
          sessionId: r.sessionId,
          name: r.localName || r.remotePath.split('/').pop() || '',
          direction: r.direction,
          progress: pct,
          label: transferStateLabel(r.state),
          stage: r.stage,
          action: transferQueueAction(r, gateway),
          secondaryAction: transferSecondaryAction(r),

          speed: transferSpeedText(r.rateBytesPerSecond, speed),
          eta:
            r.state === 'running' ? transferEtaText(r.total - r.transferred, r.rateBytesPerSecond ?? speed) : undefined,
        }
      })
  })

  async function refresh(): Promise<void> {
    if (refreshInFlight) return
    refreshInFlight = true
    try {
      refreshSharedOwnership()
      const list = await gateway.listSFTPTransfers()
      records.value = list
      if (dismissedIds.value.size > 0) {
        const known = new Set(list.map((r) => r.id))
        const next = new Set([...dismissedIds.value].filter((id) => known.has(id)))
        if (next.size !== dismissedIds.value.size) dismissedIds.value = next
      }
      const now = Date.now()
      const knownIds = new Set(list.map((r) => r.id))
      for (const r of list) {
        if (r.state === 'running') {
          const previous = speedSamples.value[r.id] ?? []
          const previousSample = previous[previous.length - 1]
          let next = [...previous]

          if (previousSample && r.transferred < previousSample.bytes) {
            next = []
            delete lastProgressAt.value[r.id]
          }

          if (previousSample && r.transferred > previousSample.bytes) {
            lastProgressAt.value[r.id] = now
          }
          next.push({ at: now, bytes: r.transferred })

          const cutoff = now - speedWindowMs
          while (next.length > 1 && next[1]!.at <= cutoff) next.shift()
          speedSamples.value[r.id] = next

          const progressedAt = lastProgressAt.value[r.id]
          if (progressedAt === undefined || now - progressedAt >= speedStaleMs) {
            speeds.value[r.id] = 0
          } else {
            const first = next[0]
            const last = next[next.length - 1]
            const elapsed = (last.at - first.at) / 1000
            const bytes = last.bytes - first.bytes
            speeds.value[r.id] = elapsed > 0 && bytes > 0 ? bytes / elapsed : 0
          }
        } else {
          delete speedSamples.value[r.id]
          delete lastProgressAt.value[r.id]
          delete speeds.value[r.id]
        }
      }

      for (const id of Object.keys(speedSamples.value)) {
        if (!knownIds.has(id)) delete speedSamples.value[id]
      }
      for (const id of Object.keys(lastProgressAt.value)) {
        if (!knownIds.has(id)) delete lastProgressAt.value[id]
      }
      for (const id of Object.keys(speeds.value)) {
        if (!knownIds.has(id)) delete speeds.value[id]
      }
    } catch {
      const now = Date.now()
      for (const [id, progressedAt] of Object.entries(lastProgressAt.value)) {
        if (now - progressedAt >= speedStaleMs) speeds.value[id] = 0
      }
    } finally {
      refreshInFlight = false
    }
  }

  function transferStateLabel(state: string): string {
    const key = `sftp.ui.transfer.${state}`
    const localized = translate(key)
    return localized === key ? state : localized
  }

  onMounted(() => {
    listenSharedTransferClaims()
    void refresh()
    timer = setInterval(() => void refresh(), 800)
  })

  onBeforeUnmount(() => {
    clearInterval(timer)
  })

  function dismissTransfer(id: string): void {
    const next = new Set(dismissedIds.value)
    next.add(id)
    dismissedIds.value = next
  }

  return { items, records, error, refresh, dismissTransfer }
}
