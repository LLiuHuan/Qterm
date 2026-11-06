<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { PanelTop } from '@lucide/vue'
import { QtermToaster, IconButton, SessionOverlay } from '../components/common'
import { ToolRail } from '../components/workspace'
import SessionWindowTitlebar from '../components/workspace/SessionWindowTitlebar.vue'
import RdpCanvasPane from '../features/remotedesktop/RdpCanvasPane.vue'
import { useRdpWindowHandover } from '../composables/useRdpWindowHandover'
import { useSettingsState } from '../features/settings/useSettingsState'
import { closeCurrentWindow, setSessionWindowTitle } from '../services/sessionWindows'
import { registerRdpWindowSession } from '../services/remotedesktop'
import { useI18n } from '../locales'
import { toast } from 'vue-sonner'

const params = typeof window !== 'undefined' ? new URLSearchParams(window.location.search) : new URLSearchParams()
const windowTitle = (params.get('rdpTitle') ?? '').trim()
const initialSessionId = (params.get('rdpSession') ?? '').trim()
const initialHostId = (params.get('rdpHost') ?? '').trim()
const initialHost = initialHostId || undefined

interface RdpPaneActions {
  sessionId?: string
  statusTone?: 'idle' | 'connected' | 'error'

  canPasteToRemote?: boolean
  canCopyFromRemote?: boolean
  beginHandover?: () => void
  cancelHandover?: () => void
  openLog?: () => void
  reconnect?: () => Promise<void>
  disconnect?: () => void
  pasteToRemote?: () => Promise<void> | void
  copyFromRemote?: () => Promise<void> | void
}

const pane = ref<RdpPaneActions | null>(null)

const windowSessionRegistered = ref(Boolean(initialSessionId))
const { isDark } = useSettingsState()
const { t } = useI18n()
const sessionId = computed(() => pane.value?.sessionId?.trim() || initialSessionId)
const returnableSessionId = computed(() => (windowSessionRegistered.value ? sessionId.value : ''))
const statusTone = computed(() => pane.value?.statusTone ?? 'idle')
const { returningToMain, returnToMain } = useRdpWindowHandover({
  sessionId: () => returnableSessionId.value,
  title: () => windowTitle,
  hostId: () => initialHostId,
  beginHandover: () => pane.value?.beginHandover?.(),
  cancelHandover: () => pane.value?.cancelHandover?.(),
})

onMounted(() => {
  void setSessionWindowTitle(windowTitle || initialHostId || 'RDP')
})

async function handleOpened(openedSessionId: string): Promise<void> {
  const openedId = openedSessionId.trim()
  if (!openedId) {
    windowSessionRegistered.value = false
    return
  }

  if (openedId === initialSessionId) {
    windowSessionRegistered.value = Boolean(openedSessionId.trim())
    return
  }
  try {
    await registerRdpWindowSession(openedId)
    windowSessionRegistered.value = true
  } catch {
    windowSessionRegistered.value = false
    toast.error(t('shell.app.returnToTabsFailed'))
  }
}

function closeRdpWindow(): void {
  void closeCurrentWindow()
}
</script>

<template>
  <div
    class="qterm-shell is-session-window h-screen w-screen text-sm"
    style="--qt-theme-divider: var(--qt-border)"
    :class="{ dark: isDark, 'is-dark': isDark }"
  >
    <SessionWindowTitlebar :title="windowTitle || initialHostId || 'RDP'" :status-tone="statusTone">
      <template #actions>
        <IconButton
          v-if="returnableSessionId"
          :label="t('shell.app.returnToTabs')"
          :disabled="returningToMain"
          :aria-busy="returningToMain"
          @click="returnToMain"
        >
          <PanelTop :size="16" aria-hidden="true" />
        </IconButton>
      </template>
    </SessionWindowTitlebar>

    <SessionOverlay
      v-if="returningToMain"
      variant="mask"
      tone="info"
      busy
      class="session-window-handover-mask"
      :message="t('shell.app.returnToTabsBusy')"
    />

    <div class="app-body">
      <main class="workspace">
        <div class="terminal-workspace-body min-h-0 min-w-0 relative col-[1] row-[2] flex flex-col overflow-hidden">
          <RdpCanvasPane
            ref="pane"
            class="h-full w-full"
            :initial-session-id="initialSessionId || undefined"
            initial-session-mode="window"
            :host-id="initialHost"
            :host-title="windowTitle || initialHostId"
            @opened="handleOpened"
            @close="closeRdpWindow"
          />
        </div>
      </main>

      <ToolRail
        :active-tool="null"
        :local-session="false"
        :can-reconnect="true"
        :can-disconnect="true"
        :rdp-active="true"
        :rdp-window-available="false"
        :rdp-can-paste="pane?.canPasteToRemote ?? true"
        :rdp-can-copy="pane?.canCopyFromRemote ?? true"
        @rdp-paste="pane?.pasteToRemote?.()"
        @rdp-copy="pane?.copyFromRemote?.()"
        @rdp-reconnect="pane?.reconnect?.()"
        @rdp-disconnect="pane?.disconnect?.()"
        @rdp-log="pane?.openLog?.()"
      />
    </div>

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
