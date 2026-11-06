<script setup lang="ts">
import { computed, onMounted, ref, type Component } from 'vue'
import { Check, Minus, Terminal, Upload } from '@lucide/vue'
import {
  ElectermIcon,
  FinalShellIcon,
  MobaXtermIcon,
  SecureCRTIcon,
  TermiusIcon,
  WindTermIcon,
  XShellIcon,
  PuTTYIcon,
  JsonIcon,
  TerminalIcon,
} from '../../components/common/icon'
import { QtermButton, QtermCheckbox, QtermFormMessage, QtermInput, QtermSecretInput } from '../../components/common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { ScrollArea } from '../../components/common/control-primitives'
import { useI18n } from '@/locales'
import { toast } from 'vue-sonner'
import {
  createHostImportGateway,
  hostImportErrorMessage,
  type HostImportSource,
  type SSHConfigEntry,
} from '@/services/hostImport.ts'
import {
  createCredentialGateway,
  type CredentialRecord,
  type CredentialVaultStatus,
  type ResourceBundleSelection,
} from '@/services/credentials.ts'
import { decodeAppError, translateErrorKey } from '@/services/errors.ts'

function isInternalGroupRef(name: string | undefined): boolean {
  const trimmed = (name ?? '').trim()
  if (!trimmed) return false
  if (/^group_\d+$/i.test(trimmed)) return true
  return /^\d+$/.test(trimmed)
}

const props = withDefaults(
  defineProps<{
    open: boolean

    credentials?: CredentialRecord[]

    vaultStatus?: CredentialVaultStatus | null
  }>(),
  { credentials: () => [], vaultStatus: undefined },
)
const emit = defineEmits<{ close: []; imported: [] }>()

const { t } = useI18n()
const gateway = createHostImportGateway()

const step = ref<'sources' | 'preview'>('sources')
const sources = ref<HostImportSource[]>([])
const activeSource = ref<HostImportSource | null>(null)
const busy = ref(false)
const error = ref('')
const backupPassword = ref('')

const backupSelection = ref<ResourceBundleSelection | null>(null)
const backupBusy = ref(false)
const backupInput = ref<HTMLInputElement | null>(null)

const sourceSecret = ref('')

const activePath = ref('')

interface PreviewRow {
  key: string
  checked: boolean
  duplicate: boolean
  name: string
  group: string
  address: string
  port: number
  user: string

  secret: string
  source: SSHConfigEntry
}

const rows = ref<PreviewRow[]>([])
const foundCount = ref(0)

const dialogStyle = computed(() => {
  const width = step.value === 'preview' ? 'min(1180px, calc(100vw - 48px))' : 'min(560px, calc(100vw - 48px))'
  return { width, maxWidth: width }
})

const SOURCE_ICONS: Record<string, Component> = {
  xshell: XShellIcon,
  sshconfig: TerminalIcon,
  putty: PuTTYIcon,
  termius: TermiusIcon,
  securecrt: SecureCRTIcon,
  json: JsonIcon,
  mobaxterm: MobaXtermIcon,
  windterm: WindTermIcon,
  finalshell: FinalShellIcon,
  electerm: ElectermIcon,
  'securecrt-xml': SecureCRTIcon,
}

const SOURCE_HINTS: Record<string, string> = {
  xshell: '.xts',
  mobaxterm: '.mxtsessions',
  windterm: '.sessions',
  securecrt: 'Sessions directory',
  'securecrt-xml': '.xml',
  finalshell: 'conn directory',
  termius: 'local IndexedDB',
  electerm: '.json',
  putty: '.reg',
  json: '.json',
  sshconfig: '~/.ssh/config',
}

function sourceHint(source: HostImportSource): string {
  return (
    SOURCE_HINTS[source.id] ??
    (source.pathKind === 'directory' ? t('shell.importConfig.directory') : source.defaultPath || '—')
  )
}

const SOURCE_LABEL_KEYS: Record<string, string> = {
  sshconfig: 'shell.importConfig.sourceSshConfig',
}

function sourceLabel(source: HostImportSource): string {
  const key = SOURCE_LABEL_KEYS[source.id]
  return key ? t(key) : source.label
}

const selectableRows = computed(() => rows.value.filter((row) => !row.duplicate))

const importableCount = computed(() => rows.value.filter((row) => !row.duplicate).length)
const selectedCount = computed(() => selectableRows.value.filter((row) => row.checked).length)
const allSelected = computed(() => selectableRows.value.length > 0 && selectableRows.value.every((row) => row.checked))

