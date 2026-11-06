<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Check, Copy, ShieldAlert, ShieldQuestion } from 'lucide'
import { QtermButton } from '../../components/common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { useI18n } from '@/locales'

const props = defineProps<{
  open: boolean

  state: string
  hostTitle: string
  fingerprint: string
  subject: string
  issuer: string
  busy?: boolean
}>()

const emit = defineEmits<{ (event: 'confirm'): void; (event: 'cancel'): void }>()

const { t } = useI18n()
const showDetails = ref(false)
const copied = ref(false)

const changed = computed(() => props.state === 'mismatch')
const title = computed(() => (changed.value ? t('shell.rdp.certChangedTitle') : t('shell.rdp.certUnknownTitle')))
const description = computed(() =>
  changed.value
    ? t('shell.rdp.certChangedDescription', { host: props.hostTitle })
    : t('shell.rdp.certUnknownDescription', { host: props.hostTitle }),
)
const confirmLabel = computed(() => (changed.value ? t('shell.rdp.certContinueAnyway') : t('shell.rdp.certContinue')))

const fingerprintLines = computed(() => {
  const value = props.fingerprint.replace(/[^0-9a-fA-F]/g, '').toUpperCase()
  return value ? (value.match(/.{1,16}/g) ?? []) : []
})

async function copyFingerprint(): Promise<void> {
  try {
    await navigator.clipboard.writeText(props.fingerprint)
    copied.value = true

    window.setTimeout(() => {
      copied.value = false
    }, 1600)
  } catch {
    copied.value = false
  }
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      showDetails.value = false
      copied.value = false
    }
  },
)
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(value) => !value && emit('cancel')">
    <QtermDialogContent size="md" :close-on-escape="true">
      <QtermDialogHeader :title="title" :description="description" @close="emit('cancel')">
        <template #icon>
          <component :is="changed ? ShieldAlert : ShieldQuestion" :size="18" aria-hidden="true" />
        </template>
      </QtermDialogHeader>

      <div class="flex flex-col gap-3 px-5 py-4 text-sm text-qt-text">
        <p
          class="rounded-md border px-3 py-2 leading-5"
          :class="
            changed
              ? 'border-red-500/50 bg-red-500/10 text-red-600 dark:text-red-300'
              : 'border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-200'
          "
        >
          {{ changed ? t('shell.rdp.certChangedHint') : t('shell.rdp.certUnknownHint') }}
        </p>

        <div class="rounded-md border-d bg-qt-panel">
          <div class="flex-cb gap-2 border-b border-qt-border px-3 py-1.5">
            <span class="text-xs text-qt-text-muted">{{ t('shell.rdp.certFingerprint') }}</span>
            <QtermButton variant="ghost" size="xs" type="button" @click="copyFingerprint">
              <component :is="copied ? Check : Copy" :size="13" aria-hidden="true" />
              <span class="ml-1 text-xs">{{ copied ? t('shell.rdp.certCopied') : t('shell.rdp.certCopy') }}</span>
            </QtermButton>
          </div>
          <div class="px-3 py-2 font-mono text-xs leading-5 break-all">
            <div v-for="line in fingerprintLines" :key="line">{{ line }}</div>
            <div v-if="fingerprintLines.length === 0" class="text-qt-text-muted">
              {{ t('shell.rdp.certUnknownValue') }}
            </div>
          </div>
        </div>

        <button
          class="self-start rounded px-1 py-0.5 text-xs text-qt-accent"
          type="button"
          @click="showDetails = !showDetails"
        >
          {{ showDetails ? t('shell.rdp.certHideDetails') : t('shell.rdp.certShowDetails') }}
        </button>

        <dl v-if="showDetails" class="grid grid-cols-[5.5rem_1fr] gap-x-3 gap-y-1.5 text-xs">
          <dt class="text-qt-text-muted">{{ t('shell.rdp.certSubject') }}</dt>
          <dd class="break-all">{{ props.subject || t('shell.rdp.certUnknownValue') }}</dd>
          <dt class="text-qt-text-muted">{{ t('shell.rdp.certIssuer') }}</dt>
          <dd class="break-all">{{ props.issuer || t('shell.rdp.certUnknownValue') }}</dd>
        </dl>
      </div>

      <QtermDialogFooter>
        <div></div>
        <div class="flex items-center gap-2">
          <QtermButton variant="outline" type="button" :disabled="props.busy" @click="emit('cancel')">
            {{ t('shell.rdp.certCancel') }}
          </QtermButton>
          <QtermButton type="button" :disabled="props.busy" @click="emit('confirm')">
            {{ confirmLabel }}
          </QtermButton>
        </div>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>
