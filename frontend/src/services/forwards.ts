import {
  CloseForward as closeForwardBinding,
  ForwardStatus as forwardStatusBinding,
  OpenForward as openForwardBinding,
} from '../../bindings/changeme/internal/app/service'
import { ForwardKind } from '../../bindings/changeme/internal/model/models'
import type {
  ForwardConfig as BackendForwardConfig,
  ForwardStatus as BackendForwardStatus,
} from '../../bindings/changeme/internal/model/models'
import { createRuntimeGateway } from './runtime'

export type PortForwardKind = 'local' | 'remote' | 'dynamic'

export type PortForwardProxy = 'socks5' | 'http'

export interface PortForwardConfig {
  id: string

  name: string

  group: string
  kind: PortForwardKind
  hostId: string
  listenHost: string
  listenPort: number
  targetHost: string
  targetPort: number

  proxy?: PortForwardProxy
  autoOpen: boolean
}

export type PortForwardState = 'stopped' | 'starting' | 'running' | 'reconnecting' | 'disconnected' | 'error'

export interface PortForwardStatus {
  id: string
  state: PortForwardState
  error?: string

  listenPort: number
  retryAttempt: number
  updatedAt: number
}

export const PORT_FORWARD_STATUS_EVENT = 'qterm:forward-status'

export interface PortForwardGateway {
  readonly persistent: boolean
  open(config: PortForwardConfig): Promise<void>
  close(id: string): Promise<void>
  status(id: string): Promise<PortForwardStatus>
}

function toBackendKind(kind: PortForwardKind): ForwardKind {
  if (kind === 'remote') return ForwardKind.ForwardRemote
  if (kind === 'dynamic') return ForwardKind.ForwardDynamic
  return ForwardKind.ForwardLocal
}

function toBackend(config: PortForwardConfig): BackendForwardConfig {
  return {
    id: config.id,
    name: config.name,
    kind: toBackendKind(config.kind),
    hostId: config.hostId,
    listenHost: config.listenHost,
    listenPort: config.listenPort,
    targetHost: config.targetHost,
    targetPort: config.targetPort,

    proxy: config.kind === 'dynamic' ? (config.proxy ?? 'socks5') : '',
    autoOpen: config.autoOpen,
  }
}

function fromBackend(status: BackendForwardStatus): PortForwardStatus {
  return {
    id: status.id,
    state: (status.state ?? 'stopped') as unknown as PortForwardState,
    error: status.error || undefined,
    listenPort: status.listenPort ?? 0,
    retryAttempt: status.retryAttempt ?? 0,
    updatedAt: status.updatedAt ?? 0,
  }
}

class WailsPortForwardGateway implements PortForwardGateway {
  readonly persistent = true

  async open(config: PortForwardConfig): Promise<void> {
    await openForwardBinding(toBackend(config))
  }

  async close(id: string): Promise<void> {
    await closeForwardBinding(id)
  }

  async status(id: string): Promise<PortForwardStatus> {
    return fromBackend(await forwardStatusBinding(id))
  }
}

class MemoryPortForwardGateway implements PortForwardGateway {
  readonly persistent = false
  private readonly states = new Map<string, PortForwardStatus>()

  async open(config: PortForwardConfig): Promise<void> {
    this.states.set(config.id, {
      id: config.id,
      state: 'running',
      listenPort: config.listenPort,
      retryAttempt: 0,
      updatedAt: Date.now(),
    })
  }

  async close(id: string): Promise<void> {
    this.states.set(id, { id, state: 'stopped', listenPort: 0, retryAttempt: 0, updatedAt: Date.now() })
  }

  async status(id: string): Promise<PortForwardStatus> {
    return this.states.get(id) ?? { id, state: 'stopped', listenPort: 0, retryAttempt: 0, updatedAt: 0 }
  }
}

export function createPortForwardGateway(): PortForwardGateway {
  return createRuntimeGateway<PortForwardGateway>(
    () => new WailsPortForwardGateway(),
    () => new MemoryPortForwardGateway(),
  )
}
