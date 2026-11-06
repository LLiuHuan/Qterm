export interface VncServerIdentity {
  software: string

  version: string

  banner: string

  source: string
}

export const VNC_UNKNOWN_SERVER_IDENTITY: VncServerIdentity = {
  software: '',
  version: '',
  banner: '',
  source: '',
}

export function readServerIdentity(source: unknown): VncServerIdentity {
  const raw = (source ?? {}) as {
    serverSoftware?: unknown
    serverVersion?: unknown
    serverBanner?: unknown
    identitySource?: unknown
  }
  return {
    software: typeof raw.serverSoftware === 'string' ? raw.serverSoftware.trim().toLowerCase() : '',
    version: typeof raw.serverVersion === 'string' ? raw.serverVersion.trim() : '',
    banner: typeof raw.serverBanner === 'string' ? raw.serverBanner.trim() : '',
    source: typeof raw.identitySource === 'string' ? raw.identitySource.trim() : '',
  }
}

export const KASM_VNC_SOFTWARE = 'kasmvnc'

export interface KasmDialectGate {
  install: boolean

  dialectVerdict: 'kasmvnc' | 'other' | 'unknown'

  heuristic: boolean

  software: string

  version: string

  reason: 'identity-kasmvnc' | 'identity-other' | 'unknown-heuristic-on' | 'unknown-heuristic-off'
}

const PROXY_SOFTWARE = new Set(['nginx', 'apache', 'httpd', 'caddy', 'haproxy', 'traefik', 'envoy', 'iis'])

export function resolveKasmDialectGate(identity: VncServerIdentity, heuristic: boolean): KasmDialectGate {
  const software = identity.software
  const base = { heuristic, software, version: identity.version }
  if (software === KASM_VNC_SOFTWARE) {
    return { ...base, install: true, dialectVerdict: 'kasmvnc', reason: 'identity-kasmvnc' }
  }
  if (software && !PROXY_SOFTWARE.has(software)) {
    return { ...base, install: false, dialectVerdict: 'other', reason: 'identity-other' }
  }

  return {
    ...base,
    install: heuristic,
    dialectVerdict: 'unknown',
    reason: heuristic ? 'unknown-heuristic-on' : 'unknown-heuristic-off',
  }
}
