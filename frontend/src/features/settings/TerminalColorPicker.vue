<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from '@/locales'
import {
  ColorAreaRoot,
  ColorAreaArea,
  ColorAreaThumb,
  ColorFieldRoot,
  ColorFieldInput,
  ColorSliderRoot,
  ColorSliderTrack,
  ColorSliderThumb,
  ColorSwatch,
  PopoverRoot,
  PopoverTrigger,
  PopoverPortal,
  PopoverContent,
  normalizeColor,
  colorToString,
  type Color,
} from 'reka-ui'

const props = defineProps<{ modelValue: string; label: string; selected?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string]; select: [value: string] }>()
const { t } = useI18n()

const color = ref(normalizeColor(props.modelValue || '#3b82f6'))
const colorValue = computed(() => colorToString(color.value, 'hex'))
watch(
  () => props.modelValue,
  (value) => {
    if (value === colorValue.value) return

    try {
      color.value = normalizeColor(value || '#3b82f6')
    } catch {
      return
    }
  },
)
function update(value: string | Color): void {
  try {
    color.value = typeof value === 'string' ? normalizeColor(value) : value
  } catch {
    return
  }
  emit('update:modelValue', colorValue.value)
}
</script>

<template>
  <PopoverRoot>
    <PopoverTrigger
      class="terminal-color-swatch"
      :class="{ 'is-transparent': !props.modelValue }"
      :aria-pressed="props.selected"
      @click="props.modelValue && emit('select', props.modelValue)"
      :aria-label="props.label + t('appearance.core.colorPicker')"
    >
      <ColorSwatch :color="color" class="terminal-color-swatch-fill" />
    </PopoverTrigger>
    <PopoverPortal>
      <PopoverContent
        class="qterm-dialog terminal-color-popover"
        :side-offset="8"
        :collision-padding="12"
        align="start"
        :aria-label="props.label + t('appearance.core.colorPickerDialog')"
      >
        <ColorAreaRoot
          v-slot="{ style }"
          :model-value="color"
          color-space="hsl"
          x-channel="saturation"
          y-channel="lightness"
          @update:model-value="update"
        >
          <ColorAreaArea
            class="terminal-color-area relative h-[150px] overflow-hidden rounded-[5px] bg-[#fff]"
            :style="style"
          >
            <ColorAreaThumb class="terminal-color-thumb" />
          </ColorAreaArea>
        </ColorAreaRoot>
        <div class="terminal-color-sliders grid grid-cols-[32px_1fr] gap-[12px] mt-[12px]">
          <ColorSwatch
            :color="color"
            class="terminal-color-current size-[32px] rounded-[50%] bg-(--reka-color-swatch-color)"
          />
          <div>
            <ColorSliderRoot
              :model-value="color"
              channel="hue"
              color-space="hsl"
              class="terminal-color-slider"
              @update:model-value="update"
            >
              <ColorSliderTrack class="terminal-color-track" />
              <ColorSliderThumb class="terminal-color-thumb" :aria-label="t('appearance.core.hue')" />
            </ColorSliderRoot>
            <ColorSliderRoot
              :model-value="color"
              channel="alpha"
              class="terminal-color-slider"
              @update:model-value="update"
            >
              <ColorSliderTrack class="terminal-color-track" />
              <ColorSliderThumb class="terminal-color-thumb" :aria-label="t('appearance.core.alpha')" />
            </ColorSliderRoot>
          </div>
        </div>
        <div class="terminal-color-fields text-xs grid grid-cols-[1.3fr_repeat(3,_1fr)] gap-[6px] mt-[12px]">
          <ColorFieldRoot :model-value="color" class="terminal-color-hex" @update:model-value="update">
            <label class="text-xs">Hex<ColorFieldInput class="text-xs" :aria-label="props.label + ' Hex'" /></label>
          </ColorFieldRoot>
          <ColorFieldRoot
            v-for="channel in ['hue', 'saturation', 'lightness'] as const"
            :key="channel"
            :model-value="color"
            :channel="channel"
            color-space="hsl"
            @update:model-value="update"
          >
            <label class="text-xs"
              >{{ { hue: 'H', saturation: 'S', lightness: 'L' }[channel]
              }}<ColorFieldInput class="text-xs" :aria-label="channel"
            /></label>
          </ColorFieldRoot>
        </div>
      </PopoverContent>
    </PopoverPortal>
  </PopoverRoot>
</template>
