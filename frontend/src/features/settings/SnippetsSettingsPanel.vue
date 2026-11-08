<script setup lang="ts">
import {
  IconButton,
  QtermButton,
  QtermEmptyState,
  QtermFormMessage,
  QtermInput,
  QtermSwitch,
} from '../../components/common'
import QtermSearchToolbar from '../../components/common/layout/QtermSearchToolbar.vue'
import QtermLoadMore from '../../components/common/layout/QtermLoadMore.vue'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Copy, FolderPlus, Pencil, Pin, Plus, Trash2 } from '@lucide/vue'
import SettingsGroupHeader from '../../components/common/settings/SettingsGroupHeader.vue'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { Label } from '../../components/common/control-primitives'
import { ScrollArea } from '../../components/common/control-primitives'
import SnippetHostTree from './SnippetHostTree.vue'
import { errorMessage } from '@/services/errors.ts'
import {
  Accordion,
  AccordionItem,
  AccordionTrigger,
  AccordionContent,
} from '../../components/common/control-primitives'
import { QtermCombobox, type QtermComboboxOption } from '../../components/common/form-controls'
import { createHostGateway, type GroupRecord, type HostRecord } from '@/services/hosts.ts'
import { createSnippetGateway, type Snippet } from '@/services/snippets.ts'
import DeleteConfirmDialog from '../../components/dialogs/DeleteConfirmDialog.vue'
import { useI18n } from '@/locales'
import { toast } from 'vue-sonner'
import { writeClipboardText } from '@/services/clipboard.ts'
import HostIconPicker from '../../components/dialogs/HostIconPicker.vue'
import SegmentedTabs from '../../components/common/layout/SegmentedTabs.vue'
import SnippetIcon from '../snippets/SnippetIcon.vue'
import { snippetColorStyle } from '../snippets/appearance'
import ColorTagField from '../../components/common/form-controls/ColorTagField.vue'
import SnippetContentEditor from './SnippetContentEditor.vue'
import { useSnippetGroupPagination } from '../snippets/useSnippetGroupPagination'

const snippets = ref<Snippet[]>([])
const searchQuery = ref('')
const expandedGroups = ref<string[]>([])
const props = withDefaults(defineProps<{ standalone?: boolean; createRequest?: number }>(), {
  standalone: false,
  createRequest: 0,
})
const groups = ref<GroupRecord[]>([])
const commandGroups = ref<GroupRecord[]>([])

const GROUP_NONE_VALUE = '__none__'

const commandGroupOptions = computed<QtermComboboxOption[]>(() => [
  { value: GROUP_NONE_VALUE, label: t('settings.snippets.group.none') },
  ...commandGroups.value.map((group) => ({ value: group.id, label: group.name })),
])
const assetGroupOptions = computed<QtermComboboxOption[]>(() => [
  { value: GROUP_NONE_VALUE, label: t('settings.snippets.association.groupNone') },
  ...groups.value.map((group) => ({ value: group.id, label: group.name })),
])
const groupSaving = ref(false)
const hosts = ref<HostRecord[]>([])
const open = ref(false)
const editing = ref<Snippet | null>(null)
const name = ref('')
const content = ref('')
const groupId = ref('')
const assetGroupId = ref('')
const hostIds = ref<string[]>([])
const autoExecute = ref(true)
const icon = ref('')
const color = ref('')
const {
  loadMore: loadMoreSnippetGroup,
  reset: resetSnippetGroupPagination,
  reveal: revealSnippetGroupItem,
  visibleCount: visibleSnippetGroupCount,
  visibleItems: visibleSnippetGroupItems,
} = useSnippetGroupPagination(20)

const pinned = ref(false)
const expandedPanels = ref<string[]>([])
const draggingGroup = ref<string | null>(null)
const draggingSnippet = ref<string | null>(null)
const movingSnippet = ref(false)
const dropIndicator = ref('')
const dropPosition = ref<'before' | 'after'>('before')
const dropLineOffset = ref(0)
const assetError = ref('')
const groupDialogOpen = ref(false)
const groupName = ref('')
const editingGroupId = ref('')
const deleteGroupId = ref('')
const groupError = ref('')
const deleteTarget = ref<{ id: string; name: string } | null>(null)
const deleteSaving = ref(false)
const deleteError = ref('')
const error = ref('')
const saving = ref(false)
const loading = ref(true)
const snippetGateway = createSnippetGateway()
const hostGateway = createHostGateway()
const { t } = useI18n()

