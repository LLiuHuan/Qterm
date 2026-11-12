<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { CircleAlert, Layers, LogIn, LogOut, Pencil, Plus, RefreshCw, Trash2 } from '@lucide/vue'
import {
  IconButton,
  QtermButton,
  QtermCheckbox,
  QtermEmptyState,
  QtermInput,
  RefreshIconButton,
} from '../../components/common'
import { ScrollArea } from '../../components/common/control-primitives'
import DeleteConfirmDialog from '../../components/dialogs/DeleteConfirmDialog.vue'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { useI18n } from '../../locales'
import { decodeAppError, errorMessage } from '../../services/errors'
import { sendTerminalInput } from '../../services/terminal'
import type { TmuxGateway, TmuxSession } from '../../services/tmux'
import { beginTmuxEnter, cancelTmuxEnter, getTmuxRuntimeState, markTmuxExited } from '../../services/tmuxRuntime'
import { toast } from 'vue-sonner'

const props = defineProps<{
  gateway: TmuxGateway
  sessionId: string

  connected: boolean
}>()

const emit = defineEmits<{ focusTerminal: [] }>()

const { t } = useI18n()

const sessions = ref<TmuxSession[]>([])
const loading = ref(false)
const busy = ref(false)

const unavailable = ref(false)
const errorText = ref('')
const tmuxVersion = ref('')
const creating = ref(false)
const newName = ref('')

const enterAfterCreate = ref(false)
const createError = ref('')
const createInput = ref<{ $el?: HTMLElement } | null>(null)
const renaming = ref<TmuxSession | null>(null)
const renameName = ref('')
const renameError = ref('')
const renameInput = ref<{ $el?: HTMLElement } | null>(null)
const pendingDelete = ref<TmuxSession | null>(null)
const deleteError = ref('')

let requestSeq = 0
let versionRequestSeq = 0
let generation = 0
let disposed = false

const POLL_INTERVAL_MS = 5000
let pollTimer: ReturnType<typeof setInterval> | null = null

const canRefresh = computed(() => Boolean(props.sessionId) && props.connected && !busy.value && !loading.value)
const canOperate = computed(() => canRefresh.value && !unavailable.value && !errorText.value)
const runtimeState = computed(() => getTmuxRuntimeState(props.sessionId))
const canEnter = computed(() => canOperate.value && runtimeState.value.phase !== 'pending')
const canDetach = computed(() => canOperate.value && runtimeState.value.phase === 'active')

function isCurrent(sessionId: string, operationGeneration: number): boolean {
  return !disposed && generation === operationGeneration && props.sessionId === sessionId && props.connected
}

function clearContext(): void {
  generation += 1
  requestSeq += 1
  versionRequestSeq += 1
  sessions.value = []
  loading.value = false
  busy.value = false
  unavailable.value = false
  errorText.value = ''
  tmuxVersion.value = ''
  creating.value = false
  newName.value = ''
  enterAfterCreate.value = false
  createError.value = ''
  renaming.value = null
  renameName.value = ''
  renameError.value = ''
  pendingDelete.value = null
  deleteError.value = ''
}

async function refreshVersion(): Promise<void> {
  const sessionId = props.sessionId
  const operationGeneration = generation
  const seq = ++versionRequestSeq
  tmuxVersion.value = ''
  if (!props.connected || !sessionId || !props.gateway.version) return
  try {
    const version = await props.gateway.version(sessionId)
    if (!isCurrent(sessionId, operationGeneration) || seq !== versionRequestSeq) return
    tmuxVersion.value = normalizeVersion(version)
  } catch {
    if (isCurrent(sessionId, operationGeneration) && seq === versionRequestSeq) tmuxVersion.value = ''
  }
}

function normalizeVersion(raw: string): string {
  let value = raw.trim()
  while (value.toLowerCase().startsWith('tmux ')) value = value.slice('tmux '.length).trim()
  return value ? `tmux ${value}` : ''
}

async function refresh(): Promise<void> {
  if (!props.connected || !props.sessionId) return

  const sessionId = props.sessionId
  const operationGeneration = generation
  const seq = ++requestSeq
  loading.value = true

  sessions.value = []
  unavailable.value = false
  errorText.value = ''

  try {
    const rows = await props.gateway.list(sessionId)
    if (!isCurrent(sessionId, operationGeneration) || seq !== requestSeq) return
    sessions.value = rows
  } catch (error) {
    if (!isCurrent(sessionId, operationGeneration) || seq !== requestSeq) return
    sessions.value = []
    if (decodeAppError(error)?.messageKey === 'tmux.unavailable') {
      unavailable.value = true
      errorText.value = ''
    } else {
      unavailable.value = false
      errorText.value = errorMessage(error, t('shell.tmux.loadFailed'))
    }
  } finally {
    if (isCurrent(sessionId, operationGeneration) && seq === requestSeq) loading.value = false
  }
}

