<script setup lang="ts">
import { QtermButton, QtermInput } from '../common'
import { computed, ref, watch } from 'vue'
import { Label } from '../common/control-primitives'
import { QtermDialog, QtermDialogContent, QtermDialogDescription, QtermDialogHeader } from '../common/dialog'
import { activeTransferConflict, remoteDirectoryRequest, type ConflictAction } from '../../services/transfer-conflicts'
import { useI18n } from '@/locales'
const { t } = useI18n()
const directory = ref('')
const isRzDirectory = computed(() => remoteDirectoryRequest.value?.source === 'rz')
const directoryTitle = computed(() =>
  isRzDirectory.value ? t('shell.transfer.rzDirectory') : t('shell.transfer.uploadDirectory'),
)
const directoryDescription = computed(() =>
  isRzDirectory.value ? t('shell.transfer.rzDirectoryHint') : t('shell.transfer.uploadDirectoryHint'),
)
const directoryLabel = computed(() =>
  isRzDirectory.value ? t('shell.transfer.rzDirectory') : t('shell.transfer.uploadDirectory'),
)
const directoryInputId = computed(() => (isRzDirectory.value ? 'rz-directory' : 'terminal-upload-directory'))
watch(remoteDirectoryRequest, () => {
  directory.value = ''
})
const choices = computed<{ action: ConflictAction; label: string; hint: string }[]>(() => [
  { action: 'overwrite', label: t('shell.transfer.overwrite'), hint: t('shell.transfer.overwriteHint') },
  { action: 'copy', label: t('shell.transfer.copy'), hint: t('shell.transfer.copyHint') },
  { action: 'stop', label: t('shell.transfer.stop'), hint: t('shell.transfer.stopHint') },
  { action: 'cancel', label: t('shell.transfer.cancel'), hint: t('shell.transfer.cancelHint') },
])
</script>
<template>
  <QtermDialog :open="Boolean(activeTransferConflict)">
    <QtermDialogContent
      v-if="activeTransferConflict"
      class="transfer-conflict-dialog"
      size="md"
      @pointer-down-outside.prevent
    >
      <QtermDialogHeader :title="t('shell.transfer.conflictTitle')" :show-close="false" />
      <div class="dialog-body">
        <QtermDialogDescription class="conflict-description">{{
          t('shell.transfer.conflictMessage', { name: activeTransferConflict.name })
        }}</QtermDialogDescription>
        <div class="conflict-choices">
          <QtermButton
            v-for="choice in choices"
            :key="choice.action"
            variant="outline"
            class="conflict-choice"
            @click="activeTransferConflict?.resolve(choice.action)"
          >
            <span>{{ choice.label }}</span
            ><small class="text-xs">{{ choice.hint }}</small>
          </QtermButton>
        </div>
      </div>
    </QtermDialogContent>
  </QtermDialog>
  <QtermDialog :open="Boolean(remoteDirectoryRequest)">
    <QtermDialogContent v-if="remoteDirectoryRequest" class="sm:max-w-[440px]" @pointer-down-outside.prevent>
      <QtermDialogHeader :title="directoryTitle" :show-close="false" />
      <form class="dialog-body" @submit.prevent="remoteDirectoryRequest?.resolve(directory.trim())">
        <QtermDialogDescription>{{ directoryDescription }}</QtermDialogDescription>
        <Label :for="directoryInputId">{{ directoryLabel }}</Label>
        <QtermInput
          :id="directoryInputId"
          v-model="directory"
          :placeholder="t('shell.transfer.directoryPlaceholder')"
          autocomplete="off"
        />
        <div class="flex justify-end gap-2">
          <QtermButton type="button" variant="outline" @click="remoteDirectoryRequest?.resolve(null)">{{
            t('shell.common.cancel')
          }}</QtermButton>
          <QtermButton type="submit" :disabled="!directory.trim().startsWith('/')">{{
            t('shell.transfer.continue')
          }}</QtermButton>
        </div>
      </form>
    </QtermDialogContent>
  </QtermDialog>
</template>
<style scoped>
.conflict-description {
  overflow-wrap: anywhere;
  line-height: 1.6;
}
.conflict-choices {
  display: grid;
  gap: 8px;
}
.conflict-choice {
  height: auto;
  min-height: 58px;
  display: flex;
  align-items: flex-start;
  flex-direction: column;
  gap: 4px;
  padding: 10px 12px;
  white-space: normal;
  text-align: left;
}
.conflict-choice small {
  color: var(--qt-muted);
  font-weight: 400;
}
</style>
