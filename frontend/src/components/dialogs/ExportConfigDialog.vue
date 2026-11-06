<script setup lang="ts">
import { ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import { QtermButton, QtermFormMessage, QtermInput } from '../common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../common/dialog'
import { Label } from '../common/control-primitives'
import type { CredentialGateway } from '@/services/credentials.ts'
import { errorMessage } from '@/services/errors.ts'
import { useI18n } from '@/locales'

const props = defineProps<{
  open: boolean
  gateway: CredentialGateway
}>()

const emit = defineEmits<{
  close: []
  exported: []
}>()

const { t } = useI18n()

const password = ref('')
const confirmation = ref('')
const error = ref('')
const busy = ref(false)

function reset(): void {
  password.value = ''
  confirmation.value = ''
  error.value = ''
  busy.value = false
}

function requestClose(): void {
  if (busy.value) return
  reset()
  emit('close')
}

function validate(): boolean {
  if (password.value.length < 8) {
    error.value = t('asset.password.tooShort')
    return false
  }
  if (password.value.length > 4096) {
    error.value = t('asset.password.tooLong')
    return false
  }
  if (password.value !== confirmation.value) {
    error.value = t('asset.password.mismatch')
    return false
  }
  return true
}

function errorText(cause: unknown): string {
  return errorMessage(cause, t('asset.export.failure'))
}

async function submit(): Promise<void> {
  error.value = ''
  if (!validate()) return
  busy.value = true
  try {
    const exported = await props.gateway.exportResources(password.value)
    if (exported) {
      toast.success(t('asset.export.success'))
      emit('exported')
      reset()
      emit('close')
    }
  } catch (cause) {
    error.value = errorText(cause)
  } finally {
    busy.value = false
  }
}

watch(
  () => props.open,
  (open) => {
    if (!open) reset()
  },
)
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => !open && requestClose()">
    <QtermDialogContent v-if="props.open" size="md" data-export-config-dialog>
      <QtermDialogHeader
        :title="t('asset.export.title')"
        :description="t('asset.export.description')"
        :close-disabled="busy"
        @close="requestClose"
      />
      <form class="dialog-body security-form" @submit.prevent="submit">
        <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
          <Label for="export-config-password">{{ t('asset.export.password') }}</Label>
          <QtermInput
            id="export-config-password"
            v-model="password"
            type="password"
            autocomplete="new-password"
            :placeholder="t('asset.export.passwordPlaceholder')"
            autofocus
          />
        </div>
        <div class="form-field min-w-0 grid gap-[6px] text-qt-muted">
          <Label for="export-config-confirmation">{{ t('asset.export.confirmPassword') }}</Label>
          <QtermInput
            id="export-config-confirmation"
            v-model="confirmation"
            type="password"
            autocomplete="new-password"
            :placeholder="t('asset.export.confirmPasswordPlaceholder')"
          />
        </div>
        <QtermFormMessage v-if="error" variant="error">{{ error }}</QtermFormMessage>
      </form>
      <QtermDialogFooter>
        <QtermButton variant="outline" type="button" :disabled="busy" @click="requestClose">{{
          t('asset.export.cancel')
        }}</QtermButton>
        <QtermButton type="button" :disabled="busy" data-export-config-submit @click="submit">{{
          busy ? t('asset.export.busy') : t('asset.export.submit')
        }}</QtermButton>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>
