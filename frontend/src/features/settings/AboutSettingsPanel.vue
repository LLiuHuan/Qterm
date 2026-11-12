<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ClipboardCopy, Download, ExternalLink, RefreshCw, RotateCcw, Trash2 } from '@lucide/vue'
import { System } from '@wailsio/runtime'
import { toast } from 'vue-sonner'
import { IconButton, QtermButton, QtermSwitch } from '../../components/common'
import { useI18n } from '@/locales'
import { QTERM_DEV_BUILD, QTERM_VERSION } from '../../services/appInfo'
import { writeClipboardText } from '../../services/clipboard'
import type { CredentialGateway } from '../../services/credentials'
import {
  clearDebugLog,
  debugLoggingEnabled,
  diagnosticsAvailable,
  exportDebugLog,
  readDebugLog,
  setDebugLoggingEnabled,
} from '../../services/diagnostics'
import { checkUpdate, downloadUpdate, installUpdate, ensureUpdateChecks, useUpdateState } from '../updates/updateStore'
import { translateErrorKey } from '../../services/errors'
import { formatByteSize } from '../../lib/format'
import { openExternalUrl } from '../../services/externalLink'
import { updatesAvailable } from '../../services/updates'
import AiMessageText from '../ai/AiMessageText.vue'
import AssetTransferPanel from './AssetTransferPanel.vue'

const props = defineProps<{
  credentialGateway?: CredentialGateway
  locked?: boolean
}>()

const emit = defineEmits<{ resourcesImported: [] }>()
const { t } = useI18n()
const desktopAvailable = diagnosticsAvailable()
const debugEnabled = ref(false)
const debugBusy = ref(false)
const environment = ref({ runtime: t('about.environment.browser'), os: browserOS(), arch: browserArchitecture() })

const supportInformation = computed(() =>
  [
    `Qterm: v${QTERM_VERSION}`,
    `${t('about.environment.label')}: ${environment.value.runtime}`,
    `${t('about.os.label')}: ${environment.value.os}`,
    `${t('about.arch.label')}: ${environment.value.arch}`,
  ].join('\n'),
)

const updateState = useUpdateState()
const updateStatus = computed(() => updateState.status)
const updateRelease = computed(() => updateStatus.value.release)
const updateAvailable = updatesAvailable()

const updateDescriptionKey = computed(() => {
  if (!updateAvailable) return 'about.update.desktopOnly'
  return QTERM_DEV_BUILD ? 'about.update.devBuild' : 'about.update.description'
})
const downloading = computed(() => updateStatus.value.state === 'downloading')
const installing = computed(() => updateStatus.value.state === 'installing')
const canCheck = computed(() => updateAvailable && !updateState.busy && !downloading.value)
const canDownload = computed(
  () => updateStatus.value.state === 'available' && Boolean(updateRelease.value?.asset.url) && !updateState.busy,
)
const canInstall = computed(() => updateStatus.value.state === 'ready' && !updateState.busy)
const updateError = computed(() =>
  updateState.errorKey ? translateErrorKey(updateState.errorKey, t('about.update.state.failed')) : '',
)
const updateStateLabel = computed(() => t(`about.update.state.${updateStatus.value.state}`))

onMounted(() => {
  ensureUpdateChecks()
})

async function checkNow(): Promise<void> {
  await checkUpdate()
}

async function downloadNow(): Promise<void> {
  await downloadUpdate()
}

async function installNow(): Promise<void> {
  await installUpdate()
  if (updateState.errorKey) {
    toast.error(translateErrorKey(updateState.errorKey, t('about.update.state.failed')))
    return
  }
  toast.info(t('about.update.restarting'))

  window.setTimeout(() => {
    void import('@wailsio/runtime').then(({ Application }) => Application.Quit())
  }, 400)
}

function openReleasePage(): void {
  const url = updateRelease.value?.pageUrl
  if (url) void openExternalUrl(url)
}

onMounted(async () => {
  if (!desktopAvailable) return
  try {
    const info = await System.Environment()
    const osName = info.OSInfo?.Name || info.OS
    const osVersion = info.OSInfo?.Version ? ` ${info.OSInfo.Version}` : ''
    environment.value = {
      runtime: t('about.environment.desktop'),
      os: `${osName}${osVersion}`,
      arch: info.Arch || t('about.value.unknown'),
    }
    debugEnabled.value = await debugLoggingEnabled()
  } catch {
    toast.error(t('about.support.loadFailure'))
  }
})

function browserOS(): string {
  return navigator.platform || t('about.value.unknown')
}

function browserArchitecture(): string {
  const match = navigator.userAgent.match(/\b(arm64|aarch64|x86_64|x64|amd64)\b/i)
  return match?.[1] ?? t('about.value.unknown')
}

