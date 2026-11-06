<script setup lang="ts">
import { computed } from 'vue'
import { markdownUrl } from './markdownPolicy'

defineOptions({ inheritAttrs: false })
const props = defineProps<{ src?: string; alt?: string; title?: string }>()
const source = computed(() => markdownUrl(props.src))
</script>

<template>
  <img
    v-if="source"
    :src="source"
    :alt="props.alt || ''"
    :title="props.title"
    class="max-w-full rounded-md"
    loading="lazy"
    decoding="async"
    referrerpolicy="no-referrer"
  />
  <span v-else>{{ props.alt }}</span>
</template>
