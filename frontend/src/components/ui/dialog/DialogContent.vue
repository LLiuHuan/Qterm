<script setup lang="ts">
import type { DialogContentEmits, DialogContentProps } from 'reka-ui'

import type { HTMLAttributes } from 'vue'
import { XIcon } from '@lucide/vue'
import { reactiveOmit } from '@vueuse/core'
import { DialogClose, DialogContent, DialogPortal, useForwardPropsEmits } from 'reka-ui'
import { computed, inject } from 'vue'
import { cn } from '@/lib/utils'
import { applyInitialDialogFocus } from './initialFocus'
import { Button } from '@/components/ui/button'
import DialogOverlay from './DialogOverlay.vue'
import { dialogThemeContextKey } from './themeContext'

defineOptions({
  inheritAttrs: false,
})

const props = withDefaults(
  defineProps<
    DialogContentProps & {
      class?: HTMLAttributes['class']
      showCloseButton?: boolean
      closeOnEscape?: boolean

      overlayClass?: HTMLAttributes['class']
    }
  >(),
  {
    showCloseButton: true,
    closeOnEscape: false,
  },
)
const emits = defineEmits<DialogContentEmits>()
const dialogTheme = inject(dialogThemeContextKey, null)
const themeClass = computed(() => dialogTheme?.value.className ?? '')
const themeStyle = computed(() => dialogTheme?.value.style ?? {})

const delegatedProps = reactiveOmit(props, 'class', 'showCloseButton', 'closeOnEscape', 'overlayClass')

const forwarded = useForwardPropsEmits(delegatedProps, emits)
</script>

<template>
  <DialogPortal>
    <div class="pointer-events-none fixed inset-0 z-50 flex-cc p-4">
      <DialogOverlay class="pointer-events-auto" :class="props.overlayClass" />
      <DialogContent
        data-slot="dialog-content"
        v-bind="{
          ...$attrs,
          ...forwarded,
          onOpenAutoFocus: (event: Event) => {
            emits('openAutoFocus', event)
            applyInitialDialogFocus(event)
          },
          onEscapeKeyDown: (event: KeyboardEvent) => {
            if (!props.closeOnEscape) {
              event.preventDefault()
              event.stopPropagation()
            }
          },
        }"
        :class="
          cn(
            'bg-popover text-popover-foreground data-open:animate-in data-closed:animate-out data-closed:fade-out-0 data-open:fade-in-0 data-closed:zoom-out-95 data-open:zoom-in-95 ring-foreground/10 relative grid max-h-full w-full gap-4 rounded-xl p-4 text-sm ring-1 duration-100 sm:max-w-sm pointer-events-auto z-50 outline-none',
            themeClass,
            props.class,
          )
        "
        :style="themeStyle"
      >
        <slot />

        <DialogClose v-if="showCloseButton" data-slot="dialog-close" as-child>
          <Button variant="ghost" class="absolute top-2 right-2" size="icon-sm">
            <XIcon />
            <span class="sr-only">Close</span>
          </Button>
        </DialogClose>
      </DialogContent>
    </div>
  </DialogPortal>
</template>
