<script setup lang="ts">
import { computed } from 'vue'
import { Pause, Play, Users, X } from '@lucide/vue'
import { QtermButton } from '../../../components/common'
import { useI18n } from '../../../locales'
import type { SyncAction, SyncGroupView } from '../types'

const props = defineProps<{ group: SyncGroupView | null }>()
const emit = defineEmits<{ action: [action: SyncAction] }>()
const { t } = useI18n()

const isActive = computed(() => props.group?.status === 'active')
const count = computed(() => props.group?.sessionIds.length ?? 0)
</script>

<template>
  <div
    v-if="props.group"
    class="flex min-h-8 flex-wrap items-center gap-2 border-t border-qt-border px-3 py-1.5 text-xs text-qt-text"
    data-terminal-sync-status
  >
    <Users :size="14" class="shrink-0 text-qt-accent" aria-hidden="true" />
    <span class="font-medium">{{
      isActive ? t('terminalControl.sync.status.active') : t('terminalControl.sync.status.paused')
    }}</span>
    <span class="text-qt-muted">· {{ t('terminalControl.sync.status.count', { count }) }}</span>
    <span class="ml-auto flex items-center gap-1">
      <QtermButton size="xs" variant="ghost" @click="emit('action', isActive ? 'pause' : 'resume')">
        <Pause v-if="isActive" :size="13" aria-hidden="true" />
        <Play v-else :size="13" aria-hidden="true" />
        {{ isActive ? t('terminalControl.sync.pause') : t('terminalControl.sync.resume') }}
      </QtermButton>
      <QtermButton size="xs" variant="ghost" @click="emit('action', 'manage')">
        <Users :size="13" aria-hidden="true" />{{ t('terminalControl.sync.manage') }}
      </QtermButton>
      <QtermButton size="xs" variant="ghost" class="text-qt-danger" @click="emit('action', 'end')">
        <X :size="13" aria-hidden="true" />{{ t('terminalControl.sync.end') }}
      </QtermButton>
    </span>
  </div>
</template>
