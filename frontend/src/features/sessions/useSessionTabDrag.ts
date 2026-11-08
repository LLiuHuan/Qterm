import { computed, onBeforeUnmount, ref, type CSSProperties, type Ref } from 'vue'
import { decideTabDrop, isSplitDropCandidate, toRect } from './tabDrop'
import type { SplitDirection } from './paneDrop'
import type { translate } from '@/locales'

type ReadonlyRef<T> = { readonly value: T }

type TabDragKind = 'session' | 'group' | 'sftp' | 'rdp'

interface TabReorderEvent {
  sourceId: string
  targetId: string
  position: 'before' | 'after'
}

interface DragGhost {
  title: string
  x: number
  y: number
  hint: string
}

interface SessionTabDragOptions {
  stage: Ref<HTMLElement | null>
  tabBar: Ref<HTMLElement | null>
  t: typeof translate

  tabOrderOf: (id: string) => number | null

  tabTitle: (id: string) => string
  selfSplit: ReadonlyRef<{ source: string } | null>
  dragTabIntoStage: (sourceId: string, clientX: number, clientY: number) => void
  dropTabIntoStage: () => boolean
  commitSelfSplit: () => { destination: string; direction: SplitDirection } | null
  cancelTabDrop: () => void
  click: (sessionId: string) => void
  openSessionWindow: (sessionId: string) => void

  reorderTabs: (event: TabReorderEvent) => void

  requestSplit: (destination: string, direction: SplitDirection) => void
}

