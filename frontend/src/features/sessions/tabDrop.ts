export interface DropRect {
  left: number
  top: number
  right: number
  bottom: number
}

export interface DropPoint {
  x: number
  y: number
}

export const TAB_BAR_REORDER_PADDING = 24

export function toRect(
  rect: { left: number; top: number; right: number; bottom: number } | null | undefined,
): DropRect | null {
  return rect ? { left: rect.left, top: rect.top, right: rect.right, bottom: rect.bottom } : null
}

export function pointInRect(rect: DropRect | null, point: DropPoint | null, padding = 0): boolean {
  if (!rect || !point) return false
  return (
    point.x >= rect.left - padding &&
    point.x <= rect.right + padding &&
    point.y >= rect.top - padding &&
    point.y <= rect.bottom + padding
  )
}

export function isSplitDropCandidate(input: {
  point: DropPoint | null
  bar: DropRect | null
  stage: DropRect | null
}): boolean {
  const { point, bar, stage } = input
  if (!point) return false
  if (pointInRect(bar, point, TAB_BAR_REORDER_PADDING)) return false
  return pointInRect(stage, point)
}

export type TabDropDecision = 'reorder' | 'window' | 'none'

export function decideTabDrop(input: {
  point: DropPoint | null
  bar: DropRect | null
  stage: DropRect | null
}): TabDropDecision {
  const { point, bar, stage } = input
  if (!point) return 'none'
  if (pointInRect(bar, point, TAB_BAR_REORDER_PADDING)) return 'reorder'
  if (pointInRect(stage, point)) return 'none'
  return 'window'
}
