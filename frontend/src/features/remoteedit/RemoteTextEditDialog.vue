<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Loader2 } from '@lucide/vue'
import { toast } from 'vue-sonner'
import { QtermButton, QtermFormMessage } from '../../components/common'
import {
  QtermDialog,
  QtermDialogContent,
  QtermDialogDescription,
  QtermDialogFooter,
  QtermDialogHeader,
  QtermDialogTitle,
} from '../../components/common/dialog'
import { Separator } from '../../components/common/control-primitives'
import { RemoteTextEditor } from '../../components/common/editor'
import { translateErrorKey } from '../../services/errors'
import { useI18n } from '@/locales'
import { detectEditorLanguage } from './fileLanguage'
import RemoteEditConflictDialog from './RemoteEditConflictDialog.vue'
import { openWithLocalProgram, readRemoteContentForDiff, readRemoteText, writeRemoteText } from './useRemoteEdits'
import {
  currentEditorTheme,
  encodeRemoteText,
  refusalLocaleKey,
  type ConflictChoice,
  type RemoteTextRefusal,
} from './remoteEditModel'

const props = defineProps<{
  sessionId: string
  remotePath: string
  name: string
}>()

const emit = defineEmits<{ close: [] }>()

const { t } = useI18n()
const editorRef = ref<InstanceType<typeof RemoteTextEditor> | null>(null)

const readDone = ref(false)

const editorReady = ref(false)
const saving = ref(false)
const draft = ref('')

const baselineHash = ref('')
const dirty = ref(false)
const errorKey = ref('')
const refusal = ref<RemoteTextRefusal>(null)
const fallbackNotice = ref(false)
const savedNotice = ref(false)

const conflictOpen = ref(false)
const conflictBusy = ref(false)
const conflictRepeated = ref(false)
const conflictRemoteContent = ref<string | undefined>(undefined)
const conflictLocalContent = ref<string | undefined>(undefined)
const conflictHash = ref('')

const closeConfirmOpen = ref(false)

const loading = computed(() => {
  if (!readDone.value) return true
  if (refusal.value || errorKey.value) return false
  return !editorReady.value
})

const READY_FALLBACK_MS = 10_000
let readyFallbackTimer: ReturnType<typeof setTimeout> | null = null

function clearReadyFallback(): void {
  if (readyFallbackTimer === null) return
  clearTimeout(readyFallbackTimer)
  readyFallbackTimer = null
}

function onEditorReady(): void {
  clearReadyFallback()
  editorReady.value = true

  void nextTick(() => editorRef.value?.focus())
}

function armReadyFallback(): void {
  clearReadyFallback()
  readyFallbackTimer = setTimeout(() => {
    readyFallbackTimer = null
    editorReady.value = true
  }, READY_FALLBACK_MS)
}

const language = computed(() => detectEditorLanguage(props.name || props.remotePath, draft.value))
const theme = ref(currentEditorTheme())

let themeObserver: MutationObserver | null = null
function observeTheme(): void {
  themeObserver = new MutationObserver(() => {
    theme.value = currentEditorTheme()
  })
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
}

function messageFor(key: string, fallbackKey: string): string {
  if (!key) return t(fallbackKey)
  return translateErrorKey(key, translateErrorKey('error.unknown', t('error.unknown')))
}

async function loadFromRemote(): Promise<void> {
  readDone.value = false
  editorReady.value = false
  clearReadyFallback()
  errorKey.value = ''
  const result = await readRemoteText(props.sessionId, props.remotePath)
  readDone.value = true
  if (!result.ok) {
    if (result.refusal) {
      refusal.value = result.refusal
      return
    }
    errorKey.value = result.errorKey
    return
  }
  refusal.value = null
  draft.value = result.content
  baselineHash.value = result.hash
  dirty.value = false
  savedNotice.value = false

  armReadyFallback()
}

async function refreshFromRemote(): Promise<void> {
  await loadFromRemote()
}

onMounted(() => {
  observeTheme()
  void loadFromRemote()
})

onBeforeUnmount(() => {
  clearReadyFallback()
  themeObserver?.disconnect()
  themeObserver = null
})

watch(
  () => props.remotePath,
  () => void loadFromRemote(),
)

