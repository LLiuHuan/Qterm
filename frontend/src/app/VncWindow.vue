<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { PanelTop } from '@lucide/vue'
import { QtermToaster, IconButton, SessionOverlay } from '../components/common'
import { ToolRail } from '../components/workspace'
import SessionWindowTitlebar from '../components/workspace/SessionWindowTitlebar.vue'
import VncCanvasPane from '../features/vnc/VncCanvasPane.vue'
import { requestVncTabOpen } from '../features/vnc/vncTabHandover'
import { useSettingsState } from '../features/settings/useSettingsState'
import { closeCurrentWindow, setSessionWindowTitle } from '../services/sessionWindows'
import { useI18n } from '../locales'
import { toast } from 'vue-sonner'

const params = typeof window !== 'undefined' ? new URLSearchParams(window.location.search) : new URLSearchParams()
const windowTitle = (params.get('vncTitle') ?? '').trim()
const initialHostId = (params.get('vncHost') ?? '').trim()

interface VncPaneActions {
  sessionId?: string
  statusTone?: 'idle' | 'connected' | 'error'
  fitEnabled?: boolean
  reconnect?: () => Promise<void> | void
  disconnect?: () => void
  toggleFit?: () => void

  pasteToRemote?: () => Promise<unknown> | unknown
}

const pane = ref<VncPaneActions | null>(null)
const { isDark } = useSettingsState()
const { t } = useI18n()
const statusTone = computed(() => pane.value?.statusTone ?? 'idle')

const returningToTabs = ref(false)

onMounted(() => {
  void setSessionWindowTitle(windowTitle || initialHostId || 'VNC')
})

function closeWindow(): void {
  void closeCurrentWindow()
}

async function returnToTabs(): Promise<void> {
  if (returningToTabs.value) return
  const hostId = initialHostId.trim()

  if (!hostId) return
  returningToTabs.value = true
  try {
    const outcome = await requestVncTabOpen(hostId, windowTitle)
    if (!outcome.ok) {
      returningToTabs.value = false
      toast.warning(
        outcome.reason === 'no-listener' ? t('shell.app.returnToTabsNoWindow') : t('shell.app.returnToTabsFailed'),
      )
      return
    }

    await closeCurrentWindow()
    returningToTabs.value = false
  } catch {
    returningToTabs.value = false
    toast.error(t('shell.app.returnToTabsFailed'))
  }
}
</script>

<template>
  <div
    class="qterm-shell is-session-window h-screen w-screen text-sm"
    style="--qt-theme-divider: var(--qt-border)"
    :class="{ dark: isDark, 'is-dark': isDark }"
  >
    <SessionWindowTitlebar :title="windowTitle || initialHostId || 'VNC'" :status-tone="statusTone">
      <template #actions>
        <IconButton
          v-if="initialHostId"
          :label="t('shell.app.returnToTabs')"
          :disabled="returningToTabs"
          :aria-busy="returningToTabs"
          @click="returnToTabs"
        >
          <PanelTop :size="16" aria-hidden="true" />
        </IconButton>
      </template>
    </SessionWindowTitlebar>

    <SessionOverlay
      v-if="returningToTabs"
      variant="mask"
      tone="info"
      busy
      class="session-window-handover-mask"
      :message="t('shell.app.returnToTabsBusy')"
    />

    <div class="app-body">
      <main class="workspace">
        <div class="terminal-workspace-body min-h-0 min-w-0 relative col-[1] row-[2] flex flex-col overflow-hidden">
          <VncCanvasPane
            ref="pane"
            class="h-full w-full"
            :host-id="initialHostId"
            :host-title="windowTitle || initialHostId"
            @close="closeWindow"
          />
        </div>
      </main>

      <ToolRail
        :active-tool="null"
        :local-session="false"
        :can-reconnect="true"
        :can-disconnect="true"
        :rdp-active="true"
        :vnc-active="true"
        :vnc-fit="pane?.fitEnabled ?? true"
        @vnc-fit="pane?.toggleFit?.()"
        @vnc-paste="pane?.pasteToRemote?.()"
        @rdp-reconnect="pane?.reconnect?.()"
        @rdp-disconnect="pane?.disconnect?.()"
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
