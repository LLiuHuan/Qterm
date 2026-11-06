<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { Bell } from '@lucide/vue'
import { toast } from 'vue-sonner'
import { IconButton, QtermButton } from '../../components/common'
import { useI18n } from '@/locales'
import { dismissUpdate, installUpdate, shouldNotifyUpdate, useUpdateState } from './updateStore'

const { t } = useI18n()
const state = useUpdateState()

const visible = computed(() => shouldNotifyUpdate())
const version = computed(() => state.status.release?.version ?? '')
const installing = computed(() => state.status.state === 'installing' || state.busy)
const open = ref(false)

watch(version, () => {
  open.value = false
})

function onDocumentPointerDown(event: PointerEvent): void {
  const target = event.target as HTMLElement | null
  if (!target?.closest('[data-update-bell-root]')) open.value = false
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape') open.value = false
}

function bindDismissers(bind: boolean): void {
  if (typeof document === 'undefined') return
  const method = bind ? 'addEventListener' : 'removeEventListener'
  document[method]('pointerdown', onDocumentPointerDown as EventListener, true)
  document[method]('keydown', onKeydown as EventListener, true)
}

watch(open, (value) => bindDismissers(value))
onBeforeUnmount(() => bindDismissers(false))

function toggle(): void {
  open.value = !open.value
}

function later(): void {
  dismissUpdate()
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
      @click="toggle"
    >
      <Bell :size="16" aria-hidden="true" />
      <span
        class="pointer-events-none absolute top-[0.3rem] right-[0.3rem] size-[0.375rem] rounded-full bg-qt-accent"
        data-update-bell-dot
        aria-hidden="true"
      />
    </IconButton>
    <div
      v-if="open"
      class="settings-card absolute top-full right-0 z-50 mt-2 flex w-72 flex-col gap-1 rounded-lg border border-solid border-qt-border p-3 shadow-lg [background:color-mix(in_srgb,var(--qt-panel)_96%,transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
      role="dialog"
      :aria-label="t('about.update.bell.title', { version })"
      data-update-bell-panel
    >
      <strong class="text-sm">{{ t('about.update.bell.title', { version }) }}</strong>
      <p class="m-0 text-xs text-qt-muted">{{ t('about.update.bell.body') }}</p>
      <div class="mt-2 flex justify-end gap-2">
        <QtermButton variant="outline" type="button" data-update-bell-dismiss @click="later">
          {{ t('about.update.bell.dismiss') }}
        </QtermButton>
        <QtermButton type="button" :disabled="installing" data-update-bell-install @click="installNow">
          {{ installing ? t('about.update.installing') : t('about.update.install') }}
        </QtermButton>
      </div>
    </div>
  </div>
</template>
