<script setup lang="ts">
import { computed } from 'vue'
import { QtermButton, QtermFormMessage } from '../../components/common'
import {
  QtermDialog,
  QtermDialogContent,
  QtermDialogDescription,
  QtermDialogHeader,
} from '../../components/common/dialog'
import { useI18n } from '@/locales'
import { CONFLICT_CHOICES, conflictShowsDiff, type ConflictChoice, type ConflictKind } from './remoteEditModel'
import { diffLines } from './lineDiff'

const props = withDefaults(
  defineProps<{
    open: boolean
    kind: ConflictKind
    remotePath: string

    remoteContent?: string

    localContent?: string

    busy?: boolean

    repeated?: boolean
  }>(),
  { remoteContent: undefined, localContent: undefined, busy: false, repeated: false },
)

const emit = defineEmits<{ choose: [choice: ConflictChoice] }>()
const { t } = useI18n()

const showsDiff = computed(
  () => conflictShowsDiff(props.kind) && props.remoteContent !== undefined && props.localContent !== undefined,
)

const diff = computed(() => (showsDiff.value ? diffLines(props.remoteContent ?? '', props.localContent ?? '') : null))

const fileName = computed(() => props.remotePath.split('/').filter(Boolean).at(-1) ?? props.remotePath)

function diffLineClass(kind: 'equal' | 'add' | 'remove'): string {
  if (kind === 'add') return 'bg-qt-success/10 text-qt-text'
  if (kind === 'remove') return 'bg-qt-danger/10 text-qt-text'
  return 'text-qt-muted'
}
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open: boolean) => !open && emit('choose', 'cancel')">
    <QtermDialogContent v-if="props.open" class="remote-edit-conflict-dialog" size="2xl" @pointer-down-outside.prevent>
      <QtermDialogHeader :title="t('remote.ui.conflict.title')" :description="fileName" :show-close="false" />
      <div class="dialog-body">
        <QtermDialogDescription class="conflict-description text-xs">
          {{ t('remote.ui.conflict.description') }}
        </QtermDialogDescription>
        <QtermFormMessage v-if="props.repeated" variant="error">
          {{ t('remote.ui.conflict.repeated') }}
        </QtermFormMessage>

        <template v-if="props.kind === 'in-app'">
          <template v-if="showsDiff && diff">
            <div class="flex-cb gap-2">
              <span class="text-xs text-qt-muted">{{ t('remote.ui.conflict.diffTitle') }}</span>
              <span class="text-xs text-qt-muted">
                {{
                  diff.status === 'too-large'
                    ? t('remote.ui.conflict.diffUnavailable')
                    : diff.added === 0 && diff.removed === 0
                      ? t('remote.ui.conflict.diffIdentical')
                      : t('remote.ui.conflict.diffSummary', { added: diff.added, removed: diff.removed })
                }}
              </span>
            </div>

            <div
              v-if="diff.status === 'ok'"
              class="remote-edit-conflict-diff max-h-60 overflow-auto rounded-md border-d py-1 font-mono text-xs"
              data-testid="remote-edit-conflict-diff"
            >
              <div
                v-for="(line, index) in diff.lines"
                :key="index"
                class="flex gap-2 px-2 whitespace-pre"
                :class="diffLineClass(line.kind)"
              >
                <span class="w-10 shrink-0 text-right text-qt-muted">{{ line.leftNumber ?? '' }}</span>
                <span class="w-10 shrink-0 text-right text-qt-muted">{{ line.rightNumber ?? '' }}</span>
                <span class="shrink-0 text-qt-muted">{{
                  line.kind === 'add' ? '+' : line.kind === 'remove' ? '-' : ' '
                }}</span>
                <span>{{ line.text }}</span>
              </div>
            </div>
            <QtermFormMessage v-else variant="hint">{{ t('remote.ui.conflict.diffUnavailable') }}</QtermFormMessage>
          </template>
          <QtermFormMessage v-else variant="hint">{{ t('remote.ui.conflict.diffUnavailable') }}</QtermFormMessage>
        </template>
        <QtermFormMessage v-else variant="hint">{{ t('remote.ui.conflict.externalNotice') }}</QtermFormMessage>

        <div class="conflict-choices">
          <QtermButton
            v-for="choice in CONFLICT_CHOICES"
            :key="choice.choice"
            :variant="choice.danger ? 'destructive' : 'outline'"
            class="conflict-choice"
            :disabled="props.busy"
            :data-conflict-choice="choice.choice"
            @click="emit('choose', choice.choice)"
          >
            <span>{{ t(choice.labelKey) }}</span>
            <small class="text-xs">{{ t(choice.hintKey) }}</small>
          </QtermButton>
        </div>
      </div>
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

.remote-edit-conflict-diff {
  overflow-wrap: normal;
}
</style>
