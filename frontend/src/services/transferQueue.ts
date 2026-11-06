import { formatByteRate } from '../lib/format'
import type { SftpGateway, SFTPTransferRecord } from './sftp'

export type TransferQueueAction = 'cancel' | 'retry' | 'dismiss' | 'pause' | 'resume'

export function transferQueueAction(
  transfer: SFTPTransferRecord,
  gateway: SftpGateway,
): TransferQueueAction | undefined {
  if (transfer.state === 'failed') return 'retry'
  if (transfer.state === 'paused') return gateway.resumeSFTPTransfer ? 'resume' : 'cancel'
  if (transfer.state === 'running' && transfer.direction === 'upload' && gateway.pauseSFTPTransfer) return 'pause'
  if (transfer.state === 'queued' || transfer.state === 'running') return 'cancel'
  return undefined
}

export function transferSecondaryAction(transfer: SFTPTransferRecord): TransferQueueAction | undefined {
  if (transfer.state === 'paused') return 'cancel'
  if (transfer.state === 'running') return 'cancel'
  return undefined
}

export function transferSpeedText(rate: number | undefined, fallbackRate = 0): string | undefined {
  const value = (rate ?? 0) > 0 ? (rate as number) : fallbackRate
  return value > 0 ? formatByteRate(value) : undefined
}

export function transferEtaText(remainingBytes: number, rate: number | undefined): string | undefined {
  const value = rate ?? 0
  if (value <= 0 || remainingBytes <= 0) return undefined
  const rawSeconds = Math.ceil(remainingBytes / value)
  if (rawSeconds <= 0 || rawSeconds > 99 * 3600) return undefined
  if (rawSeconds < 60) return `${Math.max(5, Math.round(rawSeconds / 5) * 5)}s`
  const minutes = Math.round(rawSeconds / 60)
  if (minutes < 60) return `${minutes}m`
  return `${Math.floor(minutes / 60)}h ${minutes % 60}m`
}
