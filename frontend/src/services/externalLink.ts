import { withTimeout } from '../lib/async'
import { hasWailsRuntime } from './runtime'

export type WebSearchEngine = 'google' | 'bing' | 'baidu' | 'duckduckgo'

export const WEB_SEARCH_ENGINES: readonly WebSearchEngine[] = ['google', 'bing', 'baidu', 'duckduckgo']

const WEB_SEARCH_TEMPLATES: Record<WebSearchEngine, string> = {
  google: 'https://www.google.com/search?q={query}',
  bing: 'https://www.bing.com/search?q={query}',
  baidu: 'https://www.baidu.com/s?wd={query}',
  duckduckgo: 'https://duckduckgo.com/?q={query}',
}

export function webSearchUrl(engine: WebSearchEngine, query: string): string {
  return WEB_SEARCH_TEMPLATES[engine].replace('{query}', encodeURIComponent(query))
}

function isHttpUrl(value: string): boolean {
  try {
    const parsed = new URL(value)
    return parsed.protocol === 'https:' || parsed.protocol === 'http:'
  } catch {
    return false
  }
}

const OPEN_EXTERNAL_TIMEOUT_MS = 2000

export async function openExternalUrl(url: string): Promise<void> {
  const target = url.trim()
  if (!isHttpUrl(target)) {
    console.warn('[externalLink] 只允许打开 http(s) 链接，已忽略本次请求')
    return
  }
  try {
    if (hasWailsRuntime()) {
      const { Browser } = await import('@wailsio/runtime')

      const outcome = await withTimeout(Promise.resolve(Browser.OpenURL(target)), OPEN_EXTERNAL_TIMEOUT_MS)
      if (outcome.timedOut) {
        console.warn('[externalLink] 宿主打开外链超时（已放弃等待，不再阻塞界面）')
      }
      return
    }

    window.open(target, '_blank', 'noopener')
  } catch {
    console.warn('[externalLink] 打开外部链接失败')
  }
}
