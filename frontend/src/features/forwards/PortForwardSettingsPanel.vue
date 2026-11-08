<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { FolderPlus, Loader2, Pencil, Plus, Route, Trash2 } from '@lucide/vue'
import { QtermButton, QtermEmptyState, QtermSearchToolbar } from '@/components/common'
import { Accordion, AccordionContent, AccordionItem } from '@/components/common/control-primitives'
import SettingsGroupHeader from '@/components/common/settings/SettingsGroupHeader.vue'
import { ScrollArea } from '@/components/common/control-primitives'
import QtermLoadMore from '@/components/common/layout/QtermLoadMore.vue'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '@/components/common/dialog'
import { useI18n } from '@/locales'
import type { HostRecord } from '@/services/hosts'
import type { PortForwardConfig } from '../../services/forwards'
import { useSnippetGroupPagination } from '../snippets/useSnippetGroupPagination'
import PortForwardEditorDialog from './PortForwardEditorDialog.vue'
import PortForwardGroupDialog from './PortForwardGroupDialog.vue'
import {
  dissolveForwardGroup,
  forwardDisplayName,
  forwardHostLabel,
  forwardKindLabelKey,
  forwardSections,
  forwardSummary,
  mergeForwardGroups,
  renameForwardGroup,
  type ForwardSection,
} from './forwardView'
import { usePortForwardActions } from './usePortForwardActions'

const props = withDefaults(
  defineProps<{
    forwards?: PortForwardConfig[]
    hosts?: HostRecord[]

    forwardGroups?: string[]

    activeHostId?: string

    loading?: boolean

    error?: string
  }>(),
  {
    forwards: () => [],
    hosts: () => [],
    forwardGroups: () => [],
    activeHostId: '',
    loading: false,
    error: '',
  },
)

const emit = defineEmits<{
  'update:forwards': [value: PortForwardConfig[]]
  'update:forwardGroups': [value: string[]]
}>()

const { t } = useI18n()
const actions = usePortForwardActions({
  forwards: () => props.forwards,
  update: (next) => emit('update:forwards', next),
})

const sshHosts = computed(() =>
  props.hosts.filter((host) => !host.protocol || host.protocol.trim().toLocaleLowerCase() === 'ssh'),
)

const groupOptions = computed(() => mergeForwardGroups(props.forwards, props.forwardGroups))

const ungroupedLabel = computed(() => t('shell.forwards.ungrouped'))

const query = ref('')

const sections = computed(() =>
  forwardSections(props.forwards, {
    hosts: props.hosts,
    kindLabel: (row) => t(forwardKindLabelKey(row.kind)),
    ungroupedLabel: ungroupedLabel.value,
    groups: props.forwardGroups,
    query: query.value,
  }),
)

const expandedKeys = ref<string[]>([])

watch(
  sections,
  (list) => {
    const keys = list.map((section) => section.key)
    if (query.value.trim()) {
      expandedKeys.value = keys
      return
    }
    const missing = keys.filter((key) => !expandedKeys.value.includes(key))
    if (missing.length) expandedKeys.value = [...expandedKeys.value, ...missing]
  },
  { immediate: true },
)

watch(query, () => {
  if (query.value.trim()) expandedKeys.value = sections.value.map((section) => section.key)
})

const { loadMore, reset, visibleCount, visibleItems } = useSnippetGroupPagination(20)

watch(query, () => reset())

const editorOpen = ref(false)
const editing = ref<PortForwardConfig | null>(null)

function openCreate(): void {
  editing.value = null
  editorOpen.value = true
}

function openEdit(row: PortForwardConfig): void {
  editing.value = row
  editorOpen.value = true
}

async function handleSubmit(payload: PortForwardConfig): Promise<void> {
  await actions.save(payload)
}

function handleCreateGroup(name: string): void {
  emit('update:forwardGroups', mergeForwardGroups(props.forwards, [...props.forwardGroups, name]))
}

const groupDialogOpen = ref(false)
const groupDialogMode = ref<'create' | 'rename'>('create')
const groupRenameFrom = ref('')

function openGroupCreate(): void {
  groupDialogMode.value = 'create'
  groupRenameFrom.value = ''
  groupDialogOpen.value = true
}

function openRename(section: ForwardSection): void {
  groupDialogMode.value = 'rename'
  groupRenameFrom.value = section.name
  groupDialogOpen.value = true
}

