import type { HostRecord } from '@/services/hosts.ts'

export function hostConnectionSummary(host: HostRecord): string {
  if (host.protocol === 'serial') {
    const baud = host.serialBaud && host.serialBaud > 0 ? host.serialBaud : 115200
    return `${host.address} · ${baud} 8N1`
  }
  return `${host.username}@${host.address}:${host.port}`
}

export function hostTags(host: HostRecord): string[] {
  return (host.tags ?? []).filter((tag) => tag.trim() !== '')
}

const HOST_CARD_TAG_LIMIT = 3

export function visibleHostTags(host: HostRecord): string[] {
  return hostTags(host).slice(0, HOST_CARD_TAG_LIMIT)
}

export function hiddenHostTagCount(host: HostRecord): number {
  return Math.max(0, hostTags(host).length - HOST_CARD_TAG_LIMIT)
}
