<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { FolderClosed } from '@lucide/vue'
import HostIcon from '../common/icon/HostIcon.vue'
import {
  hiddenHostTagCount,
  hostAddressLine,
  hostConnectionSummary,
  hostTags,
  visibleHostTags,
} from '../../features/hosts/hostSummary'
import { ASSET_DRAG_RETURN_MS } from '../../features/hosts/assetDrag'
import type { HostRecord } from '@/services/hosts.ts'

const props = withDefaults(
  defineProps<{
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

    variant?: 'card' | 'row'
  }>(),
  { host: null, returnTo: null, variant: 'card' },
)

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

const rowSubtitle = computed(() => (props.kind === 'host' && props.host ? hostAddressLine(props.host) : ''))

const RETURN_OFFSETS = [0, 0.18, 0.42, 0.66, 0.86, 1]
const RETURN_OPACITY = [1, 1, 0.95, 0.85, 0.6, 0.15]

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
    const distance = Math.hypot(dx, dy)

    const lift = Math.min(110, Math.max(24, distance * 0.22))
    const controlX = dx * 0.5
    const controlY = dy * 0.5 - lift * 2
    const frames = RETURN_OFFSETS.map((offset, index) => {
      const inverse = 1 - offset

      const x = 2 * inverse * offset * controlX + offset * offset * dx
      const y = 2 * inverse * offset * controlY + offset * offset * dy
      const scale = 1 + (scaleX - 1) * offset
      const scaleVertical = 1 + (scaleY - 1) * offset
      return {
        offset,
        transform: `translate3d(${x}px, ${y}px, 0) scale(${scale}, ${scaleVertical})`,
        opacity: RETURN_OPACITY[index],
      }
    })
    const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    const animation = element.animate(frames, {
      duration: reduceMotion ? 1 : ASSET_DRAG_RETURN_MS,
      easing: 'cubic-bezier(0.25, 0.8, 0.35, 1)',
      fill: 'forwards',
    })
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
        v-if="props.variant === 'row'"
        class="flex min-w-0 items-center gap-2.5 rounded-md border border-qt-border bg-[var(--qt-panel)] px-[7px] shadow-[0_8px_20px_rgb(0_0_0/18%)]"
        :class="rowSubtitle ? 'min-h-10 py-1' : 'min-h-8'"
        :style="{ width: cardWidth }"
      >
        <HostIcon
          v-if="props.kind === 'host'"
          class="flex-none"
          :icon="props.host?.icon ?? ''"
          :protocol="props.host?.protocol ?? ''"
          :size="16"
        />
        <span v-else class="grid size-4 flex-none place-items-center text-qt-info">
          <FolderClosed :size="16" />
        </span>
        <span class="host-row-main min-w-0 flex flex-1 flex-col gap-px">
          <span class="truncate text-sm">{{ title }}</span>
          <span v-if="rowSubtitle" class="truncate text-xs text-qt-muted">{{ rowSubtitle }}</span>
        </span>
      </div>
      <div
        v-else
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
