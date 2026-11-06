import type { CredentialPromptKind } from '../features/sessions/credentialAutofill'
import { createRuntimeGateway } from './runtime'

export interface CredentialPromptRuleRecord {
  id: string
  kind: CredentialPromptKind
  pattern: string
  enabled: boolean
  sortOrder: number
  createdAt: string
  updatedAt: string
}

export interface CredentialPromptRuleDraft {
  kind: CredentialPromptKind
  pattern: string
  enabled: boolean
}

export interface CredentialPromptRuleGateway {
  readonly persistent: boolean
  list(): Promise<CredentialPromptRuleRecord[]>
  create(input: CredentialPromptRuleDraft): Promise<CredentialPromptRuleRecord>
  update(id: string, input: CredentialPromptRuleDraft): Promise<CredentialPromptRuleRecord>
  remove(id: string): Promise<void>

  typeIntoSession(sessionID: string, credentialID: string): Promise<void>
}

function createID(): string {
  return `prompt-rule-${Math.random().toString(36).slice(2, 10)}`
}

class MemoryCredentialPromptRuleGateway implements CredentialPromptRuleGateway {
  readonly persistent = false
  private rules: CredentialPromptRuleRecord[] = []

  async list(): Promise<CredentialPromptRuleRecord[]> {
    return this.rules.map((rule) => ({ ...rule }))
  }

  async create(input: CredentialPromptRuleDraft): Promise<CredentialPromptRuleRecord> {
    const now = new Date().toISOString()
    const rule: CredentialPromptRuleRecord = {
      id: createID(),
      kind: input.kind,
      pattern: input.pattern,
      enabled: input.enabled,
      sortOrder: this.rules.length,
      createdAt: now,
      updatedAt: now,
    }
    this.rules.push(rule)
    return { ...rule }
  }

  async update(id: string, input: CredentialPromptRuleDraft): Promise<CredentialPromptRuleRecord> {
    const index = this.rules.findIndex((rule) => rule.id === id)
    if (index < 0) throw new Error('规则不存在')
    const updated: CredentialPromptRuleRecord = {
      ...this.rules[index]!,
      kind: input.kind,
      pattern: input.pattern,
      enabled: input.enabled,
      updatedAt: new Date().toISOString(),
    }
    this.rules[index] = updated
    return { ...updated }
  }

  async remove(id: string): Promise<void> {
    this.rules = this.rules.filter((rule) => rule.id !== id)
  }

  async typeIntoSession(): Promise<void> {
    throw new Error('浏览器预览没有真实会话，无法注入凭据')
  }
}

class WailsCredentialPromptRuleGateway implements CredentialPromptRuleGateway {
  readonly persistent = true

  async list(): Promise<CredentialPromptRuleRecord[]> {
    const { ListCredentialPromptRules } = await import('../../bindings/changeme/internal/app/service')
    const rules = await ListCredentialPromptRules()
    return (rules ?? []).map((rule) => ({
      id: rule.id,
      kind: rule.kind === 'username' ? 'username' : 'password',
      pattern: rule.pattern,
      enabled: rule.enabled,
      sortOrder: Number(rule.sortOrder ?? 0),
      createdAt: rule.createdAt,
      updatedAt: rule.updatedAt,
    }))
  }

  async create(input: CredentialPromptRuleDraft): Promise<CredentialPromptRuleRecord> {
    const { CreateCredentialPromptRule } = await import('../../bindings/changeme/internal/app/service')
    const rule = await CreateCredentialPromptRule({
      kind: input.kind,
      pattern: input.pattern,
      enabled: input.enabled,
    })
    return {
      id: rule.id,
      kind: rule.kind === 'username' ? 'username' : 'password',
      pattern: rule.pattern,
      enabled: rule.enabled,
      sortOrder: Number(rule.sortOrder ?? 0),
      createdAt: rule.createdAt,
      updatedAt: rule.updatedAt,
    }
  }

  async update(id: string, input: CredentialPromptRuleDraft): Promise<CredentialPromptRuleRecord> {
    const { UpdateCredentialPromptRule } = await import('../../bindings/changeme/internal/app/service')
    const rule = await UpdateCredentialPromptRule(id, {
      kind: input.kind,
      pattern: input.pattern,
      enabled: input.enabled,
    })
    return {
      id: rule.id,
      kind: rule.kind === 'username' ? 'username' : 'password',
      pattern: rule.pattern,
      enabled: rule.enabled,
      sortOrder: Number(rule.sortOrder ?? 0),
      createdAt: rule.createdAt,
      updatedAt: rule.updatedAt,
    }
  }

  async remove(id: string): Promise<void> {
    const { DeleteCredentialPromptRule } = await import('../../bindings/changeme/internal/app/service')
    await DeleteCredentialPromptRule(id)
  }

  async typeIntoSession(sessionID: string, credentialID: string): Promise<void> {
    const { TypeCredentialIntoSession } = await import('../../bindings/changeme/internal/app/service')
    await TypeCredentialIntoSession(sessionID, credentialID)
  }
}

export function createCredentialPromptRuleGateway(): CredentialPromptRuleGateway {
  return createRuntimeGateway<CredentialPromptRuleGateway>(
    () => new WailsCredentialPromptRuleGateway(),
    () => new MemoryCredentialPromptRuleGateway(),
  )
}
