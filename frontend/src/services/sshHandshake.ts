import { hasWailsRuntime } from './runtime'

export interface SSHHandshakeAlgorithmCatalog {
  hostKeyAlgorithms: string[]
  deprecatedHostKeyAlgorithms: string[]
  keyExchanges: string[]
  deprecatedKeyExchanges: string[]
}

function toList(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string' && item !== '') : []
}

export async function loadSSHHandshakeAlgorithms(): Promise<SSHHandshakeAlgorithmCatalog | null> {
  if (!hasWailsRuntime()) return null
  const { SSHHandshakeAlgorithmCatalog: fetchCatalog } = await import('../../bindings/changeme/internal/app/service')
  const result = await fetchCatalog()
  if (!result) return null
  return {
    hostKeyAlgorithms: toList(result.hostKeyAlgorithms),
    deprecatedHostKeyAlgorithms: toList(result.deprecatedHostKeyAlgorithms),
    keyExchanges: toList(result.keyExchanges),
    deprecatedKeyExchanges: toList(result.deprecatedKeyExchanges),
  }
}