function sessionsDiffer(next: TmuxSession[]): boolean {
  const current = sessions.value
  if (next.length !== current.length) return true
  return next.some((row, index) => {
    const prev = current[index]
    return (
      row.name !== prev.name ||
      row.windows !== prev.windows ||
      row.attached !== prev.attached ||
      row.createdAt !== prev.createdAt
    )
  })
}

async function pollSessions(): Promise<void> {
  if (disposed || !props.connected || !props.sessionId) return

  if (busy.value || loading.value) return

  if (creating.value || renaming.value || pendingDelete.value) return
  if (typeof document !== 'undefined' && document.hidden) return

  const sessionId = props.sessionId
  const operationGeneration = generation
  try {
    const rows = await props.gateway.list(sessionId)
    if (!isCurrent(sessionId, operationGeneration)) return

    unavailable.value = false
    errorText.value = ''
    if (sessionsDiffer(rows)) sessions.value = rows
  } catch {}
}

function startPolling(): void {
  stopPolling()
  pollTimer = setInterval(() => {
    void pollSessions()
  }, POLL_INTERVAL_MS)
}

function stopPolling(): void {
  if (pollTimer === null) return
  clearInterval(pollTimer)
  pollTimer = null
}

function handleVisibilityChange(): void {
  if (typeof document !== 'undefined' && !document.hidden) void pollSessions()
}

async function focusInput(getTarget: () => { $el?: HTMLElement } | null, selector: string): Promise<void> {
  await nextTick()
  const element = getTarget()?.$el
  const input =
    element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement
      ? element
      : element?.querySelector<HTMLInputElement | HTMLTextAreaElement>('input,textarea')
  ;(input ?? document.querySelector<HTMLInputElement | HTMLTextAreaElement>(selector))?.focus()
}

async function focusCreateInput(): Promise<void> {
  await focusInput(() => createInput.value, 'input[name="tmux-session-name"]')
}

async function toggleCreate(): Promise<void> {
  if (!canOperate.value || busy.value) return
  creating.value = !creating.value
  createError.value = ''
  if (creating.value) await focusCreateInput()
  else newName.value = ''
}

function cancelCreate(): void {
  if (busy.value) return
  creating.value = false
  newName.value = ''
  enterAfterCreate.value = false
  createError.value = ''
}

async function createSession(): Promise<void> {
  if (!canOperate.value || busy.value) return

  const name = newName.value.trim()
  if (!/^[A-Za-z0-9_-]{1,64}$/.test(name)) {
    createError.value = t('shell.tmux.invalidName')
    return
  }

  const sessionId = props.sessionId
  const operationGeneration = generation
  const shouldEnter = enterAfterCreate.value
  busy.value = true
  createError.value = ''
  try {
    await props.gateway.create(sessionId, name)
    if (!isCurrent(sessionId, operationGeneration)) return
    creating.value = false
    newName.value = ''

    toast.success(t('shell.tmux.created', { name }))
    if (shouldEnter && isCurrent(sessionId, operationGeneration)) {
      await writeAttachCommand(sessionId, name, operationGeneration)
    }
    if (isCurrent(sessionId, operationGeneration)) await refresh()
  } catch (error) {
    if (isCurrent(sessionId, operationGeneration)) {
      createError.value = errorMessage(error, t('shell.tmux.createFailed'))
    }
  } finally {
    if (isCurrent(sessionId, operationGeneration)) busy.value = false
  }
}

async function openRename(session: TmuxSession): Promise<void> {
  if (!canOperate.value || busy.value) return
  renaming.value = session
  renameName.value = session.name
  renameError.value = ''
  await focusInput(() => renameInput.value, '[data-tmux-rename-dialog] input')
}

function closeRename(): void {
  if (busy.value) return
  renaming.value = null
  renameName.value = ''
  renameError.value = ''
}

