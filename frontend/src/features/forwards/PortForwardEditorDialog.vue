<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Plus } from '@lucide/vue'
import { IconButton, QtermButton, QtermInput, QtermNumberField, QtermSwitch, SegmentedTabs } from '@/components/common'
import { QtermCombobox, type QtermComboboxOption } from '@/components/common/form-controls'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '@/components/common/dialog'
import AssetPickerDialog from '@/components/dialogs/AssetPickerDialog.vue'
import PortForwardGroupDialog from './PortForwardGroupDialog.vue'
import { useI18n } from '@/locales'
import type { HostRecord } from '@/services/hosts'
import type { PortForwardConfig, PortForwardKind } from '../../services/forwards'
import { FORWARD_UNGROUPED_KEY, forwardHostLabel, forwardSummary } from './forwardView'

const props = withDefaults(
  defineProps<{
    open: boolean

    config?: PortForwardConfig | null

    hosts?: HostRecord[]

    activeHostId?: string

    groups?: string[]
  }>(),
  { config: null, hosts: () => [], activeHostId: '', groups: () => [] },
)

const emit = defineEmits<{
  'update:open': [open: boolean]
  submit: [config: PortForwardConfig]

  createGroup: [name: string]
}>()

const { t } = useI18n()

const draft = ref<PortForwardConfig>(emptyDraft())
const draftError = ref('')

function updateGroup(value: string | number): void {
  draft.value = { ...draft.value, group: value === FORWARD_UNGROUPED_KEY ? '' : String(value) }
}
const hostPickerOpen = ref(false)
const groupDialogOpen = ref(false)

function emptyDraft(): PortForwardConfig {
  return {
    id: '',
    name: '',
    group: '',
    kind: 'local',
    hostId: props.activeHostId,
    listenHost: '127.0.0.1',
    listenPort: 0,
    targetHost: '127.0.0.1',
    targetPort: 0,
    autoOpen: false,
  }
}

const listenScope = ref<'loopback' | 'any' | 'custom'>('loopback')

function scopeOfHost(host: string): 'loopback' | 'any' | 'custom' {
  if (host === '127.0.0.1') return 'loopback'
  if (host === '0.0.0.0') return 'any'
  return 'custom'
}

function resetDraft(): void {
  draft.value = props.config ? { ...props.config } : emptyDraft()
  listenScope.value = scopeOfHost(draft.value.listenHost)
  draftError.value = ''
  hostPickerOpen.value = false
}

watch(
  () => props.open,
  (open) => {
    if (open) resetDraft()
  },
  { immediate: true },
)

const listenScopeOptions = computed(() => [
  { value: 'loopback', label: t('shell.forwards.scopeLoopback') },
  { value: 'any', label: t('shell.forwards.scopeAny') },
  { value: 'custom', label: t('shell.forwards.scopeCustom') },
])

const kindOptions = computed(() => [
  { value: 'local', label: t('shell.forwards.kindLocal') },
  { value: 'remote', label: t('shell.forwards.kindRemote') },
  { value: 'dynamic', label: t('shell.forwards.kindDynamic') },
])

const kindHint = computed(() => {
  if (draft.value.kind === 'remote') return t('shell.forwards.hintRemote')
  if (draft.value.kind === 'dynamic') return t('shell.forwards.hintDynamic')
  return t('shell.forwards.hintLocal')
})

const groupOptions = computed<QtermComboboxOption[]>(() => [
  { value: FORWARD_UNGROUPED_KEY, label: t('shell.forwards.ungrouped') },
  ...props.groups.map((group) => ({ value: group, label: group })),
])

function handleGroupCreated(name: string): void {
  emit('createGroup', name)
  draft.value = { ...draft.value, group: name }
}

function setListenScope(value: string): void {
  if (value === 'loopback') {
    listenScope.value = 'loopback'
    draft.value = { ...draft.value, listenHost: '127.0.0.1' }
    return
  }
  if (value === 'any') {
    listenScope.value = 'any'
    draft.value = { ...draft.value, listenHost: '0.0.0.0' }
    return
  }
  if (value !== 'custom') return

  if (listenScope.value !== 'custom') draft.value = { ...draft.value, listenHost: '127.0.0.1' }
  listenScope.value = 'custom'
}

