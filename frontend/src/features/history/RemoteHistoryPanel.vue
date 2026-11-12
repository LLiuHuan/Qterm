<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { CircleAlert, Copy, FileClock, History, Loader2, Plus, SearchX } from '@lucide/vue'
import { QtermButton, QtermEmptyState, QtermSearchField, RefreshIconButton } from '@/components/common'
import { ScrollArea } from '@/components/common/control-primitives'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
} from '@/components/common/overlay-controls'
import { useI18n } from '@/locales'
import { toast } from 'vue-sonner'
import { errorMessage } from '@/services/errors'
import { writeClipboardText } from '@/services/clipboard'
import {
  createRemoteHistoryGateway,
  filterEntries,
  sortEntriesForDisplay,
  type RemoteHistoryEntry,
  type RemoteHistoryGateway,
} from '@/services/remoteHistory'
import { runCommandInSession } from '@/features/sessions/terminalInput'

const props = defineProps<{
  sessionId: string
  sessionState: string
  hostName?: string

  gateway?: RemoteHistoryGateway
}>()

const emit = defineEmits<{ createSnippet: [content: string] }>()

const { t } = useI18n()
const gateway = props.gateway ?? createRemoteHistoryGateway()

const loading = ref(false)
const errorText = ref('')
const entries = ref<RemoteHistoryEntry[]>([])
const truncated = ref(false)
const home = ref('')
const query = ref('')
const loaded = ref(false)

const connected = computed(() => props.sessionState === 'connected')
const visibleEntries = computed(() => filterEntries(sortEntriesForDisplay(entries.value), query.value))

async function load(): Promise<void> {
  if (!props.sessionId) {
    entries.value = []
    loaded.value = false
    return
  }
  loading.value = true
  errorText.value = ''
  try {
    const result = await gateway.listRemoteHistory(props.sessionId)
    entries.value = result.entries
    truncated.value = result.truncated
    home.value = result.home ?? ''
    loaded.value = true
  } catch (cause) {
    entries.value = []
    loaded.value = false
    errorText.value = errorMessage(cause, t('history.remote.loadFailed'))
  } finally {
    loading.value = false
  }
}

function isRunnable(entry: RemoteHistoryEntry): boolean {
  return !entry.command.includes('\n') && !entry.command.includes('\r')
}

function execute(entry: RemoteHistoryEntry): void {
  if (!connected.value) {
    toast.warning(t('history.remote.noSession'))
    return
  }
  if (!isRunnable(entry)) {
    toast.warning(t('history.remote.executeMultiLine'))
    return
  }
  if (!runCommandInSession(props.sessionId, entry.command)) {
    toast.warning(t('history.remote.executeFailed'))
  }
}

function saveAsSnippet(entry: RemoteHistoryEntry): void {
  emit('createSnippet', entry.command)
}

async function copyCommand(entry: RemoteHistoryEntry): Promise<void> {
  try {
    await writeClipboardText(entry.command)
    toast.success(t('history.remote.copied'))
  } catch {
    toast.error(t('history.remote.copyFailed'))
  }
}

function shellLabel(shell: string): string {
  return shell ? shell.toUpperCase() : t('history.remote.unknownShell')
}

function entryTime(entry: RemoteHistoryEntry): string {
  if (!entry.timestamp) return ''
  const date = new Date(entry.timestamp * 1000)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleString()
}

function entryTitle(entry: RemoteHistoryEntry): string {
  const parts = [entry.command.trim()]
  const meta = [shellLabel(entry.shell), entry.source, entryTime(entry)].filter(Boolean)
  if (meta.length) parts.push(meta.join(' · '))
  return parts.join('\n')
}

onMounted(load)
watch(() => props.sessionId, load)
</script>

