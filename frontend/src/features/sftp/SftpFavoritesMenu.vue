<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { Check, Pencil, Star, X } from '@lucide/vue'
import { IconButton, QtermInput } from '../../components/common'
import { Popover, PopoverContent, PopoverTrigger } from '../../components/common/overlay-controls'
import { useI18n } from '../../locales'
import {
  defaultFavoriteName,
  favoriteDisplayName,
  FAVORITE_NAME_MAX_LENGTH,
  normalizeFavoriteName,
  type SFTPFavorite,
} from '../../services/sftpFavorites'

const props = withDefaults(
  defineProps<{
    favorites: SFTPFavorite[]
    currentPath: string
    disabled?: boolean
    busy?: boolean
  }>(),
  { disabled: false, busy: false },
)

const emit = defineEmits<{
  navigate: [path: string]
  add: [path: string, name: string]
  remove: [path: string]
  rename: [path: string, name: string]
}>()

const { t } = useI18n()

const currentFavorite = computed(() => props.favorites.find((item) => item.path === props.currentPath) ?? null)
const canAdd = computed(() => props.currentPath.startsWith('/') && !currentFavorite.value && !props.disabled)

const draftName = ref('')
const draftPlaceholder = computed(() => defaultFavoriteName(props.currentPath))

watch(
  () => props.currentPath,
  () => {
    draftName.value = ''
    cancelRename()
  },
)

const renamingPath = ref('')
const renameDraft = ref('')
const renameInput = ref<HTMLInputElement | null>(null)

function captureRenameInput(target: unknown): void {
  const element = (target as { $el?: unknown } | null)?.$el
  renameInput.value = element instanceof HTMLInputElement ? element : null
}

async function beginRename(item: SFTPFavorite): Promise<void> {
  if (props.busy || props.disabled) return
  renamingPath.value = item.path
  renameDraft.value = item.name
  await nextTick()
  renameInput.value?.focus()
  renameInput.value?.select()
}

function cancelRename(): void {
  renamingPath.value = ''
  renameDraft.value = ''
}

function commitRename(path: string): void {
  const name = normalizeFavoriteName(renameDraft.value)
  cancelRename()
  emit('rename', path, name)
}

function submitAdd(): void {
  if (!canAdd.value || props.busy) return
  emit('add', props.currentPath, normalizeFavoriteName(draftName.value))
  draftName.value = ''
}
</script>