function chooseHost(choice: { kind: 'host'; host: HostRecord } | { kind: 'local' }): void {
  if (choice.kind !== 'host') return
  draft.value = { ...draft.value, hostId: choice.host.id }
  hostPickerOpen.value = false
}

function validateDraft(): string {
  const value = draft.value
  if (!value.hostId) return t('shell.forwards.errorHost')
  if (value.listenPort < 0 || value.listenPort > 65535) return t('shell.forwards.errorListenPort')
  if (value.kind !== 'dynamic') {
    if (!value.targetHost.trim()) return t('shell.forwards.errorTargetHost')
    if (value.targetPort <= 0 || value.targetPort > 65535) return t('shell.forwards.errorTargetPort')
  }
  return ''
}

function submit(): void {
  const error = validateDraft()
  draftError.value = error
  if (error) return
  const payload: PortForwardConfig = {
    ...draft.value,
    name: draft.value.name.trim(),
    group: draft.value.group.trim(),
    id: draft.value.id || `forward-${Date.now().toString(36)}`,
  }

  if (!payload.name) payload.name = forwardSummary(payload)
  emit('submit', payload)
  emit('update:open', false)
}
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => emit('update:open', open)">
    <QtermDialogContent v-if="props.open" size="lg">
      <QtermDialogHeader
        :title="props.config ? t('shell.forwards.editTitle') : t('shell.forwards.createTitle')"
        :close-disabled="false"
        @close="emit('update:open', false)"
      />
      <div class="dialog-body flex-col gap-4">
        <div class="grid gap-3 sm:grid-cols-2">
          <label class="flex min-w-0 flex-col gap-1">
            <span class="text-xs text-qt-muted">{{ t('shell.forwards.name') }}</span>
            <QtermInput
              v-model="draft.name"
              :aria-label="t('shell.forwards.name')"
              :placeholder="t('shell.forwards.namePlaceholder')"
              spellcheck="false"
            />
          </label>
          <div class="flex min-w-0 flex-col gap-1">
            <span class="text-xs text-qt-muted">{{ t('shell.forwards.group') }}</span>
            <div class="flex min-w-0 items-center gap-1.5">
              <QtermCombobox
                class="min-w-0 flex-1"
                :model-value="draft.group || FORWARD_UNGROUPED_KEY"
                :options="groupOptions"
                :placeholder="t('shell.forwards.groupPlaceholder')"
                v-bind="{ ariaLabel: t('shell.forwards.group') }"
                @update:model-value="updateGroup"
              />

              <IconButton
                variant="outline"
                size="icon"
                class="shrink-0"
                :label="t('shell.forwards.groupCreate')"
                @click="groupDialogOpen = true"
              >
                <Plus :size="16" aria-hidden="true" />
              </IconButton>
            </div>
          </div>
        </div>

        <div class="flex flex-col gap-1.5">
          <span class="text-xs text-qt-muted">{{ t('shell.forwards.kind') }}</span>
          <SegmentedTabs
            :model-value="draft.kind"
            :options="kindOptions"
            :label="t('shell.forwards.kind')"
            @update:model-value="(value) => (draft = { ...draft, kind: value as PortForwardKind })"
          />
        </div>

        <div class="flex flex-col gap-1.5">
          <span class="text-xs text-qt-muted">{{ t('shell.forwards.host') }}</span>
          <QtermButton variant="outline" type="button" class="justify-between" @click="hostPickerOpen = true">
            <span class="truncate">{{
              draft.hostId ? forwardHostLabel(draft, props.hosts) : t('shell.forwards.hostPlaceholder')
            }}</span>
          </QtermButton>
        </div>

        <div class="flex flex-col gap-1.5">
          <span class="text-xs font-medium text-qt-text">{{ t('shell.forwards.localService') }}</span>
          <SegmentedTabs
            :model-value="listenScope"
            :options="listenScopeOptions"
            :label="t('shell.forwards.listenScope')"
            @update:model-value="setListenScope"
          />

          <div class="grid gap-3" :class="{ 'sm:grid-cols-[minmax(0,1fr)_10rem]': listenScope === 'custom' }">
            <label v-if="listenScope === 'custom'" class="flex min-w-0 flex-col gap-1">
              <span class="text-[11px] text-qt-muted">{{ t('shell.forwards.listenHost') }}</span>
              <QtermInput
                v-model="draft.listenHost"
                class="font-mono text-xs"
                :aria-label="t('shell.forwards.listenHost')"
                spellcheck="false"
              />
            </label>
            <div class="flex min-w-0 flex-col gap-1">
              <span class="text-[11px] text-qt-muted">{{ t('shell.forwards.listenPort') }}</span>
              <QtermNumberField
                class="w-full"
                :model-value="draft.listenPort"
                :min="0"
                :max="65535"
                :step="1"
                :aria-label="t('shell.forwards.listenPort')"
                :decrease-label="t('shell.forwards.listenPort')"
                :increase-label="t('shell.forwards.listenPort')"
                @update:model-value="(value) => (draft = { ...draft, listenPort: value ?? 0 })"
              />
            </div>
          </div>
          <span v-if="draft.listenHost !== '127.0.0.1'" class="text-[11px] text-qt-warning">
            {{ t('shell.forwards.exposeWarning') }}
          </span>
        </div>

        <div v-if="draft.kind !== 'dynamic'" class="flex flex-col gap-1.5">
          <span class="text-xs font-medium text-qt-text">{{ t('shell.forwards.targetService') }}</span>
          <div class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_10rem]">
            <label class="flex min-w-0 flex-col gap-1">
              <span class="text-[11px] text-qt-muted">{{ t('shell.forwards.targetHost') }}</span>
              <QtermInput
                v-model="draft.targetHost"
                class="font-mono text-xs"
                :aria-label="t('shell.forwards.targetHost')"
                spellcheck="false"
              />
            </label>
            <div class="flex flex-col gap-1">
              <span class="text-[11px] text-qt-muted">{{ t('shell.forwards.targetPort') }}</span>
              <QtermNumberField
                class="w-full"
                :model-value="draft.targetPort"
                :min="1"
                :max="65535"
                :step="1"
                :aria-label="t('shell.forwards.targetPort')"
                :decrease-label="t('shell.forwards.targetPort')"
                :increase-label="t('shell.forwards.targetPort')"
                @update:model-value="(value) => (draft = { ...draft, targetPort: value ?? 0 })"
              />
            </div>
          </div>
        </div>

        <label class="flex items-center justify-between gap-3 text-xs">
          <span>{{ t('shell.forwards.autoOpen') }}</span>
          <QtermSwitch
            :model-value="draft.autoOpen"
            :aria-label="t('shell.forwards.autoOpen')"
            @update:model-value="(value) => (draft = { ...draft, autoOpen: value })"
          />
        </label>

        <p v-if="draftError" class="m-0 text-xs text-qt-danger" role="alert">{{ draftError }}</p>
        <div class="rounded-md border border-qt-border bg-qt-subtle px-2 py-1.5 text-[11px] text-qt-muted">
          <div class="mb-0.5">{{ t('shell.forwards.preview') }}</div>
          <code class="font-mono text-qt-text">{{ forwardSummary(draft) }}</code>
          <div class="mt-1">{{ kindHint }}</div>
        </div>
      </div>
      <QtermDialogFooter>
        <QtermButton variant="outline" type="button" @click="emit('update:open', false)">
          {{ t('shell.common.cancel') }}
        </QtermButton>
        <QtermButton type="button" @click="submit">{{ t('shell.common.confirm') }}</QtermButton>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>

  <PortForwardGroupDialog
    :open="groupDialogOpen"
    mode="create"
    :existing="props.groups"
    @update:open="(open) => (groupDialogOpen = open)"
    @submit="handleGroupCreated"
  />

  <AssetPickerDialog
    :open="hostPickerOpen"
    mode="connect"
    :title="t('shell.forwards.pickHostTitle')"
    :description="t('shell.forwards.pickHostDescription')"
    :hosts="props.hosts"
    :current="null"
    :show-local-terminal="false"
    @update:open="(open) => (hostPickerOpen = open)"
    @select="chooseHost"
  />
</template>
