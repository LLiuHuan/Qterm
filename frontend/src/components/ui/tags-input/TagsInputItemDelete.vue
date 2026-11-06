<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { TagsInputItemDelete, type TagsInputItemDeleteProps, useForwardProps } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps<TagsInputItemDeleteProps & { class?: HTMLAttributes['class'] }>()
const forwarded = useForwardProps(props)
</script>

<template>
  <TagsInputItemDelete
    v-bind="forwarded"
    :class="cn('inline-flex shrink-0 items-center rounded-sm opacity-60 hover:opacity-100', props.class)"
  >
    <slot />
  </TagsInputItemDelete>
</template>
