<script setup lang="ts">
import { computed } from 'vue'
import { SftpFileIconResolver, type SftpIconEntryType } from './fileIcon'
import { sftpIconSources, type SftpIconName } from './fileIconAssets'

const props = withDefaults(defineProps<{ name: string; directory?: boolean; symlink?: boolean; size?: number }>(), {
  directory: false,
  symlink: false,
  size: 17,
})
const entryType = computed<SftpIconEntryType>(() =>
  props.directory ? 'directory' : props.symlink ? 'symlink' : 'file',
)
const iconName = computed(() => SftpFileIconResolver.resolve(props.name, entryType.value))
const fallbackIcon = computed<SftpIconName>(() => (entryType.value === 'directory' ? 'folder' : 'document'))

const resolvedIcon = computed(() => sftpIconSources[iconName.value] ?? sftpIconSources[fallbackIcon.value])
const lightSource = computed(() => resolvedIcon.value?.light ?? '')
const darkSource = computed(() => resolvedIcon.value?.dark ?? lightSource.value)
</script>

<template>
  <span
    class="sftp-file-icon"
    :style="{ width: `${size}px`, height: `${size}px` }"
    aria-hidden="true"
    :data-icon="`material:${iconName}`"
  >
    <img class="dark:hidden" :src="lightSource" :alt="iconName" />

    <img class="hidden dark:block" :src="darkSource" :alt="iconName" />
  </span>
</template>

<style scoped>
.sftp-file-icon {
  display: inline-flex;
  flex: 0 0 auto;
}

.sftp-file-icon img {
  width: 100%;
  height: 100%;

  object-fit: contain;
}
</style>
