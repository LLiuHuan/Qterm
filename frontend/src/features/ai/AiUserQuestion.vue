<script setup lang="ts">
import { computed, ref } from 'vue'
import { HelpCircle, Send } from '@lucide/vue'
import { QtermButton } from '../../components/common'
import { useI18n } from '../../locales'
import type { AgentStep } from '../../services/aiAgent'

const props = defineProps<{
  step: AgentStep
  disabled?: boolean
}>()

const emit = defineEmits<{ select: [choice: string]; cancel: [] }>()
const { t } = useI18n()
const custom = ref('')

interface AskPreview {
  question?: string
  options?: string[]
}

const preview = computed<AskPreview | null>(() => {
  const data = props.step.preview?.data
  if (!data || typeof data !== 'object') return null
  return data as AskPreview
})

const question = computed(() => preview.value?.question ?? props.step.title ?? '')
const options = computed(() => (preview.value?.options ?? []).filter((option) => option.trim() !== ''))

function choose(option: string): void {
  if (props.disabled) return
  emit('select', option)
}

function submitCustom(): void {
  const value = custom.value.trim()
  if (!value || props.disabled) return
  emit('select', value)
}
</script>

<template>
  <div class="flex min-w-0 flex-col gap-2 rounded-lg border border-qt-info/40 bg-qt-info/5 p-3" data-ai-question>
    <p class="m-0 flex items-start gap-1.5 text-xs font-medium text-qt-info">
      <HelpCircle class="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
      <span class="min-w-0 whitespace-pre-wrap break-words">{{ question }}</span>
    </p>
    <div class="flex min-w-0 flex-wrap gap-1.5">
      <QtermButton
        v-for="option in options"
        :key="option"
        size="sm"
        variant="outline"
        class="h-7! min-h-7! max-w-full truncate text-xs"
        :disabled="props.disabled"
        :data-ai-question-option="option"
        @click="choose(option)"
      >
        {{ option }}
      </QtermButton>
    </div>
    <div class="flex min-w-0 items-center gap-2">
      <input
        v-model="custom"
        type="text"
        class="min-w-0 flex-1 rounded-md border border-solid border-qt-border bg-transparent px-2 py-1 text-xs text-qt-text"
        :placeholder="t('shell.aiChat.question.customPlaceholder')"
        :disabled="props.disabled"
        data-ai-question-custom
        @keydown.enter.prevent="submitCustom"
      />
      <QtermButton
        size="sm"
        class="h-7! min-h-7! text-xs"
        :disabled="props.disabled || !custom.trim()"
        data-ai-question-submit
        @click="submitCustom"
      >
        <Send class="size-3.5" aria-hidden="true" />
        {{ t('shell.aiChat.question.submit') }}
      </QtermButton>
      <QtermButton
        size="sm"
        variant="outline"
        class="h-7! min-h-7! text-xs"
        :disabled="props.disabled"
        data-ai-question-cancel
        @click="emit('cancel')"
      >
        {{ t('shell.aiChat.question.cancel') }}
      </QtermButton>
    </div>
  </div>
</template>
