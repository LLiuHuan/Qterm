export type AiContextSourceKind = 'terminal' | 'selection' | 'file'

export interface AiContextSource {
  id: string
  kind: AiContextSourceKind
  label: string
  text: string

  truncated?: boolean
}

export interface ContextSourceInput {
  id?: string
  kind: AiContextSourceKind
  label: string
  text: string
  truncated?: boolean
}

export type ContextSourceErrorCode =
  | 'file_unreadable'
  | 'source_limit'
  | 'source_empty'
  | 'file_too_large'
  | 'file_binary'
  | 'context_budget'
  | 'invalid_source'

export interface ContextSourceError {
  code: ContextSourceErrorCode
  message?: string
}

export interface AddContextSourceResult {
  source?: AiContextSource
  error?: ContextSourceError
}

export interface BuildAiContextResult {
  text: string
  bytes: number
  truncatedSourceIds: string[]
  error?: ContextSourceError
}

export const AI_CONTEXT_SOURCE_LIMIT = 6
export const AI_CONTEXT_BUDGET_BYTES = 8 * 1024
export const AI_CONTEXT_FILE_LIMIT_BYTES = 64 * 1024

const TERMINAL_TRUNCATION_MARKER = '[terminal context truncated]\n'

let sourceSequence = 0

function nextSourceId(kind: AiContextSourceKind): string {
  sourceSequence += 1
  return `ai-context-${kind}-${Date.now().toString(36)}-${sourceSequence.toString(36)}`
}

export function byteLength(value: string): number {
  return new TextEncoder().encode(value).byteLength
}

export function truncateUtf8Tail(value: string, maxBytes: number): { text: string; truncated: boolean } {
  if (maxBytes <= 0) return { text: '', truncated: value.length > 0 }
  const bytes = new TextEncoder().encode(value)
  if (bytes.byteLength <= maxBytes) return { text: value, truncated: false }

  let start = bytes.byteLength - maxBytes

  while (start < bytes.byteLength && (bytes[start]! & 0xc0) === 0x80) start += 1
  return { text: new TextDecoder().decode(bytes.slice(start)), truncated: true }
}

function validKind(value: unknown): value is AiContextSourceKind {
  return value === 'terminal' || value === 'selection' || value === 'file'
}

function cleanInput(input: ContextSourceInput): ContextSourceInput | null {
  if (!validKind(input.kind)) return null
  const label = input.label.trim()
  if (!label || typeof input.text !== 'string' || !input.text.trim()) return null
  return { ...input, label, text: input.text }
}

export function createContextSource(input: ContextSourceInput): AddContextSourceResult {
  const cleaned = cleanInput(input)
  if (!cleaned) return { error: { code: 'invalid_source' } }
  return {
    source: {
      id: cleaned.id?.trim() || nextSourceId(cleaned.kind),
      kind: cleaned.kind,
      label: cleaned.label,
      text: cleaned.text,
      ...(cleaned.truncated ? { truncated: true } : {}),
    },
  }
}

export function addContextSource(
  sources: readonly AiContextSource[],
  input: ContextSourceInput,
): AddContextSourceResult & {
  sources: AiContextSource[]
} {
  if (sources.length >= AI_CONTEXT_SOURCE_LIMIT) {
    return { sources: [...sources], error: { code: 'source_limit' } }
  }
  const result = createContextSource(input)
  if (!result.source) return { sources: [...sources], error: result.error }
  return { source: result.source, sources: [...sources, result.source] }
}

export function removeContextSource(sources: readonly AiContextSource[], id: string): AiContextSource[] {
  return sources.filter((source) => source.id !== id)
}

export function clearContextSources(): AiContextSource[] {
  return []
}

function isBinaryUtf8(bytes: Uint8Array): boolean {
  if (bytes.includes(0)) return true
  try {
    const decoded = new TextDecoder('utf-8', { fatal: true }).decode(bytes)
    return decoded.includes('\u0000')
  } catch {
    return true
  }
}

export async function readContextFile(file: File): Promise<AddContextSourceResult> {
  if (file.size > AI_CONTEXT_FILE_LIMIT_BYTES) return { error: { code: 'file_too_large' } }
  const bytes = new Uint8Array(await file.arrayBuffer())
  if (isBinaryUtf8(bytes)) return { error: { code: 'file_binary' } }
  const text = new TextDecoder('utf-8', { fatal: true }).decode(bytes)
  return createContextSource({ kind: 'file', label: file.name, text })
}

function buildEntry(source: AiContextSource, text: string): string {
  return `[${source.label}]\n${text}\n`
}

export function buildContext(
  sources: readonly AiContextSource[],
  budgetBytes = AI_CONTEXT_BUDGET_BYTES,
): BuildAiContextResult {
  if (!Number.isSafeInteger(budgetBytes) || budgetBytes < 0) {
    return { text: '', bytes: 0, truncatedSourceIds: [], error: { code: 'invalid_source' } }
  }

  let text = ''
  const truncatedSourceIds: string[] = []
  for (const source of sources) {
    const header = `[${source.label}]\n`
    const remaining = budgetBytes - byteLength(text)
    if (remaining <= 0) {
      if (source.kind !== 'terminal') {
        return { text, bytes: byteLength(text), truncatedSourceIds, error: { code: 'context_budget' } }
      }
      truncatedSourceIds.push(source.id)
      continue
    }

    const sourceBudget = remaining - byteLength(header) - byteLength('\n')
    if (sourceBudget < byteLength(source.text)) {
      if (source.kind !== 'terminal') {
        return { text, bytes: byteLength(text), truncatedSourceIds, error: { code: 'context_budget' } }
      }
      if (sourceBudget < 0) {
        truncatedSourceIds.push(source.id)
        continue
      }
      const marker = TERMINAL_TRUNCATION_MARKER
      const tailBudget = Math.max(0, sourceBudget - byteLength(marker))
      const tail = truncateUtf8Tail(source.text, tailBudget)
      const markerText = sourceBudget >= byteLength(marker) ? marker : truncateUtf8Tail(marker, sourceBudget).text
      const entry = buildEntry(source, `${markerText}${tail.text}`)
      text += entry
      truncatedSourceIds.push(source.id)
      continue
    }

    text += buildEntry(source, source.text)
    if (source.truncated) truncatedSourceIds.push(source.id)
  }
  return { text, bytes: byteLength(text), truncatedSourceIds }
}
