import { computed, ref, toValue, watch, type MaybeRefOrGetter } from 'vue'
import { formatDateTime } from '../../lib/datetime'
import type { SftpBrowserEntry } from './SftpBrowserPane.vue'

type SortField = 'name' | 'modTime' | 'size'

export interface UseSftpBrowserListOptions {
  entries: () => SftpBrowserEntry[]

  selectedKeys: () => string[]

  locale: MaybeRefOrGetter<string>
}

const VIRTUALIZATION_THRESHOLD = 80
const VIRTUAL_ROW_HEIGHT = 44
const VIRTUAL_OVERSCAN = 8

const VIRTUAL_LIST_VERTICAL_PADDING = 8

export function useSftpBrowserList(options: UseSftpBrowserListOptions) {
  const sortField = ref<SortField>('name')
  const sortAscending = ref(true)
  const entryNameCollator = computed(() => new Intl.Collator(toValue(options.locale)))
  const selectedKeySet = computed(() => new Set(options.selectedKeys()))

  const scrollHost = ref<HTMLElement | null>(null)
  const scrollTop = ref(0)
  const viewportHeight = ref(0)
  let scrollViewport: HTMLElement | null = null
  let viewportResizeObserver: ResizeObserver | undefined

  const filterOpen = ref(false)
  const filterQuery = ref('')
  const entries = computed(() => options.entries())

  const filterMatchCount = computed(() => {
    const query = filterQuery.value.trim().toLocaleLowerCase()
    return query
      ? entries.value.filter((entry) => entry.name.toLocaleLowerCase().includes(query)).length
      : entries.value.length
  })

  const filteredEntries = computed(() => {
    const query = filterQuery.value.trim().toLocaleLowerCase()
    if (!query) return entries.value
    return entries.value.filter((entry) => entry.name.toLocaleLowerCase().includes(query))
  })

  function clearFilter(): void {
    filterQuery.value = ''
  }

  watch(filterOpen, (open) => {
    if (open) filterQuery.value = ''
  })

  const displayEntries = computed<SftpBrowserEntry[]>(() => {
    const direction = sortAscending.value ? 1 : -1
    const field = sortField.value
    const collator = entryNameCollator.value
    const currentEntries = filteredEntries.value

    const entryTimes =
      field === 'modTime' ? new Map(currentEntries.map((entry) => [entry, Date.parse(entry.modTime) || 0])) : null

    function comparePrimary(left: SftpBrowserEntry, right: SftpBrowserEntry): number {
      if (field === 'size') return (left.size - right.size) * direction
      if (field === 'modTime') {
        const leftTime = entryTimes?.get(left) ?? 0
        const rightTime = entryTimes?.get(right) ?? 0
        return (leftTime - rightTime) * direction
      }
      return collator.compare(left.name ?? '', right.name ?? '') * direction
    }

    return [...currentEntries].sort((left, right) => {
      const primary = comparePrimary(left, right)
      if (primary !== 0) return primary

      const leftDirectory = left.type === 'directory'
      const rightDirectory = right.type === 'directory'
      if (leftDirectory !== rightDirectory) return leftDirectory ? -1 : 1

      const byName = collator.compare(left.name ?? '', right.name ?? '')
      if (byName !== 0) return byName

      return left.key < right.key ? -1 : left.key > right.key ? 1 : 0
    })
  })

  const isVirtualized = computed(() => displayEntries.value.length > VIRTUALIZATION_THRESHOLD)
  const virtualWindow = computed(() => {
    if (!isVirtualized.value) return { start: 0, end: displayEntries.value.length }
    const visibleHeight = viewportHeight.value || 420
    const start = Math.max(0, Math.floor(scrollTop.value / VIRTUAL_ROW_HEIGHT) - VIRTUAL_OVERSCAN)
    const visibleCount = Math.ceil(visibleHeight / VIRTUAL_ROW_HEIGHT) + VIRTUAL_OVERSCAN * 2
    return { start, end: Math.min(displayEntries.value.length, start + visibleCount) }
  })
  const virtualEntries = computed(() => displayEntries.value.slice(virtualWindow.value.start, virtualWindow.value.end))
  const virtualOffset = computed(() => virtualWindow.value.start * VIRTUAL_ROW_HEIGHT)
  const virtualContentHeight = computed(() => displayEntries.value.length * VIRTUAL_ROW_HEIGHT)

  function updateScrollMetrics(): void {
    if (!scrollViewport) return
    scrollTop.value = scrollViewport.scrollTop
    viewportHeight.value = scrollViewport.clientHeight
  }

  function bindScrollViewport(): void {
    scrollViewport?.removeEventListener('scroll', updateScrollMetrics)
    viewportResizeObserver?.disconnect()
    scrollViewport = scrollHost.value?.querySelector<HTMLElement>("[data-slot='scroll-area-viewport']") ?? null
    scrollViewport?.addEventListener('scroll', updateScrollMetrics, { passive: true })

    if (scrollViewport && typeof ResizeObserver !== 'undefined') {
      viewportResizeObserver = new ResizeObserver(updateScrollMetrics)
      viewportResizeObserver.observe(scrollViewport)
    }
    updateScrollMetrics()
  }

  function resetVirtualScroll(): void {
    scrollTop.value = 0
    if (scrollViewport) scrollViewport.scrollTop = 0
    updateScrollMetrics()
  }

  function disposeScrollViewport(): void {
    viewportResizeObserver?.disconnect()
    viewportResizeObserver = undefined
    scrollViewport?.removeEventListener('scroll', updateScrollMetrics)
    scrollViewport = null
  }

  watch(filterQuery, resetVirtualScroll)
  watch([sortField, sortAscending], resetVirtualScroll)

  function toggleSort(field: SortField): void {
    if (sortField.value === field) {
      sortAscending.value = !sortAscending.value
      return
    }
    sortField.value = field
    sortAscending.value = true
  }

  function sortMark(field: SortField): string {
    if (sortField.value !== field) return ''
    return sortAscending.value ? '↑' : '↓'
  }

  function formatEntryTime(value: string): string {
    return formatDateTime(value)
  }

  return {
    VIRTUAL_LIST_VERTICAL_PADDING,
    VIRTUAL_ROW_HEIGHT,
    clearFilter,
    displayEntries,
    disposeScrollViewport,
    filterMatchCount,
    filterOpen,
    filterQuery,
    filteredEntries,
    formatEntryTime,
    isVirtualized,
    resetVirtualScroll,
    scrollHost,
    selectedKeySet,
    sortAscending,
    sortField,
    sortMark,
    toggleSort,
    virtualContentHeight,
    virtualEntries,
    virtualOffset,
    bindScrollViewport,
  }
}
