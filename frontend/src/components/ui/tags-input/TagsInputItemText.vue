<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { TagsInputItemText, type TagsInputItemTextProps, useForwardProps } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps<TagsInputItemTextProps & { class?: HTMLAttributes['class'] }>()
const forwarded = useForwardProps(props)
</script>

<template>
  <TagsInputItemText v-bind="forwarded" :class="cn('truncate', props.class)">
    <slot />
  </TagsInputItemText>
</template>
