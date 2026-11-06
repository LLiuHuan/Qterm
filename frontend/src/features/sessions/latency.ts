import { translate } from '@/locales'

export function latencyDisplay(milliseconds: number | null | undefined) {
  if (milliseconds == null || !Number.isFinite(milliseconds) || milliseconds < 0) {
    return { text: '—', tone: 'unknown', title: translate('shell.workspace.latencyUnknown') }
  }
  const value = Math.max(1, Math.round(milliseconds))
  return {
    text: `${value}ms`,
    tone: value < 100 ? 'good' : value < 300 ? 'fair' : 'slow',
    title: translate('shell.workspace.latencyTitle'),
  }
}
