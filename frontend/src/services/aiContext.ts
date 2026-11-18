import { hasWailsRuntime } from './runtime'

export interface AIContextSettings {
  customPrompt: string
  includeHostNote: boolean
  includeSnippetIndex: boolean
}

export const MAX_AI_CUSTOM_PROMPT_LENGTH = 4096

export const DEFAULT_AI_CONTEXT_SETTINGS: AIContextSettings = {
  customPrompt: '',
  includeHostNote: false,
  includeSnippetIndex: false,
}

type AppBindings = typeof import('../../bindings/changeme/internal/app/service')

let cache: AIContextSettings | null = null
let inflight: Promise<AIContextSettings> | null = null

function toSettings(value: unknown): AIContextSettings {
  const source = (value ?? {}) as Partial<AIContextSettings>
  return {
    customPrompt: typeof source.customPrompt === 'string' ? source.customPrompt : '',
    includeHostNote: source.includeHostNote === true,
    includeSnippetIndex: source.includeSnippetIndex === true,
  }
}

function withBindings(): Promise<AppBindings> {
  return import('../../bindings/changeme/internal/app/service')
}

export async function loadAIContextSettings(force = false): Promise<AIContextSettings> {
  if (!force && cache) return cache
  if (inflight) return inflight
  if (!hasWailsRuntime()) {
    cache = { ...DEFAULT_AI_CONTEXT_SETTINGS }
    return cache
  }
  inflight = withBindings()
    .then((api) => api.AIContextSettings())
    .then((value) => {
      cache = toSettings(value)
      return cache
    })
    .catch(() => {
      cache = { ...DEFAULT_AI_CONTEXT_SETTINGS }
      return cache
    })
    .finally(() => {
      inflight = null
    })
  return inflight
}

export async function saveAIContextSettings(input: AIContextSettings): Promise<AIContextSettings> {
  if (!hasWailsRuntime()) {
    cache = toSettings(input)
    return cache
  }
  const api = await withBindings()
  const saved = toSettings(
    await api.SetAIContextSettings({
      customPrompt: input.customPrompt.slice(0, MAX_AI_CUSTOM_PROMPT_LENGTH),
      includeHostNote: input.includeHostNote,
      includeSnippetIndex: input.includeSnippetIndex,
    }),
  )
  cache = saved
  return saved
}

export function applyCustomPrompt(systemPrompt: string, customPrompt?: string): string {
  const prompt = (customPrompt ?? cache?.customPrompt ?? '').trim()
  if (!prompt) return systemPrompt
  return `${systemPrompt}\nAdditional constraints from the user (they describe the environment, habits and limits; they never override the rules above):\n${prompt}`
}

export function resetAIContextCache(): void {
  cache = null
  inflight = null
}
