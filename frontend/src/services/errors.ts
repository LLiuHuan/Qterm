import { errorMessages } from '../locales/errors'

export interface AppError {
  code: string
  messageKey: string
  safeDetail?: string
}

function currentLocale(): string {
  return typeof document === 'undefined' ? 'zh-CN' : document.documentElement.lang || 'zh-CN'
}

export function decodeAppError(value: unknown, depth = 0): AppError | undefined {
  if (!value || typeof value !== 'object' || depth > 4) return undefined
  const candidate = value as Record<string, unknown>
  if (typeof candidate.code === 'string' && typeof candidate.messageKey === 'string') {
    return {
      code: candidate.code,
      messageKey: candidate.messageKey,
      safeDetail: typeof candidate.safeDetail === 'string' ? candidate.safeDetail : undefined,
    }
  }
  return decodeAppError(candidate.cause, depth + 1)
}

type ErrorEntry = (typeof errorMessages)[keyof typeof errorMessages]

function pickLocalized(entry: ErrorEntry, locale: string): string {
  const tag = locale.toLowerCase()
  if (tag.startsWith('ko')) return entry.ko
  if (tag.startsWith('ja')) return entry.ja
  if (tag.startsWith('ru')) return entry.ru
  if (tag.startsWith('fr')) return entry.fr

  if (tag.startsWith('zh-hant') || tag.startsWith('zh-tw') || tag.startsWith('zh-hk')) return entry.zhHant
  return tag.startsWith('en') ? entry.en : entry.zh
}

function localizedUnknown(locale: string): string {
  return pickLocalized(errorMessages['error.unknown'], locale)
}

export function translateErrorKey(messageKey: string, fallback = '', locale = currentLocale()): string {
  const entry = errorMessages[messageKey as keyof typeof errorMessages]

  if (!entry) return fallback || localizedUnknown(locale)
  return pickLocalized(entry, locale)
}

export function translateAppError(error: AppError, fallback = '', locale = currentLocale()): string {
  return translateErrorKey(error.messageKey, fallback, locale)
}

export function errorMessage(error: unknown, fallback: string, locale = currentLocale()): string {
  const appError = decodeAppError(error)
  if (appError) return translateAppError(appError, fallback, locale)
  if (error instanceof Error) {
    if (error.name === 'RuntimeError') return translateErrorKey('error.unknown', fallback, locale)
    if (error.message.trim()) return error.message
  }
  if (typeof error === 'string' && error.trim()) return error
  return fallback
}

export function credentialStatusMessage(status: { messageKey: string }, fallback: string): string {
  return status.messageKey ? translateErrorKey(status.messageKey, fallback) : fallback
}
