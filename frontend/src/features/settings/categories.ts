export type SettingsCategory =
  | 'credentials'
  | 'knownHosts'
  | 'ai'
  | 'proxies'
  | 'portForwards'
  | 'application'
  | 'appearance'
  | 'terminal'
  | 'snippets'
  | 'shortcuts'
  | 'history'
  | 'recordings'
  | 'highlight'
  | 'credentialPrompt'
  | 'rdp'
  | 'about'

export type SettingsCategoryGroupId = 'manage' | 'preference'

export interface SettingsCategoryGroup {
  id: SettingsCategoryGroupId
  items: readonly SettingsCategory[]
}

export const SETTINGS_CATEGORY_GROUPS: readonly SettingsCategoryGroup[] = [
  {
    id: 'manage',
    items: [
      'credentials',
      'knownHosts',
      'proxies',
      'portForwards',
      'snippets',
      'credentialPrompt',
      'history',
      'recordings',
    ],
  },
  {
    id: 'preference',
    items: ['application', 'appearance', 'terminal', 'rdp', 'highlight', 'shortcuts', 'ai', 'about'],
  },
]

export const SETTINGS_CATEGORIES: readonly SettingsCategory[] = SETTINGS_CATEGORY_GROUPS.flatMap((group) => group.items)
