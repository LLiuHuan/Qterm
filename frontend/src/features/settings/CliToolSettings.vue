<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { QtermButton, QtermFormMessage, QtermSettingsRow, QtermSettingsSectionHeader } from '../../components/common'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../components/common/form-controls'
import { useI18n } from '../../locales'
import { errorMessage } from '../../services/errors.ts'
import { toast } from '../../services/toast'
import {
  applyCliToolSettings,
  getCliToolState,
  installCliToolBinary,
  installCliToolSkills,
  uninstallCliToolBinary,
  uninstallCliToolSkills,
  type CliExecGuard,
  type CliToolState,
} from '../../services/cliTool'

const { t } = useI18n()

const state = ref<CliToolState | null>(null)
const busy = ref(false)
const error = ref('')

const installed = computed(() => state.value?.installed === true)

const removable = computed(() => state.value?.removable === true)
const skillsInstalled = computed(() => state.value?.skillsInstalled === true)
const binaryAvailable = computed(() => state.value?.binaryAvailable === true)

const ready = computed(() => state.value !== null && !busy.value)
const guard = computed<CliExecGuard>(() => state.value?.settings.execGuard ?? 'rules')

const guardOptions = computed(() => [
  { value: 'off' as const, label: t('application.cli.guard.off') },
  { value: 'rules' as const, label: t('application.cli.guard.rules') },
  { value: 'strict' as const, label: t('application.cli.guard.strict') },
])

const guardHelp = computed(() => {
  switch (guard.value) {
    case 'off':
      return t('application.cli.guard.offHelp')
    case 'strict':
      return t('application.cli.guard.strictHelp')
    default:
      return t('application.cli.guard.rulesHelp')
  }
})

const installedPath = computed(() => {
  const snapshot = state.value
  if (!snapshot) return ''
  return snapshot.removable ? snapshot.linkPath : snapshot.binaryPath
})

const hintText = computed(() => {
  const snapshot = state.value
  const key = snapshot?.hintKey ?? ''
  if (!key) return ''
  return snapshot?.pathFile
    ? t(`application.cli.hint.${key}`, { file: snapshot.pathFile })
    : t(`application.cli.hint.${key}`)
})

const HINT_ERROR_KEYS: readonly string[] = [
  'binaryMissing',
  'unsupportedPlatform',
  'notOnPath',
  'restartShell',
  'shellPathUpdated',
]

const hintIsError = computed(() => HINT_ERROR_KEYS.includes(state.value?.hintKey ?? ''))

function describeError(cause: unknown): string {
  const message = errorMessage(cause, '')
  return /^[a-zA-Z0-9_]+(\.[a-zA-Z0-9_]+)+$/.test(message) ? t('application.cli.toast.failed') : message
}

async function load(): Promise<void> {
  try {
    state.value = await getCliToolState()
  } catch (cause) {
    error.value = describeError(cause)
  }
}

async function run(action: () => Promise<CliToolState>, successKey: string): Promise<void> {
  if (!ready.value) return
  busy.value = true
  error.value = ''
  try {
    state.value = await action()
    toast.success(t(successKey))
  } catch (cause) {
    error.value = describeError(cause)
    toast.error(t('application.cli.toast.failed'))
  } finally {
    busy.value = false
  }
}

function isGuard(value: string): value is CliExecGuard {
  return value === 'off' || value === 'rules' || value === 'strict'
}

async function setGuard(value: unknown): Promise<void> {
  const next = typeof value === 'string' ? value : ''
  if (!isGuard(next) || next === guard.value) return
  await run(() => applyCliToolSettings({ execGuard: next }), 'application.cli.toast.settingsSaved')
}

onMounted(() => {
  void load()
})
</script>

<template>
  <div class="flex flex-col gap-4">
    <QtermSettingsSectionHeader :label="t('application.cli.title')" :description="t('application.cli.description')" />
    <div
      class="settings-card border border-solid border-qt-border rounded-xl [background:color-mix(in_srgb,var(--qt-panel)_32%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
    >
      <QtermSettingsRow
        layout="stacked"
        :label="t('application.cli.binary')"
        :description="t('application.cli.binaryHelp')"
      >
        <div class="flex flex-col gap-2" data-cli-binary>
          <small class="text-xs">
            {{
              installed ? t('application.cli.installed', { path: installedPath }) : t('application.cli.notInstalled')
            }}
          </small>

          <div v-if="!installed || removable" class="flex items-center gap-2">
            <QtermButton
              variant="outline"
              type="button"
              :disabled="!ready || !binaryAvailable"
              @click="run(installCliToolBinary, 'application.cli.toast.installed')"
            >
              {{ installed ? t('application.cli.reinstall') : t('application.cli.install') }}
            </QtermButton>
            <QtermButton
              v-if="installed && removable"
              variant="outline"
              type="button"
              :disabled="!ready"
              @click="run(uninstallCliToolBinary, 'application.cli.toast.uninstalled')"
            >
              {{ t('application.cli.uninstall') }}
            </QtermButton>
          </div>
        </div>
      </QtermSettingsRow>

      <QtermSettingsRow
        layout="stacked"
        :label="t('application.cli.skills.title')"
        :description="t('application.cli.skills.description')"
      >
        <div class="flex flex-col gap-2" data-cli-skills>
          <small class="text-xs">
            {{
              skillsInstalled
                ? t('application.cli.skills.installed', { path: state?.skillsPath ?? '' })
                : t('application.cli.skills.notInstalled')
            }}
          </small>
          <div class="flex items-center gap-2">
            <QtermButton
              variant="outline"
              type="button"
              :disabled="!ready"
              @click="run(installCliToolSkills, 'application.cli.toast.skillsInstalled')"
            >
              {{ t('application.cli.skills.install') }}
            </QtermButton>
            <QtermButton
              v-if="skillsInstalled"
              variant="outline"
              type="button"
              :disabled="!ready"
              @click="run(uninstallCliToolSkills, 'application.cli.toast.skillsUninstalled')"
            >
              {{ t('application.cli.skills.uninstall') }}
            </QtermButton>
          </div>
        </div>
      </QtermSettingsRow>

      <QtermSettingsRow :label="t('application.cli.guard.label')" :description="guardHelp">
        <div class="w-40 shrink-0">
          <Select
            :model-value="guard"
            :disabled="!ready"
            :aria-label="t('application.cli.guard.label')"
            @update:model-value="setGuard"
          >
            <SelectTrigger class="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent position="popper" align="end">
              <SelectItem v-for="option in guardOptions" :key="option.value" :value="option.value">
                {{ option.label }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
      </QtermSettingsRow>

      <div v-if="hintText || error" class="flex flex-col gap-1 pt-3 pr-3.5 pb-3 pl-3.5">
        <small v-if="hintText" class="text-xs" :class="hintIsError ? 'text-qt-danger' : ''" data-cli-hint>{{
          hintText
        }}</small>
        <QtermFormMessage v-if="error" as="small" variant="error">{{ error }}</QtermFormMessage>
      </div>
    </div>
  </div>
</template>
