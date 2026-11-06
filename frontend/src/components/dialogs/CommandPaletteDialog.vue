<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { Search } from '@lucide/vue'
import { QtermInput } from '@/components/common'
import { ScrollArea } from '@/components/common/control-primitives'
import { QtermDialog, QtermDialogContent } from '@/components/common/dialog'
import { useI18n } from '@/locales'

export interface PaletteItem {
  id: string

  label: string

  hint?: string

  group: string
}

const props = withDefaults(
  defineProps<{
    open: boolean
    items: PaletteItem[]
  }>(),
  {},
)

const emit = defineEmits<{
  'update:open': [open: boolean]
  select: [item: PaletteItem]
}>()

const { t } = useI18n()
const query = ref('')
const activeIndex = ref(0)
const panelRef = ref<HTMLElement | null>(null)

const filtered = computed(() => {
  const keyword = query.value.trim().toLocaleLowerCase()
  if (!keyword) return props.items
  return props.items.filter((item) =>
    `${item.label} ${item.hint ?? ''} ${item.group}`.toLocaleLowerCase().includes(keyword),
  )
})

const groups = computed(() => {
  const buckets: Array<{ name: string; items: PaletteItem[] }> = []
  for (const item of filtered.value) {
    const bucket = buckets.find((entry) => entry.name === item.group)
    if (bucket) bucket.items.push(item)
    else buckets.push({ name: item.group, items: [item] })
  }
  return buckets
})

const flat = computed(() => groups.value.flatMap((group) => group.items))

watch(
  () => props.open,
  (open) => {
    if (!open) return
    query.value = ''
    activeIndex.value = 0

    void nextTick(() => panelRef.value?.querySelector<HTMLInputElement>('input')?.focus())
  },
)

watch(query, () => {
  activeIndex.value = 0
})

const listRef = ref<HTMLElement | null>(null)

watch(activeIndex, () => {
  void nextTick(() => {
    const row = listRef.value?.querySelector<HTMLElement>('[data-palette-active="true"]')
    row?.scrollIntoView({ block: 'nearest' })
  })
})

function move(step: number): void {
  const total = flat.value.length
  if (!total) return

  activeIndex.value = Math.min(total - 1, Math.max(0, activeIndex.value + step))
}

function choose(index = activeIndex.value): void {
  const item = flat.value[index]
  if (!item) return
  emit('select', item)
  emit('update:open', false)
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    move(1)
    return
  }
  if (event.key === 'ArrowUp') {
    event.preventDefault()
    move(-1)
    return
  }
  if (event.key === 'Enter') {
    event.preventDefault()
    choose()
  }
}
</script>

<template>
  <QtermDialog :open="props.open" @update:open="(open) => emit('update:open', open)">
    <QtermDialogContent v-if="props.open" size="md" close-on-escape data-command-palette>
      <div ref="panelRef" class="flex flex-col gap-2 p-3">
        <label class="relative flex min-w-0 items-center pr-2 text-qt-muted">
          <Search :size="16" class="pointer-events-none absolute left-2.5 z-1" aria-hidden="true" />
          <QtermInput
            v-model="query"
            class="qterm-reference-input bg-qt-panel pl-8"
            :placeholder="t('shell.palette.placeholder')"
            :aria-label="t('shell.palette.placeholder')"
            autocomplete="off"
            @keydown="onKeydown"
          />
        </label>

        <p v-if="!flat.length" class="panel-empty-note m-0 px-1 py-6 text-center text-xs text-qt-muted">
          {{ t('shell.palette.empty') }}
        </p>

        <div v-else ref="listRef" class="h-[52vh]">
          <ScrollArea type="hover" :scroll-hide-delay="600" class="h-full">
            <div class="pr-2">
              <section v-for="group in groups" :key="group.name" class="flex flex-col">
                <h4 class="m-0 px-1 pt-2 pb-1 text-[11px] font-normal text-qt-muted">{{ group.name }}</h4>
                <button
                  v-for="item in group.items"
                  :key="item.id"
                  type="button"
                  class="flex min-w-0 items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs transition-colors"
                  :class="
                    flat[activeIndex]?.id === item.id
                      ? 'bg-[color-mix(in_srgb,var(--qt-accent)_16%,transparent)] text-qt-text shadow-[inset_2px_0_0_var(--qt-accent)]'
                      : 'text-qt-text hover:bg-qt-hover'
                  "
                  :data-palette-item="item.id"
                  :data-palette-active="flat[activeIndex]?.id === item.id ? 'true' : undefined"
                  @click="choose(flat.findIndex((entry) => entry.id === item.id))"
                  @mouseenter="activeIndex = flat.findIndex((entry) => entry.id === item.id)"
                >
                  <span class="min-w-0 flex-1 truncate">{{ item.label }}</span>
                  <span v-if="item.hint" class="shrink-0 truncate text-[11px] text-qt-muted">{{ item.hint }}</span>
                </button>
              </section>
            </div>
          </ScrollArea>
        </div>

        <p class="m-0 px-1 text-[11px] text-qt-muted">{{ t('shell.palette.hint') }}</p>
      </div>
    </QtermDialogContent>
  </QtermDialog>
</template>
