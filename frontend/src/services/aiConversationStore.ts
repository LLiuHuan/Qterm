import type { AgentRun } from './aiAgent'
import { normalizeContextBreakdown } from './ai'
import type { AIContextBreakdown, AIUsage } from './ai'
import type { ArchivedChat, StoredMessage } from './aiProviders'

export type AiConversationMode = 'ask' | 'agent'

export interface ConversationMessage extends StoredMessage {
  id: string

  at?: number
  mode?: AiConversationMode
  providerId?: string
  model?: string
  feedback?: 'up' | 'down'
  agentRun?: AgentRun
  manualStepIds?: string[]
  targetTitle?: string

  context?: AIContextBreakdown
}

export interface ConversationCheckpoint {
  messages: ConversationMessage[]
  createdAt: number
}

export interface AiConversationRecord {
  id: string

  hostId?: string
  sessionId: string
  sessionTitle: string
  title: string
  createdAt: number
  updatedAt: number

  providerId?: string
  model?: string
  messages: ConversationMessage[]

  checkpoint?: ConversationCheckpoint
}

export interface ConversationScope {
  hostId?: string
  sessionId?: string
  sessionTitle?: string
}

export interface ConversationPreferences {
  providerId?: string
  model?: string
}

const STORE_KEY = 'qterm.ai.conversations.v3'
const ACTIVE_KEY = 'qterm.ai.active-conversation.v1'
const HOST_ACTIVE_KEY = 'qterm.ai.active-conversation.v2'
const RECORD_LIMIT = 50
const MESSAGE_LIMIT = 100

function normalizeText(value: string | undefined): string {
  return value?.trim() ?? ''
}

function hostIdOf(value: { hostId?: string }): string {
  return normalizeText(value.hostId)
}

function sessionIdOf(value: { sessionId?: string }): string {
  return normalizeText(value.sessionId)
}

function sessionScopeId(sessionId: string | undefined): string {
  return sessionIdOf({ sessionId }) || '__qterm_unattached__'
}

function recordHostId(record: AiConversationRecord): string {
  return hostIdOf(record)
}

function belongsToScope(record: AiConversationRecord, scope: ConversationScope): boolean {
  const hostId = hostIdOf(scope)
  if (hostId) return recordHostId(record) === hostId
  return !recordHostId(record) && record.sessionId === sessionIdOf(scope)
}

function createId(prefix: string): string {
  return `${prefix}-${globalThis.crypto?.randomUUID?.() ?? `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`}`
}

function readRecords(): AiConversationRecord[] {
  try {
    const parsed = JSON.parse(window.localStorage.getItem(STORE_KEY) ?? '[]') as unknown
    if (!Array.isArray(parsed)) return []
    return parsed.filter(isRecord).slice(0, RECORD_LIMIT)
  } catch {
    return []
  }
}

function writeRecords(records: AiConversationRecord[]): boolean {
  try {
    window.localStorage.setItem(STORE_KEY, JSON.stringify(records.slice(0, RECORD_LIMIT)))
    return true
  } catch {
    return false
  }
}

function isRecord(value: unknown): value is AiConversationRecord {
  if (!value || typeof value !== 'object') return false
  const item = value as Partial<AiConversationRecord>
  return (
    typeof item.id === 'string' &&
    typeof item.sessionId === 'string' &&
    typeof item.sessionTitle === 'string' &&
    typeof item.title === 'string' &&
    typeof item.createdAt === 'number' &&
    typeof item.updatedAt === 'number' &&
    Array.isArray(item.messages)
  )
}

