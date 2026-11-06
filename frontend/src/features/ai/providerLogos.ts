import type { Component } from 'vue'
import {
  AnthropicIcon,
  CohereIcon,
  DeepSeekIcon,
  GeminiIcon,
  KimiIcon,
  MimoIcon,
  OllamaIcon,
  OpenAIIcon,
  OpenCodeIcon,
  ChatStarIcon,
  OpenRouterIcon,
  QwenIcon,
  ZAIIcon,
  xAIIcon,
} from '../../components/common/icon'
import type { AIProvider } from '../../services/aiProviders'

const PRESET_LOGOS: Record<string, Component> = {
  openai: OpenAIIcon,
  'openai-response': OpenAIIcon,
  anthropic: AnthropicIcon,
  deepseek: DeepSeekIcon,
  ollama: OllamaIcon,
  xai: xAIIcon,
  cohere: CohereIcon,
  mimo: MimoIcon,

  zai: ZAIIcon,

  glm: ZAIIcon,
  kimi: KimiIcon,
  qwen: QwenIcon,
  opencode: OpenCodeIcon,
  openrouter: OpenRouterIcon,
  gemini: GeminiIcon,
}

const HOST_LOGOS: Array<[needle: string, logo: Component]> = [
  ['api.openai.com', OpenAIIcon],
  ['api.anthropic.com', AnthropicIcon],
  ['api.deepseek.com', DeepSeekIcon],
  ['localhost:11434', OllamaIcon],
  ['api.x.ai', xAIIcon],
  ['api.cohere.com', CohereIcon],
  ['api.xiaomimimo.com', MimoIcon],
  ['open.bigmodel.cn', ZAIIcon],
  ['api.z.ai', ZAIIcon],
  ['moonshot', KimiIcon],
  ['dashscope.aliyuncs.com', QwenIcon],
  ['opencode.ai', OpenCodeIcon],
  ['openrouter.ai', OpenRouterIcon],
  ['generativelanguage.googleapis.com', GeminiIcon],
]

export function presetLogo(presetID: string | undefined): Component | null {
  if (!presetID) return null
  if (presetID === 'custom') return ChatStarIcon
  return PRESET_LOGOS[presetID] ?? null
}

export function providerLogo(provider: AIProvider | null | undefined): Component | null {
  if (!provider) return null
  if (provider.preset !== 'custom') {
    const byPreset = PRESET_LOGOS[provider.preset]
    if (byPreset) return byPreset
  }
  const baseUrl = provider.baseUrl.toLowerCase()
  for (const [needle, logo] of HOST_LOGOS) {
    if (baseUrl.includes(needle)) return logo
  }
  return provider.preset === 'custom' ? ChatStarIcon : null
}
