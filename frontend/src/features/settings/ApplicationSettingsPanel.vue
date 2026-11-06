<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  IconButton,
  QtermButton,
  QtermFormMessage,
  QtermInput,
  QtermNumberField,
  QtermSettingsRow,
  QtermSettingsSectionHeader,
  QtermSwitch,
  SegmentedTabs,
} from '@/components/common'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/common/form-controls'
import LocalDataEncryptionSettings from './LocalDataEncryptionSettings.vue'
import AppLockSettings from '../applock/AppLockSettings.vue'
import type {
  ChangeEncryptionKeyInput,
  CredentialVaultStatus,
  ExportCredentialVaultKeyInput,
  ImportCredentialVaultKeyInput,
  ResetCredentialVaultInput,
} from '@/services/credentials.ts'
import {
  AppLocaleSetting,
  SESSION_HISTORY_LIMIT_MAX,
  SESSION_HISTORY_LIMIT_MIN,
  SFTP_MAX_TRANSFERS_MAX,
  SFTP_MAX_TRANSFERS_MIN,
  SftpDirectoryFollowMode,
  SftpDragFolderUploadMode,
} from './settingsStore'
import { setSessionHistoryLimit } from '@/services/history.ts'
import { FolderOpen, FolderSearch, RotateCcw } from '@lucide/vue'
import { detectX11Server, type X11DetectionResult } from '../../services/x11'
import { openRecordingDirectory, selectRecordingDirectory } from '../../services/recordings'
import { selectSFTPEditor, selectSFTPLocalDirectorySetting } from '@/services/sftp.ts'
import { toast } from 'vue-sonner'
import { LOCALE_OPTIONS, isSupportedLocale, useI18n } from '@/locales'

const props = withDefaults(
  defineProps<{
    locale: AppLocaleSetting
    sessionHistoryLimit: number
    recordAllSessions?: boolean
    sftpDirectoryFollowMode: SftpDirectoryFollowMode

    sftpDragFolderUploadMode?: SftpDragFolderUploadMode
    sftpAutoDirectoryFollow: boolean
    sftpShowHiddenFiles: boolean
    sftpMaxTransfers: number
    sftpLocalDirectory: string
    sftpEditorPath: string
    x11Display?: string
    x11XauthPath?: string
    x11AuthorityFile?: string

    recordingDirectory?: string
    recordingDirectoryEffective?: string
    windowRememberSize: boolean

    closeToTray?: boolean

    restoreOpenSessions?: boolean

    quitConfirm?: boolean

    settingsInWindow?: boolean
    windowRememberPosition: boolean
    vaultStatus?: CredentialVaultStatus | null
    securityWorking?: boolean
    securityError?: string
    locked?: boolean
  }>(),
  {
    vaultStatus: null,
    securityWorking: false,
    securityError: '',
    locked: false,
    x11Display: '',
    x11XauthPath: '',
    x11AuthorityFile: '',
    recordingDirectory: '',
    recordingDirectoryEffective: '',
    sftpDragFolderUploadMode: 'normal',
  },
)
const { t } = useI18n()

const emit = defineEmits<{
  'update:locale': [value: AppLocaleSetting]
  'update:sessionHistoryLimit': [value: number]
  'update:recordAllSessions': [value: boolean]
  'update:sftpDirectoryFollowMode': [value: SftpDirectoryFollowMode]
  'update:sftpDragFolderUploadMode': [value: SftpDragFolderUploadMode]
  'update:sftpAutoDirectoryFollow': [value: boolean]
  'update:sftpShowHiddenFiles': [value: boolean]
  'update:sftpMaxTransfers': [value: number]
  'update:sftpLocalDirectory': [value: string]
  'update:sftpEditorPath': [value: string]
  'update:x11Display': [value: string]
  'update:x11XauthPath': [value: string]
  'update:x11AuthorityFile': [value: string]
  'update:recordingDirectory': [value: string]
  'update:windowRememberSize': [value: boolean]
  'update:closeToTray': [value: boolean]
  'update:restoreOpenSessions': [value: boolean]
  'update:quitConfirm': [value: boolean]
  'update:settingsInWindow': [value: boolean]
  'update:windowRememberPosition': [value: boolean]
  changeEncryptionKey: [input: ChangeEncryptionKeyInput]
  exportRecovery: [input: ExportCredentialVaultKeyInput]
  importRecovery: [input: ImportCredentialVaultKeyInput]
  unlockCustomKey: [key: string]
  resetVault: [input: ResetCredentialVaultInput]
}>()

