<script setup lang="ts">
import { QtermButton } from '../common'
import { ArrowRightLeft, Copy, Folder, Link, Pencil, Trash2 } from '@lucide/vue'
import AssetContextMenu from '../common/overlay-controls/AssetContextMenu.vue'
import HostIcon from '../common/icon/HostIcon.vue'
import { tagColorStyle } from '../../lib/colorTag'
import type { HostRecord } from '@/services/hosts.ts'
import { computed } from 'vue'
import { useI18n } from '@/locales'

const props = defineProps<{
  host: HostRecord
  selected?: boolean
  showAddress?: boolean
}>()

const emit = defineEmits<{
  select: [id: string]
  open: [host: HostRecord]
  edit: [host: HostRecord]
  delete: [host: HostRecord]
  copy: [host: HostRecord, kind: 'ip' | 'connection' | 'id']
}>()
const { t } = useI18n()

const colorStyle = computed(() => tagColorStyle(props.host.color))

const actions = computed(() => [
  { id: 'open', label: t('shell.asset.connect'), icon: Link },
  { id: 'tunnel', label: t('shell.asset.tunnel'), icon: ArrowRightLeft, disabled: true },
  { id: 'sftp', label: t('shell.asset.sftp'), icon: Folder, disabled: true },
  { id: 'copy-ip', label: t('shell.asset.copyIp'), icon: Copy, separatorBefore: true },
  { id: 'copy-id', label: t('shell.asset.copyId'), icon: Copy },
  { id: 'edit', label: t('shell.asset.edit'), icon: Pencil, separatorBefore: true },
  { id: 'copy', label: t('shell.asset.copy'), icon: Copy },
  { id: 'delete', label: t('shell.asset.delete'), icon: Trash2, destructive: true },
])

function handleAction(id: string): void {
  if (id === 'open') emit('open', props.host)
  else if (id === 'edit') emit('edit', props.host)
  else if (id === 'delete') emit('delete', props.host)
  else if (id === 'copy-ip') emit('copy', props.host, 'ip')
  else if (id === 'copy-id') emit('copy', props.host, 'id')
  else if (id === 'copy') emit('copy', props.host, 'connection')
}

const addressLine = computed(() => {
  const base = `${props.host.address}:${props.host.port}`
  const tags = (props.host.tags ?? []).filter((tag) => tag.trim() !== '')
  return tags.length > 0 ? `${base} · ${tags.join('、')}` : base
})

const hostTitle = computed(() => {
  const connection = `${props.host.username}@${props.host.address}:${props.host.port}`
  const base = t('shell.asset.hostTitle', { connection })
  const tags = props.host.tags ?? []
  return tags.length > 0 ? `${base} · ${tags.join(', ')}` : base
})

const hostAriaLabel = computed(() => {
  const connection = `${props.host.username}@${props.host.address}:${props.host.port}`
  const base = t('shell.asset.hostAria', { name: props.host.name, connection })
  const tags = props.host.tags ?? []
  return tags.length > 0 ? `${base} · ${tags.join(', ')}` : base
})
</script>

<template>
  <div class="host-row-shell relative min-w-0 my-[1px]" data-asset-kind="host" :data-asset-id="props.host.id">
    <AssetContextMenu :actions="actions" @action="handleAction">
      <QtermButton
        variant="ghost"
        class="host-row relative"
        :class="{
          'is-selected': props.selected,
          'has-address': props.showAddress,
          'has-host-color': !!colorStyle,
        }"
        :style="colorStyle"
        :aria-label="hostAriaLabel"
        :title="hostTitle"
        @click="emit('select', props.host.id)"
        @dblclick="emit('open', props.host)"
      >
        <span
          v-if="colorStyle"
          class="host-color-tag pointer-events-none absolute inset-y-0 left-0 w-[3px] rounded-full bg-(--tag-color)"
          aria-hidden="true"
        />
        <HostIcon :icon="props.host.icon" :protocol="props.host.protocol" />
        <span class="host-row-main min-w-0 flex flex-1 flex-col gap-px">
          <span class="host-name text-sm overflow-hidden text-ellipsis whitespace-nowrap">{{ props.host.name }}</span>

          <span
            v-if="props.showAddress !== false"
            class="host-address overflow-hidden text-ellipsis whitespace-nowrap text-qt-muted text-xs"
            >{{ addressLine }}</span
          >
        </span>
      </QtermButton>
    </AssetContextMenu>
  </div>
</template>
