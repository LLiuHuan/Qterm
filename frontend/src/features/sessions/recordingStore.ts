import { reactive } from 'vue'
import { listRecordings } from '../../services/recordings'

export type RecordingDisplayState = 'none' | 'recording' | 'paused'

export type RecordingStateMode = 'segment' | 'full'

const recordingStates = reactive<Record<string, RecordingDisplayState>>({})

function storageKey(sessionId: string, mode: RecordingStateMode): string {
  return `${mode}:${sessionId}`
}

export function getRecordingState(sessionId: string, mode: RecordingStateMode = 'segment'): RecordingDisplayState {
  return recordingStates[storageKey(sessionId, mode)] ?? 'none'
}

export function setRecordingState(sessionId: string, mode: RecordingStateMode, state: RecordingDisplayState): void {
  recordingStates[storageKey(sessionId, mode)] = state
}

export function markRecordingStopped(sessionId: string, mode: RecordingStateMode = 'segment'): void {
  setRecordingState(sessionId, mode, 'none')
}

export async function refreshRecordingState(sessionId: string): Promise<void> {
  try {
    const item = (await listRecordings()).find(
      (entry) =>
        entry.sessionId === sessionId && entry.mode !== 'full' && ['recording', 'paused'].includes(entry.status),
    )
    setRecordingState(sessionId, 'segment', item?.status === 'paused' ? 'paused' : item ? 'recording' : 'none')
  } catch {}
}

export function resetRecordingStateStore(): void {
  for (const key of Object.keys(recordingStates)) delete recordingStates[key]
}
