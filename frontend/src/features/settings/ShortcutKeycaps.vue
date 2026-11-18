<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    tokens: readonly string[]

    label?: string

    compact?: boolean
  }>(),
  { label: '', compact: false },
)

const keycapSizeClass = computed(() =>
  props.compact
    ? 'h-5 min-w-[20px] px-[4px] pt-px text-[11px] leading-none'
    : 'h-6 min-w-[26px] px-[6px] pt-px text-xs leading-none',
)
</script>

<template>
  <span
    v-if="tokens.length"
    class="inline-flex items-center gap-1"
    :aria-label="label || undefined"
    :aria-hidden="label ? undefined : 'true'"
  >
    <template v-for="(token, index) in tokens" :key="`${token}-${index}`">
      <kbd
        :class="[
          'inline-flex items-center justify-center rounded-[4px] border border-solid border-qt-border bg-qt-subtle font-mono text-qt-text',
          keycapSizeClass,
        ]"
        >{{ token }}</kbd
      >

      <span v-if="index < tokens.length - 1" class="pt-px text-xs leading-none text-qt-muted">+</span>
    </template>
  </span>
</template>
