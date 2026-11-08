<script setup lang="ts">
import {
  HostIcon,
  IconButton,
  QtermButton,
  QtermEmptyState,
  QtermInput,
  QtermSecretInput,
  SegmentedTabs,
} from '../../components/common'
import OtpPanel from '../otp/OtpPanel.vue'
import type { OtpEntry } from '../../services/otp'
import CredentialCreateDialog from '../../components/dialogs/CredentialCreateDialog.vue'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../components/common/form-controls'
import { Label, ScrollArea } from '../../components/common/control-primitives'
import { computed, ref, watch } from 'vue'
import { FileKey, KeyRound, Loader2, LockKeyhole, Pencil, Search, Trash2 } from '@lucide/vue'
import { toast } from 'vue-sonner'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import {
  credentialErrorMessage,
  sortCredentialsByCreatedAtDesc,
  type CredentialGateway,
  type CredentialKind,
  type CredentialRecord,
  type CredentialVaultStatus,
  type LocalPrivateKey,
} from '../../services/credentials'
import type { HostRecord } from '@/services/hosts.ts'
import { useI18n } from '@/locales'

const props = defineProps<{
  otpEntries?: OtpEntry[]
  initialView?: 'credentials' | 'otp'
  gateway: CredentialGateway
  credentials: CredentialRecord[]

  hosts?: HostRecord[]
  status: CredentialVaultStatus | null
  loading: boolean
  error: string
}>()

const emit = defineEmits<{ changed: []; openSecurity: []; 'update:otpEntries': [value: OtpEntry[]] }>()
const { t } = useI18n()

const view = ref<'credentials' | 'otp'>(props.initialView ?? 'credentials')
watch(
  () => props.initialView,
  (value) => {
    view.value = value ?? 'credentials'
  },
)
const items = ref<CredentialRecord[]>([])
const query = ref('')
const kindFilter = ref<'all' | 'password' | 'privateKey' | 'sshAgent'>('all')
const formOpen = ref(false)
const formKind = ref<'password' | 'privateKey' | 'sshAgent'>('password')

const formInitialKeyMode = ref<'generate' | 'upload' | 'manual'>('generate')
const editing = ref<CredentialRecord | null>(null)

const quickAddOpen = ref(false)
const quickAddToken = ref('')
const quickAddName = ref('')
const quickAddPassphrase = ref('')
const quickAddSubmitting = ref(false)
const scanningLocalKeys = ref(false)
const localKeyChoices = ref<LocalPrivateKey[]>([])
const pendingDelete = ref<CredentialRecord | null>(null)
const deleting = ref(false)

watch(
  () => props.credentials,
  (value) => {
    items.value = value.slice()
  },
  { immediate: true },
)

const filtered = computed(() => {
  const needle = query.value.trim().toLowerCase()

  return sortCredentialsByCreatedAtDesc(
    items.value.filter(
      (item) =>
        (kindFilter.value === 'all' || item.kind === kindFilter.value) &&
        (!needle || item.name.toLowerCase().includes(needle)),
    ),
  )
})

const loadErrorDetail = computed(() => (props.error === t('shell.error.credentialsRead') ? '' : props.error))

const hasFilter = computed(() => query.value.trim() !== '' || kindFilter.value !== 'all')

const hostsByCredential = computed(() => {
  const map = new Map<string, HostRecord[]>()
  for (const host of props.hosts ?? []) {
    const credentialId = host.credentialId?.trim()
    if (!credentialId) continue
    const list = map.get(credentialId)
    if (list) list.push(host)
    else map.set(credentialId, [host])
  }
  return map
})

const CREDENTIAL_PAGE_SIZE = 20
const visibleCount = ref(CREDENTIAL_PAGE_SIZE)

const visibleItems = computed(() => filtered.value.slice(0, visibleCount.value))
const hasMoreItems = computed(() => visibleCount.value < filtered.value.length)

function loadMore(): void {
  if (hasMoreItems.value) visibleCount.value += CREDENTIAL_PAGE_SIZE
}

