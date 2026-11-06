import { computed, onBeforeUnmount, ref, watch, type CSSProperties, type Ref } from 'vue'
import type { SessionTab } from '../../types/qterm'
import { evenPixel } from '../../lib/geometry'
import { pointInRect, toRect, type DropRect } from './tabDrop'
import {
  paneDropTarget,
  splitDirectionFromStagePoint,
  stagePointPercent,
  tabDropTargetFromPoint,
  type SplitDirection,
} from './paneDrop'

export type { SplitDirection }
type SplitBranch = 'first' | 'second'
type Layout = string | { axis: 'x' | 'y'; first: Layout; second: Layout; ratio?: number }
interface PaneRect {
  id: string
  x: number
  y: number
  width: number
  height: number
}

export type PaneFocusDirection = 'left' | 'right' | 'up' | 'down'
export interface SplitResizeHandle {
  key: string
  path: SplitBranch[]
  axis: 'x' | 'y'
  x: number
  y: number
  width: number
  height: number
  ratio: number
}
interface DropTarget {
  id: string
  direction: SplitDirection | 'center'
}

const PANE_GAP_PX = 2
const MIN_SPLIT_RATIO = 0.12

function splitRatio(layout: Exclude<Layout, string>): number {
  return Math.min(1 - MIN_SPLIT_RATIO, Math.max(MIN_SPLIT_RATIO, layout.ratio ?? 0.5))
}

export function nearestPaneInDirection(
  list: readonly PaneRect[],
  current: PaneRect,
  direction: PaneFocusDirection,
): PaneRect | null {
  const EPSILON = 0.5
  const horizontal = direction === 'left' || direction === 'right'

  const forwardDistance = (rect: PaneRect): number => {
    const currentSize = horizontal ? current.width : current.height
    const rectSize = horizontal ? rect.width : rect.height

    if (currentSize <= EPSILON || rectSize <= EPSILON) {
      return (horizontal ? rect.x : rect.y) - (horizontal ? current.x : current.y)
    }
    if (direction === 'left') return current.x - (rect.x + rect.width)
    if (direction === 'right') return rect.x - (current.x + current.width)
    if (direction === 'up') return current.y - (rect.y + rect.height)
    return rect.y - (current.y + current.height)
  }
  const overlaps = (aStart: number, aSize: number, bStart: number, bSize: number): boolean =>
    Math.min(aStart + aSize, bStart + bSize) - Math.max(aStart, bStart) > EPSILON
  const perpendicularOverlap = (rect: PaneRect): boolean => {
    if (horizontal) {
      if (current.height <= EPSILON || rect.height <= EPSILON) return true
      return overlaps(current.y, current.height, rect.y, rect.height)
    }
    if (current.width <= EPSILON || rect.width <= EPSILON) return true
    return overlaps(current.x, current.width, rect.x, rect.width)
  }
  const centerDistance = (rect: PaneRect): number =>
    Math.hypot(
      rect.x + rect.width / 2 - (current.x + current.width / 2),
      rect.y + rect.height / 2 - (current.y + current.height / 2),
    )
  let best: PaneRect | null = null
  let bestScore = Number.POSITIVE_INFINITY
  let bestIndex = Number.POSITIVE_INFINITY
  list.forEach((rect, index) => {
    if (rect.id === current.id) return
    if (forwardDistance(rect) < -EPSILON) return
    if (!perpendicularOverlap(rect)) return
    const score = centerDistance(rect)
    if (score < bestScore - EPSILON || (Math.abs(score - bestScore) <= EPSILON && index < bestIndex)) {
      best = rect
      bestScore = score
      bestIndex = index
    }
  })
  return best
}

function rectangles(layout: Layout | null, x = 0, y = 0, width = 100, height = 100): PaneRect[] {
  if (!layout) return []
  if (typeof layout === 'string') return [{ id: layout, x, y, width, height }]
  const ratio = splitRatio(layout)
  return layout.axis === 'x'
    ? [
        ...rectangles(layout.first, x, y, width * ratio, height),
        ...rectangles(layout.second, x + width * ratio, y, width * (1 - ratio), height),
      ]
    : [
        ...rectangles(layout.first, x, y, width, height * ratio),
        ...rectangles(layout.second, x, y + height * ratio, width, height * (1 - ratio)),
      ]
}

