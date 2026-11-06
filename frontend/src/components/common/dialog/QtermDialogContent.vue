<script setup lang="ts">
import type { DialogContentEmits, DialogContentProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { reactiveOmit } from '@vueuse/core'
import { useForwardPropsEmits } from 'reka-ui'
import { X } from '@lucide/vue'
import { DialogClose, DialogContent } from '../../ui/dialog'
import QtermButton from '../button/QtermButton.vue'
import { useI18n } from '../../../locales'

defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<
    DialogContentProps & {
      class?: HTMLAttributes['class']

      overlayClass?: HTMLAttributes['class']
      size?: 'sm' | 'md' | 'lg' | 'xl' | '2xl' | '3xl'
      showCloseButton?: boolean
      closeOnEscape?: boolean
    }
  >(),

  { class: undefined, overlayClass: undefined, size: 'sm', showCloseButton: false, closeOnEscape: false },
)
const { t } = useI18n()
const emit = defineEmits<DialogContentEmits>()

const forwarded = useForwardPropsEmits(reactiveOmit(props, 'class', 'overlayClass', 'size', 'showCloseButton'), emit)

const sizes = {
  sm: 'sm:max-w-[420px]',
  md: 'sm:max-w-[480px]',
  lg: 'sm:max-w-[560px]',
  xl: 'sm:max-w-[600px]',
  '2xl': 'sm:max-w-[820px]',
  '3xl': 'sm:max-w-[900px]',
}
</script>

<template>
  <DialogContent
    v-bind="{ ...$attrs, ...forwarded }"
    :show-close-button="false"
    :class="['qterm-dialog gap-0 p-0', sizes[size], props.class]"
    :overlay-class="props.overlayClass"
  >
    <slot />
    <DialogClose v-if="showCloseButton" as-child>
      <QtermButton
        class="absolute top-2 right-2 hover:border-qt-border"
        variant="ghost"
        size="icon-sm"
        type="button"
        :aria-label="t('shell.app.close')"
      >
        <X :size="16" aria-hidden="true" />
      </QtermButton>
    </DialogClose>
  </DialogContent>
</template>
