import { ref, type Ref } from 'vue'
import type { ResourceSnapshot } from '../services/sessions'
import { networkMetrics } from '../features/monitor/networkMetrics'
import { resourceNetworks } from '../features/monitor/resourceSelection'

export const MAX_RESOURCE_HISTORY_POINTS = 60

export interface ResourceHistoryPoint {
  timestamp: number

  cpu: number

  memory: number

  rx: number

  tx: number
}

export interface ResourceHistory {
  points: Ref<ResourceHistoryPoint[]>
  push: (snapshot: ResourceSnapshot) => void
  reset: () => void
}

export function createResourceHistory(now: () => number = Date.now): ResourceHistory {
  const points = ref<ResourceHistoryPoint[]>([])
  let networkName: string | undefined
  return {
    points,
    push(snapshot: ResourceSnapshot) {
      if ((snapshot.sampleWindowMs ?? 0) <= 0) return
      const selectedName = resourceNetworks(snapshot.networks)[0]?.name

      if (networkName !== selectedName) points.value = []
      networkName = selectedName
      const network = networkMetrics(snapshot)
      const next: ResourceHistoryPoint = {
        timestamp: now(),
        cpu: snapshot.cpuUsagePercent,
        memory: snapshot.memoryUsagePercent,
        rx: network.receiveRate,
        tx: network.transmitRate,
      }
      const appended = [...points.value, next]
      points.value =
        appended.length > MAX_RESOURCE_HISTORY_POINTS
          ? appended.slice(appended.length - MAX_RESOURCE_HISTORY_POINTS)
          : appended
    },
    reset() {
      points.value = []
      networkName = undefined
    },
  }
}

const historyCache = new Map<string, ResourceHistory>()
const HISTORY_CACHE_LIMIT = 8

export function resourceHistoryFor(sessionId: string): ResourceHistory {
  const existing = historyCache.get(sessionId)
  if (existing) return existing
  const created = createResourceHistory()
  historyCache.set(sessionId, created)
  if (historyCache.size > HISTORY_CACHE_LIMIT) {
    const oldest = historyCache.keys().next().value
    if (oldest !== undefined && oldest !== sessionId) historyCache.delete(oldest)
  }
  return created
}

export function dropResourceHistory(sessionId: string): void {
  historyCache.delete(sessionId)
}
