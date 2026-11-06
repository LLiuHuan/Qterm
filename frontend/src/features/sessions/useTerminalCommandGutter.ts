import type { IMarker, Terminal } from '@xterm/xterm'
import { shallowRef } from 'vue'
import type { GutterRow } from './terminalGutter'
import {
  parseShellIntegrationEvent,
  type ShellIntegrationEvent,
  type ShellIntegrationPhase,
  type ShellIntegrationProtocol,
} from './terminalShellIntegration'

type CommandRecord = {
  marker: IMarker

  outputMarker: IMarker | null

  finishedMarker: IMarker | null
  number: number
  startedAt: Date
  startedTick: number
  exitCode?: number
  durationMs?: number
  approximate: boolean
}

type CommandCycle = {
  inputMarker: IMarker | null
  phase: ShellIntegrationPhase
  record: CommandRecord | null
}

type RecentEvent = {
  phase: ShellIntegrationPhase
  source: ShellIntegrationProtocol

  eventKey: string
  cursorLine: number
  receivedAt: number
}

export function useTerminalCommandGutter(options: { visible: () => boolean; approximateTitle: () => string }) {
  const rows = shallowRef<GutterRow[]>([])
  let terminal: Terminal | null = null
  let subscriptions: Array<{ dispose(): void }> = []
  let records: CommandRecord[] = []
  let cycle: CommandCycle = { inputMarker: null, phase: 'A', record: null }
  let hasReliableEvent = false
  let fallbackSuspended = false
  let nextNumber = 1
  let frame: number | null = null
  let recentEvent: RecentEvent | null = null

  function sync(): void {
    const instance = terminal
    if (!instance || !options.visible() || instance.buffer.active.type === 'alternate') {
      rows.value = []
      return
    }
    const buffer = instance.buffer.active
    const viewport = buffer.viewportY
    const next: GutterRow[] = []

    const inputMarker = cycle.inputMarker
    if (inputMarker && !inputMarker.isDisposed) {
      const offset = inputMarker.line - viewport

      if (offset >= 0 && offset < instance.rows) next.push({ key: nextNumber, offset, lineNumber: nextNumber })
    }

    const retainedRecords: CommandRecord[] = []
    for (const record of records) {
      if (record.marker.isDisposed) {
        disposeRecordMarkers(record)
        continue
      }
      retainedRecords.push(record)
    }
    records = retainedRecords
    for (const record of records) {
      const offset = record.marker.line - viewport
      if (offset < 0 || offset >= instance.rows) continue
      if (record.approximate) {
        next.push({
          key: record.number,
          offset,
          lineNumber: record.number,
          label: `≈#${record.number}`,
          title: options.approximateTitle(),
        })
        continue
      }
      const time = record.startedAt.toLocaleTimeString()
      const duration = record.durationMs === undefined ? '' : ` · ${Math.round(record.durationMs)} ms`
      const status = record.exitCode === undefined ? '…' : record.exitCode === 0 ? '✓' : `✕${record.exitCode}`
      next.push({
        key: record.number,
        offset,
        lineNumber: record.number,
        label: `#${record.number} ${status}`,
        title: `${time}${duration}${record.exitCode === undefined ? '' : ` · exit ${record.exitCode}`}`,
      })
    }
    rows.value = next
  }

  function schedule(): void {
    if (frame !== null) return
    frame = window.requestAnimationFrame(() => {
      frame = null
      sync()
    })
  }

  function disposeMarker(marker: IMarker | null): void {
    if (marker && !marker.isDisposed) marker.dispose()
  }

  function disposeRecordMarkers(record: CommandRecord): void {
    disposeMarker(record.marker)
    disposeMarker(record.outputMarker)
    disposeMarker(record.finishedMarker)
  }

  function discardInputMarker(): void {
    disposeMarker(cycle.inputMarker)
    cycle.inputMarker = null
  }

  function discardRecord(record: CommandRecord | null): void {
    if (!record) return
    records = records.filter((item) => item !== record)
    disposeRecordMarkers(record)
  }

  function discardIncompleteCycle(): void {
    discardRecord(cycle.record)
    cycle.record = null
    discardInputMarker()
  }

  function clearRecords(): void {
    const retainedRecords = new Set(records)
    for (const record of records) disposeRecordMarkers(record)

    if (cycle.record && !retainedRecords.has(cycle.record)) disposeRecordMarkers(cycle.record)
    disposeMarker(cycle.inputMarker)
    records = []
    cycle = { inputMarker: null, phase: 'A', record: null }
    recentEvent = null
    rows.value = []
  }

  function isDuplicate(event: ShellIntegrationEvent, cursorLine: number, now: number): boolean {
    const eventKey = event.phase === 'D' ? String(event.exitCode ?? 'invalid') : ''
    const previous = recentEvent
    const duplicate =
      previous !== null &&
      previous.phase === event.phase &&
      previous.source !== event.source &&
      previous.eventKey === eventKey &&
      previous.cursorLine === cursorLine &&
      now - previous.receivedAt <= 100
    recentEvent = {
      phase: event.phase,
      source: event.source,
      eventKey,
      cursorLine,
      receivedAt: now,
    }
    return duplicate
  }

  function acceptReliableMode(): void {
    if (hasReliableEvent) return
    hasReliableEvent = true

    for (const record of records) {
      if (record.approximate) disposeMarker(record.marker)
    }
    records = records.filter((record) => !record.approximate)
  }

  function handlePromptStart(): void {
    if (cycle.record?.exitCode === undefined) discardIncompleteCycle()
    else discardInputMarker()
    cycle.phase = 'A'
    cycle.record = null
  }

  function handleInputStart(instance: Terminal): void {
    if (cycle.record?.exitCode === undefined) discardIncompleteCycle()
    else discardInputMarker()
    cycle.inputMarker = instance.registerMarker(0) ?? null
    cycle.phase = 'B'
    cycle.record = null
  }

  function handleCommandStart(instance: Terminal): void {
    if (cycle.phase === 'C' || cycle.phase === 'D') return
    const marker = cycle.inputMarker
    if (!marker || marker.isDisposed) {
      cycle.phase = 'C'
      return
    }
    const record: CommandRecord = {
      marker,
      outputMarker: instance.registerMarker(0) ?? null,
      finishedMarker: null,
      number: nextNumber++,
      startedAt: new Date(),
      startedTick: performance.now(),
      approximate: false,
    }
    records.push(record)
    cycle.record = record
    cycle.inputMarker = null
    cycle.phase = 'C'
  }

  function handleCommandFinish(instance: Terminal, event: ShellIntegrationEvent): void {
    const record = cycle.record
    if (!record || cycle.phase !== 'C') return
    if (event.exitCode === undefined) {
      discardIncompleteCycle()
      cycle.phase = 'D'
      return
    }
    record.exitCode = event.exitCode
    record.durationMs = Math.max(0, performance.now() - record.startedTick)
    record.finishedMarker = instance.registerMarker(0) ?? null
    cycle.record = null
    cycle.phase = 'D'
  }

  function onOsc(source: ShellIntegrationProtocol, data: string): boolean {
    const event = parseShellIntegrationEvent(source, data)
    if (!event) return false
    acceptReliableMode()

    const instance = terminal
    if (!instance) return true
    const now = performance.now()
    const cursorLine = instance.buffer.active.cursorY
    if (isDuplicate(event, cursorLine, now)) return true

    if (instance.buffer.active.type === 'alternate') return true

    switch (event.phase) {
      case 'A':
        handlePromptStart()
        break
      case 'B':
        handleInputStart(instance)
        break
      case 'C':
        handleCommandStart(instance)
        break
      case 'D':
        handleCommandFinish(instance, event)
        break
    }
    schedule()
    return true
  }

  function approximateEnter(hasInput: boolean): void {
    const instance = terminal
    if (!instance || hasReliableEvent || fallbackSuspended || !hasInput || instance.buffer.active.type === 'alternate')
      return
    const marker = instance.registerMarker(0)
    if (!marker) return
    records.push({
      marker,
      outputMarker: null,
      finishedMarker: null,
      number: nextNumber++,
      startedAt: new Date(),
      startedTick: performance.now(),
      approximate: true,
    })
    schedule()
  }

  function suspendFallback(): void {
    fallbackSuspended = true
  }

  function attach(instance: Terminal): void {
    detach()
    terminal = instance
    subscriptions.push(instance.parser.registerOscHandler(133, (data) => onOsc(133, data)))
    subscriptions.push(instance.parser.registerOscHandler(633, (data) => onOsc(633, data)))

    if (typeof instance.parser?.registerCsiHandler === 'function') {
      subscriptions.push(
        instance.parser.registerCsiHandler({ final: 'J' }, (params) => {
          if (params[0] === 3 && instance.buffer.active.type === 'normal') clearRecords()
          return false
        }),
      )
    }
    subscriptions.push(instance.onRender(schedule), instance.onScroll(schedule), instance.onResize(schedule))
    sync()
  }

  function detach(): void {
    if (frame !== null) window.cancelAnimationFrame(frame)
    frame = null
    for (const subscription of subscriptions) subscription.dispose()
    subscriptions = []
    clearRecords()
    hasReliableEvent = false
    fallbackSuspended = false
    nextNumber = 1
    terminal = null
    rows.value = []
  }

  return { rows, attach, detach, sync, approximateEnter, suspendFallback, notifyCleared: clearRecords }
}
