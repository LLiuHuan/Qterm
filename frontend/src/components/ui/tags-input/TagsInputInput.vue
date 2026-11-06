<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { TagsInputInput, type TagsInputInputProps, useForwardProps } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps<TagsInputInputProps & { class?: HTMLAttributes['class'] }>()
const forwarded = useForwardProps(props)
</script>

<template>
  <TagsInputInput
    v-bind="forwarded"
    :class="cn('min-w-24 flex-1 bg-transparent text-sm outline-none placeholder:text-qt-muted', props.class)"
  />
</template>