async function save(): Promise<void> {
  if (loading.value || saving.value || refusal.value) return
  saving.value = true
  errorKey.value = ''
  savedNotice.value = false
  const result = await writeRemoteText(
    props.sessionId,
    props.remotePath,
    encodeRemoteText(draft.value),

    baselineHash.value,
  )
  saving.value = false
  if (result.ok) {
    baselineHash.value = result.currentHash
    dirty.value = false
    savedNotice.value = true
    return
  }
  if (result.conflict) {
    conflictHash.value = result.currentHash
    conflictRepeated.value = false
    const remote = await readRemoteContentForDiff(props.sessionId, props.remotePath)
    conflictRemoteContent.value = remote?.content
    conflictLocalContent.value = draft.value
    conflictOpen.value = true
    return
  }
  errorKey.value = result.errorKey
  toast.error(t('remote.ui.toast.saveFailed', { reason: messageFor(result.errorKey, 'error.unknown') }))
}

async function overwrite(): Promise<void> {
  conflictBusy.value = true
  const result = await writeRemoteText(
    props.sessionId,
    props.remotePath,
    encodeRemoteText(draft.value),
    conflictHash.value,
  )
  conflictBusy.value = false
  if (result.ok) {
    baselineHash.value = result.currentHash
    dirty.value = false
    savedNotice.value = true
    conflictOpen.value = false
    return
  }
  if (result.conflict) {
    conflictHash.value = result.currentHash
    conflictRepeated.value = true
    const remote = await readRemoteContentForDiff(props.sessionId, props.remotePath)
    conflictRemoteContent.value = remote?.content
    conflictLocalContent.value = draft.value
    return
  }
  conflictOpen.value = false
  errorKey.value = result.errorKey
}

async function reloadFromRemote(): Promise<void> {
  conflictBusy.value = true
  const remote = await readRemoteContentForDiff(props.sessionId, props.remotePath)
  conflictBusy.value = false
  if (!remote) {
    conflictOpen.value = false
    errorKey.value = 'remote.edit_read_failed'
    return
  }
  draft.value = remote.content
  baselineHash.value = remote.hash
  dirty.value = false
  savedNotice.value = false
  conflictOpen.value = false
}

function onConflictChoice(choice: ConflictChoice): void {
  if (choice === 'reload') void reloadFromRemote()
  else if (choice === 'overwrite') void overwrite()
  else conflictOpen.value = false
}

function requestClose(): void {
  if (dirty.value || conflictOpen.value) {
    closeConfirmOpen.value = true
    return
  }
  emit('close')
}

function confirmClose(): void {
  closeConfirmOpen.value = false
  emit('close')
}

async function openWithLocalProgramFromDialog(): Promise<void> {
  const result = await openWithLocalProgram(props.sessionId, props.remotePath)
  if (result.ok) {
    toast.success(t('remote.ui.toast.openedLocal', { path: props.name }))
    emit('close')
    return
  }
  toast.error(t('remote.ui.toast.openFailed', { reason: messageFor(result.errorKey, 'error.unknown') }))
}

const refusalMessage = computed(() => (refusal.value ? t(refusalLocaleKey(refusal.value)) : ''))
</script>

