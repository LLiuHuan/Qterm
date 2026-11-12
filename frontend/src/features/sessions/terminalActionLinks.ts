import type { ILink } from '@xterm/xterm'

export type TerminalActionLinkKind = 'url' | 'ipv4' | 'hostPort' | 'archive'

export interface TerminalActionLink {
  start: number

  end: number
  text: string
  kind: TerminalActionLinkKind
}

export interface TerminalActionLinkLine {
  row: number
  text: string

  columns: number[]

  endColumns: number[]
}

interface TerminalActionLinkCell {
  getChars(): string
  getWidth(): number
}

interface TerminalActionBufferLine {
  readonly isWrapped: boolean
  readonly length: number
  getCell(x: number): TerminalActionLinkCell | undefined
}

interface TerminalActionBuffer {
  readonly length: number
  getLine(y: number): TerminalActionBufferLine | undefined
}

const KIND_PRIORITY: Record<TerminalActionLinkKind, number> = {
  url: 0,
  hostPort: 1,
  ipv4: 2,
  archive: 3,
}

const URL_PATTERN = /https?:\/\/[^\s"'<>()[\]{}]+/g

const HOST_PORT_PATTERN = /(?<![\w.-])(?:[A-Za-z0-9][A-Za-z0-9.-]*|(?:\d{1,3}\.){3}\d{1,3}):(\d{1,5})(?![\w.-])/g
const IPV4_PATTERN = /(?<![\w.-])(?:\d{1,3}\.){3}\d{1,3}(?![\w.-])/g
const ARCHIVE_PATTERN =
  /(?<![\p{L}\p{N}_.-])[\p{L}\p{N}_.-]+\.(?:tar\.gz|tar\.bz2|tar\.xz|tar\.lz|tar\.zst|tar|tgz|tbz2|txz|zip|rar|7z|zst|lz4|gz|bz2|xz)(?![\p{L}\p{N}_.-])/giu

function trimUrlTail(value: string): string {
  return value.replace(/[.,;:!?]+$/, '')
}

function isIpv4(value: string): boolean {
  const parts = value.split('.')
  if (parts.length !== 4) return false
  return parts.every((part) => {
    if (!/^\d{1,3}$/.test(part)) return false
    return Number(part) <= 255
  })
}

function isValidPort(value: string): boolean {
  if (!/^\d{1,5}$/.test(value)) return false
  const port = Number(value)
  return port >= 1 && port <= 65535
}

function isValidHost(host: string): boolean {
  if (/^(?:\d{1,3}\.){3}\d{1,3}$/.test(host)) return isIpv4(host)

  if (!/^[A-Za-z0-9](?:[A-Za-z0-9.-]*[A-Za-z0-9])?$/.test(host)) return false
  const labels = host.split('.')
  return labels.every((label) => /^[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?$/.test(label)) && /[A-Za-z]/.test(host)
}

function collect(
  line: string,
  pattern: RegExp,
  kind: TerminalActionLinkKind,
  validate?: (match: RegExpExecArray, source: string) => string | null,
): TerminalActionLink[] {
  const links: TerminalActionLink[] = []
  pattern.lastIndex = 0
  for (const match of line.matchAll(pattern)) {
    const raw = match[0]
    if (!raw) continue
    const value = validate ? validate(match, line) : raw
    if (!value) continue
    const start = match.index ?? 0
    links.push({ start, end: start + value.length, text: value, kind })
  }
  return links
}

export function findTerminalActionLinks(line: string): TerminalActionLink[] {
  if (!line) return []

  const candidates: TerminalActionLink[] = [
    ...collect(line, URL_PATTERN, 'url', (match) => {
      const value = trimUrlTail(match[0])
      return value.length > 'https://'.length ? value : null
    }),
    ...collect(line, HOST_PORT_PATTERN, 'hostPort', (match) => {
      const port = match[1] ?? ''
      if (!isValidPort(port)) return null
      const host = match[0].slice(0, match[0].lastIndexOf(':'))
      if (!isValidHost(host)) return null
      return match[0]
    }),
    ...collect(line, IPV4_PATTERN, 'ipv4', (match, source) => {
      const end = (match.index ?? 0) + match[0].length

      if (/^:\d/.test(source.slice(end))) return null
      return isIpv4(match[0]) ? match[0] : null
    }),
    ...collect(line, ARCHIVE_PATTERN, 'archive'),
  ]

  candidates.sort((a, b) => a.start - b.start || KIND_PRIORITY[a.kind] - KIND_PRIORITY[b.kind])

  const result: TerminalActionLink[] = []
  let lastEnd = 0
  for (const candidate of candidates) {
    if (candidate.start < lastEnd) continue
    result.push(candidate)
    lastEnd = candidate.end
  }
  return result
}

export function toTerminalActionLinkLine(
  line: Pick<TerminalActionBufferLine, 'length' | 'getCell'>,
  row: number,
  maxColumns?: number,
  trimRight = true,
): TerminalActionLinkLine {
  const textParts: string[] = []
  const columns: number[] = []
  const endColumns: number[] = []
  const limit = Math.min(line.length, maxColumns ?? line.length)

  for (let column = 0; column < limit;) {
    const cell = line.getCell(column)
    const width = cell?.getWidth() ?? 1
    if (width <= 0) {
      column += 1
      continue
    }

    const chars = cell?.getChars() || ' '
    textParts.push(chars)
    for (let offset = 0; offset < chars.length; offset += 1) {
      columns.push(column)
      endColumns.push(column + width)
    }
    column += width
  }

  let text = textParts.join('')
  if (trimRight) {
    while (text.endsWith(' ')) {
      text = text.slice(0, -1)
      columns.pop()
      endColumns.pop()
    }
  }

  return { row, text, columns, endColumns }
}

function actionLinkForLines(
  lines: readonly TerminalActionLinkLine[],
  activate: (link: TerminalActionLink) => void,
): ILink[] {
  if (lines.length === 0) return []

  const offsets: number[] = []
  let offset = 0
  for (const line of lines) {
    offsets.push(offset)
    offset += line.text.length
  }
  const logicalText = lines.map((line) => line.text).join('')
  const links = findTerminalActionLinks(logicalText)

  return links.flatMap((link) => {
    let first: { line: TerminalActionLinkLine; start: number; end: number } | undefined
    let last: { line: TerminalActionLinkLine; start: number; end: number } | undefined

    for (let index = 0; index < lines.length; index += 1) {
      const line = lines[index]
      const lineStart = offsets[index] ?? 0
      const lineEnd = lineStart + line.text.length
      const start = Math.max(link.start, lineStart)
      const end = Math.min(link.end, lineEnd)
      if (start >= end) continue
      const segment = { line, start: start - lineStart, end: end - lineStart }
      first ??= segment
      last = segment
    }

    if (!first || !last) return []
    const firstColumn = first.line.columns[first.start] ?? 0
    const lastEndColumn = last.line.endColumns[last.end - 1] ?? firstColumn + 1
    return [
      {
        range: {
          start: { x: firstColumn + 1, y: first.line.row },
          end: { x: lastEndColumn, y: last.line.row },
        },
        text: link.text,
        activate: () => activate(link),
        decorations: { underline: true, pointerCursor: true },
      },
    ]
  })
}

function trimActionLinkLine(line: TerminalActionLinkLine): TerminalActionLinkLine {
  let length = line.text.length
  while (length > 0 && line.text[length - 1] === ' ') length -= 1
  if (length === line.text.length) return line
  return {
    row: line.row,
    text: line.text.slice(0, length),
    columns: line.columns.slice(0, length),
    endColumns: line.endColumns.slice(0, length),
  }
}

const MAX_LOGICAL_LINK_CHARS = 16_384
const MAX_LOGICAL_LINK_LINES = 200

export function toXtermActionLinksFromBuffer(
  buffer: TerminalActionBuffer,
  bufferLineNumber: number,
  maxColumns: number,
  activate: (link: TerminalActionLink) => void,
): ILink[] {
  const requestedIndex = bufferLineNumber - 1
  if (requestedIndex < 0 || requestedIndex >= buffer.length) return []

  let start = requestedIndex
  let backtrack = 0
  while (start > 0 && buffer.getLine(start)?.isWrapped) {
    start -= 1
    backtrack += 1
    if (backtrack >= MAX_LOGICAL_LINK_LINES) return []
  }

  const lines: TerminalActionLinkLine[] = []
  let logicalLength = 0
  for (let index = start; index < buffer.length; index += 1) {
    const line = buffer.getLine(index)
    if (!line || (index > start && !line.isWrapped)) break
    if (lines.length >= MAX_LOGICAL_LINK_LINES) return []
    const nextLine = buffer.getLine(index + 1)
    const columns = Math.min(line.length, maxColumns)

    const wideWrapPadding =
      nextLine?.isWrapped && nextLine.getCell(0)?.getWidth() === 2 && line.getCell(columns - 1)?.getChars() === ''
    const mapped = toTerminalActionLinkLine(line, index + 1, columns - (wideWrapPadding ? 1 : 0), false)
    logicalLength += mapped.text.length
    if (logicalLength > MAX_LOGICAL_LINK_CHARS) return []
    lines.push(mapped)
  }

  if (lines.length > 0) lines[lines.length - 1] = trimActionLinkLine(lines[lines.length - 1])

  return actionLinkForLines(lines, activate).filter(
    (link) => link.range.start.y <= bufferLineNumber && link.range.end.y >= bufferLineNumber,
  )
}

export function toXtermActionLinks(line: string, row: number, activate: (link: TerminalActionLink) => void): ILink[] {
  const columns = Array.from({ length: line.length }, (_, index) => index)
  const endColumns = Array.from({ length: line.length }, (_, index) => index + 1)
  return actionLinkForLines([{ row, text: line, columns, endColumns }], activate)
}
