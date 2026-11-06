export interface CredentialInput {
  apiKey: string
  appId: string
  appKey: string
}

export interface CredentialStatus {
  provider: Provider
  configured: boolean
}

export enum Provider {
  $zero = '',

  ProviderGoogle = 'google',

  ProviderMicrosoft = 'microsoft',

  ProviderDeepL = 'deepl',

  ProviderBaidu = 'baidu',

  ProviderAli = 'ali',

  ProviderYoudao = 'youdao',

  ProviderLibreTranslate = 'libretranslate',
}

export interface Result {
  text: string
  detectedLanguage: string
}
