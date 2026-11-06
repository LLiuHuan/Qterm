import { createPortForwardGateway, type PortForwardConfig, type PortForwardStatus } from '../../services/forwards'

function isActiveStatus(status: PortForwardStatus | null): boolean {
  return status?.state === 'running' || status?.state === 'starting' || status?.state === 'reconnecting'
}

export interface PortForwardActionsOptions {
  forwards: () => PortForwardConfig[]

  update: (next: PortForwardConfig[]) => void
}

export function usePortForwardActions(options: PortForwardActionsOptions) {
  const gateway = createPortForwardGateway()

  async function activeStatus(id: string): Promise<PortForwardStatus | null> {
    return gateway.status(id).catch(() => null)
  }

  async function save(payload: PortForwardConfig): Promise<boolean> {
    const current = options.forwards()
    const exists = current.some((row) => row.id === payload.id)
    const wasActive = exists && isActiveStatus(await activeStatus(payload.id))
    options.update(exists ? current.map((row) => (row.id === payload.id ? payload : row)) : [...current, payload])
    if (wasActive) {
      await gateway.close(payload.id).catch(() => {})
      await gateway.open(payload).catch(() => {})
    }
    return wasActive
  }

  async function remove(row: PortForwardConfig): Promise<void> {
    await gateway.close(row.id).catch(() => {})
    options.update(options.forwards().filter((item) => item.id !== row.id))
  }

  async function setEnabled(row: PortForwardConfig, enabled: boolean): Promise<PortForwardStatus> {
    if (enabled) await gateway.open(row)
    else await gateway.close(row.id)
    return gateway.status(row.id)
  }

  return { gateway, save, remove, setEnabled, activeStatus }
}
