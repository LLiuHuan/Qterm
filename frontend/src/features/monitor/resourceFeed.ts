import { ref, shallowRef, type Ref, type ShallowRef } from 'vue'
import type { ResourceSnapshot, SessionGateway } from '../../services/sessions'
import { createResourceHistory, type ResourceHistory } from '../../composables/useResourceHistory'

const RESOURCE_FEED_INTERVAL_MS = 5000
const IDLE_FEED_LIMIT = 8

export interface ResourceFeedLease {
  snapshot: ShallowRef<ResourceSnapshot | null>
  error: ShallowRef<unknown>
  loading: Ref<boolean>
  history: ResourceHistory
  release: (reset?: boolean) => void
}

interface ScheduledPoll {
  handle: ReturnType<typeof setTimeout>
  version: number
}

interface ResourceFeedEntry {
  snapshot: ShallowRef<ResourceSnapshot | null>
  error: ShallowRef<unknown>
  loading: Ref<boolean>
  history: ResourceHistory
  consumers: number
  version: number
  inFlightVersion: number | null
  timer?: ScheduledPoll
  idleSequence: number
}

const feedsByGateway = new WeakMap<SessionGateway, Map<string, ResourceFeedEntry>>()
let idleSequence = 0

function isCurrent(entry: ResourceFeedEntry, version: number): boolean {
  return entry.version === version && entry.consumers > 0
}

function clearScheduledPoll(entry: ResourceFeedEntry): void {
  if (!entry.timer) return
  clearTimeout(entry.timer.handle)
  entry.timer = undefined
}

function schedulePoll(entry: ResourceFeedEntry, gateway: SessionGateway, sessionId: string, version: number): void {
  if (!isCurrent(entry, version) || entry.inFlightVersion !== null || entry.timer) return

  const scheduled: ScheduledPoll = {
    handle: setTimeout(() => {
      if (entry.timer !== scheduled) return
      entry.timer = undefined
      if (isCurrent(entry, version)) void poll(entry, gateway, sessionId, version)
    }, RESOURCE_FEED_INTERVAL_MS),
    version,
  }
  entry.timer = scheduled
}

async function poll(
  entry: ResourceFeedEntry,
  gateway: SessionGateway,
  sessionId: string,
  version: number,
): Promise<void> {
  if (!isCurrent(entry, version) || entry.inFlightVersion !== null) return
  entry.inFlightVersion = version
  entry.loading.value = entry.snapshot.value === null

  try {
    const readResources = gateway.readResources
    if (!readResources) throw { code: 'UNSUPPORTED', messageKey: 'error.unknown' }
    const next = await readResources.call(gateway, sessionId)
    if (!isCurrent(entry, version)) return

    entry.snapshot.value = next
    entry.error.value = null
    entry.loading.value = false

    entry.history.push(next)
  } catch (cause) {
    if (!isCurrent(entry, version)) return
    entry.snapshot.value = null
    entry.error.value = cause
    entry.loading.value = false
  } finally {
    if (entry.inFlightVersion === version) {
      entry.inFlightVersion = null
      if (isCurrent(entry, version)) schedulePoll(entry, gateway, sessionId, version)
    }
  }
}

function startPolling(entry: ResourceFeedEntry, gateway: SessionGateway, sessionId: string): void {
  if (entry.consumers <= 0 || entry.inFlightVersion !== null || entry.timer) return
  void poll(entry, gateway, sessionId, entry.version)
}

function resetEntry(entry: ResourceFeedEntry): void {
  clearScheduledPoll(entry)
  entry.version += 1
  entry.inFlightVersion = null
  entry.snapshot.value = null
  entry.error.value = null
  entry.loading.value = true
  entry.history.reset()
}

function invalidateGeneration(entry: ResourceFeedEntry): void {
  clearScheduledPoll(entry)
  entry.version += 1
  entry.inFlightVersion = null
}

function pruneIdleEntries(entries: Map<string, ResourceFeedEntry>): void {
  const idle = [...entries.entries()]
    .filter(([, entry]) => entry.consumers === 0)
    .sort(([, left], [, right]) => left.idleSequence - right.idleSequence)

  while (idle.length > IDLE_FEED_LIMIT) {
    const oldest = idle.shift()
    if (oldest) entries.delete(oldest[0])
  }
}

function createEntry(): ResourceFeedEntry {
  return {
    snapshot: shallowRef<ResourceSnapshot | null>(null),
    error: shallowRef<unknown>(null),
    loading: ref(true),
    history: createResourceHistory(),
    consumers: 0,
    version: 0,
    inFlightVersion: null,
    idleSequence: 0,
  }
}

export function acquireResourceFeed(gateway: SessionGateway, sessionId: string): ResourceFeedLease {
  let entries = feedsByGateway.get(gateway)
  if (!entries) {
    entries = new Map<string, ResourceFeedEntry>()
    feedsByGateway.set(gateway, entries)
  }

  let entry = entries.get(sessionId)
  if (!entry) {
    entry = createEntry()
    entries.set(sessionId, entry)
  }

  entry.consumers += 1
  entry.idleSequence = 0
  const leaseVersion = entry.version
  startPolling(entry, gateway, sessionId)

  let released = false
  return {
    snapshot: entry.snapshot,
    error: entry.error,
    loading: entry.loading,
    history: entry.history,
    release(reset = false) {
      if (released) return
      released = true
      entry!.consumers = Math.max(0, entry!.consumers - 1)

      const resetCurrentGeneration = reset && (entry!.version === leaseVersion || entry!.consumers === 0)
      if (resetCurrentGeneration) resetEntry(entry!)
      else if (entry!.consumers === 0) invalidateGeneration(entry!)

      if (entry!.consumers > 0) {
        if (resetCurrentGeneration) startPolling(entry!, gateway, sessionId)
        return
      }

      entry!.idleSequence = ++idleSequence
      pruneIdleEntries(entries!)
    },
  }
}

export function resetResourceFeed(gateway: SessionGateway, sessionId: string): void {
  const entries = feedsByGateway.get(gateway)
  const entry = entries?.get(sessionId)
  if (!entry) return
  resetEntry(entry)
  if (entry.consumers === 0) entries!.delete(sessionId)
}