function handleGroupSubmit(name: string): void {
  if (groupDialogMode.value === 'create') {
    handleCreateGroup(name)
    return
  }
  const from = groupRenameFrom.value
  if (from === name) return
  const nextRows = renameForwardGroup(props.forwards, from, name)
  emit('update:forwards', nextRows)

  emit(
    'update:forwardGroups',
    mergeForwardGroups(
      nextRows,
      props.forwardGroups.map((item) => (item === from ? name : item)),
    ),
  )
}

const pendingDelete = ref<PortForwardConfig | null>(null)
const deleting = ref(false)

function closeDeleteDialog(): void {
  if (deleting.value) return
  pendingDelete.value = null
}

async function confirmDelete(): Promise<void> {
  const target = pendingDelete.value
  if (!target || deleting.value) return
  deleting.value = true
  try {
    await actions.remove(target)
    pendingDelete.value = null
  } finally {
    deleting.value = false
  }
}

const pendingGroupDelete = ref<ForwardSection | null>(null)

function confirmGroupDelete(): void {
  const target = pendingGroupDelete.value
  if (!target) return
  const nextRows = dissolveForwardGroup(props.forwards, target.name)
  emit('update:forwards', nextRows)
  emit(
    'update:forwardGroups',
    mergeForwardGroups(
      nextRows,
      props.forwardGroups.filter((item) => item !== target.name),
    ),
  )
  pendingGroupDelete.value = null
}
</script>

