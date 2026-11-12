import type { SessionRecord } from './sessions'

const FAVORITES_LIMIT = 20

export const FAVORITE_NAME_MAX_LENGTH = 64

export const ROOT_DIRECTORY_NAME = '根目录'

export interface SFTPFavorite {
  path: string

  name: string
}

const memory = new Map<string, SFTPFavorite[]>()

function hasWailsRuntime(): boolean {
  return Boolean((window as Window & { _wails?: { environment?: { OS?: string } } })._wails?.environment?.OS)
}

export function cleanFavoritePath(raw: string): string {
  const trimmed = (raw ?? '').trim()
  if (!trimmed.startsWith('/')) return ''
  const collapsed = trimmed.replace(/\/{2,}/g, '/')
  const withoutTrailing = collapsed.length > 1 ? collapsed.replace(/\/+$/, '') : collapsed
  return withoutTrailing.startsWith('/') ? withoutTrailing : ''
}

export function normalizeFavoriteName(raw: string): string {
  const stripped = Array.from(raw ?? '')
    .filter((char) => {
      const code = char.codePointAt(0) ?? 0
      return code >= 0x20 && code !== 0x7f && char !== '\u200b' && char !== '\ufeff'
    })
    .join('')
    .trim()
  return Array.from(stripped).slice(0, FAVORITE_NAME_MAX_LENGTH).join('').trim()
}

export function normalizeFavorites(items: readonly SFTPFavorite[]): SFTPFavorite[] {
  const result: SFTPFavorite[] = []
  const seen = new Set<string>()
  for (const item of items) {
    const cleaned = cleanFavoritePath(item?.path ?? '')
    if (!cleaned || seen.has(cleaned)) continue
    seen.add(cleaned)
    result.push({ path: cleaned, name: normalizeFavoriteName(item?.name ?? '') })
    if (result.length >= FAVORITES_LIMIT) break
  }
  return result
}

export function defaultFavoriteName(path: string): string {
  const trimmed = (path ?? '').trim().replace(/\/+$/, '')
  if (!trimmed) return ROOT_DIRECTORY_NAME
  const base = trimmed.slice(trimmed.lastIndexOf('/') + 1)
  return base || ROOT_DIRECTORY_NAME
}

export function favoriteDisplayName(favorite: SFTPFavorite): string {
  return favorite.name || defaultFavoriteName(favorite.path)
}

export function isFavoritePath(favorites: readonly SFTPFavorite[], path: string): boolean {
  const cleaned = cleanFavoritePath(path)
  return Boolean(cleaned) && favorites.some((item) => item.path === cleaned)
}

export async function resolveFavoritesHostId(sessionId: string): Promise<string> {
  if (!sessionId || !hasWailsRuntime()) return ''
  try {
    const { GetSession } = await import('../../bindings/changeme/internal/app/service')
    const session = (await GetSession(sessionId)) as SessionRecord | null
    return session?.hostId ?? ''
  } catch {
    return ''
  }
}

export async function loadSFTPFavorites(hostId: string): Promise<SFTPFavorite[]> {
  if (!hostId) return []
  if (!hasWailsRuntime()) return normalizeFavorites([...(memory.get(hostId) ?? [])])
  const { ListSFTPFavorites } = await import('../../bindings/changeme/internal/app/service')
  const stored = await ListSFTPFavorites(hostId)
  return normalizeFavorites((stored ?? []).map((item) => ({ path: item.path ?? '', name: item.name ?? '' })))
}

export async function saveSFTPFavorites(hostId: string, favorites: readonly SFTPFavorite[]): Promise<SFTPFavorite[]> {
  const normalized = normalizeFavorites(favorites)
  if (!hostId) return normalized
  if (!hasWailsRuntime()) {
    memory.set(hostId, normalized)
    return normalized
  }
  const { SetSFTPFavorites } = await import('../../bindings/changeme/internal/app/service')
  await SetSFTPFavorites(
    hostId,
    normalized.map((item) => ({ path: item.path, name: item.name })),
  )
  return normalized
}
