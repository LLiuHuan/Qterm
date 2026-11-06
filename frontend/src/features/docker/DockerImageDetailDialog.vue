<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { Check, Copy } from '@lucide/vue'
import { QtermButton, QtermInfoList, type QtermInfoItem } from '../../components/common'
import { writeClipboardText } from '@/services/clipboard'
import {
  dockerRemoveTarget,
  type DockerGateway,
  type DockerImage,
  type DockerImageDetailView,
} from '../../services/docker'
import { formatDateTime } from '@/lib/datetime'
import { useI18n } from '@/locales'
import DockerResourceDetailDialog from './DockerResourceDetailDialog.vue'

const props = withDefaults(
  defineProps<{
    open: boolean

    image?: DockerImage | null
    sessionId: string
    gateway: DockerGateway
  }>(),
  { image: null },
)

const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()

function imageRef(): string {
  const image = props.image
  if (!image) return ''
  return dockerRemoveTarget(image) || image.id || ''
}

async function load(): Promise<DockerImageDetailView> {
  return props.gateway.imageDetail(props.sessionId, imageRef())
}

function description(): string {
  const image = props.image
  if (!image) return ''
  return dockerRemoveTarget(image)
}

function basicItems(detail: DockerImageDetailView): QtermInfoItem[] {
  const items: QtermInfoItem[] = [
    { label: t('shell.docker.imageId'), value: detail.id, mono: true },
    { label: t('shell.docker.imageReference'), value: detail.reference || '<none>', mono: true },
    { label: t('shell.docker.imageDigest'), value: detail.digest, mono: true },
    { label: t('shell.docker.detailCreated'), value: formatDateTime(detail.created) },
    { label: t('shell.docker.imageSize'), value: detail.size },
    { label: t('shell.docker.imageSizeBytes'), value: `${detail.sizeBytes} B`, mono: true },
    { label: t('shell.docker.imageArchitecture'), value: detail.architecture },
    { label: t('shell.docker.imageOs'), value: detail.os },
    { label: t('shell.docker.imageAuthor'), value: detail.author },
  ]
  if (detail.layers > 0) {
    items.push({
      label: t('shell.docker.imageLayers'),
      value: t('shell.docker.imageLayersValue', { count: detail.layers }),
    })
  }
  if (props.image) {
    items.push({
      label: t('shell.docker.detailInUse'),
      value: t(props.image.inUse ? 'shell.docker.detailYes' : 'shell.docker.detailNo'),
    })
  }

  items.push(
    { label: t('shell.docker.imageExposedPorts'), values: detail.exposedPorts, mono: true },
    { label: t('shell.docker.imageRepoTags'), values: detail.repoTags, mono: true },
    { label: t('shell.docker.imageRepoDigests'), values: detail.repoDigests, mono: true },
  )
  return items
}

function commandItems(detail: DockerImageDetailView): QtermInfoItem[] {
  return [
    { label: t('shell.docker.detailEntrypoint'), value: detail.entrypoint, mono: true },
    { label: t('shell.docker.detailCommand'), value: detail.command, mono: true },
  ]
}

const envCopied = ref(false)
let copiedTimer: number | undefined

function stopCopiedTimer(): void {
  if (copiedTimer !== undefined) {
    window.clearTimeout(copiedTimer)
    copiedTimer = undefined
  }
}

async function copyEnvironment(lines: string[]): Promise<void> {
  if (!lines.length) return
  try {
    await writeClipboardText(lines.join('\n'))
    envCopied.value = true
    stopCopiedTimer()
    copiedTimer = window.setTimeout(() => {
      envCopied.value = false
      copiedTimer = undefined
    }, 1600)
  } catch {
    envCopied.value = false
  }
}

onBeforeUnmount(stopCopiedTimer)
</script>

<template>
  <DockerResourceDetailDialog
    :open="props.open"
    :title="t('shell.docker.imageDetailTitle')"
    :description="description()"
    :target-key="imageRef()"
    :load="load"
    @close="emit('close')"
  >
    <template #default="{ detail }">
      <section class="grid gap-2">
        <h3 class="m-0 text-sm font-medium text-qt-text">{{ t('shell.docker.detailBasic') }}</h3>
        <QtermInfoList :items="basicItems(detail)" />
      </section>

      <section class="grid gap-2">
        <h3 class="m-0 text-sm font-medium text-qt-text">{{ t('shell.docker.detailCommandSection') }}</h3>
        <QtermInfoList :items="commandItems(detail)" />
      </section>

      <section class="grid gap-2">
        <div class="flex items-center justify-between gap-2">
          <h3 class="m-0 text-sm font-medium text-qt-text">{{ t('shell.docker.detailEnv') }}</h3>
          <QtermButton
            v-if="detail.environment.length"
            variant="ghost"
            size="icon-sm"
            type="button"
            :aria-label="envCopied ? t('shell.docker.detailEnvCopied') : t('shell.docker.detailEnvCopy')"
            :title="envCopied ? t('shell.docker.detailEnvCopied') : t('shell.docker.detailEnvCopy')"
            :data-copied="envCopied ? 'true' : 'false'"
            @click="copyEnvironment(detail.environment)"
          >
            <Check v-if="envCopied" :size="16" class="size-4" aria-hidden="true" />
            <Copy v-else :size="16" class="size-4" aria-hidden="true" />
          </QtermButton>
        </div>
        <ul
          v-if="detail.environment.length"
          class="m-0 grid list-none gap-1 rounded-lg border-d px-4 py-2 font-mono text-xs text-qt-text"
          data-testid="docker-image-detail-env"
        >
          <li v-for="(item, index) in detail.environment" :key="`env-${index}`" class="break-all">{{ item }}</li>
        </ul>
        <span v-else class="block text-xs text-qt-muted">{{ t('shell.docker.imageEnvEmpty') }}</span>
      </section>

      <section class="grid gap-2">
        <h3 class="m-0 text-sm font-medium text-qt-text">{{ t('shell.docker.detailLabels') }}</h3>
        <QtermInfoList
          v-if="detail.labels.length"
          :items="detail.labels.map((label) => ({ label: label.key, value: label.value, mono: true }))"
        />
        <span v-else class="block text-xs text-qt-muted">{{ t('shell.docker.imageLabelsEmpty') }}</span>
      </section>
    </template>
  </DockerResourceDetailDialog>
</template>
