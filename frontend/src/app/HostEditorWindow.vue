<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, toRefs, watch } from 'vue'
import { toast } from 'vue-sonner'
import { QtermToaster } from '../components/common'
import GroupEditorDialog from '../components/dialogs/GroupEditorDialog.vue'
import HostEditorDialog from '../components/dialogs/HostEditorDialog.vue'
import SessionWindowTitlebar from '../components/workspace/SessionWindowTitlebar.vue'
import { useWindowTheme } from '../composables/useWindowTheme'
import { useCredentialState } from '../composables/useCredentialState'
import { useGroupEditor } from '../features/hosts/useGroupEditor'
import { useHostEditor } from '../features/hosts/useHostEditor'
import { useSettingsState } from '../features/settings/useSettingsState'
import { useI18n } from '../locales'
import { createHostGateway, hostErrorMessage, type GroupRecord, type HostRecord } from '../services/hosts'
import { createProxyGateway, type ProxyRecord } from '../services/proxies'
import { createSessionGateway } from '../services/sessions'
import { hasWailsRuntime } from '../services/runtime'
import {
  broadcastHostsChanged,
  closeCurrentWindow,
  closeHostEditorWindow,
  connectHostEditorTarget,
  openSettingsWindow,
  setSessionWindowTitle,
  showHostEditorWindow,
} from '../services/sessionWindows'

const params = typeof window !== 'undefined' ? new URLSearchParams(window.location.search) : new URLSearchParams()

const initialHostID = (params.get('hostID') ?? '').trim()

const { t } = useI18n()
const { settings, settingsReady, isDark } = useSettingsState()
const { pageBackgroundStyle, dialogThemeContext } = useWindowTheme({
  ...toRefs(settings),
  isDark,
  terminalLightTheme: computed(() => settings.terminalLightTheme ?? 'default-light'),
  terminalDarkTheme: computed(() => settings.terminalDarkTheme ?? 'default-dark'),
  terminalThemeBackgroundScope: computed(() => settings.terminalThemeBackgroundScope ?? 'terminal'),
})

const hostGateway = createHostGateway()
const sessionGateway = createSessionGateway()
const proxyGateway = createProxyGateway()

const credentialState = useCredentialState()
const {
  credentialOptions,
  credentialVaultStatus,
  credentialOptionsLoading,
  credentialOptionsError,
  credentialGateway,
  refreshCredentials,
} = credentialState

const hosts = ref<HostRecord[]>([])
const groups = ref<GroupRecord[]>([])
const proxies = ref<ProxyRecord[]>([])

async function loadHosts(): Promise<void> {
  const [hostList, groupList] = await Promise.all([hostGateway.list(), hostGateway.listGroups()])
  hosts.value = hostList
  groups.value = groupList
}

async function loadGroups(): Promise<void> {
  groups.value = await hostGateway.listGroups()
}

async function loadProxies(): Promise<void> {
  try {
    proxies.value = await proxyGateway.list()
  } catch {
    proxies.value = []
  }
}

async function closeEditorWindow(): Promise<void> {
  if (hasWailsRuntime()) {
    await closeHostEditorWindow()
    return
  }
  await closeCurrentWindow()
}

const hostEditor = useHostEditor({
  hostGateway,
  sessionGateway,
  credentials: credentialState,
  onSaved: async () => {
    try {
      await loadHosts()
    } catch {}

    await broadcastHostsChanged()
    await closeEditorWindow()
  },
})

const {
  hostDialogOpen,
  editingHostId,
  formError,
  savingHost,
  testingConnection,
  detectHostIcon,
  detectingIcon,
  authMode,
  directSecret,
  draft,
  openCreateDialog,
  openEditHostDialog,
  credentialKindLabel,
  setAuthMode,
  setPasswordSource,
  saveHost,
  testHostConnection,
} = hostEditor

function openTarget(hostID: string): void {
  const id = hostID.trim()
  if (!id) {
    openCreateDialog()
    return
  }
  const host = hosts.value.find((item) => item.id === id)
  if (!host) {
    toast.error(t('shell.error.targetHostMissing'))
    openCreateDialog()
    return
  }
  openEditHostDialog(host)
}

async function handleTargetEvent(hostID: string, version: number): Promise<void> {
  try {
    await loadHosts()
  } catch (error) {
    toast.error(hostErrorMessage(error, t('shell.error.loadHosts')))
  }
  if (version === targetChangeVersion) openTarget(hostID)
}

let stopTargetListener: (() => void) | undefined
let pendingHostID = initialHostID
let initialising = true
let targetChangeVersion = 0
let targetUpdate = Promise.resolve()

const windowTitle = computed(() =>
  editingHostId.value ? t('shell.hostEditor.editTitle') : t('shell.hostEditor.createTitle'),
)

watch(windowTitle, (value) => void setSessionWindowTitle(value), { immediate: true })

