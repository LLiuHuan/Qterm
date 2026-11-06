<script lang="ts" setup>
import type { ToasterProps } from 'vue-sonner'

import { CircleCheckIcon, InfoIcon, Loader2Icon, OctagonXIcon, TriangleAlertIcon, XIcon } from '@lucide/vue'
import { reactiveOmit } from '@vueuse/core'
import { Toaster as Sonner } from 'vue-sonner'
import { cn } from '@/lib/utils'
import 'vue-sonner/style.css'

const props = defineProps<ToasterProps>()
const delegatedProps = reactiveOmit(props, 'class', 'toastOptions')
</script>

<template>
  <Sonner
    :class="cn('toaster group', props.class)"
    :style="{
      '--normal-bg': 'var(--popover)',
      '--normal-text': 'var(--popover-foreground)',
      '--normal-border': 'var(--border)',
      '--border-radius': 'var(--radius)',
      '--gray2': 'hsl(var(--popover) / 0.9)',
      '--gray3': 'var(--border)',
      '--gray4': 'var(--border)',
      '--gray5': 'var(--border)',
      '--gray12': 'var(--popover-foreground)',
    }"
    :toast-options="
      props.toastOptions ?? {
        classes: {
          toast: 'rounded-2xl !text-sm',
          // 标题/描述都要限高：后端错误文本可能带一大段诊断输出（例如 RDP helper 的 stderr 刷屏），
          // 不限高时提示框会被铺满整屏、把界面盖住（真机反馈）。这里最多各占几行，
          // 完整内容仍在对应功能自己的诊断入口里（如 RDP 右侧竖栏的「日志」弹窗）。
          title: 'line-clamp-4 break-words',
          description: 'line-clamp-6 break-words whitespace-pre-wrap',
          success: '!border-qt-success !text-qt-success',
          info: '!border-qt-info !text-qt-info',
          warning: '!border-qt-warning !text-qt-warning',
          error: '!border-qt-danger !text-qt-danger',
          loading: '!border-qt-accent !text-qt-accent',
        },
      }
    "
    v-bind="delegatedProps"
  >
    <template #success-icon>
      <CircleCheckIcon class="size-4 text-qt-success" />
    </template>
    <template #info-icon>
      <InfoIcon class="size-4 text-qt-info" />
    </template>
    <template #warning-icon>
      <TriangleAlertIcon class="size-4 text-qt-warning" />
    </template>
    <template #error-icon>
      <OctagonXIcon class="size-4 text-qt-danger" />
    </template>
    <template #loading-icon>
      <div>
        <Loader2Icon class="size-4 animate-spin text-qt-accent" />
      </div>
    </template>
    <template #close-icon>
      <XIcon class="size-4" />
    </template>
  </Sonner>
</template>
