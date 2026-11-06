<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands'
import {
  bracketMatching,
  HighlightStyle,
  indentOnInput,
  StreamLanguage,
  syntaxHighlighting,
  type StreamParser,
} from '@codemirror/language'
import { tags } from '@lezer/highlight'
import { Annotation, Compartment, EditorState, type Extension } from '@codemirror/state'
import { drawSelection, EditorView, highlightActiveLine, highlightSpecialChars, keymap } from '@codemirror/view'
import { normalizeEditorLanguage, type EditorLanguage, type RemoteTextEditorProps } from './types'

const props = withDefaults(defineProps<RemoteTextEditorProps>(), {
  readonly: false,
  theme: 'light',
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
  save: []
  dirtyChange: [boolean]

  ready: []
}>()

const host = ref<HTMLElement | null>(null)
let view: EditorView | null = null
let disposed = false

let baseline = ''

const externalChange = Annotation.define<boolean>()

const readonlyCompartment = new Compartment()

const languageCompartment = new Compartment()

const highlightStyle = HighlightStyle.define([
  { tag: tags.comment, color: 'var(--qt-muted)', fontStyle: 'italic' },
  { tag: [tags.keyword, tags.controlKeyword], color: 'var(--qt-accent)', fontWeight: '600' },
  { tag: tags.string, color: 'var(--qt-success)' },
  { tag: [tags.number, tags.bool], color: 'var(--qt-info)' },
  { tag: tags.variableName, color: 'var(--qt-text)' },
  { tag: tags.propertyName, color: 'var(--qt-text)' },
  { tag: [tags.typeName, tags.className], color: 'var(--qt-info)' },
  { tag: [tags.tagName, tags.heading], color: 'var(--qt-accent)' },
  { tag: [tags.attributeName, tags.link], color: 'var(--qt-info)' },
  { tag: [tags.operator, tags.punctuation], color: 'var(--qt-muted)' },
])

const PHP_KEYWORDS = Object.fromEntries(
  [
    'abstract',
    'and',
    'array',
    'as',
    'break',
    'callable',
    'case',
    'catch',
    'class',
    'clone',
    'const',
    'continue',
    'declare',
    'default',
    'do',
    'echo',
    'else',
    'elseif',
    'empty',
    'enddeclare',
    'endfor',
    'endforeach',
    'endif',
    'endswitch',
    'endwhile',
    'enum',
    'extends',
    'final',
    'finally',
    'fn',
    'for',
    'foreach',
    'function',
    'global',
    'goto',
    'if',
    'implements',
    'include',
    'include_once',
    'instanceof',
    'insteadof',
    'interface',
    'isset',
    'list',
    'match',
    'namespace',
    'new',
    'or',
    'print',
    'private',
    'protected',
    'public',
    'readonly',
    'require',
    'require_once',
    'return',
    'static',
    'switch',
    'throw',
    'trait',
    'try',
    'unset',
    'use',
    'var',
    'while',
    'xor',
    'yield',
  ].map((word) => [word, true]),
) as Record<string, boolean>

type ParserLoader = () => Promise<StreamParser<unknown> | null>

const PARSER_LOADERS: Record<Exclude<EditorLanguage, 'plaintext'>, ParserLoader> = {
  python: () => import('@codemirror/legacy-modes/mode/python').then((mode) => mode.python),
  shell: () => import('@codemirror/legacy-modes/mode/shell').then((mode) => mode.shell),
  yaml: () => import('@codemirror/legacy-modes/mode/yaml').then((mode) => mode.yaml),
  ini: () => import('@codemirror/legacy-modes/mode/properties').then((mode) => mode.properties),
  toml: () => import('@codemirror/legacy-modes/mode/toml').then((mode) => mode.toml),
  lua: () => import('@codemirror/legacy-modes/mode/lua').then((mode) => mode.lua),
  xml: () => import('@codemirror/legacy-modes/mode/xml').then((mode) => mode.xml),
  html: () => import('@codemirror/legacy-modes/mode/xml').then((mode) => mode.html),
  markdown: () => Promise.resolve(null),
  dockerfile: () => import('@codemirror/legacy-modes/mode/dockerfile').then((mode) => mode.dockerFile),
  sql: () => import('@codemirror/legacy-modes/mode/sql').then((mode) => mode.standardSQL),
  javascript: () => import('@codemirror/legacy-modes/mode/javascript').then((mode) => mode.javascript),
  typescript: () => import('@codemirror/legacy-modes/mode/javascript').then((mode) => mode.typescript),
  go: () => import('@codemirror/legacy-modes/mode/go').then((mode) => mode.go),
  cpp: () => import('@codemirror/legacy-modes/mode/clike').then((mode) => mode.cpp),
  rust: () => import('@codemirror/legacy-modes/mode/rust').then((mode) => mode.rust),

  php: () =>
    import('@codemirror/legacy-modes/mode/clike').then((mode) => mode.clike({ name: 'php', keywords: PHP_KEYWORDS })),
  ruby: () => import('@codemirror/legacy-modes/mode/ruby').then((mode) => mode.ruby),
  perl: () => import('@codemirror/legacy-modes/mode/perl').then((mode) => mode.perl),
}

function readonlyExtensions(readonly: boolean): Extension[] {
  return [EditorView.editable.of(!readonly), EditorState.readOnly.of(readonly)]
}

async function languageExtensions(language: EditorLanguage): Promise<Extension[]> {
  if (language === 'plaintext') return []
  try {
    const parser = await PARSER_LOADERS[language]()
    return parser ? [StreamLanguage.define(parser)] : []
  } catch (error) {
    if (import.meta.env.DEV) console.warn('[qterm] codemirror language failed to load', language, error)
    return []
  }
}

function createExtensions(): Extension[] {
  return [
    history(),
    drawSelection(),
    highlightActiveLine(),
    highlightSpecialChars(),
    bracketMatching(),
    indentOnInput(),
    keymap.of([...defaultKeymap, ...historyKeymap]),
    syntaxHighlighting(highlightStyle),
    EditorView.lineWrapping,
    readonlyCompartment.of(readonlyExtensions(props.readonly)),

    EditorView.contentAttributes.of({
      role: 'textbox',
      'aria-multiline': 'true',
      spellcheck: 'false',
    }),
    EditorView.updateListener.of((update) => {
      if (!update.docChanged) return
      if (update.transactions.some((transaction) => transaction.annotation(externalChange))) return
      emit('update:modelValue', update.state.doc.toString())
      emit('dirtyChange', update.state.doc.toString() !== baseline)
    }),
  ]
}

function disposeEditor(): void {
  if (disposed) return
  disposed = true

  const active = document.activeElement
  if (active instanceof HTMLElement && host.value?.contains(active)) active.blur()
  view?.destroy()
  view = null
}

onMounted(async () => {
  if (!host.value) return
  baseline = props.modelValue
  const language = normalizeEditorLanguage(props.language)

  const extensionsForLanguage = await languageExtensions(language)
  if (disposed || !host.value) return
  view = new EditorView({
    state: EditorState.create({
      doc: props.modelValue,
      extensions: [...createExtensions(), languageCompartment.of(extensionsForLanguage)],
    }),
    parent: host.value,
  })
  emit('ready')
})

watch(
  () => props.modelValue,
  (value) => {
    if (!view || value === view.state.doc.toString()) return
    view.dispatch({
      changes: { from: 0, to: view.state.doc.length, insert: value },
      annotations: externalChange.of(true),
    })

    baseline = value
    emit('dirtyChange', false)
  },
)

watch(
  () => props.readonly,
  (value) => {
    view?.dispatch({ effects: readonlyCompartment.reconfigure(readonlyExtensions(value)) })
  },
)

watch(
  () => props.language,
  async (value) => {
    const language = normalizeEditorLanguage(value)
    if (!view) return
    const extensions = await languageExtensions(language)
    if (!view) return
    view.dispatch({ effects: languageCompartment.reconfigure(extensions) })
  },
)

onBeforeUnmount(disposeEditor)

defineExpose({
  focus() {
    view?.focus()
  },
  dispose() {
    disposeEditor()
  },
})
</script>

<template>
  <div ref="host" class="remote-text-editor-codemirror" data-shortcut-passthrough="true" />
</template>

<style scoped>
.remote-text-editor-codemirror {
  width: 100%;
  height: 100%;
  min-height: 0;
  overflow: auto;
  color: var(--qt-text);
  background: var(--qt-app);
  font-family: 'SF Mono', SFMono-Regular, Menlo, Consolas, 'Liberation Mono', monospace;
  font-size: 12px;
}

.remote-text-editor-codemirror :deep(.cm-editor) {
  height: 100%;
  outline: none;
  background: transparent;
}

.remote-text-editor-codemirror :deep(.cm-editor.cm-focused) {
  outline: none;
}

.remote-text-editor-codemirror :deep(.cm-scroller) {
  font-family: inherit;
  line-height: 1.5;
}

.remote-text-editor-codemirror :deep(.cm-activeLine) {
  background: var(--qt-hover);
}

.remote-text-editor-codemirror :deep(.cm-selectionBackground),
.remote-text-editor-codemirror :deep(.cm-editor ::selection) {
  background: var(--qt-active) !important;
}
</style>
