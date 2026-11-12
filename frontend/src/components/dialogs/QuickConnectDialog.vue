<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { PlugZap } from '@lucide/vue'
import { QtermButton, QtermFormMessage, QtermInput, SegmentedTabs } from '../../components/common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { useI18n } from '../../locales'
import { openAdHocSession, type AdHocOpenResult, type AdHocProtocol } from '../../services/adhocSession'
import { BULK_CONNECT_LIMIT, flattenMultilinePaste, parseAdHocTargets } from '../../features/hosts/bulkConnect'

const props = defineProps<{ open: boolean }>()

interface AdHocOpenedEntry {
  hostId: string
  displayName: string
  protocol: AdHocProtocol
  address: string
  result: AdHocOpenResult
}

const emit = defineEmits<{
  close: []

  opened: [payload: { entries: AdHocOpenedEntry[]; failed: number }]
}>()

const { t } = useI18n()

const protocol = ref<AdHocProtocol>('ssh')
const target = ref('')
const busy = ref(false)
const error = ref('')

const tabs = computed(() => [
  { value: 'ssh', label: t('shell.adhoc.tabSsh') },
  { value: 'telnet', label: t('shell.adhoc.tabTelnet') },
])

const placeholder = computed(() =>
  protocol.value === 'telnet' ? t('shell.adhoc.placeholderTelnet') : t('shell.adhoc.placeholderSsh'),
)

const parsedTargets = computed(() => parseAdHocTargets(protocol.value, target.value))
const targetCount = computed(() => parsedTargets.value.targets.length)

const canSubmit = computed(() => !busy.value && target.value.trim().length > 0)

function reset(): void {
  target.value = ''
  error.value = ''
  busy.value = false
}

watch(
  () => props.open,
  (open) => {
    if (open) reset()
  },
)

function close(): void {
  if (busy.value) return
  emit('close')
}

function onPaste(event: ClipboardEvent): void {
  const text = event.clipboardData?.getData('text') ?? ''
  if (!/[\r\n]/.test(text)) return
  event.preventDefault()
  const flattened = flattenMultilinePaste(text)
  const element = event.target as HTMLInputElement | null
  const start = element?.selectionStart ?? target.value.length
  const end = element?.selectionEnd ?? start
  target.value = `${target.value.slice(0, start)}${flattened}${target.value.slice(end)}`
  error.value = ''
}

async function submit(): Promise<void> {
  if (busy.value) return

  const selectedProtocol = protocol.value
  const { targets, errors } = parseAdHocTargets(selectedProtocol, target.value)

  if (errors.length) {
    error.value = t('shell.adhoc.errorTargets', { list: errors.map((item) => item.value).join(', ') })
    return
  }
  if (targets.length === 0) {
    error.value = t('shell.adhoc.error.empty')
    return
  }
  if (targets.length > BULK_CONNECT_LIMIT) {
    error.value = t('shell.adhoc.errorLimit', { limit: BULK_CONNECT_LIMIT })
    return
  }

  if (selectedProtocol === 'ssh' && targets.some((item) => !item.input.username)) {
    error.value = t('shell.adhoc.error.username')
    return
  }

  busy.value = true
  error.value = ''
  const entries: AdHocOpenedEntry[] = []
  let failed = 0
  try {
    for (const item of targets) {
      try {
        const result = await openAdHocSession({ ...item.input, password: '' })
        entries.push({
          hostId: result.hostId,
          displayName: item.displayName,
          protocol: selectedProtocol,
          address: item.input.address,
          result,
        })
      } catch {
        failed += 1
      }
    }
  } finally {
    busy.value = false
  }

  if (entries.length === 0) {
    error.value = t('shell.adhoc.openFailed')
    return
  }
  emit('opened', { entries, failed })
  emit('close')
}
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => !open && close()">
    <QtermDialogContent v-if="props.open" data-adhoc-dialog>
      <QtermDialogHeader
        :title="t('shell.adhoc.title')"
        :description="t('shell.adhoc.description')"
        :show-close="false"
      />
      <form class="dialog-body flex flex-col gap-3" @submit.prevent="submit">
        <SegmentedTabs v-model="protocol" :options="tabs" :label="t('shell.adhoc.protocol')" data-adhoc-protocol />

        <QtermInput
          v-model="target"
          :placeholder="placeholder"
          :aria-label="t('shell.adhoc.target')"
          autocomplete="off"
          autocapitalize="off"
          autocorrect="off"
          spellcheck="false"
          autofocus
          data-adhoc-target
          @paste="onPaste"
        />

        <p v-if="!error" class="text-xs text-qt-muted" data-adhoc-hint>{{ t('shell.adhoc.multiHint') }}</p>
        <QtermFormMessage v-if="error" :message="error" tone="error" />
      </form>
      <QtermDialogFooter>
        <div></div>
        <div class="dialog-footer-actions flex items-center gap-[8px]">
          <QtermButton variant="outline" :disabled="busy" @click="close">{{ t('shell.adhoc.cancel') }}</QtermButton>
          <QtermButton data-adhoc-submit :disabled="!canSubmit" @click="submit">
            <PlugZap :size="14" />{{
              targetCount > 1 ? t('shell.adhoc.connectCount', { count: targetCount }) : t('shell.adhoc.connect')
            }}
          </QtermButton>
        </div>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>
