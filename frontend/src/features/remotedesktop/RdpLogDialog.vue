<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Check, Copy } from 'lucide'
import { QtermButton } from '../../components/common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { useI18n } from '@/locales'

const props = defineProps<{
  open: boolean
  hostTitle: string

  stateText: string

  stderrText: string

  actionHint: string
}>()

const emit = defineEmits<{ (event: 'close'): void }>()

const { t } = useI18n()
const copied = ref(false)
const body = ref<HTMLElement | null>(null)

const content = computed(() => {
  const lines = [`# ${t('shell.rdp.logState')}: ${props.stateText || '-'}`]
  if (props.actionHint) {
    lines.push(`# ${t('shell.rdp.logAction')}: ${props.actionHint}`)
  }
  lines.push('', props.stderrText || t('shell.rdp.logEmpty'))
  return lines.join('\n')
})

async function copyAll(): Promise<void> {
  try {
    await navigator.clipboard.writeText(content.value)
    copied.value = true
    window.setTimeout(() => {
      copied.value = false
    }, 1600)
  } catch {
    copied.value = false
  }
}

watch(
  () => [props.open, props.stderrText] as const,
  () => {
    if (!props.open) return
    requestAnimationFrame(() => {
      const element = body.value
      if (element) element.scrollTop = element.scrollHeight
    })
  },
  { immediate: true },
)
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(value) => !value && emit('close')">
    <QtermDialogContent size="lg" :close-on-escape="true">
      <QtermDialogHeader
        :title="t('shell.rdp.logTitle', { host: props.hostTitle })"
        :description="t('shell.rdp.logDescription')"
        @close="emit('close')"
      />
      <div class="px-5 py-3">
        <pre
          ref="body"
          class="max-h-[60vh] overflow-auto rounded-md border-d bg-black/80 p-3 font-mono text-[11px] leading-4 whitespace-pre-wrap text-amber-100"
          >{{ content }}</pre>
      </div>
      <QtermDialogFooter>
        <div></div>
        <div class="flex items-center gap-2">
          <QtermButton variant="outline" type="button" @click="copyAll">
            <component :is="copied ? Check : Copy" :size="13" aria-hidden="true" />
            <span class="ml-1">{{ copied ? t('shell.rdp.certCopied') : t('shell.rdp.logCopy') }}</span>
          </QtermButton>
          <QtermButton type="button" @click="emit('close')">{{ t('shell.rdp.certCancel') }}</QtermButton>
        </div>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>
