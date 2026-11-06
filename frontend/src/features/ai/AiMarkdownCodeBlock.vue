<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { computed, ref, watch } from 'vue'
import { useTimeoutFn } from '@vueuse/core'
import { Check, Copy } from '@lucide/vue'
import { IconButton } from '../../components/common'
import { writeClipboardText } from '../../services/clipboard'
import { useI18n } from '../../locales'
import { toast } from 'vue-sonner'
import { textContent } from '@comark/vue/utils'
import {
  CodeBlock,
  CodeBlockActions,
  CodeBlockContent,
  CodeBlockHeader,
  CodeBlockLine,
  CodeBlockTitle,
  CodeBlockToken,
  useHighlightedCode,
} from '../../components/common/ai'

defineOptions({ inheritAttrs: false })

const props = defineProps<{
  // eslint-disable-next-line vue/prop-name-casing
  __node?: Parameters<typeof textContent>[0]
  class?: HTMLAttributes['class']
  style?: HTMLAttributes['style']
  language?: string
  filename?: string
}>()

const code = computed(() => {
  const value = props.__node ? textContent(props.__node).replace(/\n$/, '') : ''
  return value
})

const language = computed(() => {
  const value = props.language?.trim().replace(/^language-/i, '')
  return value || 'text'
})

const title = computed(() => props.filename?.trim() || (language.value === 'text' ? '' : language.value))
const highlightedLines = useHighlightedCode(code, language)
const { t } = useI18n()
const copied = ref(false)
const resetCopied = useTimeoutFn(
  () => {
    copied.value = false
  },
  2000,
  { immediate: false },
)
watch(code, () => {
  copied.value = false
})

async function copyCode(): Promise<void> {
  try {
    await writeClipboardText(code.value)
    copied.value = true
    resetCopied.start()
  } catch {
    toast.error(t('shell.aiChat.copyFailed'))
  }
}
</script>

<template>
  <CodeBlock class="not-typeset my-4 min-w-0 max-w-full" data-ai-markdown-code>
    <CodeBlockHeader class="px-3 py-1.5">
      <CodeBlockTitle class="font-mono text-xs font-medium uppercase tracking-[0.08em]">
        {{ title || 'text' }}
      </CodeBlockTitle>
      <CodeBlockActions>
        <IconButton
          :label="t(copied ? 'shell.aiChat.copied' : 'shell.aiChat.copyCommand')"
          size="icon-sm"
          class="size-7! min-h-7!"
          @click="copyCode"
          ><Check v-if="copied" class="size-3.5" aria-hidden="true" /><Copy v-else class="size-3.5" aria-hidden="true"
        /></IconButton>
      </CodeBlockActions>
    </CodeBlockHeader>
    <CodeBlockContent
      line-numbers
      :class="['max-h-[30rem] overflow-auto text-xs leading-5', props.class]"
      :style="props.style"
    >
      <CodeBlockLine v-for="(line, lineIndex) in highlightedLines" :key="lineIndex">
        <CodeBlockToken v-for="(token, tokenIndex) in line" :key="tokenIndex" :token="token" />
      </CodeBlockLine>
    </CodeBlockContent>
  </CodeBlock>
</template>
