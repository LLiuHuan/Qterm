<script setup lang="ts">
import { computed } from 'vue'
import { ShieldAlert } from '@lucide/vue'
import { QtermButton } from '../../components/common'
import {
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemFooter,
  ItemGroup,
  ItemHeader,
  ItemMedia,
  ItemTitle,
} from '../../components/common/ai'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { useI18n } from '../../locales'

const props = defineProps<{
  open: boolean
  command: string
  sessionTitle: string

  available: boolean

  destructiveReason?: string
}>()

const emit = defineEmits<{ close: []; confirm: [] }>()

const { t } = useI18n()
const lines = computed(() => props.command.split('\n').length)
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => !open && emit('close')">
    <QtermDialogContent v-if="props.open" data-ai-command-confirm class="max-w-[520px]">
      <QtermDialogHeader
        :title="t('shell.aiChat.confirmTitle')"
        :description="t('shell.aiChat.confirmDescription')"
        :show-close="false"
      />
      <div class="dialog-body">
        <ItemGroup>
          <Item variant="outline" class="flex-col items-stretch">
            <div class="flex items-center gap-2">
              <ItemMedia variant="icon">
                <ShieldAlert :size="15" class="text-qt-warning" aria-hidden="true" />
              </ItemMedia>
              <ItemContent>
                <ItemHeader>
                  <ItemTitle>{{ t('shell.aiChat.confirmItemTitle') }}</ItemTitle>
                </ItemHeader>
                <ItemDescription data-ai-command-target>
                  {{ t('shell.aiChat.targetSession', { name: props.sessionTitle }) }}
                </ItemDescription>
              </ItemContent>
            </div>
            <pre
              class="m-0 max-h-[220px] overflow-auto rounded-md border-d bg-qt-subtle px-3 py-2 font-mono text-xs whitespace-pre-wrap break-words"
              data-ai-command-text
              >{{ props.command }}</pre>
            <p v-if="props.destructiveReason" class="m-0 text-xs text-qt-danger" data-ai-destructive-warning>
              {{ t('shell.aiChat.destructiveWarning', { reason: props.destructiveReason }) }}
            </p>
            <p v-if="lines > 1" class="m-0 text-xs text-qt-warning">{{ t('shell.aiChat.multiLineHint') }}</p>
            <ItemFooter>
              <ItemActions>
                <QtermButton variant="outline" @click="emit('close')">{{ t('shell.aiChat.cancel') }}</QtermButton>
                <QtermButton data-ai-command-run :disabled="!props.available" @click="emit('confirm')">
                  {{ t('shell.aiChat.runCommand') }}
                </QtermButton>
              </ItemActions>
            </ItemFooter>
          </Item>
        </ItemGroup>
      </div>
      <QtermDialogFooter>
        <span class="text-[11px] text-qt-muted">{{ t('shell.aiChat.confirmFootnote') }}</span>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>