<template>
  <section class="flex h-full min-h-0 flex-col gap-4">
    <QtermSearchToolbar
      v-model="query"
      :placeholder="t('shell.forwards.search')"
      :search-aria-label="t('shell.forwards.search')"
      :clear-aria-label="t('shell.forwards.clearSearch')"
    >
      <QtermButton variant="outline" size="sm" type="button" data-forward-group-create @click="openGroupCreate">
        <FolderPlus :size="14" /> {{ t('settings.portForwards.group.create') }}
      </QtermButton>
      <QtermButton size="sm" type="button" data-forward-create @click="openCreate">
        <Plus :size="14" /> {{ t('settings.portForwards.action.create') }}
      </QtermButton>
    </QtermSearchToolbar>

    <QtermEmptyState
      v-if="props.loading"
      variant="outlined"
      role="status"
      busy
      :icon="Loader2"
      icon-class="spin"
      :description="t('settings.portForwards.loading')"
    />

    <QtermEmptyState v-else-if="props.error" variant="outlined" role="alert" :description="props.error" />

    <ScrollArea v-else-if="sections.length" type="hover" :scroll-hide-delay="600" class="min-h-0 flex-1">
      <div class="flex flex-col gap-2 pr-2">
        <Accordion v-model="expandedKeys" type="multiple" class="flex flex-col gap-2">
          <AccordionItem
            v-for="section in sections"
            :key="section.key"
            :value="section.key"
            class="overflow-hidden rounded-md border-d"
            data-forward-settings-group
          >
            <SettingsGroupHeader
              :title="section.name"
              :count="section.rows.length"
              :folder="section.name !== ungroupedLabel"
              data-forward-group-toggle
            >
              <template v-if="section.name !== ungroupedLabel" #actions>
                <QtermButton
                  variant="ghost"
                  size="icon-sm"
                  type="button"
                  data-forward-group-rename
                  :aria-label="t('settings.portForwards.group.rename')"
                  @click="openRename(section)"
                >
                  <Pencil :size="14" />
                </QtermButton>
                <QtermButton
                  variant="ghost"
                  size="icon-sm"
                  class="settings-delete-action"
                  type="button"
                  data-forward-group-delete
                  :aria-label="t('settings.portForwards.group.delete')"
                  @click="pendingGroupDelete = section"
                >
                  <Trash2 :size="14" />
                </QtermButton>
              </template>
            </SettingsGroupHeader>

            <AccordionContent class="pb-0">
              <div v-if="section.rows.length" class="flex flex-col border-t border-qt-border">
                <article
                  v-for="row in visibleItems(section.key, section.rows, true)"
                  :key="row.id"
                  class="settings-library-row flex min-w-0 items-center gap-3 px-3 py-2"
                  data-forward-settings-row
                >
                  <Route :size="18" class="shrink-0 text-qt-muted" aria-hidden="true" />
                  <div class="flex min-w-0 flex-1 flex-col gap-0.5">
                    <div class="flex min-w-0 flex-wrap items-center gap-2">
                      <strong class="min-w-0 truncate text-sm">{{ forwardDisplayName(row) }}</strong>
                      <span class="shrink-0 rounded-full border-d bg-qt-subtle px-2 text-xs leading-4 text-qt-muted">{{
                        t(forwardKindLabelKey(row.kind))
                      }}</span>
                    </div>
                    <span class="truncate text-xs text-qt-muted">
                      {{ forwardSummary(row) }} · {{ forwardHostLabel(row, sshHosts) }}
                    </span>
                  </div>
                  <div class="settings-library-row-actions flex shrink-0 items-center gap-1">
                    <QtermButton
                      variant="ghost"
                      size="icon-sm"
                      type="button"
                      data-forward-edit
                      :aria-label="t('shell.forwards.edit')"
                      @click="openEdit(row)"
                    >
                      <Pencil :size="15" />
                    </QtermButton>
                    <QtermButton
                      variant="ghost"
                      size="icon-sm"
                      class="settings-delete-action"
                      type="button"
                      data-forward-delete
                      :aria-label="t('shell.forwards.remove')"
                      @click="pendingDelete = row"
                    >
                      <Trash2 :size="15" />
                    </QtermButton>
                  </div>
                </article>
                <QtermLoadMore
                  class="px-3 py-1.5"
                  :visible="visibleCount(section.key, section.rows.length, true)"
                  :total="section.rows.length"
                  :load-more-label="t('settings.portForwards.pagination.loadMore')"
                  @load-more="loadMore(section.key, section.rows.length)"
                />
              </div>
            </AccordionContent>
          </AccordionItem>
        </Accordion>
      </div>
    </ScrollArea>

    <QtermEmptyState v-else-if="query.trim()" variant="outlined" :icon="Route" :title="t('shell.forwards.noMatch')" />

    <QtermEmptyState
      v-else
      variant="outlined"
      :icon="Route"
      :title="t('settings.portForwards.empty.title')"
      :description="t('settings.portForwards.empty.description')"
    />
  </section>

  <PortForwardEditorDialog
    :open="editorOpen"
    :config="editing"
    :hosts="sshHosts"
    :active-host-id="props.activeHostId"
    :groups="groupOptions"
    @update:open="(open) => (editorOpen = open)"
    @submit="handleSubmit"
    @create-group="handleCreateGroup"
  />

  <PortForwardGroupDialog
    :open="groupDialogOpen"
    :mode="groupDialogMode"
    :initial="groupRenameFrom"
    :existing="groupOptions"
    @update:open="(open) => (groupDialogOpen = open)"
    @submit="handleGroupSubmit"
  />

  <QtermDialog :open="pendingDelete !== null" @update:open="(open) => !open && closeDeleteDialog()">
    <QtermDialogContent v-if="pendingDelete" data-forward-delete-dialog>
      <QtermDialogHeader
        :title="t('settings.portForwards.dialog.deleteTitle')"
        :description="t('settings.portForwards.dialog.deleteDescription')"
        :show-close="false"
      />
      <div class="dialog-body">
        <p class="m-0">
          {{ t('settings.portForwards.dialog.deleteConfirm', { name: forwardDisplayName(pendingDelete) }) }}
        </p>
        <p class="mt-2 mb-0 text-xs text-qt-muted">
          {{ t('settings.portForwards.dialog.deleteHint') }}
        </p>
      </div>
      <QtermDialogFooter>
        <div></div>
        <div class="dialog-footer-actions flex items-center gap-[8px]">
          <QtermButton variant="outline" :disabled="deleting" @click="closeDeleteDialog">{{
            t('shell.common.cancel')
          }}</QtermButton>
          <QtermButton variant="destructive" data-forward-delete-confirm :disabled="deleting" @click="confirmDelete">{{
            t('shell.forwards.remove')
          }}</QtermButton>
        </div>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>

  <QtermDialog :open="pendingGroupDelete !== null" @update:open="(open) => !open && (pendingGroupDelete = null)">
    <QtermDialogContent v-if="pendingGroupDelete" data-forward-group-delete-dialog>
      <QtermDialogHeader
        :title="t('settings.portForwards.group.deleteTitle')"
        :description="t('settings.portForwards.group.deleteDescription')"
        :show-close="false"
      />
      <div class="dialog-body">
        <p class="m-0">
          {{ t('settings.portForwards.group.deleteConfirm', { name: pendingGroupDelete.name }) }}
        </p>
      </div>
      <QtermDialogFooter>
        <div></div>
        <div class="dialog-footer-actions flex items-center gap-[8px]">
          <QtermButton variant="outline" @click="pendingGroupDelete = null">{{ t('shell.common.cancel') }}</QtermButton>
          <QtermButton variant="destructive" data-forward-group-delete-confirm @click="confirmGroupDelete">{{
            t('settings.portForwards.group.delete')
          }}</QtermButton>
        </div>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>
