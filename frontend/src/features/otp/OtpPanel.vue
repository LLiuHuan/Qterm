<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import { writeClipboardText } from '@/services/clipboard'
import { Copy, KeyRound, Pencil, Plus, QrCode, ScanLine, Trash2 } from '@lucide/vue'
import {
  QtermButton,
  QtermInput,
  QtermSecretInput,
  QtermNumberField,
  SegmentedTabs,
  QtermEmptyState,
} from '@/components/common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '@/components/common/dialog'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from '@/components/common/overlay-controls'
import { useI18n } from '@/locales'
import type { HostRecord } from '@/services/hosts'
import { createOtpGateway, type OtpAlgorithm, type OtpCode, type OtpEntry, type OtpType } from '@/services/otp'
import { decodeOtpQrImage, parseOtpProvisioningUri } from '@/services/otpQr'
import qrcode from '@/vendor/qrcode-generator'

const props = withDefaults(
  defineProps<{
    entries: OtpEntry[]
    hosts: HostRecord[]
    activeHostId?: string
    mode?: 'manage' | 'compact'
  }>(),
  { activeHostId: '', mode: 'manage' },
)

const emit = defineEmits<{ 'update:entries': [value: OtpEntry[]]; manage: [] }>()
const { t } = useI18n()
const gateway = createOtpGateway()

const codes = ref<Record<string, OtpCode>>({})
const dialogOpen = ref(false)
const editingId = ref('')
const draft = ref<OtpEntry>(emptyDraft())
const draftSecret = ref('')
const draftError = ref('')
const saving = ref(false)
const qrEntry = ref<OtpEntry | null>(null)
const qrSvg = ref('')
const qrError = ref('')
const qrFileInput = ref<HTMLInputElement | null>(null)
const qrImportError = ref('')
const codeErrors = ref<Record<string, string>>({})
const codeDeadlines = ref<Record<string, number>>({})
const actionError = ref('')

const query = ref('')
const visibleEntries = computed(() =>
  props.entries.filter((entry) =>
    [entry.name, entry.username ?? ''].join(' ').toLowerCase().includes(query.value.trim().toLowerCase()),
  ),
)
let qrImportEpoch = 0
let qrExportEpoch = 0
let disposed = false
const pendingCodes = new Set<string>()
const advancing = ref<Record<string, boolean>>({})

const typeOptions = computed(() => [
  { value: 'totp', label: 'TOTP' },
  { value: 'hotp', label: 'HOTP' },
])
const algorithmOptions = computed(() => [
  { value: 'SHA1', label: 'SHA1' },
  { value: 'SHA256', label: 'SHA256' },
  { value: 'SHA512', label: 'SHA512' },
])
function emptyDraft(): OtpEntry {
  return {
    id: '',
    name: '',
    username: '',
    hostId: undefined,
    type: 'totp',
    algorithm: 'SHA1',
    digits: 6,
    period: 30,
    counter: 0,
  }
}

function errorMessage(error: unknown): string {
  const key = error instanceof Error ? error.message : String(error)
  if (key === 'otp_qr_image_required') return t('shell.otp.scanImageRequired')
  if (key === 'otp_qr_image_too_large') return t('shell.otp.scanImageTooLarge')
  if (key === 'otp_qr_not_found') return t('shell.otp.scanNotFound')
  if (key === 'invalid_otp_uri') return t('shell.otp.scanInvalid')
  return t('shell.otp.scanFailed')
}

function setCodeError(id: string, message: string): void {
  codeErrors.value = { ...codeErrors.value, [id]: message }
}

function clearCodeError(id: string): void {
  if (!(id in codeErrors.value)) return
  const next = { ...codeErrors.value }
  delete next[id]
  codeErrors.value = next
}

