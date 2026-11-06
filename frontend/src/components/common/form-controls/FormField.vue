<script setup lang="ts">
import { computed, useSlots } from 'vue'
import type { HTMLAttributes } from 'vue'
import { Label } from '../../ui/label'
import QtermFormMessage from '../feedback/QtermFormMessage.vue'

defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    label?: string
    forId: string
    hint?: string
    error?: string
    class?: HTMLAttributes['class']
  }>(),
  { label: undefined, hint: undefined, error: undefined, class: undefined },
)

const slots = useSlots()
const hasError = () => Boolean(props.error) || Boolean(slots.error)
const hasHint = () => !hasError() && (Boolean(props.hint) || Boolean(slots.hint))

const messageId = computed(() => `${props.forId}-message`)
const controlAttrs = () => ({
  id: props.forId,
  'aria-describedby': hasError() || hasHint() ? messageId.value : undefined,
  'aria-invalid': hasError() ? true : undefined,
})
</script>

<template>
  <div v-bind="$attrs" class="form-field min-w-0 grid gap-[6px] text-qt-muted" :class="props.class">
    <Label v-if="props.label || slots.label" :for="props.forId">
      <slot name="label">{{ props.label }}</slot>
    </Label>
    <slot :control-attrs="controlAttrs()" />
    <QtermFormMessage v-if="hasError()" :id="messageId" variant="error">
      <slot name="error">{{ props.error }}</slot>
    </QtermFormMessage>
    <QtermFormMessage v-else-if="hasHint()" :id="messageId" variant="hint">
      <slot name="hint">{{ props.hint }}</slot>
    </QtermFormMessage>
  </div>
</template>
