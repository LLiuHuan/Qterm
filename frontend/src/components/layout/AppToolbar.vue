<script setup lang="ts">
import { AppWindow, FolderOpen, LockKeyhole, PlugZap, Server, Settings, SquareTerminal, Terminal } from '@lucide/vue'
import { ExportIcon, ImportIcon } from '@/components/common/icon'
import { Moon, PanelLeftClose, PanelLeftOpen, Sun } from 'lucide'
import { IconButton, MorphingIcon } from '../common'
import { useI18n } from '@/locales'
import { armCurrentWindowDrag } from '../../services/sessionWindows'
import UpdateBell from '../../features/updates/UpdateBell.vue'

const props = defineProps<{
  sidebarCollapsed: boolean
  isDark: boolean
  sftpActive?: boolean

  assetLibraryMode?: boolean

  assetLibraryActive?: boolean

  appLockEnabled?: boolean
}>()
const { t } = useI18n()

const emit = defineEmits<{
  toggleSidebar: []
  toggleAssetLibrary: []
  createTerminal: []
  createLocalTerminal: []
  temporaryConnection: []
  importConfig: []
  exportConfig: []
  openSftp: []
  createWindow: []
  toggleMaximise: []
  openSettings: []
  toggleTheme: []
  lockApp: []
}>()

function handleTitlebarDoubleClick(event: MouseEvent): void {
  if ((event.target as HTMLElement).closest('.topbar-actions')) return
  event.preventDefault()
  event.stopPropagation()
  emit('toggleMaximise')
}
</script>

<template>
  <header
    class="topbar relative z-10! flex min-w-0 items-center gap-[0.3125rem] px-2.5 select-none border-b-d bg-qt-bar"
    @dblclick="handleTitlebarDoubleClick"
  >
    <div class="window-drag-region" aria-hidden="true" @mousedown="armCurrentWindowDrag"></div>
    <div class="topbar-actions topbar-actions--primary">
      <IconButton
        v-if="props.assetLibraryMode"
        :label="t('shell.assetLibrary.toggle')"
        :active="props.assetLibraryActive"
        :pressed="props.assetLibraryActive"
        @click="$emit('toggleAssetLibrary')"
      >
        <Server :size="16" aria-hidden="true" />
      </IconButton>
      <IconButton
        v-else
        :label="props.sidebarCollapsed ? t('shell.toolbar.expandSidebar') : t('shell.toolbar.collapseSidebar')"
        :pressed="!props.sidebarCollapsed"
        @click="$emit('toggleSidebar')"
      >
        <MorphingIcon
          :icon="props.sidebarCollapsed ? PanelLeftOpen : PanelLeftClose"
          :size="16"
          reduced-motion="user"
          aria-hidden="true"
        />
      </IconButton>
      <IconButton :label="t('shell.toolbar.newTerminal')" @click="$emit('createTerminal')">
        <SquareTerminal :size="16" aria-hidden="true" />
      </IconButton>
      <IconButton :label="t('shell.toolbar.localTerminal')" @click="$emit('createLocalTerminal')">
        <Terminal :size="16" aria-hidden="true" />
      </IconButton>

      <IconButton :label="t('shell.importConfig.title')" data-toolbar-import @click="$emit('importConfig')">
        <ImportIcon :size="16" aria-hidden="true" />
      </IconButton>

      <IconButton :label="t('shell.exportConfig.title')" data-toolbar-export @click="$emit('exportConfig')">
        <ExportIcon :size="16" aria-hidden="true" />
      </IconButton>

      <IconButton
        :label="t('shell.toolbar.temporaryConnection')"
        data-toolbar-adhoc-connect
        @click="$emit('temporaryConnection')"
      >
        <PlugZap :size="16" aria-hidden="true" />
      </IconButton>
      <IconButton
        :label="t('shell.toolbar.sftp')"
        :active="props.sftpActive"
        :pressed="props.sftpActive"
        @click="$emit('openSftp')"
      >
        <FolderOpen :size="16" aria-hidden="true" />
      </IconButton>
      <IconButton :label="t('shell.toolbar.newWindow')" @click="$emit('createWindow')">
        <AppWindow :size="16" aria-hidden="true" />
      </IconButton>
    </div>

    <div class="topbar-spacer relative z-1 min-w-3 flex-1" style="--wails-draggable: no-drag"></div>

    <div class="topbar-actions" :aria-label="t('shell.toolbar.global')">
      <UpdateBell />
      <IconButton :label="t('shell.toolbar.lock')" :disabled="!props.appLockEnabled" @click="emit('lockApp')">
        <LockKeyhole :size="16" aria-hidden="true" />
      </IconButton>
      <IconButton
        :label="props.isDark ? t('shell.toolbar.lightTheme') : t('shell.toolbar.darkTheme')"
        @click="$emit('toggleTheme')"
      >
        <MorphingIcon :icon="props.isDark ? Sun : Moon" :size="16" reduced-motion="user" aria-hidden="true" />
      </IconButton>
      <IconButton :label="t('shell.toolbar.settings')" @click="$emit('openSettings')">
        <Settings :size="16" aria-hidden="true" />
      </IconButton>
    </div>
  </header>
</template>
