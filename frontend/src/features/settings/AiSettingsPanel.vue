<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Pencil, Plus, ShieldAlert, Sparkles, Trash2 } from '@lucide/vue'
import {
  IconButton,
  QtermButton,
  QtermEmptyState,
  QtermNumberField,
  QtermSettingsRow,
  QtermSettingsSectionHeader,
  QtermSwitch,
  SegmentedTabs,
} from '../../components/common'
import { QtermCombobox } from '../../components/common/form-controls'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { useI18n } from '../../locales'
import { toast } from 'vue-sonner'
import { aiErrorMessage, clearProviderKey, providerKeyConfigured } from '../../services/ai'
import { readApprovalMode, writeApprovalMode, type AIApprovalMode } from '../../services/aiApproval'
import { readAgentSettings, writeAgentSettings, type AgentSettings } from '../../services/aiAgent'
import {
  readActiveProviderID,
  readProviders,
  writeActiveProviderID,
  writeProviders,
  type AIProvider,
} from '../../services/aiProviders'
import AiProviderDialog from '../../components/dialogs/AiProviderDialog.vue'
import { providerLogo } from '../ai/providerLogos'

const { t } = useI18n()

const providers = ref<AIProvider[]>([])
const activeID = ref('')

const keyStatus = ref<Record<string, boolean | undefined>>({})
const editorOpen = ref(false)
const editing = ref<AIProvider | null>(null)
const error = ref('')
const agentSettings = ref<AgentSettings>(readAgentSettings())
const approvalMode = ref<AIApprovalMode>(readApprovalMode())
const automaticWarningOpen = ref(false)
const approvalModes: AIApprovalMode[] = ['strict', 'balanced', 'review', 'automatic']
const approvalModeOptions = computed(() =>
  approvalModes.map((mode) => ({ value: mode, label: t(`shell.aiChat.mode.${mode}`) })),
)
const executionChannelOptions = computed(() => [
  { value: 'background', label: t('settings.ai.agent.execution.background') },
  { value: 'terminal', label: t('settings.ai.agent.execution.terminal') },
])

function persist(next: AIProvider[]): void {
  providers.value = next
  writeProviders(next)
  void refreshKeys()
}

function setExecutionChannel(value: string): void {
  saveAgentSettings({ executionChannel: value === 'terminal' ? 'terminal' : 'background' })
}

function saveAgentSettings(patch: Partial<AgentSettings>): void {
  agentSettings.value = { ...agentSettings.value, ...patch }
  writeAgentSettings(agentSettings.value)
}

function setApprovalMode(value: string | number): void {
  const mode = String(value) as AIApprovalMode
  if (!approvalModes.includes(mode)) return
  if (mode === 'automatic' && approvalMode.value !== 'automatic') {
    automaticWarningOpen.value = true
    return
  }
  approvalMode.value = mode
  writeApprovalMode(mode)
}

function enableAutomaticMode(): void {
  approvalMode.value = 'automatic'
  writeApprovalMode('automatic')
  automaticWarningOpen.value = false
}

async function refreshKeys(): Promise<void> {
  const entries = await Promise.all(
    providers.value.map(async (provider) => {
      try {
        return [provider.id, await providerKeyConfigured(provider.id)] as const
      } catch {
        return [provider.id, undefined] as const
      }
    }),
  )
  keyStatus.value = Object.fromEntries(entries)
}

onMounted(() => {
  providers.value = readProviders()
  activeID.value = readActiveProviderID()
  void refreshKeys()
})

function toggleEnabled(provider: AIProvider, enabled: boolean): void {
  persist(providers.value.map((item) => (item.id === provider.id ? { ...item, enabled } : item)))

  if (!enabled && activeID.value === provider.id) {
    const next = providers.value.find((item) => item.enabled && item.id !== provider.id)
    activeID.value = next?.id ?? ''
    writeActiveProviderID(activeID.value)
  }
}

function activate(provider: AIProvider): void {
  if (!provider.enabled) return

  if (activeID.value === provider.id) return
  activeID.value = provider.id
  writeActiveProviderID(provider.id)
}

function openCreate(): void {
  editing.value = null
  editorOpen.value = true
}

function openEdit(provider: AIProvider): void {
  editing.value = provider
  editorOpen.value = true
}

function handleSaved(saved: AIProvider): void {
  const exists = providers.value.some((provider) => provider.id === saved.id)
  persist(
    exists
      ? providers.value.map((provider) => (provider.id === saved.id ? saved : provider))
      : [...providers.value, saved],
  )
  if (!activeID.value) activate(saved)
  editorOpen.value = false
}

async function remove(provider: AIProvider): Promise<void> {
  error.value = ''
  try {
    await clearProviderKey(provider.id)
  } catch (cause) {
    error.value = aiErrorMessage(cause, t('settings.ai.keyClearFailed'))
    return
  }
  persist(providers.value.filter((item) => item.id !== provider.id))
  if (activeID.value === provider.id) {
    activeID.value = providers.value.find((item) => item.enabled)?.id ?? ''
    writeActiveProviderID(activeID.value)
  }
  toast.success(t('settings.ai.providerRemoved'))
}

