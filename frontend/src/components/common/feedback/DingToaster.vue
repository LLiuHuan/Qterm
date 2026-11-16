<script lang="ts" setup>
import { computed, defineAsyncComponent } from 'vue'
import { useI18n } from '@/locales'

const DingToaster = defineAsyncComponent(() => import('@yusufceng1z/ding-vue').then((mod) => mod.Toaster))

const props = defineProps<{ dark: boolean }>()

const { t } = useI18n()

const labels = computed(() => ({
  region: t('shell.toast.region'),
  dismiss: t('shell.toast.dismiss'),
}))
</script>

<template>
  <DingToaster position="top-center" :theme="props.dark ? 'dark' : 'light'" :labels="labels" />
</template>
