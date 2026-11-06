import { ref } from 'vue'

export function useSnippetGroupPagination(pageSize = 20) {
  const visibleCounts = ref<Record<string, number>>({})

  function reset(): void {
    visibleCounts.value = {}
  }

  function visibleCount(groupKey: string, total: number, expanded: boolean): number {
    if (!expanded) return 0
    return Math.min(total, visibleCounts.value[groupKey] ?? pageSize)
  }

  function visibleItems<T>(groupKey: string, items: T[], expanded: boolean): T[] {
    return items.slice(0, visibleCount(groupKey, items.length, expanded))
  }

  function loadMore(groupKey: string, total: number): void {
    visibleCounts.value = {
      ...visibleCounts.value,
      [groupKey]: Math.min(total, (visibleCounts.value[groupKey] ?? pageSize) + pageSize),
    }
  }

  function reveal(groupKey: string, itemIndex: number): void {
    if (itemIndex < 0) return
    const requiredCount = Math.ceil((itemIndex + 1) / pageSize) * pageSize
    visibleCounts.value = {
      ...visibleCounts.value,
      [groupKey]: Math.max(visibleCounts.value[groupKey] ?? pageSize, requiredCount),
    }
  }

  return { loadMore, reset, reveal, visibleCount, visibleItems }
}
