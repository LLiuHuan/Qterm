<script setup lang="ts">
import { QtermInfoList, type QtermInfoItem } from '../../components/common'
import type { DockerGateway, DockerVolume, DockerVolumeDetailView } from '../../services/docker'
import { formatDateTime } from '@/lib/datetime'
import { useI18n } from '@/locales'
import DockerResourceDetailDialog from './DockerResourceDetailDialog.vue'

const props = withDefaults(
  defineProps<{
    open: boolean

    volume?: DockerVolume | null
    sessionId: string
    gateway: DockerGateway
  }>(),
  { volume: null },
)

const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()

function volumeName(): string {
  return props.volume?.name ?? ''
}

async function load(): Promise<DockerVolumeDetailView> {
  return props.gateway.volumeDetail(props.sessionId, volumeName())
}

function basicItems(detail: DockerVolumeDetailView): QtermInfoItem[] {
  return [
    { label: t('shell.docker.detailName'), value: detail.name, mono: true },
    { label: t('shell.docker.volumeDriver'), value: detail.driver },
    { label: t('shell.docker.volumeScope'), value: detail.scope },
    { label: t('shell.docker.detailCreated'), value: formatDateTime(detail.createdAt) },
    { label: t('shell.docker.volumeMountpoint'), value: detail.mountpoint, mono: true },
  ]
}

function optionItems(detail: DockerVolumeDetailView): QtermInfoItem[] {
  return detail.options.map((option) => ({ label: option.key, value: option.value, mono: true }))
}

function labelItems(detail: DockerVolumeDetailView): QtermInfoItem[] {
  return detail.labels.map((label) => ({ label: label.key, value: label.value, mono: true }))
}
</script>

<template>
  <DockerResourceDetailDialog
    :open="props.open"
    :title="t('shell.docker.volumeDetailTitle')"
    :description="volumeName()"
    :target-key="volumeName()"
    :load="load"
    @close="emit('close')"
  >
    <template #default="{ detail }">
      <section class="grid gap-2">
        <h3 class="m-0 text-sm font-medium text-qt-text">{{ t('shell.docker.detailBasic') }}</h3>
        <QtermInfoList :items="basicItems(detail)" />
      </section>

      <section class="grid gap-2">
        <h3 class="m-0 text-sm font-medium text-qt-text">{{ t('shell.docker.volumeOptions') }}</h3>
        <QtermInfoList v-if="detail.options.length" :items="optionItems(detail)" />
        <span v-else class="block text-xs text-qt-muted">{{ t('shell.docker.volumeOptionsEmpty') }}</span>
      </section>

      <section class="grid gap-2">
        <h3 class="m-0 text-sm font-medium text-qt-text">{{ t('shell.docker.detailLabels') }}</h3>
        <QtermInfoList v-if="detail.labels.length" :items="labelItems(detail)" />
        <span v-else class="block text-xs text-qt-muted">{{ t('shell.docker.volumeLabelsEmpty') }}</span>
      </section>
    </template>
  </DockerResourceDetailDialog>
</template>
