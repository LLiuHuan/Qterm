import { errorMessage } from './errors'

export interface KnownHostEntry {
  host: string
  keyType: string

  sha256Fingerprint: string
  comment: string

  marker: string
  source: 'managed' | 'system'

  line: number

  editable: boolean
}

export interface KnownHostsGateway {
  path(): Promise<string>
  list(): Promise<KnownHostEntry[]>
  remove(line: number): Promise<void>
}

export const previewKnownHostsGateway: KnownHostsGateway = {
  async path() {
    return '~/.ssh/known_hosts'
  },
  async list() {
    return []
  },
  async remove() {
    throw new Error('浏览器预览不能修改 known_hosts')
  },
}

export function createKnownHostsGateway(): KnownHostsGateway {
  return {
    async path(): Promise<string> {
      const { KnownHostsPath } = await import('../../bindings/changeme/internal/app/service')
      return (await KnownHostsPath()) ?? ''
    },
    async list(): Promise<KnownHostEntry[]> {
      const { ListKnownHosts } = await import('../../bindings/changeme/internal/app/service')
      const entries = (await ListKnownHosts()) as KnownHostEntry[] | null
      return entries ?? []
    },
    async remove(line: number): Promise<void> {
      const { DeleteKnownHost } = await import('../../bindings/changeme/internal/app/service')
      await DeleteKnownHost(line)
    },
  }
}

export function knownHostsErrorMessage(cause: unknown, fallback: string): string {
  return errorMessage(cause, fallback)
}