async function copySupportInformation(): Promise<void> {
  try {
    await writeClipboardText(supportInformation.value)
    toast.success(t('about.support.copySuccess'))
  } catch {
    toast.error(t('about.support.copyFailure'))
  }
}

async function updateDebugLogging(enabled: boolean): Promise<void> {
  if (!desktopAvailable || debugBusy.value) return
  debugBusy.value = true
  try {
    await setDebugLoggingEnabled(enabled)
    debugEnabled.value = enabled
    toast.success(t(enabled ? 'about.debug.enabled' : 'about.debug.disabled'))
  } catch {
    toast.error(t('about.debug.actionFailure'))
  } finally {
    debugBusy.value = false
  }
}

async function clearLog(): Promise<void> {
  if (debugBusy.value) return
  debugBusy.value = true
  try {
    await clearDebugLog()
    toast.success(t('about.debug.clearSuccess'))
  } catch {
    toast.error(t('about.debug.actionFailure'))
  } finally {
    debugBusy.value = false
  }
}

async function copyLog(): Promise<void> {
  if (debugBusy.value) return
  debugBusy.value = true
  try {
    const content = await readDebugLog()
    if (!content) {
      toast.info(t('about.debug.empty'))
      return
    }
    await writeClipboardText(content)
    toast.success(t('about.debug.copySuccess'))
  } catch {
    toast.error(t('about.debug.actionFailure'))
  } finally {
    debugBusy.value = false
  }
}

