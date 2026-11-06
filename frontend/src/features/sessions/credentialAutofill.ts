export type CredentialPromptKind = 'username' | 'password'

export interface CredentialPromptRule {
  id: string
  kind: CredentialPromptKind
  pattern: string
  enabled: boolean
}

export interface PromptCredential {
  id: string
  name: string
  kind: string
  username?: string
}

export interface CredentialPromptMatch {
  kind: CredentialPromptKind

  line: string

  credentials: PromptCredential[]
}

// eslint-disable-next-line no-control-regex -- 匹配 OSC/ANSI 控制序列是刻意为之
const ANSI_PATTERN = /[\u001b\u009b][[\]()#;?]*(?:[0-9]{1,4}(?:;[0-9]{0,4})*)?[0-9A-ORZcf-nqry=><~]/g
// eslint-disable-next-line no-control-regex -- 同上，OSC 需要匹配 ESC 与 BEL
const OSC_PATTERN = /\u001b\][\s\S]*?(?:\u0007|\u001b\\)/g
const PROMPT_TERMINATOR_PATTERN = /[:\uff1a]\s*$/u
const MAX_PROMPT_LINE_LENGTH = 500

export function stripTerminalControlSequences(text: string): string {
  return text.replace(OSC_PATTERN, '').replace(ANSI_PATTERN, '')
}

export function extractCredentialPromptLine(output: string): string {
  const stripped = stripTerminalControlSequences(output)
  const lastChar = stripped[stripped.length - 1]
  if (lastChar === '\r' || lastChar === '\n') return ''

  const normalized = stripped.replace(/\r/g, '\n').replace(/\n+/g, '\n')
  const lines = normalized.split('\n')
  const line = (lines[lines.length - 1] ?? '').trim()
  return line.length > MAX_PROMPT_LINE_LENGTH ? line.slice(-MAX_PROMPT_LINE_LENGTH) : line
}

export function compilePromptPattern(pattern: string): RegExp | null {
  const trimmed = pattern.trim()
  if (!trimmed) return null
  try {
    return new RegExp(trimmed, 'im')
  } catch {
    return null
  }
}

export function isValidPromptPattern(pattern: string): boolean {
  return compilePromptPattern(pattern) !== null
}

export function promptKindForLine(line: string, rules: CredentialPromptRule[]): CredentialPromptKind | null {
  if (!line || !PROMPT_TERMINATOR_PATTERN.test(line)) return null
  for (const rule of rules) {
    if (!rule.enabled) continue
    const pattern = compilePromptPattern(rule.pattern)
    if (!pattern) continue
    if (pattern.test(line)) return rule.kind
  }
  return null
}

export function filterPromptCredentials(
  credentials: PromptCredential[],
  kind: CredentialPromptKind,
): PromptCredential[] {
  if (kind === 'username') {
    return credentials.filter((credential) => Boolean(credential.username?.trim()))
  }
  return credentials.filter((credential) => credential.kind === 'password')
}

export function detectCredentialPrompt(
  output: string,
  rules: CredentialPromptRule[],
  credentials: PromptCredential[],
): CredentialPromptMatch | null {
  const line = extractCredentialPromptLine(output)
  if (!line) return null
  const kind = promptKindForLine(line, rules)
  if (!kind) return null
  return { kind, line, credentials: filterPromptCredentials(credentials, kind) }
}
