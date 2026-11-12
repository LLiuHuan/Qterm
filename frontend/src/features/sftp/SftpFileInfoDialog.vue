<script setup lang="ts">
import { computed } from 'vue'
import { Copy } from '@lucide/vue'
import { QtermButton } from '../../components/common'
import { QtermDialog, QtermDialogContent, QtermDialogHeader } from '../../components/common/dialog'
import { useI18n } from '../../locales'

export interface SftpFileInfoEntry {
  name: string
  key?: string
  path?: string
  directory?: string
  type: 'file' | 'directory' | 'symlink' | 'other'
  targetType?: 'file' | 'directory' | 'symlink' | 'other'
  symlink?: boolean
  mode?: string
  uid?: number
  gid?: number
  modTime?: string
  size?: number
  sizeText?: string
}

const props = defineProps<{
  open: boolean
  entry: SftpFileInfoEntry | null

  path?: string
}>()

const emit = defineEmits<{
  'update:open': [open: boolean]
  copyPath: [path: string]
}>()

const { t, locale } = useI18n()

const displayPath = computed(() => props.path || props.entry?.key || props.entry?.path || '')

const isSymlink = computed(() => Boolean(props.entry?.symlink || props.entry?.type === 'symlink'))

const typeLabel = computed(() => {
  const entry = props.entry
  if (!entry) return t('sftp.ui.unknown')
  if (isSymlink.value) {
    const target = entry.targetType
    if (target === 'directory') return t('sftp.ui.info.symlinkToDirectory')
    return t('sftp.ui.info.symlinkToFile')
  }
  if (entry.type === 'directory') return t('sftp.ui.folder')
  if (entry.type === 'file') return t('sftp.ui.file')
  return t('sftp.ui.unknown')
})

const sizeLabel = computed(() => {
  const entry = props.entry
  if (!entry) return t('sftp.ui.unknown')
  if (entry.type === 'directory') return t('sftp.ui.directoryName')
  const text = entry.sizeText || t('sftp.ui.unknown')
  const bytes = typeof entry.size === 'number' ? entry.size : 0

  return `${text}（${new Intl.NumberFormat(locale.value || undefined).format(bytes)} ${t('sftp.ui.bytes')}）`
})

const modifiedLabel = computed(() => {
  const raw = props.entry?.modTime
  if (!raw) return t('sftp.ui.unknown')
  const parsed = new Date(raw)
  if (Number.isNaN(parsed.getTime())) return t('sftp.ui.unknown')
  return parsed.toLocaleString(locale.value || undefined)
})

function numberOrUnknown(value: number | undefined): string {
  return typeof value === 'number' && Number.isFinite(value) ? String(value) : t('sftp.ui.unknown')
}
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => emit('update:open', open)">
    <QtermDialogContent v-if="props.open && props.entry" size="lg">
      <QtermDialogHeader :title="t('sftp.ui.fileInfo')" @close="emit('update:open', false)" />
      <div class="sftp-info-grid" data-sftp-file-info>
        <div>
          <span>{{ t('sftp.ui.info.name') }}</span
          ><span>{{ props.entry.name || t('sftp.ui.unknown') }}</span>
        </div>
        <div>
          <span>{{ t('sftp.ui.info.type') }}</span
          ><span data-sftp-info-type>{{ typeLabel }}</span>
        </div>
        <div>
          <span>{{ t('sftp.ui.info.path') }}</span>
          <span class="sftp-info-path-value">
            <span>{{ displayPath || t('sftp.ui.unknown') }}</span>
            <QtermButton
              variant="ghost"
              size="icon"
              type="button"
              :aria-label="t('sftp.ui.copyPath')"
              :disabled="!displayPath"
              @click="emit('copyPath', displayPath)"
              ><Copy :size="14"
            /></QtermButton>
          </span>
        </div>
        <div>
          <span>{{ t('sftp.ui.info.size') }}</span
          ><span data-sftp-info-size>{{ sizeLabel }}</span>
        </div>
        <div>
          <span>{{ t('sftp.ui.info.modified') }}</span
          ><span data-sftp-info-modified>{{ modifiedLabel }}</span>
        </div>
        <div>
          <span>{{ t('sftp.ui.info.permissions') }}</span
          ><span>{{ props.entry.mode || t('sftp.ui.unknown') }}</span>
        </div>
        <div>
          <span>{{ t('sftp.ui.info.owner') }}</span
          ><span data-sftp-info-uid>{{ numberOrUnknown(props.entry.uid) }}</span>
        </div>
        <div>
          <span>{{ t('sftp.ui.info.group') }}</span
          ><span data-sftp-info-gid>{{ numberOrUnknown(props.entry.gid) }}</span>
        </div>
        <div>
          <span>{{ t('sftp.ui.info.symlink') }}</span
          ><span>{{ isSymlink ? t('sftp.ui.yes') : t('sftp.ui.no') }}</span>
        </div>
      </div>
    </QtermDialogContent>
  </QtermDialog>
</template>