async function refreshCode(entry: OtpEntry, force = false): Promise<void> {
  if (disposed || pendingCodes.has(entry.id)) return
  const current = codes.value[entry.id]
  if (!force && entry.type === 'totp' && current && (codeDeadlines.value[entry.id] ?? 0) > Date.now()) return
  pendingCodes.add(entry.id)
  const original = JSON.stringify(entry)
  const isCurrent = () => !disposed && JSON.stringify(props.entries.find((item) => item.id === entry.id)) === original
  try {
    const code = await gateway.generate(entry)
    if (!isCurrent()) return
    codes.value = { ...codes.value, [entry.id]: code }
    codeDeadlines.value = { ...codeDeadlines.value, [entry.id]: Date.now() + code.remainingSeconds * 1000 }
    clearCodeError(entry.id)
  } catch {
    if (!isCurrent()) return

    delete codes.value[entry.id]
    setCodeError(entry.id, t('shell.otp.codeError'))
  } finally {
    pendingCodes.delete(entry.id)
    const latest = props.entries.find((item) => item.id === entry.id)
    if (!disposed && latest && JSON.stringify(latest) !== original) void refreshCode(latest, true)
  }
}

function tick(): void {
  const next = { ...codes.value }
  let changed = false
  for (const entry of props.entries) {
    if (entry.type !== 'totp') continue
    const current = next[entry.id]
    if (!current) continue
    const remaining = Math.max(0, Math.ceil(((codeDeadlines.value[entry.id] ?? 0) - Date.now()) / 1000))
    if (remaining <= 0) {
      void refreshCode(entry, true)
      continue
    }
    next[entry.id] = { ...current, remainingSeconds: remaining }
    changed = true
  }
  if (changed) codes.value = next
}

async function refreshAll(): Promise<void> {
  await Promise.all(props.entries.map((entry) => refreshCode(entry, true)))
}

async function copy(entry: OtpEntry): Promise<void> {
  if (entry.type === 'totp') await refreshCode(entry, true)
  if (disposed || codeErrors.value[entry.id] || pendingCodes.has(entry.id)) return
  if (entry.type === 'totp' && (codeDeadlines.value[entry.id] ?? 0) <= Date.now()) return
  const code = codes.value[entry.id]?.code
  if (!code) return
  try {
    await writeClipboardText(code)
    if (disposed) return
    actionError.value = ''
    toast.success(t('shell.otp.copied'))
  } catch {
    if (disposed) return
    actionError.value = t('shell.app.copyFailed')
    toast.error(actionError.value)
  }
}

function progressPercent(entry: OtpEntry): number {
  if (entry.type !== 'totp') return 0
  const remaining = codes.value[entry.id]?.remainingSeconds ?? 0
  return Math.min(100, Math.max(0, (remaining / Math.max(1, entry.period)) * 100))
}

async function advanceCounter(entry: OtpEntry): Promise<void> {
  if (advancing.value[entry.id] || entry.counter >= 999999999) return
  advancing.value[entry.id] = true
  const nextCounter = Math.min(999999999, entry.counter + 1)
  const nextEntry: OtpEntry = { ...entry, counter: nextCounter }
  const original = JSON.stringify(entry)
  try {
    const code = await gateway.generate(nextEntry)

    if (disposed || JSON.stringify(props.entries.find((item) => item.id === entry.id)) !== original) return
    codes.value = { ...codes.value, [entry.id]: code }
    codeDeadlines.value = { ...codeDeadlines.value, [entry.id]: Date.now() + code.remainingSeconds * 1000 }
    clearCodeError(entry.id)
    emit(
      'update:entries',
      props.entries.map((item) => (item.id === entry.id ? nextEntry : item)),
    )
  } catch {
    setCodeError(entry.id, t('shell.otp.codeError'))
  } finally {
    advancing.value[entry.id] = false
  }
}

async function openQr(entry: OtpEntry): Promise<void> {
  const epoch = ++qrExportEpoch
  qrEntry.value = entry
  qrSvg.value = ''
  qrError.value = ''
  try {
    const uri = await gateway.provisioningUri(entry)
    if (disposed || epoch !== qrExportEpoch) return
    const qr = qrcode(0, 'M')
    qr.addData(uri)
    qr.make()

    qrSvg.value = qr.createSvgTag({ cellSize: 4, margin: 2, scalable: true })
  } catch {
    if (!disposed && epoch === qrExportEpoch) qrError.value = t('shell.otp.generateError')
  }
}

