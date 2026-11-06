<script setup lang="ts">
withDefaults(
  defineProps<{
    size?: number | string
  }>(),
  { size: '1em' },
)
</script>

<template>
  <svg xmlns="http://www.w3.org/2000/svg" :width="size" :height="size" viewBox="0 0 48 48">
    <path d="M0 0h48v48H0z" fill="none" />
    <g fill="none" stroke-linejoin="round" stroke-width="4">
      <rect width="40" height="32" x="4" y="8" fill="#2F88FF" stroke="#000" rx="2" />
      <path stroke="#fff" stroke-linecap="round" d="M12 18L19 24L12 30" />
      <path stroke="#fff" stroke-linecap="round" d="M23 32H36" />
    </g>
  </svg>
</template>

<style scoped></style>
