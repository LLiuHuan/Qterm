import { appMessages } from './app'
import { appLockMessages } from './applock'
import { previewMessages } from './preview'
import { remoteEditMessages } from './remote-edit'
import { settingsMessages } from './settings'
import { sftpMessages } from './sftp'
import { shellMessages } from './shell'

export const SUPPORTED_LOCALES = ['zh-CN', 'zh-Hant', 'en-US', 'ko-KR', 'ja-JP', 'ru-RU', 'fr-FR'] as const

export type AppLocale = (typeof SUPPORTED_LOCALES)[number]

export type MessageTable = Record<string, string>

const DOMAIN_TABLES: readonly Record<AppLocale, MessageTable>[] = [
  settingsMessages,
  shellMessages,
  remoteEditMessages,
  previewMessages,
  sftpMessages,
  appLockMessages,
  appMessages,
]

function assembleLocale(locale: AppLocale): MessageTable {
  return Object.assign({}, ...DOMAIN_TABLES.map((tables) => tables[locale]))
}

export const zhCN = assembleLocale('zh-CN')
export const zhHant = assembleLocale('zh-Hant')
export const enUS = assembleLocale('en-US')
export const koKR = assembleLocale('ko-KR')
export const jaJP = assembleLocale('ja-JP')
export const ruRU = assembleLocale('ru-RU')
export const frFR = assembleLocale('fr-FR')

export const messages: Record<AppLocale, MessageTable> = {
  'zh-CN': zhCN,
  'zh-Hant': zhHant,
  'en-US': enUS,
  'ko-KR': koKR,
  'ja-JP': jaJP,
  'ru-RU': ruRU,
  'fr-FR': frFR,
}
