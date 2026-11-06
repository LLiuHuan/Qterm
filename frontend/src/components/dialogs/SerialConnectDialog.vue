<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { Cable } from '@lucide/vue'
import { QtermButton, QtermFormMessage, QtermInput, QtermNumberField, SegmentedTabs } from '../../components/common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { useI18n } from '../../locales'
import {
  openSerialSession,
  listSerialPorts,
  readLastSerialTarget,
  rememberSerialTarget,
  serialSessionErrorMessage,
  SERIAL_BAUD_OPTIONS,
  SERIAL_DEFAULT_BAUD,
  type SerialPortInfo,
} from '../../services/serialSession'
import type { AdHocOpenResult } from '../../services/adhocSession'

const props = defineProps<{ open: boolean }>()

const emit = defineEmits<{
  close: []
  opened: [payload: { hostId: string; displayName: string; result: AdHocOpenResult }]
}>()

const { t } = useI18n()

const device = ref('')
const baud = ref<number>(SERIAL_DEFAULT_BAUD)
const ports = ref<SerialPortInfo[]>([])
const busy = ref(false)
const error = ref('')

const baudOptions = computed(() => SERIAL_BAUD_OPTIONS.map((value) => ({ value: String(value), label: String(value) })))

watch(
  () => props.open,
  (open) => {
    if (!open) return

    const last = readLastSerialTarget()
    device.value = last?.device ?? ''
    baud.value = last?.baud ?? SERIAL_DEFAULT_BAUD
    error.value = ''
    busy.value = false
  },
)

onMounted(() => void refreshPorts())

async function refreshPorts(): Promise<void> {
  ports.value = await listSerialPorts()
}

function close(): void {
  if (busy.value) return
  emit('close')
}

async function submit(): Promise<void> {
  if (busy.value) return
  const target = device.value.trim()
  if (!target) {
    error.value = t('shell.serial.deviceRequired')
    return
  }
  busy.value = true
  error.value = ''
  try {
    const result = await openSerialSession(target, baud.value)
    rememberSerialTarget(target, baud.value)
    emit('opened', { hostId: result.hostId, displayName: target, result })
    emit('close')
  } catch (cause) {
    error.value = serialSessionErrorMessage(cause, t('shell.serial.openFailed'))
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => !open && close()">
    <QtermDialogContent v-if="props.open" data-serial-connect-dialog>
      <QtermDialogHeader
        :title="t('shell.serial.title')"
        :description="t('shell.serial.description')"
        :show-close="false"
      />
      <form class="dialog-body flex flex-col gap-3" @submit.prevent="submit">
        <label class="flex flex-col gap-1 text-xs text-qt-muted">
          {{ t('shell.serial.device') }}
          <QtermInput
            v-model="device"
            :placeholder="t('shell.serial.devicePlaceholder')"
            :aria-label="t('shell.serial.device')"
            autocomplete="off"
            spellcheck="false"
            autofocus
            data-serial-device
          />
        </label>

        <div v-if="ports.length" class="flex flex-wrap gap-1.5">
          <QtermButton
            v-for="port in ports"
            :key="port.name"
            variant="outline"
            size="sm"
            type="button"
            class="font-mono text-xs"
            @click="device = port.name"
          >
            {{ port.name }}
          </QtermButton>
        </div>
        <div class="flex flex-col gap-1 text-xs text-qt-muted">
          {{ t('shell.serial.baud') }}
          <div class="flex items-center gap-2">
            <SegmentedTabs
              :model-value="String(baud)"
              :options="baudOptions"
              :label="t('shell.serial.baud')"
              @update:model-value="baud = Number($event)"
            />
            <QtermNumberField
              v-model="baud"
              class="w-[120px]"
              :min="50"
              :max="4000000"
              :step="1"
              :step-snapping="false"
              :aria-label="t('shell.serial.baud')"
            />
          </div>
        </div>
        <QtermFormMessage v-if="error" :message="error" tone="error" />
      </form>
      <QtermDialogFooter>
        <div></div>
        <div class="dialog-footer-actions flex items-center gap-[8px]">
          <QtermButton variant="outline" :disabled="busy" @click="close">{{ t('shell.serial.cancel') }}</QtermButton>
          <QtermButton data-serial-connect-submit :disabled="busy || !device.trim()" @click="submit">
            <Cable :size="14" />{{ t('shell.serial.connect') }}
          </QtermButton>
        </div>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>
