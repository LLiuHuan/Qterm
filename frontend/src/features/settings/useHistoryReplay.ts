import { onBeforeUnmount, ref } from 'vue'
import { toast } from 'vue-sonner'
import type { TerminalRecordingEvent } from '../../../bindings/changeme/internal/model/models'
import {
  REPLAY_FALLBACK_COLUMNS,
  REPLAY_FALLBACK_ROWS,
  resolveReplayGrid,
} from '../../components/workspace/replayGeometry'
import { errorMessage } from '../../services/errors'
import { readRecording } from '../../services/recordings'
import type { translate } from '@/locales'

export interface DynamicReplayPlayer {
  play: () => Promise<boolean | void> | undefined
  pause: () => Promise<boolean | void> | undefined
  seek: (milliseconds: number) => Promise<boolean | void> | undefined
}

interface UseHistoryReplayOptions {
  t: typeof translate
}

export function useHistoryReplay(options: UseHistoryReplayOptions) {
  const replayOpen = ref(false)
  const replayText = ref('')
  const replayColumns = ref(REPLAY_FALLBACK_COLUMNS)
  const replayRows = ref(REPLAY_FALLBACK_ROWS)
  const replaySpeed = ref('1')
  const replayRunning = ref(false)
  const replaySuspended = ref(false)
  const replayRecordingId = ref('')

  const replayMode = ref<'static' | 'dynamic'>('static')
  const replayProgress = ref(0)
  const replayDurationMs = ref(0)
  const replayElapsedMs = ref(0)
  const replayEvents = ref<TerminalRecordingEvent[]>([])
  const replayLoading = ref(false)
  const replayFullscreen = ref(false)
  const dynamicReplayPlayer = ref<DynamicReplayPlayer | null>(null)

  let replayGeneration = 0
  let replayTimer: number | undefined
  let replayWake: (() => void) | undefined

  function decodeBase64(value: string): string {
    const binary = atob(value)
    return new TextDecoder().decode(Uint8Array.from(binary, (char) => char.charCodeAt(0)))
  }

  function clearReplayTimer(): void {
    if (replayTimer !== undefined) window.clearTimeout(replayTimer)
    replayTimer = undefined

    replayWake?.()
    replayWake = undefined
  }

  function waitReplayTick(durationMs: number): Promise<void> {
    return new Promise((resolve) => {
      replayWake = resolve
      replayTimer = window.setTimeout(() => {
        replayTimer = undefined
        replayWake = undefined
        resolve()
      }, durationMs)
    })
  }

  function stopReplay(): void {
    replayGeneration += 1
    replayRunning.value = false
    replaySuspended.value = false
    clearReplayTimer()

    void dynamicReplayPlayer.value?.pause()
  }

  function renderReplayAt(milliseconds: number): void {
    const target = Math.max(0, milliseconds)
    let text = ''

    let columns = REPLAY_FALLBACK_COLUMNS
    let rows = REPLAY_FALLBACK_ROWS
    for (const event of replayEvents.value) {
      if (Number(event.atMs) > target) break
      if (event.type === 2) {
        if (event.columns > 0) columns = event.columns
        if (event.rows > 0) rows = event.rows
      } else if (event.type === 1) {
        try {
          text += decodeBase64(event.data)
        } catch {
          text += '�'
        }
      }
    }
    replayText.value = text
    replayColumns.value = columns
    replayRows.value = rows
    replayElapsedMs.value = target
    replayProgress.value = replayDurationMs.value > 0 ? Math.round((target / replayDurationMs.value) * 100) : 0
  }

  function seekReplay(event: Event): void {
    if (replayMode.value === 'static' || replayLoading.value || replayDurationMs.value <= 0) return
    const value = Number((event.target as HTMLInputElement).value)
    if (replayMode.value === 'dynamic') {
      replayElapsedMs.value = (value / 100) * replayDurationMs.value
      replayProgress.value = value
      void dynamicReplayPlayer.value?.seek(replayElapsedMs.value)
      return
    }
    const suspended = !replayRunning.value || replaySuspended.value
    void playFrom((value / 100) * replayDurationMs.value, suspended)
  }

  function toggleReplayRunning(): void {
    if (replayMode.value === 'dynamic') {
      if (replayRunning.value) {
        replaySuspended.value = !replaySuspended.value
        if (replaySuspended.value) void dynamicReplayPlayer.value?.pause()
        else void dynamicReplayPlayer.value?.play()
      } else {
        replayRunning.value = true
        replaySuspended.value = false
        void dynamicReplayPlayer.value?.play()
      }
      return
    }
    if (!replayRunning.value) {
      void playFrom(replayElapsedMs.value >= replayDurationMs.value ? 0 : replayElapsedMs.value)
      return
    }
    replaySuspended.value = !replaySuspended.value
    clearReplayTimer()
  }

  function onDynamicReplayReady(durationMs: number): void {
    replayDurationMs.value = Math.max(replayDurationMs.value, durationMs)
    replayLoading.value = false
    replayRunning.value = true
    replaySuspended.value = false
    void dynamicReplayPlayer.value?.play()
  }

  function onDynamicReplayTimeupdate(currentMs: number, durationMs: number): void {
    replayElapsedMs.value = currentMs
    replayDurationMs.value = Math.max(replayDurationMs.value, durationMs)
    replayProgress.value = replayDurationMs.value > 0 ? (currentMs / replayDurationMs.value) * 100 : 0
  }

  function onDynamicReplayPlay(): void {
    replayRunning.value = true
    replaySuspended.value = false
  }

  function onDynamicReplayPause(): void {
    replaySuspended.value = true
  }

  function onDynamicReplayEnded(): void {
    replayRunning.value = false
    replaySuspended.value = false
    replayProgress.value = 100
  }

  function onDynamicReplayError(error: unknown): void {
    replayLoading.value = false
    replayRunning.value = false
    replaySuspended.value = false
    toast.error(errorMessage(error, options.t('history.recording.replayFailure')))
  }

  function toggleFullscreen(): void {
    replayFullscreen.value = !replayFullscreen.value
  }

  function closeReplayDialog(open: boolean): void {
    replayOpen.value = open
    if (!open) {
      stopReplay()
      replayLoading.value = false
      replayFullscreen.value = false
      replayText.value = ''
      replayColumns.value = REPLAY_FALLBACK_COLUMNS
      replayRows.value = REPLAY_FALLBACK_ROWS
      replayRecordingId.value = ''
      replayProgress.value = 0
      replayDurationMs.value = 0
      replayElapsedMs.value = 0
      replayEvents.value = []
    }
  }

  async function waitForNextEvent(generation: number, targetMs: number): Promise<void> {
    while (replayElapsedMs.value < targetMs && generation === replayGeneration) {
      if (replaySuspended.value) {
        await waitReplayTick(40)
        continue
      }
      const speed = Number(replaySpeed.value) || 1
      const startedAt = performance.now()
      await waitReplayTick(Math.min((targetMs - replayElapsedMs.value) / speed, 40))
      if (generation !== replayGeneration) return
      if (!replaySuspended.value) {
        replayElapsedMs.value = Math.min(targetMs, replayElapsedMs.value + (performance.now() - startedAt) * speed)
        replayProgress.value = replayDurationMs.value > 0 ? (replayElapsedMs.value / replayDurationMs.value) * 100 : 0
      }
    }

    while (replaySuspended.value && generation === replayGeneration) await waitReplayTick(40)
  }

  async function playFrom(milliseconds: number, suspended = false): Promise<void> {
    stopReplay()
    const generation = replayGeneration
    const target = Math.min(replayDurationMs.value, Math.max(0, milliseconds))
    renderReplayAt(target)
    replayRunning.value = true
    replaySuspended.value = suspended
    try {
      for (const event of replayEvents.value) {
        const atMs = Number(event.atMs) || 0
        if (atMs <= target) continue
        await waitForNextEvent(generation, atMs)
        if (generation !== replayGeneration) return
        if (event.type === 2) {
          if (event.columns > 0) replayColumns.value = event.columns
          if (event.rows > 0) replayRows.value = event.rows
        } else if (event.type === 1) {
          try {
            replayText.value += decodeBase64(event.data)
          } catch {
            replayText.value += '�'
          }
        }
      }
      if (generation === replayGeneration) replayProgress.value = 100
    } finally {
      if (generation === replayGeneration) {
        replayRunning.value = false
        replaySuspended.value = false
        clearReplayTimer()
      }
    }
  }

  async function replayRecording(id: string): Promise<void> {
    if (!id) return
    stopReplay()
    const generation = replayGeneration
    replayLoading.value = true
    replayText.value = ''
    replayColumns.value = REPLAY_FALLBACK_COLUMNS
    replayRows.value = REPLAY_FALLBACK_ROWS
    replayRecordingId.value = id
    replayEvents.value = []
    replayProgress.value = 0
    replayDurationMs.value = 0
    replayElapsedMs.value = 0
    replayOpen.value = true
    try {
      const events = await readRecording(id)
      if (generation !== replayGeneration) return
      replayEvents.value = events

      const grid = resolveReplayGrid(events)
      replayColumns.value = grid.columns
      replayRows.value = grid.rows
      replayDurationMs.value = events.reduce((latest, event) => {
        const atMs = Number(event.atMs)
        return Number.isFinite(atMs) ? Math.max(latest, atMs) : latest
      }, 0)
      replayLoading.value = false
      if (replayMode.value === 'static') {
        renderReplayAt(replayDurationMs.value)
        replayProgress.value = 100
      } else {
        replayRunning.value = false
        replaySuspended.value = false
      }
    } catch (cause) {
      if (generation === replayGeneration)
        toast.error(errorMessage(cause, options.t('history.recording.replayFailure')))
    } finally {
      if (generation === replayGeneration) replayLoading.value = false
    }
  }

  function formatPlaybackTime(milliseconds: number): string {
    const totalSeconds = Math.max(0, Math.floor(milliseconds / 1000))
    const minutes = Math.floor(totalSeconds / 60)
    const seconds = totalSeconds % 60
    return `${minutes}:${String(seconds).padStart(2, '0')}`
  }

  onBeforeUnmount(stopReplay)

  return {
    closeReplayDialog,
    dynamicReplayPlayer,
    formatPlaybackTime,
    onDynamicReplayEnded,
    onDynamicReplayError,
    onDynamicReplayPause,
    onDynamicReplayPlay,
    onDynamicReplayReady,
    onDynamicReplayTimeupdate,
    replayColumns,
    replayDurationMs,
    replayElapsedMs,
    replayEvents,
    replayFullscreen,
    replayLoading,
    replayMode,
    replayOpen,
    replayProgress,
    replayRecordingId,
    replayRows,
    replayRunning,
    replaySpeed,
    replaySuspended,
    replayText,
    replayRecording,
    seekReplay,
    stopReplay,
    toggleFullscreen,
    toggleReplayRunning,
  }
}
