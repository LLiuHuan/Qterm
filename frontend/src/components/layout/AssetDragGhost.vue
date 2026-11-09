<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { FolderClosed } from '@lucide/vue'
import HostIcon from '../common/icon/HostIcon.vue'
import { hiddenHostTagCount, hostConnectionSummary, hostTags, visibleHostTags } from '../../features/hosts/hostSummary'
import type { HostRecord } from '@/services/hosts.ts'

const props = defineProps<{
  kind: 'host' | 'group'

  host?: HostRecord | null

  label: string

  valid: boolean

  x: number
  y: number

  anchorX: number
  anchorY: number

  sourceWidth: number

  returnTo?: { left: number; top: number; width: number; height: number } | null
}>()

const emit = defineEmits<{ returned: [] }>()

const root = ref<HTMLElement | null>(null)
const size = ref({ width: 0, height: 0 })
onMounted(() => {
  const rect = root.value?.getBoundingClientRect()
  if (rect) size.value = { width: rect.width, height: rect.height }
})

const position = computed(() => ({
  left: `${props.x - size.value.width * props.anchorX}px`,
  top: `${props.y - size.value.height * props.anchorY}px`,
}))

const cardWidth = computed(() => (props.sourceWidth > 0 ? `${props.sourceWidth}px` : '16rem'))

const tags = computed(() => (props.kind === 'host' && props.host ? visibleHostTags(props.host) : []))
const hiddenTags = computed(() => (props.kind === 'host' && props.host ? hiddenHostTagCount(props.host) : 0))
const hasTags = computed(
  () => props.kind === 'host' && props.host !== null && props.host !== undefined && hostTags(props.host).length > 0,
)

const title = computed(() => (props.kind === 'host' ? (props.host?.name ?? props.label) : props.label))
const subtitle = computed(() => (props.kind === 'host' && props.host ? hostConnectionSummary(props.host) : ''))

watch(
  () => props.returnTo,
  (target) => {
    const element = root.value
    if (!target || !element) return
    const base = element.getBoundingClientRect()
    if (base.width === 0 || base.height === 0) {
      emit('returned')
      return
    }
    const dx = target.left - base.left
    const dy = target.top - base.top
    const scaleX = target.width > 0 ? target.width / base.width : 1
    const scaleY = target.height > 0 ? target.height / base.height : 1
    const animation = element.animate(
      [
        { transform: 'translate3d(0, 0, 0) scale(1, 1)', opacity: 1 },
        { transform: `translate3d(${dx}px, ${dy}px, 0) scale(${scaleX}, ${scaleY})`, opacity: 0.35 },
      ],
      { duration: 220, easing: 'cubic-bezier(0.2, 0.8, 0.3, 1)', fill: 'forwards' },
    )
    animation.onfinish = () => emit('returned')
    animation.oncancel = () => emit('returned')
  },
  { flush: 'post' },
)
</script>

<template>
  <Teleport to="body">
    <div
      ref="root"
      class="asset-drag-ghost"
      :class="{ 'is-invalid': !props.valid }"
      :style="position"
      aria-hidden="true"
    >
      <div
        class="flex min-w-0 items-center gap-3 rounded-lg border-d bg-[var(--qt-panel)] p-3 text-left shadow-[0_10px_24px_rgb(0_0_0/18%)]"
        :style="{ width: cardWidth }"
      >
        <span
          class="grid size-10 flex-none place-items-center rounded-lg border-d bg-qt-subtle"
          :class="{ 'text-qt-info': props.kind === 'group' }"
        >
          <HostIcon
            v-if="props.kind === 'host'"
            :icon="props.host?.icon ?? ''"
            :protocol="props.host?.protocol ?? ''"
            :size="22"
          />
          <FolderClosed v-else :size="20" />
        </span>
        <span class="min-w-0">
          <span class="block truncate text-sm">{{ title }}</span>
          <span v-if="subtitle" class="block truncate text-xs text-qt-muted">{{ subtitle }}</span>
          <span v-if="hasTags" class="mt-1 flex flex-wrap items-center gap-1">
            <span
              v-for="tag in tags"
              :key="tag"
              class="max-w-24 truncate rounded bg-qt-control-muted-bg px-1.5 py-0.5 text-[11px] leading-4 text-qt-control-muted-text"
              >{{ tag }}</span
            >
            <span v-if="hiddenTags > 0" class="text-[11px] leading-4 text-qt-muted">+{{ hiddenTags }}</span>
          </span>
        </span>
      </div>
    </div>
  </Teleport>
</template>