const grouped = computed(() => {
  const query = searchQuery.value.trim().toLocaleLowerCase()
  const map = new Map<string, Snippet[]>(commandGroups.value.map((group) => [group.id, []]))
  map.set('', [])
  for (const item of snippets.value) {
    if (query && !`${item.name}\n${item.content}`.toLocaleLowerCase().includes(query)) continue
    const key = item.groupId || ''
    map.set(key, [...(map.get(key) || []), item])
  }
  return [...map.entries()]
    .filter(([, items]) => !query || items.length > 0 || draggingSnippet.value)
    .map(
      ([id, items]) =>
        [
          id,
          commandGroups.value.find((group) => group.id === id)?.name || t('settings.snippets.group.ungrouped'),
          [...items].sort((a, b) => Number(!!b.pinned) - Number(!!a.pinned)),
        ] as const,
    )
})

function snippetGroupKey(groupID: string): string {
  return groupID || '__ungrouped__'
}

function snippetGroupExpanded(groupID: string): boolean {
  return expandedGroups.value.includes(snippetGroupKey(groupID))
}

watch(searchQuery, (query) => {
  resetSnippetGroupPagination()
  expandedGroups.value = query.trim() ? grouped.value.map(([id]) => snippetGroupKey(id)) : []
})

function startSnippetDrag(event: DragEvent, id: string): void {
  if (movingSnippet.value) {
    event.preventDefault()
    return
  }
  draggingSnippet.value = id
  draggingGroup.value = null
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('application/x-qterm-snippet', id)
  }
}
function endSnippetDrag(): void {
  draggingSnippet.value = null
  dropIndicator.value = ''
  dropLineOffset.value = 0
}
function overSnippet(event: DragEvent, id: string): void {
  if (!draggingSnippet.value || movingSnippet.value) return
  event.preventDefault()
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
  const midpoint = rect.top + rect.height / 2

  const hysteresis = 6
  if (dropIndicator.value !== id) {
    dropPosition.value = event.clientY < midpoint ? 'before' : 'after'
  } else if (dropPosition.value === 'before' && event.clientY > midpoint + hysteresis) {
    dropPosition.value = 'after'
  } else if (dropPosition.value === 'after' && event.clientY < midpoint - hysteresis) {
    dropPosition.value = 'before'
  }
  dropIndicator.value = id
  const target = event.currentTarget as HTMLElement
  const targetRect = target.getBoundingClientRect()
  if (dropPosition.value === 'before') {
    dropLineOffset.value = 0
  } else {
    const next = target.nextElementSibling as HTMLElement | null
    const nextTop = next?.getBoundingClientRect().top

    dropLineOffset.value =
      nextTop === undefined ? Math.max(0, targetRect.height - 3) : Math.max(0, nextTop - targetRect.top)
  }
}
async function dropSnippet(groupID: string, targetID = ''): Promise<void> {
  const id = draggingSnippet.value
  const position = targetID ? dropPosition.value : 'after'
  endSnippetDrag()
  if (!id || id === targetID || movingSnippet.value) return
  movingSnippet.value = true
  error.value = ''
  try {
    await snippetGateway.reorder(id, groupID, targetID, position)
    snippets.value = await snippetGateway.list()
    const expanded = groupID || '__ungrouped__'
    if (!expandedGroups.value.includes(expanded)) expandedGroups.value.push(expanded)
    const destination = grouped.value.find(([candidateID]) => candidateID === groupID)
    revealSnippetGroupItem(expanded, destination?.[2].findIndex((item) => item.id === id) ?? -1)
  } catch (cause) {
    error.value = errorMessage(cause, t('settings.snippets.error.move'))
  } finally {
    movingSnippet.value = false
  }
}
function dropOnGroup(groupID: string, title: string): void {
  if (draggingSnippet.value) void dropSnippet(groupID)
  else dropGroup(title)
}

