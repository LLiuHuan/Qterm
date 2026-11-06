<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands'
import {
  bracketMatching,
  HighlightStyle,
  indentOnInput,
  StreamLanguage,
  syntaxHighlighting,
} from '@codemirror/language'
import { shell } from '@codemirror/legacy-modes/mode/shell'
import { tags } from '@lezer/highlight'
import { Annotation, EditorState, type Extension } from '@codemirror/state'
import {
  drawSelection,
  EditorView,
  highlightActiveLine,
  highlightSpecialChars,
  keymap,
  placeholder as cmPlaceholder,
} from '@codemirror/view'

defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    modelValue?: string
    id?: string
    placeholder?: string
    ariaLabel?: string
    class?: HTMLAttributes['class']
  }>(),
  {
    modelValue: '',
    id: undefined,
    placeholder: undefined,
    ariaLabel: undefined,
    class: undefined,
  },
)

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const editorHost = ref<HTMLElement | null>(null)
const externalChange = Annotation.define<boolean>()
let editorView: EditorView | null = null

const shellHighlightStyle = HighlightStyle.define([
  { tag: tags.comment, color: 'var(--qt-muted)', fontStyle: 'italic' },
  {
    tag: [tags.keyword, tags.controlKeyword],
    color: 'var(--qt-accent)',
    fontWeight: '600',
  },
  { tag: tags.string, color: 'var(--qt-success)' },
  { tag: [tags.number, tags.bool], color: 'var(--qt-info)' },
  { tag: tags.variableName, color: 'var(--qt-text)' },
  { tag: [tags.operator, tags.punctuation], color: 'var(--qt-muted)' },
])

function createEditorExtensions(): Extension[] {
  const contentAttributes: Record<string, string> = {
    role: 'textbox',
    'aria-multiline': 'true',
    spellcheck: 'false',
  }
  if (props.id) contentAttributes.id = props.id
  if (props.ariaLabel) contentAttributes['aria-label'] = props.ariaLabel

  return [
    history(),
    drawSelection(),
    highlightActiveLine(),
    highlightSpecialChars(),
    bracketMatching(),
    indentOnInput(),
    keymap.of([...defaultKeymap, ...historyKeymap]),
    StreamLanguage.define(shell),
    syntaxHighlighting(shellHighlightStyle, { fallback: true }),
    EditorView.lineWrapping,
    EditorView.contentAttributes.of(contentAttributes),
    ...(props.placeholder ? [cmPlaceholder(props.placeholder)] : []),
    EditorView.updateListener.of((update) => {
      if (!update.docChanged || update.transactions.some((transaction) => transaction.annotation(externalChange)))
        return
      emit('update:modelValue', update.state.doc.toString())
    }),
  ]
}

onMounted(() => {
  if (!editorHost.value) return
  editorView = new EditorView({
    state: EditorState.create({
      doc: props.modelValue,
      extensions: createEditorExtensions(),
    }),
    parent: editorHost.value,
  })
})

watch(
  () => props.modelValue,
  (value) => {
    if (!editorView || value === editorView.state.doc.toString()) return
    editorView.dispatch({
      changes: { from: 0, to: editorView.state.doc.length, insert: value },
      annotations: externalChange.of(true),
    })
  },
)

onBeforeUnmount(() => {
  editorView?.destroy()
  editorView = null
})
</script>

<template>
  <div ref="editorHost" class="snippet-content-editor" :class="props.class" v-bind="$attrs" />
</template>
