<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { Info, LoaderCircle, TriangleAlert, X } from '@lucide/vue'
import QtermButton from '../button/QtermButton.vue'

defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    variant?: 'mask' | 'card'

    tone?: 'info' | 'busy' | 'warning' | 'error'

    title?: string

    message?: string

    surface?: 'theme' | 'canvas'

    busy?: boolean

    titleSize?: 'sm' | 'md'

    dismissible?: boolean

    closeLabel?: string

    titleAttrs?: Record<string, string>
  }>(),
  {
    variant: 'card',
    tone: 'info',
    surface: 'theme',
    title: '',
    message: '',
    busy: false,
    titleSize: 'md',
    dismissible: false,
    closeLabel: '',
    titleAttrs: undefined,
  },
)

const emit = defineEmits<{ close: [] }>()

const attrs = useAttrs()

const role = computed(() => (props.tone === 'error' ? 'alert' : 'status'))

const ariaLive = computed(() => (props.tone === 'error' ? undefined : 'polite'))

const toneBorderClass = computed(() => {
  switch (props.tone) {
    case 'warning':
      return 'border-qt-warning/35'
    case 'error':
      return 'border-qt-danger/40'
    default:
      return 'border-qt-theme-divider'
  }
})

const toneClass = computed(() => {
  if (props.surface === 'canvas') {
    switch (props.tone) {
      case 'busy':
      case 'warning':
        return 'text-amber-300'
      case 'error':
        return 'text-red-300'
      default:
        return 'text-sky-300'
    }
  }
  switch (props.tone) {
    case 'busy':
      return 'text-qt-accent'
    case 'warning':
      return 'text-qt-warning'
    case 'error':
      return 'text-qt-danger'
    default:
      return 'text-qt-info'
  }
})

const showSpinner = computed(() => props.busy || props.tone === 'busy' || props.tone === 'info')

const glassClass = computed(() => [
  'session-overlay-glass',
  'relative flex h-auto w-fit max-w-[min(100%-2rem,28rem)] min-w-28 min-h-28 flex-col items-center justify-center gap-3 px-7 py-6',
  props.surface === 'canvas' ? 'session-overlay-glass--canvas' : toneBorderClass.value,
])

const closeButtonClass =
  'absolute top-1 right-1 text-qt-overlay-muted hover:bg-(--qt-overlay-hover) hover:text-qt-overlay-fg'

const titleClass = computed(() =>
  props.titleSize === 'sm' ? 'text-xs font-normal text-qt-overlay-fg' : 'text-sm font-medium text-qt-overlay-fg',
)
const messageClass = 'max-w-lg text-xs leading-5 break-words text-qt-overlay-muted'
</script>

<template>
  <div
    v-if="variant === 'mask'"
    v-bind="attrs"
    class="flex flex-col items-center justify-center gap-3 px-6 text-center font-semibold text-qt-text session-overlay--mask [-webkit-backdrop-filter:blur(12px)_saturate(1.2)] [backdrop-filter:blur(12px)_saturate(1.2)]"
    :role="role"
    :aria-live="ariaLive"
    :aria-busy="props.busy ? 'true' : undefined"
  >
    <LoaderCircle v-if="showSpinner" :size="28" class="animate-spin" :class="toneClass" aria-hidden="true" />
    <span v-if="props.title">{{ props.title }}</span>
    <span v-if="props.message" class="max-w-lg text-sm font-normal">{{ props.message }}</span>
    <slot />
  </div>

  <div
    v-else
    v-bind="attrs"
    class="session-overlay absolute inset-0 z-10 flex flex-col items-center justify-center gap-3 px-6 text-center"
    :class="surface === 'canvas' ? 'session-overlay--canvas' : 'session-overlay--theme'"
    :role="role"
    :aria-live="ariaLive"
    :aria-busy="props.busy ? 'true' : undefined"
  >
    <div :class="glassClass">
      <QtermButton
        v-if="dismissible"
        variant="ghost"
        size="icon-xs"
        class="pointer-events-auto"
        :class="closeButtonClass"
        :title="props.closeLabel || undefined"
        :aria-label="props.closeLabel || undefined"
        data-session-overlay-close
        @click="emit('close')"
      >
        <X :size="14" aria-hidden="true" />
      </QtermButton>

      <slot name="icon">
        <LoaderCircle v-if="showSpinner" :size="40" class="animate-spin" :class="toneClass" aria-hidden="true" />
        <TriangleAlert v-else-if="tone === 'error'" :size="34" :class="toneClass" aria-hidden="true" />
        <Info v-else :size="32" :class="toneClass" aria-hidden="true" />
      </slot>

      <strong v-if="props.title" v-bind="props.titleAttrs" :class="titleClass">{{ props.title }}</strong>
      <span v-if="props.message" :class="messageClass">
        {{ props.message }}
      </span>

      <slot />

      <div v-if="$slots.actions" class="mt-1 flex-cc gap-2">
        <slot name="actions" />
      </div>
    </div>
  </div>
</template>
