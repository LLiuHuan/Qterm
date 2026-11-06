export interface StructuredCommandCard {
  title?: string
  command: string
  target?: string
  description?: string
}

export interface StructuredReply {
  text: string
  commandCards: StructuredCommandCard[]
}

export function looksLikeStructuredReply(content: string): boolean {
  const trimmed = content.trimStart()
  if (trimmed.includes('"text"') && trimmed.includes('"commandCards"')) return true
  const body = trimmed.startsWith('```') ? trimmed.slice(trimmed.indexOf('\n') + 1).trimStart() : trimmed
  return body.startsWith('{') && !body.startsWith('{{')
}

export function parseStructuredReply(content: string): StructuredReply | null {
  const stripped = content
    .trim()
    .replace(/^```(?:json)?\s*\n?/, '')
    .replace(/```\s*$/, '')
    .trim()
  const start = stripped.indexOf('{')
  const end = stripped.lastIndexOf('}')
  if (start < 0 || end <= start) return null
  try {
    const parsed = JSON.parse(stripped.slice(start, end + 1)) as {
      text?: unknown
      commandCards?: unknown
    }
    if (typeof parsed.text !== 'string' || !parsed.text.trim()) return null
    const cards = Array.isArray(parsed.commandCards)
      ? parsed.commandCards
          .filter((card): card is Record<string, unknown> => Boolean(card) && typeof card === 'object')
          .map((card) => ({
            command: typeof card.command === 'string' ? card.command : '',
            title: typeof card.title === 'string' ? card.title : undefined,
            target: typeof card.target === 'string' ? card.target : undefined,
            description: typeof card.description === 'string' ? card.description : undefined,
          }))
          .filter((card) => card.command.trim() !== '')
      : []
    return { text: parsed.text, commandCards: cards }
  } catch {
    return null
  }
}

export function extractThinkContent(content: string): { visible: string; reasoning: string } {
  const parts: string[] = []
  const closed = /<think>([\s\S]*?)<\/think>/gi
  let visible = content.replace(closed, (_match, inner: string) => {
    if (inner.trim()) parts.push(inner.trim())
    return ''
  })
  const open = visible.lastIndexOf('<think>')
  if (open >= 0) {
    const trailing = visible.slice(open + 7).trim()
    if (trailing) parts.push(trailing)
    visible = visible.slice(0, open)
  }
  return { visible: visible.trim(), reasoning: parts.join('\n\n') }
}
