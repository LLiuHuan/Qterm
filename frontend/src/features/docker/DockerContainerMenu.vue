<script setup lang="ts">
import { computed } from 'vue'
import { Activity, Info, Play, RotateCw, ScrollText, Square, SquareTerminal, Trash2 } from '@lucide/vue'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from '../../components/common/overlay-controls'
import { dockerStateLabelKey, type DockerContainer, type DockerContainerAction } from '../../services/docker'
import { useI18n } from '@/locales'

const props = defineProps<{
  container: DockerContainer

  busy: boolean
}>()

const emit = defineEmits<{
  action: [action: DockerContainerAction]
  detail: []
  logs: []

  stats: []
  terminal: []
  remove: []
}>()

const { t } = useI18n()

const running = computed(() => props.container.state === 'running')

const stateLabel = computed(() => {
  const key = dockerStateLabelKey(props.container.state)
  return key ? t(key) : props.container.status || props.container.state
})

const secondary = computed(() => props.container.ports || props.container.status || stateLabel.value)
</script>

<template>
  <ContextMenu>
    <ContextMenuTrigger as-child>
      <div
        class="mx-1 flex min-w-0 cursor-default items-center gap-2 rounded-md px-1.5 py-2 transition-colors [&:not(:first-child)]:mt-0.5 hover:bg-qt-hover focus-visible:bg-qt-hover focus-visible:outline-none"
        :data-container-id="props.container.id"
        :data-container-state="props.container.state"
        @dblclick="emit('detail')"
      >
        <span
          class="size-1.5 shrink-0 rounded-full"
          :class="running ? 'bg-qt-success' : 'bg-qt-muted/70'"
          role="img"
          :aria-label="stateLabel"
          :title="stateLabel"
        />
        <div class="min-w-0 flex-1">
          <span class="block truncate text-xs text-qt-text" :title="props.container.name">{{
            props.container.name
          }}</span>
          <span class="block truncate text-xs text-qt-muted" :title="secondary">{{ secondary }}</span>
        </div>
      </div>
    </ContextMenuTrigger>
    <ContextMenuContent class="w-40">
      <ContextMenuItem :disabled="props.busy" @select="emit('detail')">
        <Info :size="14" aria-hidden="true" />{{ t('shell.docker.detail') }}
      </ContextMenuItem>
      <ContextMenuItem :disabled="props.busy" @select="emit('logs')">
        <ScrollText :size="14" aria-hidden="true" />{{ t('shell.docker.logs') }}
      </ContextMenuItem>
      <ContextMenuItem :disabled="props.busy" @select="emit('stats')">
        <Activity :size="14" aria-hidden="true" />{{ t('shell.docker.monitor') }}
      </ContextMenuItem>
      <ContextMenuItem :disabled="props.busy" @select="emit('terminal')">
        <SquareTerminal :size="14" aria-hidden="true" />{{ t('shell.docker.terminal') }}
      </ContextMenuItem>
      <ContextMenuSeparator />

      <ContextMenuItem v-if="running" :disabled="props.busy" @select="emit('action', 'stop')">
        <Square :size="14" aria-hidden="true" />{{ t('shell.docker.stop') }}
      </ContextMenuItem>
      <ContextMenuItem v-else :disabled="props.busy" @select="emit('action', 'start')">
        <Play :size="14" aria-hidden="true" />{{ t('shell.docker.start') }}
      </ContextMenuItem>
      <ContextMenuItem :disabled="props.busy" @select="emit('action', 'restart')">
        <RotateCw :size="14" aria-hidden="true" />{{ t('shell.docker.restart') }}
      </ContextMenuItem>
      <ContextMenuSeparator />
      <ContextMenuItem variant="destructive" :disabled="props.busy" @select="emit('remove')">
        <Trash2 :size="14" aria-hidden="true" />{{ t('shell.docker.delete') }}
      </ContextMenuItem>
    </ContextMenuContent>
  </ContextMenu>
</template>
