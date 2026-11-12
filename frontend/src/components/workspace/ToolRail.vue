<script setup lang="ts">
import {
  Activity,
  Braces,
  ClipboardCopy,
  ClipboardPaste,
  FolderOpen,
  History,
  MonitorUp,
  ScrollText,
  Palette,
  Gauge,
  Server,
  KeyRound,
  Route,
  Scaling,
  Search,
  Sparkles,
  Unplug,
  Layers,
} from '@lucide/vue'
import { DockerIcon } from '../../components/common/icon'
import type { Component } from 'vue'
import { computed } from 'vue'
import { Separator } from '../common/control-primitives'
import IconButton from '../common/button/IconButton.vue'
import RefreshIconButton from '../common/button/RefreshIconButton.vue'
import type { ToolPanel } from '@/types/qterm.ts'
import { useI18n } from '@/locales'

const props = withDefaults(
  defineProps<{
    activeTool: ToolPanel | null
    canReconnect: boolean
    canDisconnect: boolean

    localSession: boolean

    sshCapable?: boolean

    disabledTools?: readonly ToolPanel[]

    rdpActive?: boolean

    rdpWindowAvailable?: boolean

    rdpWindowLabel?: string

    vncActive?: boolean

    vncFit?: boolean

    rdpCanPaste?: boolean
    rdpCanCopy?: boolean
  }>(),
  {
    rdpActive: false,
    rdpWindowAvailable: true,
    rdpWindowLabel: '',
    rdpCanPaste: true,
    rdpCanCopy: true,
    vncActive: false,
    vncFit: true,
    disabledTools: () => [],
  },
)

const emit = defineEmits<{
  toggleTool: [tool: ToolPanel]
  reconnect: []
  disconnect: []
  rdpPaste: []
  rdpCopy: []
  rdpWindow: []
  rdpReconnect: []
  rdpDisconnect: []
  rdpLog: []
  vncFit: []
  vncPaste: []
}>()
const { t } = useI18n()

const tools = computed<Array<{ id: ToolPanel; label: string; icon: Component; disabled: boolean }>>(() => {
  const disabled = (id: ToolPanel) => props.disabledTools.includes(id)
  const entry = (id: ToolPanel, label: string, icon: Component) => ({ id, label, icon, disabled: disabled(id) })
  const snippetTool = entry('snippets', t('shell.toolRail.snippets'), Braces)

  const aiTool = entry('ai', t('shell.aiChat.title'), Sparkles)
  if (props.localSession) return [snippetTool, aiTool]
  if (props.sshCapable === false) return [snippetTool, aiTool]
  return [
    entry('resources', t('shell.toolRail.resources'), Activity),
    snippetTool,
    entry('sftp', t('shell.toolRail.sftp'), FolderOpen),
    entry('docker', t('shell.docker.title'), DockerIcon),
    entry('tmux', t('shell.tmux.title'), Layers),
    entry('tunnels', t('shell.toolRail.tunnels'), Route),
    entry('otp', t('shell.toolRail.otp'), KeyRound),
    entry('processes', t('shell.monitor.tabProcesses'), Gauge),

    entry('services', t('shell.services.title'), Server),
    aiTool,
  ]
})
</script>

