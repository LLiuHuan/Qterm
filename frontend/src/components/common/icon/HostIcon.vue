<script setup lang="ts">
import { computed } from 'vue'
import { Server } from '@lucide/vue'
import { builtinHostIcons } from '@/features/hosts/builtinIcons.ts'
import { customHostIconSrc } from '@/features/hosts/customHostIcon.ts'
import { resolveHostIcon } from '@/features/hosts/hostIcons.ts'
import { useI18n } from '@/locales'

const props = withDefaults(defineProps<{ icon?: string; size?: number; protocol?: string }>(), {
  icon: '',
  protocol: '',
  size: 18,
})
const { t } = useI18n()

const resolvedIcon = computed(() => resolveHostIcon({ icon: props.icon, protocol: props.protocol }))
const selected = computed(() => builtinHostIcons.find((item) => item.value === resolvedIcon.value))
const customSrc = computed(() => customHostIconSrc(resolvedIcon.value))
</script>

<template>
  <span class="host-logo" :style="{ width: `${size}px`, height: `${size}px` }">
    <img
      v-if="selected"
      :src="selected.src"
      :alt="selected.label"
      :class="{ 'qterm-theme-aware-logo': selected.themeAware }"
      draggable="false"
    />
    <img
      v-else-if="customSrc"
      :src="customSrc"
      :alt="t('shell.hostIcon.alt', { label: t('shell.hostIcon.custom') })"
      draggable="false"
    />
    <Server v-else :size="size" :aria-label="t('shell.asset.host')" />
  </span>
</template>

<style scoped>
.host-logo {
  display: inline-grid;
  place-items: center;
  flex: none;
}
.host-logo img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
  object-position: center;
}
</style>
