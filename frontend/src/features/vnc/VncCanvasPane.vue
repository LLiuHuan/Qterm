<template>
  <section
    ref="panel"
    class="relative flex h-full w-full flex-col overflow-hidden rounded-md border border-qt-theme-divider bg-black"
    :aria-label="t('shell.vnc.paneLabel', { host: hostTitle || hostId })"
    tabindex="-1"
    @keydown.capture="onKeydown"
    @paste="onPaste"
    @focusin="handleFocusChange"
    @focusout="handleFocusChange"
  >
    <div ref="viewport" class="relative min-h-0 w-full flex-1 overflow-hidden bg-black" data-vnc-viewport>
      <div
        v-if="transportBadge || (viewOnly && status === 'connected')"
        class="pointer-events-none absolute top-2 left-2 z-20 flex items-center gap-2"
      >
        <span
          v-if="viewOnly && status === 'connected'"
          class="rounded bg-black/55 px-2 py-0.5 text-[11px] text-white/85"
          data-vnc-viewonly-badge
        >
          {{ t('shell.vnc.viewOnly') }}
        </span>
        <span
          v-if="transportBadge"
          class="rounded px-2 py-0.5 text-[11px]"
          :class="transportBadge.tone"
          data-vnc-transport-badge
        >
          {{ transportBadge.label }}
        </span>
      </div>

      <div v-if="kasmCompatAlert" class="pointer-events-none absolute top-2 right-2 z-20 flex items-center gap-2">
        <span
          class="rounded bg-amber-400/90 px-2 py-0.5 text-[11px] font-medium text-black"
          data-vnc-kasm-compat-degraded
          :title="kasmCompatAlertTitle"
        >
          {{ kasmCompatAlertBadge }}
        </span>
      </div>

      <SessionOverlay
        v-if="reconnecting"
        class="pointer-events-none"
        surface="canvas"
        tone="warning"
        busy
        dismissible
        data-vnc-status="reconnecting"
        :title="t('shell.vnc.reconnecting', { attempt: reconnectAttempt, limit: reconnectLimit })"
        :title-attrs="{ 'data-vnc-reconnecting': '' }"
        :close-label="t('shell.rdp.closePane')"
        @close="emit('close')"
      >
        <div class="pointer-events-auto">
          <QtermButton
            variant="outline"
            class="border-qt-overlay-border bg-qt-overlay-surface text-qt-overlay-fg"
            data-vnc-reconnect-cancel
            @click="emit('close')"
          >
            {{ t('shell.rdp.closePane') }}
          </QtermButton>
        </div>
      </SessionOverlay>

      <SessionOverlay
        v-else-if="status === 'connecting' || status === 'idle'"
        class="pointer-events-none"
        surface="canvas"
        tone="info"
        busy
        data-vnc-status="connecting"
        :title="t('shell.vnc.connecting')"
      >
        <span class="text-[11px] leading-4 text-qt-overlay-muted">{{ t('shell.vnc.connectingHint') }}</span>
      </SessionOverlay>

      <SessionOverlay
        v-else-if="status === 'failed'"
        surface="canvas"
        tone="error"
        data-vnc-status="failed"
        :title="t('shell.vnc.connectionFailed')"
      >
        <span class="text-xs leading-5 break-words text-qt-overlay-muted" data-vnc-failure-reason>
          {{ errorText }}
        </span>

        <span v-if="reconnectExhausted" class="text-xs leading-5 text-amber-200/90" data-vnc-reconnect-exhausted>
          {{ t('shell.terminal.autoReconnectStopped', { count: reconnectLimit }) }}
        </span>
        <template #actions>
          <QtermButton data-vnc-retry @click="reconnect()">
            {{ t('shell.rdp.retryConnect') }}
          </QtermButton>
          <QtermButton
            variant="outline"
            class="border-qt-overlay-border bg-qt-overlay-surface text-qt-overlay-fg"
            @click="emit('close')"
          >
            {{ t('shell.rdp.closePane') }}
          </QtermButton>
        </template>
      </SessionOverlay>

      <SessionOverlay
        v-else-if="status === 'password-required'"
        surface="canvas"
        tone="warning"
        data-vnc-status="password-required"
        :title="t('shell.vnc.passwordRequired')"
      >
        <form class="flex flex-col items-center gap-3" @submit.prevent="submitPassword()">
          <span class="text-xs leading-5 text-qt-overlay-muted">{{ t('shell.vnc.passwordHint') }}</span>
          <QtermInput
            v-model="passwordDraft"
            type="password"
            name="password"
            class="w-full"
            autocomplete="off"
            :aria-label="t('shell.vnc.passwordLabel')"
            data-vnc-password
          />
          <QtermButton type="submit" :disabled="passwordDraft.length === 0" data-vnc-password-submit>
            {{ t('shell.common.confirm') }}
          </QtermButton>
        </form>
      </SessionOverlay>

      <SessionOverlay
        v-else-if="status === 'disconnected'"
        class="pointer-events-none"
        surface="canvas"
        tone="info"
        title-size="sm"
        data-vnc-status="disconnected"
      >
        <span class="text-sm text-qt-overlay-fg">{{ t('shell.vnc.disconnected') }}</span>
      </SessionOverlay>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import { QtermButton, QtermInput, SessionOverlay } from '../../components/common'
