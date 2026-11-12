import { reactive } from 'vue'

export type TmuxRuntimePhase = 'idle' | 'pending' | 'active'

export interface TmuxRuntimeState {
  phase: TmuxRuntimePhase
  sessionName: string
  token: string
}

const states = new Map<string, TmuxRuntimeState>()
const parserTails = new Map<string, string>()
const MARKER_PREFIX = '\u001b]777;qterm-tmux;'
const MAX_PARSER_TAIL = 256
const TOKEN_PATTERN = /^[0-9a-f]{32}$/

function createState(): TmuxRuntimeState {
  return reactive({ phase: 'idle', sessionName: '', token: '' }) as TmuxRuntimeState
}

export function getTmuxRuntimeState(sessionId: string): TmuxRuntimeState {
  const existing = states.get(sessionId)
  if (existing) return existing
  const state = createState()
  states.set(sessionId, state)
  return state
}

function newToken(): string {
  const bytes = new Uint8Array(16)
  try {
    crypto.getRandomValues(bytes)
  } catch {
    for (let index = 0; index < bytes.length; index += 1) bytes[index] = Math.floor(Math.random() * 256)
  }
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
}

export function beginTmuxEnter(sessionId: string, sessionName: string): string {
  const token = newToken()
  const state = getTmuxRuntimeState(sessionId)
  state.phase = 'pending'
  state.sessionName = sessionName
  state.token = token
  return token
}

export function cancelTmuxEnter(sessionId: string, token: string): void {
  const state = getTmuxRuntimeState(sessionId)
  if (state.phase !== 'pending' || state.token !== token) return
  resetState(state)
}

export function markTmuxExited(sessionId: string): void {
  const state = getTmuxRuntimeState(sessionId)
  if (state.phase === 'active') resetState(state)
}

export function clearTmuxRuntimeState(sessionId: string): void {
  resetState(getTmuxRuntimeState(sessionId))
  parserTails.delete(sessionId)
}

function resetState(state: TmuxRuntimeState): void {
  state.phase = 'idle'
  state.sessionName = ''
  state.token = ''
}

function handleMarker(sessionId: string, token: string, event: 'enter' | 'exit'): void {
  if (!TOKEN_PATTERN.test(token)) return
  const state = getTmuxRuntimeState(sessionId)
  if (state.token !== token) return
  if (event === 'enter' && state.phase === 'pending') {
    state.phase = 'active'
  } else if (event === 'exit' && (state.phase === 'pending' || state.phase === 'active')) {
    resetState(state)
  }
}

export function consumeTmuxRuntimeOutput(sessionId: string, data: Uint8Array): void {
  if (!sessionId || !data.length) return
  const decoder = new TextDecoder('latin1')
  let tail = parserTails.get(sessionId) ?? ''
  tail += decoder.decode(data)

  while (tail.length) {
    const start = tail.indexOf(MARKER_PREFIX)
    if (start < 0) {
      parserTails.set(sessionId, tail.slice(-MARKER_PREFIX.length + 1))
      return
    }
    tail = tail.slice(start + MARKER_PREFIX.length)
    const bel = tail.indexOf('\u0007')
    const st = tail.indexOf('\u001b\\')
    const end = bel >= 0 && st >= 0 ? Math.min(bel, st) : Math.max(bel, st)
    if (end < 0) {
      parserTails.set(sessionId, (MARKER_PREFIX + tail).slice(-MAX_PARSER_TAIL))
      return
    }
    const payload = tail.slice(0, end)
    tail = tail.slice(end + (st >= 0 && st === end ? 2 : 1))
    const separator = payload.indexOf(';')
    if (separator > 0)
      handleMarker(sessionId, payload.slice(0, separator), payload.slice(separator + 1) as 'enter' | 'exit')
  }
  parserTails.delete(sessionId)
}