function normalizeMessages(messages: ConversationMessage[]): ConversationMessage[] {
  return messages
    .filter(
      (message) =>
        Boolean(message) &&
        (message.role === 'user' || message.role === 'assistant') &&
        typeof message.content === 'string',
    )
    .slice(-MESSAGE_LIMIT)
    .map((message) => {
      const context = normalizeContextBreakdown(message.context)
      return {
        ...message,

        images: undefined,
        id: typeof message.id === 'string' && message.id ? message.id : createId('aimsg'),
        ...(message.mode === 'agent' ? { mode: 'agent' as const } : { mode: 'ask' as const }),
        ...(message.agentRun && typeof message.agentRun.id === 'string'
          ? {
              agentRun: {
                ...message.agentRun,
                draft: '',
                steps: (message.agentRun.steps ?? []).map((step) => ({
                  ...step,
                  observation: step.observation ? step.observation.slice(0, 4096) : step.observation,
                })),
              },
            }
          : {}),
        ...(message.executions?.length
          ? {
              executions: message.executions.filter(
                (execution) =>
                  Number.isSafeInteger(execution.segmentIndex) &&
                  typeof execution.command === 'string' &&
                  typeof execution.target === 'string' &&
                  typeof execution.output === 'string',
              ),
            }
          : {}),
        ...(message.usage ? { usage: normalizeUsage(message.usage) } : {}),
        ...(context ? { context } : {}),
      }
    })
}

function normalizeUsage(usage: AIUsage): AIUsage | undefined {
  const fields = ['inputTokens', 'outputTokens', 'totalTokens', 'cacheReadTokens', 'cacheWriteTokens'] as const
  const result: AIUsage = {}
  for (const field of fields) {
    const value = usage[field]
    if (typeof value === 'number' && Number.isSafeInteger(value) && value >= 0) result[field] = value
  }
  return Object.keys(result).length ? result : undefined
}

function activeKey(scope: ConversationScope): string {
  const hostId = hostIdOf(scope)
  return hostId
    ? `${HOST_ACTIVE_KEY}.${encodeURIComponent(hostId)}`
    : `${ACTIVE_KEY}.${sessionScopeId(scope.sessionId)}`
}

function readActiveId(scope: ConversationScope): string {
  try {
    return window.localStorage.getItem(activeKey(scope)) ?? ''
  } catch {
    return ''
  }
}

function writeActiveId(scope: ConversationScope, id: string): boolean {
  try {
    window.localStorage.setItem(activeKey(scope), id)
    return true
  } catch {
    return false
  }
}

function toRecord(record: AiConversationRecord): AiConversationRecord {
  const { hostId, providerId, model, ...rest } = record
  const normalizedHostId = hostIdOf({ hostId })
  const normalizedProviderId = normalizeText(providerId)
  const normalizedModel = normalizeText(model)
  return {
    ...rest,
    ...(normalizedHostId ? { hostId: normalizedHostId } : {}),
    ...(normalizedProviderId ? { providerId: normalizedProviderId } : {}),
    ...(normalizedModel ? { model: normalizedModel } : {}),
    messages: normalizeMessages(record.messages),

    checkpoint:
      record.checkpoint && Array.isArray(record.checkpoint.messages) && record.checkpoint.messages.length
        ? {
            messages: normalizeMessages(record.checkpoint.messages),
            createdAt: typeof record.checkpoint.createdAt === 'number' ? record.checkpoint.createdAt : Date.now(),
          }
        : undefined,
  }
}

export function createConversation(
  scope: ConversationScope,
  title = '',
  preferences: ConversationPreferences = {},
): AiConversationRecord {
  const now = Date.now()
  const hostId = hostIdOf(scope)
  const providerId = normalizeText(preferences.providerId)
  const model = normalizeText(preferences.model)
  return {
    id: createId('aiconv'),
    ...(hostId ? { hostId } : {}),
    sessionId: sessionIdOf(scope),
    sessionTitle: normalizeText(scope.sessionTitle),
    title: title.trim() || 'AI',
    createdAt: now,
    updatedAt: now,
    ...(providerId ? { providerId } : {}),
    ...(model ? { model } : {}),
    messages: [],
  }
}

export function readActiveConversation(scope: ConversationScope): AiConversationRecord | null {
  const records = readRecords()
  const activeId = readActiveId(scope)
  const match =
    records.find((record) => record.id === activeId && belongsToScope(record, scope)) ??
    records.filter((record) => belongsToScope(record, scope)).sort((left, right) => right.updatedAt - left.updatedAt)[0]
  return match ? toRecord(match) : null
}

