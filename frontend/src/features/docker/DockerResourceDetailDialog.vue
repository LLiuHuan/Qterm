<script setup lang="ts" generic="T">
import { computed, onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import { QtermButton, QtermFormMessage } from '../../components/common'
import { QtermDialog, QtermDialogContent, QtermDialogHeader } from '../../components/common/dialog'
import { ScrollArea } from '../../components/common/control-primitives'
import { errorMessage } from '../../services/errors'
import { useI18n } from '@/locales'

const props = withDefaults(
  defineProps<{
    open: boolean
    title: string

    description?: string

    targetKey?: string

    load: () => Promise<T>
  }>(),
  { description: '', targetKey: '' },
)

const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()

const detail = shallowRef<T | null>(null)
const loading = ref(false)
const error = ref('')

let requestSequence = 0

async function runLoad(): Promise<void> {
  const version = ++requestSequence
  loading.value = true
  error.value = ''
  try {
    const next = await props.load()
    if (version !== requestSequence) return
    detail.value = next
  } catch (cause) {
    if (version !== requestSequence) return
    detail.value = null
    error.value = errorMessage(cause, t('shell.docker.actionFailed'))
  } finally {
    if (version === requestSequence) loading.value = false
  }
}

function reset(): void {
  requestSequence += 1
  detail.value = null
  error.value = ''
  loading.value = false
}

watch(
  () => [props.open, props.targetKey] as const,
  () => {
    if (!props.open) {
      reset()
      return
    }
    void runLoad()
  },
  { immediate: true },
)

onBeforeUnmount(reset)

const content = computed(() => detail.value as T)
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => !open && emit('close')">
    <QtermDialogContent
      v-if="props.open"
      size="lg"
      show-close-button
      close-on-escape
      class="docker-detail-dialog"
      @pointer-down-outside="(event) => event.preventDefault()"
    >
      <QtermDialogHeader :title="props.title" :description="props.description" :show-close="false" />

      <div class="docker-detail-body">
        <ScrollArea class="docker-detail-scroll" type="scroll" :scroll-hide-delay="600">
          <div class="flex flex-col gap-4 px-2 py-3">
            <span v-if="loading" class="block text-xs text-qt-muted">{{ t('shell.docker.detailLoading') }}</span>
            <div v-else-if="error" class="flex flex-col items-start gap-2">
              <QtermFormMessage variant="error">{{ error }}</QtermFormMessage>
              <QtermButton variant="outline" size="sm" type="button" @click="runLoad">
                {{ t('shell.docker.retry') }}
              </QtermButton>
            </div>
            <slot v-else-if="detail" :detail="content" />
          </div>
        </ScrollArea>
      </div>
    </QtermDialogContent>
  </QtermDialog>
</template>
