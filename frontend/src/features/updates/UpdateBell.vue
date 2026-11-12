<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Bell } from '@lucide/vue'
import { toast } from 'vue-sonner'
import { IconButton, QtermButton } from '../../components/common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import AiMessageText from '../ai/AiMessageText.vue'
import { useI18n } from '@/locales'
import { installUpdate, shouldNotifyUpdate, useUpdateState } from './updateStore'

const { t } = useI18n()
const state = useUpdateState()

const visible = computed(() => shouldNotifyUpdate())
const version = computed(() => state.status.release?.version ?? '')

const notes = computed(() => (state.status.release?.notes ?? '').trim())
const installing = computed(() => state.status.state === 'installing' || state.busy)
const open = ref(false)

watch(version, () => {
  open.value = false
})

function dismiss(): void {
  open.value = false
}

async function installNow(): Promise<void> {
  await installUpdate()
  if (state.errorKey) return
  open.value = false
  toast.info(t('about.update.restarting'))

  window.setTimeout(() => {
    void import('@wailsio/runtime').then(({ Application }) => Application.Quit())
  }, 400)
}
</script>

<template>
  <div v-if="visible" class="relative flex-none" data-update-bell-root>
    <IconButton
      :label="t('about.update.bell.label', { version })"
      :aria-expanded="open"
      data-update-bell
      @click="open = true"
    >
      <Bell :size="16" aria-hidden="true" />
      <span
        class="pointer-events-none absolute top-[0.3rem] right-[0.3rem] size-[0.375rem] rounded-full bg-qt-accent"
        data-update-bell-dot
        aria-hidden="true"
      />
    </IconButton>

    <QtermDialog :open="open" @update:open="(value) => !value && dismiss()">
      <QtermDialogContent
        v-if="open"
        size="lg"
        close-on-escape
        data-update-bell-panel
        data-update-dialog
        :aria-label="t('about.update.bell.title', { version })"
      >
        <QtermDialogHeader :title="t('about.update.bell.title', { version })" @close="dismiss" />

        <div class="mt-3 flex min-h-0 flex-col gap-1.5" data-update-notes-block>
          <span class="text-xs text-qt-muted">{{ t('about.update.releaseNotes') }}</span>
          <div class="max-h-[46vh] overflow-auto rounded-md border-d p-3 text-xs text-qt-muted" data-update-notes>
            <AiMessageText v-if="notes" :text="notes" />
            <p v-else class="m-0">{{ t('about.update.bell.emptyNotes') }}</p>
          </div>
        </div>

        <QtermDialogFooter>
          <QtermButton variant="outline" type="button" data-update-bell-dismiss @click="dismiss">
            {{ t('about.update.bell.dismiss') }}
          </QtermButton>
          <QtermButton type="button" :disabled="installing" data-update-bell-install @click="installNow">
            {{ installing ? t('about.update.installing') : t('about.update.install') }}
          </QtermButton>
        </QtermDialogFooter>
      </QtermDialogContent>
    </QtermDialog>
  </div>
</template>
