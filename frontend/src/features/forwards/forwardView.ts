import type { HostRecord } from '../../services/hosts'
import type { PortForwardConfig, PortForwardKind } from '../../services/forwards'

export const FORWARD_UNGROUPED_KEY = '__ungrouped__'

export function forwardKindLabelKey(kind: PortForwardKind): string {
  if (kind === 'remote') return 'shell.forwards.kindRemote'
  if (kind === 'dynamic') return 'shell.forwards.kindDynamic'
  return 'shell.forwards.kindLocal'
}

export function forwardListenAddress(row: PortForwardConfig): string {
  return `${row.listenHost || '127.0.0.1'}:${row.listenPort}`
}

export function forwardSummary(row: PortForwardConfig): string {
  const listener = forwardListenAddress(row)
  if (row.kind === 'dynamic') return `${listener} (${(row.proxy ?? 'socks5').toUpperCase()})`
  return `${listener} → ${row.targetHost}:${row.targetPort}`
}

export function forwardDisplayName(row: PortForwardConfig): string {
  return row.name.trim() || forwardSummary(row)
}

export function forwardHostLabel(row: PortForwardConfig, hosts: HostRecord[]): string {
  const host = hosts.find((item) => item.id === row.hostId)
  return host?.name || host?.address || row.hostId
}

export function filterForwardsByHost(rows: PortForwardConfig[], hostId: string): PortForwardConfig[] {
  const target = hostId.trim()
  if (!target) return rows
  return rows.filter((row) => row.hostId === target)
}

export function hasForwardForHost(rows: PortForwardConfig[], hostId: string): boolean {
  const target = hostId.trim()
  if (!target) return false
  return rows.some((row) => row.hostId === target)
}

export function forwardGroupName(row: PortForwardConfig): string {
  return row.group.trim()
}

export function forwardMatchesQuery(
  row: PortForwardConfig,
  query: string,
  hosts: HostRecord[],
  kindLabel: string,
): boolean {
  const keyword = query.trim().toLocaleLowerCase()
  if (!keyword) return true
  const haystack = [
    row.name,
    row.group,
    forwardListenAddress(row),
    row.targetHost,
    String(row.targetPort),

    row.kind === 'dynamic' ? (row.proxy ?? 'socks5') : '',
    kindLabel,
    forwardHostLabel(row, hosts),
  ]
    .join('\n')
    .toLocaleLowerCase()
  return haystack.includes(keyword)
}

export interface ForwardSection {
  key: string
  name: string
  rows: PortForwardConfig[]
}

export interface ForwardSectionOptions {
  hosts: HostRecord[]

  kindLabel: (row: PortForwardConfig) => string

  ungroupedLabel: string

  groups?: string[]

  query?: string
}

export function forwardSections(rows: PortForwardConfig[], options: ForwardSectionOptions): ForwardSection[] {
  const keyword = (options.query ?? '').trim()
  const grouped = new Map<string, PortForwardConfig[]>()

  if (!keyword) {
    for (const name of options.groups ?? []) {
      const value = name.trim()
      if (value && !grouped.has(value)) grouped.set(value, [])
    }
  }
  for (const row of rows) {
    if (!forwardMatchesQuery(row, keyword, options.hosts, options.kindLabel(row))) continue
    const name = forwardGroupName(row)
    const bucket = grouped.get(name)
    if (bucket) bucket.push(row)
    else grouped.set(name, [row])
  }
  return [...grouped.entries()]
    .sort(([left], [right]) => {
      if (left === right) return 0
      if (!left) return 1
      if (!right) return -1
      return left.localeCompare(right)
    })
    .map(([name, items]) => ({
      key: name || FORWARD_UNGROUPED_KEY,
      name: name || options.ungroupedLabel,
      rows: items,
    }))
}

export function existingForwardGroups(rows: PortForwardConfig[]): string[] {
  const seen = new Set<string>()
  for (const row of rows) {
    const value = forwardGroupName(row)
    if (value) seen.add(value)
  }
  return [...seen].sort((left, right) => left.localeCompare(right))
}

export function mergeForwardGroups(rows: PortForwardConfig[], groups: string[]): string[] {
  const merged = new Set<string>(existingForwardGroups(rows))
  for (const group of groups) {
    const value = group.trim()
    if (value) merged.add(value)
  }
  return [...merged].sort((left, right) => left.localeCompare(right))
}

export function renameForwardGroup(rows: PortForwardConfig[], from: string, to: string): PortForwardConfig[] {
  const target = to.trim()
  return rows.map((row) => (forwardGroupName(row) === from ? { ...row, group: target } : row))
}

export function dissolveForwardGroup(rows: PortForwardConfig[], group: string): PortForwardConfig[] {
  return rows.map((row) => (forwardGroupName(row) === group ? { ...row, group: '' } : row))
}
