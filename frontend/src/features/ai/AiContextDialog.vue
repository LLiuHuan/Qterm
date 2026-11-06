<script setup lang="ts">
import { computed } from 'vue'
import { Layers } from '@lucide/vue'
import { QtermButton, QtermEmptyState } from '../../components/common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { useI18n } from '../../locales'
import { formatByteSize } from '../../lib/format'
import type { ContextBreakdown } from '../../../bindings/changeme/internal/ai/models'

const props = defineProps<{
  open: boolean

  breakdown: ContextBreakdown | null

  mode: 'ask' | 'agent'
}>()

const emit = defineEmits<{ close: [] }>()

const { t } = useI18n()

const modeLabel = computed(() =>
  props.mode === 'agent' ? t('shell.aiChat.agent.modeAgent') : t('shell.aiChat.agent.modeAsk'),
)

const segments = computed(() => {
  const breakdown = props.breakdown
  return [
    {
      key: 'system',
      label: t('shell.aiChat.contextDetailSystem'),
      value: breakdown?.system ?? 0,
      swatch: 'bg-qt-accent/70',
    },
    {
      key: 'tools',
      label: t('shell.aiChat.contextDetailTools'),
      value: breakdown?.tools ?? 0,
      swatch: 'bg-qt-accent/40',
    },
    {
      key: 'messages',
      label: t('shell.aiChat.contextDetailMessages'),
      value: breakdown?.messages ?? 0,
      swatch: 'bg-qt-accent',
    },
    {
      key: 'context',
      label: t('shell.aiChat.contextDetailAttached'),
      value: breakdown?.context ?? 0,
      swatch: 'bg-qt-accent/20',
    },
  ]
})

const total = computed(() => segments.value.reduce((sum, segment) => sum + segment.value, 0))
const limit = computed(() => props.breakdown?.limit ?? 0)

const budgeted = computed(() =>
  props.breakdown ? props.breakdown.system + props.breakdown.messages + props.breakdown.context : 0,
)
const usedPercent = computed(() =>
  limit.value > 0 ? Math.min(100, Math.round((budgeted.value / limit.value) * 100)) : 0,
)
const isOverBudget = computed(() => limit.value > 0 && budgeted.value > limit.value)

function sharePercent(value: number): number {
  if (total.value <= 0) return 0
  return (value / total.value) * 100
}

function shareText(value: number): string {
  const share = sharePercent(value)
  if (share <= 0) return '0%'
  return share < 10 ? `${share.toFixed(1)}%` : `${Math.round(share)}%`
}
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => !open && emit('close')">
    <QtermDialogContent v-if="props.open" data-ai-context-details class="max-w-[440px]">
      <QtermDialogHeader
        :title="t('shell.aiChat.contextDetails')"
        :description="t('shell.aiChat.contextDetailsDescription')"
        :badge="modeLabel"
        :show-close="false"
      />
      <div class="dialog-body flex min-h-0 flex-col gap-3">
        <template v-if="props.breakdown && total > 0">
          <div
            class="flex h-2 w-full overflow-hidden rounded-full bg-qt-subtle"
            role="img"
            :aria-label="t('shell.aiChat.contextDetails')"
            data-ai-context-bar
          >
            <span
              v-for="segment in segments"
              v-show="segment.value > 0"
              :key="segment.key"
              :class="segment.swatch"
              :style="{ width: `${sharePercent(segment.value)}%` }"
              :data-ai-context-segment="segment.key"
            />
          </div>

          <div class="grid gap-1.5 text-xs">
            <div
              v-for="segment in segments"
              :key="segment.key"
              class="flex items-center justify-between gap-3"
              :data-ai-context-row="segment.key"
            >
              <span class="flex min-w-0 items-center gap-2 text-qt-muted">
                <span class="size-2 shrink-0 rounded-[2px]" :class="segment.swatch" aria-hidden="true" />
                <span class="truncate">{{ segment.label }}</span>
              </span>
              <span class="flex shrink-0 items-baseline gap-2 tabular-nums">
                <span v-if="segment.value > 0" class="text-qt-muted">{{ shareText(segment.value) }}</span>
                <span class="text-qt-text">{{ formatByteSize(segment.value) }}</span>
              </span>
            </div>
          </div>

          <div class="border-t border-qt-border pt-2">
            <div class="flex items-center justify-between gap-3 text-xs">
              <span class="text-qt-muted">{{ t('shell.aiChat.contextDetailTotal') }}</span>
              <span class="text-qt-text tabular-nums" data-ai-context-total>{{ formatByteSize(total) }}</span>
            </div>
            <div class="mt-1.5 flex items-center justify-between gap-3 text-xs">
              <span class="text-qt-muted">{{ t('shell.aiChat.contextDetailLimit') }}</span>
              <span v-if="limit > 0" class="flex items-baseline gap-2 tabular-nums">
                <span :class="isOverBudget ? 'text-qt-danger' : 'text-qt-muted'" data-ai-context-percent
                  >{{ usedPercent }}%</span
                >
                <span class="text-qt-text" data-ai-context-limit>{{ formatByteSize(limit) }}</span>
              </span>
              <span v-else class="text-qt-muted" data-ai-context-no-limit>{{
                t('shell.aiChat.contextDetailNoLimit')
              }}</span>
            </div>
            <template v-if="limit > 0">
              <div class="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-qt-muted/20" data-ai-context-budget>
                <div
                  class="h-full rounded-full"
                  :class="isOverBudget ? 'bg-qt-danger' : 'bg-qt-accent'"
                  :style="{ width: `${usedPercent}%` }"
                />
              </div>
              <p class="m-0 pt-1.5 text-xs leading-4 text-qt-muted">
                {{ t('shell.aiChat.contextDetailBudgetHint') }}
              </p>
            </template>
          </div>
        </template>

        <QtermEmptyState
          v-else
          :icon="Layers"
          data-ai-context-empty
          :title="t('shell.aiChat.contextDetailEmpty')"
          :description="t('shell.aiChat.contextDetailEmptyHint')"
        />
      </div>
      <QtermDialogFooter>
        <span class="text-xs text-qt-muted">{{ t('shell.aiChat.contextDetailFootnote') }}</span>
        <div class="dialog-footer-actions flex items-center gap-2">
          <QtermButton variant="outline" type="button" data-ai-context-close @click="emit('close')">{{
            t('shell.app.close')
          }}</QtermButton>
        </div>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>
