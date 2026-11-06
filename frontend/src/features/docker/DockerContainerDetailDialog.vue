<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { Check, Copy } from '@lucide/vue'
import { QtermButton, QtermFormMessage, QtermInfoList, type QtermInfoItem } from '../../components/common'
import { QtermDialog, QtermDialogContent, QtermDialogHeader } from '../../components/common/dialog'
import { ScrollArea } from '../../components/common/control-primitives'
import { dockerStateLabelKey, type DockerContainerDetail, type DockerGateway } from '@/services/docker.ts'
import { writeClipboardText } from '@/services/clipboard.ts'
import { errorMessage } from '@/services/errors.ts'
import { formatDateTime } from '@/lib/datetime.ts'
import { useI18n } from '@/locales'

const props = withDefaults(
  defineProps<{
    open: boolean
    gateway: DockerGateway
    sessionId: string
    containerId: string

    containerName?: string
  }>(),
  { containerName: '' },
)

const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()

const detail = ref<DockerContainerDetail | null>(null)
const loading = ref(false)
const error = ref('')

let requestSequence = 0

const stateLabel = computed(() => {
  const state = detail.value?.state ?? ''
  const key = dockerStateLabelKey(state)
  return key ? t(key) : state
})

const basicItems = computed<QtermInfoItem[]>(() => [
  { label: t('shell.docker.detailName'), value: detail.value?.name },
  { label: t('shell.docker.detailId'), value: detail.value?.id, mono: true },
  { label: t('shell.docker.detailImage'), value: detail.value?.image },
  { label: t('shell.docker.detailCreated'), value: detail.value ? formatDateTime(detail.value.created) : '' },
  { label: t('shell.docker.detailState'), value: stateLabel.value },
  { label: t('shell.docker.detailEntrypoint'), value: detail.value?.entrypoint, mono: true },
  { label: t('shell.docker.detailCommand'), value: detail.value?.command, mono: true },
])

const portItems = computed<QtermInfoItem[]>(() =>
  (detail.value?.ports ?? []).map((port) => ({
    label: `${port.privatePort}/${port.type}`,
    value: port.publicPort > 0 ? String(port.publicPort) : t('shell.docker.detailPortNotPublished'),
    mono: true,
  })),
)

const mountItems = computed<QtermInfoItem[]>(() =>
  (detail.value?.mounts ?? []).map((mount) => ({
    label: mount.destination,
    value: mount.source || mount.name || mount.type,
    mono: true,
    badge: mount.readOnly ? t('shell.docker.detailReadOnly') : t('shell.docker.detailReadWrite'),
    badgeTone: mount.readOnly ? 'warning' : 'neutral',
  })),
)

const labelItems = computed<QtermInfoItem[]>(() =>
  (detail.value?.labels ?? []).map((label) => ({ label: label.key, value: label.value, mono: true })),
)

const envCopied = ref(false)
let copiedTimer: number | undefined

function stopCopiedTimer(): void {
  if (copiedTimer !== undefined) {
    window.clearTimeout(copiedTimer)
    copiedTimer = undefined
  }
}

async function copyEnvironment(): Promise<void> {
  const lines = detail.value?.environment ?? []
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

async function load(): Promise<void> {
  const version = ++requestSequence
  if (!props.open || !props.sessionId || !props.containerId) return
  loading.value = true
  error.value = ''
  try {
    const next = await props.gateway.containerDetail(props.sessionId, props.containerId)
    if (version !== requestSequence) return
    detail.value = next
  } catch (cause) {
    if (version !== requestSequence) return
    detail.value = null
    error.value = errorMessage(cause, t('shell.docker.actionFailed'))
  } finally {
    if (version === requestSequence) loading.value = false
  }
}

watch(
  () => [props.open, props.containerId],
  () => {
    if (!props.open) {
      requestSequence += 1
      detail.value = null
      error.value = ''
      loading.value = false
      return
    }
    void load()
  },
  { immediate: true },
)
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => !open && emit('close')">
    <QtermDialogContent
      v-if="props.open"
      size="lg"
      show-close-button
      close-on-escape
      class="docker-detail-dialog"
      @pointer-down-outside="(event) => event.preventDefault()"
    >
      <QtermDialogHeader
        :title="t('shell.docker.detailTitle')"
        :description="props.containerName"
        :show-close="false"
      />

      <div class="docker-detail-body">
        <ScrollArea class="docker-detail-scroll" type="scroll" :scroll-hide-delay="600">
          <div class="flex flex-col gap-4 px-2 py-3">
            <span v-if="loading" class="block text-xs text-qt-muted">{{ t('shell.docker.loading') }}</span>
            <QtermFormMessage v-else-if="error" variant="error">{{ error }}</QtermFormMessage>
            <span v-else-if="!detail" class="block text-xs text-qt-muted">{{ t('shell.docker.detailEmpty') }}</span>
            <template v-else>
              <section class="grid gap-2">
                <h3 class="m-0 text-sm font-medium text-qt-text">{{ t('shell.docker.detailBasic') }}</h3>
                <QtermInfoList :items="basicItems" />
              </section>

              <section class="grid gap-2">
                <h3 class="m-0 text-sm font-medium text-qt-text">{{ t('shell.docker.detailPorts') }}</h3>
                <QtermInfoList v-if="portItems.length" :items="portItems" />
                <span v-else class="block text-xs text-qt-muted">{{ t('shell.docker.detailPortsEmpty') }}</span>
              </section>

              <section class="grid gap-2">
                <h3 class="m-0 text-sm font-medium text-qt-text">{{ t('shell.docker.detailMounts') }}</h3>
                <QtermInfoList v-if="mountItems.length" :items="mountItems" />
                <span v-else class="block text-xs text-qt-muted">{{ t('shell.docker.detailMountsEmpty') }}</span>
              </section>

              <section v-if="detail.environment.length" class="grid gap-2">
                <div class="flex-cb gap-2">
                  <h3 class="m-0 text-sm font-medium text-qt-text">{{ t('shell.docker.detailEnv') }}</h3>
                  <QtermButton
                    variant="ghost"
                    size="icon-sm"
                    type="button"
                    :aria-label="envCopied ? t('shell.docker.detailEnvCopied') : t('shell.docker.detailEnvCopy')"
                    :title="envCopied ? t('shell.docker.detailEnvCopied') : t('shell.docker.detailEnvCopy')"
                    :data-copied="envCopied ? 'true' : 'false'"
                    @click="copyEnvironment"
                  >
                    <Check v-if="envCopied" :size="16" class="size-4" aria-hidden="true" />
                    <Copy v-else :size="16" class="size-4" aria-hidden="true" />
                  </QtermButton>
                </div>
                <ul
                  class="m-0 grid list-none gap-1 rounded-lg border-d px-4 py-2 font-mono text-xs text-qt-text"
                  data-testid="docker-detail-env"
                >
                  <li v-for="(item, index) in detail.environment" :key="`env-${index}`" class="break-all">
                    {{ item }}
                  </li>
                </ul>
              </section>

              <section v-if="labelItems.length" class="grid gap-2">
                <h3 class="m-0 text-sm font-medium text-qt-text">{{ t('shell.docker.detailLabels') }}</h3>
                <QtermInfoList :items="labelItems" />
              </section>
            </template>
          </div>
        </ScrollArea>
      </div>
    </QtermDialogContent>
  </QtermDialog>
</template>
