import { Events } from '@wailsio/runtime'
import type { AdHocProtocol } from './adhocSession'
import { hasWailsRuntime } from './runtime'

export const LAUNCH_INTENT_EVENT = 'qterm:launch-intent'

export interface LaunchIntent {
  kind: string
  target: string
}

export interface ParsedLaunchTarget {
  protocol: AdHocProtocol
  target: string
}

export type LaunchTargetError = 'empty' | 'unsupported-scheme' | 'unsupported-verb' | 'password-in-url' | 'invalid'

const SUPPORTED_SCHEMES = new Set(['qterm', 'ssh', 'telnet'])

export function parseLaunchIntent(intent: LaunchIntent): ParsedLaunchTarget | LaunchTargetError {
  const raw = (intent?.target ?? '').trim()
  if (!raw) return 'empty'

  if (intent.kind === 'open' && !raw.includes('://')) {
    const protocol: AdHocProtocol = /^telnet(\s|$)/i.test(raw) ? 'telnet' : 'ssh'
    return { protocol, target: raw }
  }

  return parseLaunchURL(raw)
}

function parseLaunchURL(raw: string): ParsedLaunchTarget | LaunchTargetError {
  let url: URL
  try {
    url = new URL(raw)
  } catch {
    return 'invalid'
  }

  const scheme = url.protocol.replace(/:$/, '').toLowerCase()
  if (!SUPPORTED_SCHEMES.has(scheme)) return 'unsupported-scheme'
  if (url.password) return 'password-in-url'

  if (scheme === 'ssh' || scheme === 'telnet') {
    return authorityTarget(scheme, url.username, url.hostname, url.port)
  }

  const verb = url.hostname.toLowerCase()
  if (verb === 'ssh' || verb === 'telnet') {
    const path = url.pathname.replace(/^\//, '')
    if (path) {
      const parsed = splitAuthority(path)
      if (parsed === 'invalid') return 'invalid'
      return authorityTarget(verb, parsed.username, parsed.host, parsed.port)
    }

    const query = url.searchParams
    if (url.password || query.has('password')) return 'password-in-url'
    return authorityTarget(verb, query.get('user') ?? '', query.get('host') ?? '', query.get('port') ?? '')
  }

  if (verb === 'open') {
    const inner = (url.searchParams.get('target') ?? '').trim()
    if (!inner) return 'invalid'
    if (inner.includes('://')) return parseLaunchURL(inner)
    const protocol: AdHocProtocol = /^telnet(\s|$)/i.test(inner) ? 'telnet' : 'ssh'
    return { protocol, target: inner }
  }

  return 'unsupported-verb'
}

function authorityTarget(
  protocol: AdHocProtocol,
  username: string,
  host: string,
  port: string | number,
): ParsedLaunchTarget | LaunchTargetError {
  const cleanHost = String(host ?? '').trim()
  if (!cleanHost) return 'invalid'

  const rawPort = String(port ?? '').trim()
  if (rawPort) {
    const portNumber = Number(rawPort)

    if (!Number.isInteger(portNumber) || portNumber < 1 || portNumber > 65535) return 'invalid'
  }

  const user = String(username ?? '').trim()
  const authority = rawPort ? `${cleanHost}:${rawPort}` : cleanHost
  return { protocol, target: user ? `${user}@${authority}` : authority }
}

function splitAuthority(text: string): { username: string; host: string; port: string } | 'invalid' {
  let rest = text
  let username = ''
  const at = rest.lastIndexOf('@')
  if (at >= 0) {
    username = rest.slice(0, at)
    rest = rest.slice(at + 1)
  }
  if (!rest) return 'invalid'

  if (rest.startsWith('[')) {
    const close = rest.indexOf(']')
    if (close < 0) return 'invalid'
    const host = rest.slice(0, close + 1)
    const tail = rest.slice(close + 1)
    if (!tail) return { username, host, port: '' }
    if (!tail.startsWith(':')) return 'invalid'
    return { username, host, port: tail.slice(1) }
  }

  const colon = rest.lastIndexOf(':')
  if (colon < 0) return { username, host: rest, port: '' }
  return { username, host: rest.slice(0, colon), port: rest.slice(colon + 1) }
}

export async function takeLaunchIntents(): Promise<LaunchIntent[]> {
  if (!hasWailsRuntime()) return []
  try {
    const { TakeLaunchIntents } = await import('../../bindings/changeme/internal/app/service')
    const intents = (await TakeLaunchIntents()) as LaunchIntent[] | null
    return Array.isArray(intents) ? intents.filter((item) => item && typeof item.target === 'string') : []
  } catch {
    return []
  }
}

export function subscribeLaunchIntent(handler: (intents: LaunchIntent[]) => void): () => void {
  if (!hasWailsRuntime()) return () => {}
  try {
    return Events.On(LAUNCH_INTENT_EVENT, (event: { data?: unknown }) => {
      const payload = (event as { data?: unknown }).data
      if (!Array.isArray(payload)) return
      const intents = payload.filter(
        (item): item is LaunchIntent =>
          !!item &&
          typeof (item as LaunchIntent).target === 'string' &&
          typeof (item as LaunchIntent).kind === 'string',
      )
      if (intents.length) handler(intents)
    })
  } catch {
    return () => {}
  }
}
