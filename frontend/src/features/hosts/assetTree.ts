import type { GroupRecord, HostRecord } from '@/services/hosts.ts'
import type { GroupedHosts } from '../../types/qterm'

export type AssetSortMode = 'name-asc' | 'name-desc' | 'custom'

export function buildAssetTree(hosts: HostRecord[], groups: GroupRecord[], query: string, sortMode: AssetSortMode) {
  const normalizedQuery = query.trim().toLocaleLowerCase()

  const groupByID = new Map(groups.map((group) => [group.id, group]))
  const groupMatchCache = new Map<string, boolean>()
  const groupMatchesQuery = (groupId: string): boolean => {
    const cached = groupMatchCache.get(groupId)
    if (cached !== undefined) return cached

    let current: GroupRecord | undefined = groupByID.get(groupId)
    const chain: string[] = []
    let matched = false
    const seen = new Set<string>()
    while (current && !seen.has(current.id)) {
      seen.add(current.id)
      if (current.name.toLocaleLowerCase().includes(normalizedQuery)) {
        matched = true
        break
      }
      chain.push(current.id)
      current = current.parentId ? groupByID.get(current.parentId) : undefined
    }
    groupMatchCache.set(groupId, matched)
    for (const id of chain) groupMatchCache.set(id, matched)
    return matched
  }
  const filteredHosts = [...hosts]
    .filter(
      (host) =>
        !normalizedQuery ||
        [
          host.name,
          host.alias ?? '',
          host.customId ?? '',
          host.address,
          host.username,
          host.group,
          ...(host.tags ?? []),
        ]
          .join(' ')
          .toLocaleLowerCase()
          .includes(normalizedQuery) ||
        (!!host.groupId && groupMatchesQuery(host.groupId)),
    )
    .sort((left, right) => compareAsset(left, right, sortMode))
  const groupIDs = new Set(groups.map((group) => group.id))
  const ungroupedHosts = filteredHosts.filter((host) => !host.groupId || !groupIDs.has(host.groupId))
  const orderedGroups = [...groups].sort((left, right) => compareAsset(left, right, sortMode))
  const groupMap = new Map<string, GroupedHosts>()
  for (const group of orderedGroups)
    groupMap.set(group.id, { id: group.id, name: group.name, sortOrder: group.sortOrder, items: [], children: [] })
  for (const host of filteredHosts) if (host.groupId) groupMap.get(host.groupId)?.items.push(host)
  const roots: GroupedHosts[] = []
  for (const group of orderedGroups) {
    const current = groupMap.get(group.id)
    if (!current) continue
    if (group.parentId && groupMap.has(group.parentId)) groupMap.get(group.parentId)!.children!.push(current)
    else roots.push(current)
  }
  const attachEntries = (group: GroupedHosts) => {
    for (const child of group.children ?? []) attachEntries(child)
    group.entries = [
      ...group.items.map((value) => ({ kind: 'host' as const, value })),
      ...(group.children ?? []).map((value) => ({ kind: 'group' as const, value })),
    ].sort((left, right) => compareAsset(left.value, right.value, sortMode))
  }
  for (const group of roots) attachEntries(group)
  const rootEntries = [
    ...roots.map((value) => ({ kind: 'group' as const, value })),
    ...ungroupedHosts.map((value) => ({ kind: 'host' as const, value })),
  ].sort((left, right) => compareAsset(left.value, right.value, sortMode))
  return { filteredHosts, ungroupedHosts, groupedHosts: roots, rootEntries }
}

export function compareAsset(
  left: { name: string; sortOrder?: number },
  right: { name: string; sortOrder?: number },
  sortMode: AssetSortMode,
): number {
  if (sortMode === 'custom')
    return (left.sortOrder ?? 0) - (right.sortOrder ?? 0) || left.name.localeCompare(right.name, 'zh-CN')
  const value = left.name.localeCompare(right.name, 'zh-CN')
  return sortMode === 'name-asc' ? value : -value
}

export interface SelectionClickResult {
  ids: string[]
  anchor: string | null
}

export function selectionAfterClick(
  orderedIDs: string[],
  current: string[],
  anchor: string | null,
  id: string,
  modifiers: { toggle: boolean; range: boolean },
): SelectionClickResult {
  if (modifiers.range && anchor && orderedIDs.includes(anchor) && orderedIDs.includes(id)) {
    const from = orderedIDs.indexOf(anchor)
    const to = orderedIDs.indexOf(id)
    const [start, end] = from <= to ? [from, to] : [to, from]
    return { ids: orderedIDs.slice(start, end + 1), anchor }
  }
  if (modifiers.toggle) {
    const next = current.includes(id) ? current.filter((item) => item !== id) : [...current, id]

    const ordered = orderedIDs.filter((item) => next.includes(item))
    for (const item of next) if (!ordered.includes(item)) ordered.push(item)
    return { ids: ordered, anchor: id }
  }
  return { ids: [id], anchor: id }
}
