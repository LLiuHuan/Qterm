import { errorMessage } from './errors'

export type AIProviderType = import('../../bindings/changeme/internal/ai/models').ProviderType

export interface AIMessage {
  role: 'system' | 'user' | 'assistant'
  content: string

  images?: string[]
}

export interface AIChatInput {
  providerId: string
  type: AIProviderType
  baseUrl: string
  model: string
  messages: AIMessage[]
  contextText?: string
}

export async function providerKeyConfigured(providerId: string): Promise<boolean> {
  if (!providerId) return false
  const { AIProviderKeyConfigured } = await import('../../bindings/changeme/internal/app/service')
  return Boolean(await AIProviderKeyConfigured(providerId))
}

export async function saveProviderKey(providerId: string, key: string): Promise<void> {
  const { SaveAIProviderKey } = await import('../../bindings/changeme/internal/app/service')
  await SaveAIProviderKey(providerId, key)
}

export async function clearProviderKey(providerId: string): Promise<void> {
  const { ClearAIProviderKey } = await import('../../bindings/changeme/internal/app/service')
  await ClearAIProviderKey(providerId)
}

export async function listProviderModels(input: {
  providerId: string
  type: AIProviderType
  baseUrl: string
}): Promise<string[]> {
  const { ListAIModels } = await import('../../bindings/changeme/internal/app/service')
  const models = (await ListAIModels(input)) as string[] | null
  return models ?? []
}

export interface AIProviderPreviewInput {
  providerId: string
  type: AIProviderType
  baseUrl: string
  model: string
  draftKey: string
}

export async function previewProviderModels(input: AIProviderPreviewInput): Promise<string[]> {
  const { ListAIModelsPreview } = await import('../../bindings/changeme/internal/app/service')
  return (await ListAIModelsPreview(input)) ?? []
}

export function previewProviderConnection(input: AIProviderPreviewInput): {
  promise: Promise<AIChatOutcome>
  cancel: () => void
} {
  let cancelled = false
  let cancelPending: (() => void) | undefined
  const promise = (async () => {
    const { TestAIProvider } = await import('../../bindings/changeme/internal/app/service')
    if (cancelled) throw new Error('AI provider preview cancelled')
    const pending = TestAIProvider(input)
    cancelPending = () => pending.cancel()
    const result = await pending
    return { reply: result.reply ?? '', errorKey: result.errorKey ?? '', errorDetail: result.errorDetail ?? '' }
  })()
  return {
    promise,
    cancel: () => {
      cancelled = true
      cancelPending?.()
    },
  }
}

export interface AIChatOutcome {
  reply: string

  errorKey: string

  errorDetail: string
}

export interface AIBackgroundCommandOutcome {
  output: string
  exitCode: number
}

export function runAIBackgroundCommand(
  sessionId: string,
  command: string,
): { promise: Promise<AIBackgroundCommandOutcome>; cancel: () => void } {
  let cancelRequest: (() => void) | null = null
  let cancelled = false
  const promise = (async () => {
    const { RunAIBackgroundCommand } = await import('../../bindings/changeme/internal/app/service')
    if (cancelled) throw new Error('AI background command cancelled')
    const pending = RunAIBackgroundCommand(sessionId, command)
    cancelRequest = () => pending.cancel()
    if (cancelled) cancelRequest()
    return await pending
  })()
  return {
    promise,
    cancel: () => {
      cancelled = true
      cancelRequest?.()
    },
  }
}

export function chatWithAI(input: AIChatInput): { promise: Promise<AIChatOutcome>; cancel: () => void } {
  let cancelRequest: (() => void) | null = null
  const promise = (async () => {
    const { AIChat } = await import('../../bindings/changeme/internal/app/service')
    const pending = AIChat({
      providerId: input.providerId,
      type: input.type,
      baseUrl: input.baseUrl,
      model: input.model,
      messages: input.messages,
      contextText: input.contextText ?? '',
    }) as unknown as Promise<{ reply?: string; errorKey?: string; errorDetail?: string } | null> & {
      cancel?: () => void
    }
    cancelRequest = () => pending.cancel?.()
    const result = (await pending) ?? {}
    return {
      reply: result.reply ?? '',
      errorKey: result.errorKey ?? '',
      errorDetail: result.errorDetail ?? '',
    }
  })()
  return { promise, cancel: () => cancelRequest?.() }
}

export function aiErrorMessage(cause: unknown, fallback: string): string {
  return errorMessage(cause, fallback)
}

export interface AIStreamPoll {
  content: string

  reasoning: string
  done: boolean
  errorKey: string
  errorDetail: string

  usage?: AIUsage
}

export interface AIUsage {
  inputTokens?: number
  outputTokens?: number
  totalTokens?: number
  cacheReadTokens?: number
  cacheWriteTokens?: number
}

