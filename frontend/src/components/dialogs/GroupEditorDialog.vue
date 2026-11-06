<script setup lang="ts">
import { FormField, QtermButton, QtermFormMessage, QtermInput } from '../common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../common/dialog'
import { useI18n } from '@/locales'

const props = defineProps<{ open: boolean; editing: boolean; name: string; error: string; saving: boolean }>()
defineEmits<{ close: []; submit: []; updateName: [value: string] }>()
const { t } = useI18n()
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => !open && $emit('close')">
    <QtermDialogContent v-if="props.open">
      <form :aria-label="t('shell.groupEditor.formAria')" @submit.prevent="$emit('submit')">
        <QtermDialogHeader
          :title="props.editing ? t('shell.groupEditor.renameTitle') : t('shell.groupEditor.createTitle')"
          :description="t('shell.groupEditor.description')"
          @close="$emit('close')"
        />
        <div class="dialog-body">
          <FormField v-slot="{ controlAttrs }" :label="t('shell.groupEditor.name')" for-id="group-name">
            <QtermInput
              v-bind="controlAttrs"
              :model-value="props.name"
              name="groupName"
              autocomplete="off"
              autocorrect="off"
              autocapitalize="off"
              spellcheck="false"
              :placeholder="t('shell.groupEditor.placeholder')"
              autofocus
              @update:model-value="(value) => $emit('updateName', String(value))"
            />
          </FormField>
          <QtermFormMessage v-if="props.error" variant="error">{{ props.error }}</QtermFormMessage>
        </div>
        <QtermDialogFooter>
          <QtermButton variant="outline" @click="$emit('close')">{{ t('shell.common.cancel') }}</QtermButton
          ><QtermButton type="submit" :disabled="props.saving">{{
            props.saving
              ? t('shell.common.saving')
              : props.editing
                ? t('shell.common.save')
                : t('shell.groupEditor.create')
          }}</QtermButton>
        </QtermDialogFooter>
      </form>
    </QtermDialogContent>
  </QtermDialog>
</template>
