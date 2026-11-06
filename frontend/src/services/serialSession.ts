import { errorMessage } from './errors'
import type { AdHocOpenResult } from './adhocSession'

export interface SerialPortInfo {
  name: string
}

export const SERIAL_BAUD_OPTIONS = [9600, 19200, 38400, 57600, 115200] as const

export const SERIAL_DEFAULT_BAUD = 115200

export async function listSerialPorts(): Promise<SerialPortInfo[]> {
  if (typeof window === 'undefined') return []
  try {
    const { ListSerialPorts } = await import('../../bindings/changeme/internal/app/service')
    const ports = (await ListSerialPorts()) as SerialPortInfo[] | null
    return ports ?? []
  } catch {
    return []
  }
}

export async function openSerialSession(device: string, baud: number): Promise<AdHocOpenResult> {
  const { OpenSerialSession } = await import('../../bindings/changeme/internal/app/service')
  const result = (await OpenSerialSession({ device, baud })) as {
    session?: { id?: string; hostId?: string; state?: string; attachEpoch?: number } | null
    streamToken?: string
  } | null
  const session = result?.session
  if (!session?.id) throw new Error('shell.error.openSession')
  return {
    sessionId: session.id,
    hostId: session.hostId ?? '',
    streamToken: result?.streamToken ?? '',
    state: (session.state ?? 'connecting') as AdHocOpenResult['state'],
    attachEpoch: session.attachEpoch,
  }
}

export function serialSessionErrorMessage(cause: unknown, fallback: string): string {
  return errorMessage(cause, fallback)
}

const LAST_USED_KEY = 'qterm.serial.last-used.v1'

export function rememberSerialTarget(device: string, baud: number): void {
  try {
    window.localStorage.setItem(LAST_USED_KEY, JSON.stringify({ device, baud }))
  } catch {}
}

export function readLastSerialTarget(): { device: string; baud: number } | null {
  try {
    const raw = window.localStorage.getItem(LAST_USED_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as { device?: unknown; baud?: unknown }
    if (typeof parsed.device !== 'string' || typeof parsed.baud !== 'number') return null
    return { device: parsed.device, baud: parsed.baud }
  } catch {
    return null
  }
}