import { useI18n } from '@/locales'
import { readClipboardText } from '../../services/clipboard'
import { fetchVncHostProfile } from '../../services/vnc'
import { useVncSession, readVncHostPreferences, VNC_DEFAULT_PREFERENCES } from './useVncSession'
import { createPasteKeyHandler, type PasteKeyEventLike } from './vncClipboardBridge'

const props = defineProps<{
  hostId: string

  hostTitle?: string
}>()

const emit = defineEmits<{ close: [] }>()

const { t } = useI18n()
const viewport = ref<HTMLElement | null>(null)

const transportBadgeState = ref<{ transport: string; allowInsecureTls: boolean }>({
  transport: 'tcp',
  allowInsecureTls: false,
})

function adoptTransportBadge(profile: unknown): void {
  const raw = (profile ?? {}) as { transport?: unknown; allowInsecureTls?: unknown }
  transportBadgeState.value = {
    transport: raw.transport === 'wss' ? 'wss' : 'tcp',
    allowInsecureTls: raw.allowInsecureTls === true,
  }
}

const transportBadge = computed(() => {
  if (transportBadgeState.value.transport !== 'wss') return null
  return transportBadgeState.value.allowInsecureTls
    ? { label: t('shell.vnc.badgeInsecureTls'), tone: 'bg-amber-400/90 font-medium text-black' }
    : { label: t('shell.vnc.badgeWebSocketTls'), tone: 'bg-black/55 text-white/85' }
})

const session = useVncSession({
  hostId: () => props.hostId,

  loadHostPreferences: async (hostId) => {
    try {
      const profile = await fetchVncHostProfile(hostId)

      adoptTransportBadge(profile)
      return {
        ...readVncHostPreferences(profile),

        basicAuthUsed: profile.basicAuthUsed === true || profile.transport === 'wss',
        rfbPasswordRequired: profile.rfbPasswordRequired,
        passwordAvailable: profile.passwordAvailable === true,

        rfbPasswordMode: profile.rfbPasswordMode === 'same' ? 'same' : 'auto',
      }
    } catch {
      adoptTransportBadge(null)
      return { ...VNC_DEFAULT_PREFERENCES }
    }
  },
})
const {
  status,
  errorText,
  viewOnly,
  reconnecting,
  reconnectAttempt,
  reconnectLimit,
  reconnectExhausted,
  kasmCompatAlert,
} = session

const panel = ref<HTMLElement | null>(null)

const kasmCompatAlertBadge = computed(() => 'KasmVNC 剪贴板适配未启用')
const kasmCompatAlertTitle = computed(() => {
  const alert = kasmCompatAlert.value
  if (!alert) return ''
  const identity = alert.software ? `${alert.software}${alert.version ? '/' + alert.version : ''}` : '未知'
  return (
    `面板降级：KasmVNC 二进制剪贴板适配未安装（原因 ${alert.reason}）。\n` +
    `服务端身份：${identity}（判据 ${alert.dialectVerdict}）。\n` +
    `细节：${alert.detail}\n` +
    '影响：远端复制文本时连接可能断开。请核对 noVNC 版本（package.json 中为精确版本）。'
  )
})

const handlePasteKeydown = createPasteKeyHandler({
  readClipboard: readClipboardText,
  send: (text) => session.sendClipboardText(text),
})

async function onKeydown(event: KeyboardEvent): Promise<void> {
  const result = handlePasteKeydown(event as unknown as PasteKeyEventLike)
  if (!result) return
  toastPasteResult(await result)
}

function computeAutoSyncVisible(): boolean {
  if (status.value !== 'connected') return false
  if (viewOnly.value) return false
  if (typeof document !== 'undefined') {
    if (document.visibilityState === 'hidden') return false
    if (typeof document.hasFocus === 'function' && !document.hasFocus()) return false
  }
  const element = panel.value
  if (!element) return false

  return element.offsetParent !== null
}