const groupEditor = useGroupEditor({ hostGateway, onSaved: loadGroups })
const {
  groupDialogOpen,
  editingGroupId,
  groupName,
  groupFormError,
  savingGroup,
  openCreateGroupDialog,
  closeGroupDialog: closeGroupDialogBase,
  saveGroup: saveGroupBase,
} = groupEditor

const selectCreatedGroupForHost = ref(false)

function openCreateGroupForHost(): void {
  selectCreatedGroupForHost.value = true
  openCreateGroupDialog()
}

function closeGroupDialog(): void {
  selectCreatedGroupForHost.value = false
  closeGroupDialogBase()
}

async function saveGroup(): Promise<void> {
  const group = await saveGroupBase()

  if (group && selectCreatedGroupForHost.value) draft.value.group = group.name
  selectCreatedGroupForHost.value = false
}

async function handleCredentialCreated(credentialId: string): Promise<void> {
  await refreshCredentials()
  draft.value.credentialId = credentialId
}

async function handleProxyCreated(proxyId: string): Promise<void> {
  await loadProxies()
  draft.value.proxyId = proxyId
  draft.value.proxyMode = 'none'
  draft.value.proxyHost = ''
  draft.value.proxyPort = 0
  draft.value.proxyUsername = ''
  draft.value.proxyPassword = ''
  draft.value.clearProxyPassword = false
  draft.value.proxyPasswordConfigured = false
}

async function openSecuritySettings(): Promise<void> {
  try {
    await openSettingsWindow(t('shell.toolbar.settings'), 'application')
  } catch {
    toast.error(t('shell.error.vaultUnavailable'))
  }
}

onMounted(async () => {
  stopTargetListener = connectHostEditorTarget((hostID) => {
    pendingHostID = hostID
    targetChangeVersion += 1
    if (!initialising) targetUpdate = handleTargetEvent(hostID, targetChangeVersion)
  })

  void loadProxies()
  try {
    await loadHosts()
  } catch (error) {
    toast.error(hostErrorMessage(error, t('shell.error.loadHosts')))
  }
  if (!settingsReady.value) {
    await new Promise<void>((resolve) => {
      const stop = watch(settingsReady, (ready) => {
        if (ready) {
          stop()
          resolve()
        }
      })
    })
  }
  let displayedTarget = pendingHostID
  openTarget(displayedTarget)

  initialising = false

  while (true) {
    const target = displayedTarget
    const version = targetChangeVersion
    await nextTick()
    const latest = await showHostEditorWindow(target)

    if (version !== targetChangeVersion) {
      await targetUpdate
      displayedTarget = pendingHostID
      continue
    }
    if (latest === target) break
    displayedTarget = latest
    openTarget(latest)
  }
})

onUnmounted(() => {
  stopTargetListener?.()
  stopTargetListener = undefined
})
</script>

<template>
  <div
    class="qterm-shell qterm-dialog h-screen max-h-none! w-screen overflow-hidden rounded-none! border-0! text-sm text-qt-text shadow-none!"
    :class="[dialogThemeContext.className, { dark: isDark }]"
    :style="[pageBackgroundStyle, { background: 'var(--background)', color: 'var(--foreground)' }]"
  >
    <SessionWindowTitlebar class="bg-inherit!" :title="windowTitle" :show-status="false" />

    <HostEditorDialog
      :otp-entries="settings.otpEntries ?? []"
      layout="page"
      :open="hostDialogOpen"
      :editing="!!editingHostId"
      :persistent="hostGateway.persistent"
      :draft="draft"
      :groups="groups"
      :auth-mode="authMode"
      :direct-secret="directSecret"
      :credentials="credentialOptions"
      :credential-gateway="credentialGateway"
      :proxy-gateway="proxyGateway"
      :hosts="hosts"
      :proxies="proxies"
      :editing-host-id="editingHostId || undefined"
      :vault-status="credentialVaultStatus"
      :credentials-loading="credentialOptionsLoading"
      :credentials-error="credentialOptionsError"
      :form-error="formError"
      :testing-connection="testingConnection"
      :detecting-icon="detectingIcon"
      :saving="savingHost"
      :credential-label="credentialKindLabel"
      @close="closeEditorWindow()"
      @submit="saveHost"
      @test-connection="testHostConnection"
      @detect-icon="detectHostIcon"
      @open-security="openSecuritySettings"
      @create-group="openCreateGroupForHost"
      @credential-created="handleCredentialCreated"
      @proxy-created="handleProxyCreated"
      @update-draft="Object.assign(draft, $event)"
      @update-auth-mode="setAuthMode"
      @update-password-source="setPasswordSource"
      @update-secret="directSecret = $event"
    />

    <GroupEditorDialog
      :open="groupDialogOpen"
      :editing="!!editingGroupId"
      :name="groupName"
      :error="groupFormError"
      :saving="savingGroup"
      @close="closeGroupDialog"
      @submit="saveGroup"
      @update-name="groupName = $event"
    />

    <QtermToaster
      position="top-center"
      :rich-colors="true"
      :close-button="false"
      :expand="false"
      :visible-toasts="3"
      :duration="3600"
    />
  </div>
</template>