<template>
  <aside
    class="tool-rail flex min-h-0 flex-col items-start gap-1.25 px-1 py-1.75 border-l-d"
    :aria-label="t('shell.toolRail.aria')"
  >
    <template v-if="props.rdpActive && props.vncActive">
      <IconButton
        tooltip-side="left"
        :label="props.vncFit ? t('shell.vnc.actualSize') : t('shell.vnc.fitToWindow')"
        :active="props.vncFit"
        :pressed="props.vncFit"
        @click="emit('vncFit')"
      >
        <Scaling :size="17" aria-hidden="true" />
      </IconButton>

      <IconButton tooltip-side="left" :label="t('shell.rdp.pasteToRemote')" @click="emit('vncPaste')">
        <ClipboardPaste :size="17" aria-hidden="true" />
      </IconButton>
    </template>
    <template v-if="props.rdpActive && !props.vncActive">
      <IconButton
        tooltip-side="left"
        :label="t('shell.rdp.pasteToRemote')"
        :disabled="!props.rdpCanPaste"
        @click="emit('rdpPaste')"
      >
        <ClipboardPaste :size="17" aria-hidden="true" />
      </IconButton>
      <IconButton
        tooltip-side="left"
        :label="t('shell.rdp.copyFromRemote')"
        :disabled="!props.rdpCanCopy"
        @click="emit('rdpCopy')"
      >
        <ClipboardCopy :size="17" aria-hidden="true" />
      </IconButton>
      <IconButton
        tooltip-side="left"
        v-if="props.rdpWindowAvailable"
        :label="props.rdpWindowLabel || t('shell.rdp.openWindow')"
        @click="emit('rdpWindow')"
      >
        <MonitorUp :size="17" aria-hidden="true" />
      </IconButton>

      <IconButton tooltip-side="left" :label="t('shell.rdp.logOpen')" @click="emit('rdpLog')">
        <ScrollText :size="17" aria-hidden="true" />
      </IconButton>
    </template>

    <template v-else-if="!props.rdpActive">
      <IconButton
        tooltip-side="left"
        v-for="tool in tools"
        :key="tool.id"
        :label="tool.disabled ? `${tool.label} · ${t('shell.adhoc.toolUnavailable')}` : tool.label"
        :disabled="tool.disabled"
        :active="props.activeTool === tool.id"
        :pressed="props.activeTool === tool.id"
        @click="emit('toggleTool', tool.id)"
      >
        <component :is="tool.icon" :size="17" aria-hidden="true" />
      </IconButton>
    </template>
    <template v-if="!props.rdpActive && !props.localSession">
      <Separator class="tool-rail-divider self-center" />
      <IconButton
        tooltip-side="left"
        :label="t('shell.toolRail.history')"
        :active="props.activeTool === 'history'"
        :pressed="props.activeTool === 'history'"
        @click="emit('toggleTool', 'history')"
      >
        <History :size="17" aria-hidden="true" />
      </IconButton>
      <IconButton tooltip-side="left" :label="t('shell.toolRail.findDisabled')" disabled>
        <Search :size="17" aria-hidden="true" />
      </IconButton>
    </template>
    <div class="flex-1"></div>

    <template v-if="props.rdpActive">
      <RefreshIconButton
        mode="action"
        tooltip-side="left"
        :label="t('shell.rdp.reconnect')"
        size="icon"
        icon-class="size-[17px]"
        @click="emit('rdpReconnect')"
      />
      <IconButton tooltip-side="left" :label="t('shell.rdp.disconnect')" @click="emit('rdpDisconnect')">
        <Unplug :size="17" aria-hidden="true" />
      </IconButton>
    </template>
    <template v-else>
      <RefreshIconButton
        mode="action"
        tooltip-side="left"
        :label="t('shell.toolRail.reconnect')"
        size="icon"
        icon-class="size-[17px]"
        :disabled="!props.canReconnect"
        @click="emit('reconnect')"
      />
      <IconButton
        tooltip-side="left"
        :label="t('shell.toolRail.disconnect')"
        :disabled="!props.canDisconnect"
        @click="emit('disconnect')"
      >
        <Unplug :size="17" aria-hidden="true" />
      </IconButton>
      <IconButton
        tooltip-side="left"
        :label="t('shell.toolRail.theme')"
        :active="props.activeTool === 'themes'"
        :pressed="props.activeTool === 'themes'"
        @click="emit('toggleTool', 'themes')"
      >
        <Palette :size="17" aria-hidden="true" />
      </IconButton>
    </template>
  </aside>
</template>