watch([query, kindFilter, () => props.credentials], () => {
  visibleCount.value = CREDENTIAL_PAGE_SIZE
})

function linkedHosts(item: CredentialRecord): HostRecord[] {
  return hostsByCredential.value.get(item.id) ?? []
}

const linkedHostDialog = ref<{ name: string; hosts: HostRecord[] } | null>(null)

function openLinkedHosts(item: CredentialRecord): void {
  const hosts = linkedHosts(item)
  if (hosts.length === 0) return
  linkedHostDialog.value = { name: item.name, hosts }
}

function kindLabel(kind: CredentialKind): string {
  return t('settings.credentials.kind.' + kind)
}

function openCreate(
  kind: 'password' | 'privateKey' | 'sshAgent',
  keyMode: 'generate' | 'upload' | 'manual' = 'generate',
): void {
  editing.value = null
  formKind.value = kind
  formInitialKeyMode.value = keyMode
  formOpen.value = true
}

function openEdit(item: CredentialRecord): void {
  editing.value = item
  formKind.value = item.kind === 'password' ? 'password' : item.kind === 'sshAgent' ? 'sshAgent' : 'privateKey'
  formInitialKeyMode.value = 'generate'
  formOpen.value = true
}

function closeForm(): void {
  formOpen.value = false
  editing.value = null
}

function handleCreated(): void {
  formOpen.value = false
  emit('changed')
}

function handleUpdated(): void {
  formOpen.value = false
  emit('changed')
}

async function scanLocalKeys(): Promise<void> {
  if (!props.gateway.listLocalPrivateKeys) return
  scanningLocalKeys.value = true
  try {
    localKeyChoices.value = await props.gateway.listLocalPrivateKeys()

    if (!localKeyChoices.value.some((key) => key.token === quickAddToken.value)) {
      quickAddToken.value = ''
    }
  } catch (error) {
    toast.error(credentialErrorMessage(error, t('settings.credentials.localKeyScan.failed')))
  } finally {
    scanningLocalKeys.value = false
  }
}

async function choosePrivateKey(): Promise<void> {
  if (!props.gateway.listLocalPrivateKeys) {
    openCreate('privateKey', 'upload')
    return
  }

  quickAddToken.value = ''
  quickAddName.value = ''
  quickAddPassphrase.value = ''
  quickAddOpen.value = true
  await scanLocalKeys()

  if (localKeyChoices.value.length === 0) {
    quickAddOpen.value = false
    openCreate('privateKey', 'upload')
  }
}

function selectQuickAddKey(key: LocalPrivateKey): void {
  quickAddToken.value = key.token
  quickAddName.value = key.name
}

async function submitQuickAdd(): Promise<void> {
  if (quickAddSubmitting.value) return
  if (!quickAddToken.value) {
    toast.error(t('settings.credentials.validation.nameRequired'))
    return
  }
  if (!quickAddName.value.trim()) {
    toast.error(t('settings.credentials.validation.nameRequired'))
    return
  }
  if (!props.gateway.importLocalPrivateKey) return
  quickAddSubmitting.value = true
  try {
    await props.gateway.importLocalPrivateKey({
      token: quickAddToken.value,
      name: quickAddName.value.trim(),
      passphrase: quickAddPassphrase.value || undefined,
    })
    quickAddOpen.value = false
    quickAddPassphrase.value = ''
    toast.success(t('settings.credentials.quickAdd.done'))

    emit('changed')
  } catch (error) {
    toast.error(credentialErrorMessage(error, t('settings.credentials.error.create')))
  } finally {
    quickAddSubmitting.value = false
  }
}

async function confirmDelete(): Promise<void> {
  if (!pendingDelete.value || deleting.value) return
  deleting.value = true
  try {
    await props.gateway.remove(pendingDelete.value.id)
    items.value = items.value.filter((item) => item.id !== pendingDelete.value?.id)
    pendingDelete.value = null
    emit('changed')
    toast.success(t('settings.credentials.toast.deleted'))
  } catch (error) {
    toast.error(credentialErrorMessage(error, t('settings.credentials.error.delete')))
  } finally {
    deleting.value = false
  }
}
</script>

