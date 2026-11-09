<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Loader2 } from '@lucide/vue'
import { toast } from 'vue-sonner'
import { QtermButton, QtermFormMessage } from '../../components/common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '../../components/common/dialog'
import { translateErrorKey } from '../../services/errors'
import { readRemoteImage, type RemoteImageSnapshot } from '../../services/preview'
import { useI18n } from '@/locales'
import { openWithLocalProgram } from '../remoteedit/useRemoteEdits'
import { isPreviewImageMime } from './previewableImage'

const props = withDefaults(
  defineProps<{
    sessionId: string
    remotePath: string
    name: string

    sizeText?: string

    modTime?: string
  }>(),
  { sizeText: '', modTime: '' },
)

const emit = defineEmits<{ close: [] }>()

const { t, locale } = useI18n()

interface PreviewFailure {
  errorKey: string
  localOpen: boolean
}

const CONTENT_FAILURE_KEYS = new Set([
  'remote.preview_unsupported',
  'remote.preview_file_too_large',
  'remote.preview_decode_failed',
])

function isContentFailure(errorKey: string): boolean {
  return CONTENT_FAILURE_KEYS.has(errorKey)
}

const loading = ref(true)
const failure = ref<PreviewFailure | null>(null)
const snapshot = ref<RemoteImageSnapshot | null>(null)

const imageUrl = ref('')
const pixelWidth = ref(0)
const pixelHeight = ref(0)

const fitted = ref(true)
const openingLocal = ref(false)

const ZOOM_MIN = 0.1
const ZOOM_MAX = 8
const ZOOM_STEP = 1.15
const zoom = ref(1)
const stageRef = ref<HTMLElement | null>(null)

const stageBox = ref({ width: 0, height: 0 })
let stageObserver: ResizeObserver | undefined

function measureStage(): void {
  const stage = stageRef.value
  if (!stage) return
  stageBox.value = { width: stage.clientWidth, height: stage.clientHeight }
}

const imageStyle = computed(() => {
  if (!pixelWidth.value || !pixelHeight.value) return undefined
  const box = stageBox.value

  if (!box.width || !box.height) return undefined
  const fitScale = Math.min(box.width / pixelWidth.value, box.height / pixelHeight.value)
  const scale = (fitted.value ? fitScale : 1) * zoom.value
  return {
    width: `${Math.max(1, Math.round(pixelWidth.value * scale))}px`,
    height: `${Math.max(1, Math.round(pixelHeight.value * scale))}px`,
  }
})

function clampZoom(value: number): number {
  return Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, value))
}

function onWheelZoom(event: WheelEvent): void {
  if (!imageUrl.value || !pixelWidth.value) return
  const stage = stageRef.value
  if (!stage) return
  event.preventDefault()
  const next = clampZoom(zoom.value * (event.deltaY > 0 ? 1 / ZOOM_STEP : ZOOM_STEP))
  if (next === zoom.value) return
  const rect = stage.getBoundingClientRect()
  const offsetX = event.clientX - rect.left + stage.scrollLeft
  const offsetY = event.clientY - rect.top + stage.scrollTop
  const ratio = next / zoom.value
  zoom.value = next

  void nextTick(() => {
    stage.scrollLeft = offsetX * ratio - (event.clientX - rect.left)
    stage.scrollTop = offsetY * ratio - (event.clientY - rect.top)
  })
}

const revokedUrls = new Set<string>()

function revokeOnce(url: string): void {
  if (!url || revokedUrls.has(url)) return
  revokedUrls.add(url)
  URL.revokeObjectURL(url)
}

watch(
  imageUrl,
  (_next, previous) => {
    if (previous) revokeOnce(previous)
  },
  { flush: 'sync' },
)

function releaseImageUrl(): void {
  imageUrl.value = ''
}

onBeforeUnmount(releaseImageUrl)

function decodeBase64Bytes(base64: string): Uint8Array {
  const binary = atob(base64)
  const bytes = new Uint8Array(binary.length)
  for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index)
  return bytes
}

function markDecodeFailure(): void {
  releaseImageUrl()
  failure.value = { errorKey: 'remote.preview_decode_failed', localOpen: true }
}

function buildImageUrl(content: RemoteImageSnapshot): boolean {
  const mimeType = content.mimeType.trim().toLowerCase()
  if (!isPreviewImageMime(mimeType)) {
    failure.value = { errorKey: 'remote.preview_unsupported', localOpen: true }
    return false
  }
  if (!content.contentBase64) {
    markDecodeFailure()
    return false
  }
  let bytes: Uint8Array
  try {
    bytes = decodeBase64Bytes(content.contentBase64)
  } catch {
    markDecodeFailure()
    return false
  }
  if (bytes.byteLength === 0) {
    markDecodeFailure()
    return false
  }

  const buffer = bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength) as ArrayBuffer
  imageUrl.value = URL.createObjectURL(new Blob([buffer], { type: mimeType }))
  return true
}

