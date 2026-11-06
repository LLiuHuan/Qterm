import type { IMarker, Terminal } from '@xterm/xterm'
import { ref, shallowRef } from 'vue'
import {
  computeGutterRows,
  gutterDigitsForLine,
  gutterMetricsFromSizes,
  gutterWidthForDigits,
  type GutterMetrics,
  type GutterRow,
} from './terminalGutter'

interface UseTerminalGutterOptions {
  enabled: () => boolean

  visible: () => boolean
}

export function useTerminalGutter(options: UseTerminalGutterOptions) {
  const rows = shallowRef<GutterRow[]>([])
  const rowHeight = ref(0)
  const topPadding = ref(0)
  const width = ref(0)

  let terminal: Terminal | null = null
  let subscriptions: Array<{ dispose(): void }> = []
  let frame: number | null = null

  let anchor: { marker: IMarker; absolute: number } | null = null

  let topAbsolute = 0

  let previousBaseY = 0

  let maxLineNumber = 0

  let digits = 1

  let pendingClear = false

  let awaitingClearContent = false
  let clearBaseline = 0

  let excludedRanges: Array<{ start?: IMarker; stop?: IMarker; count: number }> = []

  let bakedSkip = 0

  const alternateScreen = ref(false)

  function cancelFrame(): void {
    if (frame === null) return
    window.cancelAnimationFrame(frame)
    frame = null
  }

  function schedule(): void {
    if (frame !== null) return
    if (!terminal) return
    if (!options.enabled() || !options.visible()) return
    frame = window.requestAnimationFrame(() => {
      frame = null
      sync()
    })
  }

  function measure(instance: Terminal): GutterMetrics | null {
    const host = instance.element
    const screen = host?.querySelector<HTMLElement>('.xterm-screen')
    if (!host || !screen) return null

    const hostRect = host.getBoundingClientRect()
    const screenRect = screen.getBoundingClientRect()

    return gutterMetricsFromSizes({
      screenHeight: screen.clientHeight || screenRect.height,
      screenWidth: screen.clientWidth || screenRect.width,
      screenTopOffset: screenRect.top - hostRect.top,
      rows: instance.rows,
      cols: instance.cols,
      fontSize: Number(instance.options.fontSize ?? 13),
      lineHeight: Number(instance.options.lineHeight ?? 1.25),
    })
  }

  function disposeAnchor(): void {
    if (!anchor) return
    anchor.marker.dispose()
    anchor = null
  }

  function resolveTopAbsolute(instance: Terminal, buffer: Terminal['buffer']['active']): number {
    if (anchor && !anchor.marker.isDisposed && anchor.marker.line >= 0 && anchor.marker.line < buffer.length) {
      return anchor.absolute - anchor.marker.line
    }

    if (previousBaseY > 0 && buffer.baseY === 0) {
      return maxLineNumber
    }

    return topAbsolute
  }

  function anchorAtCursor(instance: Terminal, cursorIndex: number): void {
    disposeAnchor()
    if (typeof instance.registerMarker !== 'function') return
    const marker = instance.registerMarker(0)
    if (!marker) return
    anchor = { marker, absolute: topAbsolute + cursorIndex }
  }

  function sync(): void {
    const instance = terminal
    if (!instance) return

    if (!options.enabled() || !options.visible()) {
      if (rows.value.length > 0) rows.value = []
      return
    }

    const buffer = instance.buffer?.active
    if (!buffer) return

    const metrics = measure(instance)
    if (metrics) {
      rowHeight.value = metrics.rowHeight
      topPadding.value = metrics.topPadding
    }

    if (buffer.type === 'alternate') {
      alternateScreen.value = true
      if (rows.value.length > 0) rows.value = []
      return
    }
    alternateScreen.value = false

    const viewportY = Math.max(0, Math.min(buffer.baseY, buffer.viewportY))
    const cursorIndex = Math.min(buffer.length - 1, buffer.baseY + buffer.cursorY)
    if (pendingClear) {
      pendingClear = false

      disposeAnchor()
      for (const range of excludedRanges) {
        range.start?.dispose()
        range.stop?.dispose()
      }
      excludedRanges = []
      bakedSkip = 0
      clearBaseline = maxLineNumber
      awaitingClearContent = true
    }
    if (awaitingClearContent) {
      let firstContent = -1
      for (let index = 0; index <= cursorIndex; index += 1) {
        if (buffer.getLine(index)?.translateToString(true).trim()) {
          firstContent = index
          break
        }
      }
      topAbsolute = clearBaseline - (firstContent >= 0 ? firstContent : cursorIndex)
      if (firstContent >= 0) awaitingClearContent = false
    } else {
      topAbsolute = resolveTopAbsolute(instance, buffer)
    }

    const nextRows = computeGutterRows({
      rows: instance.rows,
      viewportY,

      cursorLine: buffer.baseY + buffer.cursorY,
      topAbsolute,
      getLine: (index) => buffer.getLine(index),
    })

    bakeDisposedRanges(buffer)

    const visibleRows: GutterRow[] = []
    for (const row of nextRows) {
      const bufferIndex = viewportY + row.offset
      if (isExcluded(bufferIndex, buffer)) continue

      if (buffer.getLine(bufferIndex)?.translateToString(true).trim() === '') continue
      const lineNumber = row.lineNumber - bakedSkip - excludedBefore(bufferIndex, buffer)
      visibleRows.push({ key: lineNumber, offset: row.offset, lineNumber })
    }

    if (visibleRows.length > 0) {
      maxLineNumber = Math.max(maxLineNumber, visibleRows[visibleRows.length - 1].lineNumber)
      digits = Math.max(digits, gutterDigitsForLine(maxLineNumber))
    }

    const nextWidth = gutterWidthForDigits(digits, metrics?.cellWidth ?? 0)
    if (nextWidth !== width.value) width.value = nextWidth

    anchorAtCursor(instance, cursorIndex)
    previousBaseY = buffer.baseY

    if (!sameRows(rows.value, visibleRows)) rows.value = visibleRows
  }

  function subscribeEvent(
    source: ((listener: () => void) => { dispose(): void }) | undefined,
    listener: () => void,
  ): void {
    if (typeof source !== 'function') return
    const disposable = source(listener)
    if (disposable && typeof disposable.dispose === 'function') subscriptions.push(disposable)
  }

  function attach(instance: Terminal): void {
    detach()
    terminal = instance

    const events = instance as unknown as {
      onRender?: (listener: () => void) => { dispose(): void }
      onWriteParsed?: (listener: () => void) => { dispose(): void }
      onScroll?: (listener: () => void) => { dispose(): void }
      onResize?: (listener: () => void) => { dispose(): void }
      buffer?: { onBufferChange?: (listener: () => void) => { dispose(): void } }
    }
    const refresh = () => schedule()
    subscribeEvent(events.onRender, refresh)
    subscribeEvent(events.onWriteParsed, refresh)
    subscribeEvent(events.onScroll, refresh)
    subscribeEvent(events.onResize, refresh)
    subscribeEvent(events.buffer?.onBufferChange, refresh)

    const core = instance as unknown as {
      parser?: {
        registerCsiHandler?: (
          id: { final: string },
          handler: (params: (number | number[])[]) => boolean,
        ) => { dispose(): void }
      }
    }
    if (typeof core.parser?.registerCsiHandler === 'function') {
      const disposable = core.parser.registerCsiHandler({ final: 'J' }, (params) => {
        if (params[0] === 3 && instance.buffer.active.type === 'normal') pendingClear = true
        return false
      })
      if (disposable && typeof disposable.dispose === 'function') subscriptions.push(disposable)
    }

    sync()
  }

  function detach(): void {
    cancelFrame()
    for (const subscription of subscriptions) subscription.dispose()
    subscriptions = []
    disposeAnchor()
    terminal = null
    topAbsolute = 0
    previousBaseY = 0
    maxLineNumber = 0
    digits = 1
    pendingClear = false
    awaitingClearContent = false
    clearBaseline = 0
    excludedRanges = []
    bakedSkip = 0
    rows.value = []
    alternateScreen.value = false
  }

  function excludedEnd(range: (typeof excludedRanges)[number], buffer: Terminal['buffer']['active']): number {
    const stop = range.stop?.line ?? -1
    if (stop < 0) return stop
    const blank = buffer.getLine(stop)?.translateToString(true).trim() === ''
    return stop + (blank ? 1 : 0)
  }

  function bakeDisposedRanges(buffer: Terminal['buffer']['active']): void {
    if (excludedRanges.length === 0) return
    const alive: typeof excludedRanges = []
    for (const range of excludedRanges) {
      if (range.start?.isDisposed || range.stop?.isDisposed) {
        bakedSkip += range.count
        continue
      }
      range.count = excludedEnd(range, buffer) - (range.start?.line ?? 0)
      alive.push(range)
    }
    excludedRanges = alive
  }

  function excludedBefore(index: number, buffer: Terminal['buffer']['active']): number {
    let count = 0
    for (const range of excludedRanges) {
      const start = range.start?.line ?? -1
      const stop = excludedEnd(range, buffer)
      if (start < 0 || stop <= start) continue
      if (stop <= index) count += stop - start
      else if (start < index) count += index - start
    }
    return count
  }

  function isExcluded(index: number, buffer: Terminal['buffer']['active']): boolean {
    for (const range of excludedRanges) {
      const start = range.start?.line ?? -1
      const stop = excludedEnd(range, buffer)
      if (start < 0 || stop <= start) continue
      if (index >= start && index < stop) return true
    }
    return false
  }

  function excludeRange(start: IMarker | undefined, stop: IMarker | undefined): void {
    if (!start || !stop || start.isDisposed || stop.isDisposed) return
    const buffer = terminal?.buffer.active
    if (!buffer) return
    const count = excludedEnd({ start, stop, count: 0 }, buffer) - start.line
    if (count <= 0) return
    excludedRanges.push({ start, stop, count })
    schedule()
  }

  function notifyCleared(): void {
    pendingClear = true
    schedule()
  }

  return {
    rows,
    rowHeight,
    topPadding,
    width,
    alternateScreen,
    attach,
    detach,
    sync,
    notifyCleared,
    excludeRange,
  }
}

function sameRows(previous: GutterRow[], next: GutterRow[]): boolean {
  if (previous.length !== next.length) return false
  for (let index = 0; index < previous.length; index += 1) {
    const a = previous[index]
    const b = next[index]
    if (a.key !== b.key || a.offset !== b.offset || a.lineNumber !== b.lineNumber) return false
  }
  return true
}
