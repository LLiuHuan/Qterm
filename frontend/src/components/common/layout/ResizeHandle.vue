<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

const props = withDefaults(
  defineProps<{
    orientation: 'vertical' | 'horizontal'
    label: string
    disabled?: boolean
    keyboardStep?: number
  }>(),
  {
    disabled: false,
    keyboardStep: 12,
  },
)

const emit = defineEmits<{
  resizeStart: []
  resize: [delta: number]
  resizeEnd: []
}>()

const active = ref(false)
const pointerIntent = ref(false)
const keyboardFocus = ref(false)
let pointerId: number | null = null
let origin = 0
let keyboardModality = false

function coordinate(event: PointerEvent): number {
  return props.orientation === 'vertical' ? event.clientX : event.clientY
}

function bodyClass(): string {
  return `qterm-resizing-${props.orientation}`
}

function cleanup(emitEnd = false): void {
  if (!active.value) return
  active.value = false
  pointerId = null
  document.body.classList.remove(bodyClass())
  window.removeEventListener('pointermove', onPointerMove)
  window.removeEventListener('pointerup', onPointerUp)
  window.removeEventListener('pointercancel', onPointerCancel)
  window.removeEventListener('blur', onPointerCancel)
  if (emitEnd) {
    emit('resizeEnd')

    window.dispatchEvent(new CustomEvent('qterm:layout-resize-end'))
  }
}

function onPointerDown(event: PointerEvent): void {
  if (props.disabled || event.button !== 0) return
  pointerIntent.value = true
  event.preventDefault()
  event.stopPropagation()
  cleanup()
  active.value = true
  pointerId = event.pointerId
  origin = coordinate(event)
  document.body.classList.add(bodyClass())
  emit('resizeStart')
  window.addEventListener('pointermove', onPointerMove)
  window.addEventListener('pointerup', onPointerUp)
  window.addEventListener('pointercancel', onPointerCancel)
  window.addEventListener('blur', onPointerCancel)
}

function onPointerMoveIntent(): void {
  pointerIntent.value = true
}

function onPointerLeave(): void {
  if (!active.value) pointerIntent.value = false
}

function onWindowKeydown(event: KeyboardEvent): void {
  if (event.key === 'Tab') keyboardModality = true
}

function onWindowPointerDown(): void {
  keyboardModality = false
}

function onFocus(): void {
  keyboardFocus.value = keyboardModality
}

function onBlur(): void {
  keyboardFocus.value = false
}

function onPointerMove(event: PointerEvent): void {
  if (!active.value || event.pointerId !== pointerId) return
  event.preventDefault()
  emit('resize', coordinate(event) - origin)
}

function onPointerUp(event: PointerEvent): void {
  if (!active.value || event.pointerId !== pointerId) return
  event.preventDefault()
  cleanup(true)
}

function onPointerCancel(): void {
  cleanup(true)
}

function onKeydown(event: KeyboardEvent): void {
  if (props.disabled) return
  const negative = props.orientation === 'vertical' ? event.key === 'ArrowLeft' : event.key === 'ArrowUp'
  const positive = props.orientation === 'vertical' ? event.key === 'ArrowRight' : event.key === 'ArrowDown'
  if (!negative && !positive) return
  event.preventDefault()
  keyboardFocus.value = true
  const multiplier = event.shiftKey ? 4 : 1
  emit('resizeStart')
  emit('resize', (negative ? -1 : 1) * props.keyboardStep * multiplier)
  emit('resizeEnd')
}

onMounted(() => {
  window.addEventListener('keydown', onWindowKeydown, true)
  window.addEventListener('pointerdown', onWindowPointerDown, true)
})

onBeforeUnmount(() => {
  cleanup()
  window.removeEventListener('keydown', onWindowKeydown, true)
  window.removeEventListener('pointerdown', onWindowPointerDown, true)
})
</script>

<template>
  <div
    class="qterm-resize-handle absolute z-20 block p-0 border-0 bg-transparent outline-none touch-none"
    :class="[
      `is-${props.orientation}`,
      {
        'is-active': active,
        'is-disabled': props.disabled,
        'has-pointer-intent': pointerIntent,
        'is-keyboard-focus': keyboardFocus,
      },
    ]"
    role="separator"
    :aria-label="props.label"
    :aria-orientation="props.orientation"
    :aria-disabled="props.disabled || undefined"
    :tabindex="props.disabled ? -1 : 0"
    @pointerdown="onPointerDown"
    @pointermove="onPointerMoveIntent"
    @pointerleave="onPointerLeave"
    @focus="onFocus"
    @blur="onBlur"
    @keydown="onKeydown"
  >
    <span class="qterm-resize-handle__line" aria-hidden="true" />
  </div>
</template>