const languageOptions = computed(() =>
  LOCALE_OPTIONS.map((option) => ({ value: option.value, label: t(option.labelKey) })),
)
const directoryFollowModeOptions = computed(() => [
  { value: 'session', label: t('application.sftp.directoryFollowMode.session') },
  { value: 'remote', label: t('application.sftp.directoryFollowMode.remote') },
  { value: 'disabled', label: t('application.sftp.directoryFollowMode.disabled') },
])
const dragFolderUploadModeOptions = computed(() => [
  { value: 'normal', label: t('application.sftp.dragFolderUploadMode.normal') },
  { value: 'archive', label: t('application.sftp.dragFolderUploadMode.archive') },
])
const limit = computed({
  get: () => props.sessionHistoryLimit,

  set: (value: number | undefined) => {
    if (value !== undefined) emit('update:sessionHistoryLimit', value)
  },
})
async function applyHistoryLimit(): Promise<void> {
  const value = Math.round(Number(props.sessionHistoryLimit))
  if (!Number.isFinite(value) || value < 20 || value > 10000) {
    toast.error(t('application.history.limit.invalid'))
    return
  }
  try {
    emit('update:sessionHistoryLimit', value)
    await setSessionHistoryLimit(value)
    toast.success(t('application.history.limit.success'))
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : t('application.history.limit.failure'))
  }
}

watch(
  () => props.locale,
  (value) => {
    if (!isSupportedLocale(value)) emit('update:locale', 'zh-CN')
  },
  { immediate: true },
)

const recordingDirectoryBusy = ref(false)

const recordingDirectoryDisplay = computed(() => props.recordingDirectoryEffective)

async function chooseRecordingDirectory(): Promise<void> {
  if (recordingDirectoryBusy.value) return
  recordingDirectoryBusy.value = true
  try {
    const value = await selectRecordingDirectory()
    if (value) emit('update:recordingDirectory', value)
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : t('history.recording.directory.selectFailed'))
  } finally {
    recordingDirectoryBusy.value = false
  }
}

async function openCurrentRecordingDirectory(): Promise<void> {
  if (recordingDirectoryBusy.value) return
  recordingDirectoryBusy.value = true
  try {
    await openRecordingDirectory()
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : t('history.recording.directory.openFailed'))
  } finally {
    recordingDirectoryBusy.value = false
  }
}

function resetRecordingDirectory(): void {
  emit('update:recordingDirectory', '')
}

const x11Detection = ref<X11DetectionResult | null>(null)
const x11Detecting = ref(false)
const x11DetectError = ref('')

const x11DetectionText = computed(() => {
  if (!x11Detection.value) return ''
  const result = x11Detection.value
  if (!result.available) {
    const reason = t(`application.x11.reason.${result.reason.replace('x11.', '')}`)
    return result.detail ? `${reason}（${result.detail}）` : reason
  }
  const display = result.display || ':0'
  const authority = result.authorityFile || t('application.x11.autoValue')
  return t('application.x11.detected', { display, authority })
})

async function detectX11(): Promise<void> {
  if (x11Detecting.value) return
  x11Detecting.value = true
  x11DetectError.value = ''
  try {
    x11Detection.value = await detectX11Server({
      display: props.x11Display ?? '',
      xauthPath: props.x11XauthPath ?? '',
      authorityFile: props.x11AuthorityFile ?? '',
    })
  } catch (cause) {
    x11Detection.value = null
    x11DetectError.value = cause instanceof Error ? cause.message : t('application.x11.detectFailed')
  } finally {
    x11Detecting.value = false
  }
}

function autoDetectX11Field(key: 'x11Display' | 'x11XauthPath' | 'x11AuthorityFile'): void {
  if (key === 'x11Display') emit('update:x11Display', '')
  else if (key === 'x11XauthPath') emit('update:x11XauthPath', '')
  else emit('update:x11AuthorityFile', '')
  x11Detection.value = null
}

async function chooseSFTPLocalDirectory(): Promise<void> {
  try {
    const value = await selectSFTPLocalDirectorySetting()
    if (value) emit('update:sftpLocalDirectory', value)
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : t('application.sftp.selectionFailed'))
  }
}

async function chooseSFTPEditor(): Promise<void> {
  try {
    const value = await selectSFTPEditor()
    if (value) emit('update:sftpEditorPath', value)
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : t('application.sftp.selectionFailed'))
  }
}

function updateDirectoryFollowMode(value: string): void {
  if (value !== 'session' && value !== 'remote' && value !== 'disabled') return
  emit('update:sftpDirectoryFollowMode', value)
}

