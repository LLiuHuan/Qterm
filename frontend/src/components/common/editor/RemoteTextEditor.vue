<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch, type Component } from 'vue'
import { useRemoteEditorFocus } from '../../../features/remoteedit/remoteEditorFocus'
import type { RemoteTextEditorEngine, RemoteTextEditorExposed, RemoteTextEditorProps } from './types'

const props = withDefaults(defineProps<RemoteTextEditorProps & { engine?: RemoteTextEditorEngine }>(), {
  readonly: false,
  theme: 'light',
  engine: 'monaco',
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
  save: []
  dirtyChange: [dirty: boolean]

  engineFallback: [engine: RemoteTextEditorEngine]

  ready: []
}>()

const adapter = shallowRef<Component | null>(null)
const adapterRef = ref<RemoteTextEditorExposed | null>(null)
const activeEngine = ref<RemoteTextEditorEngine>(props.engine)
const fallbackNotice = ref(false)

async function loadEngine(engine: RemoteTextEditorEngine): Promise<void> {
  try {
    const module =
      engine === 'monaco'
        ? await import('./RemoteTextEditorMonaco.vue')
        : await import('./RemoteTextEditorCodeMirror.vue')
    activeEngine.value = engine
    adapter.value = module.default as Component
  } catch (error) {
    if (engine === 'monaco') {
      console.warn('[qterm] monaco adapter failed to load, falling back to CodeMirror', error)
      fallbackNotice.value = true
      emit('engineFallback', 'codemirror')
      await loadEngine('codemirror')
      return
    }
    throw error
  }
}

const focusRegistry = useRemoteEditorFocus()
let unregister: (() => void) | null = null

onMounted(() => {
  void loadEngine(props.engine)

  unregister = focusRegistry?.register(() => emit('save')) ?? null
})

onBeforeUnmount(() => {
  unregister?.()
  unregister = null

  adapterRef.value?.dispose()
})

watch(
  () => props.engine,
  (engine) => {
    if (engine !== activeEngine.value) void loadEngine(engine)
  },
)

const adapterProps = computed(() => ({
  modelValue: props.modelValue,
  language: props.language,
  readonly: props.readonly,
  theme: props.theme,
}))

defineExpose<RemoteTextEditorExposed & { engine: typeof activeEngine; fallbackNotice: typeof fallbackNotice }>({
  focus() {
    adapterRef.value?.focus()
  },
  dispose() {
    adapterRef.value?.dispose()
  },

  engine: activeEngine,
  fallbackNotice,
})
</script>

<template>
  <div class="remote-text-editor" :data-engine="activeEngine">
    <component
      :is="adapter"
      v-if="adapter"
      ref="adapterRef"
      v-bind="adapterProps"
      @update:model-value="emit('update:modelValue', $event)"
      @dirty-change="emit('dirtyChange', $event)"
      @save="emit('save')"
      @ready="emit('ready')"
    />
  </div>
</template>

<style scoped>
.remote-text-editor {
  display: flex;
  width: 100%;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  border-radius: inherit;
}
</style>
