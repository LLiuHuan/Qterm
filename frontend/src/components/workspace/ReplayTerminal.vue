<script setup lang="ts">
import { FitAddon } from '@xterm/addon-fit'
import { Terminal } from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { getTerminalTheme, type TerminalTheme, type TerminalThemeId } from '../../features/sessions/terminalThemes'
import { useI18n } from '@/locales'
import { REPLAY_FALLBACK_COLUMNS, REPLAY_FALLBACK_ROWS, computeStaticGrid, shouldRecalcGrid } from './replayGeometry'

const props = withDefaults(
  defineProps<{
    text: string
    columns: number
    rows: number

    dark?: boolean
    lightTheme?: TerminalThemeId
    darkTheme?: TerminalThemeId
  }>(),
  { dark: false, lightTheme: 'default-light', darkTheme: 'default-dark' },
)
const { t } = useI18n()

const host = ref<HTMLElement | null>(null)

let terminal: Terminal | null = null
let fitAddon: FitAddon | null = null
let resizeObserver: ResizeObserver | null = null
let themeUpdatedHandler: (() => void) | null = null
let renderedLength = 0

let previousHostWidth = 0
let previousHostHeight = 0

function currentTheme(): TerminalTheme {
  return getTerminalTheme(
    props.dark ? (props.darkTheme ?? 'default-dark') : (props.lightTheme ?? 'default-light'),
    props.dark ? 'default-dark' : 'default-light',
  )
}

function fitTerminal(): void {
  if (!terminal || !host.value) return
  try {
    fitAddon?.fit()
    const proposed = fitAddon?.proposeDimensions()
    if (!proposed) return
    const recordedCols = props.columns > 0 ? props.columns : REPLAY_FALLBACK_COLUMNS
    const recordedRows = props.rows > 0 ? props.rows : REPLAY_FALLBACK_ROWS

    const cell = terminal.dimensions?.css.cell
    const safe =
      cell && cell.width > 0 && cell.height > 0
        ? computeStaticGrid(
            host.value.clientWidth,
            host.value.clientHeight,
            recordedCols,
            recordedRows,
            cell.width,
            cell.height,
          )
        : null
    const cols = Math.max(recordedCols, Math.min(proposed.cols, safe?.columns ?? proposed.cols))

    const rows = Math.max(1, Math.min(proposed.rows, safe?.rows ?? proposed.rows))
    if (cols !== terminal.cols || rows !== terminal.rows) {
      terminal.resize(cols, rows)
    }
  } catch {}
}

function syncText(): void {
  void doSyncText()
}

async function doSyncText(): Promise<void> {
  await nextTick()
  if (!terminal) return
  if (props.text.length < renderedLength) {
    terminal.reset()
    renderedLength = 0
  }
  const delta = props.text.slice(renderedLength)
  if (delta) terminal.write(delta)
  renderedLength = props.text.length
}

onMounted(() => {
  if (!host.value) return
  fitAddon = new FitAddon()
  terminal = new Terminal({
    convertEol: true,
    cursorBlink: false,
    disableStdin: true,
    fontSize: 12,
    fontFamily: 'monospace',

    scrollback: 5000,
    theme: currentTheme(),
  })
  terminal.loadAddon(fitAddon)
  terminal.open(host.value)

  fitTerminal()
  void doSyncText()

  previousHostWidth = host.value.clientWidth
  previousHostHeight = host.value.clientHeight
  if (typeof ResizeObserver === 'function') {
    resizeObserver = new ResizeObserver(() => {
      if (!host.value) return
      const w = host.value.clientWidth
      const h = host.value.clientHeight
      if (!shouldRecalcGrid(w, h, previousHostWidth, previousHostHeight)) return
      previousHostWidth = w
      previousHostHeight = h
      fitTerminal()
    })
    resizeObserver.observe(host.value)
  }

  themeUpdatedHandler = () => {
    if (terminal) terminal.options.theme = currentTheme()
  }
  window.addEventListener('qterm:terminal-theme-updated', themeUpdatedHandler)
})

watch(
  () => props.text,
  () => syncText(),
)
watch(
  () => [props.columns, props.rows],
  () => {
    fitTerminal()
  },
)
watch(
  () => [props.dark, props.lightTheme, props.darkTheme],
  () => {
    if (terminal) terminal.options.theme = currentTheme()
  },
)

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  resizeObserver = null
  if (themeUpdatedHandler) window.removeEventListener('qterm:terminal-theme-updated', themeUpdatedHandler)
  themeUpdatedHandler = null
  terminal?.dispose()
  terminal = null
  fitAddon = null
})
</script>

<template>
  <div
    ref="host"
    class="history-replay-terminal"
    :style="{ background: currentTheme().background, color: currentTheme().foreground }"
    :aria-label="t('history.replay.title')"
  />
</template>

<style scoped>
.history-replay-terminal :deep(.xterm) {
  font-size: 12px;
  font-family: monospace;
}
</style>
