<script setup lang="ts">
import type { DialogContentEmits, DialogContentProps } from 'reka-ui'

import type { HTMLAttributes } from 'vue'
import { XIcon } from '@lucide/vue'
import { reactiveOmit } from '@vueuse/core'
import { DialogClose, DialogContent, DialogOverlay, DialogPortal, useForwardPropsEmits } from 'reka-ui'
import { computed, inject } from 'vue'
import { cn } from '@/lib/utils'
import { avoidInitialScrollFocus } from './initialFocus'
import { dialogThemeContextKey } from './themeContext'

defineOptions({
  inheritAttrs: false,
})

const props = defineProps<DialogContentProps & { class?: HTMLAttributes['class'] }>()
const emits = defineEmits<DialogContentEmits>()
const dialogTheme = inject(dialogThemeContextKey, null)
const themeClass = computed(() => dialogTheme?.value.className ?? '')
const themeStyle = computed(() => dialogTheme?.value.style ?? {})

const delegatedProps = reactiveOmit(props, 'class')

const forwarded = useForwardPropsEmits(delegatedProps, emits)
</script>

<template>
  <DialogPortal>
    <DialogOverlay
      class="fixed inset-0 z-50 grid place-items-center overflow-y-auto bg-black/80 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0"
    >
      <DialogContent
        :class="
          cn(
            'relative z-50 grid outline-none w-full max-w-lg my-8 gap-4 border border-border bg-background p-6 shadow-lg duration-200 sm:rounded-lg md:w-full',
            themeClass,
            props.class,
          )
        "
        :style="themeStyle"
        v-bind="{
          ...$attrs,
          ...forwarded,
          onOpenAutoFocus: (event: Event) => {
            emits('openAutoFocus', event)
            avoidInitialScrollFocus(event)
          },
          onEscapeKeyDown: (event: KeyboardEvent) => {
            event.preventDefault()
            event.stopPropagation()
          },
        }"
        @pointer-down-outside="
          (event) => {
            const originalEvent = event.detail.originalEvent
            const target = originalEvent.target as HTMLElement
            if (originalEvent.offsetX > target.clientWidth || originalEvent.offsetY > target.clientHeight) {
              event.preventDefault()
            }
          }
        "
      >
        <slot />

        <DialogClose class="absolute top-4 right-4 p-0.5 transition-colors rounded-md hover:bg-secondary">
          <XIcon class="w-4 h-4" />
          <span class="sr-only">Close</span>
        </DialogClose>
      </DialogContent>
    </DialogOverlay>
  </DialogPortal>
</template>
