<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '../../locales'
import { diffLines } from './textDiff'

const props = defineProps<{
  oldText: string
  newText: string
  created?: boolean
}>()

const { t } = useI18n()
const diff = computed(() => diffLines(props.oldText, props.newText))

function sign(kind: 'context' | 'add' | 'remove'): string {
  if (kind === 'add') return '+'
  if (kind === 'remove') return '-'
  return ' '
}
</script>

<template>
  <div class="flex min-w-0 flex-col gap-1.5" data-ai-diff>
    <div class="flex items-center gap-2 text-xs">
      <span v-if="props.created" class="rounded bg-qt-success/15 px-1.5 py-0.5 text-qt-success">
        {{ t('shell.aiChat.diff.created') }}
      </span>
      <span class="text-qt-success">+{{ diff.added }}</span>
      <span class="text-qt-danger">-{{ diff.removed }}</span>
      <span v-if="diff.truncated" class="text-qt-muted/70">{{ t('shell.aiChat.diff.truncated') }}</span>
    </div>
    <div
      v-if="diff.lines.length"
      class="max-h-64 overflow-auto rounded-md border border-qt-border/60 bg-qt-subtle/20 py-1 font-mono text-xs leading-5"
      data-ai-diff-body
    >
      <div
        v-for="(line, index) in diff.lines"
        :key="`${index}-${line.kind}`"
        class="flex min-w-0 gap-1 px-2 whitespace-pre-wrap break-words"
        :class="{
          'bg-qt-success/10 text-qt-success': line.kind === 'add',
          'bg-qt-danger/10 text-qt-danger': line.kind === 'remove',
        }"
        :data-ai-diff-line="line.kind"
      >
        <span class="w-3 shrink-0 select-none text-qt-muted/60">{{ sign(line.kind) }}</span>
        <span class="min-w-0">{{ line.text }}</span>
      </div>
    </div>
    <p v-else class="m-0 text-xs text-qt-muted/70">{{ t('shell.aiChat.diff.empty') }}</p>
  </div>
</template>
