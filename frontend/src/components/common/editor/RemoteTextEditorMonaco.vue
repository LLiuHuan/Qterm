<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as monaco from 'monaco-editor/editor/editor.api'
import editorWorkerUrl from 'monaco-editor/editor/editor.worker?worker&url'

import 'monaco-editor/editor/browser/coreCommands'
import 'monaco-editor/editor/contrib/bracketMatching/browser/bracketMatching'
import 'monaco-editor/editor/contrib/clipboard/browser/clipboard'
import 'monaco-editor/editor/contrib/cursorUndo/browser/cursorUndo'

import 'monaco-editor/editor/contrib/folding/browser/folding'
import 'monaco-editor/editor/contrib/stickyScroll/browser/stickyScrollContribution'
import 'monaco-editor/editor/contrib/indentation/browser/indentation'
import 'monaco-editor/editor/contrib/linesOperations/browser/linesOperations'
import 'monaco-editor/editor/contrib/multicursor/browser/multicursor'
import 'monaco-editor/editor/contrib/wordOperations/browser/wordOperations'
import { normalizeEditorLanguage, type EditorLanguage, type RemoteTextEditorProps } from './types'

const props = withDefaults(defineProps<RemoteTextEditorProps>(), {
  readonly: false,
  theme: 'light',
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
  save: []
  dirtyChange: [dirty: boolean]

  ready: []
}>()

const host = ref<HTMLElement | null>(null)
let editor: monaco.editor.IStandaloneCodeEditor | null = null
let model: monaco.editor.ITextModel | null = null
let disposed = false

let baseline = ''

let applyingExternal = false

const globalScope = self as unknown as {
  MonacoEnvironment?: { getWorker(workerId: string, label: string): Worker }
  __qtermRemoteEditorProbe?: () => Record<string, unknown>
  __qtermRemoteEditorInstance?: monaco.editor.IStandaloneCodeEditor | null
}

let workerCreated = false

function installMonacoEnvironment(): void {
  if (globalScope.MonacoEnvironment) return
  globalScope.MonacoEnvironment = {
    getWorker() {
      workerCreated = true
      return new Worker(editorWorkerUrl, { type: 'module', name: 'qterm-editor' })
    },
  }
}

function installProbe(): void {
  globalScope.__qtermRemoteEditorInstance = null
  globalScope.__qtermRemoteEditorProbe = () => ({
    engine: 'monaco',
    monacoWorkerCreated: workerCreated,
    monacoModels: monaco.editor.getModels().length,
    hasModelEnvironment: Boolean(globalScope.MonacoEnvironment),

    modelLanguage: model && !model.isDisposed() ? model.getLanguageId() : null,
    registeredLanguages: [...registeredLanguages],
  })
}

const LIGHT_THEME = 'qterm-remote-light'
const DARK_THEME = 'qterm-remote-dark'

const TOKEN_SELECTORS = ['--qt-app', '--qt-subtle', '--qt-hover'] as const

let colorCanvas: HTMLCanvasElement | null = null

const SENTINEL_RGB = { red: 1, green: 2, blue: 3 }

function rasterizeColor(value: string): string | null {
  if (!value) return null
  if (!colorCanvas) {
    colorCanvas = document.createElement('canvas')
    colorCanvas.width = 1
    colorCanvas.height = 1
  }
  const context = colorCanvas.getContext('2d', { willReadFrequently: true })

  if (!context) return null
  try {
    context.fillStyle = `rgb(${SENTINEL_RGB.red}, ${SENTINEL_RGB.green}, ${SENTINEL_RGB.blue})`
    context.fillStyle = value
  } catch {
    return null
  }
  context.clearRect(0, 0, 1, 1)
  context.fillRect(0, 0, 1, 1)
  const [red = 0, green = 0, blue = 0, alpha = 255] = context.getImageData(0, 0, 1, 1).data
  if (red === SENTINEL_RGB.red && green === SENTINEL_RGB.green && blue === SENTINEL_RGB.blue) return null
  const hex = (channel: number): string => channel.toString(16).padStart(2, '0')
  const base = `#${hex(red)}${hex(green)}${hex(blue)}`
  return alpha >= 255 ? base : `${base}${hex(alpha)}`
}

function resolveThemeScope(): HTMLElement {
  const scopeClasses = ['is-dark', 'has-page-background', 'has-terminal-theme']
  let current: HTMLElement | null = host.value
  while (current) {
    const element: HTMLElement = current
    const matches =
      element.classList.contains('qterm-shell') || scopeClasses.some((name) => element.classList.contains(name))
    if (matches) return element
    current = element.parentElement
  }
  return document.documentElement
}

function resolvePalette(): Record<string, string> {
  const scope = resolveThemeScope()
  const scopeStyle = getComputedStyle(scope)
  const rootStyle = scope === document.documentElement ? scopeStyle : getComputedStyle(document.documentElement)
  const palette: Record<string, string> = {}
  for (const token of TOKEN_SELECTORS) {
    const raw = scopeStyle.getPropertyValue(token).trim() || rootStyle.getPropertyValue(token).trim()
    const color = rasterizeColor(raw)
    if (color) palette[token] = color
  }
  return palette
}

const LIGHT_FALLBACK: Record<string, string> = {
  '--qt-app': '#ffffff',
  '--qt-subtle': '#f5f5f5',
  '--qt-hover': '#0e0e0e12',
}
const DARK_FALLBACK: Record<string, string> = {
  '--qt-app': '#171717',
  '--qt-subtle': '#262626',
  '--qt-hover': '#ffffff12',
}

function colorOf(palette: Record<string, string>, token: string, dark: boolean): string {
  return palette[token] ?? (dark ? DARK_FALLBACK : LIGHT_FALLBACK)[token] ?? '#000000'
}

const HEX_COLOR = /^#(?:[0-9a-f]{3}|[0-9a-f]{4}|[0-9a-f]{6}|[0-9a-f]{8})$/i
const TRANSPARENT = '#00000000'

function themeColor(value: string | undefined): string {
  return value && HEX_COLOR.test(value.trim()) ? value.trim() : TRANSPARENT
}

function applyTheme(): void {
  const dark = props.theme === 'dark'
  const palette = resolvePalette()
  const name = dark ? DARK_THEME : LIGHT_THEME
  monaco.editor.defineTheme(name, {
    base: dark ? 'vs-dark' : 'vs',
    inherit: true,

    rules: [],
    colors: {
      'editor.background': themeColor(colorOf(palette, '--qt-app', dark)),
      'editor.lineHighlightBackground': themeColor(colorOf(palette, '--qt-hover', dark)),
      'minimap.background': themeColor(colorOf(palette, '--qt-subtle', dark)),
      'scrollbar.shadow': TRANSPARENT,
      'editorOverviewRuler.border': TRANSPARENT,
    },
  })
  monaco.editor.setTheme(name)
}

function resolveFontSize(): number {
  const probe = document.createElement('span')
  probe.style.cssText = 'position:absolute;visibility:hidden;font-size:0.75rem'
  document.body.appendChild(probe)
  const size = Number.parseFloat(getComputedStyle(probe).fontSize)
  probe.remove()
  return Number.isFinite(size) && size > 0 ? Math.round(size) : 12
}

const LANGUAGE_LOADERS: Record<Exclude<EditorLanguage, 'plaintext'>, () => Promise<unknown>> = {
  python: () => import('monaco-editor/languages/definitions/python/register'),
  shell: () => import('monaco-editor/languages/definitions/shell/register'),
  yaml: () => import('monaco-editor/languages/definitions/yaml/register'),
  ini: () => import('monaco-editor/languages/definitions/ini/register'),
  toml: () => import('monaco-editor/languages/definitions/ini/register'),
  lua: () => import('monaco-editor/languages/definitions/lua/register'),
  xml: () => import('monaco-editor/languages/definitions/xml/register'),
  html: () => import('monaco-editor/languages/definitions/html/register'),
  markdown: () => import('monaco-editor/languages/definitions/markdown/register'),
  dockerfile: () => import('monaco-editor/languages/definitions/dockerfile/register'),
  sql: () => import('monaco-editor/languages/definitions/sql/register'),
  javascript: () => import('monaco-editor/languages/definitions/javascript/register'),
  typescript: () => import('monaco-editor/languages/definitions/typescript/register'),
  go: () => import('monaco-editor/languages/definitions/go/register'),
  cpp: () => import('monaco-editor/languages/definitions/cpp/register'),
  rust: () => import('monaco-editor/languages/definitions/rust/register'),
  php: () => import('monaco-editor/languages/definitions/php/register'),
  ruby: () => import('monaco-editor/languages/definitions/ruby/register'),
  perl: () => import('monaco-editor/languages/definitions/perl/register'),
}
const registeredLanguages = new Set<EditorLanguage>()

async function ensureLanguageRegistered(language: Exclude<EditorLanguage, 'plaintext'>): Promise<void> {
  if (registeredLanguages.has(language)) return
  await LANGUAGE_LOADERS[language]()
  registeredLanguages.add(language)
}

async function applyLanguage(target: monaco.editor.ITextModel, language: EditorLanguage): Promise<void> {
  if (language === 'plaintext') {
    monaco.editor.setModelLanguage(target, 'plaintext')
    return
  }
  try {
    await ensureLanguageRegistered(language)

    if (target.isDisposed()) return
    monaco.editor.setModelLanguage(target, language)
  } catch (error) {
    if (!target.isDisposed()) monaco.editor.setModelLanguage(target, 'plaintext')
    if (import.meta.env.DEV) console.warn('[qterm] monaco language failed to load', language, error)
  }
}

function readContent(): string {
  return model && !model.isDisposed() ? model.getValue() : ''
}

function publishDirty(): void {
  emit('dirtyChange', readContent() !== baseline)
}

function replaceContent(value: string): void {
  if (!model || model.isDisposed() || model.getValue() === value) return
  applyingExternal = true
  model.setValue(value)
  applyingExternal = false

  baseline = value
  publishDirty()
}

onMounted(async () => {
  installMonacoEnvironment()
  installProbe()
  if (!host.value) return
  baseline = props.modelValue
  const language = normalizeEditorLanguage(props.language)

  let modelLanguageId: string = 'plaintext'
  if (language !== 'plaintext') {
    try {
      await ensureLanguageRegistered(language)
      modelLanguageId = language
    } catch (error) {
      if (import.meta.env.DEV) console.warn('[qterm] monaco language failed to load', language, error)
    }
  }
  if (disposed || !host.value) return
  model = monaco.editor.createModel(props.modelValue, modelLanguageId)
  editor = monaco.editor.create(host.value, {
    model,
    theme: props.theme === 'dark' ? DARK_THEME : LIGHT_THEME,
    readOnly: props.readonly,

    links: false,
    wordBasedSuggestions: 'off',
    unicodeHighlight: { ambiguousCharacters: false, invisibleCharacters: false },
    contextmenu: false,

    automaticLayout: true,

    minimap: { enabled: true, renderCharacters: false, maxColumn: 90, showSlider: 'mouseover' },
    scrollBeyondLastLine: false,
    renderWhitespace: 'selection',
    tabSize: 2,
    insertSpaces: true,
    folding: false,
    lineNumbersMinChars: 3,
    fontSize: resolveFontSize(),

    fontFamily: "'SF Mono', SFMono-Regular, Menlo, Consolas, 'Liberation Mono', monospace",
    wordWrap: 'off',
    smoothScrolling: false,
    fixedOverflowWidgets: true,

    stickyScroll: {
      enabled: true,
    },
  })
  globalScope.__qtermRemoteEditorInstance = editor

  applyTheme()
  editor.onDidChangeModelContent(() => {
    if (applyingExternal) return
    emit('update:modelValue', readContent())
    publishDirty()
  })
  if (import.meta.env.DEV) {
    console.info(
      `[qterm] remote editor ready: engine=monaco workerCreated=${workerCreated} language=${language} models=${monaco.editor.getModels().length}`,
    )
  }

  emit('ready')
})

watch(
  () => props.modelValue,
  (value) => {
    replaceContent(value)
  },
)

watch(
  () => props.readonly,
  (value) => {
    editor?.updateOptions({ readOnly: value })
  },
)

watch(
  () => props.language,
  async (value) => {
    const language = normalizeEditorLanguage(value)
    if (!model || model.isDisposed()) return
    if (model.getLanguageId() === language) return
    await applyLanguage(model, language)
  },
)

watch(
  () => props.theme,
  () => {
    applyTheme()
  },
)

function disposeEditor(): void {
  if (disposed) return
  disposed = true
  const active = document.activeElement
  if (active instanceof HTMLElement && host.value?.contains(active)) active.blur()
  editor?.dispose()
  editor = null
  globalScope.__qtermRemoteEditorInstance = null

  if (model && !model.isDisposed()) model.dispose()
  model = null
}

onBeforeUnmount(disposeEditor)

defineExpose({
  focus() {
    editor?.focus()
  },
  dispose() {
    disposeEditor()
  },
})
</script>

<template>
  <div ref="host" class="remote-text-editor-monaco" data-shortcut-passthrough="true" />
</template>

<style scoped>
.remote-text-editor-monaco {
  width: 100%;
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

.remote-text-editor-monaco :deep(.monaco-editor),
.remote-text-editor-monaco :deep(.monaco-editor .overflow-guard) {
  border-radius: inherit;
}

.remote-text-editor-monaco :deep(.minimap) {
  background: var(--qt-subtle);
}
</style>
