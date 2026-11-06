<script setup lang="ts">
import { computed } from 'vue'
import type { HTMLAttributes } from 'vue'

defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    variant?: 'hint' | 'error' | 'success'

    role?: HTMLAttributes['role']

    announce?: boolean

    as?: 'p' | 'small' | 'div'
    class?: HTMLAttributes['class']
  }>(),
  { variant: 'hint', role: undefined, announce: true, as: 'p', class: undefined },
)

const messageClass = computed(() => `${props.variant === 'hint' ? 'dialog-hint' : `form-${props.variant}`} text-xs`)
const messageRole = computed(() => {
  if (props.role) return props.role
  if (!props.announce) return undefined
  return props.variant === 'error' ? 'alert' : undefined
})
</script>

<template>
  <component :is="props.as" v-bind="$attrs" :class="[messageClass, props.class]" :role="messageRole">
    <slot />
  </component>
</template>
