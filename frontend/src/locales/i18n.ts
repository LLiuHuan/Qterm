import { computed, readonly, ref } from 'vue'
import { SUPPORTED_LOCALES, messages, type AppLocale } from './messages'

export { SUPPORTED_LOCALES } from './messages'
export type { AppLocale } from './messages'

export const LOCALE_STORAGE_KEY = 'qterm.locale.v1'

const LOCALE_NAME_KEYS: Record<AppLocale, string> = {
  'zh-CN': 'locale.zhCN',
  'zh-Hant': 'locale.zhHant',
  'en-US': 'locale.enUS',
  'ko-KR': 'locale.koKR',
  'ja-JP': 'locale.jaJP',
  'ru-RU': 'locale.ruRU',
  'fr-FR': 'locale.frFR',
}

export function isSupportedLocale(value: unknown): value is AppLocale {
  return typeof value === 'string' && (SUPPORTED_LOCALES as readonly string[]).includes(value)
}

export const LOCALE_OPTIONS: readonly { value: AppLocale; labelKey: string }[] = SUPPORTED_LOCALES.map((value) => ({
  value,
  labelKey: LOCALE_NAME_KEYS[value],
}))

const locale = ref<AppLocale>(readLocale())

function storage(): Storage | null {
  try {
    return globalThis.localStorage ?? null
  } catch {
    return null
  }
}

function readLocale(): AppLocale {
  const stored = storage()?.getItem(LOCALE_STORAGE_KEY)

  return isSupportedLocale(stored) ? stored : 'zh-CN'
}

function updateDocumentLocale(value: AppLocale): void {
  if (typeof document !== 'undefined') document.documentElement.lang = value
}

updateDocumentLocale(locale.value)

export function currentLocale(): AppLocale {
  return locale.value
}

export function setLocale(value: AppLocale): void {
  if (!isSupportedLocale(value)) return
  locale.value = value
  storage()?.setItem(LOCALE_STORAGE_KEY, value)
  updateDocumentLocale(value)
  if (typeof window !== 'undefined') window.dispatchEvent(new CustomEvent('qterm:locale-changed', { detail: value }))
}

export function translate(key: string, params: Record<string, string | number> = {}): string {
  const template = messages[locale.value][key] ?? messages['en-US'][key] ?? messages['zh-CN'][key] ?? key
  return template.replace(/\{(\w+)\}/g, (match, name: string) =>
    Object.prototype.hasOwnProperty.call(params, name) ? String(params[name]) : match,
  )
}

export function useI18n() {
  return {
    locale: readonly(locale),
    localeName: computed(() => translate(LOCALE_NAME_KEYS[locale.value])),
    locales: SUPPORTED_LOCALES,
    t: translate,
    setLocale,
  }
}
