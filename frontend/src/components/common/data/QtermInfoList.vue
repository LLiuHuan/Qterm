<script lang="ts">
export interface QtermInfoItem {
  label: string

  value?: string

  mono?: boolean

  values?: string[]

  badge?: string

  badgeTone?: 'neutral' | 'warning'
}
</script>

<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { computed } from 'vue'
import { cn } from '@/lib/utils'

const props = withDefaults(
  defineProps<{
    items?: QtermInfoItem[]

    placeholder?: string
    class?: HTMLAttributes['class']
  }>(),
  { items: () => [], placeholder: '—', class: undefined },
)

const LABEL_CLASS = 'max-w-[50vw] pr-5 py-3 text-qt-muted [overflow-wrap:anywhere]'

const ROOT_CLASS = 'grid grid-cols-[max-content_minmax(0,1fr)] rounded-lg border-d px-4 text-xs'

const VALUE_CLASS = 'flex min-w-0 items-start gap-2 py-3 [overflow-wrap:anywhere]'
const BADGE_CLASS = 'shrink-0 rounded-sm border px-1 text-xs'
const BADGE_TONE_CLASS = {
  neutral: 'border-qt-border text-qt-muted',
  warning: 'border-qt-warning/45 text-qt-warning',
} as const

const rows = computed(() =>
  props.items.map((item, index) => ({
    label: item.label,

    values: (item.values ?? []).filter((value) => value.trim() !== ''),
    value: item.value?.trim() ? item.value : props.placeholder,
    mono: item.mono === true,
    badge: item.badge ?? '',
    badgeClass: BADGE_TONE_CLASS[item.badgeTone ?? 'neutral'],
    divider: index < props.items.length - 1,
  })),
)
</script>

<template>
  <div :class="[ROOT_CLASS, cn(props.class)]" data-slot="info-list">
    <template v-for="(row, index) in rows" :key="`${row.label}-${index}`">
      <div :class="[LABEL_CLASS, row.divider && 'border-b border-qt-border/50']">{{ row.label }}</div>
      <div :class="[VALUE_CLASS, row.divider && 'border-b border-qt-border/50']">
        <span
          v-if="row.values.length"
          class="flex min-w-0 flex-col gap-0.5 text-qt-text"
          :class="{ 'font-mono': row.mono }"
        >
          <span v-for="(entry, entryIndex) in row.values" :key="entryIndex" class="min-w-0">{{ entry }}</span>
        </span>
        <span v-else class="min-w-0 text-qt-text" :class="{ 'font-mono': row.mono }">
          {{ row.value }}
        </span>
        <span v-if="row.badge" :class="[BADGE_CLASS, row.badgeClass]">{{ row.badge }}</span>
      </div>
    </template>
  </div>
</template>