<template>
  <section class="credentials-settings flex h-full min-h-0 flex-col gap-4">
    <SegmentedTabs
      :model-value="view"
      :label="t('shell.otp.libraryTabs')"
      :options="[
        { value: 'credentials', label: t('shell.otp.loginCredentials') },
        { value: 'otp', label: t('shell.otp.verificationCodes') },
      ]"
      @update:model-value="view = $event as 'credentials' | 'otp'"
    />
    <OtpPanel
      v-if="view === 'otp'"
      class="min-h-0 flex-1"
      :entries="props.otpEntries ?? []"
      :hosts="props.hosts ?? []"
      mode="manage"
      @update:entries="emit('update:otpEntries', $event)"
    />
    <div
      v-else-if="props.status?.state !== 'unlocked'"
      class="settings-card credential-locked-card flex items-center gap-[12px] p-[16px] border border-solid border-qt-border rounded-[8px] [background:color-mix(in_srgb,_var(--qt-panel)_32%,_transparent)] [-webkit-backdrop-filter:blur(16px)_saturate(135%)] [backdrop-filter:blur(16px)_saturate(135%)]"
    >
      <LockKeyhole :size="20" aria-hidden="true" />
      <div>
        <strong class="text-sm">{{ t('settings.credentials.vaultLocked.title') }}</strong>
        <p>{{ t('settings.credentials.vaultLocked.description') }}</p>
      </div>
      <QtermButton variant="outline" type="button" @click="emit('openSecurity')">{{
        t('settings.credentials.vaultLocked.openSecurity')
      }}</QtermButton>
    </div>

    <template v-else>
      <div class="credential-toolbar flex-none grid gap-[14px] pb-[14px]">
        <div class="credential-create-actions flex-wrap gap-[8px]">
          <QtermButton variant="outline" type="button" @click="openCreate('password')">
            <LockKeyhole :size="15" />{{ t('settings.credentials.action.createPassword') }}
          </QtermButton>
          <QtermButton variant="outline" type="button" @click="openCreate('privateKey')">
            <KeyRound :size="15" />{{ t('settings.credentials.action.createKey') }}
          </QtermButton>
          <QtermButton variant="outline" type="button" @click="openCreate('sshAgent')">
            <LockKeyhole :size="15" />{{ t('settings.credentials.action.createAgent') }}
          </QtermButton>
          <QtermButton variant="outline" type="button" @click="choosePrivateKey">
            <FileKey :size="15" />{{ t('settings.credentials.action.importPrivateKey') }}
          </QtermButton>
        </div>
        <div class="credential-filters gap-[10px]">
          <Select v-model="kindFilter">
            <SelectTrigger class="credential-kind-trigger" :aria-label="t('settings.credentials.filter.aria')"
              ><SelectValue
            /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{{ t('settings.credentials.filter.all', { count: items.length }) }}</SelectItem>
              <SelectItem value="password">{{ kindLabel('password') }}</SelectItem>
              <SelectItem value="privateKey">{{ kindLabel('privateKey') }}</SelectItem>
              <SelectItem value="sshAgent">{{ kindLabel('sshAgent') }}</SelectItem>
            </SelectContent>
          </Select>
          <label class="credential-search relative flex-1">
            <Search :size="15" aria-hidden="true" />
            <QtermInput
              v-model="query"
              :placeholder="t('settings.credentials.search.placeholder')"
              :aria-label="t('settings.credentials.search.aria')"
            />
          </label>
        </div>
      </div>

      <QtermEmptyState
        v-if="props.loading"
        variant="outlined"
        role="status"
        busy
        :icon="Loader2"
        icon-class="spin"
        :description="t('settings.credentials.loading')"
      />

      <QtermEmptyState
        v-else-if="props.error"
        variant="outlined"
        role="alert"
        :title="t('shell.error.credentialsRead')"
        :description="loadErrorDetail"
      />

      <ScrollArea v-else-if="filtered.length" type="hover" :scroll-hide-delay="600" class="min-h-0 flex-1">
        <div class="credential-list grid gap-2 pr-2">
          <article
            v-for="item in visibleItems"
            :key="item.id"
            class="credential-row settings-library-row rounded-md border-d"
          >
            <span
              class="credential-kind-icon w-[32px] h-[32px] flex-none grid place-items-center rounded-[8px] text-[#d97706] [background:color-mix(in_srgb,_#f59e0b_12%,_transparent)]"
              :class="`is-${item.kind}`"
            >
              <LockKeyhole v-if="item.kind === 'password'" :size="18" />
              <KeyRound v-else :size="18" />
            </span>
            <div class="credential-row-copy min-w-0 flex-1 flex items-center gap-[8px]">
              <strong class="text-sm">{{ item.name }}</strong>

              <button
                v-if="linkedHosts(item).length"
                type="button"
                class="inline-flex shrink-0 items-center rounded-full border-d bg-qt-subtle px-2 py-0 text-xs leading-4 text-qt-muted transition-colors hover:text-qt-text"
                @click="openLinkedHosts(item)"
              >
                {{ t('settings.credentials.hostCount', { count: linkedHosts(item).length }) }}
              </button>
            </div>
            <div class="credential-row-actions settings-library-row-actions">
              <QtermButton
                variant="ghost"
                size="icon-sm"
                type="button"
                :aria-label="t('settings.credentials.action.edit')"
                @click="openEdit(item)"
              >
                <Pencil :size="15" />
              </QtermButton>
              <IconButton
                danger
                size="icon-sm"
                class="credential-delete settings-delete-action"
                :label="t('settings.credentials.action.delete')"
                @click="pendingDelete = item"
                ><Trash2 :size="15"
              /></IconButton>
            </div>
          </article>
        </div>
        <QtermLoadMore
          :visible="visibleItems.length"
          :total="filtered.length"
          :load-more-label="t('settings.credentials.pagination.loadMore')"
          @load-more="loadMore"
        />
      </ScrollArea>

      <QtermEmptyState
        v-else-if="hasFilter"
        variant="outlined"
        :icon="KeyRound"
        :title="t('settings.credentials.empty.matching')"
      />

      <QtermEmptyState
        v-else
        variant="outlined"
        :icon="KeyRound"
        :title="t('settings.credentials.empty.none')"
        :description="t('settings.credentials.empty.description')"
      />
    </template>
  </section>

  <QtermDialog :open="linkedHostDialog !== null" @update:open="(open) => !open && (linkedHostDialog = null)">
    <QtermDialogContent class="w-[calc(100vw-32px)] max-w-105!">
      <QtermDialogHeader
        :title="t('settings.credentials.hostList.title')"
        :description="linkedHostDialog?.name"
        @close="linkedHostDialog = null"
      />
      <div class="dialog-body gap-0.5! py-2! px-3!">
        <div
          v-for="host in linkedHostDialog?.hosts ?? []"
          :key="host.id"
          class="flex min-w-0 items-center gap-2 rounded-md px-2 py-1.5 transition-colors hover:bg-qt-hover"
        >
          <HostIcon :icon="host.icon" :protocol="host.protocol" />
          <span class="flex min-w-0 flex-col">
            <span class="truncate text-sm">{{ host.name }}</span>
            <span class="truncate text-xs text-qt-muted">{{ host.address }}:{{ host.port }}</span>
          </span>
        </div>
      </div>
    </QtermDialogContent>
  </QtermDialog>

  <CredentialCreateDialog
    :open="formOpen"
    :kind="formKind"
    :gateway="props.gateway"
    :status="props.status"
    :editing="editing"
    :initial-key-mode="formInitialKeyMode"
    @created="handleCreated"
    @updated="handleUpdated"
    @close="closeForm"
  />

  <QtermDialog :open="quickAddOpen" @update:open="(open) => !open && (quickAddOpen = false)">
    <QtermDialogContent class="w-[calc(100vw-32px)] max-w-140!">
      <QtermDialogHeader :title="t('settings.credentials.quickAdd.title')" @close="quickAddOpen = false" />
      <div class="dialog-body">
        <div class="flex flex-col gap-1 p-1">
          <button
            v-for="key in localKeyChoices"
            :key="key.token"
            type="button"
            class="flex min-w-0 items-center gap-2 rounded-md border px-2 py-1.5 text-left transition-colors"
            :class="
              quickAddToken === key.token
                ? 'border-primary bg-(--primary)/10'
                : 'border-qt-border hover:bg-qt-background-hover'
            "
            @click="selectQuickAddKey(key)"
          >
            <FileKey :size="18" class="shrink-0 text-qt-muted" aria-hidden="true" />
            <span class="flex min-w-0 flex-1 flex-col">
              <span class="truncate text-sm">{{ key.name }}</span>
              <span class="truncate text-xs text-qt-muted">{{ key.displayPath }}</span>
            </span>
            <span class="shrink-0 text-xs text-qt-muted">{{ key.algorithm.toUpperCase() }}</span>
          </button>
          <p v-if="!localKeyChoices.length" class="panel-empty-note text-qt-muted text-xs">
            {{ t('settings.credentials.localKeyScan.empty') }}
          </p>
        </div>
        <div class="grid gap-4">
          <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
            <Label for="quick-add-key-name"
              >{{ t('settings.credentials.field.name') }} <span class="required-mark text-qt-danger">*</span></Label
            >
            <QtermInput
              id="quick-add-key-name"
              v-model="quickAddName"
              autofocus
              :placeholder="t('settings.credentials.field.namePlaceholder')"
            />
          </div>
        </div>
        <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
          <Label for="quick-add-key-passphrase"
            >{{ t('settings.credentials.field.passphrase') }}
            <span class="optional-label text-qt-faint font-normal">{{
              t('settings.credentials.field.optional')
            }}</span></Label
          >

          <QtermSecretInput
            id="quick-add-key-passphrase"
            autocomplete="new-password"
            :key="quickAddOpen ? 'open' : 'closed'"
            v-model="quickAddPassphrase"
            :placeholder="t('settings.credentials.field.passphrasePlaceholder')"
            :show-label="t('settings.credentials.passphrase.show')"
            :hide-label="t('settings.credentials.passphrase.hide')"
          />
        </div>
      </div>
      <QtermDialogFooter>
        <div></div>
        <div class="dialog-footer-actions flex items-center gap-[8px]">
          <QtermButton variant="outline" type="button" @click="quickAddOpen = false">{{
            t('settings.credentials.action.cancel')
          }}</QtermButton>
          <QtermButton type="button" :disabled="quickAddSubmitting || !quickAddToken" @click="submitQuickAdd">{{
            t('settings.credentials.quickAdd.submit')
          }}</QtermButton>
        </div>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>

  <QtermDialog :open="!!pendingDelete" @update:open="(open) => !open && !deleting && (pendingDelete = null)">
    <QtermDialogContent>
      <QtermDialogHeader
        :title="t('settings.credentials.dialog.deleteTitle')"
        :description="t('settings.credentials.dialog.deleteDescription')"
        :show-close="false"
      />
      <div class="dialog-body">
        <p>{{ t('settings.credentials.dialog.deleteMessage', { name: pendingDelete?.name ?? '' }) }}</p>
      </div>
      <QtermDialogFooter>
        <div></div>
        <div class="dialog-footer-actions flex items-center gap-[8px]">
          <QtermButton variant="outline" :disabled="deleting" @click="pendingDelete = null">{{
            t('settings.credentials.action.cancel')
          }}</QtermButton
          ><QtermButton variant="destructive" :disabled="deleting" @click="confirmDelete">{{
            t(deleting ? 'settings.credentials.action.deleteProgress' : 'settings.credentials.action.delete')
          }}</QtermButton>
        </div>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>
