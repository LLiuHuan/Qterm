<script setup lang="ts">
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '../common/control-primitives'

defineProps<{ label: string }>()
</script>

<template>
  <TooltipProvider :delay-duration="250">
    <Tooltip>
      <TooltipTrigger as-child>
        <span class="inline-flex shrink-0" tabindex="0" :aria-label="label">
          <slot />
        </span>
      </TooltipTrigger>
      <TooltipContent side="top" :aria-label="label" :side-offset="6" :collision-padding="8">{{
        label
      }}</TooltipContent>
    </Tooltip>
  </TooltipProvider>
</template>
