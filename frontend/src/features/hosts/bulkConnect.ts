import type { HostRecord } from '../../services/hosts.ts'
import type { GroupedHosts } from '../../types/qterm'
import { parseAdHocTarget, type AdHocProtocol, type ParsedAdHocTarget } from '../../services/adhocSession'

export const BULK_CONNECT_LIMIT = 20

export interface BulkRunResult {
  done: number
  failed: number
  skipped: number
}

export function collectGroupHosts(group: GroupedHosts | null | undefined): HostRecord[] {
  const hosts: HostRecord[] = []
  const seen = new Set<string>()

  const push = (host: HostRecord | null | undefined): void => {
    if (!host?.id || seen.has(host.id)) return
    seen.add(host.id)
    hosts.push(host)
  }

  const walk = (node: GroupedHosts): void => {
    for (const entry of node.entries ?? []) {
      if (entry.kind === 'host') push(entry.value)
      else walk(entry.value)
    }

    if (!node.entries?.length) {
      for (const host of node.items ?? []) push(host)
      for (const child of node.children ?? []) walk(child)
    }
  }

  if (group) walk(group)
  return hosts
}

export function pendingConnectHosts(hosts: HostRecord[], openHostIds: Iterable<string>): HostRecord[] {
  const open = new Set(openHostIds)
  return hosts.filter((host) => !open.has(host.id))
}

export async function runBulkConnect(
  hosts: HostRecord[],
  open: (host: HostRecord) => Promise<void>,
): Promise<BulkRunResult> {
  const result: BulkRunResult = { done: 0, failed: 0, skipped: 0 }
  for (const host of hosts) {
    try {
      await open(host)
      result.done += 1
    } catch {
      result.failed += 1
    }
  }
  return result
}

export interface BulkTargetParseResult {
  targets: ParsedAdHocTarget[]
  errors: Array<{ value: string; code: string }>
}

export function parseAdHocTargets(protocol: AdHocProtocol, raw: string): BulkTargetParseResult {
  const pieces = raw
    .split(/[,，;；\n\r\t]+/)
    .map((piece) => piece.trim())
    .filter((piece) => piece.length > 0)

  const targets: ParsedAdHocTarget[] = []
  const errors: Array<{ value: string; code: string }> = []
  const seen = new Set<string>()

  for (const piece of pieces) {
    const parsed = parseAdHocTarget(protocol, piece)
    if (typeof parsed === 'string') {
      errors.push({ value: piece, code: parsed })
      continue
    }
    const { address, username, port } = parsed.input
    const key = `${protocol}|${username}|${address}|${port}`
    if (seen.has(key)) continue
    seen.add(key)
    targets.push(parsed)
  }

  return { targets, errors }
}

export function flattenMultilinePaste(text: string): string {
  return text
    .split(/[\r\n]+/)
    .map((piece) => piece.trim())
    .filter((piece) => piece.length > 0)
    .join(', ')
}
