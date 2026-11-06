export {
  SETTINGS_CATEGORIES,
  SETTINGS_CATEGORY_GROUPS,
  type SettingsCategory,
  type SettingsCategoryGroup,
  type SettingsCategoryGroupId,
} from './categories'
export { useSettingsState } from './useSettingsState'
export {
  DEFAULT_SETTINGS,
  loadSettings,
  normalizeSettings,
  saveSettings,
  SETTINGS_STORAGE_KEY,
  TERMINAL_FONT_SIZE_MAX,
  TERMINAL_FONT_SIZE_MIN,
  SESSION_HISTORY_LIMIT_MAX,
  SESSION_HISTORY_LIMIT_MIN,
  THEME_MODES,
  type AppLocaleSetting,
  type KeywordHighlightRule,
  type QtermSettings,
  type TerminalDisconnectBehavior,
  type ThemeMode,
} from './settingsStore'
export { default as KnownHostsSettingsPanel } from './KnownHostsSettingsPanel.vue'
export { default as AiSettingsPanel } from './AiSettingsPanel.vue'
export { default as AssetTransferPanel } from './AssetTransferPanel.vue'
export { default as ApplicationSettingsPanel } from './ApplicationSettingsPanel.vue'
export { default as AppearanceSettingsPanel } from './AppearanceSettingsPanel.vue'
export { default as TerminalSettingsPanel } from './TerminalSettingsPanel.vue'
export { default as RdpSettingsPanel } from './RdpSettingsPanel.vue'
export { default as AboutSettingsPanel } from './AboutSettingsPanel.vue'

export { default as SnippetsSettingsPanel } from './SnippetsSettingsPanel.vue'
export { default as CredentialsSettingsPanel } from './CredentialsSettingsPanel.vue'
export { default as ProxySettingsPanel } from './ProxySettingsPanel.vue'
export { default as KeywordHighlightSettingsPanel } from './KeywordHighlightSettingsPanel.vue'
export { default as CredentialPromptSettingsPanel } from './CredentialPromptSettingsPanel.vue'
export { default as ShortcutsSettingsPanel } from './ShortcutsSettingsPanel.vue'
export {
  DEFAULT_SHORTCUTS,
  SHORTCUTS_STORAGE_KEY,
  loadShortcuts,
  normalizeShortcuts,
  saveShortcuts,
  shortcutDisplay,
  shortcutFromKeyboardEvent,
  shortcutMatches,
  type ShortcutAction,
  type ShortcutBindings,
} from './shortcutStore'
