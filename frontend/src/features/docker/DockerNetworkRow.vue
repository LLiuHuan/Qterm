<script setup lang="ts">
import { computed } from 'vue'
import { Info, Network, Trash2 } from '@lucide/vue'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from '../../components/common/overlay-controls'
import { dockerShortId, type DockerNetwork } from '../../services/docker'
import { useI18n } from '@/locales'

const props = defineProps<{
  network: DockerNetwork

  busy: boolean
}>()

const emit = defineEmits<{ detail: []; remove: [] }>()

const { t } = useI18n()

const meta = computed(() => [props.network.driver.trim(), props.network.scope.trim()].filter(Boolean).join(' · '))

const removable = computed(() => !props.network.builtIn && !!props.network.id)
</script>

<template>
  <ContextMenu>
    <ContextMenuTrigger as-child>
      <div
        class="mx-1 flex min-w-0 cursor-default items-center gap-2 rounded-md px-1.5 py-2 transition-colors [&:not(:first-child)]:mt-0.5 hover:bg-qt-hover focus-visible:bg-qt-hover focus-visible:outline-none"
        :data-network-name="props.network.name"
        :data-network-in-use="props.network.inUse ? 'true' : 'false'"
        :data-network-built-in="props.network.builtIn ? 'true' : 'false'"
        @dblclick="emit('detail')"
      >
        <Network :size="14" class="shrink-0 text-qt-muted" aria-hidden="true" />
        <div class="min-w-0 flex-1">
          <span class="block truncate text-xs text-qt-text" :title="props.network.name">{{ props.network.name }}</span>
          <span class="block truncate text-xs text-qt-muted" :title="props.network.id">{{
            dockerShortId(props.network.id) || '—'
          }}</span>
        </div>

        <span
          v-if="props.network.builtIn"
          class="shrink-0 rounded-full bg-qt-hover px-1.5 text-xs leading-4 text-qt-muted"
          :title="t('shell.docker.builtInNetworkHint')"
          >{{ t('shell.docker.builtInNetwork') }}</span
        >
        <span v-if="meta" class="shrink-0 text-xs text-qt-muted" :title="t('shell.docker.networkDriver')">{{
          meta
        }}</span>
      </div>
    </ContextMenuTrigger>
    <ContextMenuContent class="w-40">
      <ContextMenuItem :disabled="props.busy" @select="emit('detail')">
        <Info :size="14" aria-hidden="true" />{{ t('shell.docker.detail') }}
      </ContextMenuItem>
      <ContextMenuSeparator />
      <ContextMenuItem variant="destructive" :disabled="props.busy || !removable" @select="emit('remove')">
        <Trash2 :size="14" aria-hidden="true" />{{ t('shell.docker.delete') }}
      </ContextMenuItem>
    </ContextMenuContent>
  </ContextMenu>
</template>
