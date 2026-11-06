import type { IMarker, Terminal } from '@xterm/xterm'
import { ref } from 'vue'

type TerminalBuffer = Terminal['buffer']['normal']

interface LineSnapshot {
  text: string
  isWrapped: boolean
}

interface TimestampRecord {
  marker: IMarker
  timestamp: number
}

export function useTerminalLineTimestamps() {
  const revision = ref(0)

  let terminal: Terminal | null = null
  let subscriptions: Array<{ dispose(): void }> = []
  let records: TimestampRecord[] = []
  const recordsByLine = new Map<number, TimestampRecord>()
  let snapshot = new Map<number, LineSnapshot>()

  let snapshotKeyOffset = 0
  let snapshotLength = 0
  let snapshotBaseY = 0
  let snapshotAnchor: IMarker | null = null
  let snapshotAnchorLine = 0
  let snapshotReady = false
  let needsSnapshotRefresh = false
  let pendingClear = false
  let pendingOutputTimes: number[] = []
  let lastCursorLine = 0
  let dirtyLineFeeds = 0
  let activeWasAlternate = false
  let alternateEnteredAt: number | undefined

  function bumpRevision(): void {
    revision.value += 1
  }

  function normalizeTimestamp(timestamp: number | Date | undefined): number {
    const value = timestamp instanceof Date ? timestamp.getTime() : (timestamp ?? Date.now())
    return Number.isFinite(value) ? value : Date.now()
  }

  function disposeSnapshotAnchor(): void {
    snapshotAnchor?.dispose()
    snapshotAnchor = null
  }

  function disposeRecords(): void {
    for (const record of records) record.marker.dispose()
    records = []
    recordsByLine.clear()
  }

  function pruneRecords(): boolean {
    const alive: TimestampRecord[] = []
    let changed = false
    recordsByLine.clear()
    for (const record of records) {
      if (record.marker.isDisposed || record.marker.line < 0) {
        changed = true
        continue
      }
      alive.push(record)
      recordsByLine.set(record.marker.line, record)
    }
    if (changed) records = alive
    return changed
  }

  function lineSnapshot(line: ReturnType<TerminalBuffer['getLine']>): LineSnapshot | undefined {
    if (!line) return undefined
    return {
      text: line.translateToString(false),
      isWrapped: line.isWrapped,
    }
  }

  function readBuffer(buffer: TerminalBuffer): Map<number, LineSnapshot> {
    const next = new Map<number, LineSnapshot>()
    for (let index = 0; index < buffer.length; index += 1) {
      const line = lineSnapshot(buffer.getLine(index))
      if (line) next.set(index, line)
    }
    return next
  }

  function rebuildSnapshot(buffer: TerminalBuffer): void {
    snapshot = readBuffer(buffer)
    snapshotKeyOffset = 0
    snapshotLength = buffer.length
    snapshotBaseY = buffer.baseY
    snapshotReady = true
  }

  function hasContent(line: LineSnapshot | undefined): line is LineSnapshot {
    return Boolean(line && line.text.trim() !== '')
  }

  function recordAt(bufferLine: number): TimestampRecord | undefined {
    const record = recordsByLine.get(bufferLine)
    if (!record) return undefined
    if (record.marker.isDisposed || record.marker.line !== bufferLine) {
      recordsByLine.delete(bufferLine)
      return undefined
    }
    return record
  }

  function logicalRecordAt(buffer: TerminalBuffer, bufferLine: number): TimestampRecord | undefined {
    const direct = recordAt(bufferLine)
    if (direct) return direct

    let index = bufferLine
    while (index > 0) {
      const line = buffer.getLine(index)
      if (!line?.isWrapped) break
      index -= 1
      const parent = recordAt(index)
      if (parent) return parent
    }
    return undefined
  }

  function disposeRecordAt(bufferLine: number): boolean {
    const record = recordAt(bufferLine)
    if (!record) return false
    record.marker.dispose()
    recordsByLine.delete(bufferLine)
    return true
  }

  function registerRecord(buffer: TerminalBuffer, bufferLine: number, timestamp: number): boolean {
    if (!terminal || bufferLine < 0 || bufferLine >= buffer.length) return false
    if (recordAt(bufferLine)) return false

    const cursorLine = buffer.baseY + buffer.cursorY
    const marker = terminal.registerMarker(bufferLine - cursorLine)
    if (!marker || marker.isDisposed) return false
    const record = { marker, timestamp }
    records.push(record)
    recordsByLine.set(bufferLine, record)
    return true
  }

  function setSnapshotAnchor(buffer: TerminalBuffer): void {
    disposeSnapshotAnchor()
    if (!terminal || buffer.length === 0 || buffer.type !== 'normal') return
    const cursorLine = Math.max(0, Math.min(buffer.length - 1, buffer.baseY + buffer.cursorY))
    const marker = terminal.registerMarker(0)
    if (!marker || marker.isDisposed) return
    snapshotAnchor = marker
    snapshotAnchorLine = cursorLine
  }

  function trimmedRowsSinceSnapshot(): number | undefined {
    if (!snapshotAnchor || snapshotAnchor.isDisposed || snapshotAnchor.line < 0) return undefined
    return Math.max(0, snapshotAnchorLine - snapshotAnchor.line)
  }

  function resetForClear(): void {
    const hadRecords = records.length > 0
    disposeRecords()
    pendingClear = false
    if (hadRecords) bumpRevision()
  }

  function currentNormalBuffer(): TerminalBuffer | undefined {
    return terminal?.buffer?.normal
  }

  function cursorLine(buffer: TerminalBuffer): number {
    return Math.max(0, Math.min(buffer.length - 1, buffer.baseY + buffer.cursorY))
  }

  function resetDirtyRange(buffer: TerminalBuffer): void {
    lastCursorLine = buffer.length > 0 ? cursorLine(buffer) : 0
    dirtyLineFeeds = 0
  }

  function snapshotRange(buffer: TerminalBuffer, forceFull: boolean): { start: number; end: number } {
    if (forceFull || buffer.length <= 0) return { start: 0, end: Math.max(0, buffer.length - 1) }

    const screenStart = Math.max(0, Math.min(buffer.length - 1, buffer.baseY))
    const screenRows = Math.max(1, terminal?.rows ?? 1)
    const screenEnd = Math.min(buffer.length - 1, screenStart + screenRows - 1)
    const currentLine = cursorLine(buffer)
    const newRowsStart = Math.max(0, Math.min(lastCursorLine, currentLine) - dirtyLineFeeds - 1)
    const newRowsEnd = Math.min(buffer.length - 1, Math.max(lastCursorLine, currentLine) + 1)
    return { start: Math.min(screenStart, newRowsStart), end: Math.max(screenEnd, newRowsEnd) }
  }

  function markChangedLines(timestamp: number, clearWasApplied = false, reconcileAlternate = false): void {
    const instance = terminal
    const buffer = currentNormalBuffer()
    if (!instance || !buffer || instance.buffer.active.type === 'alternate') return

    pruneRecords()

    if (needsSnapshotRefresh && snapshotReady && !reconcileAlternate) {
      rebuildSnapshot(buffer)
      needsSnapshotRefresh = false
      setSnapshotAnchor(buffer)
      resetDirtyRange(buffer)
      bumpRevision()
      return
    }
    const trimmed = trimmedRowsSinceSnapshot()
    const anchorWasLost = snapshotReady && snapshotAnchor !== null && snapshotAnchor.isDisposed
    const hasStructuralReset =
      (snapshotReady && buffer.baseY < snapshotBaseY && !clearWasApplied) ||
      (snapshotReady && buffer.length < snapshotLength && !clearWasApplied)
    if (hasStructuralReset) {
      rebuildSnapshot(buffer)
      setSnapshotAnchor(buffer)
      resetDirtyRange(buffer)
      return
    }

    const forceFull = clearWasApplied || !snapshotReady || anchorWasLost
    const comparison =
      anchorWasLost || clearWasApplied ? new Map<number, LineSnapshot>() : forceFull ? new Map(snapshot) : snapshot

    const comparisonOffset = forceFull
      ? anchorWasLost || clearWasApplied
        ? 0
        : snapshotKeyOffset
      : snapshotKeyOffset + (trimmed ?? 0)
    if (forceFull) {
      snapshot.clear()
      snapshotKeyOffset = 0
    } else if (trimmed !== undefined && trimmed > 0) {
      for (let index = 0; index < trimmed; index += 1) snapshot.delete(snapshotKeyOffset + index)
      snapshotKeyOffset += trimmed
    }
    const range = snapshotRange(buffer, forceFull)
    let changed = false

    for (let index = range.start; index <= range.end; index += 1) {
      const key = index + snapshotKeyOffset
      const oldLine = comparison.get(index + comparisonOffset)
      const nextLine = lineSnapshot(buffer.getLine(index))
      if (!hasContent(nextLine)) {
        snapshot.delete(key)
        if (disposeRecordAt(index)) changed = true
        continue
      }

      snapshot.set(key, nextLine)

      const contentChanged = !oldLine || oldLine.text !== nextLine?.text || oldLine.isWrapped !== nextLine?.isWrapped
      if (!contentChanged) continue

      if (nextLine?.isWrapped && logicalRecordAt(buffer, index)) continue
      if (registerRecord(buffer, index, timestamp)) changed = true
    }

    snapshotLength = buffer.length
    snapshotBaseY = buffer.baseY
    snapshotReady = true
    needsSnapshotRefresh = false
    setSnapshotAnchor(buffer)
    resetDirtyRange(buffer)

    if (trimmed !== undefined && trimmed > 0) changed = true
    if (pruneRecords()) changed = true
    if (changed) bumpRevision()
  }

  function notifyOutputWrite(timestamp: number | Date = Date.now()): void {
    pendingOutputTimes.push(normalizeTimestamp(timestamp))
    if (pendingOutputTimes.length > 32) pendingOutputTimes.splice(0, pendingOutputTimes.length - 32)
  }

  function markParsedOutput(timestamp: number | Date = Date.now()): void {
    const queued = pendingOutputTimes.shift()
    const outputTime = normalizeTimestamp(queued ?? timestamp)
    if (terminal?.buffer.active.type === 'alternate') {
      alternateEnteredAt = Math.min(alternateEnteredAt ?? outputTime, outputTime)
      return
    }
    const clearWasApplied = pendingClear && terminal?.buffer.active.type === 'normal'
    if (clearWasApplied) resetForClear()
    else pendingClear = false
    markChangedLines(outputTime, clearWasApplied)
  }

  function notifyCleared(): void {
    if (terminal?.buffer.active.type === 'alternate') return
    resetForClear()
    const buffer = currentNormalBuffer()
    if (!buffer) return
    rebuildSnapshot(buffer)
    needsSnapshotRefresh = false
    setSnapshotAnchor(buffer)
    resetDirtyRange(buffer)
  }

  function refreshUnmarkedWrite(): void {
    const instance = terminal
    const buffer = currentNormalBuffer()
    if (!instance || !buffer || instance.buffer.active.type === 'alternate') return
    rebuildSnapshot(buffer)
    pruneRecords()
    needsSnapshotRefresh = false
    setSnapshotAnchor(buffer)
    resetDirtyRange(buffer)
  }

  function timestampAt(bufferLine: number): number | undefined {
    const instance = terminal
    const buffer = currentNormalBuffer()
    if (!instance || !buffer || instance.buffer.active.type === 'alternate') return undefined
    if (!Number.isInteger(bufferLine) || bufferLine < 0 || bufferLine >= buffer.length) return undefined
    return logicalRecordAt(buffer, bufferLine)?.timestamp
  }

  function attach(instance: Terminal): void {
    if (terminal !== instance) {
      detach()
    } else {
      for (const subscription of subscriptions) subscription.dispose()
      subscriptions = []
    }
    terminal = instance
    activeWasAlternate = instance.buffer.active.type === 'alternate'
    const buffer = currentNormalBuffer()
    if (buffer && !snapshotReady) {
      rebuildSnapshot(buffer)
      setSnapshotAnchor(buffer)
    }

    subscriptions.push(
      instance.onResize(() => {
        if (instance.buffer.active.type === 'normal') refreshUnmarkedWrite()
        else needsSnapshotRefresh = true
        bumpRevision()
      }),
    )
    subscriptions.push(
      instance.onLineFeed(() => {
        const buffer = currentNormalBuffer()
        if (!buffer || instance.buffer.active.type === 'alternate') return
        dirtyLineFeeds += 1
      }),
    )
    subscriptions.push(
      instance.buffer.onBufferChange((buffer) => {
        if (buffer.type === 'alternate') {
          activeWasAlternate = true
          alternateEnteredAt = Date.now()
          resetDirtyRange(buffer)
          bumpRevision()
          return
        }
        if (activeWasAlternate) {
          activeWasAlternate = false

          const clearWasApplied = pendingClear
          if (clearWasApplied) resetForClear()
          if (alternateEnteredAt !== undefined) markChangedLines(alternateEnteredAt, clearWasApplied, true)
          alternateEnteredAt = undefined
          refreshUnmarkedWrite()
          bumpRevision()
          return
        }
        needsSnapshotRefresh = true
        bumpRevision()
      }),
    )

    const parser = instance.parser as unknown as {
      registerCsiHandler?: (
        identifier: { final: string },
        handler: (params: (number | number[])[]) => boolean,
      ) => { dispose(): void }
    }
    if (typeof parser.registerCsiHandler === 'function') {
      const disposable = parser.registerCsiHandler({ final: 'J' }, (params) => {
        if (params[0] === 3 && instance.buffer.active.type === 'normal') pendingClear = true
        return false
      })
      if (disposable && typeof disposable.dispose === 'function') subscriptions.push(disposable)
    }
  }

  function detach(): void {
    for (const subscription of subscriptions) subscription.dispose()
    subscriptions = []
    disposeSnapshotAnchor()
    disposeRecords()
    terminal = null
    snapshot = new Map<number, LineSnapshot>()
    snapshotKeyOffset = 0
    snapshotLength = 0
    snapshotBaseY = 0
    snapshotAnchorLine = 0
    snapshotReady = false
    needsSnapshotRefresh = false
    pendingClear = false
    pendingOutputTimes = []
    activeWasAlternate = false
    alternateEnteredAt = undefined
    bumpRevision()
  }

  function clear(): void {
    notifyCleared()
  }

  return {
    revision,
    attach,
    detach,
    notifyOutputWrite,
    markParsedOutput,
    markUntimedWrite: refreshUnmarkedWrite,
    notifyCleared,
    clear,
    timestampAt,

    lookup: timestampAt,
  }
}
