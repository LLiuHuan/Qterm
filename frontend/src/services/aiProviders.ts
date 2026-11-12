import { ProviderType } from '../../bindings/changeme/internal/ai/models'
import type { AIProviderType, AIUsage } from './ai'

export interface AIProvider {
  id: string
  name: string

  preset: string
  type: AIProviderType
  baseUrl: string

  model: string

  models: string[]

  enabled: boolean
}

export interface AIProviderPreset {
  id: string
  label: string

  type: AIProviderType

  baseUrl: string
}

export const AI_PROVIDER_PRESETS: AIProviderPreset[] = [
  {
    id: 'openai',
    label: 'OpenAI',
    type: ProviderType.ProviderTypeOpenAI,
    baseUrl: 'https://api.openai.com/v1',
  },
  {
    id: 'anthropic',
    label: 'Anthropic',
    type: ProviderType.ProviderTypeAnthropic,
    baseUrl: 'https://api.anthropic.com/v1',
  },
  {
    id: 'deepseek',
    label: 'DeepSeek',
    type: ProviderType.ProviderTypeOpenAI,
    baseUrl: 'https://api.deepseek.com',
  },
  {
    id: 'ollama',
    label: 'Ollama',
    type: ProviderType.ProviderTypeOpenAI,
    baseUrl: 'http://localhost:11434/v1',
  },
  {
    id: 'xai',
    label: 'xAI',
    type: ProviderType.ProviderTypeOpenAI,
    baseUrl: 'https://api.x.ai/v1',
  },
  {
    id: 'cohere',
    label: 'Cohere',
    type: ProviderType.ProviderTypeOpenAI,
    baseUrl: 'https://api.cohere.com/compatibility/v1',
  },
  {
    id: 'mimo',
    label: 'Mimo (小米)',
    type: ProviderType.ProviderTypeOpenAI,
    baseUrl: 'https://api.xiaomimimo.com/v1',
  },
  {
    id: 'zai',
    label: 'BigModel',
    type: ProviderType.ProviderTypeOpenAI,
    baseUrl: 'https://open.bigmodel.cn/api/paas/v4',
  },
  {
    id: 'kimi',
    label: 'Kimi',
    type: ProviderType.ProviderTypeOpenAI,
    baseUrl: 'https://api.moonshot.ai/v1',
  },
  {
    id: 'qwen',
    label: 'Qwen',
    type: ProviderType.ProviderTypeOpenAI,
    baseUrl: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
  },
  {
    id: 'opencode',
    label: 'OpenCode',
    type: ProviderType.ProviderTypeOpenAI,
    baseUrl: 'https://opencode.ai/zen/go/v1',
  },
  {
    id: 'openrouter',
    label: 'OpenRouter',
    type: ProviderType.ProviderTypeOpenAI,
    baseUrl: 'https://openrouter.ai/api/v1',
  },

  {
    id: 'custom',
    label: '自定义',
    type: ProviderType.ProviderTypeOpenAI,
    baseUrl: '',
  },
]

const PROVIDERS_KEY = 'qterm.ai.providers.v1'
const ACTIVE_KEY = 'qterm.ai.active-provider.v1'