function resizeHandles(
  layout: Layout | null,
  x = 0,
  y = 0,
  width = 100,
  height = 100,
  path: SplitBranch[] = [],
): SplitResizeHandle[] {
  if (!layout || typeof layout === 'string') return []
  const ratio = splitRatio(layout)
  const current: SplitResizeHandle = {
    key: path.length ? path.join('.') : 'root',
    path,
    axis: layout.axis,
    x,
    y,
    width,
    height,
    ratio,
  }
  return layout.axis === 'x'
    ? [
        current,
        ...resizeHandles(layout.first, x, y, width * ratio, height, [...path, 'first']),
        ...resizeHandles(layout.second, x + width * ratio, y, width * (1 - ratio), height, [...path, 'second']),
      ]
    : [
        current,
        ...resizeHandles(layout.first, x, y, width, height * ratio, [...path, 'first']),
        ...resizeHandles(layout.second, x, y + height * ratio, width, height * (1 - ratio), [...path, 'second']),
      ]
}

function withSplitRatio(layout: Layout, path: SplitBranch[], ratio: number): Layout {
  if (typeof layout === 'string') return layout
  if (path.length === 0) return { ...layout, ratio }
  const [branch, ...rest] = path
  return { ...layout, [branch!]: withSplitRatio(layout[branch!], rest, ratio) }
}

function replace(layout: Layout, id: string, replacement: Layout): Layout {
  if (typeof layout === 'string') return layout === id ? replacement : layout
  return { ...layout, first: replace(layout.first, id, replacement), second: replace(layout.second, id, replacement) }
}

function prune(layout: Layout | null, keep: Set<string>): Layout | null {
  if (!layout || typeof layout === 'string') return layout && keep.has(layout) ? layout : null
  const first = prune(layout.first, keep)
  const second = prune(layout.second, keep)
  return first && second ? { ...layout, first, second } : (first ?? second)
}

export function paneStyle(rect: PaneRect | undefined, size: { width: number; height: number }): CSSProperties {
  if (!rect) return {}

  const left = Math.round((rect.x * size.width) / 100)
  const top = evenPixel((rect.y * size.height) / 100)
  const right = Math.round(((rect.x + rect.width) * size.width) / 100)
  const bottom = evenPixel(((rect.y + rect.height) * size.height) / 100)
  return {
    left: `${left + PANE_GAP_PX}px`,
    top: `${top + PANE_GAP_PX}px`,
    width: `${Math.max(0, right - left - PANE_GAP_PX * 2)}px`,
    height: `${Math.max(0, bottom - top - PANE_GAP_PX * 2)}px`,
  }
}