<template>
  <Popover>
    <PopoverTrigger as-child>
      <IconButton
        variant="ghost"
        size="icon-sm"
        type="button"
        :label="currentFavorite ? t('sftp.ui.favorites.titleCurrent') : t('sftp.ui.favorites.title')"
        :disabled="props.disabled"
        data-sftp-favorites-trigger
        :data-sftp-favorites-state="currentFavorite ? 'current' : 'idle'"
      >
        <Star
          :size="16"
          aria-hidden="true"
          :class="currentFavorite ? 'fill-qt-warning text-qt-warning' : ''"
          data-sftp-favorites-star
        />
      </IconButton>
    </PopoverTrigger>
    <PopoverContent
      class="qterm-dialog [--qt-hover:color-mix(in_srgb,var(--qt-text)_8%,transparent)]! flex w-80 flex-col gap-1.5 p-2"
      align="end"
      :side-offset="6"
      :collision-padding="12"
      data-sftp-favorites-popover
    >
      <div class="px-1 text-xs text-qt-muted">{{ t('sftp.ui.favorites.title') }}</div>
      <ul
        v-if="props.favorites.length"
        class="m-0 flex max-h-64 list-none flex-col gap-0.5 overflow-y-auto p-0"
        data-sftp-favorites-list
      >
        <li
          v-for="item in props.favorites"
          :key="item.path"
          class="flex items-center gap-1 rounded-md px-1 py-0.5 transition-colors hover:bg-(--qt-hover)"
          :data-sftp-favorite-row="item.path"
          :data-sftp-favorite-current="item.path === props.currentPath ? 'true' : 'false'"
        >
          <template v-if="renamingPath === item.path">
            <QtermInput
              :ref="captureRenameInput"
              v-model="renameDraft"
              class="h-6! min-h-0! flex-1 px-1.5! text-xs!"
              :maxlength="FAVORITE_NAME_MAX_LENGTH"
              :aria-label="t('sftp.ui.favorites.nameLabel')"
              :placeholder="defaultFavoriteName(item.path)"
              data-sftp-favorite-rename-input
              @keydown.enter.prevent="commitRename(item.path)"
              @keydown.escape.prevent="cancelRename"
            />
            <IconButton
              size="icon-xs"
              type="button"
              :label="t('sftp.ui.favorites.confirmName')"
              :data-sftp-favorite-rename-confirm="item.path"
              @click.stop="commitRename(item.path)"
            >
              <Check :size="13" aria-hidden="true" />
            </IconButton>
            <IconButton
              size="icon-xs"
              type="button"
              :label="t('shell.common.cancel')"
              :data-sftp-favorite-rename-cancel="item.path"
              @click.stop="cancelRename"
            >
              <X :size="13" aria-hidden="true" />
            </IconButton>
          </template>
          <template v-else>
            <button
              type="button"
              class="min-w-0 flex-1 rounded-md px-1 py-1 text-left focus-visible:ring-2 focus-visible:ring-qt-accent/40 focus-visible:outline-none disabled:opacity-50"
              :title="item.path"
              :disabled="props.busy"
              :data-sftp-favorite="item.path"
              @click="emit('navigate', item.path)"
            >
              <span class="flex min-w-0 items-center gap-1 text-xs text-qt-text">
                <span class="min-w-0 truncate">{{ favoriteDisplayName(item) }}</span>

                <Star
                  v-if="item.path === props.currentPath"
                  :size="11"
                  class="flex-none fill-qt-warning text-qt-warning"
                  aria-hidden="true"
                  data-sftp-favorite-current-star
                />
              </span>
              <span class="mt-0.5 block truncate font-mono text-[11px] text-qt-muted">{{ item.path }}</span>
            </button>
            <IconButton
              size="icon-xs"
              type="button"
              :label="t('sftp.ui.favorites.rename')"
              :disabled="props.busy"
              :data-sftp-favorite-rename="item.path"
              @click.stop="beginRename(item)"
            >
              <Pencil :size="12" aria-hidden="true" />
            </IconButton>
            <IconButton
              size="icon-xs"
              type="button"
              danger
              :label="t('sftp.ui.favorites.remove')"
              :disabled="props.busy"
              :data-sftp-favorite-remove="item.path"
              @click.stop="emit('remove', item.path)"
            >
              <X :size="13" aria-hidden="true" />
            </IconButton>
          </template>
        </li>
      </ul>
      <p v-else class="m-0 px-1 py-1 text-xs text-qt-muted">{{ t('sftp.ui.favorites.empty') }}</p>

      <QtermInput
        v-model="draftName"
        class="h-6! min-h-0! px-1.5! text-xs!"
        :maxlength="FAVORITE_NAME_MAX_LENGTH"
        :aria-label="t('sftp.ui.favorites.nameLabel')"
        :placeholder="draftPlaceholder"
        :disabled="!canAdd || props.busy"
        data-sftp-favorite-name
        @keydown.enter.prevent="submitAdd"
      />
      <button
        type="button"
        class="w-full rounded-md border border-dashed border-qt-border bg-transparent px-1.5 py-1 text-xs text-qt-text enabled:hover:bg-(--qt-hover) disabled:opacity-50"
        :disabled="!canAdd || props.busy"
        data-sftp-favorite-add
        @click="submitAdd"
      >
        {{ t('sftp.ui.favorites.add') }}
      </button>
    </PopoverContent>
  </Popover>
</template>
