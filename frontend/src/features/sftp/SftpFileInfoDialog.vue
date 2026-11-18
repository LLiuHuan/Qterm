<script setup lang="ts">
import { computed } from 'vue'
import { Copy } from '@lucide/vue'
import { QtermButton, QtermInfoList, type QtermInfoItem } from '../../components/common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { ScrollArea } from '../../components/common/control-primitives'
import { useI18n } from '../../locales'
import { emptyPrincipalIndex, principalLabel, type RemotePrincipalIndex } from './remotePrincipals'

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

const props = withDefaults(
  defineProps<{
    open: boolean
    entry: SftpFileInfoEntry | null

    path?: string

    principals?: RemotePrincipalIndex
  }>(),
  { path: '', principals: () => emptyPrincipalIndex },
)

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

const basicItems = computed<QtermInfoItem[]>(() => {
  if (!props.entry) return []
  const items: QtermInfoItem[] = [
    { label: t('sftp.ui.info.name'), value: props.entry.name || t('sftp.ui.unknown') },
    { label: t('sftp.ui.info.type'), value: typeLabel.value },
    { label: t('sftp.ui.info.path'), value: displayPath.value, mono: true },
    { label: t('sftp.ui.info.size'), value: sizeLabel.value },
    { label: t('sftp.ui.info.modified'), value: modifiedLabel.value },
  ]
  if (isSymlink.value) items.push({ label: t('sftp.ui.info.symlink'), value: t('sftp.ui.yes') })
  return items
})

const ownershipItems = computed<QtermInfoItem[]>(() => {
  if (!props.entry) return []
  return [
    { label: t('sftp.ui.info.permissions'), value: props.entry.mode || t('sftp.ui.unknown'), mono: true },
    {
      label: t('sftp.ui.info.owner'),
      value: principalLabel(props.principals.users, props.entry.uid, t('sftp.ui.unknown')),
    },
    {
      label: t('sftp.ui.info.group'),
      value: principalLabel(props.principals.groups, props.entry.gid, t('sftp.ui.unknown')),
    },
  ]
})
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => emit('update:open', open)">
    <QtermDialogContent
      v-if="props.open && props.entry"
      size="lg"
      class="sftp-info-dialog"
      @pointer-down-outside="(event) => event.preventDefault()"
    >
      <QtermDialogHeader
        :title="props.entry.name || t('sftp.ui.unknown')"
        :description="displayPath"
        :badge="typeLabel"
        @close="emit('update:open', false)"
      />

      <div class="min-h-0 px-4">
        <ScrollArea class="max-h-[min(62vh,620px)] w-full min-w-0" type="scroll" :scroll-hide-delay="600">
          <div class="flex flex-col gap-4 py-3" data-sftp-file-info>
            <section class="grid gap-2" data-sftp-info-basic>
              <h3 class="m-0 text-sm font-medium text-qt-text">{{ t('sftp.ui.info.sectionBasic') }}</h3>
              <QtermInfoList :items="basicItems" />
            </section>

            <section class="grid gap-2" data-sftp-info-ownership>
              <h3 class="m-0 text-sm font-medium text-qt-text">{{ t('sftp.ui.info.sectionOwnership') }}</h3>
              <QtermInfoList :items="ownershipItems" />
            </section>
          </div>
        </ScrollArea>
      </div>

      <QtermDialogFooter>
        <QtermButton
          class="ml-auto"
          variant="outline"
          size="sm"
          type="button"
          :disabled="!displayPath"
          @click="emit('copyPath', displayPath)"
        >
          <Copy :size="14" aria-hidden="true" />
          {{ t('sftp.ui.copyPath') }}
        </QtermButton>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>
