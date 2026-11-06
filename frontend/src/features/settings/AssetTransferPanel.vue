<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { FileArchive, FolderOpen } from '@lucide/vue'
import { toast } from 'vue-sonner'
import { QtermButton, QtermFormMessage, QtermInput } from '../../components/common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { Label } from '../../components/common/control-primitives'
import { ExportIcon, ImportIcon } from '@/components/common/icon'
import ExportConfigDialog from '../../components/dialogs/ExportConfigDialog.vue'
import type { CredentialGateway, ResourceBundleSelection } from '@/services/credentials.ts'
import { useI18n } from '@/locales'

const props = defineProps<{
  gateway: CredentialGateway
  disabled?: boolean
  label?: string
  description?: string
}>()
const emit = defineEmits<{ imported: [] }>()

const { t } = useI18n()
const MAX_RESOURCE_BUNDLE_BYTES = 64 * 1024 * 1024

type ImportStep = 'select' | 'password'

const exportOpen = ref(false)
const importOpen = ref(false)
const confirmImportOpen = ref(false)
const importStep = ref<ImportStep>('select')
const importPassword = ref('')
const importSelection = ref<ResourceBundleSelection | null>(null)
const importFileInput = ref<HTMLInputElement | null>(null)
const error = ref('')
const busy = ref(false)
const readingFile = ref(false)

const BASE64_CHUNK_SIZE = 0x8000

function reset(): void {
  importPassword.value = ''
  importSelection.value = null
  importStep.value = 'select'
  error.value = ''
  exportOpen.value = false
  importOpen.value = false
  confirmImportOpen.value = false
}

function cancelImport(): void {
  if (busy.value) return
  reset()
}

function openExport(): void {
  if (props.disabled || busy.value) return
  reset()
  exportOpen.value = true
}

function openImport(): void {
  if (props.disabled || busy.value) return
  reset()
  importOpen.value = true
}

function chooseImportFile(): void {
  if (busy.value || readingFile.value) return
  importFileInput.value?.click()
}

async function readImportFile(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (file.size > MAX_RESOURCE_BUNDLE_BYTES) {
    error.value = t('asset.import.fileTooLarge')
    return
  }
  readingFile.value = true
  error.value = ''
  try {
    const data = encodeBase64(new Uint8Array(await file.arrayBuffer()))
    importSelection.value = { name: file.name, data }
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : t('asset.import.failure')
    importSelection.value = null
  } finally {
    readingFile.value = false
  }
}

function encodeBase64(bytes: Uint8Array): string {
  let binary = ''
  for (let offset = 0; offset < bytes.length; offset += BASE64_CHUNK_SIZE) {
    binary += String.fromCharCode(...bytes.subarray(offset, offset + BASE64_CHUNK_SIZE))
  }
  return btoa(binary)
}

function validatePassword(password: string): boolean {
  if (password.length < 8) {
    error.value = t('asset.password.tooShort')
    return false
  }
  if (password.length > 4096) {
    error.value = t('asset.password.tooLong')
    return false
  }
  return true
}

function continueImport(): void {
  error.value = ''
  if (!importSelection.value) {
    error.value = t('asset.import.fileRequired')
    return
  }
  importStep.value = 'password'
}

function backToSelection(): void {
  if (busy.value) return
  importPassword.value = ''
  error.value = ''
  importStep.value = 'select'
}

function handleImportFailure(cause: unknown, fallbackKey = 'asset.import.failure'): void {
  error.value = errorText(cause, fallbackKey)
  confirmImportOpen.value = false
  importOpen.value = true
  importStep.value = 'password'

  importPassword.value = ''
}

async function requestImport(): Promise<void> {
  error.value = ''
  if (!importSelection.value) {
    importStep.value = 'select'
    error.value = t('asset.import.fileRequired')
    return
  }
  if (!validatePassword(importPassword.value)) return
  busy.value = true
  try {
    const valid = await props.gateway.validateResourceBundle(importSelection.value, importPassword.value)
    if (!valid) throw new Error('asset.import.passwordInvalid')
    importOpen.value = false
    confirmImportOpen.value = true
  } catch (cause) {
    handleImportFailure(cause)
  } finally {
    busy.value = false
  }
}

async function importAssets(): Promise<void> {
  const selection = importSelection.value
  if (!selection) {
    confirmImportOpen.value = false
    importOpen.value = true
    importStep.value = 'select'
    importPassword.value = ''
    error.value = t('asset.import.fileRequired')
    return
  }
  error.value = ''
  busy.value = true
  try {
    const imported = await props.gateway.importResources(importPassword.value, selection)
    if (imported) {
      toast.success(t('asset.import.success'))
      emit('imported')
      reset()
    } else {
      handleImportFailure(new Error('asset.import.failure'))
    }
  } catch (cause) {
    handleImportFailure(cause)
  } finally {
    busy.value = false
  }
}

function errorText(cause: unknown, fallbackKey: string): string {
  const message = cause instanceof Error ? cause.message : ''
  return message && message.includes('.') ? t(message) : message || t(fallbackKey)
}

onBeforeUnmount(reset)
</script>

