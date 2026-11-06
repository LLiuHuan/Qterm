import { errorMessage } from './errors'
import type { SessionState } from './sessions'

export type AdHocProtocol = 'ssh' | 'telnet'

export const ADHOC_HOST_ID_PREFIX = 'adhoc-'

export function isAdHocHostId(hostId: string | null | undefined): boolean {
  return typeof hostId === 'string' && hostId.startsWith(ADHOC_HOST_ID_PREFIX)
}

export interface AdHocSessionInput {
  protocol: AdHocProtocol
  address: string
  username: string
  port: number
  password: string
}

export interface ParsedAdHocTarget {
  input: AdHocSessionInput

  displayName: string
}

interface CommandTarget {
  authority: string
  port: number
  username: string
}

const SSH_IGNORABLE_FLAGS = new Set([
  '-4',
  '-6',
  '-A',
  '-a',
  '-C',
  '-f',
  '-G',
  '-g',
  '-K',
  '-k',
  '-M',
  '-N',
  '-n',
  '-q',
  '-s',
  '-T',
  '-t',
  '-V',
  '-v',
  '-X',
  '-x',
  '-Y',
  '-y',
])

function parseCommandTarget(protocol: AdHocProtocol, text: string): CommandTarget | 'invalid' | null {
  const tokens = text.split(/\s+/)
  const command = tokens[0]!.toLowerCase()

  if (protocol === 'telnet') {
    if (command !== 'telnet') return null
    const host = tokens[1]
    if (!host) return 'invalid'
    if (tokens[2] === undefined) return { authority: host, port: 0, username: '' }

    const port = Number(tokens[2])
    if (tokens[3] !== undefined || !Number.isInteger(port) || port < 1 || port > 65535) return 'invalid'
    return { authority: host, port, username: '' }
  }

  if (command !== 'ssh') return null

  let port = 0
  let username = ''
  let index = 1
  while (index < tokens.length) {
    const token = tokens[index]!
    if (token === '--') {
      index += 1
      break
    }
    if (!token.startsWith('-') || token === '-') break

    const portFlag = /^-p(\d*)$/.exec(token)
    if (portFlag) {
      const value = portFlag[1] || tokens[index + 1] || ''
      port = Number(value)
      if (!Number.isInteger(port) || port < 1 || port > 65535) return 'invalid'
      index += portFlag[1] ? 1 : 2
      continue
    }

    if (token === '-l') {
      const value = tokens[index + 1]
      if (!value) return 'invalid'
      username = value
      index += 2
      continue
    }
    const loginFlag = /^-l(.+)$/.exec(token)
    if (loginFlag) {
      username = loginFlag[1]!
      index += 1
      continue
    }

    if (SSH_IGNORABLE_FLAGS.has(token)) {
      index += 1
      continue
    }

    return 'invalid'
  }

  const destination = tokens[index]
  if (!destination) return 'invalid'

  return { authority: destination, port, username }
}

function parsePortSegment(raw: string): number {
  if (!/^\d+$/.test(raw)) return 0
  const port = Number(raw)
  return port >= 1 && port <= 65535 ? port : 0
}

function buildParsedTarget(
  protocol: AdHocProtocol,
  rawAuthority: string,
  portOverride: number,
  usernameOverride: string,
): ParsedAdHocTarget | string {
  let text = rawAuthority.trim()
  const prefix = `${protocol}://`
  if (text.toLowerCase().startsWith(prefix)) text = text.slice(prefix.length)

  text = text.split('/')[0]!.split('?')[0]!

  let username = usernameOverride
  const at = text.lastIndexOf('@')
  if (at >= 0) {
    username = text.slice(0, at)
    text = text.slice(at + 1)

    if (!username || /\s/.test(username)) return 'invalid'
  }

  let address = text
  let explicitPort = 0
  if (text.startsWith('[')) {
    const close = text.indexOf(']')
    if (close < 0) return 'invalid'
    address = text.slice(1, close)
    const rest = text.slice(close + 1)
    if (rest.startsWith(':')) {
      explicitPort = parsePortSegment(rest.slice(1))
      if (!explicitPort) return 'invalid'
    } else if (rest) return 'invalid'
  } else if (text.split(':').length > 2) {
    address = text
  } else {
    const colon = text.lastIndexOf(':')
    if (colon >= 0) {
      address = text.slice(0, colon)
      explicitPort = parsePortSegment(text.slice(colon + 1))
      if (!explicitPort) return 'invalid'
    }
  }
  address = address.trim()
  if (!address || /\s/.test(address)) return 'invalid'

  const port = explicitPort || portOverride

  const defaultPort = protocol === 'telnet' ? 23 : 22
  const effectivePort = port || defaultPort
  const displayName =
    (username ? `${username}@` : '') + address + (effectivePort === defaultPort ? '' : `:${effectivePort}`)
  return {
    input: { protocol, address, username, port: effectivePort, password: '' },
    displayName,
  }
}

export function parseAdHocTarget(protocol: AdHocProtocol, raw: string): ParsedAdHocTarget | string {
  const text = raw.trim()
  if (!text) return 'empty'
  const command = parseCommandTarget(protocol, text)
  if (command === 'invalid') return 'invalid'
  if (command) return buildParsedTarget(protocol, command.authority, command.port, command.username)
  return buildParsedTarget(protocol, text, 0, '')
}

export interface AdHocOpenResult {
  sessionId: string
  hostId: string
  streamToken: string

  state: Exclude<SessionState, 'preview'>
  attachEpoch?: number
}

export async function openAdHocSession(input: AdHocSessionInput): Promise<AdHocOpenResult> {
  const { OpenAdHocSession } = await import('../../bindings/changeme/internal/app/service')
  const result = (await OpenAdHocSession({
    protocol: input.protocol,
    address: input.address,
    username: input.username,
    port: input.port,
    password: input.password,
  })) as {
    session?: { id?: string; hostId?: string; state?: string; attachEpoch?: number } | null
    streamToken?: string
  } | null
  const session = result?.session
  if (!session?.id) throw new Error('shell.error.openSession')
  return {
    sessionId: session.id,
    hostId: session.hostId ?? '',
    streamToken: result?.streamToken ?? '',
    state: (session.state ?? 'connecting') as Exclude<SessionState, 'preview'>,
    attachEpoch: session.attachEpoch,
  }
}

export async function forgetAdHocSession(hostId: string): Promise<void> {
  if (!hostId) return
  try {
    const { ForgetAdHocSession } = await import('../../bindings/changeme/internal/app/service')
    await ForgetAdHocSession(hostId)
  } catch {}
}

export function adHocSessionErrorMessage(cause: unknown, fallback: string): string {
  return errorMessage(cause, fallback)
}
