<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { TagsInputClear, type TagsInputClearProps, useForwardProps } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps<TagsInputClearProps & { class?: HTMLAttributes['class'] }>()
const forwarded = useForwardProps(props)
</script>

<template>
  <TagsInputClear v-bind="forwarded" :class="cn('inline-flex items-center', props.class)">
    <slot />
  </TagsInputClear>
</template>
