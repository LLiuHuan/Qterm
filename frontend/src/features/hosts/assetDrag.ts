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

const DROP_ROW_SELECTOR = '.host-row, .asset-group-title, [data-asset-card]'

export const ASSET_DRAG_RETURN_MS = 520

interface CardSlot {
  x: number
  card: HTMLElement
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
  if (target?.kind === 'host' && position === 'inside') return null
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

function clamp01(value: number): number {
  return Math.min(1, Math.max(0, value))
}

export interface AssetDragOptions {
  cardGrid?: boolean
}

export function useAssetDrag(
  container: Ref<HTMLElement | null>,
  parentOf: (item: AssetDragItem) => string,
  onDrop: (item: AssetDragItem, drop: AssetDropTarget) => void,
  options: AssetDragOptions = {},
) {
  const CARD_SELECTOR = '[data-asset-card]'

  const CARD_SLOT_BAND = 16
  const state = reactive({
    active: false,
    label: '',

    item: null as AssetDragItem | null,
    x: 0,
    y: 0,

    anchorX: 0.5,
    anchorY: 0.5,

    sourceWidth: 0,

    returnTo: null as null | { left: number; top: number; width: number; height: number },
    valid: false,

    line: null as null | { top: number; left: number; width: number; vertical?: boolean },
  })
  let pending: { item: AssetDragItem; row: HTMLElement; pointerId: number; x: number; y: number } | null = null
  let target: AssetDropTarget | null = null
  let highlighted: HTMLElement | null = null
  let activeContainer: HTMLElement | null = null
  let frame = 0
  let expandTimer: ReturnType<typeof setTimeout> | undefined
  let expandRow: HTMLElement | null = null
  let clickTimer: ReturnType<typeof setTimeout> | undefined
  let returnTimer: ReturnType<typeof setTimeout> | undefined
  let returningRow: HTMLElement | null = null

  function markReturningRow(row: HTMLElement) {
    clearTimeout(returnTimer)
    returningRow?.removeAttribute('data-asset-drag-returning')
    returningRow = row
    row.setAttribute('data-asset-drag-returning', '')
    returnTimer = setTimeout(() => {
      row.removeAttribute('data-asset-drag-returning')
      row.removeAttribute('data-asset-drag-source')
      if (returningRow === row) returningRow = null
    }, ASSET_DRAG_RETURN_MS)
  }

  function clearReturningRow() {
    clearTimeout(returnTimer)
    returnTimer = undefined
    if (!returningRow) return
    returningRow.removeAttribute('data-asset-drag-returning')
    returningRow.removeAttribute('data-asset-drag-source')
    returningRow = null
  }

  function rowAt(element: Element | null): HTMLElement | null {
    return element?.closest<HTMLElement>(DROP_ROW_SELECTOR) ?? null
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

  function cardSlotAt(
    list: HTMLElement,
    x: number,
    y: number,
  ): { card: HTMLElement; rect: DOMRect; slot: CardSlot | null } | null {
    const entries = Array.from(list.querySelectorAll<HTMLElement>(CARD_SELECTOR))
      .map((card) => ({ card, rect: card.getBoundingClientRect() }))
      .filter((entry) => entry.rect.width > 0 && entry.rect.height > 0)
    if (!entries.length) return null

    const rows: Array<typeof entries> = []
    for (const entry of entries) {
      const row = rows.find((candidate) => Math.abs(candidate[0].rect.top - entry.rect.top) < 1)
      if (row) row.push(entry)
      else rows.push([entry])
    }

    let bestRow: typeof entries | null = null
    let bestDistance = Number.POSITIVE_INFINITY
    for (const row of rows) {
      const top = Math.min(...row.map((entry) => entry.rect.top))
      const bottom = Math.max(...row.map((entry) => entry.rect.bottom))
      const distance = y < top ? top - y : y > bottom ? y - bottom : 0
      if (distance < bestDistance) {
        bestDistance = distance
        bestRow = row
      }
    }
    if (!bestRow || bestDistance > CARD_SLOT_BAND) return null
    const line = [...bestRow].sort((left, right) => left.rect.left - right.rect.left)

    const slots: CardSlot[] = []
    const first = line[0]
    const last = line[line.length - 1]
    const gap = line.length > 1 ? Math.max(line[1].rect.left - line[0].rect.right, 0) : CARD_SLOT_BAND * 2
    slots.push({ x: first.rect.left - gap / 2, card: first.card, position: 'before' })
    for (let index = 0; index < line.length - 1; index += 1) {
      const current = line[index]
      const next = line[index + 1]
      slots.push({ x: (current.rect.right + next.rect.left) / 2, card: next.card, position: 'before' })
    }
    slots.push({ x: last.rect.right + gap / 2, card: last.card, position: 'after' })
    let nearest: CardSlot | null = null
    for (const slot of slots) {
      if (Math.abs(x - slot.x) > CARD_SLOT_BAND) continue
      if (!nearest || Math.abs(x - slot.x) < Math.abs(x - nearest.x)) nearest = slot
    }
    if (nearest) {
      const anchor = line.find((entry) => entry.card === nearest?.card) ?? first
      return { card: anchor.card, rect: anchor.rect, slot: nearest }
    }

    const hit = line.find((entry) => x >= entry.rect.left && x <= entry.rect.right)
    return hit ? { card: hit.card, rect: hit.rect, slot: null } : null
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

    const cardHit = options.cardGrid === true ? cardSlotAt(list, state.x, state.y) : null
    const row = options.cardGrid === true ? (cardHit?.card ?? null) : rowAt(element)
    const visibleRows = Array.from(list.querySelectorAll<HTMLElement>(DROP_ROW_SELECTOR)).filter(
      (entry) => entry.getClientRects().length > 0,
    )
    const lastBottom = Math.max(contentBounds.top, ...visibleRows.map((entry) => entry.getBoundingClientRect().bottom))

    if (!row && options.cardGrid === true) {
      cancelExpand()
      return
    }

    if (!row && state.y < lastBottom) {
      cancelExpand()
      return
    }

    const lastRoot = Array.from(list.children)
      .filter((node) => node instanceof HTMLElement && node.dataset.assetId)
      .at(-1)

    if (!row) {
      const anchor = lastRoot instanceof HTMLElement ? itemAt(lastRoot) : null
      target = anchor ? { groupId: parentOf(anchor), targetId: anchor.id, position: 'after' } : null
      state.valid = target !== null
      if (!target) {
        cancelExpand()
        return
      }
      state.line = {
        top: lastBottom - contentBounds.top + list.scrollTop,
        left: 0,
        width: contentBounds.width,
      }
      return
    }

    const targetItem = itemAt(row)
    const rect = row.getBoundingClientRect()
    const isCard = row.hasAttribute('data-asset-card')

    const cardLayout = isCard || options.cardGrid === true

    const cardSlot = cardLayout ? (cardHit?.slot ?? null) : null
    const position = ((): DropPosition | null => {
      if (!cardLayout) {
        const fraction = (state.y - rect.top) / rect.height
        return targetItem?.kind === 'group' && fraction > 0.25 && fraction < 0.75
          ? 'inside'
          : fraction < 0.5
            ? 'before'
            : 'after'
      }
      if (cardSlot) return cardSlot.position
      return targetItem?.kind === 'group' ? 'inside' : null
    })()
    if (!position) {
      cancelExpand()
      return
    }

    if (cardLayout && position !== 'inside' && targetItem && targetItem.kind !== pending.item.kind) {
      cancelExpand()
      return
    }
    target = resolveAssetDrop(pending.item, targetItem, position, parentOf)
    state.valid = target !== null
    if (!target) {
      cancelExpand()
      return
    }
    if (position === 'inside') {
      highlighted = row
      row.setAttribute('data-asset-drop-inside', '')
      if (expandRow !== row) {
        cancelExpand()
        expandRow = row
        expandTimer = setTimeout(() => row.dispatchEvent(new Event('asset-drag-expand')), 600)
      }
      return
    }
    cancelExpand()
    if (cardLayout) {
      const x = cardSlot?.x ?? (position === 'before' ? rect.left : rect.right)
      state.line = {
        top: rect.top - contentBounds.top + list.scrollTop - 2,
        left: Math.max(0, x - contentBounds.left),
        width: rect.height + 4,
        vertical: true,
      }
      return
    }
    const y = position === 'before' ? rect.top : rect.bottom
    const left = Math.max(0, rect.left - contentBounds.left)
    state.line = { top: y - contentBounds.top + list.scrollTop, left, width: contentBounds.width - left }
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

  function finish(returnRect?: DOMRect) {
    const wasActive = state.active
    const sourceRow = pending?.row ?? null
    pending = null

    clearReturningRow()
    if (returnRect) {
      if (sourceRow) markReturningRow(sourceRow)
    } else {
      sourceRow?.removeAttribute('data-asset-drag-source')
    }
    clearTarget()
    cancelExpand()
    cancelAnimationFrame(frame)
    document.body.classList.remove('asset-dragging')
    state.active = false
    if (returnRect) {
      state.returnTo = {
        left: returnRect.left,
        top: returnRect.top,
        width: returnRect.width,
        height: returnRect.height,
      }
    } else {
      state.returnTo = null
      state.item = null
    }
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

    const rowRect = pending.row.getBoundingClientRect()
    const drop = state.active ? target : null
    if (drop) {
      finish()
      onDrop(item, drop)
      return
    }

    finish(state.active ? rowRect : undefined)
  }
  function cancel() {
    const rect = state.active && pending ? pending.row.getBoundingClientRect() : undefined
    finish(rect)
  }
  function keydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      event.preventDefault()
      finish()
    }
  }
  function start(event: PointerEvent) {
    if (event.button !== 0 || event.isPrimary === false || pending) return

    clearReturningRow()
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
    state.item = item
    state.returnTo = null

    const sourceRect = row.getBoundingClientRect()
    state.anchorX = sourceRect.width > 0 ? clamp01((event.clientX - sourceRect.left) / sourceRect.width) : 0.5
    state.anchorY = sourceRect.height > 0 ? clamp01((event.clientY - sourceRect.top) / sourceRect.height) : 0.5
    state.sourceWidth = sourceRect.width

    state.label = row.querySelector('.host-name, .asset-group-name')?.textContent ?? row.dataset.assetLabel ?? ''
    document.addEventListener('pointermove', move, { capture: true, passive: false })
    document.addEventListener('pointerup', end, true)
    document.addEventListener('pointercancel', cancel, true)
    document.addEventListener('keydown', keydown, true)
    window.addEventListener('blur', cancel)
  }

  function clearReturn() {
    state.returnTo = null
    state.item = null
  }
  onBeforeUnmount(() => {
    finish()
    clearReturningRow()
    clearTimeout(clickTimer)
    document.removeEventListener('click', suppressClick, true)
  })
  return { state, start, clearReturn }
}
