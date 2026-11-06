import { onBeforeUnmount, reactive, type Ref } from 'vue'

export type AssetDragKind = 'host' | 'group'
export interface AssetDragItem {
  kind: AssetDragKind
  id: string
}
export type DropPosition = 'before' | 'after' | 'inside'
export interface AssetDropTarget {
  groupId: string
  targetId: string
  position: DropPosition
}

export function resolveAssetDrop(
  item: AssetDragItem,
  target: AssetDragItem | null,
  position: DropPosition,
  parentOf: (item: AssetDragItem) => string,
): AssetDropTarget | null {
  if (target?.kind === item.kind && target.id === item.id) return null
  const groupId = target ? (target.kind === 'group' && position === 'inside' ? target.id : parentOf(target)) : ''
  if (groupId === parentOf(item) && position === 'inside') return null
  if (item.kind === 'group') {
    if (item.id === 'default') return null
    const visited = new Set<string>()
    let ancestor = groupId
    while (ancestor) {
      if (ancestor === item.id || visited.has(ancestor)) return null
      visited.add(ancestor)
      ancestor = parentOf({ kind: 'group', id: ancestor })
    }
  }
  return { groupId, targetId: target?.id ?? '', position }
}

export function useAssetDrag(
  container: Ref<HTMLElement | null>,
  parentOf: (item: AssetDragItem) => string,
  onDrop: (item: AssetDragItem, drop: AssetDropTarget) => void,
) {
  const state = reactive({
    active: false,
    label: '',
    x: 0,
    y: 0,
    valid: false,
    line: null as null | { top: number; left: number; width: number },
  })
  let pending: { item: AssetDragItem; row: HTMLElement; pointerId: number; x: number; y: number } | null = null
  let target: AssetDropTarget | null = null
  let highlighted: HTMLElement | null = null
  let activeContainer: HTMLElement | null = null
  let frame = 0
  let expandTimer: ReturnType<typeof setTimeout> | undefined
  let expandRow: HTMLElement | null = null
  let clickTimer: ReturnType<typeof setTimeout> | undefined

  function rowAt(element: Element | null): HTMLElement | null {
    return element?.closest<HTMLElement>('.host-row, .asset-group-title') ?? null
  }
  function itemAt(row: HTMLElement): AssetDragItem | null {
    const shell = row.closest<HTMLElement>('[data-asset-kind][data-asset-id]')
    const kind = shell?.dataset.assetKind
    const id = shell?.dataset.assetId
    return id && (kind === 'host' || kind === 'group') ? { kind, id } : null
  }
  function clearTarget() {
    highlighted?.removeAttribute('data-asset-drop-inside')
    highlighted = null
    target = null
    state.valid = false
    state.line = null
  }
  function cancelExpand() {
    clearTimeout(expandTimer)
    expandTimer = undefined
    expandRow = null
  }
  function refreshTarget() {
    clearTarget()
    const list = activeContainer ?? container.value
    if (!list || !pending) return
    const viewport = list.closest<HTMLElement>('[data-slot="scroll-area-viewport"]') ?? list
    const bounds = viewport.getBoundingClientRect()
    const contentBounds = list.getBoundingClientRect()
    if (state.x < bounds.left || state.x > bounds.right || state.y < bounds.top || state.y > bounds.bottom) {
      cancelExpand()
      return
    }
    const element = document.elementFromPoint(state.x, state.y)
    if (!element || !viewport.contains(element)) {
      cancelExpand()
      return
    }
    const row = rowAt(element)
    const visibleRows = Array.from(list.querySelectorAll<HTMLElement>('.host-row, .asset-group-title')).filter(
      (entry) => entry.getClientRects().length > 0,
    )
    const lastBottom = Math.max(contentBounds.top, ...visibleRows.map((entry) => entry.getBoundingClientRect().bottom))

    if (!row && state.y < lastBottom) {
      cancelExpand()
      return
    }

    const lastRoot = Array.from(list.children)
      .filter((node) => node instanceof HTMLElement && node.dataset.assetId)
      .at(-1)
    const targetItem = row ? itemAt(row) : lastRoot instanceof HTMLElement ? itemAt(lastRoot) : null
    let position: DropPosition = 'after'
    if (row) {
      const rect = row.getBoundingClientRect()
      const fraction = (state.y - rect.top) / rect.height
      position =
        targetItem?.kind === 'group' && fraction > 0.25 && fraction < 0.75
          ? 'inside'
          : fraction < 0.5
            ? 'before'
            : 'after'
    }
    target = resolveAssetDrop(pending.item, targetItem, position, parentOf)
    state.valid = target !== null
    if (!target) {
      cancelExpand()
      return
    }
    if (row && position === 'inside') {
      highlighted = row
      row.setAttribute('data-asset-drop-inside', '')
      if (expandRow !== row) {
        cancelExpand()
        expandRow = row
        expandTimer = setTimeout(() => row.dispatchEvent(new Event('asset-drag-expand')), 600)
      }
    } else {
      cancelExpand()
      const rect = row?.getBoundingClientRect()
      const y = rect ? (position === 'before' ? rect.top : rect.bottom) : lastBottom
      const left = rect ? Math.max(0, rect.left - contentBounds.left) : 0
      state.line = { top: y - contentBounds.top + list.scrollTop, left, width: contentBounds.width - left }
    }
  }
  function scrollStep() {
    if (!state.active) return
    const content = activeContainer ?? container.value
    if (!content) return
    const list = content.closest<HTMLElement>('[data-slot="scroll-area-viewport"]') ?? content
    const rect = list.getBoundingClientRect()
    if (state.x >= rect.left && state.x <= rect.right && state.y >= rect.top && state.y <= rect.bottom) {
      const edge = 32
      const delta = state.y < rect.top + edge ? -8 : state.y > rect.bottom - edge ? 8 : 0
      const before = list.scrollTop
      if (delta) list.scrollTop += delta
      if (list.scrollTop !== before) refreshTarget()
    }
    frame = requestAnimationFrame(scrollStep)
  }
  function move(event: PointerEvent) {
    if (!pending || event.pointerId !== pending.pointerId) return
    state.x = event.clientX
    state.y = event.clientY
    if (!state.active) {
      if (Math.hypot(state.x - pending.x, state.y - pending.y) < 5) return
      state.active = true
      pending.row.setAttribute('data-asset-drag-source', '')
      document.body.classList.add('asset-dragging')
      window.getSelection()?.removeAllRanges()
      frame = requestAnimationFrame(scrollStep)
    }
    event.preventDefault()
    refreshTarget()
  }
  function suppressClick(event: MouseEvent) {
    event.preventDefault()
    event.stopImmediatePropagation()
    document.removeEventListener('click', suppressClick, true)
  }
  function finish() {
    const wasActive = state.active
    pending?.row.removeAttribute('data-asset-drag-source')
    pending = null
    clearTarget()
    cancelExpand()
    cancelAnimationFrame(frame)
    document.body.classList.remove('asset-dragging')
    state.active = false
    document.removeEventListener('pointermove', move, true)
    document.removeEventListener('pointerup', end, true)
    document.removeEventListener('pointercancel', cancel, true)
    document.removeEventListener('keydown', keydown, true)
    window.removeEventListener('blur', cancel)
    activeContainer = null
    if (wasActive) {
      document.addEventListener('click', suppressClick, true)
      clearTimeout(clickTimer)
      clickTimer = setTimeout(() => document.removeEventListener('click', suppressClick, true), 300)
    }
  }
  function end(event: PointerEvent) {
    if (!pending || event.pointerId !== pending.pointerId) return
    if (state.active) {
      state.x = event.clientX
      state.y = event.clientY
      refreshTarget()
    }
    const item = pending.item
    const drop = state.active ? target : null
    finish()
    if (drop) onDrop(item, drop)
  }
  function cancel() {
    finish()
  }
  function keydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      event.preventDefault()
      finish()
    }
  }
  function start(event: PointerEvent) {
    if (event.button !== 0 || event.isPrimary === false || pending) return
    const element = event.target instanceof Element ? event.target : null
    if (element?.closest('[data-sortable-ignore], input, textarea')) return
    const row = rowAt(element)
    if (!row) return

    const rowContainer = row.closest<HTMLElement>('[data-slot="scroll-area-viewport"]')
    activeContainer = container.value ?? rowContainer
    if (!activeContainer?.contains(row)) {
      activeContainer = null
      return
    }
    const item = itemAt(row)
    if (!item || (item.kind === 'group' && item.id === 'default')) return
    pending = { item, row, pointerId: event.pointerId, x: event.clientX, y: event.clientY }
    state.label = row.querySelector('.host-name, .asset-group-name')?.textContent ?? ''
    document.addEventListener('pointermove', move, { capture: true, passive: false })
    document.addEventListener('pointerup', end, true)
    document.addEventListener('pointercancel', cancel, true)
    document.addEventListener('keydown', keydown, true)
    window.addEventListener('blur', cancel)
  }
  onBeforeUnmount(() => {
    finish()
    clearTimeout(clickTimer)
    document.removeEventListener('click', suppressClick, true)
  })
  return { state, start }
}