<template>
  <div
    class="command-history-panel flex w-full min-w-0 flex-col h-full min-h-0 pt-2 pb-2 bg-(--qt-glass-surface,var(--qt-bar,var(--qt-panel)))"
  >
    <div class="command-history-fixed grid flex-none gap-3 px-2 pb-2">
      <p class="text-xs text-qt-muted" data-remote-history-hint>
        {{ t('history.remote.hint') }}
        <span v-if="home" class="font-mono">{{ home }}</span>
      </p>
      <QtermSearchField
        v-model="query"
        :placeholder="t('history.remote.search')"
        v-bind="{ ariaLabel: t('history.remote.search') }"
      />

      <div class="command-history-toolbar text-xs flex-cb gap-[0.5rem] text-qt-muted">
        <small class="text-xs">{{ t('history.remote.count', { count: visibleEntries.length }) }}</small>
        <div class="flex items-center gap-1">
          <small v-if="truncated" class="text-xs text-qt-warning" data-remote-history-truncated>
            {{ t('history.remote.truncated', { count: entries.length }) }}
          </small>
          <RefreshIconButton
            size="icon"
            :label="t('history.remote.refresh')"
            :spinning="loading"
            :disabled="loading || !sessionId"
            data-remote-history-refresh
            @click="load"
          />
        </div>
      </div>
    </div>

    <ScrollArea
      v-if="!sessionId || (loading && !loaded) || errorText || !visibleEntries.length"
      class="command-history-scroll min-h-0 flex-1"
    >
      <div class="grid gap-2 px-2 pb-2">
        <QtermEmptyState
          v-if="!sessionId"
          variant="outlined"
          :icon="FileClock"
          :title="t('history.remote.noSessionTitle')"
          :description="t('history.remote.noSession')"
        />

        <QtermEmptyState
          v-else-if="loading && !loaded"
          variant="outlined"
          role="status"
          busy
          :icon="Loader2"
          icon-class="spin"
          :title="t('history.remote.loading')"
          :description="t('history.remote.loadingHint')"
        />
        <template v-else-if="errorText">
          <QtermEmptyState
            variant="outlined"
            role="alert"
            :icon="CircleAlert"
            :title="t('history.remote.loadFailed')"
            :description="errorText"
          />
          <QtermButton class="justify-self-start" size="sm" variant="ghost" @click="load">
            {{ t('history.remote.retry') }}
          </QtermButton>
        </template>

        <QtermEmptyState
          v-else-if="query.trim()"
          variant="outlined"
          :icon="SearchX"
          :title="t('history.remote.noMatch')"
          :description="t('history.remote.noMatchHint')"
        />
        <QtermEmptyState
          v-else
          variant="outlined"
          :icon="History"
          :title="t('history.remote.empty')"
          :description="t('history.remote.emptyHint')"
        />
      </div>
    </ScrollArea>

    <div v-else class="command-history-list relative ml-2 mr-0 mb-2 flex min-h-0 flex-[1_1_auto]">
      <ScrollArea class="command-history-scroll min-h-0 w-full flex-1">
        <div class="flex w-full min-w-0 flex-col space-y-0.5 py-1 pl-2 pr-4">
          <ContextMenu v-for="(entry, index) in visibleEntries" :key="`${entry.source}-${index}-${entry.command}`">
            <ContextMenuTrigger as-child>
              <div
                class="command-history-item c-p flex w-full min-w-0 items-center gap-1.5 min-h-[32px] rounded-md px-1.5 py-2"
                data-remote-history-row
                @dblclick="execute(entry)"
              >
                <span
                  class="flex-none rounded bg-qt-hover px-1 py-0.5 text-[10px] uppercase text-qt-muted"
                  data-remote-history-shell
                  >{{ shellLabel(entry.shell) }}</span
                >
                <p
                  class="command-history-text min-w-0 flex-1 text-xs"
                  :title="entryTitle(entry)"
                  data-remote-history-command
                >
                  {{ entry.command }}
                </p>

                <small
                  v-if="entryTime(entry)"
                  class="command-history-time flex-none whitespace-nowrap text-xs text-qt-muted"
                  data-remote-history-time
                  >{{ entryTime(entry) }}</small
                >
              </div>
            </ContextMenuTrigger>

            <ContextMenuContent>
              <ContextMenuItem @select="saveAsSnippet(entry)">
                <Plus :size="14" />{{ t('shell.toolRail.snippets') }}
              </ContextMenuItem>
              <ContextMenuItem @select="copyCommand(entry)">
                <Copy :size="14" />{{ t('history.remote.copy') }}
              </ContextMenuItem>
            </ContextMenuContent>
          </ContextMenu>
        </div>
      </ScrollArea>
      <span
        class="pointer-events-none absolute inset-y-0 left-0 right-2 rounded-[8px] border border-qt-border"
        aria-hidden="true"
      />
    </div>
  </div>
</template>