function updateDragFolderUploadMode(value: string): void {
  if (value !== 'normal' && value !== 'archive') return
  emit('update:sftpDragFolderUploadMode', value)
}
</script>

<template>
  <section class="settings-panel flex flex-col gap-4" :aria-label="t('settings.application.aria')">
    <LocalDataEncryptionSettings
      v-if="props.locked"
      :status="props.vaultStatus"
      :working="props.securityWorking"
      :error="props.securityError"
      @change-encryption-key="emit('changeEncryptionKey', $event)"
      @export-recovery="emit('exportRecovery', $event)"
      @import-recovery="emit('importRecovery', $event)"
      @unlock-custom-key="emit('unlockCustomKey', $event)"
      @reset-vault="emit('resetVault', $event)"
    />

    <QtermSettingsSectionHeader
      :label="t('application.language.title')"
      :description="t('application.language.description')"
    />
    <div
      class="settings-card border border-solid border-qt-border rounded-xl [background:color-mix(in_srgb,var(--qt-panel)_32%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
    >
      <QtermSettingsRow :label="t('application.language.label')">
        <Select
          :model-value="props.locale"
          @update:model-value="(value) => emit('update:locale', String(value) as AppLocaleSetting)"
        >
          <SelectTrigger class="w-38" :aria-label="t('application.language.label')">
            <SelectValue />
          </SelectTrigger>
          <SelectContent position="popper" align="end">
            <SelectItem v-for="option in languageOptions" :key="option.value" :value="option.value">
              {{ option.label }}
            </SelectItem>
          </SelectContent>
        </Select>
      </QtermSettingsRow>
    </div>

    <QtermSettingsSectionHeader
      class="mt-2"
      :label="t('application.window.title')"
      :description="t('application.window.description')"
    />
    <div
      class="settings-card border border-solid border-qt-border rounded-xl [background:color-mix(in_srgb,var(--qt-panel)_32%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
    >
      <QtermSettingsRow
        :label="t('application.window.quitConfirm')"
        :description="
          (props.closeToTray ?? true)
            ? t('application.window.quitConfirmHelpTray')
            : t('application.window.quitConfirmHelp')
        "
      >
        <QtermSwitch
          :model-value="props.quitConfirm ?? true"
          @update:model-value="emit('update:quitConfirm', $event)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        :label="t('application.window.settingsInWindow')"
        :description="t('application.window.settingsInWindowHelp')"
      >
        <QtermSwitch
          :model-value="props.settingsInWindow ?? true"
          @update:model-value="emit('update:settingsInWindow', $event)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        :label="t('application.window.restoreOpenSessions')"
        :description="t('application.window.restoreOpenSessionsHelp')"
      >
        <QtermSwitch
          :model-value="props.restoreOpenSessions ?? true"
          @update:model-value="emit('update:restoreOpenSessions', $event)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        :label="t('application.window.closeToTray')"
        :description="t('application.window.closeToTrayHelp')"
      >
        <QtermSwitch
          :model-value="props.closeToTray ?? true"
          @update:model-value="emit('update:closeToTray', $event)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        :label="t('application.window.rememberSize')"
        :description="t('application.window.rememberSizeHelp')"
      >
        <QtermSwitch
          :model-value="props.windowRememberSize"
          @update:model-value="emit('update:windowRememberSize', $event)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        :label="t('application.window.rememberPosition')"
        :description="t('application.window.rememberPositionHelp')"
      >
        <QtermSwitch
          :model-value="props.windowRememberPosition"
          @update:model-value="emit('update:windowRememberPosition', $event)"
        />
      </QtermSettingsRow>
    </div>

    <QtermSettingsSectionHeader
      class="mt-2"
      :label="t('application.history.title')"
      :description="t('application.history.description')"
    />
    <div
      class="settings-card border border-solid border-qt-border rounded-xl [background:color-mix(in_srgb,var(--qt-panel)_32%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
    >
      <QtermSettingsRow
        :label="t('application.history.limit.label')"
        :description="t('application.history.limit.label')"
      >
        <div class="flex gap-2">
          <QtermNumberField
            v-model="limit"
            class="w-30"
            :min="SESSION_HISTORY_LIMIT_MIN"
            :max="SESSION_HISTORY_LIMIT_MAX"
            :step="10"
            :step-snapping="false"
            :aria-label="t('application.history.limit.aria')"
          />
          <QtermButton variant="outline" type="button" :disabled="props.locked" @click="applyHistoryLimit">
            {{ t('application.history.limit.apply') }}
          </QtermButton>
        </div>
      </QtermSettingsRow>
      <QtermSettingsRow :label="t('application.recording.label')" :description="t('application.recording.help')">
        <QtermSwitch
          :model-value="props.recordAllSessions"
          :aria-label="t('application.recording.label')"
          :disabled="props.locked"
          @update:model-value="emit('update:recordAllSessions', $event)"
        />
      </QtermSettingsRow>

      <div data-testid="settings-card">
        <QtermSettingsRow
          layout="stacked"
          :label="t('history.recording.directory.title')"
          :description="t('history.recording.directory.description')"
        >
          <div class="flex w-full shrink-0 items-center gap-2">
            <QtermInput
              class="flex-1"
              :model-value="props.recordingDirectory ?? ''"
              :placeholder="recordingDirectoryDisplay"
              :aria-label="t('history.recording.directory.title')"
              @update:model-value="emit('update:recordingDirectory', String($event))"
            />
            <IconButton
              :label="t('history.recording.directory.choose')"
              :disabled="recordingDirectoryBusy"
              @click="chooseRecordingDirectory()"
            >
              <FolderSearch :size="16" />
            </IconButton>
            <IconButton
              :label="t('history.recording.directory.open')"
              :disabled="recordingDirectoryBusy"
              @click="openCurrentRecordingDirectory()"
            >
              <FolderOpen :size="16" />
            </IconButton>
            <IconButton
              :label="t('history.recording.directory.reset')"
              :disabled="recordingDirectoryBusy || !props.recordingDirectory"
              @click="resetRecordingDirectory()"
            >
              <RotateCcw :size="16" />
            </IconButton>
          </div>
        </QtermSettingsRow>
      </div>
    </div>

    <QtermSettingsSectionHeader
      class="mt-2"
      :label="t('application.sftp.title')"
      :description="t('application.sftp.description')"
    />
    <div
      class="settings-card border border-solid border-qt-border rounded-xl [background:color-mix(in_srgb,var(--qt-panel)_32%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
    >
      <QtermSettingsRow
        :label="t('application.sftp.directoryFollowMode.label')"
        :description="t(`application.sftp.directoryFollowMode.${props.sftpDirectoryFollowMode}Help`)"
      >
        <SegmentedTabs
          :model-value="props.sftpDirectoryFollowMode"
          :options="directoryFollowModeOptions"
          :label="t('application.sftp.directoryFollowMode.label')"
          @update:model-value="updateDirectoryFollowMode"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        :label="t('application.sftp.directoryFollow.autoDetect')"
        :description="t('application.sftp.directoryFollow.autoDetectHelp')"
      >
        <QtermSwitch
          :model-value="props.sftpAutoDirectoryFollow"
          :disabled="props.locked"
          @update:model-value="emit('update:sftpAutoDirectoryFollow', $event)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow :label="t('application.sftp.showHidden')" :description="t('application.sftp.showHiddenHelp')">
        <QtermSwitch
          :model-value="props.sftpShowHiddenFiles"
          @update:model-value="emit('update:sftpShowHiddenFiles', $event)"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        :label="t('application.sftp.maxTransfers')"
        :description="t('application.sftp.maxTransfersHelp')"
      >
        <QtermNumberField
          class="w-30"
          :model-value="props.sftpMaxTransfers"
          :min="SFTP_MAX_TRANSFERS_MIN"
          :max="SFTP_MAX_TRANSFERS_MAX"
          @update:model-value="(value) => value !== undefined && emit('update:sftpMaxTransfers', value)"
        />
      </QtermSettingsRow>

      <QtermSettingsRow
        :label="t('application.sftp.dragFolderUploadMode.label')"
        :description="t(`application.sftp.dragFolderUploadMode.${props.sftpDragFolderUploadMode}Help`)"
      >
        <SegmentedTabs
          :model-value="props.sftpDragFolderUploadMode"
          :options="dragFolderUploadModeOptions"
          :label="t('application.sftp.dragFolderUploadMode.label')"
          @update:model-value="updateDragFolderUploadMode"
        />
      </QtermSettingsRow>

      <QtermSettingsRow
        layout="stacked"
        :label="t('application.sftp.localDirectory')"
        :description="t('application.sftp.localDirectoryHelp')"
      >
        <div class="flex w-full items-center gap-2">
          <QtermInput
            class="flex-1"
            :model-value="props.sftpLocalDirectory"
            :placeholder="t('application.sftp.localDirectoryPlaceholder')"
            @update:model-value="emit('update:sftpLocalDirectory', String($event))"
          />
          <QtermButton variant="outline" type="button" @click="chooseSFTPLocalDirectory">{{
            t('application.sftp.browse')
          }}</QtermButton>
        </div>
      </QtermSettingsRow>

      <QtermSettingsRow
        layout="stacked"
        :label="t('application.sftp.editor')"
        :description="t('application.sftp.editorHelp')"
      >
        <div class="flex w-full items-center gap-2">
          <QtermInput
            class="flex-1"
            :model-value="props.sftpEditorPath"
            :placeholder="t('application.sftp.editorPlaceholder')"
            @update:model-value="emit('update:sftpEditorPath', String($event))"
          />
          <QtermButton variant="outline" type="button" @click="chooseSFTPEditor">{{
            t('application.sftp.browse')
          }}</QtermButton>
        </div>
      </QtermSettingsRow>
    </div>

    <QtermSettingsSectionHeader
      class="mt-2"
      :label="t('application.x11.title')"
      :description="t('application.x11.description')"
    />
    <div
      class="settings-card border border-solid border-qt-border rounded-xl [background:color-mix(in_srgb,var(--qt-panel)_32%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
    >
      <QtermSettingsRow
        layout="stacked"
        :label="t('application.x11.display')"
        :description="t('application.x11.displayHelp')"
      >
        <div class="flex w-full items-center gap-2">
          <QtermInput
            class="flex-1"
            :model-value="props.x11Display ?? ''"
            :placeholder="t('application.x11.autoPlaceholder')"
            :aria-label="t('application.x11.display')"
            @update:model-value="emit('update:x11Display', String($event))"
          />
          <QtermButton variant="outline" type="button" @click="autoDetectX11Field('x11Display')">{{
            t('application.x11.autoDetect')
          }}</QtermButton>
        </div>
      </QtermSettingsRow>
      <QtermSettingsRow
        layout="stacked"
        :label="t('application.x11.xauthPath')"
        :description="t('application.x11.xauthPathHelp')"
      >
        <div class="flex w-full items-center gap-2">
          <QtermInput
            class="flex-1"
            :model-value="props.x11XauthPath ?? ''"
            :placeholder="t('application.x11.autoPlaceholder')"
            :aria-label="t('application.x11.xauthPath')"
            @update:model-value="emit('update:x11XauthPath', String($event))"
          />
          <QtermButton variant="outline" type="button" @click="autoDetectX11Field('x11XauthPath')">{{
            t('application.x11.autoDetect')
          }}</QtermButton>
        </div>
      </QtermSettingsRow>
      <QtermSettingsRow
        layout="stacked"
        :label="t('application.x11.authorityFile')"
        :description="t('application.x11.authorityFileHelp')"
      >
        <div class="flex w-full items-center gap-2">
          <QtermInput
            class="flex-1"
            :model-value="props.x11AuthorityFile ?? ''"
            :placeholder="t('application.x11.autoPlaceholder')"
            :aria-label="t('application.x11.authorityFile')"
            @update:model-value="emit('update:x11AuthorityFile', String($event))"
          />
          <QtermButton variant="outline" type="button" @click="autoDetectX11Field('x11AuthorityFile')">{{
            t('application.x11.autoDetect')
          }}</QtermButton>
        </div>
      </QtermSettingsRow>
      <div class="settings-card-row flex-cb gap-4 pt-3 pr-3.5 pb-3 pl-3.5 border-b border-b-qt-divider">
        <div>
          <small class="text-xs">{{ x11DetectionText || t('application.x11.notDetected') }}</small>
          <QtermFormMessage v-if="x11DetectError" as="small" variant="error">{{ x11DetectError }}</QtermFormMessage>
        </div>
        <QtermButton variant="outline" type="button" :disabled="x11Detecting" @click="detectX11">
          {{ x11Detecting ? t('application.x11.detecting') : t('application.x11.detectAction') }}
        </QtermButton>
      </div>
    </div>

    <LocalDataEncryptionSettings
      v-if="!props.locked"
      class="mt-2"
      :status="props.vaultStatus"
      :working="props.securityWorking"
      :error="props.securityError"
      @change-encryption-key="emit('changeEncryptionKey', $event)"
      @export-recovery="emit('exportRecovery', $event)"
      @import-recovery="emit('importRecovery', $event)"
      @unlock-custom-key="emit('unlockCustomKey', $event)"
      @reset-vault="emit('resetVault', $event)"
    />

    <QtermSettingsSectionHeader :label="t('applock.enable.label')" :description="t('applock.enable.help')" />
    <AppLockSettings />
  </section>
</template>
