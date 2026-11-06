import type { GroupRecord, HostRecord } from '@/services/hosts.ts'
import type { GroupedHosts } from '../../types/qterm'

export type AssetSortMode = 'name-asc' | 'name-desc' | 'custom'

export function buildAssetTree(hosts: HostRecord[], groups: GroupRecord[], query: string, sortMode: AssetSortMode) {
  const normalizedQuery = query.trim().toLocaleLowerCase()
  const filteredHosts = [...hosts]
    .filter(
      (host) =>
        !normalizedQuery ||
        [host.name, host.address, host.username, host.group, ...(host.tags ?? [])]
          .join(' ')
          .toLocaleLowerCase()
          .includes(normalizedQuery),
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
