<script setup lang="ts">
defineProps<{
  tokens: readonly string[]

  label?: string
}>()
</script>

<template>
  <span
    v-if="tokens.length"
    class="inline-flex items-center gap-1"
    :aria-label="label || undefined"
    :aria-hidden="label ? undefined : 'true'"
  >
    <template v-for="(token, index) in tokens" :key="`${token}-${index}`">
      <kbd class="text-[10px] leading-4 text-foreground">{{ token }}</kbd>
    </template>
  </span>
</template>
