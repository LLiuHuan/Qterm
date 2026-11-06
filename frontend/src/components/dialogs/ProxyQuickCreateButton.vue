<script setup lang="ts">
import { Plus } from '@lucide/vue'
import { IconButton } from '../common'
import { useI18n } from '@/locales'

const props = withDefaults(defineProps<{ disabled?: boolean }>(), { disabled: false })
const emit = defineEmits<{ click: [] }>()
const { t } = useI18n()
</script>

<template>
  <IconButton
    variant="outline"
    :label="t('settings.proxies.action.quickCreate')"
    :disabled="props.disabled"
    data-proxy-quick-create
    @click="emit('click')"
  >
    <Plus :size="16" aria-hidden="true" />
  </IconButton>
</template>
