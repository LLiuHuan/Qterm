<script setup lang="ts">
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
    <path fill-rule="evenodd" d="M16 6H8v12h8zm4 16H4V2h16z" />
  </svg>
</template>

<style scoped></style>
