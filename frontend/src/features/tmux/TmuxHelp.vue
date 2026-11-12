<script setup lang="ts">
import { CircleHelp } from '@lucide/vue'
import { useI18n } from '../../locales'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '../../components/common/control-primitives'

const props = withDefaults(defineProps<{ prefix?: string }>(), { prefix: '' })

const { t } = useI18n()

function combo(key: string): string {
  return props.prefix ? `${props.prefix} ${key}` : key
}

const groups = [
  {
    title: 'shell.tmux.helpGroupSession',
    rows: [
      { command: 'tmux new -s <name>', text: 'shell.tmux.helpSessionNew' },
      { command: 'tmux ls', text: 'shell.tmux.helpSessionList' },
      { command: 'tmux attach -t <name>', text: 'shell.tmux.helpSessionAttach' },
      { key: 'd', text: 'shell.tmux.helpSessionDetach' },
      { key: '$', text: 'shell.tmux.helpSessionRename' },
      { key: 's', text: 'shell.tmux.helpSessionSwitch' },
    ],
  },
  {
    title: 'shell.tmux.helpGroupWindow',
    rows: [
      { key: 'c', text: 'shell.tmux.helpWindowNew' },
      { key: 'n', text: 'shell.tmux.helpWindowNext' },
      { key: 'p', text: 'shell.tmux.helpWindowPrev' },
      { key: '0-9', text: 'shell.tmux.helpWindowIndex' },
      { key: ',', text: 'shell.tmux.helpWindowRename' },
      { key: '&', text: 'shell.tmux.helpWindowClose' },
    ],
  },
  {
    title: 'shell.tmux.helpGroupPane',
    rows: [
      { key: '%', text: 'shell.tmux.helpPaneSplitV' },
      { key: '"', text: 'shell.tmux.helpPaneSplitH' },
      { key: '←↓↑→', text: 'shell.tmux.helpPaneFocus' },
      { key: 'z', text: 'shell.tmux.helpPaneZoom' },
      { key: 'Space', text: 'shell.tmux.helpPaneLayout' },
      { key: 'x', text: 'shell.tmux.helpPaneClose' },
    ],
  },
] as const
</script>

<template>
  <TooltipProvider :delay-duration="350">
    <Tooltip>
      <TooltipTrigger as-child>
        <button
          type="button"
          class="inline-flex size-5 shrink-0 items-center justify-center rounded text-qt-muted transition-colors hover:bg-qt-surface-strong hover:text-qt-text focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-qt-accent"
          :aria-label="t('shell.tmux.helpTitle')"
          data-tmux-help
        >
          <CircleHelp class="size-3.5" aria-hidden="true" />
        </button>
      </TooltipTrigger>

      <TooltipContent
        side="bottom"
        align="center"
        :collision-padding="12"
        class="max-w-[21rem] items-start whitespace-normal leading-5"
      >
        <div
          class="grid w-[min(20rem,calc(100vw-2rem))] max-h-[min(80vh,40rem)] gap-2 overflow-y-auto pr-1"
          data-tmux-help-body
        >
          <p class="m-0 text-xs font-medium text-qt-text">{{ t('shell.tmux.helpTitle') }}</p>
          <p class="m-0 text-xs text-qt-muted">
            {{ t('shell.tmux.helpPrefix', { prefix: props.prefix || '?' }) }}
          </p>

          <div class="grid grid-cols-[auto_1fr] items-baseline gap-x-1.5 gap-y-0.5">
            <template v-for="group in groups" :key="group.title">
              <p class="col-span-2 m-0 text-xs font-medium text-qt-muted">{{ t(group.title) }}</p>
              <template v-for="row in group.rows" :key="row.text">
                <span class="whitespace-nowrap font-mono text-[11px] text-qt-text">
                  {{ 'command' in row ? row.command : combo(row.key) }}
                </span>
                <span class="text-xs text-qt-muted">{{ t(row.text) }}</span>
              </template>
            </template>
          </div>
          <p class="m-0 text-xs text-qt-muted">{{ t('shell.tmux.helpScroll') }}</p>
        </div>
      </TooltipContent>
    </Tooltip>
  </TooltipProvider>
</template>