function closeQr(): void {
  qrExportEpoch++
  qrEntry.value = null
  qrSvg.value = ''
  qrError.value = ''
}

function applyImportedUri(uri: string): void {
  const parsed = parseOtpProvisioningUri(uri)
  editingId.value = ''
  draft.value = {
    ...emptyDraft(),
    name: parsed.name,
    username: parsed.username,
    type: parsed.type,
    algorithm: parsed.algorithm,
    digits: parsed.digits,
    period: parsed.period,
    counter: parsed.counter,
  }
  draftSecret.value = parsed.secret
  draftError.value = ''
  qrImportError.value = ''
  dialogOpen.value = true
}

async function importQrImage(file: Blob | null): Promise<void> {
  if (!file) return
  const epoch = ++qrImportEpoch
  qrImportError.value = ''
  try {
    const uri = await decodeOtpQrImage(file)
    if (disposed || epoch !== qrImportEpoch) return
    applyImportedUri(uri)
  } catch (error) {
    if (!disposed && epoch === qrImportEpoch) qrImportError.value = errorMessage(error)
  }
}

function openQrImport(): void {
  qrFileInput.value?.click()
}

function onQrFileChange(event: Event): void {
  const input = event.target as HTMLInputElement
  void importQrImage(input.files?.[0] ?? null)
  input.value = ''
}

function onQrPaste(event: ClipboardEvent): void {
  const image = Array.from(event.clipboardData?.items ?? []).find((item) => item.type.startsWith('image/'))
  if (!image) return
  event.preventDefault()
  void importQrImage(image.getAsFile())
}

function openCreate(): void {
  qrImportEpoch++
  editingId.value = ''
  draft.value = emptyDraft()
  draftSecret.value = ''
  draftError.value = ''
  dialogOpen.value = true
}

function openEdit(entry: OtpEntry): void {
  qrImportEpoch++
  editingId.value = entry.id
  draft.value = { ...entry }
  draftSecret.value = ''
  draftError.value = ''
  dialogOpen.value = true
}

async function submit(): Promise<void> {
  if (!editingId.value && !draftSecret.value.trim()) {
    draftError.value = t('shell.otp.errorSecret')
    return
  }
  saving.value = true
  try {
    const saved = await gateway.save(draft.value, draftSecret.value.trim())
    const exists = props.entries.some((item) => item.id === saved.id)
    emit(
      'update:entries',
      exists ? props.entries.map((item) => (item.id === saved.id ? saved : item)) : [...props.entries, saved],
    )
    dialogOpen.value = false
    await refreshCode(saved, true)
  } catch (error) {
    draftError.value = error instanceof Error ? error.message : String(error)
  } finally {
    saving.value = false
  }
}

async function remove(entry: OtpEntry): Promise<void> {
  actionError.value = ''
  try {
    await gateway.remove(entry.id)
    emit(
      'update:entries',
      props.entries.filter((item) => item.id !== entry.id),
    )
  } catch {
    actionError.value = t('shell.otp.removeFailed')
  }
}

