<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { KeyRound } from '@lucide/vue'
import { QtermButton, QtermFormMessage, QtermSecretInput, QtermSwitch } from '../common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../common/dialog'
import { useI18n } from '@/locales'

const props = withDefaults(
  defineProps<{
    open: boolean

    title: string

    description?: string

    hostLabel?: string

    username?: string

    canRemember?: boolean

    submitting?: boolean

    error?: string
  }>(),
  { description: '', hostLabel: '', username: '', canRemember: false, submitting: false, error: '' },
)
const emit = defineEmits<{ submit: [password: string, remember: boolean]; cancel: [] }>()
const { t } = useI18n()

const password = ref('')
const remember = ref(false)

const hasSummary = computed(() => props.hostLabel !== '' || props.username !== '')

watch(
  () => props.open,
  (open) => {
    if (!open) {
      password.value = ''
      remember.value = false
    }
  },
)

function submit(): void {
  if (props.submitting || password.value === '') {
    return
  }
  emit('submit', password.value, remember.value)
}

function cancel(): void {
  if (props.submitting) {
    return
  }
  emit('cancel')
}
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => !open && cancel()">
    <QtermDialogContent v-if="props.open" size="sm" :aria-busy="props.submitting">
      <QtermDialogHeader
        :title="props.title"
        :description="props.description || t('shell.authDialog.description')"
        :close-disabled="props.submitting"
        @close="cancel"
      >
        <template #icon>
          <span
            class="inline-flex size-9 shrink-0 items-center justify-center rounded-md border border-qt-border bg-qt-subtle text-qt-muted"
            aria-hidden="true"
          >
            <KeyRound :size="18" stroke-width="1.8" />
          </span>
        </template>
      </QtermDialogHeader>

      <form class="contents" @submit.prevent="submit">
        <div class="flex flex-col gap-3 px-4 pt-4 pb-4">
          <dl v-if="hasSummary" class="remote-auth-summary">
            <div v-if="props.hostLabel">
              <dt>{{ t('shell.authDialog.host') }}</dt>
              <dd>
                <code>{{ props.hostLabel }}</code>
              </dd>
            </div>
            <div v-if="props.username">
              <dt>{{ t('shell.authDialog.username') }}</dt>
              <dd>
                <code>{{ props.username }}</code>
              </dd>
            </div>
          </dl>

          <label class="m-0 text-sm font-medium text-qt-fg" :for="`remote-auth-password`">{{
            t('shell.authDialog.password')
          }}</label>
          <QtermSecretInput
            id="remote-auth-password"
            v-model="password"
            autofocus
            autocomplete="current-password"
            :disabled="props.submitting"
            :placeholder="t('shell.authDialog.passwordPlaceholder')"
            input-class="h-qt-control-height"
          />

          <label v-if="props.canRemember" class="settings-toggle-row remote-auth-remember">
            <span>
              <strong>{{ t('shell.authDialog.remember') }}</strong>
              <small class="text-xs">{{ t('shell.authDialog.rememberHint') }}</small>
            </span>
            <QtermSwitch v-model="remember" :aria-label="t('shell.authDialog.remember')" />
          </label>

          <QtermFormMessage v-if="props.error" variant="error" role="alert" data-remote-auth-error>{{
            props.error
          }}</QtermFormMessage>
        </div>

        <QtermDialogFooter>
          <QtermButton variant="outline" type="button" :disabled="props.submitting" @click="cancel">
            {{ t('shell.authDialog.cancel') }}
          </QtermButton>

          <QtermButton type="submit" :disabled="props.submitting || password === ''">
            {{ props.submitting ? t('shell.authDialog.verifying') : t('shell.authDialog.submit') }}
          </QtermButton>
        </QtermDialogFooter>
      </form>
    </QtermDialogContent>
  </QtermDialog>
</template>

<style scoped>
.remote-auth-summary {
  display: grid;
  gap: 0.65rem;
  margin: 0;
  padding: 0.75rem;
  border: 1px solid var(--qt-border);
  border-radius: 0.6rem;
  background: var(--qt-subtle);
}
.remote-auth-summary div {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
}
.remote-auth-summary dt {
  color: var(--qt-muted);
}
.remote-auth-summary dd {
  margin: 0;
  min-width: 0;
  text-align: right;
}
.remote-auth-summary code {
  font-size: 0.8125rem;
  overflow-wrap: anywhere;
}
.remote-auth-remember {
  margin-top: 0.25rem;
}
</style>
