import type { AssetSortMode } from './assetTree'
import { compareAsset } from './assetTree'
import type { GroupRecord, HostRecord } from '../../services/hosts'

function sortByMode<T extends { name: string; sortOrder?: number }>(list: T[], mode: AssetSortMode): T[] {
  return list
    .map((item, index) => ({ item, index }))
    .sort((left, right) => compareAsset(left.item, right.item, mode) || left.index - right.index)
    .map((entry) => entry.item)
}

export function upsertHost(list: readonly HostRecord[], record: HostRecord, mode: AssetSortMode): HostRecord[] {
  const index = list.findIndex((item) => item.id === record.id)
  if (index >= 0) {
    const next = list.slice()
    next[index] = record

    return sortByMode(next, mode)
  }
  return sortByMode([...list, record], mode)
}

export function removeHost(list: readonly HostRecord[], id: string): HostRecord[] {
  const index = list.findIndex((item) => item.id === id)
  if (index < 0) return list as HostRecord[]
  return [...list.slice(0, index), ...list.slice(index + 1)]
}

export function upsertGroup(list: readonly GroupRecord[], record: GroupRecord, mode: AssetSortMode): GroupRecord[] {
  const index = list.findIndex((item) => item.id === record.id)
  if (index >= 0) {
    const next = list.slice()
    next[index] = record
    return sortByMode(next, mode)
  }
  return sortByMode([...list, record], mode)
}

export function removeGroup(list: readonly GroupRecord[], id: string): GroupRecord[] {
  const index = list.findIndex((item) => item.id === id)
  if (index < 0) return list as GroupRecord[]
  return [...list.slice(0, index), ...list.slice(index + 1)]
}

export function patchHostPlacement(
  list: readonly HostRecord[],
  id: string,
  patch: { groupId?: string; sortOrder?: number },
  mode: AssetSortMode,
): HostRecord[] {
  const index = list.findIndex((item) => item.id === id)
  if (index < 0) return list as HostRecord[]
  const next = list.slice()
  next[index] = { ...next[index], ...patch }
  return sortByMode(next, mode)
}
