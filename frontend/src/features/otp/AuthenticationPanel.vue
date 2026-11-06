<script setup lang="ts">
import { computed } from 'vue'
import { KeyRound } from '@lucide/vue'
import type { CredentialRecord } from '../../services/credentials'
import type { HostRecord } from '../../services/hosts'
import type { OtpEntry } from '../../services/otp'
import { useI18n } from '../../locales'
import OtpPanel from './OtpPanel.vue'

const props = defineProps<{
  entries: OtpEntry[]
  hosts: HostRecord[]
  activeHostId?: string
  host: HostRecord | null
  credentials: CredentialRecord[]
}>()
const emit = defineEmits<{ manage: []; 'update:entries': [entries: OtpEntry[]] }>()
const { t } = useI18n()
const credential = computed(() => props.credentials.find((item) => item.id === props.host?.credentialId))
</script>

<template>
  <section class="flex h-full min-h-0 flex-col gap-3 px-2 py-2">
    <div class="flex shrink-0 items-center gap-2">
      <span class="min-w-0 flex-1 text-xs text-qt-muted">{{ t('shell.otp.quickHint') }}</span>
    </div>
    <div v-if="credential" class="flex shrink-0 items-center gap-2 rounded-md border-d px-3 py-2">
      <KeyRound :size="16" class="shrink-0 text-qt-muted" />
      <div class="min-w-0 flex-1">
        <div class="text-xs text-qt-muted">{{ t('shell.otp.currentCredential') }}</div>
        <div class="truncate text-sm" :title="credential.name">{{ credential.name }}</div>
      </div>
    </div>
    <OtpPanel
      class="min-h-0 flex-1"
      mode="compact"
      :entries="props.entries"
      :hosts="props.hosts"
      @manage="emit('manage')"
      @update:entries="emit('update:entries', $event)"
    />
  </section>
</template>
