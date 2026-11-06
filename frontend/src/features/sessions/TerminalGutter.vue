<script setup lang="ts">
import { GUTTER_RIGHT_PADDING, type GutterRow } from './terminalGutter'

const props = withDefaults(
  defineProps<{
    rows: GutterRow[]

    width: number

    rowHeight: number

    topPadding: number

    color: string
    fontFamily: string
    fontSize: number

    suspended?: boolean
  }>(),
  { suspended: false },
)

const emit = defineEmits<{
  wheel: [deltaY: number, deltaMode: number]
  focus: []
}>()

function handleWheel(event: WheelEvent): void {
  emit('wheel', event.deltaY, event.deltaMode)
}

function handlePointerDown(): void {
  emit('focus')
}

function rowStyle(row: GutterRow): Record<string, string> {
  return {
    top: `${props.topPadding + row.offset * props.rowHeight}px`,
    height: `${props.rowHeight}px`,
    lineHeight: `${props.rowHeight}px`,
    paddingRight: `${GUTTER_RIGHT_PADDING}px`,
  }
}
</script>

<template>
  <div
    class="qterm-terminal-gutter relative h-full shrink-0 select-none overflow-hidden border-r-d text-right"
    :style="{
      width: `${width}px`,
      color,
      fontFamily,
      fontSize: `${fontSize}px`,
      visibility: suspended ? 'hidden' : undefined,
    }"
    aria-hidden="true"
    @wheel.prevent="handleWheel"
    @pointerdown="handlePointerDown"
  >
    <div
      v-for="row in rows"
      :key="row.key"
      class="absolute right-0 left-0 overflow-hidden whitespace-nowrap opacity-60 tabular-nums"
      :style="rowStyle(row)"
      :title="row.title"
    >
      {{ row.label ?? row.lineNumber }}
    </div>
  </div>
</template>
