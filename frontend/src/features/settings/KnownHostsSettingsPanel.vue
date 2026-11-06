<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Copy, KeyRound, RefreshCw, ShieldAlert, ShieldCheck, Trash2 } from '@lucide/vue'
import { IconButton, QtermButton, QtermEmptyState, QtermFormMessage, QtermSearchField } from '../../components/common'
import { ScrollArea } from '../../components/common/control-primitives'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { useI18n } from '../../locales'
import { toast } from 'vue-sonner'
import { knownHostsErrorMessage, type KnownHostEntry, type KnownHostsGateway } from '../../services/knownHosts'

const props = defineProps<{ gateway: KnownHostsGateway }>()

const { t } = useI18n()

const entries = ref<KnownHostEntry[]>([])

const hostsPath = ref('')
const loading = ref(false)
const error = ref('')
const query = ref('')
const pendingDelete = ref<KnownHostEntry | null>(null)
const deleting = ref(false)

const visibleEntries = computed(() => {
  const keyword = query.value.trim().toLowerCase()
  if (!keyword) return entries.value
  return entries.value.filter((entry) =>
    [entry.host, entry.keyType, entry.sha256Fingerprint, entry.comment].join(' ').toLowerCase().includes(keyword),
  )
})

function isHashedHost(host: string): boolean {
  return host.startsWith('|')
}

function hostText(entry: KnownHostEntry): string {
  return isHashedHost(entry.host) ? t('settings.knownHosts.hashedHost') : entry.host
}

async function load(): Promise<void> {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try {
    const [list, path] = await Promise.all([props.gateway.list(), props.gateway.path()])
    entries.value = list
    hostsPath.value = path
  } catch (cause) {
    error.value = knownHostsErrorMessage(cause, t('settings.knownHosts.loadFailed'))
  } finally {
    loading.value = false
  }
}

function askDelete(entry: KnownHostEntry): void {
  if (!entry.editable) return
  pendingDelete.value = entry
}

async function confirmDelete(): Promise<void> {
  const target = pendingDelete.value
  if (!target || deleting.value) return
  deleting.value = true
  try {
    await props.gateway.remove(target.line)
    pendingDelete.value = null
    await load()
  } catch (cause) {
    error.value = knownHostsErrorMessage(cause, t('settings.knownHosts.removeFailed'))
  } finally {
    deleting.value = false
  }
}

async function copyFingerprint(entry: KnownHostEntry): Promise<void> {
  if (!entry.sha256Fingerprint) return
  try {
    await navigator.clipboard.writeText(entry.sha256Fingerprint)
    toast.success(t('settings.knownHosts.copied'))
  } catch {
    toast.error(t('settings.knownHosts.copyFailed'))
  }
}

onMounted(() => void load())
</script>

