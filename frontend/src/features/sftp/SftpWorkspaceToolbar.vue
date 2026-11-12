<script setup lang="ts">
import { Eye, EyeOff, FolderPlus, FolderUp, Trash2, Undo2, Upload } from '@lucide/vue'
import IconButton from '../../components/common/button/IconButton.vue'
import RefreshIconButton from '../../components/common/button/RefreshIconButton.vue'
import { useI18n } from '@/locales'

const props = withDefaults(
  defineProps<{
    kind: 'local' | 'remote'
    available: boolean
    canGoUp: boolean
    loading: boolean
    showHidden: boolean
    filePickerBusy?: boolean
    selectedCount?: number
    entryBusy?: boolean
  }>(),
  {
    filePickerBusy: false,
    selectedCount: 0,
    entryBusy: false,
  },
)
const { t } = useI18n()

const emit = defineEmits<{
  goUp: []
  uploadFile: []
  uploadFolder: []
  refresh: []
  toggleHidden: []
  createFolder: []
  delete: []
}>()

const isRemote = () => props.kind === 'remote'
const canWriteRemote = () => isRemote() && props.available
</script>

<template>
  <div class="sftp-workspace-toolbar" role="toolbar" :aria-label="t('sftp.ui.sftpFunction')">
    <IconButton
      :label="t('sftp.ui.parentDirectory')"
      size="icon-sm"
      :disabled="!props.available || !props.canGoUp"
      @click="emit('goUp')"
    >
      <Undo2 :size="15" aria-hidden="true" />
    </IconButton>
    <RefreshIconButton
      :label="t('sftp.ui.refreshDirectory')"
      :spinning="props.loading"
      :disabled="!props.available || props.loading"
      icon-class="size-[15px]"
      @click="emit('refresh')"
    />
    <IconButton
      :label="t(props.showHidden ? 'sftp.ui.hideHidden' : 'sftp.ui.showHidden')"
      size="icon-sm"
      :active="props.showHidden"
      @click="emit('toggleHidden')"
    >
      <Eye v-if="props.showHidden" :size="15" aria-hidden="true" />
      <EyeOff v-else :size="15" aria-hidden="true" />
    </IconButton>
    <IconButton
      :label="t('sftp.ui.uploadFile')"
      size="icon-sm"
      :disabled="!canWriteRemote() || props.filePickerBusy"
      :pressed="props.filePickerBusy"
      :aria-busy="props.filePickerBusy"
      @click="emit('uploadFile')"
      ><Upload :size="15" aria-hidden="true"
    /></IconButton>
    <IconButton
      :label="t('sftp.ui.uploadFolder')"
      size="icon-sm"
      :disabled="!canWriteRemote() || props.filePickerBusy"
      @click="emit('uploadFolder')"
      ><FolderUp :size="15" aria-hidden="true"
    /></IconButton>
    <IconButton
      :label="t('sftp.ui.createFolder')"
      size="icon-sm"
      :disabled="!canWriteRemote() || props.entryBusy"
      @click="emit('createFolder')"
    >
      <FolderPlus :size="15" aria-hidden="true" />
    </IconButton>
    <IconButton
      :label="t('sftp.ui.deleteSelected')"
      size="icon-sm"
      :disabled="!canWriteRemote() || props.selectedCount === 0 || props.entryBusy"
      @click="emit('delete')"
      danger
    >
      <Trash2 :size="15" aria-hidden="true" />
    </IconButton>
    <span v-if="props.kind === 'local'" class="sftp-workspace-toolbar-hint text-xs">{{
      t('sftp.ui.localBrowseOnly')
    }}</span>
  </div>
</template>

<style scoped>
.sftp-workspace-toolbar {
  display: flex;
  min-width: 0;
  flex: 0 0 auto;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
}

.sftp-workspace-toolbar-hint {
  margin-left: 4px;
  color: var(--qt-muted);
  white-space: nowrap;
}
</style>