async function load(): Promise<void> {
  loading.value = true
  try {
    error.value = ''
    assetError.value = ''

    await Promise.all([
      (async () => {
        try {
          const [nextGroups, nextHosts] = await Promise.all([hostGateway.listGroups(), hostGateway.list()])
          groups.value = nextGroups
          hosts.value = nextHosts
        } catch (cause) {
          assetError.value = errorMessage(cause, t('settings.snippets.error.assetLoad'))
        }
      })(),
      (async () => {
        try {
          const [items, categories] = await Promise.all([snippetGateway.list(), snippetGateway.listGroups()])
          snippets.value = items
          commandGroups.value = categories
        } catch (cause) {
          error.value = errorMessage(cause, t('settings.snippets.error.commandLoad'))
        }
      })(),
    ])
  } catch (cause) {
    error.value = errorMessage(cause, t('settings.snippets.error.load'))
  } finally {
    loading.value = false
  }
}
function resetForm(): void {
  editing.value = null
  name.value = ''
  content.value = ''
  groupId.value = ''
  assetGroupId.value = ''
  hostIds.value = []
  autoExecute.value = true
  icon.value = ''
  color.value = ''
  pinned.value = false
  expandedPanels.value = []
  error.value = ''
}
function createGroup(): void {
  editingGroupId.value = ''
  groupName.value = ''
  groupError.value = ''
  groupDialogOpen.value = true
}
async function saveGroup(): Promise<void> {
  if (groupSaving.value) return
  if (!groupName.value.trim()) {
    groupError.value = t('settings.snippets.error.groupNameRequired')
    return
  }
  groupSaving.value = true
  groupError.value = ''
  try {
    if (editingGroupId.value) {
      await snippetGateway.renameGroup(editingGroupId.value, groupName.value.trim())
      await load()
    } else {
      const group = await snippetGateway.createGroup(groupName.value.trim())
      commandGroups.value = [...commandGroups.value, group]
    }
    groupDialogOpen.value = false
  } catch (cause) {
    groupError.value = errorMessage(
      cause,
      t(editingGroupId.value ? 'settings.snippets.error.renameGroup' : 'settings.snippets.error.createGroup'),
    )
  } finally {
    groupSaving.value = false
  }
}
function createSnippet(): void {
  resetForm()
  open.value = true
}

function openCreateDialogWithContent(command: string): void {
  createSnippet()
  content.value = command
}
function editSnippet(item: Snippet): void {
  editing.value = item
  name.value = item.name
  content.value = item.content
  groupId.value = item.groupId || ''
  assetGroupId.value = item.assetGroupId || ''
  hostIds.value = item.hostIds || []
  autoExecute.value = item.autoExecute
  icon.value = item.icon || ''
  color.value = item.color || ''
  pinned.value = !!item.pinned
  error.value = ''
  expandedPanels.value = []
  open.value = true
}
function close(): void {
  if (!saving.value) open.value = false
}
async function save(): Promise<void> {
  if (!name.value.trim() || !content.value.trim()) {
    error.value = t('settings.snippets.error.nameAndContentRequired')
    return
  }
  saving.value = true
  error.value = ''
  try {
    const payload = {
      name: name.value.trim(),
      content: content.value.trim(),
      groupId: groupId.value === '__none__' ? '' : groupId.value || '',
      assetGroupId: assetGroupId.value || '',
      hostIds: hostIds.value,
      autoExecute: autoExecute.value,
      icon: icon.value,
      color: color.value,
      pinned: pinned.value,
    }
    if (editing.value) await snippetGateway.update(editing.value.id, payload)
    else await snippetGateway.create(payload)
    await load()
    open.value = false
  } catch (cause) {
    error.value = errorMessage(cause, t('settings.snippets.error.save'))
  } finally {
    saving.value = false
  }
}
function editGroup(title: string): void {
  const group = commandGroups.value.find((item) => item.name === title)
  if (!group) return
  editingGroupId.value = group.id
  groupName.value = group.name
  groupError.value = ''
  groupDialogOpen.value = true
}
function removeGroup(title: string): void {
  const group = commandGroups.value.find((item) => item.name === title)
  if (!group) return
  deleteGroupId.value = group.id
  deleteTarget.value = { id: group.id, name: group.name }
  deleteError.value = ''
}
async function remove(item: Snippet): Promise<void> {
  deleteGroupId.value = ''
  deleteTarget.value = item
  deleteError.value = ''
}
async function copySnippet(item: Snippet): Promise<void> {
  try {
    await writeClipboardText(item.content)
    toast.success(t('settings.snippets.toast.copied'))
  } catch {
    toast.error(t('settings.snippets.toast.copyFailed'))
  }
}
async function confirmRemove(): Promise<void> {
  if (!deleteTarget.value || deleteSaving.value) return
  deleteSaving.value = true
  deleteError.value = ''
  try {
    if (deleteGroupId.value) await snippetGateway.removeGroup(deleteGroupId.value)
    else await snippetGateway.remove(deleteTarget.value.id)
    deleteTarget.value = null
    await load()
  } catch (cause) {
    deleteError.value = errorMessage(cause, t('settings.snippets.error.delete'))
  } finally {
    deleteSaving.value = false
  }
}
function selectHosts(ids: string[], checked: boolean): void {
  const selected = new Set(hostIds.value)
  for (const id of ids) {
    if (checked) selected.add(id)
    else selected.delete(id)
  }
  hostIds.value = [...selected]
}
function selectAssetGroup(id: string): void {
  assetGroupId.value = id
}
function dropGroup(title: string): void {
  const source = draggingGroup.value
  draggingGroup.value = null
  if (!source || source === title) return
  const sourceIndex = commandGroups.value.findIndex((group) => group.name === source)
  const targetIndex = commandGroups.value.findIndex((group) => group.name === title)
  if (sourceIndex < 0 || targetIndex < 0) return
  const next = [...commandGroups.value]
  const [moved] = next.splice(sourceIndex, 1)
  next.splice(targetIndex, 0, moved!)
  commandGroups.value = next
}
function handleCreateRequest(): void {
  createSnippet()
}
function handleAssetsImported(): void {
  void load()
}
defineExpose({ openCreateDialog: createSnippet, openCreateDialogWithContent })
watch(
  () => props.createRequest,
  (value, previous) => {
    if (props.standalone && value !== previous) handleCreateRequest()
  },
)
onMounted(() => {
  void load()
  window.addEventListener('qterm:snippet-create', handleCreateRequest)
  window.addEventListener('qterm:assets-imported', handleAssetsImported)
})
onBeforeUnmount(() => {
  window.removeEventListener('qterm:snippet-create', handleCreateRequest)
  window.removeEventListener('qterm:assets-imported', handleAssetsImported)
})
</script>

