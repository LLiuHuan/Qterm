<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { Ban, Keyboard, Power, RotateCcw } from '@lucide/vue'
import { QtermButton } from '../../components/common'
import { isMacPlatform, refreshIsMacPlatform } from '@/lib/platform.ts'
import { DEFAULT_SETTINGS } from './settingsStore'
import ShortcutKeycaps from './ShortcutKeycaps.vue'
import { shortcutDisplay, shortcutFromKeyboardEvent } from './shortcutStore'
import { useI18n } from '@/locales'

const props = withDefaults(
  defineProps<{
    binding: string

    name?: string
  }>(),
  {
    name: '',
  },
)

const emit = defineEmits<{
  'update:binding': [value: string]

  'recording-change': [value: boolean]
}>()

const { t } = useI18n()
const isMac = isMacPlatform
const recording = ref(false)

function setRecording(value: boolean): void {
  if (recording.value === value) return
  recording.value = value
  emit('recording-change', value)
}

const displayTokens = computed(() => (props.binding ? shortcutDisplay(props.binding, isMac.value).split(' + ') : []))
const defaultBinding = DEFAULT_SETTINGS.globalShortcut ?? ''

function handleKeydown(event: KeyboardEvent): void {
  if (!recording.value) return
  event.preventDefault()
  event.stopPropagation()
  if (event.key === 'Escape') {
    setRecording(false)
    return
  }
  const binding = shortcutFromKeyboardEvent(event, isMac.value)

  if (!binding) return
  setRecording(false)
  emit('update:binding', binding)
}

function startRecording(): void {
  setRecording(true)
}

function cancelRecording(): void {
  setRecording(false)
}

function resetBinding(): void {
  setRecording(false)
  emit('update:binding', defaultBinding)
}

function toggleBinding(): void {
  setRecording(false)
  emit('update:binding', props.binding ? '' : defaultBinding)
}

defineExpose({ cancelRecording })

onMounted(() => {
  refreshIsMacPlatform()

  window.addEventListener('keydown', handleKeydown, true)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeydown, true)
})
</script>

<template>
  <div class="shortcut-actions" data-global-shortcut>
    <div
      class="shortcut-key-display"
      :aria-label="t('settings.shortcuts.binding.aria', { name: props.name, binding: props.binding })"
      :data-global-shortcut-binding="props.binding || undefined"
    >
      <span v-if="displayTokens.length === 0" class="text-xs text-qt-muted">
        {{ t('settings.shortcuts.binding.unbound') }}
      </span>
      <ShortcutKeycaps :tokens="displayTokens" />
    </div>
    <QtermButton
      variant="outline"
      size="xs"
      type="button"
      class="shortcut-record-button min-w-[112px] transition"
      :class="{ recording }"
      :data-global-shortcut-capture="recording ? 'true' : undefined"
      @click="recording ? cancelRecording() : startRecording()"
    >
      <Keyboard :size="14" aria-hidden="true" />
      {{ recording ? t('settings.shortcuts.recording.active') : t('settings.shortcuts.recording.start') }}
    </QtermButton>
    <QtermButton
      variant="ghost"
      size="icon-xs"
      type="button"
      :title="t('settings.shortcuts.action.resetTitle')"
      :aria-label="t('settings.shortcuts.action.reset', { name: t('application.window.globalShortcut.label') })"
      :disabled="props.binding === defaultBinding"
      data-global-shortcut-reset
      @click="resetBinding"
    >
      <RotateCcw :size="14" aria-hidden="true" />
    </QtermButton>
    <QtermButton
      variant="ghost"
      size="icon-xs"
      type="button"
      data-global-shortcut-toggle
      :aria-label="
        props.binding
          ? t('settings.shortcuts.action.disable', { name: t('application.window.globalShortcut.label') })
          : t('settings.shortcuts.action.enable', { name: t('application.window.globalShortcut.label') })
      "
      :title="props.binding ? t('settings.shortcuts.action.disableTitle') : t('settings.shortcuts.action.enableTitle')"
      @click="toggleBinding"
    >
      <Ban v-if="props.binding" :size="14" aria-hidden="true" />
      <Power v-else :size="14" aria-hidden="true" />
    </QtermButton>
  </div>
</template>
