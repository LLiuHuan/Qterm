import { errorMessage } from './errors'

export interface SSHConfigEntry {
  alias: string
  hostName: string
  user: string
  port: number
  identityFile?: string

  group?: string

  password?: string
  line: number
  duplicate: boolean
}

export interface SSHConfigPreview {
  entries: SSHConfigEntry[]
  skipped: number
}

export interface ImportHostsResult {
  created: number
  failed: number
}

export interface HostImportSource {
  id: string
  label: string
  defaultPath: string

  pathKind: string
  detected: boolean
}

export interface HostImportGateway {
  listSources(): Promise<HostImportSource[]>

  pickPath(source: string): Promise<string>
  preview(source: string, path: string, secret?: string): Promise<SSHConfigPreview>
  importHosts(entries: SSHConfigEntry[]): Promise<ImportHostsResult>
}

export function createHostImportGateway(): HostImportGateway {
  return {
    async listSources(): Promise<HostImportSource[]> {
      const { ListHostImportSources } = await import('../../bindings/changeme/internal/app/service')
      const sources = (await ListHostImportSources()) as HostImportSource[] | null
      return sources ?? []
    },
    async preview(source: string, path: string, secret = ''): Promise<SSHConfigPreview> {
      const { PreviewHostImportWithSecret } = await import('../../bindings/changeme/internal/app/service')
      const result = (await PreviewHostImportWithSecret(source, path, secret)) as SSHConfigPreview | null
      return { entries: result?.entries ?? [], skipped: result?.skipped ?? 0 }
    },
    async pickPath(source: string): Promise<string> {
      const { PickHostImportPath } = await import('../../bindings/changeme/internal/app/service')
      return ((await PickHostImportPath(source)) as string | null) ?? ''
    },
    async importHosts(entries: SSHConfigEntry[]): Promise<ImportHostsResult> {
      const { ImportHosts } = await import('../../bindings/changeme/internal/app/service')
      const result = (await ImportHosts({ entries })) as ImportHostsResult | null
      return { created: result?.created ?? 0, failed: result?.failed ?? 0 }
    },
  }
}

export function hostImportErrorMessage(cause: unknown, fallback: string): string {
  return errorMessage(cause, fallback)
}
