export interface GutterRow {
  key: number

  offset: number

  lineNumber: number

  label?: string

  title?: string
}

export interface GutterMetrics {
  rowHeight: number

  topPadding: number

  cellWidth: number
}

export const GUTTER_RIGHT_PADDING = 10

export const GUTTER_BORDER_WIDTH = 1

export const GUTTER_MIN_NUMBER_WIDTH = 24

export const GUTTER_DIGIT_HEADROOM = 2

export function gutterMetricsFromSizes(sizes: {
  screenHeight: number
  screenWidth: number
  screenTopOffset: number
  rows: number
  cols: number
  fontSize: number
  lineHeight: number
}): GutterMetrics {
  const { screenHeight, screenWidth, screenTopOffset, rows, cols, fontSize, lineHeight } = sizes
  const safeFontSize = Number.isFinite(fontSize) && fontSize > 0 ? fontSize : 13
  const safeLineHeight = Number.isFinite(lineHeight) && lineHeight > 0 ? lineHeight : 1.25

  const measuredRowHeight = rows > 0 && screenHeight > 0 ? screenHeight / rows : 0
  const measuredCellWidth = cols > 0 && screenWidth > 0 ? screenWidth / cols : 0

  return {
    rowHeight: measuredRowHeight > 0 ? measuredRowHeight : safeFontSize * safeLineHeight,
    cellWidth: measuredCellWidth > 0 ? measuredCellWidth : safeFontSize * 0.6,
    topPadding: Number.isFinite(screenTopOffset) && screenTopOffset > 0 ? screenTopOffset : 0,
  }
}

export function computeGutterRows(options: {
  rows: number

  viewportY: number

  cursorLine: number

  topAbsolute: number

  getLine: (index: number) => { isWrapped: boolean } | undefined
}): GutterRow[] {
  const { rows, viewportY, cursorLine, topAbsolute, getLine } = options
  const result: GutterRow[] = []
  if (!Number.isFinite(rows) || rows <= 0) return result

  if (!Number.isFinite(viewportY) || !Number.isFinite(cursorLine) || !Number.isFinite(topAbsolute)) {
    return result
  }
  const start = Math.max(0, viewportY)

  const end = Math.min(rows - 1, cursorLine - viewportY)
  for (let offset = 0; offset <= end; offset += 1) {
    const bufferLine = start + offset
    const line = getLine(bufferLine)
    if (!line) continue

    if (line.isWrapped) continue
    const lineNumber = topAbsolute + bufferLine + 1
    result.push({ key: lineNumber, offset, lineNumber })
  }
  return result
}

export const DEFAULT_GUTTER_TIMESTAMP_FORMAT = 'HH:mm:ss'

const TIMESTAMP_TOKENS = [
  'YYYY',
  'YY',
  'SSS',
  'SS',
  'MM',
  'M',
  'DD',
  'D',
  'HH',
  'H',
  'mm',
  'm',
  'ss',
  's',
  'S',
] as const
const TIMESTAMP_TOKEN_PATTERN = new RegExp(TIMESTAMP_TOKENS.join('|'), 'g')

export function formatGutterTimestamp(timestamp: number, pattern?: string): string {
  const date = new Date(timestamp)
  if (!Number.isFinite(timestamp) || Number.isNaN(date.getTime())) return ''
  const source = pattern && pattern.trim().length > 0 ? pattern : DEFAULT_GUTTER_TIMESTAMP_FORMAT
  const pad = (value: number): string => String(value).padStart(2, '0')
  return source.replace(TIMESTAMP_TOKEN_PATTERN, (token) => {
    switch (token) {
      case 'YYYY':
        return String(date.getFullYear())
      case 'YY':
        return pad(date.getFullYear() % 100)
      case 'MM':
        return pad(date.getMonth() + 1)
      case 'M':
        return String(date.getMonth() + 1)
      case 'DD':
        return pad(date.getDate())
      case 'D':
        return String(date.getDate())
      case 'HH':
        return pad(date.getHours())
      case 'H':
        return String(date.getHours())
      case 'mm':
        return pad(date.getMinutes())
      case 'm':
        return String(date.getMinutes())
      case 'ss':
        return pad(date.getSeconds())
      case 's':
        return String(date.getSeconds())
      case 'SSS':
        return String(date.getMilliseconds()).padStart(3, '0')
      case 'SS':
        return pad(Math.floor(date.getMilliseconds() / 10))
      case 'S':
        return String(Math.floor(date.getMilliseconds() / 100))
      default:
        return token
    }
  })
}

export function gutterNumberWidth(digits: number, cellWidth: number): number {
  const safeDigits = Number.isFinite(digits) && digits > 0 ? Math.floor(digits) : 1
  const safeCellWidth = Number.isFinite(cellWidth) && cellWidth > 0 ? cellWidth : 8
  return Math.max(GUTTER_MIN_NUMBER_WIDTH, Math.ceil(safeCellWidth * safeDigits) + GUTTER_DIGIT_HEADROOM)
}

export function gutterWidthForDigits(digits: number, cellWidth: number): number {
  return gutterNumberWidth(digits, cellWidth) + GUTTER_RIGHT_PADDING + GUTTER_BORDER_WIDTH
}

export function gutterDigitsForLine(lineNumber: number): number {
  const safe = Number.isFinite(lineNumber) && lineNumber > 0 ? Math.floor(lineNumber) : 1
  return String(safe).length
}