async function renameSession(): Promise<void> {
  const target = renaming.value
  if (!target || !canOperate.value || busy.value) return

  const nextName = renameName.value.trim()
  if (!/^[A-Za-z0-9_-]{1,64}$/.test(nextName)) {
    renameError.value = t('shell.tmux.invalidName')
    return
  }
  if (nextName === target.name) {
    closeRename()
    return
  }

  const sessionId = props.sessionId
  const operationGeneration = generation
  busy.value = true
  renameError.value = ''
  try {
    await props.gateway.rename(sessionId, target.name, nextName)
    if (!isCurrent(sessionId, operationGeneration)) return
    renaming.value = null
    renameName.value = ''
    toast.success(t('shell.tmux.renamed', { name: nextName }))
    await refresh()
  } catch (error) {
    if (isCurrent(sessionId, operationGeneration)) {
      renameError.value = errorMessage(error, t('shell.tmux.renameFailed'))
    }
  } finally {
    if (isCurrent(sessionId, operationGeneration)) busy.value = false
  }
}

function requestKill(session: TmuxSession): void {
  if (!canOperate.value || busy.value) return
  pendingDelete.value = session
  deleteError.value = ''
}

function closeKill(): void {
  if (busy.value) return
  pendingDelete.value = null
  deleteError.value = ''
}

async function confirmKill(): Promise<void> {
  const target = pendingDelete.value
  if (!target || !canOperate.value || busy.value) return

  const sessionId = props.sessionId
  const operationGeneration = generation
  busy.value = true
  deleteError.value = ''
  try {
    await props.gateway.kill(sessionId, target.name)
    if (!isCurrent(sessionId, operationGeneration)) return
    pendingDelete.value = null
    toast.success(t('shell.tmux.killed', { name: target.name }))
    await refresh()
  } catch (error) {
    if (isCurrent(sessionId, operationGeneration)) {
      deleteError.value = errorMessage(error, t('shell.tmux.killFailed'))
    }
  } finally {
    if (isCurrent(sessionId, operationGeneration)) busy.value = false
  }
}

async function writeAttachCommand(sessionId: string, name: string, operationGeneration: number): Promise<void> {
  const currentRuntime = getTmuxRuntimeState(sessionId)
  const trackingToken = currentRuntime.phase === 'active' ? '' : beginTmuxEnter(sessionId, name)
  try {
    const command = await props.gateway.attachCommand(name, trackingToken)
    if (!isCurrent(sessionId, operationGeneration)) {
      if (trackingToken) cancelTmuxEnter(sessionId, trackingToken)
      return
    }
    if (!sendTerminalInput(sessionId, `${command}\r`)) {
      if (trackingToken) cancelTmuxEnter(sessionId, trackingToken)
      toast.error(t('shell.tmux.notConnected'))
      return
    }
    toast.success(t('shell.tmux.attached', { name }))

    emit('focusTerminal')
  } catch (error) {
    if (trackingToken) cancelTmuxEnter(sessionId, trackingToken)
    if (isCurrent(sessionId, operationGeneration)) {
      toast.error(errorMessage(error, t('shell.tmux.attachFailed')))
    }
  }
}

async function detachSession(): Promise<void> {
  if (!canDetach.value || busy.value) return

  const sessionId = props.sessionId
  const operationGeneration = generation
  const runtimeToken = runtimeState.value.token
  busy.value = true
  try {
    const keys = await props.gateway.detachKeys(sessionId)
    const currentRuntime = getTmuxRuntimeState(sessionId)
    if (
      !isCurrent(sessionId, operationGeneration) ||
      currentRuntime.phase !== 'active' ||
      currentRuntime.token !== runtimeToken
    )
      return
    markTmuxExited(sessionId)

    if (!keys || !sendTerminalInput(sessionId, keys)) {
      toast.error(t('shell.tmux.notConnected'))
      return
    }
    toast.success(t('shell.tmux.detached'))

    emit('focusTerminal')
  } catch (error) {
    if (isCurrent(sessionId, operationGeneration)) {
      toast.error(errorMessage(error, t('shell.tmux.detachFailed')))
    }
  } finally {
    if (isCurrent(sessionId, operationGeneration)) busy.value = false
  }
}

async function attachSession(name: string): Promise<void> {
  if (!canEnter.value || busy.value) return

  const sessionId = props.sessionId
  const operationGeneration = generation
  busy.value = true
  try {
    await writeAttachCommand(sessionId, name, operationGeneration)
  } finally {
    if (isCurrent(sessionId, operationGeneration)) busy.value = false
  }
}