async function load(): Promise<void> {
  loading.value = true
  failure.value = null
  snapshot.value = null
  pixelWidth.value = 0
  pixelHeight.value = 0
  fitted.value = true
  zoom.value = 1
  releaseImageUrl()

  const result = await readRemoteImage(props.sessionId, props.remotePath)
  loading.value = false
  if (!result.ok || !result.snapshot) {
    const errorKey = result.errorKey || 'error.unknown'
    failure.value = { errorKey, localOpen: isContentFailure(errorKey) }
    return
  }

  const content = result.snapshot
  snapshot.value = content

  if (content.truncated) {
    failure.value = {
      errorKey: content.mimeType ? 'remote.preview_file_too_large' : 'remote.preview_unsupported',
      localOpen: true,
    }
    return
  }
  buildImageUrl(content)
}

onMounted(() => {
  measureStage()

  if (typeof ResizeObserver !== 'undefined' && stageRef.value) {
    stageObserver = new ResizeObserver(() => measureStage())
    stageObserver.observe(stageRef.value)
  }
  void load()
})

onBeforeUnmount(() => {
  stageObserver?.disconnect()
  stageObserver = undefined
})

watch(
  () => `${props.sessionId}\u0000${props.remotePath}`,
  () => void load(),
)

function onImageLoad(event: Event): void {
  const image = event.target as HTMLImageElement | null
  const width = image?.naturalWidth ?? 0
  const height = image?.naturalHeight ?? 0
  if (!width || !height) {
    markDecodeFailure()
    return
  }
  pixelWidth.value = width
  pixelHeight.value = height

  zoom.value = 1
  measureStage()
}

function toggleFit(): void {
  if (!imageUrl.value) return

  if (suppressClick.value) return
  fitted.value = !fitted.value
  zoom.value = 1
}

const panning = ref(false)
const suppressClick = ref(false)
let panStart: { x: number; y: number; left: number; top: number; moved: boolean; pointerId: number } | null = null

function onStagePointerDown(event: PointerEvent): void {
  const stage = stageRef.value
  if (!stage || !imageUrl.value || event.button !== 0 || event.isPrimary === false) return
  panStart = {
    x: event.clientX,
    y: event.clientY,
    left: stage.scrollLeft,
    top: stage.scrollTop,
    moved: false,
    pointerId: event.pointerId,
  }
  stage.setPointerCapture?.(event.pointerId)
  panning.value = true
}

function onStagePointerMove(event: PointerEvent): void {
  const stage = stageRef.value
  if (!stage || !panStart || event.pointerId !== panStart.pointerId) return
  const dx = event.clientX - panStart.x
  const dy = event.clientY - panStart.y
  if (!panStart.moved && Math.hypot(dx, dy) < 3) return
  panStart.moved = true

  const overflows = stage.scrollWidth > stage.clientWidth + 1 || stage.scrollHeight > stage.clientHeight + 1
  if (!overflows) return
  stage.scrollLeft = panStart.left - dx
  stage.scrollTop = panStart.top - dy
}

function endStagePan(event: PointerEvent): void {
  const stage = stageRef.value
  if (!stage || !panStart || event.pointerId !== panStart.pointerId) return
  stage.releasePointerCapture?.(event.pointerId)
  if (panStart.moved) {
    suppressClick.value = true
    window.setTimeout(() => (suppressClick.value = false), 0)
  }
  panStart = null
  panning.value = false
}

async function openLocally(): Promise<void> {
  if (openingLocal.value) return
  openingLocal.value = true
  const result = await openWithLocalProgram(props.sessionId, props.remotePath)
  openingLocal.value = false
  if (result.ok) {
    toast.success(t('remote.ui.toast.openedLocal', { path: props.name }))
    emit('close')
    return
  }
  toast.error(t('remote.ui.toast.openFailed', { reason: translateErrorKey(result.errorKey, t('error.unknown')) }))
}

const failureMessage = computed(() =>
  failure.value ? translateErrorKey(failure.value.errorKey, t('error.unknown')) : '',
)
const canRetry = computed(() => Boolean(failure.value && !failure.value.localOpen))
const byteSizeText = computed(
  () => props.sizeText || (snapshot.value ? `${snapshot.value.size} ${t('sftp.ui.bytes')}` : ''),
)
const pixelSizeText = computed(() =>
  pixelWidth.value && pixelHeight.value ? `${pixelWidth.value} × ${pixelHeight.value}` : '',
)

