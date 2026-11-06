<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import QtermButton from '../button/QtermButton.vue'

const props = defineProps<{
  visible: number
  total: number
  loadMoreLabel: string

  summary?: string
}>()

const summaryText = computed(() => props.summary ?? `${props.visible}/${props.total}`)

const emit = defineEmits<{ loadMore: [] }>()
const trigger = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | undefined

function observeTrigger(): void {
  observer?.disconnect()
  observer = undefined
  if (!trigger.value || props.visible >= props.total) return
  const scrollViewport = trigger.value.closest<HTMLElement>('[data-slot="scroll-area-viewport"]')

  observer = new IntersectionObserver(
    (entries) => {
      if (entries.some((entry) => entry.isIntersecting) && props.visible < props.total) emit('loadMore')
    },
    { root: scrollViewport, rootMargin: '0px 0px 80px 0px' },
  )
  observer.observe(trigger.value)
}

onMounted(() => {
  void nextTick(observeTrigger)
})

watch(
  () => props.total,
  () => void nextTick(observeTrigger),
)

watch(
  () => props.visible >= props.total,
  () => void nextTick(observeTrigger),
)

onBeforeUnmount(() => observer?.disconnect())
</script>

<template>
  <footer v-if="props.total > 0" ref="trigger" class="flex min-h-8 items-center justify-end pt-1 text-qt-muted">
    <span class="text-xs tabular-nums" aria-live="polite">{{ summaryText }}</span>
    <QtermButton
      v-if="props.visible < props.total"
      class="sr-only"
      size="sm"
      variant="outline"
      type="button"
      @click="emit('loadMore')"
    >
      {{ props.loadMoreLabel }}
    </QtermButton>
  </footer>
</template>
