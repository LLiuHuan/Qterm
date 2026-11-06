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
      .filter((rule) => rule.enabled && rule.pattern && /^#[\da-f]{6}$/i.test(rule.color))
      .flatMap((rule) => {
        try {
          return [{ rule, expression: new RegExp(rule.pattern, 'gu') }]
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
    for (
      let lineIndex = firstVisibleLine;
      lineIndex < lastVisibleLine && decorationCount < MAX_KEYWORD_DECORATIONS;
      lineIndex += 1
    ) {
      const line = active.getLine(lineIndex)
      if (!line) continue
      const lineOutput = keywordLineText(line)
      const text = lineOutput.text
      if (!text) continue

      for (const { rule, expression } of [...rules].reverse()) {
        expression.lastIndex = 0
        for (const match of text.matchAll(expression)) {
          const value = match[0]
          if (!value) continue
          const range = keywordCellRange(line, lineOutput.columns, match.index ?? 0, value.length)
          const marker = terminal.registerMarker(lineIndex - cursorLine)
          if (!marker) continue
          const decoration = terminal.registerDecoration({
            marker,
            x: range.x,
            width: range.width,
            foregroundColor: rule.color,
            layer: 'top',
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
