<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { PlugZap, TriangleAlert } from '@lucide/vue'
import { QtermButton, QtermFormMessage } from '../../components/common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { useI18n } from '../../locales'
import {
  openAdHocSession,
  parseAdHocTarget,
  type AdHocOpenResult,
  type AdHocProtocol,
} from '../../services/adhocSession'
import type { ParsedLaunchTarget } from '../../services/launchIntent'

const props = defineProps<{
  open: boolean

  target: ParsedLaunchTarget | null

  source: string

  errorKey?: string
}>()

const emit = defineEmits<{
  close: []
  opened: [payload: { entries: AdHocOpenedEntry[]; failed: number }]
}>()

interface AdHocOpenedEntry {
  hostId: string
  displayName: string
  protocol: AdHocProtocol
  address: string
  result: AdHocOpenResult
}

const { t } = useI18n()

const busy = ref(false)
const error = ref('')

const parsed = computed(() => {
  if (!props.target) return null
  const result = parseAdHocTarget(props.target.protocol, props.target.target)
  return typeof result === 'string' ? null : result
})

const protocolLabel = computed(() =>
  props.target?.protocol === 'telnet' ? t('shell.adhoc.tabTelnet') : t('shell.adhoc.tabSsh'),
)

const displayAddress = computed(() => {
  const input = parsed.value?.input
  if (!input) return props.target?.target ?? ''
  return input.username ? `${input.username}@${input.address}` : input.address
})

const errorMessage = computed(() => {
  if (error.value) return error.value
  if (props.errorKey) return t(`shell.launchIntent.error.${props.errorKey}`)
  if (props.target && !parsed.value) return t('shell.launchIntent.error.invalid')
  return ''
})

const canConnect = computed(
  () => !busy.value && !!parsed.value && (props.target?.protocol !== 'ssh' || !!parsed.value.input.username),
)

watch(
  () => props.open,
  (open) => {
    if (open) {
      busy.value = false
      error.value = ''
    }
  },
)

function close(): void {
  if (busy.value) return
  emit('close')
}

async function connect(): Promise<void> {
  const item = parsed.value
  if (busy.value || !item || !props.target) return
  busy.value = true
  error.value = ''
  try {
    const result = await openAdHocSession({ ...item.input, password: '' })
    emit('opened', {
      entries: [
        {
          hostId: result.hostId,
          displayName: item.displayName,
          protocol: props.target.protocol,
          address: item.input.address,
          result,
        },
      ],
      failed: 0,
    })
    emit('close')
  } catch {
    error.value = t('shell.adhoc.openFailed')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => !open && close()">
    <QtermDialogContent v-if="props.open" data-launch-intent-dialog>
      <QtermDialogHeader
        :title="t('shell.launchIntent.title')"
        :description="t('shell.launchIntent.description')"
        :show-close="false"
      />
      <div class="dialog-body flex flex-col gap-3">
        <div
          v-if="props.target"
          class="flex flex-col gap-1 rounded-lg border border-qt-border/60 bg-qt-subtle/30 px-3 py-2"
          data-launch-intent-target
        >
          <span class="text-xs uppercase tracking-[0.08em] text-qt-muted">{{ protocolLabel }}</span>
          <span class="break-all font-mono text-sm text-qt-text">{{ displayAddress }}</span>
        </div>

        <p v-if="props.source && props.source !== displayAddress" class="break-all font-mono text-xs text-qt-muted">
          {{ props.source }}
        </p>

        <p class="flex items-start gap-2 text-xs text-qt-warning" data-launch-intent-warning>
          <TriangleAlert :size="14" class="mt-0.5 shrink-0" />
          <span>{{ t('shell.launchIntent.warning') }}</span>
        </p>
        <QtermFormMessage v-if="errorMessage" :message="errorMessage" tone="error" />
      </div>
      <QtermDialogFooter>
        <div></div>
        <div class="dialog-footer-actions flex items-center gap-[8px]">
          <QtermButton variant="outline" :disabled="busy" @click="close">{{ t('shell.adhoc.cancel') }}</QtermButton>
          <QtermButton data-launch-intent-connect :disabled="!canConnect" @click="connect">
            <PlugZap :size="14" />{{ t('shell.adhoc.connect') }}
          </QtermButton>
        </div>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>
