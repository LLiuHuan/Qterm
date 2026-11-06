<script setup lang="ts">
import type { HTMLAttributes } from 'vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{
  class?: HTMLAttributes['class']
}>()
</script>

<template>
  <p
    v-bind="$attrs"
    class="rounded-lg border border-dashed border-qt-border bg-qt-subtle px-3 py-2 text-xs text-qt-muted"
    :class="props.class"
  >
    <slot />
  </p>
</template>
