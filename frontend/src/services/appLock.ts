import { createRuntimeGateway } from './runtime'

export interface AppLockStatus {
  enabled: boolean
  hasPassword: boolean

  autoLockMinutes: number

  hasRecovery: boolean

  recoveryQuestion: string
}

export interface AppLockGateway {
  readonly persistent: boolean
  status(): Promise<AppLockStatus>
  enable(password: string, autoLockMinutes: number, question?: string, answer?: string): Promise<void>

  resetByRecovery(answer: string, next: string): Promise<void>

  broadcastLocked(locked: boolean): Promise<void>
  changePassword(current: string, next: string): Promise<void>
  disable(password: string): Promise<void>
  setAutoLockMinutes(minutes: number): Promise<void>
  verify(password: string): Promise<boolean>
}

class PreviewAppLockGateway implements AppLockGateway {
  readonly persistent = false

  private readonly key = 'qterm.app-lock.preview'

  private read(): { hash: string; autoLockMinutes: number; question?: string; answerHash?: string } | null {
    try {
      const raw = globalThis.localStorage?.getItem(this.key)
      if (!raw) return null
      const parsed = JSON.parse(raw) as {
        hash?: string
        autoLockMinutes?: number
        question?: string
        answerHash?: string
      }
      if (typeof parsed.hash !== 'string' || parsed.hash === '') return null
      return {
        hash: parsed.hash,
        autoLockMinutes: Number(parsed.autoLockMinutes) || 0,
        question: parsed.question,
        answerHash: parsed.answerHash,
      }
    } catch {
      return null
    }
  }

  private write(value: { hash: string; autoLockMinutes: number; question?: string; answerHash?: string } | null): void {
    try {
      if (!value) globalThis.localStorage?.removeItem(this.key)
      else globalThis.localStorage?.setItem(this.key, JSON.stringify(value))
    } catch {}
  }

  private async digest(password: string): Promise<string> {
    const data = new TextEncoder().encode(`qterm-preview:${password}`)
    const hash = await crypto.subtle.digest('SHA-256', data)
    return Array.from(new Uint8Array(hash))
      .map((byte) => byte.toString(16).padStart(2, '0'))
      .join('')
  }

  async status(): Promise<AppLockStatus> {
    const stored = this.read()
    return {
      enabled: stored !== null,
      hasPassword: stored !== null,
      autoLockMinutes: stored?.autoLockMinutes ?? 0,
      hasRecovery: Boolean(stored?.answerHash),
      recoveryQuestion: stored?.question ?? '',
    }
  }

  async enable(password: string, autoLockMinutes: number, question = '', answer = ''): Promise<void> {
    if (this.read()) throw new Error('applock.alreadyEnabled')
    const stored: { hash: string; autoLockMinutes: number; question?: string; answerHash?: string } = {
      hash: await this.digest(password),
      autoLockMinutes,
    }
    if (question && answer) {
      stored.question = question
      stored.answerHash = await this.digest(`answer:${answer}`)
    }
    this.write(stored)
  }

  async resetByRecovery(answer: string, next: string): Promise<void> {
    const stored = this.read()
    if (!stored?.answerHash) throw new Error('applock.noRecovery')
    if (stored.answerHash !== (await this.digest(`answer:${answer}`))) throw new Error('applock.wrongAnswer')
    this.write({ ...stored, hash: await this.digest(next) })
  }

  async broadcastLocked(): Promise<void> {}

  async changePassword(current: string, next: string): Promise<void> {
    const stored = this.read()
    if (!stored) throw new Error('applock.notEnabled')
    if (stored.hash !== (await this.digest(current))) throw new Error('applock.wrongPassword')
    this.write({ hash: await this.digest(next), autoLockMinutes: stored.autoLockMinutes })
  }

  async disable(password: string): Promise<void> {
    const stored = this.read()
    if (!stored) return
    if (stored.hash !== (await this.digest(password))) throw new Error('applock.wrongPassword')
    this.write(null)
  }

  async setAutoLockMinutes(minutes: number): Promise<void> {
    const stored = this.read()
    if (!stored) return
    this.write({ ...stored, autoLockMinutes: minutes })
  }

  async verify(password: string): Promise<boolean> {
    const stored = this.read()
    if (!stored) return false
    return stored.hash === (await this.digest(password))
  }
}

class WailsAppLockGateway implements AppLockGateway {
  readonly persistent = true

  private async bindings() {
    return import('../../bindings/changeme/internal/app/service')
  }

  async status(): Promise<AppLockStatus> {
    const { GetAppLockStatus } = await this.bindings()
    const status = await GetAppLockStatus()
    return {
      enabled: status.enabled === true,
      hasPassword: status.hasPassword === true,
      autoLockMinutes: Number(status.autoLockMinutes) || 0,
      hasRecovery: status.hasRecovery === true,
      recoveryQuestion: status.recoveryQuestion ?? '',
    }
  }

  async enable(password: string, autoLockMinutes: number, question = '', answer = ''): Promise<void> {
    const { EnableAppLock } = await this.bindings()
    await EnableAppLock(password, autoLockMinutes, question, answer)
  }

  async changePassword(current: string, next: string): Promise<void> {
    const { ChangeAppLockPassword } = await this.bindings()
    await ChangeAppLockPassword(current, next)
  }

  async disable(password: string): Promise<void> {
    const { DisableAppLock } = await this.bindings()
    await DisableAppLock(password)
  }

  async setAutoLockMinutes(minutes: number): Promise<void> {
    const { SetAppLockAutoLockMinutes } = await this.bindings()
    await SetAppLockAutoLockMinutes(minutes)
  }

  async verify(password: string): Promise<boolean> {
    const { VerifyAppLockPassword } = await this.bindings()
    return (await VerifyAppLockPassword(password)) === true
  }

  async resetByRecovery(answer: string, next: string): Promise<void> {
    const { ResetAppLockPassword } = await this.bindings()
    await ResetAppLockPassword(answer, next)
  }

  async broadcastLocked(locked: boolean): Promise<void> {
    const { NotifyAppLockState } = await this.bindings()
    await NotifyAppLockState(locked)
  }
}

export function createAppLockGateway(): AppLockGateway {
  return createRuntimeGateway<AppLockGateway>(
    () => new WailsAppLockGateway(),
    () => new PreviewAppLockGateway(),
  )
}