export function useSessionTabDrag(options: SessionTabDragOptions) {
  const {
    stage,
    tabBar,
    t,
    tabOrderOf,
    tabTitle,
    selfSplit,
    dragTabIntoStage,
    dropTabIntoStage,
    commitSelfSplit,
    cancelTabDrop,
    click,
    openSessionWindow,
    reorderTabs,
    requestSplit,
  } = options

  const draggedTabId = ref<string | null>(null)

  const draggingTab = ref<string | null>(null)

  const dragGhost = ref<DragGhost | null>(null)

  const dropPlaceholderIndex = ref<number | null>(null)
  let tabDragWidth = 0

  const tabDragStartRect = ref<{ left: number; top: number; width: number } | null>(null)
  const tabDragDelta = ref({ x: 0, y: 0 })

  let ignoreTabClick = false
  let ignoreTabClickTimer: number | undefined
  let tabDragPoint: { x: number; y: number } | null = null
  let tabDragStart: { x: number; y: number } | null = null
  let tabDragHandled = false

  let dragSourceKind: TabDragKind | null = null

  let tabDragOutsideTimer: ReturnType<typeof setTimeout> | undefined

  let tabDragActive = false

  let tabDragLeftWindow = false
  let tabDragPointerBound = false

  const tabPlaceholderStyle = computed(() => ({
    width: `${tabDragWidth}px`,
    minWidth: `${Math.max(24, tabDragWidth)}px`,
    height: '26px',
    flex: 'none' as const,
    visibility: 'hidden' as const,
  }))
  const draggedTabStyle = computed<CSSProperties | undefined>(() => {
    const rect = tabDragStartRect.value
    if (!rect) return undefined
    return {
      position: 'fixed',
      left: `${rect.left}px`,
      top: `${rect.top}px`,
      width: `${rect.width}px`,
      transform: `translate(${tabDragDelta.value.x}px, ${tabDragDelta.value.y}px)`,
      zIndex: '30',
      pointerEvents: 'none',
      boxShadow: '0 8px 20px rgba(0,0,0,0.35)',
      opacity: '0.95',
    }
  })

  function tabNodeInfo(node: HTMLElement): { id: string; kind: TabDragKind } | null {
    const sessionId = node.dataset.sessionId
    if (sessionId) return { id: sessionId, kind: 'session' }
    const groupSessionId = node.dataset.splitGroupSessionId
    if (groupSessionId) return { id: groupSessionId, kind: 'group' }
    const sftpTabId = node.dataset.sftpTabId
    if (sftpTabId) return { id: sftpTabId, kind: 'sftp' }
    const rdpTabId = node.dataset.rdpTabId
    if (rdpTabId) return { id: rdpTabId, kind: 'rdp' }
    return null
  }

  function placeholderIndexFromPoint(clientX: number): number | null {
    const nodes = Array.from(tabBar.value?.querySelectorAll<HTMLElement>('.session-tab') ?? [])
    for (const [index, node] of nodes.entries()) {
      const rect = node.getBoundingClientRect()
      if (rect.width <= 0) continue
      if (clientX < rect.left + rect.width / 2) return index
    }
    return nodes.length
  }

  function reorderAnchorFromPoint(clientX: number, sourceId: string): TabReorderEvent | null {
    const nodes = Array.from(tabBar.value?.querySelectorAll<HTMLElement>('.session-tab') ?? [])
    const entries: Array<{ id: string; center: number }> = []
    for (const node of nodes) {
      const info = tabNodeInfo(node)
      if (!info || info.id === sourceId) continue

      if (tabOrderOf(info.id) === null) continue
      const rect = node.getBoundingClientRect()
      if (rect.width <= 0) continue
      entries.push({ id: info.id, center: rect.left + rect.width / 2 })
    }
    if (entries.length === 0) return null
    const index = entries.findIndex((entry) => clientX < entry.center)
    if (index === 0) return { sourceId, targetId: entries[0]!.id, position: 'before' }
    if (index < 0) return { sourceId, targetId: entries[entries.length - 1]!.id, position: 'after' }
    return { sourceId, targetId: entries[index]!.id, position: 'before' }
  }

  function cancelOutsideTimer(): void {
    clearTimeout(tabDragOutsideTimer)
    tabDragOutsideTimer = undefined
  }

  function dragHint(decision: 'reorder' | 'window' | 'none'): string {
    if (decision === 'reorder') return t('shell.workspace.dragReorder')
    if (dragSourceKind !== 'session') return t('shell.workspace.dragNoTarget')
    return decision === 'window' ? t('shell.workspace.dragNewWindow') : t('shell.workspace.dragNoTarget')
  }

  function handleTabDragMove(event: MouseEvent | PointerEvent): void {
    const sourceId = draggedTabId.value
    if (!sourceId || !tabDragStart) return

    if (!tabDragActive && Math.hypot(event.clientX - tabDragStart.x, event.clientY - tabDragStart.y) < 6) return
    if (!tabDragActive) {
      tabDragActive = true
      ignoreTabClick = true
      draggingTab.value = sourceId
    }
    tabDragPoint = { x: event.clientX, y: event.clientY }
    if (tabDragLeftWindow) {
      tabDragLeftWindow = false
      cancelOutsideTimer()
    }
    tabDragDelta.value = { x: event.clientX - tabDragStart.x, y: event.clientY - tabDragStart.y }

    const bar = tabBar.value
    if (bar) {
      const bounds = bar.getBoundingClientRect()
      if (event.clientX < bounds.left + 40) bar.scrollLeft -= 10
      else if (event.clientX > bounds.right - 40) bar.scrollLeft += 10
    }
    const point = tabDragPoint
    const stageRect = toRect(stage.value?.getBoundingClientRect())
    const barRect = toRect(tabBar.value?.getBoundingClientRect())

    const candidate = dragSourceKind === 'session' && isSplitDropCandidate({ point, bar: barRect, stage: stageRect })
    const title = tabTitle(sourceId)
    const decision = decideTabDrop({ point, bar: barRect, stage: stageRect })

    if (!candidate) {
      cancelTabDrop()
      dropPlaceholderIndex.value = decision === 'reorder' ? placeholderIndexFromPoint(point.x) : null
      dragGhost.value = { title, x: point.x, y: point.y, hint: dragHint(decision) }
      return
    }
    dragTabIntoStage(sourceId, point.x, point.y)
    dropPlaceholderIndex.value = null

    const pickingAsset = Boolean(selfSplit.value)
    dragGhost.value = {
      title,
      x: point.x,
      y: point.y,
      hint: pickingAsset ? t('shell.workspace.dragSelfSplit') : t('shell.workspace.dragSplit'),
    }
  }

  function handleTabDragOut(event: MouseEvent): void {
    if (event.relatedTarget !== null) return
    tabDragLeftWindow = true

    if (draggingTab.value && tabDragPoint) {
      dragGhost.value = {
        title: dragGhost.value?.title ?? '',
        x: tabDragPoint.x,
        y: tabDragPoint.y,
        hint: dragSourceKind === 'session' ? t('shell.workspace.dragNewWindow') : t('shell.workspace.dragNoTarget'),
      }
      clearTimeout(tabDragOutsideTimer)
      tabDragOutsideTimer = setTimeout(() => {
        if (tabDragLeftWindow && tabDragActive) finishTabDrag()
      }, 500)
    }
  }

  function handleTabPointerDown(event: PointerEvent): void {
    if (event.button !== 0) return
    const target = event.target as HTMLElement | null

    if (!target || target.closest('button, input, textarea, [contenteditable="true"]')) return
    const tab = target.closest<HTMLElement>('.session-tab')
    if (!tab || !tabBar.value?.contains(tab)) return

    const info = tabNodeInfo(tab)
    if (!info) return
    dragSourceKind = info.kind
    draggedTabId.value = info.id
    tabDragPoint = { x: event.clientX, y: event.clientY }
    tabDragStart = { x: event.clientX, y: event.clientY }
    tabDragLeftWindow = false
    tabDragHandled = false
    tabDragActive = false
    const rect = tab.getBoundingClientRect()
    tabDragWidth = rect.width
    tabDragStartRect.value = { left: rect.left, top: rect.top, width: rect.width }
    tabDragDelta.value = { x: 0, y: 0 }
    bindTabDragPointer()
  }

  function handleTabPointerUp(): void {
    if (!draggedTabId.value) return
    finishTabDrag()
  }

  function finishTabDrag(): void {
    if (tabDragHandled) return
    tabDragHandled = true
    const active = tabDragActive
    tabDragActive = false
    cancelOutsideTimer()
    draggingTab.value = null
    tabDragStartRect.value = null
    tabDragDelta.value = { x: 0, y: 0 }
    dragGhost.value = null
    dropPlaceholderIndex.value = null
    if (!active) {
      tabDragPoint = null
      tabDragStart = null
      draggedTabId.value = null
      unbindTabDragPointer()
      return
    }
    const sourceId = draggedTabId.value
    const sourceKind = dragSourceKind
    const point = tabDragPoint
    const leftWindow = tabDragLeftWindow
    tabDragPoint = null
    tabDragStart = null
    tabDragLeftWindow = false
    unbindTabDragPointer()
    const cleanup = () => {
      cancelTabDrop()
      draggedTabId.value = null
      dragSourceKind = null
      clearTimeout(ignoreTabClickTimer)
      ignoreTabClickTimer = window.setTimeout(() => {
        ignoreTabClick = false
        ignoreTabClickTimer = undefined
      }, 0)
    }

    if (sourceId && leftWindow && sourceKind === 'session') {
      openSessionWindow(sourceId)
      cleanup()
      return
    }

    if (sourceId && sourceKind === 'session') {
      const selfSplitPlan = commitSelfSplit()
      if (selfSplitPlan) {
        requestSplit(selfSplitPlan.destination, selfSplitPlan.direction)
        cleanup()
        return
      }
      if (dropTabIntoStage()) {
        cleanup()
        return
      }
    }

    const decision = decideTabDrop({
      point,
      bar: toRect(tabBar.value?.getBoundingClientRect()),
      stage: toRect(stage.value?.getBoundingClientRect()),
    })
    const reorder = sourceId && point && decision === 'reorder' ? reorderAnchorFromPoint(point.x, sourceId) : null
    if (sourceId && decision === 'window' && sourceKind === 'session') openSessionWindow(sourceId)
    else if (reorder) reorderTabs(reorder)
    cleanup()
  }

  function bindTabDragPointer(): void {
    if (tabDragPointerBound) return
    tabDragPointerBound = true
    window.addEventListener('pointermove', handleTabDragMove, true)
    window.addEventListener('mousemove', handleTabDragMove, true)
    window.addEventListener('mouseout', handleTabDragOut, true)
    window.addEventListener('pointerup', handleTabPointerUp, true)
    window.addEventListener('mouseup', handleTabPointerUp, true)
    window.addEventListener('blur', unbindTabDragPointer)
  }

  function unbindTabDragPointer(): void {
    if (!tabDragPointerBound) return
    tabDragPointerBound = false
    window.removeEventListener('pointermove', handleTabDragMove, true)
    window.removeEventListener('mousemove', handleTabDragMove, true)
    window.removeEventListener('mouseout', handleTabDragOut, true)
    window.removeEventListener('pointerup', handleTabPointerUp, true)
    window.removeEventListener('mouseup', handleTabPointerUp, true)
    window.removeEventListener('blur', unbindTabDragPointer)
  }

  function selectTab(sessionId: string): void {
    if (!ignoreTabClick) click(sessionId)
  }

  function selectAuxiliaryTab(activate: () => void): void {
    if (!ignoreTabClick) activate()
  }

  onBeforeUnmount(() => {
    unbindTabDragPointer()
    cancelOutsideTimer()
    clearTimeout(ignoreTabClickTimer)
  })

  return {
    draggingTab,
    dragGhost,
    dropPlaceholderIndex,
    tabPlaceholderStyle,
    draggedTabStyle,
    handleTabPointerDown,
    selectTab,
    selectAuxiliaryTab,
  }
}
