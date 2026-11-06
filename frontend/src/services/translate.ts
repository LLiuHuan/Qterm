import {
  DeleteTranslationProviderCredential as deleteCredentialBinding,
  ListTranslationProviderCredentials as listCredentialsBinding,
  SaveTranslationProviderCredential as saveCredentialBinding,
  TranslateLanguages as languagesBinding,
  TranslateText as translateBinding,
} from '../../bindings/changeme/internal/app/service'
import { createRuntimeGateway } from './runtime'

export type TranslateProvider = 'google' | 'microsoft' | 'deepl' | 'baidu' | 'ali' | 'youdao' | 'libretranslate'

export type TranslateCredentialProvider = 'deepl' | 'baidu' | 'ali' | 'youdao'

export const TRANSLATE_CREDENTIAL_PROVIDERS: readonly TranslateCredentialProvider[] = [
  'deepl',
  'baidu',
  'ali',
  'youdao',
]

export const TRANSLATE_KEYLESS_PROVIDERS: readonly TranslateProvider[] = ['google', 'microsoft']

export const TRANSLATE_PROVIDER_ORDER: readonly TranslateProvider[] = [
  'google',
  'microsoft',
  'deepl',
  'baidu',
  'ali',
  'youdao',
  'libretranslate',
]

export function isTranslateProvider(value: unknown): value is TranslateProvider {
  return (
    value === 'google' ||
    value === 'microsoft' ||
    value === 'deepl' ||
    value === 'baidu' ||
    value === 'ali' ||
    value === 'youdao' ||
    value === 'libretranslate'
  )
}

export function isTranslateCredentialProvider(value: unknown): value is TranslateCredentialProvider {
  return TRANSLATE_CREDENTIAL_PROVIDERS.includes(value as TranslateCredentialProvider)
}

export function isKeylessTranslateProvider(provider: TranslateProvider): boolean {
  return TRANSLATE_KEYLESS_PROVIDERS.includes(provider)
}

export function requiresTranslateEndpoint(provider: TranslateProvider): boolean {
  return provider === 'libretranslate'
}

export interface TranslateResult {
  text: string
  detectedLanguage: string
}

export interface TranslateGateway {
  readonly persistent: boolean

  languages(): Promise<string[]>
  translate(text: string, target: string, provider: TranslateProvider, endpoint: string): Promise<TranslateResult>
}

export interface TranslateProviderCredentialStatus {
  provider: TranslateCredentialProvider
  configured: boolean
}

export interface TranslateProviderCredentialInput {
  provider: TranslateCredentialProvider
  apiKey?: string
  appId?: string
  appKey?: string
}

export interface TranslateCredentialGateway {
  readonly persistent: boolean
  list(): Promise<TranslateProviderCredentialStatus[]>
  save(input: TranslateProviderCredentialInput): Promise<void>
  remove(provider: TranslateCredentialProvider): Promise<void>
}

class WailsTranslateGateway implements TranslateGateway {
  readonly persistent = true

  async languages(): Promise<string[]> {
    return (await languagesBinding()) ?? []
  }

  async translate(
    text: string,
    target: string,
    provider: TranslateProvider,
    endpoint: string,
  ): Promise<TranslateResult> {
    return translateBinding(text, target, provider, endpoint)
  }
}

class MemoryTranslateGateway implements TranslateGateway {
  readonly persistent = false

  async languages(): Promise<string[]> {
    return ['zh-CN', 'en', 'ja']
  }

  async translate(text: string, target: string): Promise<TranslateResult> {
    return {
      text: `[preview → ${target}] ${text}`,
      detectedLanguage: '',
    }
  }
}

class MemoryTranslateCredentialGateway implements TranslateCredentialGateway {
  readonly persistent = false
  private readonly configured = new Set<TranslateCredentialProvider>()

  async list(): Promise<TranslateProviderCredentialStatus[]> {
    return TRANSLATE_CREDENTIAL_PROVIDERS.map((provider) => ({
      provider,
      configured: this.configured.has(provider),
    }))
  }

  async save(input: TranslateProviderCredentialInput): Promise<void> {
    if (!isTranslateCredentialProvider(input.provider)) throw new Error('unsupported translate provider')
    if (![input.apiKey, input.appId, input.appKey].some((value) => value?.trim())) {
      throw new Error('translation credential is empty')
    }
    this.configured.add(input.provider)
  }

  async remove(provider: TranslateCredentialProvider): Promise<void> {
    this.configured.delete(provider)
  }
}

class WailsTranslateCredentialGateway implements TranslateCredentialGateway {
  readonly persistent = true

  async list(): Promise<TranslateProviderCredentialStatus[]> {
    const statuses = (await listCredentialsBinding()) ?? []
    return statuses.flatMap((candidate): TranslateProviderCredentialStatus[] => {
      const provider = candidate.provider
      if (!isTranslateCredentialProvider(provider)) return []
      return [{ provider, configured: candidate.configured === true }]
    })
  }

  async save(input: TranslateProviderCredentialInput): Promise<void> {
    await saveCredentialBinding(input.provider, {
      apiKey: input.apiKey?.trim() ?? '',
      appId: input.appId?.trim() ?? '',
      appKey: input.appKey?.trim() ?? '',
    })
  }

  async remove(provider: TranslateCredentialProvider): Promise<void> {
    await deleteCredentialBinding(provider)
  }
}

export function createTranslateGateway(): TranslateGateway {
  return createRuntimeGateway<TranslateGateway>(
    () => new WailsTranslateGateway(),
    () => new MemoryTranslateGateway(),
  )
}

export function createTranslateCredentialGateway(): TranslateCredentialGateway {
  return createRuntimeGateway<TranslateCredentialGateway>(
    () => new WailsTranslateCredentialGateway(),
    () => new MemoryTranslateCredentialGateway(),
  )
}
