<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { PlugZap } from '@lucide/vue'
import { QtermButton, QtermFormMessage, QtermInput, SegmentedTabs } from '../../components/common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { useI18n } from '../../locales'
import {
  adHocSessionErrorMessage,
  openAdHocSession,
  parseAdHocTarget,
  type AdHocOpenResult,
  type AdHocProtocol,
} from '../../services/adhocSession'

const props = defineProps<{ open: boolean }>()

const emit = defineEmits<{
  close: []
  opened: [
    payload: { hostId: string; displayName: string; protocol: AdHocProtocol; address: string; result: AdHocOpenResult },
  ]
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

async function submit(): Promise<void> {
  if (busy.value) return

  const selectedProtocol = protocol.value
  const parsed = parseAdHocTarget(selectedProtocol, target.value)
  if (typeof parsed === 'string') {
    error.value = t(`shell.adhoc.error.${parsed}`)
    return
  }

  if (selectedProtocol === 'ssh' && !parsed.input.username) {
    error.value = t('shell.adhoc.error.username')
    return
  }
  busy.value = true
  error.value = ''
  try {
    const result = await openAdHocSession({ ...parsed.input, password: '' })
    emit('opened', {
      hostId: result.hostId,
      displayName: parsed.displayName,
      protocol: selectedProtocol,
      address: parsed.input.address,
      result,
    })
    emit('close')
  } catch (cause) {
    error.value = adHocSessionErrorMessage(cause, t('shell.adhoc.openFailed'))
  } finally {
    busy.value = false
  }
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
        />
        <QtermFormMessage v-if="error" :message="error" tone="error" />
      </form>
      <QtermDialogFooter>
        <div></div>
        <div class="dialog-footer-actions flex items-center gap-[8px]">
          <QtermButton variant="outline" :disabled="busy" @click="close">{{ t('shell.adhoc.cancel') }}</QtermButton>
          <QtermButton data-adhoc-submit :disabled="busy || !target.trim()" @click="submit">
            <PlugZap :size="14" />{{ t('shell.adhoc.connect') }}
          </QtermButton>
        </div>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>
