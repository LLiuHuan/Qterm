import { translateErrorKey } from '../../services/errors'

type Translate = (key: string, params?: Record<string, string | number>) => string

export function describeUpdateError(errorKey: string, errorDetail: string, t: Translate): string {
  const base = translateErrorKey(errorKey, t('about.update.state.failed'))
  if (!errorDetail) return base
  return base + t('about.update.errorDetail', { detail: errorDetail })
}
