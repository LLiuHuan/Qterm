import { hasWailsRuntime } from './runtime'

export interface RouteTestHopView {
  kind: string
  label: string
  address: string
  reachable: boolean
  stage: string
  errorKey: string
  latencyMs: number

  unverifiedHostKey: boolean
}

export interface RouteTestResultView {
  mode: string
  ok: boolean
  hops: RouteTestHopView[]
}

export interface RouteDraftInputView {
  hostId?: string

  protocol?: string
  address: string
  port: number
  username: string
  jumpHostId?: string

  proxyId?: string
  proxyMode?: string
  proxyHost?: string
  proxyPort?: number
  proxyUsername?: string
  proxyPassword?: string
  clearProxyPassword?: boolean
  credentialId?: string
  kind?: string
  secret?: string
}

function toRouteTestResultView(result: {
  mode?: string
  ok?: boolean
  hops?: Array<{
    kind?: string
    label?: string
    address?: string
    reachable?: boolean
    stage?: string
    errorKey?: string
    latencyMs?: number
    unverifiedHostKey?: boolean
  }> | null
}): RouteTestResultView {
  return {
    mode: result.mode || 'direct',
    ok: Boolean(result.ok),
    hops: (result.hops ?? []).map((hop) => ({
      kind: hop.kind || '',
      label: hop.label || '',
      address: hop.address || '',
      reachable: Boolean(hop.reachable),
      stage: hop.stage || '',
      errorKey: hop.errorKey || '',
      latencyMs: Number(hop.latencyMs) || 0,
      unverifiedHostKey: Boolean(hop.unverifiedHostKey),
    })),
  }
}

export async function testRouteDraft(input: RouteDraftInputView): Promise<RouteTestResultView | null> {
  if (!hasWailsRuntime()) return null

  const { TestHostRouteDraft } = await import('../../bindings/changeme/internal/app/service')
  const result = await TestHostRouteDraft({
    hostId: input.hostId || '',
    protocol: input.protocol || '',
    address: input.address,
    port: input.port,
    username: input.username,
    jumpHostId: input.jumpHostId || '',
    proxyId: input.proxyId || '',
    proxyMode: input.proxyMode || '',
    proxyHost: input.proxyHost || '',
    proxyPort: input.proxyPort || 0,
    proxyUsername: input.proxyUsername || '',
    proxyPassword: input.proxyPassword || '',
    clearProxyPassword: input.clearProxyPassword === true,
    credentialId: input.credentialId || '',
    kind: input.kind || '',
    secret: input.secret || '',
  })
  return toRouteTestResultView(result)
}
