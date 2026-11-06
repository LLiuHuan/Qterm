<script setup lang="ts">
import { FileKey, Fingerprint, KeyRound, LockKeyhole, Plus } from '@lucide/vue'
import { IconButton } from '../common'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '../common/overlay-controls'
import { useI18n } from '@/locales'

const props = withDefaults(defineProps<{ disabled?: boolean }>(), { disabled: false })

const emit = defineEmits<{
  create: [kind: 'password' | 'privateKey' | 'sshAgent', keyMode?: 'generate' | 'upload' | 'manual']
}>()

const { t } = useI18n()
</script>

<template>
  <DropdownMenu>
    <DropdownMenuTrigger as-child>
      <IconButton
        variant="outline"
        :label="t('settings.credentials.action.quickCreate')"
        :disabled="props.disabled"
        data-credential-quick-create
      >
        <Plus :size="16" aria-hidden="true" />
      </IconButton>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="end" class="w-52">
      <DropdownMenuItem :disabled="props.disabled" @select="emit('create', 'password')">
        <LockKeyhole :size="14" aria-hidden="true" />
        {{ t('settings.credentials.action.createPassword') }}
      </DropdownMenuItem>
      <DropdownMenuItem :disabled="props.disabled" @select="emit('create', 'privateKey', 'generate')">
        <KeyRound :size="14" aria-hidden="true" />
        {{ t('settings.credentials.action.createKey') }}
      </DropdownMenuItem>
      <DropdownMenuItem :disabled="props.disabled" @select="emit('create', 'privateKey', 'upload')">
        <FileKey :size="14" aria-hidden="true" />
        {{ t('settings.credentials.action.importPrivateKey') }}
      </DropdownMenuItem>
      <DropdownMenuItem :disabled="props.disabled" @select="emit('create', 'sshAgent')">
        <Fingerprint :size="14" aria-hidden="true" />
        {{ t('settings.credentials.action.createAgent') }}
      </DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>
</template>
