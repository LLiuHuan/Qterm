<script setup lang="ts">
import { useId } from 'vue'
import html from '@comark/vue/plugins/html'
import { markdownMath } from './markdownMath'
import AiMarkdownMath from './AiMarkdownMath.vue'
import { markdownFootnotes } from './markdownFootnotes'
import { Markdown } from '@comark/vue'
import security from '@comark/vue/plugins/security'
import taskList from '@comark/vue/plugins/task-list'
import AiMarkdownCodeBlock from './AiMarkdownCodeBlock.vue'
import AiMarkdownLink from './AiMarkdownLink.vue'
import AiMarkdownImage from './AiMarkdownImage.vue'
import { createMarkdownPolicy } from './markdownPolicy'

const props = withDefaults(
  defineProps<{
    text: string
    streaming?: boolean
  }>(),
  {
    streaming: false,
  },
)

const messageId = `ai-markdown-${useId().replace(/[^a-zA-Z0-9_-]/g, '')}`
const markdownPlugins = [
  html(),
  markdownMath,
  markdownFootnotes,
  taskList(),
  createMarkdownPolicy(messageId),
  security({
    allowedProtocols: ['https', 'http'],
    allowDataImages: false,
  }),
]
</script>

<template>
  <div class="typeset typeset-chat min-w-0 select-text break-words text-sm" :data-ai-markdown-message="messageId">
    <Suspense>
      <Markdown
        :value="props.text"
        :options="{ registerDefaultPlugins: false }"
        :components="{ pre: AiMarkdownCodeBlock, a: AiMarkdownLink, img: AiMarkdownImage, math: AiMarkdownMath }"
        :plugins="markdownPlugins"
        :streaming="props.streaming"
      />
      <template #fallback>
        <p class="m-0 whitespace-pre-wrap break-words text-sm leading-6">{{ props.text }}</p>
      </template>
    </Suspense>
  </div>
</template>

<style scoped>
.typeset {
  --typeset-flow: 1em;
  --typeset-leading: 1.6;
  line-height: var(--typeset-leading);

  font-synthesis: weight style;
}

.typeset :deep(.not-typeset) {
  font-synthesis: none;
}

.typeset :deep(> :first-child) {
  margin-block-start: 0;
}

.typeset :deep(> :last-child) {
  margin-block-end: 0;
}

.typeset :deep(p),
.typeset :deep(blockquote),
.typeset :deep(ul),
.typeset :deep(ol),
.typeset :deep(pre:not(:where(.not-typeset *))),
.typeset :deep(table),
.typeset :deep(hr) {
  margin-block: var(--typeset-flow);
}

.typeset :deep(:where(h1, h2, h3, h4, h5, h6):not(:where(.not-typeset *))) {
  margin-block: var(--typeset-flow) 0;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.typeset :deep(h1) {
  font-size: 1.75em;
  line-height: 1.3;
}

.typeset :deep(h2) {
  margin-block-start: calc(var(--typeset-flow) * 1.4);
  font-size: 1.25em;
  line-height: 1.4;
}

.typeset :deep(h3) {
  font-size: 1.125em;
  line-height: 1.45;
}

.typeset :deep(h4) {
  font-size: 1em;
  line-height: 1.5;
}

.typeset :deep(h5) {
  margin-block-start: calc(var(--typeset-flow) / 0.875);
  color: var(--qt-muted);
  font-size: 0.875em;
  font-weight: 500;
  line-height: 1.5;
}

.typeset :deep(h6) {
  margin-block-start: calc(var(--typeset-flow) / 0.8125);
  color: var(--qt-muted);
  font-size: 0.8125em;
  font-weight: 500;
  line-height: 1.5;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.typeset :deep(:where(strong, b):not(:where(.not-typeset *))) {
  font-weight: 600;
}

.typeset :deep(:where(em, i):not(:where(.not-typeset *))) {
  font-style: italic;
}

.typeset :deep(:where(del, s):not(:where(.not-typeset *))) {
  color: var(--qt-muted);
  text-decoration-line: line-through;
}

.typeset :deep(ul),
.typeset :deep(ol) {
  padding-inline-start: 1.5em;
}

.typeset :deep(ul) {
  list-style-type: disc;
}

.typeset :deep(ol) {
  list-style-type: decimal;
}

.typeset :deep(ul ul) {
  list-style-type: circle;
}

.typeset :deep(ul ul ul) {
  list-style-type: square;
}

.typeset :deep(li > :where(p, ul, ol)) {
  margin-block: 0.5em 0;
}

.typeset :deep(.task-list-item) {
  list-style: none;
}

.typeset :deep(input[type='checkbox']) {
  margin-inline-end: 0.5rem;
}

.typeset :deep(li + li) {
  margin-block-start: 0.25rem;
}

.typeset :deep(blockquote) {
  border-inline-start: 2px solid var(--qt-border);
  color: var(--qt-muted);
  padding-inline-start: 1em;
}

.typeset :deep(a) {
  color: var(--qt-accent);
  text-decoration: underline;
  text-underline-offset: 0.18em;
}

.typeset :deep(hr) {
  border: 0;
  border-block-start: 1px solid var(--qt-border);
}

.typeset :deep(:not(pre) > code:not(:where(.not-typeset *))) {
  border-radius: min(calc(var(--radius, 0.5em) * 0.6), 0.35em);
  background: color-mix(in srgb, var(--qt-text) 8%, var(--qt-panel));
  color: var(--qt-text);
  box-decoration-break: clone;
  -webkit-box-decoration-break: clone;
  padding: 0.125em 0.3em;
  font-family: var(--qt-font-mono);
  font-size: 0.85em;
  line-height: 1.4;
}

.typeset :deep(:where(h1, h2, h3, h4) code) {
  font-size: 0.9em;
}

.typeset :deep(kbd) {
  border: 1px solid var(--qt-border);
  border-bottom-width: 2px;
  border-radius: 0.3em;
  background: var(--qt-subtle);
  padding: 0.1em 0.35em;
  font-family: var(--qt-font-mono);
  font-size: 0.85em;
}

.typeset :deep(.footnotes-list) {
  border-top: 1px solid var(--qt-border);
  padding-block-start: 1em;
  color: var(--qt-muted);
  font-size: 0.875em;
}

.typeset :deep(.footnote-ref) {
  vertical-align: super;
  font-size: 0.75em;
  line-height: 0;
}

.typeset :deep(.footnote-backref) {
  margin-inline-start: 0.4em;
}

.typeset :deep(table) {
  display: block;
  max-width: 100%;
  overflow-x: auto;
  border-collapse: collapse;
  white-space: nowrap;
}

.typeset :deep(th),
.typeset :deep(td) {
  border: 1px solid var(--qt-border);
  padding: 0.4rem 0.65rem;
  text-align: start;
}

.typeset :deep(th) {
  background: color-mix(in srgb, var(--qt-subtle) 75%, transparent);
  font-weight: 600;
}

.typeset :deep([align='center']) {
  text-align: center;
}

.typeset :deep([align='right']) {
  text-align: right;
}
</style>
