<script setup lang="ts">
import { computed } from 'vue'
import {
  ArrowUp,
  ImagePlus,
  Paperclip,
  Bot,
  Check,
  ChevronDown,
  MessageCircle,
  ShieldCheck,
  Square,
  X,
} from '@lucide/vue'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupTextarea,
} from '../../components/common/ai'
import { useI18n } from '../../locales'
import { providerLogo } from './providerLogos'
import type { AIApprovalMode } from '../../services/aiApproval'
import type { AIProvider } from '../../services/aiProviders'

const props = defineProps<{
  modelValue: string
  mode: 'ask' | 'agent'
  approvalMode: AIApprovalMode
  provider: AIProvider | null
  providers: AIProvider[]
  busy: boolean
  contextCount?: number
  locked?: boolean
  disabled?: boolean
  placeholder?: string

  images?: string[]

  imagesEnabled?: boolean
}>()
const emit = defineEmits<{
  'update:modelValue': [value: string]
  'update:mode': [value: 'ask' | 'agent']
  approval: [value: AIApprovalMode]
  model: [providerId: string, model: string]
  submit: []
  stop: []
  context: []
  'add-images': [dataUrls: string[]]
  'remove-image': [index: number]
}>()
const { t } = useI18n()
const approvalModes: AIApprovalMode[] = ['strict', 'balanced', 'review', 'automatic']
const modeLabel = computed(() => t(`shell.aiChat.agent.${props.mode === 'agent' ? 'modeAgent' : 'modeAsk'}`))
const modelLabel = computed(() => props.provider?.model || t('shell.aiChat.noProvider'))
const providerLabel = computed(() =>
  props.provider ? `${props.provider.name || props.provider.baseUrl} · ${modelLabel.value}` : modelLabel.value,
)

const currentLogo = computed(() => providerLogo(props.provider))

function readImageFile(file: File): Promise<string | null> {
  return new Promise((resolve) => {
    const reader = new FileReader()
    reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : null)
    reader.onerror = () => resolve(null)
    reader.readAsDataURL(file)
  })
}

async function addImageFiles(files: File[]): Promise<void> {
  if (!props.imagesEnabled) return
  const images = files.filter((file) => file.type.startsWith('image/'))
  if (!images.length) return
  const dataUrls = (await Promise.all(images.map(readImageFile))).filter((url): url is string => Boolean(url))
  if (dataUrls.length) emit('add-images', dataUrls)
}

function pickImages(): void {
  if (!props.imagesEnabled || props.disabled || props.busy || props.locked) return
  const input = document.createElement('input')
  input.type = 'file'
  input.accept = 'image/png,image/jpeg,image/webp,image/gif'
  input.multiple = true
  input.addEventListener('change', () => {
    if (input.files) void addImageFiles(Array.from(input.files))
  })
  input.click()
}

function onPaste(event: ClipboardEvent): void {
  if (!props.imagesEnabled) return
  const files = Array.from(event.clipboardData?.files ?? []).filter((file) => file.type.startsWith('image/'))
  if (!files.length) return
  event.preventDefault()
  void addImageFiles(files)
}

function submit(): void {
  if (!props.busy && !props.locked && !props.disabled && props.modelValue.trim()) emit('submit')
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key !== 'Enter' || event.shiftKey || event.isComposing || event.keyCode === 229) return
  event.preventDefault()
  submit()
}
</script>