async function downloadLog(): Promise<void> {
  if (debugBusy.value) return
  debugBusy.value = true
  try {
    if (await exportDebugLog()) toast.success(t('about.debug.exportSuccess'))
  } catch {
    toast.error(t('about.debug.actionFailure'))
  } finally {
    debugBusy.value = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <section
      class="settings-card p-4 border border-solid border-qt-border rounded-xl [background:color-mix(in_srgb,var(--qt-panel)_32%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
      :aria-label="t('about.update.title')"
    >
      <div class="flex items-start justify-between gap-4">
        <span class="flex min-w-0 flex-col gap-0.5">
          <strong class="text-sm">{{ t('about.update.title') }}</strong>
          <small class="text-xs text-qt-muted">{{ t(updateDescriptionKey) }}</small>
        </span>
        <QtermButton variant="outline" type="button" class="text-xs" :disabled="!canCheck" @click="checkNow">
          <RefreshCw :size="14" aria-hidden="true" />
          {{ updateState.busy ? t('about.update.checking') : t('about.update.check') }}
        </QtermButton>
      </div>
      <dl class="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <div class="rounded-md bg-qt-subtle px-3 py-2">
          <dt class="text-xs text-qt-muted">{{ t('about.update.current') }}</dt>
          <dd class="mt-1 font-mono text-xs">v{{ QTERM_VERSION }}</dd>
        </div>
        <div class="rounded-md bg-qt-subtle px-3 py-2">
          <dt class="text-xs text-qt-muted">{{ t('about.update.latest') }}</dt>
          <dd class="mt-1 font-mono text-xs">
            {{ updateRelease ? `v${updateRelease.version}` : t('about.value.unknown') }}
          </dd>
        </div>
        <div class="rounded-md bg-qt-subtle px-3 py-2">
          <dt class="text-xs text-qt-muted">{{ t('about.update.package') }}</dt>
          <dd class="mt-1 font-mono text-xs">
            {{ updateRelease?.asset.size ? formatByteSize(updateRelease.asset.size) : t('about.value.unknown') }}
          </dd>
        </div>
        <div class="rounded-md bg-qt-subtle px-3 py-2">
          <dt class="text-xs text-qt-muted">{{ t('about.update.state') }}</dt>
          <dd class="mt-1 text-xs" data-update-state>{{ updateStateLabel }}</dd>
        </div>
      </dl>
      <div v-if="downloading" class="mt-4 flex flex-col gap-1.5" data-update-progress>
        <div class="h-1.5 overflow-hidden rounded-full bg-qt-subtle">
          <div
            class="h-full rounded-full bg-qt-accent transition-[width] duration-200"
            :style="{ width: `${updateStatus.percent}%` }"
          />
        </div>
        <span class="text-xs text-qt-muted">{{
          t('about.update.downloading', { percent: updateStatus.percent })
        }}</span>
      </div>
      <p v-if="updateError" class="mt-3 mb-0 text-xs text-qt-danger" data-update-error>{{ updateError }}</p>

      <details v-if="updateRelease?.notes" class="mt-3">
        <summary class="cursor-pointer text-xs text-qt-muted">{{ t('about.update.releaseNotes') }}</summary>
        <div class="mt-2 max-h-40 overflow-auto text-xs text-qt-muted" data-update-notes>
          <AiMessageText :text="updateRelease.notes" />
        </div>
      </details>
      <div class="mt-4 flex flex-wrap justify-end gap-2">
        <QtermButton v-if="updateRelease?.pageUrl" variant="outline" type="button" @click="openReleasePage">
          <ExternalLink :size="14" aria-hidden="true" />
          {{ t('about.update.openPage') }}
        </QtermButton>
        <QtermButton v-if="canDownload" type="button" data-update-download @click="downloadNow">
          <Download :size="14" aria-hidden="true" />
          {{ t('about.update.download') }}
        </QtermButton>
        <QtermButton v-if="canInstall" type="button" data-update-install @click="installNow">
          <RotateCcw :size="14" aria-hidden="true" />
          {{ installing ? t('about.update.installing') : t('about.update.install') }}
        </QtermButton>
      </div>
    </section>

    <section
      class="settings-card p-4 border border-solid border-qt-border rounded-xl [background:color-mix(in_srgb,var(--qt-panel)_32%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
      :aria-label="t('about.support.title')"
    >
      <div class="flex items-start justify-between gap-4">
        <span class="flex min-w-0 flex-col gap-0.5">
          <strong class="text-sm">{{ t('about.support.title') }}</strong>
          <small class="text-xs text-qt-muted">{{ t('about.support.description') }}</small>
        </span>
        <IconButton :label="t('about.support.copy')" variant="outline" @click="copySupportInformation">
          <ClipboardCopy :size="16" aria-hidden="true" />
        </IconButton>
      </div>
      <dl class="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <div class="rounded-md bg-qt-subtle px-3 py-2">
          <dt class="text-xs text-qt-muted">{{ t('about.version.label') }}</dt>
          <dd class="mt-1 font-mono text-xs">v{{ QTERM_VERSION }}</dd>
        </div>
        <div class="rounded-md bg-qt-subtle px-3 py-2">
          <dt class="text-xs text-qt-muted">{{ t('about.environment.label') }}</dt>
          <dd class="mt-1 text-xs">{{ environment.runtime }}</dd>
        </div>
        <div class="rounded-md bg-qt-subtle px-3 py-2">
          <dt class="text-xs text-qt-muted">{{ t('about.os.label') }}</dt>
          <dd class="mt-1 font-mono text-xs">{{ environment.os }}</dd>
        </div>
        <div class="rounded-md bg-qt-subtle px-3 py-2">
          <dt class="text-xs text-qt-muted">{{ t('about.arch.label') }}</dt>
          <dd class="mt-1 font-mono text-xs">{{ environment.arch }}</dd>
        </div>
      </dl>
    </section>

    <AssetTransferPanel
      v-if="props.credentialGateway"
      :gateway="props.credentialGateway"
      :disabled="props.locked"
      :label="t('about.transfer.title')"
      :description="t('about.transfer.description')"
      @imported="emit('resourcesImported')"
    />

    <section
      class="settings-card p-4 border border-solid border-qt-border rounded-[8px] [background:color-mix(in_srgb,_var(--qt-panel)_32%,_transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
      :aria-label="t('about.debug.title')"
    >
      <div class="flex items-start justify-between gap-4">
        <span class="flex min-w-0 flex-col gap-0.5">
          <strong class="text-sm">{{ t('about.debug.title') }}</strong>
          <small class="text-xs text-qt-muted">{{
            desktopAvailable ? t('about.debug.description') : t('about.debug.desktopOnly')
          }}</small>
        </span>
        <QtermSwitch
          :model-value="debugEnabled"
          :disabled="!desktopAvailable || debugBusy"
          :aria-label="t('about.debug.title')"
          @update:model-value="updateDebugLogging"
        />
      </div>
      <div class="mt-4 flex flex-wrap justify-end gap-2">
        <QtermButton variant="outline" type="button" :disabled="!desktopAvailable || debugBusy" @click="clearLog">
          <Trash2 :size="14" aria-hidden="true" />
          {{ t('about.debug.clear') }}
        </QtermButton>
        <QtermButton variant="outline" type="button" :disabled="!desktopAvailable || debugBusy" @click="copyLog">
          <ClipboardCopy :size="14" aria-hidden="true" />
          {{ t('about.debug.copy') }}
        </QtermButton>
        <QtermButton variant="outline" type="button" :disabled="!desktopAvailable || debugBusy" @click="downloadLog">
          <Download :size="14" aria-hidden="true" />
          {{ t('about.debug.export') }}
        </QtermButton>
      </div>
    </section>
  </div>
</template>
