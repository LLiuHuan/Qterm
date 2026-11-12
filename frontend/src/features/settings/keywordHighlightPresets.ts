import type { KeywordHighlightRule } from './settingsStore'

export interface KeywordHighlightPreset {
  id: 'error' | 'warning' | 'success' | 'ipv4'

  labelKey: string
  rule: KeywordHighlightRule
}

export const KEYWORD_HIGHLIGHT_PRESETS: KeywordHighlightPreset[] = [
  {
    id: 'error',
    labelKey: 'settings.keywordHighlight.preset.error',
    rule: {
      enabled: true,
      pattern: '\\b(?:ERROR|ERR|FATAL|FAIL(?:ED|URE)?|EXCEPTION|CRITICAL|PANIC|SEGFAULT|DENIED|REFUSED|ABORTED)\\b',
      note: '',
      color: '#dc2626',
      caseSensitive: false,
    },
  },
  {
    id: 'warning',
    labelKey: 'settings.keywordHighlight.preset.warning',
    rule: {
      enabled: true,
      pattern: '\\b(?:WARN(?:ING)?|DEPRECATED|CAUTION|TIMEOUT|TIMED OUT|RETRY(?:ING)?|UNREACHABLE)\\b',
      note: '',
      color: '#b45309',
      caseSensitive: false,
    },
  },
  {
    id: 'success',
    labelKey: 'settings.keywordHighlight.preset.success',
    rule: {
      enabled: true,
      pattern: '\\b(?:SUCCESS|SUCCEEDED|PASS(?:ED)?|DONE|COMPLETE(?:D)?|OK|READY)\\b',
      note: '',
      color: '#15803d',
      caseSensitive: false,
    },
  },
  {
    id: 'ipv4',
    labelKey: 'settings.keywordHighlight.preset.ipv4',
    rule: {
      enabled: true,
      pattern:
        '(?<![\\w.])(?:(?:25[0-5]|2[0-4]\\d|1\\d{2}|[1-9]?\\d)\\.){3}(?:25[0-5]|2[0-4]\\d|1\\d{2}|[1-9]?\\d)(?![\\w.])',
      note: '',
      color: '#2563eb',
      caseSensitive: false,
    },
  },
]

export function hasKeywordHighlightPreset(rules: KeywordHighlightRule[], preset: KeywordHighlightPreset): boolean {
  return rules.some(
    (rule) =>
      rule.pattern === preset.rule.pattern && (rule.caseSensitive ?? true) === (preset.rule.caseSensitive ?? true),
  )
}