<template>
  <div class="@container/ai-composer w-full min-w-0" data-ai-composer-control>
    <InputGroup
      class="ai-composer-group overflow-hidden rounded-xl border-qt-border bg-qt-panel shadow-none has-disabled:opacity-100!"
    >
      <slot name="scope" />
      <div v-if="props.images?.length" class="flex flex-wrap gap-1.5 px-2 pt-2" data-ai-composer-images>
        <div
          v-for="(image, index) in props.images"
          :key="index"
          class="group relative size-14 overflow-hidden rounded-md border border-qt-border"
        >
          <img :src="image" alt="" class="size-full object-cover" />
          <button
            type="button"
            class="absolute end-0.5 top-0.5 rounded-full bg-qt-panel/90 p-0.5 text-qt-muted opacity-0 transition-opacity hover:text-qt-danger group-hover:opacity-100"
            :aria-label="t('shell.aiChat.composer.removeImage')"
            @click="emit('remove-image', index)"
          >
            <X class="size-3" aria-hidden="true" />
          </button>
        </div>
      </div>
      <InputGroupTextarea
        :model-value="props.modelValue"
        :placeholder="props.placeholder || t('shell.aiChat.placeholder')"
        :aria-label="props.placeholder || t('shell.aiChat.placeholder')"
        :disabled="props.disabled || props.busy"
        class="max-h-48 min-h-20 px-3 pt-3 pb-2 text-xs leading-5 placeholder:text-qt-muted/80"
        data-ai-composer-input
        @update:model-value="emit('update:modelValue', String($event))"
        @keydown="onKeydown"
        @paste="onPaste"
      />
      <InputGroupAddon align="block-end" class="gap-1 px-2 py-2">
        <InputGroupButton
          size="icon-sm"
          class="size-7 shrink-0 rounded-md text-qt-muted"
          :class="props.contextCount ? 'text-qt-accent' : ''"
          :disabled="props.busy || props.locked || props.disabled"
          :aria-label="t('shell.aiChat.contextSources.title')"
          :title="t('shell.aiChat.contextSources.title')"
          data-ai-context-open
          @click="emit('context')"
        >
          <Paperclip class="size-3.5" aria-hidden="true" />
          <span v-if="props.contextCount" class="text-[10px] tabular-nums">{{ props.contextCount }}</span>
        </InputGroupButton>
        <InputGroupButton
          v-if="props.imagesEnabled"
          size="icon-sm"
          class="size-7 shrink-0 rounded-md text-qt-muted"
          :class="props.images?.length ? 'text-qt-accent' : ''"
          :disabled="props.busy || props.locked || props.disabled"
          :aria-label="t('shell.aiChat.composer.attachImage')"
          :title="t('shell.aiChat.composer.attachImage')"
          data-ai-attach-image
          @click="pickImages"
        >
          <ImagePlus class="size-3.5" aria-hidden="true" />
          <span v-if="props.images?.length" class="text-[10px] tabular-nums">{{ props.images.length }}</span>
        </InputGroupButton>
        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <InputGroupButton
              size="xs"
              class="h-7 shrink-0 gap-1 rounded-md px-1.5 text-xs text-qt-muted"
              :aria-label="`${t('shell.aiChat.agent.modeLabel')}: ${modeLabel}`"
              :title="`${t('shell.aiChat.agent.modeLabel')}: ${modeLabel}`"
              data-ai-mode-switch
            >
              <Bot v-if="props.mode === 'agent'" class="size-3.5 shrink-0" aria-hidden="true" />
              <MessageCircle v-else class="size-3.5 shrink-0" aria-hidden="true" />

              <span class="hidden @[17rem]/ai-composer:inline">{{ modeLabel }}</span>
              <ChevronDown class="hidden size-3 shrink-0 @[17rem]/ai-composer:block" aria-hidden="true" />
            </InputGroupButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent side="top" align="start" class="max-w-[calc(100vw-1rem)]" :collision-padding="8">
            <DropdownMenuItem data-ai-mode-option="ask" @select="emit('update:mode', 'ask')">
              <MessageCircle class="size-3.5" aria-hidden="true" />
              {{ t('shell.aiChat.agent.modeAsk') }}
              <Check v-if="props.mode === 'ask'" class="ms-auto size-3.5" aria-hidden="true" />
            </DropdownMenuItem>
            <DropdownMenuItem data-ai-mode-option="agent" @select="emit('update:mode', 'agent')">
              <Bot class="size-3.5" aria-hidden="true" />
              {{ t('shell.aiChat.agent.modeAgent') }}
              <Check v-if="props.mode === 'agent'" class="ms-auto size-3.5" aria-hidden="true" />
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <InputGroupButton
              size="xs"
              class="h-7 shrink-0 gap-1 rounded-md px-1.5 text-xs text-qt-muted"
              :aria-label="`${t('shell.aiChat.approvalMode')}: ${t(`shell.aiChat.mode.${props.approvalMode}`)}`"
              :title="t(`shell.aiChat.mode.${props.approvalMode}`)"
              :disabled="props.busy || props.locked"
              data-ai-approval-mode
            >
              <ShieldCheck class="size-3.5 shrink-0" aria-hidden="true" />

              <span class="hidden max-w-24 truncate @[19rem]/ai-composer:inline">{{
                t(`shell.aiChat.mode.${props.approvalMode}`)
              }}</span>
            </InputGroupButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            side="top"
            align="start"
            class="max-h-80 w-72 max-w-[calc(100vw-1rem)] overflow-auto"
            :collision-padding="8"
          >
            <DropdownMenuLabel>{{ t('shell.aiChat.approvalMode') }}</DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              v-for="approval in approvalModes"
              :key="approval"
              class="flex-col items-start gap-1 py-2"
              :data-ai-mode-option="approval"
              @select="emit('approval', approval)"
            >
              <span class="flex w-full items-center gap-2" :class="approval === 'automatic' ? 'text-qt-warning' : ''">
                {{ t(`shell.aiChat.mode.${approval}`) }}
                <Check v-if="props.approvalMode === approval" class="ms-auto size-3.5" aria-hidden="true" />
              </span>
              <span class="text-xs leading-4 text-qt-muted">{{ t(`shell.aiChat.modeHint.${approval}`) }}</span>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <InputGroupButton
              size="xs"
              class="ms-auto h-7 max-w-[65%] min-w-0 shrink gap-1 rounded-md px-1.5 text-xs"
              :aria-label="providerLabel"
              :title="providerLabel"
              :disabled="props.busy || props.locked || !props.providers.length"
              data-ai-provider-switch
            >
              <component :is="currentLogo" v-if="currentLogo" :size="14" class="shrink-0" aria-hidden="true" />
              <span class="min-w-0 truncate" data-ai-model-label>{{ modelLabel }}</span>
              <ChevronDown class="size-3 shrink-0 text-qt-muted" aria-hidden="true" />
            </InputGroupButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            side="top"
            align="end"
            class="max-h-80 w-72 max-w-[calc(100vw-1rem)] overflow-auto"
            :collision-padding="8"
          >
            <template v-for="(entry, entryIndex) in props.providers" :key="entry.id">
              <DropdownMenuSeparator v-if="entryIndex > 0" />
              <DropdownMenuLabel class="flex items-center gap-2 truncate">
                <component
                  :is="providerLogo(entry)"
                  v-if="providerLogo(entry)"
                  :size="14"
                  class="shrink-0"
                  aria-hidden="true"
                />
                <span class="truncate">{{ entry.name || entry.baseUrl }}</span>
              </DropdownMenuLabel>
              <DropdownMenuItem
                v-for="model in Array.from(new Set([entry.model, ...entry.models].filter(Boolean)))"
                :key="`${entry.id}:${model}`"
                :data-ai-model-option="model"
                @select="emit('model', entry.id, model)"
              >
                <span class="min-w-0 truncate" :title="model">{{ model }}</span>
                <Check
                  v-if="entry.id === props.provider?.id && model === props.provider?.model"
                  class="ms-auto size-3.5 shrink-0"
                  aria-hidden="true"
                />
              </DropdownMenuItem>
            </template>
          </DropdownMenuContent>
        </DropdownMenu>
        <InputGroupButton
          v-if="props.busy"
          size="icon-sm"
          variant="outline"
          class="size-7 shrink-0 rounded-full text-xs"
          :aria-label="t('shell.aiChat.stop')"
          :title="t('shell.aiChat.stop')"
          data-ai-chat-stop
          @click="emit('stop')"
        >
          <Square class="size-3.5" aria-hidden="true" />
        </InputGroupButton>
        <InputGroupButton
          v-else
          size="icon-sm"
          variant="default"
          class="size-7 shrink-0 rounded-full text-xs"
          :aria-label="t('shell.aiChat.send')"
          :title="t('shell.aiChat.send')"
          :disabled="props.disabled || props.locked || !props.modelValue.trim()"
          data-ai-chat-send
          @click="submit"
        >
          <ArrowUp class="size-3.5" aria-hidden="true" />
        </InputGroupButton>
      </InputGroupAddon>
    </InputGroup>
  </div>
</template>

<style scoped>
[data-ai-composer-control] .ai-composer-group {
  background-color: var(--qt-app) !important;

  transition: none !important;
}

[data-ai-composer-control] .ai-composer-group > * {
  background-color: transparent !important;
}

[data-ai-composer-control] .ai-composer-group:focus-within {
  border-color: var(--qt-focus) !important;
  box-shadow: 0 0 0 1px color-mix(in srgb, var(--qt-focus) 25%, transparent) !important;
}
.ai-composer-group :deep(textarea),
.ai-composer-group :deep(textarea:is(:focus, :focus-visible)) {
  border: 0 !important;
  background: transparent !important;
  outline: none;
  box-shadow: none !important;
}
</style>
