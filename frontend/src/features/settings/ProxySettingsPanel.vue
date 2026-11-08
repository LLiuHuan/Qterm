<script setup lang="ts">
import { computed, ref } from 'vue'
import { Loader2, Network, Pencil, Plus, Trash2 } from '@lucide/vue'
import { toast } from 'vue-sonner'
import { IconButton, QtermButton, QtermEmptyState } from '../../components/common'
import { ScrollArea } from '../../components/common/control-primitives'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import ProxyEditorDialog from '../../components/dialogs/ProxyEditorDialog.vue'
import { useI18n } from '@/locales'
import type { HostRecord } from '@/services/hosts.ts'
import { proxyErrorMessage, type ProxyGateway, type ProxyRecord } from '../../services/proxies'

const props = withDefaults(
  defineProps<{
    gateway: ProxyGateway
    proxies: ProxyRecord[]

    hosts?: HostRecord[]
    loading?: boolean
    error?: string
  }>(),
  { hosts: () => [], loading: false, error: '' },
)

const emit = defineEmits<{ changed: [] }>()
const { t } = useI18n()

const sortedProxies = computed(() => [...props.proxies].sort((left, right) => left.name.localeCompare(right.name)))

const loadErrorDetail = computed(() => (props.error === t('settings.proxies.error.load') ? '' : props.error))

const formOpen = ref(false)
const editingProxy = ref<ProxyRecord | null>(null)

function openCreate(): void {
  editingProxy.value = null
  formOpen.value = true
}

function openEdit(item: ProxyRecord): void {
  editingProxy.value = item
  formOpen.value = true
}

function closeForm(): void {
  formOpen.value = false
  editingProxy.value = null
}

function handleCreated(): void {
  toast.success(t('settings.proxies.toast.created'))
  emit('changed')
}

function handleUpdated(): void {
  toast.success(t('settings.proxies.toast.updated'))
  emit('changed')
}

const pendingDelete = ref<ProxyRecord | null>(null)
const deleting = ref(false)

const referencingHosts = computed(() =>
  pendingDelete.value ? props.hosts.filter((host) => (host.proxyId ?? '') === pendingDelete.value?.id) : [],
)

function closeDeleteDialog(): void {
  if (deleting.value) return
  pendingDelete.value = null
}

async function confirmDelete(): Promise<void> {
  const target = pendingDelete.value
  if (!target || deleting.value) return
  deleting.value = true
  try {
    const count = await props.gateway.remove(target.id)
    pendingDelete.value = null
    toast.success(t('settings.proxies.toast.deleted', { count }))
    emit('changed')
  } catch (error) {
    toast.error(proxyErrorMessage(error, t('settings.proxies.error.delete')))
  } finally {
    deleting.value = false
  }
}
</script>

<template>
  <section class="flex h-full min-h-0 flex-col gap-4">
    <div class="flex items-start justify-between gap-3">
      <p class="panel-empty-note text-qt-muted m-0 text-xs">{{ t('settings.proxies.description') }}</p>
      <QtermButton class="shrink-0" variant="outline" type="button" data-proxy-create @click="openCreate">
        <Plus :size="15" />{{ t('settings.proxies.action.create') }}
      </QtermButton>
    </div>

    <QtermEmptyState
      v-if="props.loading"
      variant="outlined"
      role="status"
      busy
      :icon="Loader2"
      icon-class="spin"
      :description="t('settings.proxies.loading')"
    />

    <QtermEmptyState
      v-else-if="props.error"
      variant="outlined"
      role="alert"
      :title="t('settings.proxies.error.load')"
      :description="loadErrorDetail"
    />

    <ScrollArea v-else-if="sortedProxies.length" type="hover" :scroll-hide-delay="600" class="min-h-0 flex-1">
      <div class="flex flex-col gap-2 pr-2">
        <article
          v-for="proxy in sortedProxies"
          :key="proxy.id"
          class="settings-library-row flex min-w-0 items-center gap-3 rounded-md border-d px-3 py-2"
          data-proxy-row
        >
          <Network :size="18" class="shrink-0 text-qt-muted" aria-hidden="true" />
          <div class="flex min-w-0 flex-1 flex-col gap-0.5">
            <div class="flex min-w-0 items-center gap-2">
              <strong class="truncate text-sm">{{ proxy.name }}</strong>
              <span class="shrink-0 rounded-full border-d bg-qt-subtle px-2 text-xs leading-4 text-qt-muted">{{
                proxy.mode.toUpperCase()
              }}</span>
            </div>
            <span class="truncate text-xs text-qt-muted">
              {{ proxy.host }}:{{ proxy.port }}
              <template v-if="proxy.username">
                · {{ t('settings.proxies.row.username', { name: proxy.username }) }}</template
              >
              <template v-if="proxy.passwordConfigured"> · {{ t('settings.proxies.row.passwordConfigured') }}</template>
            </span>
          </div>
          <div class="settings-library-row-actions flex shrink-0 items-center gap-1">
            <QtermButton
              variant="ghost"
              size="icon-sm"
              type="button"
              data-proxy-edit
              :aria-label="t('settings.proxies.action.edit')"
              @click="openEdit(proxy)"
            >
              <Pencil :size="15" />
            </QtermButton>
            <IconButton
              danger
              size="icon-sm"
              class="settings-delete-action"
              data-proxy-delete
              :label="t('settings.proxies.action.delete')"
              @click="pendingDelete = proxy"
            >
              <Trash2 :size="15" />
            </IconButton>
          </div>
        </article>
      </div>
    </ScrollArea>

    <QtermEmptyState
      v-else
      variant="outlined"
      :icon="Network"
      :title="t('settings.proxies.empty.title')"
      :description="t('settings.proxies.empty.description')"
    />
  </section>

  <ProxyEditorDialog
    :open="formOpen"
    :gateway="props.gateway"
    :proxy="editingProxy"
    @close="closeForm"
    @created="handleCreated"
    @updated="handleUpdated"
  />

  <QtermDialog :open="pendingDelete !== null" @update:open="(open) => !open && closeDeleteDialog()">
    <QtermDialogContent v-if="pendingDelete" data-proxy-delete-dialog>
      <QtermDialogHeader
        :title="t('settings.proxies.dialog.deleteTitle')"
        :description="t('settings.proxies.dialog.deleteDescription')"
        :show-close="false"
      />
      <div class="dialog-body">
        <p>{{ t('settings.proxies.dialog.deleteConfirm', { name: pendingDelete.name }) }}</p>

        <p v-if="referencingHosts.length" class="text-qt-danger" data-proxy-delete-references>
          {{ t('settings.proxies.dialog.deleteReferenced', { count: referencingHosts.length }) }}
        </p>
        <p v-else class="text-qt-muted" data-proxy-delete-references>
          {{ t('settings.proxies.dialog.deleteUnreferenced') }}
        </p>
      </div>
      <QtermDialogFooter>
        <div></div>
        <div class="dialog-footer-actions flex items-center gap-[8px]">
          <QtermButton variant="outline" :disabled="deleting" @click="closeDeleteDialog">{{
            t('settings.proxies.action.cancel')
          }}</QtermButton>
          <QtermButton variant="destructive" data-proxy-delete-confirm :disabled="deleting" @click="confirmDelete">{{
            t(deleting ? 'settings.proxies.action.deleteProgress' : 'settings.proxies.action.delete')
          }}</QtermButton>
        </div>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>
