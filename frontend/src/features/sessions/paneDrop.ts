import type { DropPoint } from './tabDrop'

export type SplitDirection = 'left' | 'right' | 'top' | 'bottom'

export interface PanePercentRect {
  id: string
  x: number
  y: number
  width: number
  height: number
}

export interface PaneDropTarget {
  id: string
  direction: SplitDirection | 'center'
}

export const SPLIT_EDGE_RATIO = 0.28

export interface DropStageRect {
  left: number
  top: number
  width: number
  height: number
}

export function stagePointPercent(stage: DropStageRect | null, point: DropPoint): DropPoint | null {
  if (!stage || !stage.width || !stage.height) return null
  return { x: ((point.x - stage.left) / stage.width) * 100, y: ((point.y - stage.top) / stage.height) * 100 }
}

export function paneDropTarget(
  panes: readonly PanePercentRect[],
  sourceId: string | null,
  point: DropPoint | null,
): PaneDropTarget | null {
  if (!point) return null
  const pane = panes.find(
    (item) =>
      point.x >= item.x && point.x <= item.x + item.width && point.y >= item.y && point.y <= item.y + item.height,
  )
  if (!pane || pane.id === sourceId) return null
  const rx = (point.x - pane.x) / pane.width
  const ry = (point.y - pane.y) / pane.height

  const edges: Array<[SplitDirection, number]> = [
    ['left', rx],
    ['right', 1 - rx],
    ['top', ry],
    ['bottom', 1 - ry],
  ]
  edges.sort((a, b) => a[1] - b[1])
  const edge = edges[0]!
  return { id: pane.id, direction: edge[1] < SPLIT_EDGE_RATIO ? edge[0] : 'center' }
}

export function longEdgeDirection(pane: PanePercentRect, stageSize: { width: number; height: number }): SplitDirection {
  const paneWidth = (pane.width / 100) * stageSize.width
  const paneHeight = (pane.height / 100) * stageSize.height
  return paneHeight > paneWidth ? 'bottom' : 'right'
}

export function tabDropTargetFromPoint(input: {
  sourceId: string
  pointPercent: DropPoint | null
  panes: readonly PanePercentRect[]
  stageSize: { width: number; height: number }
}): PaneDropTarget | null {
  const target = paneDropTarget(input.panes, input.sourceId, input.pointPercent)
  if (!target || target.direction !== 'center') return target
  const pane = input.panes.find((item) => item.id === target.id)
  if (!pane) return target
  return { id: target.id, direction: longEdgeDirection(pane, input.stageSize) }
}

export function splitDirectionFromStagePoint(point: DropPoint | null): SplitDirection | null {
  if (!point) return null
  const { x, y } = point
  if (x < 0 || x > 100 || y < 0 || y > 100) return null
  if (Math.abs(x - 50) >= Math.abs(y - 50)) return x < 50 ? 'left' : 'right'
  return y < 50 ? 'top' : 'bottom'
}
