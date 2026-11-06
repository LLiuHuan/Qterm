<script setup lang="ts">
import { armCurrentWindowDrag, toggleCurrentWindowMaximise } from '../../services/sessionWindows'

withDefaults(
  defineProps<{
    title: string
    statusTone?: string

    showStatus?: boolean
  }>(),
  { statusTone: 'idle', showStatus: true },
)

function hasTitlebarAction(event: MouseEvent): boolean {
  return Boolean((event.target as HTMLElement).closest('.session-window-titlebar__actions'))
}

function handleDoubleClick(event: MouseEvent): void {
  if (hasTitlebarAction(event)) return
  event.preventDefault()
  event.stopPropagation()
  void toggleCurrentWindowMaximise()
}

function handleMouseDown(event: MouseEvent): void {
  if (hasTitlebarAction(event)) return
  armCurrentWindowDrag(event)
}
</script>

<template>
  <header class="session-window-titlebar text-sm" @mousedown="handleMouseDown" @dblclick="handleDoubleClick">
    <span
      v-if="showStatus"
      class="session-window-titlebar__status size-[7px] flex-none rounded-[50%] bg-qt-muted"
      :class="`status-dot--${statusTone}`"
    ></span>
    <span class="session-window-titlebar__title truncate">{{ title }}</span>
    <div v-if="$slots.actions" class="session-window-titlebar__actions">
      <slot name="actions" />
    </div>
  </header>
</template>