<template>
  <section class="security-asset-transfer" :aria-label="props.label || t('asset.transfer.label')">
    <input
      ref="importFileInput"
      class="sr-only"
      type="file"
      accept=".qtrb,application/octet-stream"
      @change="readImportFile"
    />

    <div class="security-status-card justify-between">
      <span>
        <strong class="text-sm">{{ props.label || t('asset.transfer.label') }}</strong>
        <small class="text-xs">{{ props.description || t('asset.transfer.description') }}</small>
      </span>
      <div class="security-inline-actions pb-px">
        <QtermButton variant="outline" type="button" :disabled="props.disabled || busy" @click="openExport">
          <ExportIcon :size="14" aria-hidden="true" />
          {{ t('asset.export.button') }}
        </QtermButton>
        <QtermButton variant="outline" type="button" :disabled="props.disabled || busy" @click="openImport">
          <ImportIcon :size="14" aria-hidden="true" />
          {{ t('asset.import.button') }}
        </QtermButton>
      </div>
    </div>

    <ExportConfigDialog
      :open="exportOpen"
      :gateway="props.gateway"
      @close="exportOpen = false"
      @exported="emit('imported')"
    />

    <QtermDialog :open="importOpen" @update:open="(open) => !open && cancelImport()">
      <QtermDialogContent size="md">
        <QtermDialogHeader
          :title="importStep === 'select' ? t('asset.import.selectTitle') : t('asset.import.passwordTitle')"
          :description="
            importStep === 'select' ? t('asset.import.selectDescription') : t('asset.import.passwordDescription')
          "
          :close-disabled="busy"
          @close="reset"
        />
        <form
          class="dialog-body security-form"
          @submit.prevent="importStep === 'select' ? continueImport() : requestImport()"
        >
          <template v-if="importStep === 'select'">
            <div class="flex items-center gap-2.5 rounded-lg border-d bg-qt-subtle px-3 py-2.5">
              <FileArchive
                :size="20"
                aria-hidden="true"
                class="shrink-0"
                :class="importSelection ? 'text-qt-text' : 'text-qt-muted'"
              />
              <span class="flex min-w-0 flex-1 flex-col gap-0.5">
                <strong class="truncate text-sm">
                  {{
                    importSelection
                      ? t('asset.import.selectedFile', { name: importSelection.name })
                      : t('asset.import.noFile')
                  }}
                </strong>
                <small class="text-xs text-qt-muted">{{ t('asset.import.fileHint') }}</small>
              </span>
              <QtermButton
                type="button"
                variant="outline"
                class="shrink-0"
                :disabled="readingFile"
                @click="chooseImportFile"
              >
                <FolderOpen :size="14" aria-hidden="true" />
                {{ readingFile ? t('asset.import.reading') : t('asset.import.selectButton') }}
              </QtermButton>
            </div>
          </template>
          <template v-else>
            <QtermFormMessage variant="hint">
              {{ t('asset.import.selectedFile', { name: importSelection?.name ?? '' }) }}
            </QtermFormMessage>
            <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
              <Label for="asset-import-password">{{ t('asset.import.password') }}</Label>
              <QtermInput
                id="asset-import-password"
                v-model="importPassword"
                type="password"
                autocomplete="current-password"
                :placeholder="t('asset.import.passwordPlaceholder')"
                autofocus
              />
            </div>
          </template>
          <QtermFormMessage v-if="error" variant="error">{{ error }}</QtermFormMessage>
        </form>
        <QtermDialogFooter>
          <QtermButton variant="outline" type="button" :disabled="busy" @click="reset">{{
            t('asset.export.cancel')
          }}</QtermButton>
          <QtermButton
            v-if="importStep === 'password'"
            variant="outline"
            type="button"
            :disabled="busy"
            @click="backToSelection"
          >
            {{ t('asset.import.back') }}
          </QtermButton>
          <QtermButton
            type="button"
            :disabled="busy || readingFile"
            @click="importStep === 'select' ? continueImport() : requestImport()"
          >
            {{ t('asset.import.continue') }}
          </QtermButton>
        </QtermDialogFooter>
      </QtermDialogContent>
    </QtermDialog>

    <QtermDialog :open="confirmImportOpen" @update:open="(open) => !open && cancelImport()">
      <QtermDialogContent size="md">
        <QtermDialogHeader
          :title="t('asset.import.confirmTitle')"
          :description="t('asset.import.confirmDescription')"
          :close-disabled="busy"
          @close="cancelImport"
        />
        <div class="dialog-body">
          <QtermFormMessage>{{ t('asset.import.confirmHint') }}</QtermFormMessage>
          <QtermFormMessage v-if="error" variant="error">{{ error }}</QtermFormMessage>
        </div>
        <QtermDialogFooter>
          <QtermButton variant="outline" type="button" :disabled="busy" @click="cancelImport">{{
            t('asset.export.cancel')
          }}</QtermButton>
          <QtermButton type="button" :disabled="busy" @click="importAssets">{{
            busy ? t('asset.import.busy') : t('asset.import.confirm')
          }}</QtermButton>
        </QtermDialogFooter>
      </QtermDialogContent>
    </QtermDialog>
  </section>
</template>
