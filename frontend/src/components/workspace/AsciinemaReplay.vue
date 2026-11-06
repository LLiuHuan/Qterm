<script setup lang="ts">
import * as AsciinemaPlayer from 'asciinema-player'
import type { Player } from 'asciinema-player'
import 'asciinema-player/dist/bundle/asciinema-player.css'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { TerminalRecordingEvent } from '../../../bindings/changeme/internal/model/models'
import { getTerminalTheme, type TerminalTheme, type TerminalThemeId } from '../../features/sessions/terminalThemes'
import { useI18n } from '@/locales'
import { REPLAY_FALLBACK_COLUMNS, REPLAY_FALLBACK_ROWS, computeReplayGrid, shouldRecalcGrid } from './replayGeometry'

const props = withDefaults(
  defineProps<{
    events: TerminalRecordingEvent[]
    columns: number
    rows: number

    dark?: boolean
    lightTheme?: TerminalThemeId
    darkTheme?: TerminalThemeId
    speed?: number
    autoplay?: boolean

    controls?: boolean
  }>(),
  {
    dark: false,
    lightTheme: 'default-light',
    darkTheme: 'default-dark',
    speed: 1,
    autoplay: true,
    controls: false,
  },
)

const emit = defineEmits<{
  (event: 'ready', durationMs: number): void
  (event: 'play'): void
  (event: 'playing'): void
  (event: 'pause'): void
  (event: 'ended'): void
  (event: 'timeupdate', currentTimeMs: number, durationMs: number): void
  (event: 'error', error: unknown): void
}>()

const { t } = useI18n()
const host = ref<HTMLElement | null>(null)
const playerReady = ref(false)

const playerFailed = ref(false)

let player: Player | null = null

let pollFrame: number | undefined
let isPlaying = false
let generation = 0
let resizeObserver: ResizeObserver | null = null

let previousHostWidth = 0
let previousHostHeight = 0
let resizeDebounceTimer: number | undefined

const hostStyle = computed(() => {
  if (playerFailed.value) return { background: 'transparent', color: 'inherit' }
  const theme = currentTheme()
  return { background: theme.background, color: theme.foreground }
})

type PlayerEventListener = (eventName: string, handler: () => void) => void

function addPlayerListener(instance: Player, eventName: string, handler: () => void): void {
  ;(instance.addEventListener as unknown as PlayerEventListener)(eventName, handler)
}

function currentTheme(): TerminalTheme {
  return getTerminalTheme(
    props.dark ? props.darkTheme : props.lightTheme,
    props.dark ? 'default-dark' : 'default-light',
  )
}

function normalizeSpeed(value: number): number {
  return Number.isFinite(value) && value > 0 ? value : 1
}

function decodeBase64(data: string): string {
  if (!data) return ''
  try {
    const binary = globalThis.atob(data)
    const bytes = Uint8Array.from(binary, (character) => character.charCodeAt(0))
    return new TextDecoder().decode(bytes)
  } catch {
    return '\ufffd'
  }
}

function asciicastTheme(theme: TerminalTheme): { fg: string; bg: string; palette: string } {
  return {
    fg: theme.foreground,
    bg: theme.background,
    palette: [
      theme.black,
      theme.red,
      theme.green,
      theme.yellow,
      theme.blue,
      theme.magenta,
      theme.cyan,
      theme.white,
      theme.brightBlack,
      theme.brightRed,
      theme.brightGreen,
      theme.brightYellow,
      theme.brightBlue,
      theme.brightMagenta,
      theme.brightCyan,
      theme.brightWhite,
    ].join(':'),
  }
}

