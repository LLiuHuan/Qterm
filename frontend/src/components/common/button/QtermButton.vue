<script setup lang="ts">
import type { ButtonVariants } from '../../ui/button'
import { Button } from '../../ui/button'

defineOptions({ inheritAttrs: false })

withDefaults(
  defineProps<{
    variant?: ButtonVariants['variant']
    size?: ButtonVariants['size']
    type?: 'button' | 'submit' | 'reset'
    disabled?: boolean
  }>(),
  { variant: 'default', size: 'default', type: 'button', disabled: false },
)
</script>

<template>
  <Button v-bind="$attrs" class="qterm-button" :variant="variant" :size="size" :type="type" :disabled="disabled">
    <slot />
  </Button>
</template>

<style scoped>
.qterm-button {
  font-family: inherit;
  line-height: 1.2;
  border-radius: var(--qt-control-radius, 6px);
  gap: var(--qt-control-gap, 4px);
  transition-duration: 150ms;
}

.qterm-button[data-size='default'] {
  min-height: var(--qt-button-height, 28px);
}

.qterm-button[data-size='sm'] {
  min-height: var(--qt-button-height-sm, 28px);
}

.qterm-button[data-size='xs'],
.qterm-button[data-size='icon-xs'] {
  min-height: var(--qt-button-height-xs, 24px);
}

.qterm-button[data-size='lg'],
.qterm-button[data-size='icon-lg'] {
  min-height: var(--qt-button-height-lg, 36px);
}

.qterm-button[data-size='icon'] {
  width: var(--qt-control-height, 32px);
  min-height: var(--qt-control-height, 32px);
}

.qterm-button[data-size='icon-sm'] {
  width: var(--qt-button-height, 28px);
  min-height: var(--qt-button-height, 28px);
}

.qterm-button[data-variant='link'] {
  min-height: unset;
}

.qterm-button[data-variant='default'] {
  color: var(--qt-overlay-primary-text, var(--qt-control-primary-text, var(--primary-foreground)));
  border-color: var(--qt-overlay-primary-border, transparent);
  background-color: var(--qt-overlay-primary-bg, var(--qt-control-primary-bg, var(--primary)));
}

.qterm-button[data-variant='default']:hover:not(:disabled) {
  background-color: color-mix(
    in srgb,
    var(--qt-overlay-primary-bg, var(--qt-control-primary-bg, var(--primary))) 88%,
    currentColor 12%
  );
}

.qterm-button[data-variant='secondary'] {
  color: var(--qt-overlay-primary-text, var(--qt-control-secondary-text, var(--secondary-foreground)));
  background-color: var(--qt-control-secondary-bg, var(--secondary));
}

.qterm-button[data-variant='outline'] {
  color: var(--qt-overlay-fg, var(--qt-control-text, var(--foreground)));
  border-color: var(--qt-overlay-border, var(--qt-control-border, var(--border)));
  background-color: var(--qt-control-input-bg, transparent);
}

.qterm-button[data-variant='ghost'],
.qterm-button[data-variant='link'] {
  color: var(--qt-overlay-fg, var(--qt-control-text, var(--foreground)));
  background-color: transparent;
}

.qterm-button[data-variant='outline']:hover:not(:disabled),
.qterm-button[data-variant='ghost']:hover:not(:disabled) {
  background-color: var(--qt-overlay-hover, var(--qt-hover));
}

.qterm-button[data-variant='destructive'] {
  color: var(--qt-control-danger, var(--destructive));
  border-color: #dc2626;
  background-color: color-mix(
    in srgb,
    var(--qt-control-danger, var(--destructive)) 2%,
    var(--qt-panel, var(--background))
  );
}

.dark .qterm-button[data-variant='destructive'] {
  color: var(--qt-control-danger, var(--destructive));
  border-color: var(--qt-control-danger, var(--destructive));
  background-color: color-mix(
    in srgb,
    var(--qt-control-danger, var(--destructive)) 2%,
    var(--qt-panel, var(--background))
  );
}

.qterm-button[data-variant='destructive']:hover:not(:disabled),
.qterm-button[data-variant='destructive']:focus-visible:not(:disabled) {
  color: #ffffff;
  border-color: #e60000;
  background-color: #e60000;
}
</style>
