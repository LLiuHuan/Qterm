<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { MessageSquare, Search } from '@lucide/vue'
import { QtermButton, QtermEmptyState, QtermInput } from '../../components/common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { useI18n } from '../../locales'
import type { ArchivedChat } from '../../services/aiProviders'

const props = defineProps<{
  open: boolean
  chats: ArchivedChat[]
  activeId: string

  providerLabel: string
}>()

const emit = defineEmits<{ close: []; restore: [id: string]; clear: [] }>()

const { t } = useI18n()
const query = ref('')

watch(
  () => props.open,
  (open) => {
    if (open) query.value = ''
  },
)

const filtered = computed(() => {
  const keyword = query.value.trim().toLowerCase()
  if (!keyword) return props.chats
  return props.chats.filter((chat) => {
    if (
      [chat.title, chat.modelLabel || (chat.legacy ? t('shell.aiChat.historyLegacy') : props.providerLabel)].some(
        (value) => value.toLowerCase().includes(keyword),
      )
    )
      return true
    return chat.messages.some((message) => message.content.toLowerCase().includes(keyword))
  })
})

function questionCount(chat: ArchivedChat): number {
  return chat.messages.filter((message) => message.role === 'user').length
}

function formatTime(at: number): string {
  if (!at || !Number.isFinite(at)) return t('shell.aiChat.notAvailable')
  const date = new Date(at)
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => !open && emit('close')">
    <QtermDialogContent
      v-if="props.open"
      data-ai-history-dialog
      size="lg"
      :close-on-escape="true"
      class="flex! h-[min(680px,calc(100dvh-2rem))] min-h-0 min-w-0 flex-col overflow-hidden"
    >
      <QtermDialogHeader
        :title="t('shell.aiChat.historyTitle')"
        :description="t('shell.aiChat.historyDescription')"
        :show-close="true"
        @close="emit('close')"
      />
      <div class="dialog-body flex! min-h-0 min-w-0 flex-1 flex-col gap-3 overflow-hidden!">
        <div class="flex min-w-0 shrink-0 items-center gap-2">
          <span class="relative min-w-0 flex-1">
            <Search
              :size="14"
              class="pointer-events-none absolute inset-y-0 start-2.5 my-auto text-qt-muted"
              aria-hidden="true"
            />
            <QtermInput
              v-model="query"
              class="w-full min-w-0 ps-8"
              :placeholder="t('shell.aiChat.historySearch')"
              :aria-label="t('shell.aiChat.historySearch')"
              data-ai-history-search
            />
          </span>
          <QtermButton variant="ghost" class="shrink-0 text-qt-danger" data-ai-history-clear @click="emit('clear')">
            {{ t('shell.aiChat.historyClear') }}
          </QtermButton>
        </div>

        <div
          v-if="filtered.length"
          class="min-h-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto overscroll-contain [scrollbar-gutter:stable]"
          data-ai-history-list
        >
          <div class="flex min-w-0 flex-col gap-2 pe-1">
            <button
              v-for="chat in filtered"
              :key="chat.id"
              type="button"
              class="flex w-full min-w-0 max-w-full flex-col gap-1.5 rounded-lg border border-qt-border/70 px-3 py-2.5 text-start transition-colors hover:bg-qt-hover focus-visible:bg-qt-hover focus-visible:outline-none"
              :class="chat.id === props.activeId ? 'border-qt-accent/45' : ''"
              :data-ai-history-item="chat.id"
              @click="emit('restore', chat.id)"
            >
              <span class="flex w-full min-w-0 items-center gap-2">
                <span class="min-w-0 flex-1 truncate text-sm font-medium" :title="chat.title">{{
                  chat.title || t('shell.aiChat.historyUntitled')
                }}</span>
                <span
                  v-if="chat.id === props.activeId"
                  class="shrink-0 rounded-md bg-qt-subtle px-1.5 text-[11px] leading-4 text-qt-muted"
                  >{{ t('shell.aiChat.historyCurrent') }}</span
                >
                <span
                  v-if="chat.legacy"
                  class="shrink-0 rounded-md bg-qt-subtle px-1.5 text-xs leading-4 text-qt-muted"
                  >{{ t('shell.aiChat.historyLegacy') }}</span
                >
              </span>
              <span class="flex w-full min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-xs text-qt-muted">
                <span class="font-mono">{{ formatTime(chat.updatedAt) }}</span>
                <span aria-hidden="true">·</span>
                <span>{{ t('shell.aiChat.historyTurns', { count: questionCount(chat) }) }}</span>
                <span aria-hidden="true">·</span>
                <span
                  class="min-w-0 flex-1 basis-32 truncate"
                  :title="chat.modelLabel || (chat.legacy ? t('shell.aiChat.historyLegacy') : props.providerLabel)"
                  >{{ chat.modelLabel || (chat.legacy ? t('shell.aiChat.historyLegacy') : props.providerLabel) }}</span
                >
              </span>
            </button>
          </div>
        </div>

        <QtermEmptyState
          v-else
          :icon="MessageSquare"
          :title="query ? t('shell.aiChat.historyNoMatch') : t('shell.aiChat.historyEmpty')"
          :description="query ? t('shell.aiChat.historyNoMatchHint') : t('shell.aiChat.historyEmptyHint')"
        />
      </div>
      <QtermDialogFooter>
        <span class="min-w-0 flex-1 text-xs leading-5 text-qt-muted">{{ t('shell.aiChat.historyFootnote') }}</span>
        <QtermButton variant="outline" class="shrink-0" data-ai-history-close @click="emit('close')">{{
          t('shell.app.close')
        }}</QtermButton>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>