<template>
  <section class="flex h-full min-h-0 flex-col gap-4" :aria-label="t('settings.knownHosts.title')">
    <div class="flex min-w-0 items-center gap-2 pr-2">
      <ShieldCheck :size="16" class="shrink-0 text-qt-muted" aria-hidden="true" />
      <h3 class="m-0 shrink-0 text-sm font-medium">{{ t('settings.knownHosts.title') }}</h3>
      <span
        v-if="hostsPath"
        class="min-w-0 truncate font-mono text-[11px] text-qt-muted"
        :title="hostsPath"
        data-known-host-path
        >{{ hostsPath }}</span
      >
      <span class="ms-auto shrink-0 text-xs text-qt-muted">{{
        t('settings.knownHosts.count', { count: entries.length })
      }}</span>
    </div>

    <div class="flex items-center gap-2 pr-2">
      <QtermSearchField
        v-model="query"
        class="min-w-0 flex-1"
        v-bind="{ ariaLabel: t('settings.knownHosts.search') }"
        :placeholder="t('settings.knownHosts.search')"
      />
      <IconButton
        :label="loading ? t('settings.knownHosts.loading') : t('settings.knownHosts.refresh')"
        size="icon-sm"
        :disabled="loading"
        data-known-host-refresh
        @click="load"
      >
        <RefreshCw :size="14" aria-hidden="true" :class="{ 'animate-spin': loading }" />
      </IconButton>
    </div>

    <p v-if="loading && !entries.length" class="panel-empty-note text-qt-muted text-xs">
      {{ t('settings.knownHosts.loading') }}
    </p>
    <QtermFormMessage v-else-if="error" variant="error">{{ error }}</QtermFormMessage>

    <ScrollArea v-else-if="visibleEntries.length" type="hover" :scroll-hide-delay="600" class="min-h-0 flex-1">
      <div class="flex flex-col gap-2 pr-2">
        <article
          v-for="entry in visibleEntries"
          :key="`${entry.source}:${entry.line}`"
          class="settings-library-row flex min-w-0 items-center gap-3 rounded-md border-d px-3 py-2"
          data-known-host-row
        >
          <ShieldAlert v-if="entry.marker" :size="18" class="shrink-0 text-qt-warning" aria-hidden="true" />
          <ShieldCheck v-else :size="18" class="shrink-0 text-qt-muted" aria-hidden="true" />
          <div class="flex min-w-0 flex-1 flex-col gap-0.5">
            <div class="flex min-w-0 items-center gap-2">
              <strong class="truncate font-mono text-sm" :title="hostText(entry)">{{ hostText(entry) }}</strong>
              <span class="shrink-0 rounded-full border-d bg-qt-subtle px-2 text-xs leading-4 text-qt-muted">{{
                entry.keyType
              }}</span>
              <span v-if="entry.marker" class="shrink-0 rounded-full border-d px-2 text-xs leading-4 text-qt-warning">{{
                entry.marker
              }}</span>
              <span
                v-if="!entry.editable"
                class="shrink-0 rounded-full border-d bg-qt-subtle px-2 text-xs leading-4 text-qt-muted"
                >{{ t('settings.knownHosts.readOnly') }}</span
              >
            </div>
            <span class="truncate font-mono text-xs text-qt-muted" :title="entry.sha256Fingerprint">
              {{ entry.sha256Fingerprint || t('settings.knownHosts.noFingerprint') }}
              <template v-if="entry.comment"> · {{ entry.comment }}</template>
            </span>
          </div>
          <div class="settings-library-row-actions flex shrink-0 items-center gap-1">
            <IconButton
              v-if="entry.sha256Fingerprint"
              :label="t('settings.knownHosts.copy')"
              size="icon-sm"
              data-known-host-copy
              @click="copyFingerprint(entry)"
            >
              <Copy :size="15" aria-hidden="true" />
            </IconButton>
            <IconButton
              v-if="entry.editable"
              danger
              size="icon-sm"
              :label="t('settings.knownHosts.delete')"
              data-known-host-delete
              @click="askDelete(entry)"
            >
              <Trash2 :size="15" aria-hidden="true" />
            </IconButton>
          </div>
        </article>
      </div>
    </ScrollArea>

    <div v-else-if="entries.length" class="flex flex-col items-center gap-2 py-10 text-center">
      <KeyRound :size="28" class="text-qt-muted" aria-hidden="true" />
      <strong class="text-sm">{{ t('settings.knownHosts.noMatch') }}</strong>
    </div>

    <QtermEmptyState
      v-else
      class="flex-1"
      :icon="KeyRound"
      :title="t('settings.knownHosts.empty')"
      :description="t('settings.knownHosts.emptyHint')"
    />

    <QtermDialog :open="pendingDelete !== null" @update:open="(open) => !open && (pendingDelete = null)">
      <QtermDialogContent v-if="pendingDelete" data-known-host-delete-dialog>
        <QtermDialogHeader
          :title="t('settings.knownHosts.deleteTitle')"
          :description="t('settings.knownHosts.deleteDescription')"
          :show-close="false"
        />
        <div class="dialog-body">
          <p class="text-sm text-qt-text">
            {{ t('settings.knownHosts.deleteConfirm', { host: hostText(pendingDelete) }) }}
          </p>
        </div>
        <QtermDialogFooter>
          <div></div>
          <div class="dialog-footer-actions flex items-center gap-[8px]">
            <QtermButton variant="outline" :disabled="deleting" @click="pendingDelete = null">{{
              t('settings.knownHosts.cancel')
            }}</QtermButton>
            <QtermButton variant="destructive" :disabled="deleting" @click="confirmDelete">{{
              t('settings.knownHosts.delete')
            }}</QtermButton>
          </div>
        </QtermDialogFooter>
      </QtermDialogContent>
    </QtermDialog>
  </section>
</template>
