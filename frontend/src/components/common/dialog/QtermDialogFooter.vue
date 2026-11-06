<script setup lang="ts">
import { Separator } from '../../ui/separator'

const props = withDefaults(
  defineProps<{
    divider?: boolean
  }>(),
  { divider: true },
)
</script>

<template>
  <footer class="dialog-footer" data-slot="dialog-footer">
    <Separator v-if="props.divider" class="dialog-footer-separator top-0" />
    <slot />
  </footer>
</template>