export function listConversations(scope: ConversationScope = {}): AiConversationRecord[] {
  return readRecords()
    .filter((record) => belongsToScope(record, scope))
    .sort((left, right) => right.updatedAt - left.updatedAt)
    .map(toRecord)
}

export function saveConversation(record: AiConversationRecord, activate = true): boolean {
  const next = toRecord({ ...record, updatedAt: Date.now() })
  const records = readRecords().filter((item) => item.id !== next.id)
  if (!writeRecords([next, ...records])) return false
  if (activate && !writeActiveId(next, next.id)) return false
  return true
}

export function activateConversation(scope: ConversationScope, id: string): AiConversationRecord | null {
  const selected = readRecords().find((record) => record.id === id && belongsToScope(record, scope))
  if (!selected) return null
  if (!writeActiveId(scope, selected.id)) return null
  return toRecord(selected)
}

export function readConversation(id: string, scope?: ConversationScope): AiConversationRecord | null {
  const record = readRecords().find((item) => item.id === id && (!scope || belongsToScope(item, scope)))
  return record ? toRecord(record) : null
}

export function listLegacyConversations(): AiConversationRecord[] {
  return readRecords()
    .filter((record) => !recordHostId(record))
    .sort((left, right) => right.updatedAt - left.updatedAt)
    .map(toRecord)
}

export function migrateSessionConversations(scope: ConversationScope): boolean {
  const hostId = hostIdOf(scope)
  const sessionId = sessionIdOf(scope)
  if (!hostId || !sessionId) return false

  const records = readRecords()
  let changed = false
  const migrated = records.map((record) => {
    if (recordHostId(record) || record.sessionId !== sessionId) return record
    changed = true
    return { ...record, hostId }
  })
  return !changed || writeRecords(migrated)
}

export function removeConversation(id: string): boolean {
  const records = readRecords()
  return writeRecords(records.filter((record) => record.id !== id))
}

export function appendMessage(record: AiConversationRecord, message: ConversationMessage): AiConversationRecord {
  return {
    ...record,
    messages: normalizeMessages([...record.messages, message]),
    updatedAt: Date.now(),
  }
}

export function makeMessage(
  message: Omit<ConversationMessage, 'id'> & Partial<Pick<ConversationMessage, 'id'>>,
): ConversationMessage {
  return { ...message, id: message.id || createId('aimsg'), at: message.at ?? Date.now() }
}

export function readLegacyAgentChats(providerId: string): ArchivedChat[] {
  try {
    const runs: unknown = JSON.parse(window.localStorage.getItem(`qterm.ai.agent.runs.v1.${providerId}`) ?? '[]')
    if (!Array.isArray(runs)) return []
    return runs.flatMap((run: Partial<AgentRun>) => {
      if (!run || typeof run.id !== 'string' || typeof run.prompt !== 'string' || !Array.isArray(run.steps)) return []
      let answer = [...run.steps].reverse().find((step) => step?.kind === 'final_answer')?.observation ?? ''

      if (answer.trim().startsWith('{')) {
        try {
          const payload = JSON.parse(answer)
          if (payload?.action === 'final_answer' && typeof payload.arguments?.answer === 'string') {
            answer = payload.arguments.answer
          }
        } catch {}
      }
      const output = run.steps
        .filter((step) => step?.kind === 'execute_command')
        .map((step) =>
          [step.command, step.observation, step.exitCode == null ? '' : `exit ${step.exitCode}`]
            .filter(Boolean)
            .join('\n'),
        )
        .join('\n\n')
      return [
        {
          id: `legacy-agent:${providerId}:${run.id}`,
          title: run.prompt.slice(0, 60),
          updatedAt: Date.parse(run.updatedAt ?? '') || 0,
          messages: [
            { role: 'user' as const, content: run.prompt },
            { role: 'assistant' as const, content: answer, output },
          ],
        },
      ]
    })
  } catch {
    return []
  }
}
