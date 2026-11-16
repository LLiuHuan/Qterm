<script setup lang="ts">
import { listExtendPath } from './listMorphIcons'

withDefaults(
  defineProps<{
    size?: number | string
  }>(),
  { size: '1em' },
)
</script>

<template>
  <svg xmlns="http://www.w3.org/2000/svg" :width="size" :height="size" viewBox="0 0 24 24">
    <path d="M0 0h24v24H0z" fill="none" />
    <path
      fill="none"
      stroke="currentColor"
      stroke-linecap="round"
      stroke-linejoin="round"
      stroke-width="2"
      :d="listExtendPath"
    />
  </svg>
</template>

<style scoped></style>
