import { settingsAssetsEn, settingsBaseEn, settingsCoreEn } from './en-US'
import { settingsAssetsFr, settingsBaseFr, settingsCoreFr } from './fr-FR'
import { settingsAssetsJa, settingsBaseJa, settingsCoreJa } from './ja-JP'
import { settingsAssetsKo, settingsBaseKo, settingsCoreKo } from './ko-KR'
import { settingsAssetsRu, settingsBaseRu, settingsCoreRu } from './ru-RU'
import { settingsAssetsZh, settingsBaseZh, settingsCoreZh } from './zh-CN'
import { settingsAssetsZhHant, settingsBaseZhHant, settingsCoreZhHant } from './zh-Hant'

export const settingsMessages = {
  'zh-CN': { ...settingsAssetsZh, ...settingsCoreZh, ...settingsBaseZh },
  'zh-Hant': { ...settingsAssetsZhHant, ...settingsCoreZhHant, ...settingsBaseZhHant },
  'en-US': { ...settingsAssetsEn, ...settingsCoreEn, ...settingsBaseEn },
  'ko-KR': { ...settingsAssetsKo, ...settingsCoreKo, ...settingsBaseKo },
  'ja-JP': { ...settingsAssetsJa, ...settingsCoreJa, ...settingsBaseJa },
  'ru-RU': { ...settingsAssetsRu, ...settingsCoreRu, ...settingsBaseRu },
  'fr-FR': { ...settingsAssetsFr, ...settingsCoreFr, ...settingsBaseFr },
}
