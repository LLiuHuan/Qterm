<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Laptop } from '@lucide/vue'
import { HostIcon, QtermButton, QtermSearchToolbar } from '../common'
import { QtermDialog, QtermDialogContent, QtermDialogHeader } from '../common/dialog'
import { ScrollArea } from '../common/control-primitives'
import type { HostRecord } from '@/services/hosts.ts'
import { useI18n } from '@/locales'

export type AssetChoice = { kind: 'host'; host: HostRecord } | { kind: 'local' }

export type AssetPickerMode = 'split' | 'connect'

export type AssetPickerDirection = 'left' | 'right' | 'top' | 'bottom'

interface AssetEntry {
  key: string
  choice: AssetChoice
  name: string
  address: string

  icon: string | undefined

  protocol: string | undefined

  pinned: boolean

  haystack: string
}

const LOCAL_TERMINAL_KEY = 'local-terminal'

const LOCAL_TERMINAL_ADDRESS = '127.0.0.1'

const props = withDefaults(
  defineProps<{
    open: boolean

    mode?: AssetPickerMode

    direction?: AssetPickerDirection
    hosts: HostRecord[]

    current: AssetChoice | null

    showAddress?: boolean

    showLocalTerminal?: boolean

    title?: string

    description?: string
  }>(),
  {
    mode: 'split',
    direction: 'right',
    showAddress: true,
    showLocalTerminal: true,
    title: undefined,
    description: undefined,
  },
)

const emit = defineEmits<{
  'update:open': [value: boolean]
  select: [choice: AssetChoice]
}>()

const { t } = useI18n()
const query = ref('')

watch(
  () => props.open,
  (open) => {
    if (open) query.value = ''
  },
)

function hostAddress(host: HostRecord): string {
  return `${host.address}:${host.port}`
}

function hostEntry(host: HostRecord, pinned: boolean): AssetEntry {
  return {
    key: `${pinned ? 'current' : 'host'}-${host.id}`,
    choice: { kind: 'host', host },
    name: host.name,
    address: props.showAddress === false ? '' : hostAddress(host),
    icon: host.icon,
    protocol: host.protocol,
    pinned,
    haystack: `${host.name}\n${host.address}\n${host.username}\n${host.port}`.toLocaleLowerCase(),
  }
}

const localTerminalEntry = computed<AssetEntry>(() => {
  const name = t('shell.assetPicker.localTerminal')
  const address = props.showAddress === false ? '' : LOCAL_TERMINAL_ADDRESS
  return {
    key: LOCAL_TERMINAL_KEY,
    choice: { kind: 'local' },
    name,
    address,
    icon: '',
    protocol: undefined,
    pinned: false,
    haystack: `${name}\n${LOCAL_TERMINAL_ADDRESS}`.toLocaleLowerCase(),
  }
})

const entries = computed<AssetEntry[]>(() => {
  const list: AssetEntry[] = []

  const current = props.mode === 'connect' ? null : props.current
  if (current?.kind === 'local') {
    const name = t('shell.session.localTitle')
    list.push({
      key: 'current-local',
      choice: current,
      name,
      address: '',
      icon: '',
      protocol: undefined,
      pinned: true,
      haystack: name.toLocaleLowerCase(),
    })
  } else if (current?.kind === 'host') {
    list.push(hostEntry(current.host, true))
  }

  if (props.showLocalTerminal) list.push(localTerminalEntry.value)
  for (const host of props.hosts) list.push(hostEntry(host, false))
  return list
})

const visibleEntries = computed<AssetEntry[]>(() => {
  const value = query.value.trim().toLocaleLowerCase()
  if (!value) return entries.value
  return entries.value.filter((entry) => entry.haystack.includes(value))
})

const showEmptyHint = computed(() =>
  query.value.trim() ? visibleEntries.value.length === 0 : props.hosts.length === 0,
)

const directionLabel = computed(() => {
  const key =
    props.direction === 'left'
      ? 'shell.splitPicker.directionLeft'
      : props.direction === 'top'
        ? 'shell.splitPicker.directionTop'
        : props.direction === 'bottom'
          ? 'shell.splitPicker.directionBottom'
          : 'shell.splitPicker.directionRight'
  return t(key)
})

const title = computed(() =>
  props.title !== undefined
    ? props.title
    : props.mode === 'connect'
      ? t('shell.workspace.quickConnect')
      : t('shell.splitPicker.title'),
)

const description = computed(() => {
  if (props.description !== undefined) return props.description
  return props.mode === 'connect'
    ? t('shell.assetPicker.connectDescription')
    : t('shell.splitPicker.description', { direction: directionLabel.value })
})

const emptyLabel = computed(() => (query.value.trim() ? t('shell.splitPicker.noMatch') : t('shell.splitPicker.empty')))

function choose(choice: AssetChoice): void {
  emit('select', choice)
  emit('update:open', false)
}
</script>

<template>
  <QtermDialog :open="props.open" @update:open="emit('update:open', $event)">
    <QtermDialogContent
      v-if="props.open"
      size="md"
      class="asset-picker-dialog"
      :close-on-escape="true"
      :show-close-button="false"
    >
      <QtermDialogHeader :title="title" :description="description" @close="emit('update:open', false)" />

      <div class="flex flex-col px-4 pb-4">
        <QtermSearchToolbar
          v-model="query"
          class="mt-3"
          :placeholder="t('shell.splitPicker.placeholder')"
          :search-aria-label="t('shell.splitPicker.search')"
          :clear-aria-label="t('shell.splitPicker.clear')"
        />

        <ScrollArea class="-mx-[7px] mt-2 h-72" type="hover">
          <div class="flex flex-col gap-px px-[7px] pb-1">
            <p v-if="showEmptyHint" class="px-2 py-6 text-center text-xs text-qt-muted">
              {{ emptyLabel }}
            </p>
            <QtermButton
              v-for="entry in visibleEntries"
              :key="entry.key"
              variant="ghost"
              class="host-row"
              :class="{ 'has-address': Boolean(entry.address), 'is-selected': entry.pinned }"
              :data-asset-choice="entry.key"
              :data-asset-choice-pinned="entry.pinned ? 'true' : undefined"
              :data-asset-choice-fixed="entry.key === LOCAL_TERMINAL_KEY ? 'true' : undefined"
              :title="entry.address ? `${entry.name} (${entry.address})` : entry.name"
              @click="choose(entry.choice)"
            >
              <HostIcon
                v-if="entry.icon || entry.choice.kind === 'host'"
                :icon="entry.icon"
                :protocol="entry.protocol"
                :size="14"
              />
              <Laptop v-else :size="14" aria-hidden="true" />
              <span class="host-row-main min-w-0 flex flex-1 flex-col gap-px">
                <span
                  class="host-name flex items-center gap-1.5 text-xs overflow-hidden text-ellipsis whitespace-nowrap"
                >
                  {{ entry.name }}
                  <span v-if="entry.pinned" class="text-qt-accent">{{ t('shell.splitPicker.current') }}</span>
                </span>
                <span
                  v-if="entry.address"
                  class="host-address overflow-hidden text-ellipsis whitespace-nowrap text-qt-muted text-xs"
                  >{{ entry.address }}</span
                >
              </span>
            </QtermButton>
          </div>
        </ScrollArea>
      </div>
    </QtermDialogContent>
  </QtermDialog>
</template>
