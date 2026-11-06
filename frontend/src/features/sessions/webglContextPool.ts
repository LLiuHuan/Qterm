export const MAX_WEBGL_CONTEXTS = 12

type WebglContextOwner = object

interface WebglContextState {
  visible: boolean
  active: boolean
}

interface WebglContextLease extends WebglContextState {
  release: () => void
  lastUsed: number
}

const leases = new Map<WebglContextOwner, WebglContextLease>()
let usageClock = 0

function evictionPriority(lease: WebglContextLease): number {
  if (!lease.visible) return 0
  if (!lease.active) return 1
  return 2
}

function enforceCapacity(): void {
  while (leases.size > MAX_WEBGL_CONTEXTS) {
    const victim = [...leases.entries()].sort(([, left], [, right]) => {
      const statePriority = evictionPriority(left) - evictionPriority(right)
      return statePriority || left.lastUsed - right.lastUsed
    })[0]
    if (!victim) return
    const [owner, lease] = victim
    leases.delete(owner)
    try {
      lease.release()
    } catch {}
  }
}

export function retainWebglContext(owner: WebglContextOwner, release: () => void, state: WebglContextState): void {
  const existing = leases.get(owner)
  const becameVisible = state.visible && !existing?.visible
  const becameActive = state.active && !existing?.active
  leases.set(owner, {
    release,
    ...state,
    lastUsed: !existing || becameVisible || becameActive ? ++usageClock : existing.lastUsed,
  })
  enforceCapacity()
}

export function updateWebglContext(owner: WebglContextOwner, state: WebglContextState): void {
  const existing = leases.get(owner)
  if (!existing) return
  retainWebglContext(owner, existing.release, state)
}

export function forgetWebglContext(owner: WebglContextOwner): void {
  leases.delete(owner)
}

export function hasOtherWebglOwner(owner: WebglContextOwner): boolean {
  for (const key of leases.keys()) {
    if (key !== owner) return true
  }
  return false
}
