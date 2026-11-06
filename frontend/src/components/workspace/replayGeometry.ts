export const REPLAY_FALLBACK_COLUMNS = 80
export const REPLAY_FALLBACK_ROWS = 24

export const REPLAY_AP_BORDER_PX = 18

export const XTERM_PADDING_X_PX = 4
export const XTERM_PADDING_Y_PX = 2

export const REPLAY_GRID_SAFETY_MARGIN = 3

export const REPLAY_DYNAMIC_HORIZONTAL_SAFETY_MARGIN = 6

const DOM_PROBE_COLUMNS = 200

export function measureMonospaceCharWidth(fontSizePx: string): number {
  const fallback = Number.parseFloat(fontSizePx) * 0.6
  if (typeof document === 'undefined' || !document.body) return fallback
  const probe = document.createElement('div')
  probe.style.position = 'absolute'
  probe.style.left = '-9999px'
  probe.style.top = '0'
  probe.style.height = '0'
  probe.style.overflow = 'hidden'
  probe.style.visibility = 'hidden'
  probe.style.font = `${fontSizePx} monospace`
  const inner = document.createElement('div')
  inner.style.width = `${DOM_PROBE_COLUMNS}ch`
  inner.style.height = '1px'
  probe.appendChild(inner)
  document.body.appendChild(probe)
  const width = inner.getBoundingClientRect().width
  probe.remove()
  if (!Number.isFinite(width) || width <= 0) return fallback
  return width / DOM_PROBE_COLUMNS
}

export const REPLAY_GRID_HYSTERESIS_PX = 1
export type GridDimensions = { columns: number; rows: number }
export type PreviousGrid = { columns: number; rows: number } | null

export function shouldRecalcGrid(
  hostWidth: number,
  hostHeight: number,
  previousWidth: number,
  previousHeight: number,
): boolean {
  return (
    Math.abs(hostWidth - previousWidth) >= REPLAY_GRID_HYSTERESIS_PX ||
    Math.abs(hostHeight - previousHeight) >= REPLAY_GRID_HYSTERESIS_PX
  )
}

export function computeReplayGrid(
  hostWidth: number,
  hostHeight: number,
  recordedColumns: number,
  recordedRows: number,
): GridDimensions | null {
  if (!Number.isFinite(hostWidth) || hostWidth <= 0 || !Number.isFinite(hostHeight) || hostHeight <= 0) return null
  const borderPx = REPLAY_AP_BORDER_PX
  const safetyMarginX = REPLAY_DYNAMIC_HORIZONTAL_SAFETY_MARGIN
  const safetyMarginY = REPLAY_GRID_SAFETY_MARGIN
  const cellW = measureMonospaceCharWidth('12px')
  const cellH = 12
  const fittedCols = Math.max(2, Math.floor((hostWidth - borderPx - safetyMarginX) / cellW))
  const fittedRows = Math.max(1, Math.floor((hostHeight - borderPx - safetyMarginY) / cellH))
  const columns = Math.max(recordedColumns, fittedCols)
  const rows = Math.max(recordedRows, fittedRows)
  if (rows < 1 || columns < 2) return null
  return { columns, rows }
}

export function computeStaticGrid(
  hostWidth: number,
  hostHeight: number,
  recordedColumns: number,
  recordedRows: number,
  cellWidth: number,
  cellHeight: number,
): GridDimensions | null {
  if (!Number.isFinite(hostWidth) || hostWidth <= 0 || !Number.isFinite(hostHeight) || hostHeight <= 0) return null
  if (!Number.isFinite(cellWidth) || cellWidth <= 0 || !Number.isFinite(cellHeight) || cellHeight <= 0) return null
  const paddingX = XTERM_PADDING_X_PX * 2
  const paddingY = XTERM_PADDING_Y_PX * 2
  const safetyMargin = REPLAY_GRID_SAFETY_MARGIN
  const fittedCols = Math.max(2, Math.floor((hostWidth - paddingX - safetyMargin) / cellWidth))
  const fittedRows = Math.max(1, Math.floor((hostHeight - paddingY - safetyMargin) / cellHeight))
  const columns = Math.max(recordedColumns, fittedCols)
  const rows = Math.max(recordedRows, fittedRows)
  if (rows < 1 || columns < 2) return null
  return { columns, rows }
}

export interface ReplayGridEventLike {
  type: number
  columns: number
  rows: number
}

export function resolveReplayGrid(events: readonly ReplayGridEventLike[]): {
  columns: number
  rows: number
} {
  for (const event of events) {
    if (event.type !== 2) continue
    if (event.columns > 0 && event.rows > 0) {
      return { columns: Math.floor(event.columns), rows: Math.floor(event.rows) }
    }
  }
  return { columns: REPLAY_FALLBACK_COLUMNS, rows: REPLAY_FALLBACK_ROWS }
}
