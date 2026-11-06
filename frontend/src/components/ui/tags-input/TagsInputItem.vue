<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { TagsInputItem, type TagsInputItemProps, useForwardProps } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps<TagsInputItemProps & { class?: HTMLAttributes['class'] }>()
const forwarded = useForwardProps(props)
</script>

<template>
  <TagsInputItem
    v-bind="forwarded"
    :class="
      cn(
        'inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs data-[state=active]:ring-1 data-[state=active]:ring-(--qt-control-focus)',
        props.class,
      )
    "
  >
    <slot />
  </TagsInputItem>
</template>
