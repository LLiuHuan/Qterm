import { hasWailsRuntime } from './runtime'

export type AICommandRuleAction = 'confirm' | 'deny'

export interface AICommandRule {
  id: string
  pattern: string
  action: AICommandRuleAction
  note: string
}

export interface AICommandRuleInput {
  pattern: string
  action: AICommandRuleAction
  note: string
}

export const MAX_AI_COMMAND_RULES = 200

async function withRuntime<T>(
  run: (api: typeof import('../../bindings/changeme/internal/app/service')) => Promise<T>,
  fallback: T,
): Promise<T> {
  if (!hasWailsRuntime()) {
    return fallback
  }
  const api = await import('../../bindings/changeme/internal/app/service')
  return run(api)
}

function toRules(value: unknown): AICommandRule[] {
  if (!Array.isArray(value)) {
    return []
  }
  return value.map((entry) => {
    const record = (entry ?? {}) as Record<string, unknown>
    return {
      id: typeof record.id === 'string' ? record.id : '',
      pattern: typeof record.pattern === 'string' ? record.pattern : '',
      action: record.action === 'deny' ? 'deny' : 'confirm',
      note: typeof record.note === 'string' ? record.note : '',
    }
  })
}

export async function loadAICommandRules(): Promise<AICommandRule[]> {
  return withRuntime(async (api) => toRules(await api.AICommandRules()), [])
}

export async function createAICommandRule(input: AICommandRuleInput): Promise<AICommandRule[]> {
  return withRuntime(async (api) => toRules(await api.CreateAICommandRule(input)), [])
}

export async function updateAICommandRule(ruleID: string, input: AICommandRuleInput): Promise<AICommandRule[]> {
  return withRuntime(async (api) => toRules(await api.UpdateAICommandRule(ruleID, input)), [])
}

export async function deleteAICommandRule(ruleID: string): Promise<AICommandRule[]> {
  return withRuntime(async (api) => toRules(await api.DeleteAICommandRule(ruleID)), [])
}

export async function importAICommandRules(): Promise<AICommandRule[]> {
  return withRuntime(async (api) => toRules(await api.ImportAICommandRulesFromFile()), [])
}

export async function exportAICommandRules(): Promise<void> {
  await withRuntime(async (api) => {
    await api.ExportAICommandRulesToFile()
  }, undefined)
}

export function validateRulePattern(pattern: string): boolean {
  if (pattern.trim() === '') {
    return false
  }
  try {
    new RegExp(pattern)
    return true
  } catch {
    return false
  }
}
