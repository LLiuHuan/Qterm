<script setup lang="ts">
import type { ButtonVariants } from '../../ui/button'
import QtermButton from './QtermButton.vue'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '../../ui/tooltip'

const props = withDefaults(
  defineProps<{
    label: string
    variant?: ButtonVariants['variant']
    size?: ButtonVariants['size']
    active?: boolean
    disabled?: boolean
    pressed?: boolean
    ariaBusy?: boolean
    tooltipSide?: 'top' | 'right' | 'bottom' | 'left'

    danger?: boolean
  }>(),
  {
    variant: 'ghost',
    size: 'icon',
    active: false,
    disabled: false,
    pressed: undefined,
    tooltipSide: 'bottom',
    danger: false,
  },
)

defineEmits<{ click: [event: MouseEvent] }>()

defineOptions({ inheritAttrs: false })
</script>

<template>
  <TooltipProvider :delay-duration="350">
    <Tooltip>
      <TooltipTrigger as-child>
        <QtermButton
          v-bind="$attrs"
          :variant="props.variant"
          :size="props.size"
          class="icon-button p-0 flex-none"
          :class="{
            'is-active': props.active,
            'icon-button--outline': props.variant === 'outline',
            'icon-button--danger': props.danger,
          }"
          type="button"
          :aria-label="props.label"
          :aria-pressed="props.pressed"
          :aria-busy="props.ariaBusy"
          :disabled="props.disabled"
          @click="$emit('click', $event)"
        >
          <slot />
        </QtermButton>
      </TooltipTrigger>
      <TooltipContent :side="props.tooltipSide">{{ props.label }}</TooltipContent>
    </Tooltip>
  </TooltipProvider>
</template>

<style scoped>
.icon-button:hover {
  background-color: var(--qt-hover) !important;
}

.icon-button.icon-button--danger:hover:not(:disabled),
.icon-button.icon-button--danger:focus-visible:not(:disabled) {
  color: var(--qt-danger) !important;
  background-color: color-mix(in srgb, var(--qt-danger) 14%, transparent) !important;
}

.icon-button.icon-button--outline {
  border: 1px solid var(--qt-control-border, var(--qt-border));
}

.icon-button.icon-button--outline:hover:not(:disabled) {
  background-color: var(--qt-theme-hover, var(--qt-hover)) !important;
  box-shadow: none !important;
}
</style>
