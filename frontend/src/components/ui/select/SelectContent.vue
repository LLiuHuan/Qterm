<script setup lang="ts">
import type { SelectContentEmits, SelectContentProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { reactiveOmit } from '@vueuse/core'
import { SelectContent, SelectPortal, SelectViewport, useForwardPropsEmits } from 'reka-ui'
import { cn } from '@/lib/utils'
import { ScrollAreaRoot, ScrollAreaViewport } from 'reka-ui'
import ScrollBar from '../scroll-area/ScrollBar.vue'
import { computed, inject } from 'vue'
import { dialogThemeContextKey } from '../dialog/themeContext'

defineOptions({
  inheritAttrs: false,
})

const props = withDefaults(
  defineProps<
    SelectContentProps & {
      class?: HTMLAttributes['class']
      onViewportScroll?: (event: Event) => void
    }
  >(),
  {
    position: 'item-aligned',
    align: 'center',
    onViewportScroll: undefined,
  },
)
const emits = defineEmits<SelectContentEmits>()

const delegatedProps = reactiveOmit(props, 'class', 'onViewportScroll')

const forwarded = useForwardPropsEmits(delegatedProps, emits)
const dialogTheme = inject(dialogThemeContextKey, null)
const themeClass = computed(() => dialogTheme?.value.className ?? '')
const themeStyle = computed(() => dialogTheme?.value.style ?? {})
</script>

<template>
  <SelectPortal>
    <SelectContent
      data-slot="select-content"
      :data-align-trigger="position === 'item-aligned'"
      v-bind="{ ...$attrs, ...forwarded }"
      :class="
        cn(
          'bg-popover text-popover-foreground data-open:animate-in data-closed:animate-out data-closed:fade-out-0 data-open:fade-in-0 data-closed:zoom-out-95 data-open:zoom-in-95 data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-right-2 data-[side=right]:slide-in-from-left-2 data-[side=top]:slide-in-from-bottom-2 ring-foreground/10 min-w-36 rounded-lg p-1 shadow-md ring-1 duration-100 data-[side=inline-start]:slide-in-from-right-2 data-[side=inline-end]:slide-in-from-left-2 relative z-50 max-h-(--reka-select-content-available-height) origin-(--reka-select-content-transform-origin) overflow-hidden data-[align-trigger=true]:animate-none',
          position === 'popper' &&
            'data-[side=bottom]:translate-y-1 data-[side=left]:-translate-x-1 data-[side=right]:translate-x-1 data-[side=top]:-translate-y-1',
          'qterm-select-content',
          themeClass,
          props.class,
        )
      "
      :style="themeStyle"
    >
      <div
        v-if="$slots.header"
        class="qterm-select-content-header pt-[4px] pr-[4px] pb-[8px] pl-[4px] border-b-d bg-qt-panel"
      >
        <slot name="header" />
      </div>
      <ScrollAreaRoot type="hover" :scroll-hide-delay="600" class="qterm-select-scroll"
        ><ScrollAreaViewport as-child data-slot="scroll-area-viewport" @scroll.passive="props.onViewportScroll">
          <SelectViewport
            :data-position="position"
            :class="cn('data-[position=popper]:w-full data-[position=popper]:min-w-0  space-y-1')"
          >
            <slot />
          </SelectViewport> </ScrollAreaViewport
        ><ScrollBar
      /></ScrollAreaRoot>
    </SelectContent>
  </SelectPortal>
</template>