const selectAllState = computed<boolean | 'indeterminate'>(() => {
  if (!selectableRows.value.length) return false
  if (allSelected.value) return true
  return selectedCount.value > 0 ? 'indeterminate' : false
})

onMounted(async () => {
  sources.value = await gateway.listSources()
})

function reset(): void {
  step.value = 'sources'
  activeSource.value = null
  activePath.value = ''
  rows.value = []
  foundCount.value = 0
  error.value = ''
  backupPassword.value = ''
  sourceSecret.value = ''
}

function close(): void {
  if (busy.value) return
  reset()
  emit('close')
}

function backToSources(): void {
  if (busy.value) return
  step.value = 'sources'
  activeSource.value = null
  activePath.value = ''
  rows.value = []
  foundCount.value = 0
  error.value = ''
  sourceSecret.value = ''
}

async function chooseSource(source: HostImportSource): Promise<void> {
  if (busy.value) return
  activeSource.value = source
  activePath.value = ''
  rows.value = []
  foundCount.value = 0
  error.value = ''
  sourceSecret.value = ''

  if (source.defaultPath) {
    await scan(source.defaultPath)
    return
  }
  await reselect()
}

async function reselect(): Promise<void> {
  if (!activeSource.value || busy.value) return
  const picked = await gateway.pickPath(activeSource.value.id)
  if (!picked) return
  await scan(picked)
}

async function scan(path: string): Promise<void> {
  if (!activeSource.value || busy.value) return
  activePath.value = path
  busy.value = true
  error.value = ''
  try {
    const preview = await gateway.preview(activeSource.value.id, path, sourceSecret.value)
    buildRows(preview.entries)
  } catch (cause) {
    error.value = hostImportErrorMessage(cause, t('settings.import.scanFailed'))
    step.value = 'preview'
  } finally {
    busy.value = false
  }
}

function buildRows(entries: SSHConfigEntry[]): void {
  foundCount.value = entries.length
  rows.value = entries.map((entry, index) => ({
    key: `${index}:${entry.hostName}:${entry.port}`,
    checked: !entry.duplicate,
    duplicate: entry.duplicate,
    name: entry.alias,
    group: isInternalGroupRef(entry.group) ? '' : (entry.group ?? ''),
    address: entry.hostName,
    port: entry.port,
    user: entry.user,
    secret: entry.password ?? '',
    source: entry,
  }))
  step.value = 'preview'
  if (!entries.length) error.value = t('settings.import.empty')
}

function toggleAll(): void {
  const next = !allSelected.value
  rows.value = rows.value.map((row) => (row.duplicate ? row : { ...row, checked: next }))
}

async function runImport(): Promise<void> {
  if (busy.value || !selectedCount.value) return
  busy.value = true
  error.value = ''
  try {
    const chosen = rows.value
      .filter((row) => row.checked && !row.duplicate)
      .map((row) => ({
        ...row.source,
        alias: row.name.trim() || row.address,
        hostName: row.address.trim(),
        port: row.port,
        user: row.user.trim(),
        secret: row.secret.trim(),
      }))
    const result = await gateway.importHosts(chosen)
    toast.success(t('settings.import.done', { created: result.created, failed: result.failed }))
    emit('imported')

    busy.value = false
    close()
  } catch (cause) {
    error.value = hostImportErrorMessage(cause, t('settings.import.importFailed'))
  } finally {
    busy.value = false
  }
}

async function restoreBackup(): Promise<void> {
  if (backupBusy.value) return
  if (!backupSelection.value) {
    error.value = t('shell.importConfig.backupFileRequired')
    return
  }
  if (!backupPassword.value.trim()) {
    error.value = t('shell.importConfig.backupPasswordRequired')
    return
  }
  backupBusy.value = true
  error.value = ''
  try {
    const gateway = createCredentialGateway()

    const valid = await gateway.validateResourceBundle(backupSelection.value, backupPassword.value)
    if (!valid) {
      error.value = t('shell.importConfig.backupPasswordInvalid')
      backupPassword.value = ''
      return
    }
    const done = await gateway.importResources(backupPassword.value, backupSelection.value)
    if (!done) {
      error.value = t('shell.importConfig.backupFailed')
      return
    }
    toast.success(t('shell.importConfig.backupRestored'))
    emit('imported')
    close()
  } catch (cause) {
    const decoded = decodeAppError(cause)
    error.value =
      (decoded ? translateErrorKey(decoded.messageKey, decoded.safeDetail ?? '') : '') ||
      t('shell.importConfig.backupFailed')
  } finally {
    backupBusy.value = false
  }
}

