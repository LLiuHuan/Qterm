<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'

const DURATION_MS = 240

const EASING = 'ease-in-out'

const props = withDefaults(
  defineProps<{
    show: boolean

    duration?: number
  }>(),
  { duration: DURATION_MS },
)

const root = ref<HTMLElement | null>(null)

const visible = ref(props.show)

const heightStyle = ref<string | null>(null)

const transitioning = ref(false)

let frame = 0
let timer = 0

function reducedMotion(): boolean {
  return typeof window !== 'undefined' && typeof window.matchMedia === 'function'
    ? window.matchMedia('(prefers-reduced-motion: reduce)').matches
    : false
}

function clearScheduled() {
  if (frame) window.cancelAnimationFrame(frame)
  if (timer) window.clearTimeout(timer)
  frame = 0
  timer = 0
}

function settle() {
  clearScheduled()
  heightStyle.value = null
  transitioning.value = false
}

function measureNatural(el: HTMLElement): number {
  const { overflow, maxHeight } = el.style
  el.style.overflow = 'visible'
  el.style.maxHeight = 'none'
  const height = el.offsetHeight
  el.style.overflow = overflow
  el.style.maxHeight = maxHeight
  return height
}

function scheduleSettle() {
  timer = window.setTimeout(settle, props.duration + 40)
}

function nextFrame(callback: () => void) {
  frame = window.requestAnimationFrame(callback)
}

function expand() {
  const el = root.value
  if (!el) return

  clearScheduled()
  visible.value = true

  void nextTick(() => {
    const target = measureNatural(el)
    heightStyle.value = '0px'
    transitioning.value = false
    void el.offsetHeight
    nextFrame(() => {
      heightStyle.value = `${target}px`
      transitioning.value = true
      scheduleSettle()
    })
  })
}

function collapse() {
  const el = root.value
  if (!el) return

  clearScheduled()

  heightStyle.value = `${measureNatural(el)}px`
  transitioning.value = false
  void el.offsetHeight
  nextFrame(() => {
    heightStyle.value = '0px'
    transitioning.value = true
    timer = window.setTimeout(() => {
      visible.value = false
      settle()
    }, props.duration + 40)
  })
}

function onTransitionEnd(event: TransitionEvent) {
  if (event.target !== root.value || event.propertyName !== 'max-height') return
  if (!props.show) visible.value = false
  settle()
}

watch(
  () => props.show,
  (show) => {
    if (reducedMotion()) {
      clearScheduled()
      heightStyle.value = null
      transitioning.value = false
      visible.value = show
      return
    }
    if (show) expand()
    else collapse()
  },
)

onBeforeUnmount(clearScheduled)
</script>

<template>
  <div
    v-show="visible"
    ref="root"
    :style="
      heightStyle === null
        ? undefined
        : {
            maxHeight: heightStyle,
            overflow: 'hidden',
            transition: transitioning ? `max-height ${props.duration}ms ${EASING}` : undefined,
          }
    "
    @transitionend="onTransitionEnd"
  >
    <slot />
  </div>
</template>