<template>
  <QtermDialog :open="true" @update:open="(open) => !open && requestClose()">
    <QtermDialogContent
      class="remote-text-edit-dialog z-60 grid-cols-1"
      overlay-class="z-60"
      size="3xl"
      :close-on-escape="!saving"
      @pointer-down-outside.prevent
    >
      <header class="dialog-header relative" data-slot="dialog-header">
        <Separator class="dialog-header-separator" />
        <div class="dialog-header-content flex flex-col">
          <QtermDialogTitle data-testid="remote-edit-title">{{ props.name }}</QtermDialogTitle>

          <QtermDialogDescription class="truncate" :title="props.remotePath" data-testid="remote-edit-path">
            {{ props.remotePath }}
          </QtermDialogDescription>
        </div>
        <QtermButton
          variant="outline"
          :disabled="saving || loading || Boolean(refusal)"
          :title="t('remote.ui.editor.refreshHint')"
          data-testid="remote-edit-refresh"
          @click="refreshFromRemote"
        >
          {{ t('remote.ui.editor.refresh') }}
        </QtermButton>
        <QtermButton variant="outline" :disabled="saving" data-testid="remote-edit-close" @click="requestClose">
          {{ t('remote.ui.editor.close') }}
        </QtermButton>
        <QtermButton
          :disabled="saving || loading || Boolean(refusal)"
          :pressed="saving"
          :aria-busy="saving"
          data-testid="remote-edit-save"
          @click="save"
        >
          {{ t('remote.ui.editor.save') }}
        </QtermButton>
      </header>
      <div class="dialog-body flex-col">
        <template v-if="refusal">
          <p class="text-xs text-qt-muted" role="alert">{{ refusalMessage }}</p>
          <div class="flex justify-end gap-2">
            <QtermButton variant="outline" @click="emit('close')">{{ t('sftp.ui.cancel') }}</QtermButton>
            <QtermButton data-testid="remote-edit-open-local" @click="openWithLocalProgramFromDialog">
              {{ t('remote.ui.openWithLocal') }}
            </QtermButton>
          </div>
        </template>

        <template v-else>
          <QtermFormMessage v-if="errorKey" variant="error">
            {{ messageFor(errorKey, 'remote.edit_read_failed') }}
          </QtermFormMessage>
          <p v-if="fallbackNotice" class="text-xs text-qt-muted" role="status">
            {{ t('remote.ui.editor.fallback') }}
          </p>

          <div
            class="remote-text-edit-host relative h-[58vh] min-h-0 overflow-hidden rounded-md border-d"
            data-testid="remote-edit-editor-host"
          >
            <RemoteTextEditor
              v-if="readDone"
              ref="editorRef"
              v-model="draft"
              :language="language"
              :theme="theme"
              @dirty-change="dirty = $event"
              @engine-fallback="fallbackNotice = true"
              @ready="onEditorReady"
            />

            <div
              v-if="loading"
              class="absolute inset-0 flex-cc bg-qt-app"
              role="status"
              data-testid="remote-edit-loading"
            >
              <div class="flex items-center gap-2 text-qt-muted">
                <Loader2 :size="18" class="spin" aria-hidden="true" />
                <span class="text-xs">{{ t('remote.ui.editor.loading') }}</span>
              </div>
            </div>
          </div>

          <div v-if="!loading" class="flex items-center gap-3 text-xs">
            <span class="text-qt-muted" role="status" data-testid="remote-edit-dirty-state">
              {{ dirty ? t('remote.ui.editor.dirty') : t('remote.ui.editor.clean') }}
            </span>
            <span v-if="savedNotice && !dirty" class="text-qt-success">{{ t('remote.ui.editor.saved') }}</span>
          </div>
        </template>
      </div>
    </QtermDialogContent>
  </QtermDialog>

  <RemoteEditConflictDialog
    :open="conflictOpen"
    kind="in-app"
    :remote-path="props.remotePath"
    :remote-content="conflictRemoteContent"
    :local-content="conflictLocalContent"
    :busy="conflictBusy"
    :repeated="conflictRepeated"
    @choose="onConflictChoice"
  />

  <QtermDialog :open="closeConfirmOpen" @update:open="(open) => !open && (closeConfirmOpen = false)">
    <QtermDialogContent
      v-if="closeConfirmOpen"
      class="z-70"
      overlay-class="z-70"
      size="sm"
      @pointer-down-outside.prevent
    >
      <QtermDialogHeader :title="t('remote.ui.editor.closeDirtyTitle')" :show-close="false" />
      <div class="dialog-body">
        <p class="text-xs text-qt-muted">{{ t('remote.ui.editor.closeDirtyMessage') }}</p>
      </div>
      <QtermDialogFooter class="flex justify-end gap-2">
        <QtermButton variant="outline" @click="closeConfirmOpen = false">{{ t('sftp.ui.cancel') }}</QtermButton>
        <QtermButton variant="destructive" data-testid="remote-edit-close-confirm" @click="confirmClose">
          {{ t('remote.ui.editor.closeDirtyConfirm') }}
        </QtermButton>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>

<style scoped>
.remote-text-edit-dialog {
  max-height: var(--qt-viewport-content-height-even, 768px);
}
</style>
