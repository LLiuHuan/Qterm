<script setup lang="ts">
import AiMessageText from '@/features/ai/AiMessageText.vue'

const props = defineProps<{ text: string; streaming?: boolean }>()
</script>

<template>
  <div class="min-w-0">
    <AiMessageText :text="props.text" :streaming="props.streaming ?? false" />
  </div>
</template>
