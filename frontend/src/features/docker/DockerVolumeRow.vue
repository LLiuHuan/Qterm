<script setup lang="ts">
import { Database, Info, Trash2 } from '@lucide/vue'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from '../../components/common/overlay-controls'
import type { DockerVolume } from '../../services/docker'
import { useI18n } from '@/locales'

const props = defineProps<{
  volume: DockerVolume

  busy: boolean
}>()

const emit = defineEmits<{ detail: []; remove: [] }>()

const { t } = useI18n()
</script>

<template>
  <ContextMenu>
    <ContextMenuTrigger as-child>
      <div
        class="mx-1 flex min-w-0 cursor-default items-center gap-2 rounded-md px-1.5 py-2 transition-colors [&:not(:first-child)]:mt-0.5 hover:bg-qt-hover focus-visible:bg-qt-hover focus-visible:outline-none"
        :data-volume-name="props.volume.name"
        :data-volume-in-use="props.volume.inUse ? 'true' : 'false'"
        @dblclick="emit('detail')"
      >
        <Database :size="14" class="shrink-0 text-qt-muted" aria-hidden="true" />
        <div class="min-w-0 flex-1">
          <span class="block truncate text-xs text-qt-text" :title="props.volume.name">{{ props.volume.name }}</span>

          <span class="block truncate text-xs text-qt-muted" :title="props.volume.mountpoint">{{
            props.volume.mountpoint || '—'
          }}</span>
        </div>
        <span
          v-if="props.volume.driver"
          class="shrink-0 text-xs text-qt-muted"
          :title="t('shell.docker.volumeDriver')"
          >{{ props.volume.driver }}</span
        >
      </div>
    </ContextMenuTrigger>
    <ContextMenuContent class="w-40">
      <ContextMenuItem :disabled="props.busy" @select="emit('detail')">
        <Info :size="14" aria-hidden="true" />{{ t('shell.docker.detail') }}
      </ContextMenuItem>
      <ContextMenuSeparator />
      <ContextMenuItem variant="destructive" :disabled="props.busy" @select="emit('remove')">
        <Trash2 :size="14" aria-hidden="true" />{{ t('shell.docker.delete') }}
      </ContextMenuItem>
    </ContextMenuContent>
  </ContextMenu>
</template>