function chooseBackupFile(): void {
  if (backupBusy.value) return
  backupInput.value?.click()
}

async function readBackupFile(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  try {
    const bytes = new Uint8Array(await file.arrayBuffer())

    const magic = String.fromCharCode(...bytes.subarray(0, 4))
    if (magic === 'QTRP') {
      backupSelection.value = null
      error.value = t('shell.importConfig.backupWrongType')
      return
    }
    let binary = ''
    for (let offset = 0; offset < bytes.length; offset += 0x8000) {
      binary += String.fromCharCode(...bytes.subarray(offset, offset + 0x8000))
    }
    backupSelection.value = { name: file.name, data: btoa(binary) }
    error.value = ''
  } catch {
    backupSelection.value = null
    error.value = t('shell.importConfig.backupFileFailed')
  }
}
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => !open && close()">
    <QtermDialogContent v-if="props.open" data-import-config-dialog size="3xl" :style="dialogStyle">
      <QtermDialogHeader
        :title="
          activeSource
            ? t('shell.importConfig.fromSource', { name: sourceLabel(activeSource) })
            : t('shell.importConfig.title')
        "
        :description="activeSource ? t('shell.importConfig.pickHint') : t('shell.importConfig.description')"
        :show-close="false"
      />

      <ScrollArea type="hover" :scroll-hide-delay="600" class="import-config-scroll">
        <div class="dialog-body import-config-body flex flex-col gap-3">
          <template v-if="step === 'sources'">
            <div class="flex items-center justify-between gap-2">
              <h3 class="m-0 text-sm font-medium">{{ t('shell.importConfig.sessions') }}</h3>
              <span v-if="busy" class="text-xs text-qt-muted">{{ t('shell.importConfig.scanning') }}</span>
            </div>
            <div class="grid grid-cols-4 gap-1.5" data-import-config-sources>
              <button
                v-for="source in sources"
                :key="source.id"
                type="button"
                class="flex aspect-square flex-col items-center justify-center gap-1 rounded-xl border border-qt-border/70 px-2 py-2 text-center transition-colors hover:bg-qt-hover focus-visible:bg-qt-hover focus-visible:outline-none"
                :data-import-source="source.id"
                @click="chooseSource(source)"
              >
                <component
                  :is="SOURCE_ICONS[source.id] ?? Terminal"
                  class="size-6 shrink-0 text-qt-muted"
                  aria-hidden="true"
                />
                <span class="line-clamp-2 text-[11px] leading-tight font-medium">{{ sourceLabel(source) }}</span>

                <span
                  class="w-full truncate text-center text-[10px] text-qt-muted"
                  :title="source.defaultPath || t('shell.importConfig.directory')"
                >
                  {{ sourceHint(source) }}
                </span>
              </button>
            </div>
            <p class="m-0 text-xs text-qt-muted">{{ t('shell.importConfig.mergeHint') }}</p>

            <div class="flex flex-col gap-2 border-t border-qt-border/60 pt-3" data-import-config-backup>
              <h3 class="m-0 text-sm font-medium">{{ t('shell.importConfig.backupTitle') }}</h3>
              <input
                ref="backupInput"
                type="file"
                accept=".qtrb,.qtp,application/octet-stream"
                class="hidden"
                data-import-config-backup-input
                @change="readBackupFile"
              />
              <button
                type="button"
                class="flex items-center gap-3 rounded-xl border border-qt-border/70 px-4 py-3 text-start transition-colors hover:bg-qt-hover focus-visible:bg-qt-hover focus-visible:outline-none"
                :disabled="backupBusy"
                data-import-config-backup-file
                @click="chooseBackupFile"
              >
                <img
                  src="/qterm-icon.png"
                  alt=""
                  aria-hidden="true"
                  class="size-9 shrink-0 rounded-lg object-contain"
                />
                <span class="flex min-w-0 flex-col">
                  <span class="text-sm font-medium">
                    {{ backupSelection?.name || t('shell.importConfig.backupFileTitle') }}
                  </span>
                  <span class="text-xs text-qt-muted">{{ t('shell.importConfig.backupHint') }}</span>
                </span>
              </button>

              <div v-if="backupSelection" class="flex items-center gap-2">
                <QtermSecretInput
                  id="import-config-backup-password"
                  v-model="backupPassword"
                  class="min-w-0 flex-1"
                  :placeholder="t('shell.importConfig.backupPassword')"
                  autocomplete="off"
                />
                <QtermButton
                  variant="outline"
                  :disabled="backupBusy || !backupPassword.trim()"
                  data-import-config-restore
                  @click="restoreBackup"
                >
                  <Upload :size="14" />{{ t('shell.importConfig.restore') }}
                </QtermButton>
              </div>
            </div>
          </template>

          <template v-else>
            <div
              class="flex flex-wrap items-center justify-between gap-2 text-xs text-qt-muted"
              data-import-config-path
            >
              <span class="min-w-0 truncate" :title="activePath">
                {{ t('shell.importConfig.sourcePath', { path: activePath }) }}
              </span>
              <QtermButton variant="ghost" size="sm" :disabled="busy" data-import-config-pick @click="reselect">
                {{ t('shell.importConfig.chooseFile') }}
              </QtermButton>
              <span class="flex shrink-0 items-center gap-3" data-import-config-stats>
                <span>{{ t('shell.importConfig.found', { count: foundCount }) }}</span>
                <span>{{ t('shell.importConfig.importable', { count: importableCount }) }}</span>
                <span>{{ t('shell.importConfig.selected', { count: selectedCount }) }}</span>
              </span>
            </div>
            <label
              v-if="activeSource?.id === 'windterm'"
              class="flex items-end gap-2 text-xs text-qt-muted"
              data-import-config-windterm-secret
            >
              <span class="flex min-w-0 flex-1 flex-col gap-1">
                {{ t('shell.importConfig.masterPassword') }}
                <QtermSecretInput
                  id="import-config-master-password"
                  v-model="sourceSecret"
                  :placeholder="t('shell.importConfig.masterPasswordHint')"
                  autocomplete="off"
                />
              </span>
              <QtermButton
                variant="outline"
                :disabled="busy || !activePath"
                data-import-config-rescan
                @click="scan(activePath)"
              >
                {{ t('shell.importConfig.rescan') }}
              </QtermButton>
            </label>

            <div class="overflow-hidden rounded-lg border border-qt-border/70" data-import-preview>
              <table class="import-config-table-head w-full min-w-245 table-fixed border-collapse text-xs">
                <colgroup>
                  <col class="w-10" />
                  <col class="w-[21%]" />
                  <col class="w-[15%]" />
                  <col class="w-[21%]" />
                  <col class="w-[7%]" />
                  <col class="w-[11%]" />
                  <col class="w-[18%]" />
                  <col class="w-[7%]" />
                </colgroup>
                <thead class="bg-qt-panel text-qt-muted">
                  <tr>
                    <th class="w-10 py-2 ps-3 pe-1">
                      <QtermCheckbox
                        :model-value="selectAllState"
                        :aria-label="t('settings.import.selectAll')"
                        @update:model-value="toggleAll"
                      >
                        <Minus v-if="selectAllState === 'indeterminate'" :size="14" aria-hidden="true" />
                        <Check v-else :size="14" aria-hidden="true" />
                      </QtermCheckbox>
                    </th>
                    <th class="px-2 py-2 text-start font-medium">{{ t('shell.importConfig.columnName') }}</th>
                    <th class="px-2 py-2 text-start font-medium">{{ t('shell.importConfig.columnGroup') }}</th>
                    <th class="px-2 py-2 text-start font-medium">{{ t('shell.importConfig.columnAddress') }}</th>
                    <th class="px-2 py-2 text-start font-medium">{{ t('shell.importConfig.columnPort') }}</th>
                    <th class="px-2 py-2 text-start font-medium">{{ t('shell.importConfig.columnUser') }}</th>
                    <th class="px-2 py-2 text-start font-medium">{{ t('shell.importConfig.columnAuth') }}</th>
                    <th class="py-2 ps-2 pe-3 text-start font-medium">{{ t('shell.importConfig.columnStatus') }}</th>
                  </tr>
                </thead>
              </table>
              <ScrollArea
                type="hover"
                :scroll-hide-delay="600"
                class="import-config-table h-[min(560px,calc(100vh-380px))] min-h-[200px] overflow-auto border-t border-qt-border/60"
              >
                <table class="w-full min-w-245 table-fixed border-collapse text-xs" data-import-preview-table>
                  <colgroup>
                    <col class="w-10" />
                    <col class="w-[21%]" />
                    <col class="w-[15%]" />
                    <col class="w-[21%]" />
                    <col class="w-[7%]" />
                    <col class="w-[11%]" />
                    <col class="w-[18%]" />
                    <col class="w-[7%]" />
                  </colgroup>
                  <tbody>
                    <tr
                      v-for="row in rows"
                      :key="row.key"
                      class="border-b border-qt-border/40 last:border-b-0"
                      :class="row.duplicate ? 'opacity-45' : ''"
                      :data-import-row="row.key"
                    >
                      <td class="py-1.5 ps-3 pe-1">
                        <QtermCheckbox
                          :model-value="row.checked"
                          :disabled="row.duplicate"
                          :aria-label="row.name"
                          @update:model-value="row.checked = Boolean($event)"
                        />
                      </td>
                      <td class="px-2 py-1.5"><QtermInput v-model="row.name" class="h-8 w-full min-w-0 text-xs" /></td>
                      <td class="px-2 py-1.5">
                        <QtermInput
                          v-model="row.group"
                          class="h-8 text-xs"
                          :placeholder="t('shell.importConfig.groupPlaceholder')"
                        />
                      </td>
                      <td class="px-2 py-1.5">
                        <QtermInput v-model="row.address" class="h-8 w-full min-w-0 text-xs" />
                      </td>
                      <td class="px-2 py-1.5">
                        <QtermInput v-model.number="row.port" inputmode="numeric" class="h-8 w-full min-w-0 text-xs" />
                      </td>
                      <td class="px-2 py-1.5"><QtermInput v-model="row.user" class="h-8 w-full min-w-0 text-xs" /></td>

                      <td class="px-2 py-1.5">
                        <QtermSecretInput
                          :id="`import-secret-${row.key}`"
                          v-model="row.secret"
                          class="h-8 w-full min-w-0 text-xs"
                          input-class="h-8 text-xs"
                          autocomplete="off"
                          :placeholder="t('shell.importConfig.authPlaceholder')"
                          :aria-label="t('shell.importConfig.columnAuth')"
                        />
                      </td>
                      <td class="py-1.5 ps-2 pe-3">
                        <span
                          class="inline-flex items-center rounded-full border border-qt-border/70 px-2 py-0.5 text-[10px] text-qt-muted"
                          >{{
                            row.duplicate ? t('shell.importConfig.statusExisting') : t('shell.importConfig.statusNew')
                          }}</span
                        >
                      </td>
                    </tr>
                  </tbody>
                </table>
              </ScrollArea>
            </div>
          </template>

          <QtermFormMessage v-if="error" variant="error">{{ error }}</QtermFormMessage>
        </div>
      </ScrollArea>
      <QtermDialogFooter>
        <span class="text-[11px] text-qt-muted">{{ t('shell.importConfig.mergeHint') }}</span>
        <div class="dialog-footer-actions flex items-center gap-[8px]">
          <QtermButton variant="outline" :disabled="busy" @click="step === 'sources' ? close() : backToSources()">
            {{ step === 'sources' ? t('shell.importConfig.cancel') : t('shell.importConfig.back') }}
          </QtermButton>
          <QtermButton
            v-if="step === 'preview'"
            data-import-config-start
            :disabled="busy || !selectedCount"
            @click="runImport"
          >
            {{ t('shell.importConfig.importSelected', { count: selectedCount }) }}
          </QtermButton>
        </div>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>

<style scoped>
.import-config-scroll :deep([data-slot='scroll-area-viewport']) {
  max-height: calc(100vh - 260px);
  min-height: 0;
}

.import-config-scroll :deep(.dialog-body) {
  flex: none;
  overflow: visible;
}

.import-config-scroll :deep([data-slot='scroll-area-viewport'] > div) {
  display: block !important;
  width: 100%;
  min-width: 0;
}

.import-config-th {
  position: sticky;
  top: 0;
  z-index: 2;
  background-color: var(--qt-panel, var(--background));
  box-shadow: inset 0 -1px 0 var(--qt-border);
}

.import-config-table :deep([data-slot='scroll-area-viewport'] > div) {
  display: block !important;
  width: 100%;
  min-width: 0;
}
</style>