function normalizeAIUsage(value: unknown): AIUsage | undefined {
  if (!value || typeof value !== 'object') return undefined
  const raw = value as Record<string, unknown>
  const count = (key: keyof AIUsage): number | undefined => {
    const value = raw[key]
    return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0 ? value : undefined
  }
  const usage: AIUsage = {
    ...(count('inputTokens') !== undefined ? { inputTokens: count('inputTokens') } : {}),
    ...(count('outputTokens') !== undefined ? { outputTokens: count('outputTokens') } : {}),
    ...(count('totalTokens') !== undefined ? { totalTokens: count('totalTokens') } : {}),
    ...(count('cacheReadTokens') !== undefined ? { cacheReadTokens: count('cacheReadTokens') } : {}),
    ...(count('cacheWriteTokens') !== undefined ? { cacheWriteTokens: count('cacheWriteTokens') } : {}),
  }
  return Object.keys(usage).length ? usage : undefined
}

export interface AIStreamSession {
  streamId: string

  context?: AIContextBreakdown | null

  onUpdate: (poll: AIStreamPoll) => void
  stop: () => void
}

export type AIContextBreakdown = import('../../bindings/changeme/internal/ai/models').ContextBreakdown

function normalizeCount(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0 ? Math.round(value) : 0
}

export function normalizeContextBreakdown(value: unknown): AIContextBreakdown | null {
  if (!value || typeof value !== 'object') return null
  const raw = value as Record<string, unknown>
  const breakdown: AIContextBreakdown = {
    system: normalizeCount(raw.system),
    tools: normalizeCount(raw.tools),
    messages: normalizeCount(raw.messages),
    context: normalizeCount(raw.context),
    limit: normalizeCount(raw.limit),
  }
  if (breakdown.system + breakdown.tools + breakdown.messages + breakdown.context === 0) return null
  return breakdown
}

const STREAM_POLL_MS = 60

export interface AIContextCompaction {
  compacted: boolean
  reason?: string
  summary?: string
  keptMessages?: AIMessage[]
  beforeBytes: number
  afterBytes: number
}

export async function compactAIConversation(
  input: AIChatInput,
  keepRecent: number,
  priorSummary = '',
): Promise<AIContextCompaction> {
  const { CompactAIConversation } = await import('../../bindings/changeme/internal/app/service')
  const result = await CompactAIConversation(
    {
      providerId: input.providerId,
      type: input.type,
      baseUrl: input.baseUrl,
      model: input.model,
      messages: input.messages,
      contextText: '',
    },
    keepRecent,
    priorSummary,
  )
  return {
    compacted: Boolean(result?.compacted),
    reason: result?.reason || undefined,
    summary: result?.summary || undefined,
    keptMessages: (result?.keptMessages ?? []) as AIMessage[],
    beforeBytes: result?.beforeBytes ?? 0,
    afterBytes: result?.afterBytes ?? 0,
  }
}

export async function startChatStream(
  input: AIChatInput,
  onUpdate: (poll: AIStreamPoll) => void,
): Promise<AIStreamSession> {
  const { AIChatStream, AIChatStreamPoll, AIChatStreamCancel, AIChatStreamRelease } =
    await import('../../bindings/changeme/internal/app/service')
  const started = (await AIChatStream({
    providerId: input.providerId,
    type: input.type,
    baseUrl: input.baseUrl,
    model: input.model,
    messages: input.messages,
    contextText: input.contextText ?? '',
  })) as { streamId?: string; errorKey?: string; context?: unknown } | null
  const streamId = started?.streamId ?? ''
  if (!streamId) throw new Error(started?.errorKey || 'ai.stream_failed')
  const context = normalizeContextBreakdown(started?.context)

  let stopped = false
  let timer: ReturnType<typeof setTimeout> | null = null

  const finish = (poll: AIStreamPoll): void => {
    onUpdate(poll)
    void AIChatStreamRelease(streamId)
  }

  const tick = async (): Promise<void> => {
    if (stopped) return
    try {
      const poll = (await AIChatStreamPoll(streamId)) as {
        content?: string
        reasoning?: string
        done?: boolean
        errorKey?: string
        errorDetail?: string
        usage?: unknown
      } | null
      const next: AIStreamPoll = {
        content: poll?.content ?? '',
        reasoning: poll?.reasoning ?? '',
        done: Boolean(poll?.done),
        errorKey: poll?.errorKey ?? '',
        errorDetail: poll?.errorDetail ?? '',
        usage: normalizeAIUsage(poll?.usage),
      }
      if (next.done) {
        finish(next)
        return
      }
      onUpdate(next)
    } catch {}
    timer = setTimeout(() => void tick(), STREAM_POLL_MS)
  }

  void tick()

  return {
    streamId,
    context,
    onUpdate,
    stop: () => {
      stopped = true
      if (timer) clearTimeout(timer)
      void AIChatStreamCancel(streamId)
    },
  }
}
