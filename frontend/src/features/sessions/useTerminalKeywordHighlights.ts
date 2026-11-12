import type { IDecoration, IMarker, Terminal } from '@xterm/xterm'
import type { KeywordHighlightRule } from '../settings/settingsStore'

type Disposable = { dispose(): void }

interface UseTerminalKeywordHighlightsOptions {
  terminal: () => Terminal | null
  enabled: () => boolean | undefined
  rules: () => KeywordHighlightRule[] | undefined
}

interface KeywordDecoration {
  marker: IMarker
  decoration: IDecoration
}

const MAX_KEYWORD_DECORATIONS = 1000

const KEYWORD_COLOR_PATTERN = /^#[\da-f]{6}$/i

const MAX_LOGICAL_LINE_CHARS = 16384

interface KeywordLine {
  length: number
  isWrapped?: boolean
  getCell(x: number): { getChars(): string; getWidth(): number } | undefined
}

interface LogicalLineSegment {
  lineIndex: number
  line: KeywordLine
  text: string
  columns: number[]
  offset: number
}

export function useTerminalKeywordHighlights(options: UseTerminalKeywordHighlightsOptions) {
  let keywordHighlightSubscription: Disposable | null = null
  let keywordRenderSubscription: Disposable | null = null
  let keywordHighlightFrame: number | null = null
  let keywordDecorations: KeywordDecoration[] = []

  function clearKeywordHighlights(): void {
    for (const { decoration, marker } of keywordDecorations) {
      decoration.dispose()
      marker.dispose()
    }
    keywordDecorations = []
  }

  function keywordLineText(line: {
    length: number
    getCell(x: number): { getChars(): string; getWidth(): number } | undefined
  }): { text: string; columns: number[] } {
    const chars: string[] = []
    const columns: number[] = []
    for (let column = 0; column < line.length;) {
      const cell = line.getCell(column)
      const width = cell?.getWidth() ?? 1
      if (width <= 0) {
        column += 1
        continue
      }
      const value = cell?.getChars() || ' '
      chars.push(value)
      for (let offset = 0; offset < value.length; offset += 1) columns.push(column)
      column += width
    }
    return { text: chars.join(''), columns }
  }

  function keywordCellRange(
    line: { getCell(x: number): { getWidth(): number } | undefined },
    columns: number[],
    start: number,
    length: number,
  ): { x: number; width: number } {
    const x = columns[start] ?? 0
    const lastColumn = columns[Math.max(start, start + length - 1)] ?? x
    const lastCell = line.getCell(lastColumn)
    const width = Math.max(1, lastColumn - x + (lastCell?.getWidth() || 1))
    return { x, width }
  }

  function logicalLineStart(
    active: { getLine(index: number): KeywordLine | undefined },
    lineIndex: number,
    maxBacktrack: number,
  ): number {
    let start = lineIndex
    let steps = 0
    while (start > 0 && steps < maxBacktrack) {
      const line = active.getLine(start)
      if (!line?.isWrapped) break
      start -= 1
      steps += 1
    }
    return start
  }

  function buildLogicalLine(
    active: { length: number; getLine(index: number): KeywordLine | undefined },
    start: number,
  ): { segments: LogicalLineSegment[]; text: string; lastLineIndex: number } {
    const segments: LogicalLineSegment[] = []
    let text = ''
    let index = start
    let lastLineIndex = start - 1
    while (index < active.length) {
      const line = active.getLine(index)
      if (!line) break
      if (index > start && !line.isWrapped) break
      const output = keywordLineText(line)
      segments.push({ lineIndex: index, line, text: output.text, columns: output.columns, offset: text.length })
      text += output.text
      lastLineIndex = index
      if (text.length >= MAX_LOGICAL_LINE_CHARS) break
      index += 1
    }
    return { segments, text, lastLineIndex }
  }

  function renderKeywordHighlights(): void {
    keywordHighlightFrame = null
    const terminal = options.terminal()
    if (!terminal || !options.enabled() || typeof terminal.registerMarker !== 'function') {
      clearKeywordHighlights()
      return
    }
    const active = terminal.buffer?.active
    if (!active || active.type === 'alternate' || typeof terminal.registerDecoration !== 'function') {
      clearKeywordHighlights()
      return
    }
    const rules = (options.rules() ?? [])
      .filter((rule) => rule.enabled && rule.pattern && KEYWORD_COLOR_PATTERN.test(rule.color))
      .flatMap((rule) => {
        try {
          return [
            {
              rule,

              expression: new RegExp(rule.pattern, (rule.caseSensitive ?? true) ? 'gu' : 'giu'),

              backgroundColor: KEYWORD_COLOR_PATTERN.test(rule.backgroundColor ?? '')
                ? rule.backgroundColor
                : undefined,
            },
          ]
        } catch {
          return []
        }
      })
    if (rules.length === 0) {
      clearKeywordHighlights()
      return
    }
    clearKeywordHighlights()
    const cursorLine = active.baseY + active.cursorY

    const firstVisibleLine = Math.max(0, active.viewportY)
    const lastVisibleLine = Math.min(active.length, firstVisibleLine + terminal.rows)
    let decorationCount = 0

    const orderedRules = [...rules].reverse()
    for (let lineIndex = firstVisibleLine; lineIndex < lastVisibleLine;) {
      if (decorationCount >= MAX_KEYWORD_DECORATIONS) break
      const line = active.getLine(lineIndex)
      if (!line) {
        lineIndex += 1
        continue
      }

      const logicalStart = line.isWrapped ? logicalLineStart(active, lineIndex, terminal.rows) : lineIndex
      const { segments, text, lastLineIndex } = buildLogicalLine(active, logicalStart)

      lineIndex = Math.max(lineIndex + 1, lastLineIndex + 1)
      if (!text || segments.length === 0) continue
      for (const { rule, expression, backgroundColor } of orderedRules) {
        expression.lastIndex = 0
        for (const match of text.matchAll(expression)) {
          const value = match[0]
          if (!value) continue
          const matchStart = match.index ?? 0
          const matchEnd = matchStart + value.length

          for (const segment of segments) {
            const segmentStart = segment.offset
            const segmentEnd = segment.offset + segment.text.length
            const from = Math.max(matchStart, segmentStart)
            const to = Math.min(matchEnd, segmentEnd)
            if (from >= to) continue
            const range = keywordCellRange(segment.line, segment.columns, from - segmentStart, to - from)
            const marker = terminal.registerMarker(segment.lineIndex - cursorLine)
            if (!marker) continue

            const decoration = terminal.registerDecoration({
              marker,
              x: range.x,
              width: range.width,
              foregroundColor: rule.color,
              ...(backgroundColor ? { backgroundColor } : {}),
              layer: 'bottom',
            })
            if (decoration) {
              keywordDecorations.push({ marker, decoration })
              decorationCount += 1
            } else {
              marker.dispose()
            }
            if (decorationCount >= MAX_KEYWORD_DECORATIONS) break
          }
          if (decorationCount >= MAX_KEYWORD_DECORATIONS) break
        }
        if (decorationCount >= MAX_KEYWORD_DECORATIONS) break
      }
    }
  }

  function queueKeywordHighlights(): void {
    if (keywordHighlightFrame !== null) return
    keywordHighlightFrame = window.requestAnimationFrame(renderKeywordHighlights)
  }

  function subscribe(terminal: Terminal): void {
    keywordHighlightSubscription?.dispose()
    keywordRenderSubscription?.dispose()
    keywordHighlightSubscription = null
    keywordRenderSubscription = null
    if (terminal.onWriteParsed) keywordHighlightSubscription = terminal.onWriteParsed(queueKeywordHighlights)
    if (terminal.onRender) keywordRenderSubscription = terminal.onRender(queueKeywordHighlights)
  }

  function dispose(): void {
    if (keywordHighlightFrame !== null) {
      window.cancelAnimationFrame(keywordHighlightFrame)
      keywordHighlightFrame = null
    }
    keywordHighlightSubscription?.dispose()
    keywordRenderSubscription?.dispose()
    keywordHighlightSubscription = null
    keywordRenderSubscription = null
    clearKeywordHighlights()
  }

  return { dispose, queueKeywordHighlights, subscribe }
}