function createAsciicastData(): string {
  const recordedCols = props.columns > 0 ? props.columns : REPLAY_FALLBACK_COLUMNS
  const recordedRows = props.rows > 0 ? props.rows : REPLAY_FALLBACK_ROWS
  const theme = currentTheme()

  const element = host.value
  const grid =
    element && element.clientWidth > 0 && element.clientHeight > 0
      ? computeReplayGrid(element.clientWidth, element.clientHeight, recordedCols, recordedRows)
      : null
  const width = grid?.columns ?? recordedCols
  const height = grid?.rows ?? recordedRows
  const header = {
    version: 2,
    width,
    height,
    timestamp: 0,
    theme: asciicastTheme(theme),
  }
  const frames: string[] = []
  for (const event of props.events) {
    const atMs = Number(event.atMs)
    const time = Number.isFinite(atMs) && atMs >= 0 ? atMs / 1000 : 0
    if (event.type === 1 && event.data) {
      frames.push(JSON.stringify([time, 'o', decodeBase64(event.data)]))
    } else if (event.type === 2 && event.columns > 0 && event.rows > 0) {
      frames.push(JSON.stringify([time, 'r', `${event.columns}x${event.rows}`]))
    } else if (event.type === 4 && event.data) {
      frames.push(JSON.stringify([time, 'i', decodeBase64(event.data)]))
    }
  }
  return `${JSON.stringify(header)}\n${frames.join('\n')}${frames.length ? '\n' : ''}`
}

function currentTimeMs(): number {
  return Math.max(0, (player?.getCurrentTime() ?? 0) * 1000)
}

function durationMs(): number {
  return Math.max(0, (player?.getDuration() ?? 0) * 1000)
}

function emitTime(): void {
  emit('timeupdate', currentTimeMs(), durationMs())
}

function stopPolling(): void {
  if (pollFrame !== undefined) window.cancelAnimationFrame(pollFrame)
  pollFrame = undefined
}

function startPolling(): void {
  stopPolling()
  const tick = () => {
    if (!isPlaying) {
      pollFrame = undefined
      return
    }
    emitTime()
    pollFrame = window.requestAnimationFrame(tick)
  }
  pollFrame = window.requestAnimationFrame(tick)
}

function teardown(): void {
  stopPolling()
  playerReady.value = false
  isPlaying = false
  player?.dispose()
  player = null
}

function attachPlayerEvents(instance: Player, instanceGeneration: number): void {
  const isCurrent = () => instanceGeneration === generation && player === instance
  addPlayerListener(instance, 'ready', () => {
    if (!isCurrent()) return
    playerReady.value = true
    emit('ready', durationMs())
    emitTime()
  })
  addPlayerListener(instance, 'play', () => {
    if (!isCurrent()) return
    isPlaying = true
    startPolling()
    emit('play')
  })
  addPlayerListener(instance, 'playing', () => {
    if (!isCurrent()) return
    isPlaying = true
    startPolling()
    emit('playing')
  })
  addPlayerListener(instance, 'pause', () => {
    if (!isCurrent()) return
    isPlaying = false
    stopPolling()
    emitTime()
    emit('pause')
  })
  addPlayerListener(instance, 'ended', () => {
    if (!isCurrent()) return
    isPlaying = false
    stopPolling()
    emitTime()
    emit('ended')
  })
  addPlayerListener(instance, 'seeked', () => {
    if (!isCurrent()) return
    emitTime()
  })
  addPlayerListener(instance, 'error', () => {
    if (!isCurrent()) return
    emit('error', new Error('asciinema player failed to load the recording'))
  })
}

function removeInjectedNodes(element: HTMLElement, keep: readonly ChildNode[]): void {
  for (const node of Array.from(element.childNodes)) {
    if (!keep.includes(node)) node.remove()
  }
}

function createPlayer(): void {
  const element = host.value
  if (!element) return
  const instanceGeneration = ++generation
  teardown()

  if (!props.events.length) return
  const vueOwnedNodes = Array.from(element.childNodes)
  try {
    const recordedCols = props.columns > 0 ? props.columns : REPLAY_FALLBACK_COLUMNS
    const recordedRows = props.rows > 0 ? props.rows : REPLAY_FALLBACK_ROWS
    const grid =
      element.clientWidth > 0 && element.clientHeight > 0
        ? computeReplayGrid(element.clientWidth, element.clientHeight, recordedCols, recordedRows)
        : null
    const cols = grid?.columns ?? recordedCols
    const rows = grid?.rows ?? recordedRows
    const instance = AsciinemaPlayer.create({ data: createAsciicastData() }, element, {
      autoPlay: props.autoplay,
      controls: props.controls,
      speed: normalizeSpeed(props.speed),
      cols,
      rows,

      fit: 'none',
      terminalFontSize: '12px',
      terminalFontFamily: 'monospace',
      terminalLineHeight: 1,
      cursorMode: 'hidden',
      keystrokeOverlay: false,
    })
    player = instance
    playerFailed.value = false
    attachPlayerEvents(instance, instanceGeneration)

    window.setTimeout(() => {
      if (instanceGeneration !== generation || player !== instance || playerReady.value) return
      playerReady.value = true
      emit('ready', durationMs())
      emitTime()
    }, 0)
  } catch (error) {
    teardown()
    removeInjectedNodes(element, vueOwnedNodes)
    playerFailed.value = true
    emit('error', error)
  }
}

