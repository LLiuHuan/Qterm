<script setup lang="ts">
import {
  ArrowLeftToLine,
  ArrowRightToLine,
  CopyPlus,
  ExternalLink,
  Layers2,
  ListX,
  Lock,
  LockOpen,
  Maximize2,
  PanelTopClose,
  Pencil,
  X,
} from '@lucide/vue'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuShortcut,
  ContextMenuTrigger,
} from '../common/overlay-controls'
import { computed } from 'vue'
import ShortcutKeycaps from '../../features/settings/ShortcutKeycaps.vue'
import { shortcutBindings, shortcutKeycapTokens } from '../../features/settings/shortcutStore'

import { isMacPlatform } from '@/lib/platform.ts'
import { useI18n } from '@/locales'

export type SessionMenuCloseScope = 'left' | 'right' | 'others' | 'all'
export type SessionMenuCloseOperation = 'tab' | SessionMenuCloseScope

const props = withDefaults(
  defineProps<{
    isSplit: boolean
    visible: boolean

    locked?: boolean

    sessionActions?: boolean

    canRename?: boolean

    canDuplicate?: boolean

    canLock?: boolean

    canCloseTab?: boolean

    canCloseScope?: boolean

    canOpenWindow?: boolean

    canCloseSession?: boolean

    open?: boolean
    canCloseLeft?: boolean
    canCloseRight?: boolean
    canCloseOthers?: boolean
  }>(),
  {
    sessionActions: false,
    canRename: false,
    canDuplicate: false,
    canLock: false,
    canCloseTab: false,
    canCloseScope: false,
    canOpenWindow: true,
    canCloseSession: true,
    canCloseLeft: false,
    canCloseRight: false,
    canCloseOthers: false,
  },
)

const emit = defineEmits<{
  single: []
  remove: []
  close: []
  openWindow: []

  rename: []

  duplicate: []

  toggleLock: []

  closeScope: [scope: SessionMenuCloseOperation]
  'update:open': [open: boolean]
}>()

const { t } = useI18n()

const closeTabTokens = computed(() => shortcutKeycapTokens(shortcutBindings.closeTab ?? '', isMacPlatform.value))

const caps = computed(() => ({
  rename: props.sessionActions || props.canRename,
  duplicate: props.sessionActions || props.canDuplicate,
  lock: props.sessionActions || props.canLock,
  closeTab: props.sessionActions || props.canCloseTab,
  closeScope: props.sessionActions || props.canCloseScope,
}))
</script>

<template>
  <ContextMenu :open="props.open" @update:open="(value) => emit('update:open', value)">
    <ContextMenuTrigger as-child><slot /></ContextMenuTrigger>
    <ContextMenuContent class="asset-context-menu min-w-44">
      <template v-if="caps.rename || caps.duplicate || caps.lock || caps.closeTab || caps.closeScope">
        <ContextMenuItem v-if="caps.rename" @select="emit('rename')"
          ><Pencil :size="14" />{{ t('shell.sessionMenu.rename') }}</ContextMenuItem
        >
        <ContextMenuItem v-if="caps.duplicate" @select="emit('duplicate')"
          ><CopyPlus :size="14" />{{ t('shell.sessionMenu.duplicate') }}</ContextMenuItem
        >
        <ContextMenuItem v-if="caps.lock" @select="emit('toggleLock')">
          <Lock v-if="locked" :size="14" />
          <LockOpen v-else :size="14" />
          {{ locked ? t('shell.sessionMenu.unlock') : t('shell.sessionMenu.lock') }}
        </ContextMenuItem>
        <ContextMenuSeparator v-if="caps.closeTab || caps.closeScope" />

        <ContextMenuItem v-if="caps.closeTab" @select="emit('close')"
          ><X :size="14" />{{ t('shell.sessionMenu.closeTab') }}
          <ContextMenuShortcut>
            <ShortcutKeycaps :tokens="closeTabTokens" :label="closeTabTokens.join(' + ')" /> </ContextMenuShortcut
        ></ContextMenuItem>
        <ContextMenuSeparator v-if="caps.closeTab && caps.closeScope" />

        <template v-if="caps.closeScope">
          <ContextMenuItem :disabled="!canCloseLeft" @select="emit('closeScope', 'left')"
            ><ArrowLeftToLine :size="14" />{{ t('shell.sessionMenu.closeLeft') }}</ContextMenuItem
          >
          <ContextMenuItem :disabled="!canCloseRight" @select="emit('closeScope', 'right')"
            ><ArrowRightToLine :size="14" />{{ t('shell.sessionMenu.closeRight') }}</ContextMenuItem
          >
          <ContextMenuItem :disabled="!canCloseOthers" @select="emit('closeScope', 'others')"
            ><ListX :size="14" />{{ t('shell.sessionMenu.closeOthers') }}</ContextMenuItem
          >
          <ContextMenuItem variant="destructive" @select="emit('closeScope', 'all')"
            ><Layers2 :size="14" />{{ t('shell.sessionMenu.closeAll') }}</ContextMenuItem
          >
        </template>
        <ContextMenuSeparator />
      </template>
      <template v-if="isSplit">
        <ContextMenuItem @select="emit('single')"
          ><Maximize2 :size="14" />{{ t('shell.sessionMenu.single') }}</ContextMenuItem
        >
        <ContextMenuItem v-if="visible" @select="emit('remove')"
          ><PanelTopClose :size="14" />{{ t('shell.sessionMenu.remove') }}</ContextMenuItem
        >
        <ContextMenuSeparator />
      </template>

      <ContextMenuItem v-if="canOpenWindow" @select="emit('openWindow')"
        ><ExternalLink :size="14" />{{ t('shell.sessionMenu.openWindow') }}</ContextMenuItem
      >
      <ContextMenuItem v-if="canCloseSession" variant="destructive" @select="emit('close')"
        ><X :size="14" />{{ t('shell.sessionMenu.close') }}</ContextMenuItem
      >
    </ContextMenuContent>
  </ContextMenu>
</template>