watch(dialogOpen, (open) => {
  if (!open) {
    qrImportEpoch++
    draftSecret.value = ''
  }
})
watch(
  () => props.entries,
  () => {
    void refreshAll()
  },
)
let timer: number | undefined
onMounted(() => {
  void refreshAll()
  timer = window.setInterval(tick, 1000)
})
onBeforeUnmount(() => {
  disposed = true
  qrImportEpoch++
  qrExportEpoch++
  draftSecret.value = ''
  qrSvg.value = ''
  if (timer !== undefined) window.clearInterval(timer)
  timer = undefined
})
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <div class="flex items-center justify-between gap-2 px-2 pt-2 pb-2">
      <QtermInput
        v-model="query"
        class="min-w-0 flex-1"
        :placeholder="t('shell.otp.search')"
        :aria-label="t('shell.otp.search')"
      />
      <div class="flex shrink-0 items-center gap-2">
        <QtermButton v-if="props.mode === 'manage'" size="sm" variant="outline" type="button" @click="openQrImport">
          <ScanLine :size="14" aria-hidden="true" />
          {{ t('shell.otp.scan') }}
        </QtermButton>
        <QtermButton v-if="props.mode === 'manage'" size="sm" variant="outline" type="button" @click="openCreate">
          <Plus :size="14" aria-hidden="true" />
          {{ t('shell.otp.add') }}
        </QtermButton>
        <QtermButton v-else size="sm" variant="ghost" type="button" @click="emit('manage')">
          {{ t('shell.otp.manage') }}
        </QtermButton>
      </div>
      <input ref="qrFileInput" class="hidden" type="file" accept="image/*" @change="onQrFileChange" />
    </div>

    <div v-if="visibleEntries.length === 0" class="flex-1">
      <QtermEmptyState :icon="KeyRound" :title="t('shell.otp.empty')" :description="t('shell.otp.emptyHint')" />
    </div>

    <div v-else class="min-h-0 flex-1 overflow-y-auto px-2 pb-2">
      <div class="flex flex-col rounded-[8px] border border-qt-border p-1">
        <ContextMenu v-for="entry in visibleEntries" :key="entry.id">
          <ContextMenuTrigger as-child>
            <div
              class="flex flex-wrap items-center gap-3 rounded-md px-1.5 py-2 text-xs transition-colors [&:not(:first-child)]:mt-0.5 hover:bg-qt-hover"
              tabindex="0"
            >
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2">
                  <span class="min-w-0 truncate text-sm">{{ entry.name || t('shell.otp.untitled') }}</span>
                  <span class="rounded border border-qt-border px-1 text-[10px] text-qt-muted">{{
                    entry.type.toUpperCase()
                  }}</span>
                </div>
                <div class="mt-0.5 truncate text-[11px] text-qt-muted">
                  <span v-if="entry.username">{{ entry.username }} · </span>
                  <template v-if="props.mode === 'compact'">
                    <span v-if="entry.type === 'totp'">{{ entry.period }}s</span>
                  </template>
                  <template v-else>
                    <span>{{ entry.algorithm }} · {{ entry.digits }} {{ t('shell.otp.digitsUnit') }}</span>
                    <span v-if="entry.type === 'totp'"> · {{ entry.period }}s</span>
                  </template>
                </div>
              </div>
              <div class="flex shrink-0 items-center gap-2">
                <div class="flex min-w-[4.5rem] flex-col items-end leading-tight">
                  <span class="font-mono text-base tracking-wider tabular-nums">
                    {{ codes[entry.id]?.code ?? '······' }}
                  </span>

                  <span v-if="entry.type === 'totp'" class="text-[11px] text-qt-muted tabular-nums">
                    {{ t('shell.otp.remainingSeconds', { seconds: codes[entry.id]?.remainingSeconds ?? 0 }) }}
                  </span>
                  <span v-else class="text-[11px] text-qt-muted tabular-nums">
                    {{ t('shell.otp.counterValue', { count: codes[entry.id]?.counter ?? entry.counter }) }}
                  </span>
                </div>
                <QtermButton
                  size="icon-sm"
                  variant="ghost"
                  type="button"
                  :aria-label="t('shell.otp.copy')"
                  :title="t('shell.otp.copy')"
                  @click.stop="copy(entry)"
                >
                  <Copy :size="14" aria-hidden="true" />
                </QtermButton>
                <QtermButton
                  v-if="entry.type !== 'totp'"
                  :disabled="advancing[entry.id] || entry.counter >= 999999999"
                  size="sm"
                  variant="outline"
                  type="button"
                  @click="advanceCounter(entry)"
                >
                  {{ t('shell.otp.next') }}
                </QtermButton>
              </div>
              <div
                v-if="entry.type === 'totp'"
                class="basis-full overflow-hidden rounded-full bg-qt-muted/20"
                role="progressbar"
                :aria-label="t('shell.otp.remainingSeconds', { seconds: codes[entry.id]?.remainingSeconds ?? 0 })"
                aria-valuemin="0"
                :aria-valuemax="entry.period"
                :aria-valuenow="codes[entry.id]?.remainingSeconds ?? 0"
              >
                <span
                  class="block h-1 rounded-full bg-qt-accent transition-[width] duration-1000 ease-linear"
                  :style="{ width: `${progressPercent(entry)}%` }"
                />
              </div>
              <p v-if="codeErrors[entry.id]" class="basis-full text-[11px] text-qt-danger" role="alert">
                {{ codeErrors[entry.id] }}
              </p>
            </div>
          </ContextMenuTrigger>
          <ContextMenuContent class="min-w-40">
            <ContextMenuItem @select="copy(entry)">
              <Copy :size="14" aria-hidden="true" />{{ t('shell.otp.copy') }}
            </ContextMenuItem>
            <ContextMenuItem v-if="props.mode === 'manage'" @select="openQr(entry)">
              <QrCode :size="14" aria-hidden="true" />{{ t('shell.otp.qr') }}
            </ContextMenuItem>
            <ContextMenuItem v-if="props.mode === 'manage'" @select="openEdit(entry)">
              <Pencil :size="14" aria-hidden="true" />{{ t('shell.otp.edit') }}
            </ContextMenuItem>
            <ContextMenuSeparator v-if="props.mode === 'manage'" />
            <ContextMenuItem v-if="props.mode === 'manage'" variant="destructive" @select="remove(entry)">
              <Trash2 :size="14" aria-hidden="true" />{{ t('shell.otp.remove') }}
            </ContextMenuItem>
          </ContextMenuContent>
        </ContextMenu>
      </div>
    </div>
    <p v-if="actionError" class="m-0 px-2 pb-2 text-[11px] text-qt-danger" role="alert">{{ actionError }}</p>
    <p v-if="qrImportError" class="m-0 px-2 pb-2 text-[11px] text-qt-danger" role="alert">{{ qrImportError }}</p>

    <QtermDialog :open="qrEntry !== null" @update:open="(open) => !open && closeQr()">
      <QtermDialogContent v-if="qrEntry">
        <QtermDialogHeader :title="t('shell.otp.qrTitle')" @close="closeQr" />
        <div class="flex min-h-0 flex-col items-center gap-3 overflow-auto p-4">
          <!-- eslint-disable-next-line vue/no-v-html -->
          <div v-if="qrSvg" class="otp-qr" v-html="qrSvg"></div>
          <p v-else-if="qrError" class="m-0 text-xs text-qt-danger" role="alert">{{ qrError }}</p>
          <p class="m-0 text-center text-xs text-qt-muted">{{ t('shell.otp.qrHint') }}</p>
        </div>
        <QtermDialogFooter>
          <QtermButton variant="outline" type="button" @click="closeQr">
            {{ t('shell.common.cancel') }}
          </QtermButton>
        </QtermDialogFooter>
      </QtermDialogContent>
    </QtermDialog>

    <QtermDialog :open="dialogOpen" @update:open="(open) => (dialogOpen = open)">
      <QtermDialogContent v-if="dialogOpen" @paste="onQrPaste">
        <QtermDialogHeader
          :title="editingId ? t('shell.otp.editTitle') : t('shell.otp.createTitle')"
          :description="t('shell.otp.scanHint')"
          :close-disabled="saving"
          @close="dialogOpen = false"
        />
        <div class="dialog-body flex-col">
          <div class="grid gap-3 sm:grid-cols-2">
            <div class="flex min-w-0 flex-col gap-1.5">
              <span class="text-xs text-qt-muted">{{ t('shell.otp.name') }}</span>
              <QtermInput
                v-model="draft.name"
                class="text-xs"
                :aria-label="t('shell.otp.name')"
                placeholder="Github"
                :disabled="saving"
              />
            </div>
            <div class="flex min-w-0 flex-col gap-1.5">
              <span class="text-xs text-qt-muted">{{ t('shell.otp.username') }}</span>
              <QtermInput
                v-model="draft.username"
                class="text-xs"
                :aria-label="t('shell.otp.username')"
                placeholder="user@example.com"
                spellcheck="false"
                autocomplete="off"
                :disabled="saving"
              />
            </div>
          </div>
          <div class="flex flex-col gap-1.5">
            <span class="text-xs text-qt-muted">{{ t('shell.otp.secret') }}</span>
            <QtermSecretInput
              id="otp-secret"
              v-model="draftSecret"
              class="font-mono text-xs"
              :aria-label="t('shell.otp.secret')"
              :placeholder="t('shell.otp.secretPlaceholder')"
              spellcheck="false"
              autocomplete="off"
              :disabled="saving"
            />
            <span class="text-[11px] text-qt-muted">{{ t('shell.otp.secretHint') }}</span>
          </div>
          <div class="flex flex-col gap-1.5">
            <span class="text-xs text-qt-muted">{{ t('shell.otp.type') }}</span>
            <SegmentedTabs
              :model-value="draft.type"
              :options="typeOptions"
              :label="t('shell.otp.type')"
              @update:model-value="(value) => (draft = { ...draft, type: value as OtpType })"
            />
          </div>

          <div class="otp-parameter-row otp-algorithm-row flex w-full min-w-0 flex-col gap-1.5">
            <span class="text-xs text-qt-muted">{{ t('shell.otp.algorithm') }}</span>
            <SegmentedTabs
              class="w-full"
              :model-value="draft.algorithm"
              :options="algorithmOptions"
              :label="t('shell.otp.algorithm')"
              @update:model-value="(value) => (draft = { ...draft, algorithm: value as OtpAlgorithm })"
            />
          </div>
          <div class="grid w-full min-w-0 grid-cols-2 gap-3">
            <div class="flex min-w-0 flex-col gap-1.5">
              <span class="text-xs text-qt-muted">{{ t('shell.otp.digits') }}</span>
              <QtermNumberField
                :model-value="draft.digits"
                :min="4"
                :max="10"
                :step="1"
                :aria-label="t('shell.otp.digits')"
                :decrease-label="t('shell.otp.digits')"
                :increase-label="t('shell.otp.digits')"
                @update:model-value="(value) => (draft = { ...draft, digits: value ?? 6 })"
              />
            </div>

            <div v-if="draft.type === 'totp'" class="flex min-w-0 flex-col gap-1.5">
              <span class="text-xs text-qt-muted">{{ t('shell.otp.period') }}</span>
              <QtermNumberField
                :model-value="draft.period"
                :min="10"
                :max="300"
                :step="1"
                :aria-label="t('shell.otp.period')"
                :decrease-label="t('shell.otp.period')"
                :increase-label="t('shell.otp.period')"
                @update:model-value="(value) => (draft = { ...draft, period: value ?? 30 })"
              />
            </div>
            <div v-else class="flex min-w-0 flex-col gap-1.5">
              <span class="text-xs text-qt-muted">{{ t('shell.otp.counter') }}</span>
              <QtermNumberField
                :model-value="draft.counter"
                :min="0"
                :max="999999999"
                :step="1"
                :aria-label="t('shell.otp.counter')"
                :decrease-label="t('shell.otp.counter')"
                :increase-label="t('shell.otp.counter')"
                @update:model-value="(value) => (draft = { ...draft, counter: value ?? 0 })"
              />
            </div>
          </div>
          <p v-if="draftError" class="m-0 text-xs text-qt-danger" role="alert">{{ draftError }}</p>
        </div>
        <QtermDialogFooter>
          <QtermButton variant="outline" type="button" :disabled="saving" @click="dialogOpen = false">
            {{ t('shell.common.cancel') }}
          </QtermButton>
          <QtermButton type="button" :disabled="saving" @click="submit">{{ t('shell.common.confirm') }}</QtermButton>
        </QtermDialogFooter>
      </QtermDialogContent>
    </QtermDialog>
  </div>
</template>

<style scoped>
.otp-algorithm-row :deep(.qterm-segmented-tabs),
.otp-algorithm-row :deep(.qterm-segmented-tabs__list) {
  width: 100%;
}

.otp-qr {
  width: 220px;
  height: 220px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #fff;
  border-radius: 6px;
  padding: 6px;
}

.otp-qr :deep(svg) {
  width: 100%;
  height: 100%;
}
</style>