function formatModTime(value: string): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleString(locale.value)
}

const modTimeText = computed(() => formatModTime(snapshot.value?.modTime || props.modTime))
</script>

<template>
  <QtermDialog :open="true" @update:open="(open) => !open && emit('close')">
    <QtermDialogContent
      class="image-preview-dialog z-60 grid-cols-1"
      overlay-class="z-60"
      size="3xl"
      :close-on-escape="true"
    >
      <QtermDialogHeader :title="name" :description="remotePath" @close="emit('close')" />
      <div class="dialog-body flex-col">
        <div
          ref="stageRef"
          class="relative flex h-[58vh] min-h-0 overflow-auto rounded-md border-d bg-transparent"
          data-testid="image-preview-stage"
          :class="panning ? 'cursor-grabbing' : 'cursor-grab'"
          @wheel="onWheelZoom"
          @pointerdown="onStagePointerDown"
          @pointermove="onStagePointerMove"
          @pointerup="endStagePan"
          @pointercancel="endStagePan"
        >
          <img
            v-if="imageUrl"
            :src="imageUrl"
            :alt="t('preview.ui.imageAlt', { name })"
            :title="t('preview.ui.toggleFit')"
            class="m-auto select-none"
            :class="[
              // 有显式像素尺寸时不能再叠 `max-*`：那会把放大后的盒子按舞台高度截断、比例失真。
              // 还没量到尺寸（对话框入场、单测里的 jsdom）就退回 CSS 的 contain / 原始像素。
              imageStyle
                ? 'max-h-none max-w-none'
                : fitted
                  ? 'max-h-full max-w-full object-contain'
                  : 'max-h-none max-w-none',
              fitted ? 'cursor-zoom-in' : 'cursor-zoom-out',
            ]"
            :style="imageStyle"
            :draggable="false"
            :data-fit="fitted ? 'fit' : 'actual'"
            :data-zoom="zoom"
            data-testid="image-preview-image"
            @load="onImageLoad"
            @error="markDecodeFailure"
            @click="toggleFit"
          />

          <div v-if="loading" class="absolute inset-0 flex-cc" role="status" data-testid="image-preview-loading">
            <div class="flex items-center gap-2 text-qt-muted">
              <Loader2 :size="18" class="spin" aria-hidden="true" />
              <span class="text-xs">{{ t('preview.ui.loading') }}</span>
            </div>
          </div>

          <div
            v-else-if="failure"
            class="absolute inset-0 flex flex-col items-center justify-center gap-3 px-6 text-center"
            role="alert"
            data-testid="image-preview-error"
          >
            <QtermFormMessage variant="error" :announce="false" class="max-w-[32rem]">{{
              failureMessage
            }}</QtermFormMessage>
            <div class="flex flex-wrap justify-center gap-2">
              <QtermButton v-if="canRetry" variant="outline" data-testid="image-preview-retry" @click="load">
                {{ t('sftp.ui.retry') }}
              </QtermButton>
              <QtermButton v-if="failure.localOpen" data-testid="image-preview-error-open-local" @click="openLocally">
                {{ t('remote.ui.openWithLocal') }}
              </QtermButton>
            </div>
          </div>
        </div>
      </div>

      <QtermDialogFooter class="flex-wrap gap-x-3 gap-y-1 text-xs">
        <span v-if="byteSizeText" class="text-qt-muted" data-testid="image-preview-size">{{ byteSizeText }}</span>
        <span
          v-if="pixelSizeText"
          class="text-qt-muted"
          :title="t('preview.ui.pixelSize', { width: pixelWidth, height: pixelHeight })"
          data-testid="image-preview-pixels"
          >{{ pixelSizeText }}</span
        >
        <span v-if="modTimeText" class="text-qt-muted" data-testid="image-preview-modtime">{{
          t('preview.ui.modifiedAt', { time: modTimeText })
        }}</span>

        <span v-if="imageUrl && zoom !== 1" class="text-qt-muted" data-testid="image-preview-zoom"
          >{{ Math.round(zoom * 100) }}%</span
        >

        <span v-if="imageUrl" class="text-qt-muted" data-testid="image-preview-fit">{{
          t(fitted ? 'preview.ui.fitWindow' : 'preview.ui.actualSize')
        }}</span>

        <QtermButton
          v-if="!failure"
          class="ml-auto"
          variant="outline"
          :disabled="openingLocal"
          :title="t('preview.ui.openWithLocalHint')"
          data-testid="image-preview-open-local"
          @click="openLocally"
        >
          {{ t('remote.ui.openWithLocal') }}
        </QtermButton>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>
