<script setup lang="ts">
import { computed } from 'vue'
import { openExternalUrl } from '../../services/externalLink'
import { markdownUrl } from './markdownPolicy'

defineOptions({ inheritAttrs: false })
const props = defineProps<{ href?: string; title?: string; id?: string; class?: string }>()
const footnoteId = /^ai-markdown-[a-zA-Z0-9_-]+-fn(?:ref)?-[a-zA-Z0-9_-]+$/
const fragment = computed(() =>
  props.href?.startsWith('#') && footnoteId.test(props.href.slice(1)) ? props.href : undefined,
)
const target = computed(() => fragment.value || markdownUrl(props.href))
const anchorId = computed(() => (props.id && footnoteId.test(props.id) ? props.id : undefined))

function followLink(event: MouseEvent): void {
  if (fragment.value) {
    const root = (event.currentTarget as HTMLElement | null)?.closest('[data-ai-markdown-message]')
    const destination = Array.from(root?.querySelectorAll('[id]') ?? []).find(
      (node) => node.id === fragment.value?.slice(1),
    )
    destination?.scrollIntoView({ block: 'nearest' })
  } else if (target.value) {
    void openExternalUrl(target.value)
  }
}
</script>

<template>
  <a
    v-if="target"
    :id="anchorId"
    :class="props.class"
    :href="target"
    :title="props.title"
    rel="noopener noreferrer"
    @click.prevent="followLink"
    ><slot
  /></a>
  <span v-else><slot /></span>
</template>