<template>
  <section class="snippets-settings flex h-full min-h-0 flex-col gap-4">
    <div v-if="!props.standalone" class="flex h-full min-h-0 flex-col gap-4">
      <QtermSearchToolbar
        v-model="searchQuery"
        :placeholder="t('settings.snippets.search.placeholder')"
        :search-aria-label="t('settings.snippets.search.aria')"
        :clear-aria-label="t('settings.snippets.search.clear')"
      >
        <QtermButton variant="outline" size="sm" type="button" @click="createGroup">
          <FolderPlus :size="14" /> {{ t('settings.snippets.action.createGroup') }}
        </QtermButton>
        <QtermButton size="sm" type="button" @click="createSnippet">
          <Plus :size="14" /> {{ t('settings.snippets.action.create') }}
        </QtermButton>
      </QtermSearchToolbar>
      <QtermFormMessage v-if="error && !open" variant="error">{{ error }}</QtermFormMessage>
      <QtermEmptyState
        v-if="loading"
        variant="outlined"
        :description="t('settings.snippets.loading')"
        role="status"
        busy
      />
      <QtermEmptyState
        v-else-if="!snippets.length && !commandGroups.length"
        variant="outlined"
        :title="t('settings.snippets.empty.title')"
        :description="t('settings.snippets.empty.description')"
      >
        <template #icon>
          <div class="grid size-8.5 place-items-center rounded-[9px] bg-qt-subtle">
            <Plus :size="18" />
          </div>
        </template>
      </QtermEmptyState>
      <QtermEmptyState
        v-else-if="searchQuery.trim() && !grouped.length"
        variant="outlined"
        :description="t('settings.snippets.empty.matching')"
      />
      <ScrollArea v-else type="hover" :scroll-hide-delay="600" class="min-h-0 flex-1">
        <div class="snippets-groups pr-2 pb-1">
          <Accordion v-model="expandedGroups" type="multiple" class="snippet-list-accordion">
            <AccordionItem
              v-for="[categoryID, title, items] in grouped"
              :key="categoryID"
              :value="categoryID || '__ungrouped__'"
              class="snippet-group"
              draggable="true"
              @dragstart="draggingGroup = title"
              @dragover.prevent
              @drop.stop.prevent="dropOnGroup(categoryID, title)"
              @dragend="draggingGroup = null"
            >
              <SettingsGroupHeader
                :title="title"
                :count="items.length"
                :folder="!!categoryID"
                title-class="snippet-group-title"
                data-snippet-group-toggle
              >
                <template v-if="commandGroups.some((group) => group.name === title)" #actions>
                  <QtermButton
                    variant="ghost"
                    size="icon-sm"
                    :aria-label="t('settings.snippets.action.editGroup', { name: title })"
                    @click.stop="editGroup(title)"
                    ><Pencil :size="14"
                  /></QtermButton>
                  <IconButton
                    danger
                    size="icon-sm"
                    class="snippet-delete-action"
                    :label="t('settings.snippets.action.deleteGroup', { name: title })"
                    @click.stop="removeGroup(title)"
                    ><Trash2 :size="14"
                  /></IconButton>
                </template>
              </SettingsGroupHeader>
              <AccordionContent class="pb-0">
                <div v-if="items.length" class="flex flex-col border-t border-qt-border">
                  <article
                    v-for="item in visibleSnippetGroupItems(
                      snippetGroupKey(categoryID),
                      items,
                      snippetGroupExpanded(categoryID),
                    )"
                    :key="item.id"
                    class="snippet-row relative flex items-center justify-between gap-2.5 px-3 py-2"
                    :draggable="!movingSnippet"
                    :class="{
                      'has-snippet-color': !!snippetColorStyle(item.color),
                      'snippet-drop-before': dropIndicator === item.id && dropPosition === 'before',
                      'snippet-drop-after': dropIndicator === item.id && dropPosition === 'after',
                    }"
                    :style="[
                      snippetColorStyle(item.color),
                      dropIndicator === item.id ? { '--snippet-drop-offset': `${dropLineOffset}px` } : undefined,
                    ]"
                    @dragstart.stop="startSnippetDrag($event, item.id)"
                    @dragend.stop="endSnippetDrag"
                    @dragover.stop="overSnippet($event, item.id)"
                    @drop.stop.prevent="dropSnippet(categoryID, item.id)"
                  >
                    <div class="snippet-row-main flex-1 min-w-0 text-sm">
                      <strong class="snippet-name text-sm"
                        ><SnippetIcon :icon="item.icon" /><span>{{ item.name }}</span
                        ><Pin v-if="item.pinned" :size="13" :aria-label="t('settings.snippets.pinned')" /></strong
                      ><code class="text-xs">{{ item.content }}</code>
                    </div>
                    <div class="snippet-row-actions">
                      <QtermButton
                        variant="ghost"
                        size="icon"
                        :aria-label="t('settings.snippets.action.copy', { name: item.name })"
                        @click="copySnippet(item)"
                        ><Copy :size="16" /></QtermButton
                      ><QtermButton
                        variant="ghost"
                        size="icon"
                        :aria-label="t('settings.snippets.action.edit', { name: item.name })"
                        @click="editSnippet(item)"
                        ><Pencil :size="16" /></QtermButton
                      ><QtermButton
                        variant="ghost"
                        size="icon"
                        class="snippet-delete-action"
                        :aria-label="t('settings.snippets.action.delete')"
                        @click="remove(item)"
                        ><Trash2 :size="16"
                      /></QtermButton>
                    </div>
                  </article>
                  <QtermLoadMore
                    v-if="snippetGroupExpanded(categoryID) && items.length"
                    class="px-3 py-2"
                    :visible="visibleSnippetGroupCount(snippetGroupKey(categoryID), items.length, true)"
                    :total="items.length"
                    :load-more-label="t('settings.snippets.pagination.loadMore')"
                    @load-more="loadMoreSnippetGroup(snippetGroupKey(categoryID), items.length)"
                  />
                </div>
              </AccordionContent>
            </AccordionItem>
          </Accordion>
        </div>
      </ScrollArea>
    </div>

    <QtermDialog :open="open" @update:open="(value) => !value && close()">
      <QtermDialogContent v-if="open" class="qterm-command-dialog snippets-dialog">
        <form @submit.prevent="save">
          <QtermDialogHeader
            :title="t(editing ? 'settings.snippets.dialog.editTitle' : 'settings.snippets.dialog.createTitle')"
            :description="t('settings.snippets.dialog.description')"
            @close="close"
          />
          <ScrollArea type="hover" :scroll-hide-delay="600" class="snippet-dialog-scroll"
            ><div class="dialog-body snippets-form text-sm flex-col">
              <div class="form-field min-w-0 grid gap-1.5 text-qt-muted">
                <Label for="snippet-name"
                  >{{ t('settings.snippets.field.name') }} <span class="required-mark text-qt-danger">*</span></Label
                >
                <div class="host-identity-field grid grid-cols-[auto_minmax(0,1fr)] gap-2 items-center">
                  <HostIconPicker v-model="icon" :label="t('settings.snippets.icon')">
                    <template #fallback><SnippetIcon /></template>
                  </HostIconPicker>
                  <QtermInput
                    id="snippet-name"
                    v-model="name"
                    :placeholder="t('settings.snippets.field.namePlaceholder')"
                    autofocus
                  />
                </div>
              </div>
              <div class="form-field min-w-0 grid gap-1.5 text-qt-muted">
                <Label for="snippet-content"
                  >{{ t('settings.snippets.field.content') }} <span class="required-mark text-qt-danger">*</span></Label
                ><SnippetContentEditor
                  id="snippet-content"
                  v-model="content"
                  :placeholder="t('settings.snippets.field.contentPlaceholder')"
                  :aria-label="t('settings.snippets.field.content')"
                /><small class="text-xs">{{ t('settings.snippets.field.contentHint') }}</small>
              </div>
              <div class="form-field min-w-0 grid gap-1.5 text-qt-muted">
                <Label for="snippet-group">{{ t('settings.snippets.field.group') }}</Label>
                <QtermCombobox
                  id="snippet-group"
                  :model-value="groupId || GROUP_NONE_VALUE"
                  :options="commandGroupOptions"
                  :placeholder="t('settings.snippets.field.groupPlaceholder')"
                  :aria-label="t('settings.snippets.field.group')"
                  @update:model-value="(value) => (groupId = value === GROUP_NONE_VALUE ? '' : String(value))"
                />
              </div>
              <div class="form-field snippet-execution-mode min-w-0 grid gap-1.5 text-qt-muted">
                <Label id="snippet-execution-label">{{ t('settings.snippets.executionMode') }}</Label>
                <SegmentedTabs
                  :model-value="autoExecute ? 'execute' : 'append'"
                  :label="t('settings.snippets.executionMode')"
                  :options="[
                    { value: 'execute', label: t('settings.snippets.executeNow') },
                    { value: 'append', label: t('settings.snippets.appendInput') },
                  ]"
                  @update:model-value="autoExecute = $event === 'execute'"
                />
                <small class="text-xs">{{
                  t(autoExecute ? 'settings.snippets.executeHint' : 'settings.snippets.appendHint')
                }}</small>
              </div>
              <div class="form-field min-w-0 grid gap-1.5 text-qt-muted">
                <Label id="snippet-color-label">{{ t('settings.snippets.color') }}</Label>

                <ColorTagField
                  :model-value="color"
                  :label="t('settings.snippets.color')"
                  :none-label="t('settings.snippets.colorNone')"
                  :custom-label="t('settings.snippets.customColor')"
                  :color-label="(value) => t('settings.snippets.colorValue', { color: value })"
                  @update:model-value="color = $event"
                />
              </div>
              <label class="settings-toggle-row snippet-pin-control c-p">
                <span>
                  <strong class="text-sm">{{ t('settings.snippets.pinned') }}</strong>
                  <small class="text-xs">{{ t('settings.snippets.pinnedHint') }}</small>
                </span>
                <QtermSwitch v-model="pinned" :aria-label="t('settings.snippets.pinned')" />
              </label>
              <Accordion v-model="expandedPanels" type="multiple" class="snippet-associations">
                <AccordionItem value="groups">
                  <AccordionTrigger class="text-sm">{{ t('settings.snippets.association.groups') }}</AccordionTrigger>
                  <AccordionContent class="text-sm">
                    <p class="snippet-association-hint mt-0 mr-0 mb-2.5 ml-0 text-qt-muted leading-[1.6] text-xs">
                      {{ t('settings.snippets.association.groupHint') }}
                    </p>
                    <QtermCombobox
                      id="snippet-asset-group"
                      :model-value="assetGroupId || GROUP_NONE_VALUE"
                      :options="assetGroupOptions"
                      :placeholder="t('settings.snippets.association.groupPlaceholder')"
                      :aria-label="t('settings.snippets.association.groupAria')"
                      @update:model-value="(value) => selectAssetGroup(value === GROUP_NONE_VALUE ? '' : String(value))"
                    />
                  </AccordionContent>
                </AccordionItem>
                <AccordionItem value="hosts">
                  <AccordionTrigger class="text-sm"
                    >{{ t('settings.snippets.association.hosts')
                    }}<small
                      v-if="hostIds.length"
                      class="snippet-association-count ml-1 text-qt-muted font-normal text-xs"
                      >{{ t('settings.snippets.association.count', { count: hostIds.length }) }}</small
                    ></AccordionTrigger
                  >
                  <AccordionContent class="text-sm">
                    <p class="snippet-association-hint mt-0 mr-0 mb-2.5 ml-0 text-qt-muted leading-[1.6] text-xs">
                      {{ t('settings.snippets.association.hostHint') }}
                    </p>
                    <QtermFormMessage v-if="assetError" variant="error">
                      {{ assetError }}
                      <QtermButton type="button" variant="outline" size="sm" @click="load">{{
                        t('settings.snippets.action.retry')
                      }}</QtermButton>
                    </QtermFormMessage>
                    <ScrollArea v-else type="hover" :scroll-hide-delay="600" class="snippet-hosts">
                      <SnippetHostTree :groups="groups" :hosts="hosts" :selected="hostIds" @select="selectHosts" />
                    </ScrollArea>
                  </AccordionContent>
                </AccordionItem>
              </Accordion>
              <QtermFormMessage v-if="error" variant="error">{{ error }}</QtermFormMessage>
            </div>
          </ScrollArea>
          <QtermDialogFooter>
            <QtermButton variant="outline" type="button" @click="close">{{
              t('settings.snippets.action.cancel')
            }}</QtermButton
            ><QtermButton type="submit" :disabled="saving">{{
              t(
                saving
                  ? 'settings.snippets.action.saveProgress'
                  : editing
                    ? 'settings.snippets.action.saveChanges'
                    : 'settings.snippets.action.create',
              )
            }}</QtermButton>
          </QtermDialogFooter>
        </form>
      </QtermDialogContent>
    </QtermDialog>
    <QtermDialog :open="groupDialogOpen" @update:open="(value) => !value && (groupDialogOpen = false)">
      <QtermDialogContent v-if="groupDialogOpen" class="qterm-command-dialog">
        <form @submit.prevent="saveGroup">
          <QtermDialogHeader
            :title="
              t(
                editingGroupId
                  ? 'settings.snippets.dialog.groupEditTitle'
                  : 'settings.snippets.dialog.groupCreateTitle',
              )
            "
            :description="t('settings.snippets.dialog.groupDescription')"
            @close="groupDialogOpen = false"
          />
          <div class="dialog-body">
            <div class="form-field min-w-0 grid gap-1.5 text-qt-muted">
              <Label for="snippet-group-name"
                >{{ t('settings.snippets.field.groupName') }} <span class="required-mark text-qt-danger">*</span></Label
              ><QtermInput
                id="snippet-group-name"
                v-model="groupName"
                :placeholder="t('settings.snippets.field.groupNamePlaceholder')"
                autofocus
              />
            </div>
            <QtermFormMessage v-if="groupError" variant="error">{{ groupError }}</QtermFormMessage>
          </div>
          <QtermDialogFooter>
            <QtermButton variant="outline" type="button" @click="groupDialogOpen = false">{{
              t('settings.snippets.action.cancel')
            }}</QtermButton
            ><QtermButton type="submit" :disabled="groupSaving">{{
              t(groupSaving ? 'settings.snippets.action.confirmProgress' : 'settings.snippets.action.confirm')
            }}</QtermButton>
          </QtermDialogFooter>
        </form>
      </QtermDialogContent>
    </QtermDialog>
    <DeleteConfirmDialog
      :open="!!deleteTarget"
      :title="t(deleteGroupId ? 'settings.snippets.delete.groupTitle' : 'settings.snippets.delete.snippetTitle')"
      :message="t('settings.snippets.delete.message', { name: deleteTarget?.name ?? '' })"
      :hint="deleteGroupId ? t('settings.snippets.delete.groupHint') : ''"
      :error="deleteError"
      :loading="deleteSaving"
      @close="deleteTarget = null"
      @confirm="confirmRemove"
    />
  </section>
</template>
