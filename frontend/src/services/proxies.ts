import { errorMessage } from './errors'
import { createRuntimeGateway } from './runtime'

export type ProxyMode = 'socks5' | 'http'

export interface ProxyRecord {
  id: string
  name: string
  mode: ProxyMode
  host: string
  port: number
  username: string

  passwordConfigured?: boolean
  createdAt: string
  updatedAt: string
}

export interface ProxyInput {
  name: string
  mode: ProxyMode
  host: string
  port: number
  username: string
  password: string
  clearPassword: boolean
}

export interface ProxyGateway {
  readonly persistent: boolean
  list(): Promise<ProxyRecord[]>
  create(input: ProxyInput): Promise<ProxyRecord>
  update(id: string, input: ProxyInput): Promise<ProxyRecord>

  remove(id: string): Promise<number>
}

export function normalizeProxyRecord(record: Partial<ProxyRecord>): ProxyRecord {
  return {
    id: record.id || '',
    name: record.name || '',

    mode: record.mode === 'http' ? 'http' : 'socks5',
    host: record.host || '',
    port: Number(record.port) || 0,
    username: record.username || '',
    passwordConfigured: record.passwordConfigured === true,
    createdAt: record.createdAt || '',
    updatedAt: record.updatedAt || '',
  }
}

class WailsProxyGateway implements ProxyGateway {
  readonly persistent = true

  async list(): Promise<ProxyRecord[]> {
    const { ListProxies } = await import('../../bindings/changeme/internal/app/service')
    const records = await ListProxies()
    return (records ?? []).map((record) => normalizeProxyRecord(record as Partial<ProxyRecord>))
  }

  async create(input: ProxyInput): Promise<ProxyRecord> {
    const { CreateProxy } = await import('../../bindings/changeme/internal/app/service')

    return normalizeProxyRecord((await CreateProxy(input as Parameters<typeof CreateProxy>[0])) as Partial<ProxyRecord>)
  }

  async update(id: string, input: ProxyInput): Promise<ProxyRecord> {
    const { UpdateProxy } = await import('../../bindings/changeme/internal/app/service')

    return normalizeProxyRecord(
      (await UpdateProxy(id, input as Parameters<typeof UpdateProxy>[1])) as Partial<ProxyRecord>,
    )
  }

  async remove(id: string): Promise<number> {
    const { DeleteProxy } = await import('../../bindings/changeme/internal/app/service')
    return Number(await DeleteProxy(id)) || 0
  }
}

class UnavailableProxyGateway implements ProxyGateway {
  readonly persistent = false

  async list(): Promise<ProxyRecord[]> {
    return []
  }

  async create(): Promise<ProxyRecord> {
    throw notReadyError()
  }

  async update(): Promise<ProxyRecord> {
    throw notReadyError()
  }

  async remove(): Promise<number> {
    throw notReadyError()
  }
}

function notReadyError(): { code: string; messageKey: string } {
  return { code: 'unavailable', messageKey: 'proxy.not_ready' }
}

export function createProxyGateway(): ProxyGateway {
  return createRuntimeGateway<ProxyGateway>(
    () => new WailsProxyGateway(),
    () => new UnavailableProxyGateway(),
  )
}

export function proxyErrorMessage(error: unknown, fallback: string): string {
  return errorMessage(error, fallback)
}