async function play(): Promise<boolean | void> {
  return player?.play()
}

async function pause(): Promise<boolean | void> {
  return player?.pause()
}

async function seek(milliseconds: number): Promise<boolean | void> {
  if (!player || !Number.isFinite(milliseconds)) return
  const target = Math.max(0, milliseconds) / 1000
  const result = await player.seek(target)
  emitTime()
  return result
}

function getCurrentTime(): number {
  return currentTimeMs()
}

function getDuration(): number {
  return durationMs()
}

defineExpose({ play, pause, seek, getCurrentTime, getDuration })

function handleResize(): void {
  const element = host.value
  if (!element) return
  const w = element.clientWidth
  const h = element.clientHeight
  if (!shouldRecalcGrid(w, h, previousHostWidth, previousHostHeight)) return
  previousHostWidth = w
  previousHostHeight = h
  if (resizeDebounceTimer !== undefined) clearTimeout(resizeDebounceTimer)
  resizeDebounceTimer = window.setTimeout(() => {
    resizeDebounceTimer = undefined
    const wasPlaying = isPlaying
    const time = currentTimeMs()
    createPlayer()
    if (!player || !wasPlaying) return
    void player.play().then(() => {
      if (time > 0) void player?.seek(time / 1000)
    })
  }, 150)
}

onMounted(() => {
  createPlayer()

  if (typeof ResizeObserver === 'function' && host.value) {
    previousHostWidth = host.value.clientWidth
    previousHostHeight = host.value.clientHeight
    resizeObserver = new ResizeObserver(() => handleResize())
    resizeObserver.observe(host.value)
  }
})

watch(
  () => [
    props.events,
    props.columns,
    props.rows,
    props.dark,
    props.lightTheme,
    props.darkTheme,
    props.speed,
    props.autoplay,
    props.controls,
  ],
  () => {
    const wasPlaying = isPlaying
    const time = currentTimeMs()
    createPlayer()
    if (!player || !wasPlaying) return
    void player.play().then(() => {
      if (time > 0) void player?.seek(time / 1000)
    })
  },
  { deep: false },
)

onBeforeUnmount(() => {
  generation += 1
  resizeObserver?.disconnect()
  resizeObserver = null
  if (resizeDebounceTimer !== undefined) clearTimeout(resizeDebounceTimer)
  resizeDebounceTimer = undefined
  teardown()
})
</script>

<template>
  <div
    ref="host"
    class="history-replay-terminal history-replay-asciinema"
    :class="{ 'is-failed': playerFailed }"
    :style="hostStyle"
    :aria-label="t('history.replay.title')"
    :data-ready="String(playerReady)"
  >
    <p v-if="playerFailed" class="history-replay-asciinema__failure text-xs" role="alert">
      {{ t('history.recording.replayFailure') }}
    </p>
  </div>
</template>

<style scoped>
.history-replay-asciinema {
  display: block;
  min-width: 0;
  padding: 0;

  overflow: auto;
}

.history-replay-asciinema__failure {
  margin: 0;
  padding: 0.75rem 1rem;
  color: var(--qt-danger, #b42318);
}

.history-replay-asciinema :deep(.ap-wrapper) {
  width: max-content;
  min-width: 100%;
  height: auto;
  min-height: 100%;
  justify-content: flex-start;
  align-items: flex-start;
}

.history-replay-asciinema :deep(.ap-player) {
  flex: none;
  max-width: none;
}
</style>
