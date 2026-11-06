<script setup lang="ts">
import { ref } from 'vue'
import { Check, ChevronDown, ChevronUp, Download, Pause, Play, RotateCcw, Upload, X } from '@lucide/vue'
import IconButton from '../button/IconButton.vue'
import QtermButton from '../button/QtermButton.vue'
import { useI18n } from '../../../locales'

export interface TransferProgressItem {
  id: string
  name: string
  direction: 'upload' | 'download'
  progress: number
  label: string

  stage?: string
  completed?: boolean
  action?: 'cancel' | 'retry' | 'dismiss' | 'pause' | 'resume'

  secondaryAction?: 'cancel' | 'retry' | 'dismiss' | 'pause' | 'resume'
  speed?: string
  eta?: string
}

defineProps<{ items: TransferProgressItem[] }>()
const emit = defineEmits<{
  action: [id: string, action: NonNullable<TransferProgressItem['action']>]
  dismiss: [id: string]
}>()
const { t } = useI18n()
const collapsed = ref(false)
function actionLabel(action: NonNullable<TransferProgressItem['action']>): string {
  if (action === 'cancel') return t('shell.transferQueue.cancel')
  if (action === 'retry') return t('shell.transferQueue.retry')
  if (action === 'pause') return t('shell.transferQueue.pause')
  if (action === 'resume') return t('shell.transferQueue.resume')
  return t('shell.transferQueue.dismiss')
}

function itemLabel(item: TransferProgressItem): string {
  return item.stage === 'extracting' ? t('sftp.ui.transfer.stage.extracting') : item.label
}
</script>

<template>
  <section
    v-if="items.length"
    class="transfer-queue text-xs"
    :class="{ 'is-collapsed': collapsed }"
    :aria-label="t('shell.transferQueue.aria')"
  >
    <div class="transfer-header">
      <strong class="text-sm">{{ t('shell.transferQueue.title') }}</strong>
      <span class="transfer-count">{{ items.length }}</span>
      <IconButton
        :label="collapsed ? t('shell.transferQueue.expand') : t('shell.transferQueue.collapseShort')"
        size="icon-sm"
        @click="collapsed = !collapsed"
      >
        <ChevronDown v-if="collapsed" :size="14" aria-hidden="true" />
        <ChevronUp v-else :size="14" aria-hidden="true" />
      </IconButton>
    </div>
    <div v-if="!collapsed" class="transfer-list">
      <div v-for="item in items" :key="item.id" class="transfer-item">
        <Upload v-if="item.direction === 'upload'" :size="14" aria-hidden="true" />
        <Download v-else :size="14" aria-hidden="true" />
        <span class="transfer-main text-sm">
          <strong>{{ item.name }}</strong>
          <small class="text-xs">{{ itemLabel(item) }} · {{ item.progress }}%</small>
          <span
            class="transfer-track"
            role="progressbar"
            :aria-label="item.name"
            :aria-valuenow="item.progress"
            :aria-valuemin="0"
            :aria-valuemax="100"
          >
            <i :style="{ width: `${item.progress}%` }" />
          </span>
          <span v-if="item.speed || item.eta" class="transfer-meta text-xs">
            <span v-if="item.speed">{{ item.speed }}</span>
            <span v-if="item.speed && item.eta"> · </span>
            <span v-if="item.eta">{{ item.eta }}</span>
          </span>
        </span>
        <Check
          v-if="item.completed"
          class="transfer-success"
          :size="14"
          :aria-label="t('shell.transferQueue.completed')"
        />
        <IconButton
          v-if="item.action"
          :label="actionLabel(item.action)"
          size="icon-sm"
          @click="emit('action', item.id, item.action)"
        >
          <RotateCcw v-if="item.action === 'retry'" :size="13" aria-hidden="true" />
          <Pause v-else-if="item.action === 'pause'" :size="13" aria-hidden="true" />
          <Play v-else-if="item.action === 'resume'" :size="13" aria-hidden="true" />
          <X v-else :size="13" aria-hidden="true" />
        </IconButton>
        <IconButton
          v-if="item.secondaryAction"
          :label="actionLabel(item.secondaryAction)"
          size="icon-sm"
          @click="emit('action', item.id, item.secondaryAction)"
        >
          <X :size="13" aria-hidden="true" />
        </IconButton>
        <QtermButton
          v-if="item.action === 'retry'"
          variant="ghost"
          size="icon-sm"
          type="button"
          :aria-label="t('shell.transferQueue.dismiss')"
          :title="t('shell.transferQueue.dismiss')"
          @click="emit('dismiss', item.id)"
        >
          <X :size="13" aria-hidden="true" />
        </QtermButton>
      </div>
    </div>
  </section>
</template>

<style scoped>
.transfer-queue {
  position: fixed;
  right: 22px;
  bottom: 18px;
  z-index: 20;
  width: min(360px, calc(100vw - 44px));
  max-height: var(--qt-transfer-queue-height-even, 420px);
  overflow: auto;
  padding: 10px;
  border: 1px solid var(--qt-border);
  border-radius: 10px;
  background: color-mix(in srgb, var(--qt-panel) 85%, transparent);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  box-shadow: var(--qt-shadow);
}

.transfer-queue.is-collapsed {
  overflow: hidden;
}

.transfer-header {
  min-height: 26px;
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--qt-muted);
}

.transfer-header strong {
  margin-right: auto;
  color: var(--qt-text);
  font-weight: 600;
}

.transfer-count {
  min-width: 18px;
  padding: 1px 5px;
  border-radius: 999px;
  color: var(--qt-text);
  background: var(--qt-hover);
  text-align: center;
}

.transfer-list {
  display: grid;
  gap: 2px;
}

.transfer-item {
  min-height: 44px;
  padding: 6px 10px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  gap: 7px;
  color: var(--qt-muted);
}

.transfer-item:hover,
.transfer-item:focus-within {
  background: var(--qt-hover);
}

.transfer-main {
  min-width: 0;
  flex: 1;
  display: grid;
  gap: 2px;
}

.transfer-main strong {
  overflow: hidden;
  color: var(--qt-text);
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.transfer-main small {
  color: var(--qt-muted);
}

.transfer-track {
  height: 2px;
  border-radius: 999px;
  background: var(--qt-border);
  overflow: hidden;
}

.transfer-track i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--qt-accent);
  transition: width 160ms ease;
}

.transfer-success {
  color: var(--qt-success);
}

@media (prefers-reduced-motion: reduce) {
  .transfer-track i {
    transition: none;
  }
}

.transfer-meta {
  display: flex;
  gap: 2px;
  color: var(--qt-muted);
  line-height: 1;
}
</style>