function typeLabel(type: AIProvider['type']): string {
  return t(`settings.ai.providerType.${type}`)
}

function providerLabel(provider: AIProvider): string {
  return provider.name || provider.baseUrl
}
</script>

<template>
  <section class="settings-panel flex flex-col gap-4" :aria-label="t('settings.ai.title')">
    <QtermSettingsSectionHeader
      class="flex-cb gap-3"
      :label="t('settings.ai.section.providers.title')"
      :description="t('settings.ai.section.providers.hint')"
    >
      <template #actions>
        <QtermButton size="sm" class="text-xs" type="button" data-ai-provider-create @click="openCreate">
          <Plus :size="14" />{{ t('settings.ai.addProvider') }}
        </QtermButton>
      </template>
    </QtermSettingsSectionHeader>

    <p v-if="error" class="m-0 text-xs text-qt-danger">{{ error }}</p>

    <div class="flex flex-col gap-2" data-ai-provider-list>
      <div
        v-for="provider in providers"
        :key="provider.id"
        class="flex-cb gap-4 rounded-md border border-solid px-3 py-2.5 transition-colors"
        :class="
          provider.id === activeID
            ? 'border-qt-accent bg-[color-mix(in_srgb,var(--qt-accent)_6%,transparent)]'
            : 'border-qt-border hover:bg-[color-mix(in_srgb,var(--qt-accent)_4%,transparent)]'
        "
        data-ai-provider-row
        @click="activate(provider)"
      >
        <button
          type="button"
          class="flex min-w-0 flex-1 items-center gap-3 border-0 bg-transparent p-0 text-left text-qt-text c-p disabled:c-not-allowed"
          :aria-pressed="provider.id === activeID"
          :disabled="!provider.enabled"
          :aria-label="t('settings.ai.activateProvider', { name: providerLabel(provider) })"
          data-ai-provider-select
          @click.stop="activate(provider)"
        >
          <span class="flex size-5 shrink-0 items-center justify-center">
            <component :is="providerLogo(provider)" v-if="providerLogo(provider)" :size="18" aria-hidden="true" />
            <Sparkles v-else :size="16" class="text-qt-muted" aria-hidden="true" />
          </span>
          <span class="flex min-w-0 flex-1 flex-col gap-0.5">
            <span class="flex min-w-0 items-center gap-2">
              <strong class="truncate text-sm">{{ providerLabel(provider) }}</strong>
              <span class="shrink-0 rounded-full border-d bg-qt-subtle px-2 text-xs leading-4 text-qt-muted">{{
                typeLabel(provider.type)
              }}</span>
              <span
                v-if="keyStatus[provider.id] === false"
                class="shrink-0 rounded-full border-d px-2 text-xs leading-4 text-qt-warning"
                data-ai-key-missing
                >{{ t('settings.ai.keyMissing') }}</span
              >
            </span>
            <span class="truncate font-mono text-xs text-qt-muted">
              {{ provider.baseUrl }}
              <template v-if="provider.model"> · {{ provider.model }}</template>
              · {{ t('settings.ai.modelCount', { count: provider.models.length }) }}
            </span>
          </span>
        </button>
        <div class="flex shrink-0 items-center gap-1">
          <QtermSwitch
            :model-value="provider.enabled"
            :aria-label="t('settings.ai.enabled')"
            data-ai-provider-enabled
            @update:model-value="toggleEnabled(provider, $event)"
            @click.stop
          />
          <IconButton :label="t('settings.ai.editProvider')" size="icon-sm" @click.stop="openEdit(provider)">
            <Pencil :size="14" aria-hidden="true" />
          </IconButton>
          <IconButton danger :label="t('settings.ai.removeProvider')" size="icon-sm" @click.stop="remove(provider)">
            <Trash2 :size="14" aria-hidden="true" />
          </IconButton>
        </div>
      </div>

      <QtermEmptyState
        v-if="!providers.length"
        class="min-h-[160px] py-6"
        :icon="Sparkles"
        :title="t('settings.ai.noProviders')"
        :description="t('settings.ai.noProvidersHint')"
      />
    </div>

    <QtermSettingsSectionHeader
      class="settings-subsection-header mt-2"
      :label="t('settings.ai.approvalTitle')"
      :description="t('shell.aiChat.modeHint.' + approvalMode)"
    />
    <div
      class="settings-card border border-solid border-qt-border rounded-xl [background:color-mix(in_srgb,var(--qt-panel)_32%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
      data-ai-approval-settings
    >
      <QtermSettingsRow :label="t('shell.aiChat.approvalMode')">
        <div class="w-40 shrink-0">
          <QtermCombobox
            :model-value="approvalMode"
            :options="approvalModeOptions"
            :aria-label="t('shell.aiChat.approvalMode')"
            data-ai-approval-mode
            @update:model-value="setApprovalMode"
          />
        </div>
      </QtermSettingsRow>
    </div>

    <QtermSettingsSectionHeader
      class="settings-subsection-header mt-2"
      :label="t('settings.ai.agent.title')"
      :description="t('settings.ai.agent.hint')"
    />
    <div
      class="settings-card border border-solid border-qt-border rounded-xl [background:color-mix(in_srgb,var(--qt-panel)_32%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
      data-ai-agent-settings
    >
      <QtermSettingsRow
        as="label"
        for-id="settings-ai-agent-max-steps"
        class="settings-switch-row c-p"
        :label="t('settings.ai.agent.maxSteps')"
        :description="t('settings.ai.agent.maxStepsHelp')"
      >
        <QtermNumberField
          id="settings-ai-agent-max-steps"
          class="w-24 shrink-0"
          :model-value="agentSettings.maxSteps"
          :min="1"
          :max="50"
          :aria-label="t('settings.ai.agent.maxSteps')"
          data-ai-agent-max-steps
          @update:model-value="(value) => value !== undefined && saveAgentSettings({ maxSteps: value })"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        as="label"
        for-id="settings-ai-agent-model-timeout"
        class="settings-switch-row c-p"
        :label="t('settings.ai.agent.modelTimeout')"
      >
        <QtermNumberField
          id="settings-ai-agent-model-timeout"
          class="w-24 shrink-0"
          :model-value="agentSettings.modelTimeoutSeconds"
          :min="5"
          :max="600"
          :step="5"
          :aria-label="t('settings.ai.agent.modelTimeout')"
          data-ai-agent-model-timeout
          @update:model-value="(value) => value !== undefined && saveAgentSettings({ modelTimeoutSeconds: value })"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        as="label"
        for-id="settings-ai-agent-step-timeout"
        class="settings-switch-row c-p"
        :label="t('settings.ai.agent.stepTimeout')"
      >
        <QtermNumberField
          id="settings-ai-agent-step-timeout"
          class="w-24 shrink-0"
          :model-value="agentSettings.stepTimeoutSeconds"
          :min="5"
          :max="600"
          :step="5"
          :aria-label="t('settings.ai.agent.stepTimeout')"
          data-ai-agent-step-timeout
          @update:model-value="(value) => value !== undefined && saveAgentSettings({ stepTimeoutSeconds: value })"
        />
      </QtermSettingsRow>
      <QtermSettingsRow
        as="label"
        for-id="settings-ai-agent-inline-lines"
        class="settings-switch-row c-p"
        :label="t('settings.ai.agent.inlineLines')"
      >
        <QtermNumberField
          id="settings-ai-agent-inline-lines"
          class="w-24 shrink-0"
          :model-value="agentSettings.inlineLines"
          :min="1"
          :max="200"
          :step="1"
          :aria-label="t('settings.ai.agent.inlineLines')"
          data-ai-agent-inline-lines
          @update:model-value="(value) => value !== undefined && saveAgentSettings({ inlineLines: value })"
        />
      </QtermSettingsRow>
      <QtermSettingsRow :label="t('settings.ai.agent.executionChannel')">
        <SegmentedTabs
          :model-value="agentSettings.executionChannel"
          :options="executionChannelOptions"
          :label="t('settings.ai.agent.executionChannel')"
          data-ai-agent-channel
          @update:model-value="setExecutionChannel"
        />
      </QtermSettingsRow>
    </div>

    <AiProviderDialog :open="editorOpen" :provider="editing" @close="editorOpen = false" @saved="handleSaved" />
    <QtermDialog :open="automaticWarningOpen" @update:open="(open) => !open && (automaticWarningOpen = false)">
      <QtermDialogContent v-if="automaticWarningOpen" class="max-w-[460px]" data-ai-settings-automatic-warning>
        <QtermDialogHeader
          :title="t('shell.aiChat.automaticWarningTitle')"
          :description="t('shell.aiChat.automaticWarningDescription')"
        />
        <div
          class="flex items-start gap-2 rounded-lg border border-qt-warning/40 bg-qt-warning/5 px-3 py-3 text-sm leading-6 text-qt-text"
        >
          <ShieldAlert class="mt-0.5 size-4 shrink-0 text-qt-warning" aria-hidden="true" />
          <span>{{ t('shell.aiChat.automaticWarningBody') }}</span>
        </div>
        <QtermDialogFooter>
          <QtermButton variant="outline" @click="automaticWarningOpen = false">{{
            t('shell.aiChat.cancel')
          }}</QtermButton>
          <QtermButton variant="destructive" data-ai-settings-automatic-confirm @click="enableAutomaticMode">
            {{ t('shell.aiChat.enableAutomatic') }}
          </QtermButton>
        </QtermDialogFooter>
      </QtermDialogContent>
    </QtermDialog>
  </section>
</template>
