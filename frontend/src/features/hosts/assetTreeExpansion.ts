import { computed, inject, provide, ref, type ComputedRef, type InjectionKey } from 'vue'

export interface AssetTreeExpansion {
  isOpen(groupId: string): boolean

  setOpen(groupId: string, open: boolean): void

  register(groupId: string): void

  unregister(groupId: string): void

  hasGroups: ComputedRef<boolean>

  hasExpanded: ComputedRef<boolean>

  collapseAll(): void

  expandAll(): void
}

export const assetTreeExpansionKey: InjectionKey<AssetTreeExpansion> = Symbol('assetTreeExpansion')

export function createAssetTreeExpansion(): AssetTreeExpansion {
  const collapsed = ref<ReadonlySet<string>>(new Set())
  const registered = ref<ReadonlyArray<string>>([])

  function isOpen(groupId: string): boolean {
    return !collapsed.value.has(groupId)
  }

  function setOpen(groupId: string, open: boolean): void {
    const next = new Set(collapsed.value)
    if (open) next.delete(groupId)
    else next.add(groupId)
    collapsed.value = next
  }

  function register(groupId: string): void {
    if (registered.value.includes(groupId)) return
    registered.value = [...registered.value, groupId]
  }

  function unregister(groupId: string): void {
    if (!registered.value.includes(groupId)) return
    registered.value = registered.value.filter((id) => id !== groupId)
  }

  const hasGroups = computed(() => registered.value.length > 0)
  const hasExpanded = computed(() => registered.value.some((id) => !collapsed.value.has(id)))

  function collapseAll(): void {
    collapsed.value = new Set(registered.value)
  }

  function expandAll(): void {
    collapsed.value = new Set()
  }

  return {
    isOpen,
    setOpen,
    register,
    unregister,
    hasGroups,
    hasExpanded,
    collapseAll,
    expandAll,
  }
}

export function useAssetTreeExpansion(): AssetTreeExpansion {
  const existing = inject(assetTreeExpansionKey, null)
  if (existing) return existing
  const created = createAssetTreeExpansion()
  provide(assetTreeExpansionKey, created)
  return created
}