function createProviderID(): string {
  return `ai-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
}

export function readProviders(): AIProvider[] {
  try {
    const raw = window.localStorage.getItem(PROVIDERS_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw) as unknown
    if (!Array.isArray(parsed)) return []
    const providers: AIProvider[] = []
    for (const item of parsed) {
      if (!item || typeof item !== 'object') continue
      const provider = item as Partial<AIProvider>
      if (
        typeof provider.id !== 'string' ||
        typeof provider.name !== 'string' ||
        typeof provider.type !== 'string' ||
        typeof provider.baseUrl !== 'string' ||
        typeof provider.model !== 'string' ||
        !Array.isArray(provider.models)
      ) {
        continue
      }
      providers.push({
        id: provider.id,
        name: provider.name,
        preset: typeof provider.preset === 'string' ? provider.preset : 'custom',
        type: provider.type,
        baseUrl: normalizeProviderBaseUrl(provider.type, provider.baseUrl),
        model: provider.model,
        models: provider.models.filter((value): value is string => typeof value === 'string'),

        enabled: provider.enabled !== false,
      })
    }
    return providers
  } catch {
    return []
  }
}

export function writeProviders(providers: AIProvider[]): void {
  try {
    window.localStorage.setItem(PROVIDERS_KEY, JSON.stringify(providers))
  } catch {}
}

export function readActiveProviderID(): string {
  try {
    return window.localStorage.getItem(ACTIVE_KEY) ?? ''
  } catch {
    return ''
  }
}

export function writeActiveProviderID(id: string): void {
  try {
    window.localStorage.setItem(ACTIVE_KEY, id)
  } catch {}
}

function normalizeProviderBaseUrl(type: AIProviderType, baseUrl: string): string {
  if (type !== ProviderType.ProviderTypeAnthropic) return baseUrl
  const base = baseUrl.trim().replace(/\/+$/, '')
  if (!base) return baseUrl
  return /^https?:\/\/[^/]+$/.test(base) ? `${base}/v1` : base
}

export function createProviderFromPreset(preset: AIProviderPreset): AIProvider {
  return {
    id: createProviderID(),
    name: preset.id === 'custom' ? '' : preset.label,
    preset: preset.id,
    type: preset.type,
    baseUrl: preset.baseUrl,
    model: '',
    models: [],
    enabled: true,
  }
}

export function findProvider(providers: AIProvider[], id: string): AIProvider | undefined {
  return providers.find((provider) => provider.id === id)
}

const HISTORY_KEY = 'qterm.ai.history.v1'
const CONVERSATIONS_KEY = 'qterm.ai.conversations.v2'

const HISTORY_LIMIT = 100
const ARCHIVE_LIMIT = 20

export interface StoredMessage {
  role: 'user' | 'assistant'
  content: string

  reasoning?: string

  output?: string

  executions?: Array<{
    segmentIndex: number
    command: string
    target: string
    output: string
    exitCode?: number
    outcome?: 'unknown' | 'not_sent'
  }>

  usage?: AIUsage

  compacted?: boolean

  compactionGeneration?: number
}

export interface ArchivedChat {
  modelLabel?: string
  legacy?: boolean
  id: string
  title: string
  updatedAt: number
  messages: StoredMessage[]
}

interface ChatStore {
  active: StoredMessage[]
  archived: ArchivedChat[]
}

function validExecutions(value: unknown): StoredMessage['executions'] {
  if (!Array.isArray(value)) return undefined
  return value
    .filter((entry): entry is NonNullable<StoredMessage['executions']>[number] =>
      Boolean(
        entry &&
        typeof entry === 'object' &&
        Number.isSafeInteger(entry.segmentIndex) &&
        entry.segmentIndex >= 0 &&
        typeof entry.command === 'string' &&
        typeof entry.target === 'string' &&
        typeof entry.output === 'string',
      ),
    )
    .slice(-20)
    .map((entry) => ({
      segmentIndex: entry.segmentIndex,
      command: entry.command,
      target: entry.target,
      output: entry.output,
      ...(Number.isSafeInteger(entry.exitCode) ? { exitCode: entry.exitCode } : {}),
      ...(entry.outcome === 'unknown' || entry.outcome === 'not_sent' ? { outcome: entry.outcome } : {}),
    }))
}

function validMessages(value: unknown): StoredMessage[] {
  if (!Array.isArray(value)) return []
  return value
    .filter((item): item is StoredMessage => {
      if (!item || typeof item !== 'object') return false
      const message = item as Partial<StoredMessage>
      return (message.role === 'user' || message.role === 'assistant') && typeof message.content === 'string'
    })
    .slice(-HISTORY_LIMIT)
    .map((message) => {
      message = { ...message, executions: validExecutions(message.executions) }
      const usage = message.usage
      const validCount = (value: unknown): value is number =>
        typeof value === 'number' && Number.isSafeInteger(value) && value >= 0
      if (!usage || typeof usage !== 'object') {
        const clean = { ...message }
        delete clean.usage
        return clean
      }
      const counts: AIUsage = {
        ...(validCount(usage.inputTokens) ? { inputTokens: usage.inputTokens } : {}),
        ...(validCount(usage.outputTokens) ? { outputTokens: usage.outputTokens } : {}),
        ...(validCount(usage.totalTokens) ? { totalTokens: usage.totalTokens } : {}),
        ...(validCount(usage.cacheReadTokens) ? { cacheReadTokens: usage.cacheReadTokens } : {}),
        ...(validCount(usage.cacheWriteTokens) ? { cacheWriteTokens: usage.cacheWriteTokens } : {}),
      }
      if (!Object.keys(counts).length) {
        const clean = { ...message }
        delete clean.usage
        return clean
      }
      return {
        ...message,
        usage: counts,
      }
    })
}

function readChatStore(providerID: string): ChatStore {
  if (!providerID) return { active: [], archived: [] }
  try {
    const raw = window.localStorage.getItem(`${CONVERSATIONS_KEY}.${providerID}`)
    if (raw) {
      const parsed = JSON.parse(raw) as unknown
      if (parsed && typeof parsed === 'object') {
        const store = parsed as Partial<ChatStore>
        return {
          active: validMessages(store.active),
          archived: Array.isArray(store.archived)
            ? store.archived
                .filter((item): item is ArchivedChat =>
                  Boolean(
                    item &&
                    typeof item.id === 'string' &&
                    typeof item.title === 'string' &&
                    typeof item.updatedAt === 'number' &&
                    Array.isArray(item.messages),
                  ),
                )
                .slice(0, ARCHIVE_LIMIT)
                .map((item) => ({ ...item, messages: validMessages(item.messages) }))
            : [],
        }
      }
    }

    return {
      active: validMessages(JSON.parse(window.localStorage.getItem(`${HISTORY_KEY}.${providerID}`) ?? '[]')),
      archived: [],
    }
  } catch {
    return { active: [], archived: [] }
  }
}

function writeChatStore(providerID: string, store: ChatStore): boolean {
  if (!providerID) return false
  try {
    window.localStorage.setItem(`${CONVERSATIONS_KEY}.${providerID}`, JSON.stringify(store))
    return true
  } catch {
    return false
  }
}

function archive(messages: StoredMessage[]): ArchivedChat {
  return {
    id: globalThis.crypto?.randomUUID?.() ?? `chat-${Date.now()}-${Math.random().toString(36).slice(2)}`,
    title:
      messages
        .find((message) => message.role === 'user')
        ?.content.trim()
        .split('\n')[0]
        ?.slice(0, 60) || '…',
    updatedAt: Date.now(),
    messages: messages.slice(-HISTORY_LIMIT),
  }
}

export function readHistory(providerID: string): StoredMessage[] {
  return readChatStore(providerID).active
}

export function readArchivedChats(providerID: string): ArchivedChat[] {
  return readChatStore(providerID).archived
}

export function writeHistory(providerID: string, messages: StoredMessage[]): boolean {
  const store = readChatStore(providerID)
  return writeChatStore(providerID, { ...store, active: messages.slice(-HISTORY_LIMIT) })
}

export function startNewChat(providerID: string, messages: StoredMessage[]): boolean {
  const store = readChatStore(providerID)
  return writeChatStore(providerID, {
    active: [],
    archived: messages.length ? [archive(messages), ...store.archived].slice(0, ARCHIVE_LIMIT) : store.archived,
  })
}

export function openArchivedChat(providerID: string, id: string, messages: StoredMessage[]): StoredMessage[] | null {
  const store = readChatStore(providerID)
  const selected = store.archived.find((item) => item.id === id)
  if (!selected) return null
  const archived = store.archived.filter((item) => item.id !== id)
  const next: ChatStore = {
    active: selected.messages,
    archived: (messages.length ? [archive(messages), ...archived] : archived).slice(0, ARCHIVE_LIMIT),
  }
  return writeChatStore(providerID, next) ? selected.messages : null
}

export function clearArchivedChats(providerID: string): boolean {
  const store = readChatStore(providerID)
  return writeChatStore(providerID, { ...store, archived: [] })
}