function formatCreatedAt(seconds: number): string {
  if (!seconds) return '—'

  return new Date(seconds * 1000).toLocaleString(undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

onMounted(() => {
  void refresh()
  void refreshVersion()
  startPolling()
  if (typeof document !== 'undefined') document.addEventListener('visibilitychange', handleVisibilityChange)
})

watch(
  () => [props.sessionId, props.connected] as const,
  () => {
    clearContext()
    if (props.connected) {
      void refresh()
      void refreshVersion()
    }
  },
)

onBeforeUnmount(() => {
  disposed = true
  generation += 1
  requestSeq += 1
  stopPolling()
  if (typeof document !== 'undefined') document.removeEventListener('visibilitychange', handleVisibilityChange)
})
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col gap-2 p-2 text-sm" data-tmux-panel>
    <div class="flex items-center justify-between gap-2" data-tmux-toolbar>
      <p class="m-0 min-w-0 flex-1 text-xs leading-5 text-qt-muted" data-tmux-version>
        {{ tmuxVersion ? t('shell.tmux.version', { version: tmuxVersion }) : t('shell.tmux.versionUnknown') }}
      </p>
      <div class="flex shrink-0 items-center gap-1">
        <RefreshIconButton
          :label="t('shell.tmux.refresh')"
          :spinning="loading"
          :disabled="!canRefresh"
          data-tmux-refresh
          @click="refresh"
        />
        <IconButton
          size="icon-sm"
          :label="t('shell.tmux.create')"
          :disabled="!canOperate"
          data-tmux-new
          @click="toggleCreate"
        >
          <Plus class="size-3.5" aria-hidden="true" />
        </IconButton>

        <IconButton
          v-if="canDetach"
          size="icon-sm"
          :label="t('shell.tmux.disconnect')"
          data-tmux-detach
          @click="detachSession"
        >
          <LogOut class="size-3.5" aria-hidden="true" />
        </IconButton>
      </div>
    </div>

    <div v-if="creating" class="grid gap-1" data-tmux-create-row>
      <div class="flex items-center gap-2">
        <QtermInput
          ref="createInput"
          v-model="newName"
          class="min-w-0 flex-1"
          name="tmux-session-name"
          autocomplete="off"
          autocorrect="off"
          autocapitalize="off"
          spellcheck="false"
          :aria-label="t('shell.tmux.namePlaceholder')"
          :aria-invalid="Boolean(createError)"
          :placeholder="t('shell.tmux.namePlaceholder')"
          :disabled="busy"
          @keydown.enter.prevent="createSession"
          @keydown.esc.prevent="cancelCreate"
        />
        <QtermButton
          size="sm"
          class="h-7! min-h-7! shrink-0 text-xs"
          :disabled="!canOperate || busy || !newName.trim()"
          @click="createSession"
        >
          {{ t('shell.tmux.confirmCreate') }}
        </QtermButton>
      </div>
      <label class="flex w-fit cursor-pointer items-center gap-1.5 text-xs text-qt-muted">
        <QtermCheckbox
          :model-value="enterAfterCreate"
          :disabled="busy"
          data-tmux-create-enter
          @update:model-value="enterAfterCreate = $event === true"
        />
        {{ t('shell.tmux.enterAfterCreate') }}
      </label>
      <p v-if="createError" class="m-0 text-xs text-qt-danger" role="alert">{{ createError }}</p>
    </div>

    <ScrollArea
      v-if="!connected || loading || errorText || unavailable || !sessions.length"
      class="min-h-0 flex-1"
      type="hover"
      :scroll-hide-delay="600"
    >
      <QtermEmptyState
        v-if="!connected"
        variant="outlined"
        role="status"
        :icon="CircleAlert"
        :description="t('shell.tmux.notConnected')"
      />
      <QtermEmptyState
        v-else-if="loading"
        variant="outlined"
        role="status"
        busy
        :icon="RefreshCw"
        icon-class="animate-spin"
        :title="t('shell.tmux.loading')"
      />
      <QtermEmptyState
        v-else-if="errorText"
        variant="outlined"
        role="alert"
        :icon="CircleAlert"
        :title="t('shell.tmux.loadFailed')"
        :description="errorText"
      />
      <QtermEmptyState
        v-else-if="unavailable"
        variant="outlined"
        role="status"
        :icon="CircleAlert"
        :description="t('shell.tmux.unavailable')"
      />
      <QtermEmptyState
        v-else
        variant="outlined"
        :icon="Layers"
        :title="t('shell.tmux.empty')"
        :description="t('shell.tmux.emptyHint')"
      />
    </ScrollArea>
    <ScrollArea v-else class="min-h-0 min-w-0 flex-1" type="hover" :scroll-hide-delay="600" data-tmux-list>
      <div class="grid min-w-0 gap-2">
        <div
          v-for="item in sessions"
          :key="item.name"
          class="flex min-w-0 items-start gap-2 rounded-md border border-qt-border/70 px-2 py-2"
          :data-tmux-session="item.name"
        >
          <Layers class="mt-0.5 size-3.5 shrink-0 text-qt-muted" aria-hidden="true" />
          <div class="min-w-0 flex-1">
            <div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
              <span class="min-w-0 break-all font-mono text-xs text-qt-text" :title="item.name">{{ item.name }}</span>
              <span
                v-if="item.attached"
                class="shrink-0 rounded-full border border-qt-success/40 px-1.5 py-0.5 text-xs leading-4 text-qt-success"
              >
                {{ t('shell.tmux.attachedBadge') }}
              </span>
            </div>
            <div class="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 text-xs text-qt-muted">
              <span>{{ t('shell.tmux.windows', { count: item.windows }) }}</span>
              <span>{{ formatCreatedAt(item.createdAt) }}</span>
            </div>
          </div>
          <div class="flex shrink-0 items-center gap-1">
            <IconButton
              size="icon-sm"
              :label="t('shell.tmux.enter')"
              :disabled="!canEnter"
              :data-tmux-enter="item.name"
              :data-tmux-attach="item.name"
              @click="attachSession(item.name)"
            >
              <LogIn class="size-3.5" aria-hidden="true" />
            </IconButton>
            <IconButton
              size="icon-sm"
              :label="t('shell.tmux.rename')"
              :disabled="!canOperate"
              :data-tmux-rename="item.name"
              @click="openRename(item)"
            >
              <Pencil class="size-3.5" aria-hidden="true" />
            </IconButton>
            <IconButton
              size="icon-sm"
              danger
              :label="t('shell.tmux.destroy')"
              :disabled="!canOperate"
              :data-tmux-destroy="item.name"
              :data-tmux-kill="item.name"
              @click="requestKill(item)"
            >
              <Trash2 class="size-3.5" aria-hidden="true" />
            </IconButton>
          </div>
        </div>
      </div>
    </ScrollArea>

    <QtermDialog :open="renaming !== null" @update:open="(open) => !open && closeRename()">
      <QtermDialogContent v-if="renaming" data-tmux-rename-dialog>
        <QtermDialogHeader :title="t('shell.tmux.renameTitle')" :show-close="false" @close="closeRename" />
        <div class="dialog-body">
          <label class="flex flex-col gap-1">
            <span class="text-xs text-qt-muted">{{ t('shell.tmux.namePlaceholder') }}</span>
            <QtermInput
              ref="renameInput"
              v-model="renameName"
              name="tmux-rename-session"
              autocomplete="off"
              autocorrect="off"
              autocapitalize="off"
              spellcheck="false"
              :aria-label="t('shell.tmux.namePlaceholder')"
              :aria-invalid="Boolean(renameError)"
              :placeholder="t('shell.tmux.namePlaceholder')"
              :disabled="busy"
              @keydown.enter.prevent="renameSession"
              @keydown.esc.prevent="closeRename"
            />
          </label>
          <p v-if="renameError" class="m-0 text-xs text-qt-danger" role="alert">{{ renameError }}</p>
        </div>
        <QtermDialogFooter>
          <div></div>
          <div class="dialog-footer-actions flex items-center gap-[8px]">
            <QtermButton variant="outline" :disabled="busy" data-tmux-rename-cancel @click="closeRename">
              {{ t('shell.common.cancel') }}
            </QtermButton>
            <QtermButton :disabled="busy || !renameName.trim()" data-tmux-rename-submit @click="renameSession">
              {{ t('shell.common.confirm') }}
            </QtermButton>
          </div>
        </QtermDialogFooter>
      </QtermDialogContent>
    </QtermDialog>

    <DeleteConfirmDialog
      :open="pendingDelete !== null"
      :title="t('shell.tmux.destroyTitle')"
      :message="t('shell.tmux.destroyMessage', { name: pendingDelete?.name ?? '' })"
      :hint="t('shell.tmux.destroyHint')"
      :error="deleteError"
      :loading="busy"
      :confirm-text="t('shell.tmux.confirmDestroy')"
      :confirming-text="t('shell.tmux.destroying')"
      @close="closeKill"
      @confirm="confirmKill"
    />
  </div>
</template>