export function useSplitWorkspace(
  sessions: () => SessionTab[],
  activeId: () => string | null,
  stage: Ref<HTMLElement | null>,
  select: (id: string) => void,

  closeSessions: (ids: string[]) => void,

  dock?: () => DOMRect | null,
) {
  const groups = ref<Record<string, Layout>>({})

  const activeGroupId = ref<string | null>(null)

  const layout = ref<Layout | null>(null)

  const groupOrders = ref<Record<string, number>>({})

  function sessionIndexOf(sessionID: string): number {
    const index = sessions().findIndex((session) => session.id === sessionID)
    return index < 0 ? 0 : index
  }

  function setGroupOrder(groupID: string, order: number): void {
    if (!groups.value[groupID]) return
    groupOrders.value[groupID] = order
  }

  const panes = computed(() => rectangles(layout.value))
  const splitHandles = computed(() => resizeHandles(layout.value))
  const isSplit = computed(() => panes.value.length > 1)

  const dragging = ref<string | null>(null)
  const target = ref<DropTarget | null>(null)

  const docking = ref(false)

  const selfSplit = ref<{ source: string; destination: string; direction: SplitDirection } | null>(null)
  let pending: { id: string; x: number; y: number; pointerId: number } | null = null
  let suppressClick = false
  let clickTimer: ReturnType<typeof setTimeout> | undefined

  function findGroup(sessionId: string): string | null {
    for (const [gid, gLayout] of Object.entries(groups.value)) {
      if (typeof gLayout === 'string') continue
      if (rectangles(gLayout).some((p) => p.id === sessionId)) return gid
    }
    return null
  }

  const splitGroups = computed(() =>
    Object.entries(groups.value).map(([gid, gLayout]) => ({
      id: gid,
      count: rectangles(gLayout).length,

      firstSessionId: rectangles(gLayout)[0]?.id ?? gid,
    })),
  )

  const standaloneSessions = computed(() => sessions().filter((s) => !findGroup(s.id)))

  function saveGroup(): void {
    if (activeGroupId.value && layout.value && typeof layout.value !== 'string') {
      groups.value[activeGroupId.value] = layout.value
    }
  }

  function showGroup(gid: string): void {
    if (!groups.value[gid]) return
    if (activeGroupId.value === gid) {
      const panes = rectangles(groups.value[gid])
      if (panes[0]) select(panes[0].id)
      return
    }
    saveGroup()
    activeGroupId.value = gid
    layout.value = groups.value[gid]
    const first = rectangles(layout.value!)[0]
    if (first) select(first.id)
  }

  function groupSessionIds(gid: string): string[] {
    const groupLayout = groups.value[gid]
    if (!groupLayout || typeof groupLayout === 'string') return []
    return rectangles(groupLayout).map((p) => p.id)
  }

  function closeGroup(gid: string): void {
    const groupLayout = groups.value[gid]
    if (!groupLayout || typeof groupLayout === 'string') return
    const ids = rectangles(groupLayout).map((p) => p.id)

    if (activeGroupId.value === gid) activeGroupId.value = null
    closeSessions(ids)
  }

  watch(
    [() => sessions().map((s) => s.id), activeId],
    ([ids, active], previous) => {
      const keep = new Set(ids)

      for (const [gid, gLayout] of Object.entries(groups.value)) {
        const pruned = prune(gLayout, keep)
        if (!pruned || typeof pruned === 'string') {
          delete groups.value[gid]
          if (activeGroupId.value === gid) activeGroupId.value = null
        } else {
          groups.value[gid] = pruned as Layout
          if (activeGroupId.value === gid) layout.value = pruned as Layout
        }
      }

      layout.value = prune(layout.value, keep)

      if (dragging.value && !keep.has(dragging.value)) cancelDrag()
      if (!active || !keep.has(active)) {
        if (layout.value && panes.value[0]) select(panes.value[0].id)
        return
      }

      const gid = findGroup(active)
      if (gid) {
        if (activeGroupId.value !== gid) {
          saveGroup()
          activeGroupId.value = gid
          layout.value = groups.value[gid]
        }
        return
      }

      if (panes.value.some((p) => p.id === active)) return

      const oldActive = previous?.[1]
      if (oldActive && !keep.has(oldActive) && panes.value.length > 0) {
        select(panes.value[0].id)
        return
      }

      saveGroup()
      activeGroupId.value = null
      layout.value = active
    },
    { immediate: true },
  )

  function validSplit(next: Layout | null, source: string, destination: string): Layout | null {
    if (!next || typeof next === 'string') return null
    const ids = rectangles(next).map((pane) => pane.id)
    if (ids.length < 2 || !ids.includes(source) || !ids.includes(destination)) return null
    return next
  }

  function evictFromOtherGroups(sessionID: string, keepGroupID: string): void {
    for (const [gid, gLayout] of Object.entries(groups.value)) {
      if (gid === keepGroupID || typeof gLayout === 'string') continue
      if (!rectangles(gLayout).some((pane) => pane.id === sessionID)) continue
      const remaining = new Set(
        rectangles(gLayout)
          .map((pane) => pane.id)
          .filter((id) => id !== sessionID),
      )
      const pruned = prune(gLayout, remaining)
      if (pruned && typeof pruned !== 'string') groups.value[gid] = pruned
      else delete groups.value[gid]
      if (activeGroupId.value === gid) activeGroupId.value = null
    }
  }

  function droppedLayout(source: string, drop: DropTarget, base: Layout | null = layout.value): Layout | null {
    if (!base || source === drop.id || !sessions().some((s) => s.id === source)) return null
    if (drop.direction === 'center') {
      const swap = (node: Layout): Layout =>
        typeof node === 'string'
          ? node === drop.id
            ? source
            : node === source
              ? drop.id
              : node
          : { ...node, first: swap(node.first), second: swap(node.second) }
      return validSplit(swap(base), source, drop.id)
    }
    const basePanes = rectangles(base).map((pane) => pane.id)
    const pruned = prune(base, new Set(basePanes.filter((id) => id !== source)))
    if (!pruned) return null
    const before = drop.direction === 'left' || drop.direction === 'top'
    return validSplit(
      replace(pruned, drop.id, {
        axis: drop.direction === 'left' || drop.direction === 'right' ? 'x' : 'y',
        first: before ? source : drop.id,
        second: before ? drop.id : source,
        ratio: 0.5,
      }),
      source,
      drop.id,
    )
  }

  const preview = computed(() => {
    if (!dragging.value || !target.value) return undefined
    return rectangles(droppedLayout(dragging.value, target.value, layout.value)).find((p) => p.id === dragging.value)
  })

  function splitWith(destination: string, source: string, direction: SplitDirection): boolean {
    if (!destination || !source || destination === source) return false
    const all = sessions()
    if (!all.some((s) => s.id === destination) || !all.some((s) => s.id === source)) return false
    const destinationGroup = findGroup(destination)
    const base: Layout | null = destinationGroup ? (groups.value[destinationGroup] ?? null) : destination
    const next = droppedLayout(source, { id: destination, direction }, base)
    if (!next) return false

    saveGroup()
    const gid = destinationGroup ?? destination
    evictFromOtherGroups(source, gid)
    groups.value[gid] = next
    if (groupOrders.value[gid] === undefined) groupOrders.value[gid] = sessionIndexOf(gid)
    activeGroupId.value = gid
    layout.value = next
    select(source)
    return true
  }

  function single(id: string): void {
    const gid = findGroup(id)
    if (gid) {
      const groupLayout = groups.value[gid]
      const members = rectangles(groupLayout)
      if (members.length <= 2) {
        delete groups.value[gid]
      } else {
        const remaining = new Set(members.map((pane) => pane.id).filter((sessionId) => sessionId !== id))
        const next = prune(groupLayout, remaining)
        if (next && typeof next !== 'string') groups.value[gid] = next
      }
      if (activeGroupId.value === gid) activeGroupId.value = null
    }
    layout.value = id
    select(id)
  }

  function remove(id: string): void {
    const remaining = panes.value.map((p) => p.id).filter((v) => v !== id)

    if (activeGroupId.value) {
      const pruned = prune(layout.value!, new Set(remaining))
      if (pruned && typeof pruned !== 'string') {
        groups.value[activeGroupId.value] = pruned
        layout.value = pruned
        const first = rectangles(layout.value)[0]
        if (activeId() === id && first) select(first.id)
        return
      }

      delete groups.value[activeGroupId.value]
      activeGroupId.value = null
    }

    layout.value = prune(layout.value, new Set(remaining))
    const first = rectangles(layout.value)[0]
    if (activeId() === id && first) select(first.id)
  }

  function restoreSplit(): void {
    if (activeGroupId.value && groups.value[activeGroupId.value]) {
      layout.value = groups.value[activeGroupId.value]
      const first = rectangles(layout.value)[0]
      if (first) select(first.id)
    }
  }

  function setSplitRatio(path: SplitBranch[], ratio: number): void {
    if (!layout.value || typeof layout.value === 'string') return
    const next = withSplitRatio(layout.value, path, Math.min(1 - MIN_SPLIT_RATIO, Math.max(MIN_SPLIT_RATIO, ratio)))
    layout.value = next
    if (activeGroupId.value) groups.value[activeGroupId.value] = next
  }

  function click(id: string): void {
    if (!suppressClick) select(id)
  }

  function cancelDrag(): void {
    pending = null
    dragging.value = null
    target.value = null
    docking.value = false
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', finish)
    window.removeEventListener('pointercancel', cancelDrag)
    window.removeEventListener('blur', cancelDrag)
    window.removeEventListener('keydown', escape, true)
  }

  function escape(event: KeyboardEvent): void {
    if (event.key !== 'Escape') return
    event.preventDefault()
    event.stopImmediatePropagation()
    cancelDrag()
  }

  function start(event: PointerEvent, id: string): void {
    if (event.button !== 0 || (event.target as HTMLElement | null)?.closest?.('button')) return
    cancelDrag()
    pending = { id, x: event.clientX, y: event.clientY, pointerId: event.pointerId }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', finish)
    window.addEventListener('pointercancel', cancelDrag)
    window.addEventListener('blur', cancelDrag)
    window.addEventListener('keydown', escape, true)
  }

  function updateTargetFromPoint(sourceId: string, clientX: number, clientY: number): void {
    dragging.value = sourceId
    const bounds = stage.value?.getBoundingClientRect()
    target.value = null
    if (!bounds || !bounds.width || !bounds.height) return
    target.value = paneDropTarget(panes.value, sourceId, stagePointPercent(bounds, { x: clientX, y: clientY }))
  }

  function move(event: PointerEvent): void {
    if (!pending || event.pointerId !== pending.pointerId) return
    if (!dragging.value && Math.hypot(event.clientX - pending.x, event.clientY - pending.y) < 6) return
    event.preventDefault()
    const dockRect: DropRect | null = toRect(dock?.() ?? null)
    if (dockRect && pointInRect(dockRect, { x: event.clientX, y: event.clientY })) {
      dragging.value = pending.id
      target.value = null
      docking.value = true
      return
    }
    docking.value = false
    updateTargetFromPoint(pending.id, event.clientX, event.clientY)
  }

  function directionFromPoint(clientX: number, clientY: number): SplitDirection | null {
    const bounds = stage.value?.getBoundingClientRect()
    if (!bounds || !bounds.width || !bounds.height) return null
    return splitDirectionFromStagePoint(stagePointPercent(bounds, { x: clientX, y: clientY }))
  }

  function planSelfSplit(sourceId: string, clientX: number, clientY: number): boolean {
    if (sourceId !== activeId()) return false
    const direction = directionFromPoint(clientX, clientY)
    if (!direction) return false
    selfSplit.value = { source: sourceId, destination: sourceId, direction }
    dragging.value = sourceId
    target.value = null
    return true
  }

  function commitSelfSplit(): { destination: string; direction: SplitDirection } | null {
    const plan = selfSplit.value
    selfSplit.value = null
    if (!plan) return null
    dragging.value = null
    target.value = null
    return { destination: plan.destination, direction: plan.direction }
  }

  function dragTabIntoStage(sourceId: string, clientX: number, clientY: number): void {
    if (!sessions().some((item) => item.id === sourceId)) return
    selfSplit.value = null
    dragging.value = sourceId

    const bounds = stage.value?.getBoundingClientRect()
    target.value = bounds
      ? tabDropTargetFromPoint({
          sourceId,
          pointPercent: stagePointPercent(bounds, { x: clientX, y: clientY }),
          panes: panes.value,
          stageSize: { width: bounds.width, height: bounds.height },
        })
      : null
    if (!target.value && planSelfSplit(sourceId, clientX, clientY)) return
  }

  function dropTabIntoStage(): boolean {
    const source = dragging.value
    const drop = target.value
    const usable = source && drop && panes.value.some((pane) => pane.id === drop.id)
    const next = usable ? droppedLayout(source, drop) : null
    if (!next || !source || !drop) {
      cancelTabDrop()
      return false
    }
    saveGroup()
    const gid = findGroup(drop.id) || activeGroupId.value || drop.id
    evictFromOtherGroups(source, gid)
    groups.value[gid] = next
    if (groupOrders.value[gid] === undefined) groupOrders.value[gid] = sessionIndexOf(gid)
    layout.value = next
    if (!activeGroupId.value || !groups.value[activeGroupId.value]) activeGroupId.value = gid
    select(source)
    cancelTabDrop()
    return true
  }

  function cancelTabDrop(): void {
    dragging.value = null
    target.value = null
    selfSplit.value = null
  }

  function finish(event: PointerEvent): void {
    if (!pending || event.pointerId !== pending.pointerId) return
    if (dragging.value) {
      move(event)
      const source = dragging.value
      if (docking.value && source) {
        docking.value = false
        single(source)
        suppressClick = true
        clearTimeout(clickTimer)
        clickTimer = setTimeout(() => {
          suppressClick = false
        }, 0)
        cancelDrag()
        return
      }
      const next = target.value && source ? droppedLayout(source, target.value) : null
      if (next && source) {
        saveGroup()
        const gid = findGroup(source) || activeGroupId.value || target.value!.id
        groups.value[gid] = next
        layout.value = next
        if (!activeGroupId.value || !groups.value[activeGroupId.value]) {
          activeGroupId.value = gid
        }
        select(source)
      }
      suppressClick = true
      clearTimeout(clickTimer)
      clickTimer = setTimeout(() => {
        suppressClick = false
      }, 0)
    }
    cancelDrag()
  }

  onBeforeUnmount(() => {
    cancelDrag()
    clearTimeout(clickTimer)
  })

  return {
    panes,
    splitHandles,
    isSplit,
    splitGroups,
    standaloneSessions,
    activeGroupId,
    activeGroupSessionIds: computed(() => (isSplit.value ? panes.value.map((pane) => pane.id) : [])),
    dragging,
    target,
    docking,
    groupOrders,
    setGroupOrder,
    selfSplit,
    preview,
    splitWith,
    single,
    remove,
    start,
    dragTabIntoStage,
    dropTabIntoStage,
    commitSelfSplit,
    cancelTabDrop,
    click,
    showGroup,
    closeGroup,
    groupSessionIds,
    restoreSplit,
    setSplitRatio,
  }
}
