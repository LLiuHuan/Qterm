<script setup lang="ts">
import { computed } from 'vue'
import { Braces } from '@lucide/vue'
import { builtinHostIcons } from '../hosts/builtinIcons'
import { customHostIconSrc } from '../hosts/customHostIcon'

const props = withDefaults(defineProps<{ icon?: string; size?: number }>(), { icon: '', size: 16 })
const selected = computed(() => builtinHostIcons.find((item) => item.value === props.icon))
const customSrc = computed(() => customHostIconSrc(props.icon))
</script>

<template>
  <span class="snippet-icon" :style="{ width: `${size}px`, height: `${size}px` }" aria-hidden="true">
    <img
      v-if="selected"
      :src="selected.src"
      alt=""
      :class="{ 'qterm-theme-aware-logo': selected.themeAware }"
      draggable="false"
    />
    <img v-else-if="customSrc" :src="customSrc" alt="" draggable="false" />
    <Braces v-else :size="size" />
  </span>
</template>

<style scoped>
.snippet-icon {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
}
.snippet-icon img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}
</style>
