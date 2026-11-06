import {
  DeleteOTPEntry as deleteOTPBinding,
  GenerateOTPCode as generateOTPBinding,
  OTPProvisioningURI as provisioningURIBinding,
  SaveOTPEntry as saveOTPBinding,
} from '../../bindings/changeme/internal/app/service'
import type {
  OTPEntry as BackendOTPEntry,
  OTPSaveInput as BackendOTPSaveInput,
} from '../../bindings/changeme/internal/model/models'
import { createRuntimeGateway } from './runtime'

export type OtpType = 'totp' | 'hotp'
export type OtpAlgorithm = 'SHA1' | 'SHA256' | 'SHA512'

export interface OtpEntry {
  id: string

  name: string

  username?: string

  hostId?: string
  type: OtpType
  algorithm: OtpAlgorithm
  digits: number
  period: number
  counter: number
}

export interface OtpCode {
  code: string

  remainingSeconds: number

  counter: number
}

export interface OtpGateway {
  readonly persistent: boolean

  save(entry: OtpEntry, secret: string): Promise<OtpEntry>
  remove(id: string): Promise<void>
  generate(entry: OtpEntry): Promise<OtpCode>

  provisioningUri(entry: OtpEntry): Promise<string>
}

function toBackend(entry: OtpEntry): BackendOTPEntry {
  return {
    id: entry.id,
    name: entry.name,
    username: entry.username,
    hostId: entry.hostId,
    type: entry.type,
    algorithm: entry.algorithm,
    digits: entry.digits,
    period: entry.period,
    counter: entry.counter,
  }
}

class WailsOtpGateway implements OtpGateway {
  readonly persistent = true

  async save(entry: OtpEntry, secret: string): Promise<OtpEntry> {
    const input: BackendOTPSaveInput = { entry: toBackend(entry), secret }
    const saved = await saveOTPBinding(input)
    return {
      id: saved.id,
      name: saved.name,
      username: saved.username || undefined,
      hostId: saved.hostId || undefined,
      type: (saved.type === 'hotp' ? 'hotp' : 'totp') as OtpType,
      algorithm: (saved.algorithm || 'SHA1') as OtpAlgorithm,
      digits: saved.digits,
      period: saved.period,
      counter: saved.counter,
    }
  }

  async remove(id: string): Promise<void> {
    await deleteOTPBinding(id)
  }

  async provisioningUri(entry: OtpEntry): Promise<string> {
    return provisioningURIBinding(toBackend(entry))
  }

  async generate(entry: OtpEntry): Promise<OtpCode> {
    const result = await generateOTPBinding(toBackend(entry))
    return {
      code: result.code,
      remainingSeconds: result.remainingSeconds,
      counter: result.counter,
    }
  }
}

class MemoryOtpGateway implements OtpGateway {
  readonly persistent = false

  async save(entry: OtpEntry, secret: string): Promise<OtpEntry> {
    if (!secret.trim()) throw new Error('secret is required')
    return {
      ...entry,
      id: entry.id || `otp-${Date.now().toString(36)}`,
      name: entry.name || 'OTP',
      algorithm: 'SHA1',
      digits: entry.digits || 6,
      period: entry.period || 30,
      counter: entry.counter || 0,
    }
  }

  async remove(): Promise<void> {}

  async provisioningUri(entry: OtpEntry): Promise<string> {
    return `otpauth://${entry.type}/${encodeURIComponent(entry.name || 'Qterm')}?secret=JBSWY3DPEHPK3PXP`
  }

  async generate(entry: OtpEntry): Promise<OtpCode> {
    const now = Date.now()
    const period = entry.period > 0 ? entry.period : 30
    const elapsed = Math.floor(now / 1000) % period
    const seed = Math.floor(now / 1000 / period)
    const code = String((seed * 7919 + 13) % 1_000_000).padStart(6, '0')
    return { code, remainingSeconds: period - elapsed, counter: entry.counter }
  }
}

export function createOtpGateway(): OtpGateway {
  return createRuntimeGateway<OtpGateway>(
    () => new WailsOtpGateway(),
    () => new MemoryOtpGateway(),
  )
}
