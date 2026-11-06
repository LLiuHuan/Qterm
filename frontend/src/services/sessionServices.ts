import {
  ListServices as listServicesBinding,
  RunServiceAction as runServiceActionBinding,
  ServiceDetails as serviceDetailsBinding,
  ServiceLogs as serviceLogsBinding,
} from '../../bindings/changeme/internal/app/service'
import { createRuntimeGateway } from './runtime'

export interface ServiceInfo {
  name: string
  description: string
  loadState: string
  activeState: string
  subState: string
  enabledState: string
  custom: boolean
}

export interface ServiceSnapshot {
  manager: string
  services: ServiceInfo[]
  updatedAt: number
}

export interface ServiceDetail {
  name: string
  fields: Record<string, string>
}

export interface ServiceLog {
  name: string
  text: string
  truncated: boolean
}

export type ServiceActionName = 'start' | 'stop' | 'restart' | 'enable' | 'disable'

export interface SessionServiceGateway {
  readonly persistent: boolean
  list(sessionId: string): Promise<ServiceSnapshot>
  action(sessionId: string, unit: string, action: ServiceActionName): Promise<void>
  detail(sessionId: string, unit: string): Promise<ServiceDetail>
  logs(sessionId: string, unit: string, lines: number): Promise<ServiceLog>
}

class WailsSessionServiceGateway implements SessionServiceGateway {
  readonly persistent = true

  async list(sessionId: string): Promise<ServiceSnapshot> {
    const snapshot = await listServicesBinding(sessionId)
    return {
      manager: snapshot.manager ?? '',
      services: snapshot.services ?? [],
      updatedAt: snapshot.updatedAt ?? Date.now(),
    }
  }

  async action(sessionId: string, unit: string, action: ServiceActionName): Promise<void> {
    await runServiceActionBinding(sessionId, unit, action)
  }

  async detail(sessionId: string, unit: string): Promise<ServiceDetail> {
    const detail = await serviceDetailsBinding(sessionId, unit)

    const fields: Record<string, string> = {}
    for (const [key, value] of Object.entries(detail.fields ?? {})) fields[key] = String(value ?? '')
    return { name: detail.name, fields }
  }

  async logs(sessionId: string, unit: string, lines: number): Promise<ServiceLog> {
    const logs = await serviceLogsBinding(sessionId, unit, lines)
    return { name: logs.name, text: logs.text ?? '', truncated: logs.truncated ?? false }
  }
}

class MemorySessionServiceGateway implements SessionServiceGateway {
  readonly persistent = false
  private readonly state = new Map<string, ServiceInfo>([
    [
      'qterm-agent.service',
      {
        name: 'qterm-agent.service',
        description: 'Qterm helper agent (preview)',
        loadState: 'loaded',
        activeState: 'active',
        subState: 'running',
        enabledState: 'enabled',
        custom: true,
      },
    ],
    [
      'nginx.service',
      {
        name: 'nginx.service',
        description: 'A high performance web server',
        loadState: 'loaded',
        activeState: 'inactive',
        subState: 'dead',
        enabledState: 'enabled',
        custom: false,
      },
    ],
  ])

  async list(): Promise<ServiceSnapshot> {
    return { manager: 'systemd', services: [...this.state.values()], updatedAt: Date.now() }
  }

  async action(_sessionId: string, unit: string, action: ServiceActionName): Promise<void> {
    const current = this.state.get(unit)
    if (!current) return
    if (action === 'start' || action === 'restart') {
      current.activeState = 'active'
      current.subState = 'running'
    }
    if (action === 'stop') {
      current.activeState = 'inactive'
      current.subState = 'dead'
    }
    if (action === 'enable') current.enabledState = 'enabled'
    if (action === 'disable') current.enabledState = 'disabled'
  }

  async detail(_sessionId: string, unit: string): Promise<ServiceDetail> {
    const current = this.state.get(unit)
    return {
      name: unit,
      fields: {
        Id: unit,
        Description: current?.description ?? '',
        ActiveState: current?.activeState ?? 'unknown',
        SubState: current?.subState ?? 'unknown',
        UnitFileState: current?.enabledState ?? 'unknown',
      },
    }
  }

  async logs(_sessionId: string, unit: string, lines: number): Promise<ServiceLog> {
    void lines
    return { name: unit, text: `[preview] recent journal lines for ${unit}`, truncated: false }
  }
}

export function createSessionServiceGateway(): SessionServiceGateway {
  return createRuntimeGateway<SessionServiceGateway>(
    () => new WailsSessionServiceGateway(),
    () => new MemorySessionServiceGateway(),
  )
}