function syncAutoSyncGate(): void {
  const visible = computeAutoSyncVisible()

  if (typeof session.setClipboardAutoSyncVisible !== 'function') return
  session.setClipboardAutoSyncVisible(visible)
  if (visible) {
    session.startClipboardAutoSync()
  } else {
    session.stopClipboardAutoSync()
  }
}

function handleFocusChange(): void {
  syncAutoSyncGate()
}

function handleDocumentVisibility(): void {
  syncAutoSyncGate()
}

function handleWindowFocus(): void {
  syncAutoSyncGate()
}

watch(status, () => {
  syncAutoSyncGate()
})

watch(viewOnly, () => {
  syncAutoSyncGate()
})

const passwordDraft = ref('')

const fitEnabled = ref(true)

let adoptedHostScale = false

watch(
  () => session.preferences.value,
  (value) => {
    if (adoptedHostScale) return
    adoptedHostScale = true
    fitEnabled.value = value.scale !== 'actual'
    applyFit()
  },
)

function applyFit(): void {
  session.fitToContainer(fitEnabled.value)
}

function toggleFit(): void {
  fitEnabled.value = !fitEnabled.value
  applyFit()
}

async function onPaste(event: ClipboardEvent): Promise<void> {
  const target = event.target as HTMLElement | null
  if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable)) {
    return
  }
  const text = event.clipboardData?.getData('text') ?? ''
  if (!text) return
  event.preventDefault()
  toastPasteResult(await session.sendClipboardText(text))
}

async function pasteToRemote(): Promise<void> {
  toastPasteResult(await session.pasteToRemote())
}

function toastPasteResult(result: string): void {
  switch (result) {
    case 'sent':
      toast.success(t('shell.vnc.pasteSent'))
      return
    case 'empty':
      toast.error(t('shell.vnc.pasteEmpty'))
      return
    case 'read-failed':
      toast.error(t('shell.vnc.pasteReadFailed'))
      return
    case 'view-only':
      toast.warning(t('shell.vnc.pasteViewOnly'))
      return
    case 'send-failed':
    case 'not-connected':
      toast.error(t('shell.vnc.pasteFailed'))
      return
  }
}

function submitPassword(): void {
  if (passwordDraft.value.length === 0) return
  session.submitPassword(passwordDraft.value)
  passwordDraft.value = ''
}

async function connect(): Promise<void> {
  const element = viewport.value
  if (!element) return
  await session.connect(element)
}

async function reconnect(): Promise<void> {
  await session.disconnect()
  await connect()
}

function disconnect(): void {
  void session.disconnect()
}

let panelVisibilityObserver: MutationObserver | null = null

onMounted(() => {
  if (typeof document !== 'undefined') {
    document.addEventListener('visibilitychange', handleDocumentVisibility)
  }
  if (typeof window !== 'undefined') {
    window.addEventListener('focus', handleWindowFocus)
    window.addEventListener('blur', handleWindowFocus)
  }
  if (typeof MutationObserver !== 'undefined' && panel.value) {
    panelVisibilityObserver = new MutationObserver(() => {
      syncAutoSyncGate()
    })
    panelVisibilityObserver.observe(panel.value, { attributes: true, attributeFilter: ['style', 'class'] })
  }
  void connect()
})

onBeforeUnmount(() => {
  session.stopClipboardAutoSync?.()
  panelVisibilityObserver?.disconnect()
  panelVisibilityObserver = null
  if (typeof document !== 'undefined') {
    document.removeEventListener('visibilitychange', handleDocumentVisibility)
  }
  if (typeof window !== 'undefined') {
    window.removeEventListener('focus', handleWindowFocus)
    window.removeEventListener('blur', handleWindowFocus)
  }
})

onBeforeUnmount(() => {
  session.stopClipboardAutoSync?.()
  if (typeof document !== 'undefined') {
    document.removeEventListener('visibilitychange', handleDocumentVisibility)
  }
  if (typeof window !== 'undefined') {
    window.removeEventListener('focus', handleWindowFocus)
    window.removeEventListener('blur', handleWindowFocus)
  }
})

watch(status, (value) => {
  if (value === 'connected') applyFit()
  syncAutoSyncGate()
})

watch(
  () => props.hostId,
  () => {
    adoptedHostScale = false
    adoptTransportBadge(null)
    void reconnect()
  },
)

defineExpose({
  reconnect,
  disconnect,

  pasteToRemote,

  sessionId: session.sessionId,
  statusTone: session.statusTone,

  setViewOnly: session.setViewOnly,

  fitToContainer: session.fitToContainer,

  fitEnabled,

  toggleFit,
})
</script>
