<script setup lang="ts">
import { FileCode, LoaderCircle, Play, RotateCw, ScrollText, Square, SquareActivity, Trash2 } from '@lucide/vue'
import { ContextMenuContent, ContextMenuItem, ContextMenuSeparator } from '../../components/common/overlay-controls'
import type { DockerComposeAction } from '../../services/docker'
import { useI18n } from '@/locales'

const props = defineProps<{
  running: boolean

  busy: boolean

  busyAction?: DockerComposeAction | ''
}>()

const emit = defineEmits<{
  start: []
  stop: []
  restart: []
  logs: []
  stats: []
  compose: []

  remove: []
}>()

const { t } = useI18n()

function runningAction(action: DockerComposeAction): boolean {
  return props.busyAction === action
}
</script>

<template>
  <ContextMenuContent class="w-48">
    <ContextMenuItem v-if="props.running" :disabled="props.busy" @select="emit('stop')">
      <LoaderCircle v-if="runningAction('stop')" :size="14" class="animate-spin" aria-hidden="true" />
      <Square v-else :size="14" aria-hidden="true" />
      {{ t('shell.docker.stopGroup') }}
    </ContextMenuItem>
    <ContextMenuItem v-else :disabled="props.busy" @select="emit('start')">
      <LoaderCircle v-if="runningAction('up')" :size="14" class="animate-spin" aria-hidden="true" />
      <Play v-else :size="14" aria-hidden="true" />
      {{ t('shell.docker.startGroup') }}
    </ContextMenuItem>

    <ContextMenuItem :disabled="props.busy || !props.running" @select="emit('restart')">
      <LoaderCircle v-if="runningAction('restart')" :size="14" class="animate-spin" aria-hidden="true" />
      <RotateCw v-else :size="14" aria-hidden="true" />
      {{ t('shell.docker.restartGroup') }}
    </ContextMenuItem>

    <ContextMenuItem :disabled="props.busy" @select="emit('logs')">
      <ScrollText :size="14" aria-hidden="true" />
      {{ t('shell.docker.groupLogs') }}
    </ContextMenuItem>
    <ContextMenuItem :disabled="props.busy" @select="emit('stats')">
      <SquareActivity :size="14" aria-hidden="true" />
      {{ t('shell.docker.groupStats') }}
    </ContextMenuItem>
    <ContextMenuItem :disabled="props.busy" @select="emit('compose')">
      <FileCode :size="14" aria-hidden="true" />
      {{ t('shell.docker.groupComposeFile') }}
    </ContextMenuItem>
    <ContextMenuSeparator />

    <ContextMenuItem variant="destructive" :disabled="props.busy" @select="emit('remove')">
      <Trash2 :size="14" aria-hidden="true" />
      {{ t('shell.docker.deleteGroup') }}
    </ContextMenuItem>
  </ContextMenuContent>
</template>
