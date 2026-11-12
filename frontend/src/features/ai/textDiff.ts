export type DiffLineKind = 'context' | 'add' | 'remove'

export interface DiffLine {
  kind: DiffLineKind
  text: string

  oldLine?: number

  newLine?: number
}

export interface TextDiff {
  lines: DiffLine[]
  added: number
  removed: number

  truncated: boolean
}

const MAX_DIFF_CELLS = 4_000_000

function splitLines(text: string): string[] {
  const normalized = text.replace(/\r\n/g, '\n')
  if (normalized === '') return []
  const lines = normalized.split('\n')

  if (lines.length > 0 && lines[lines.length - 1] === '') lines.pop()
  return lines
}

function replaceAll(oldLines: string[], newLines: string[]): TextDiff {
  const lines: DiffLine[] = []
  oldLines.forEach((text, index) => lines.push({ kind: 'remove', text, oldLine: index + 1 }))
  newLines.forEach((text, index) => lines.push({ kind: 'add', text, newLine: index + 1 }))
  return { lines, added: newLines.length, removed: oldLines.length, truncated: true }
}

export function diffLines(oldText: string, newText: string): TextDiff {
  const oldLines = splitLines(oldText)
  const newLines = splitLines(newText)
  if (oldLines.length === 0 && newLines.length === 0) {
    return { lines: [], added: 0, removed: 0, truncated: false }
  }
  if ((oldLines.length + 1) * (newLines.length + 1) > MAX_DIFF_CELLS) {
    return replaceAll(oldLines, newLines)
  }

  const columns = newLines.length + 1
  const dp = new Int32Array((oldLines.length + 1) * columns)
  for (let i = oldLines.length - 1; i >= 0; i--) {
    for (let j = newLines.length - 1; j >= 0; j--) {
      dp[i * columns + j] =
        oldLines[i] === newLines[j]
          ? dp[(i + 1) * columns + j + 1] + 1
          : Math.max(dp[(i + 1) * columns + j], dp[i * columns + j + 1])
    }
  }

  const lines: DiffLine[] = []
  let added = 0
  let removed = 0
  let i = 0
  let j = 0
  while (i < oldLines.length && j < newLines.length) {
    if (oldLines[i] === newLines[j]) {
      lines.push({ kind: 'context', text: oldLines[i], oldLine: i + 1, newLine: j + 1 })
      i++
      j++
      continue
    }
    if (dp[(i + 1) * columns + j] >= dp[i * columns + j + 1]) {
      lines.push({ kind: 'remove', text: oldLines[i], oldLine: i + 1 })
      removed++
      i++
      continue
    }
    lines.push({ kind: 'add', text: newLines[j], newLine: j + 1 })
    added++
    j++
  }
  while (i < oldLines.length) {
    lines.push({ kind: 'remove', text: oldLines[i], oldLine: i + 1 })
    removed++
    i++
  }
  while (j < newLines.length) {
    lines.push({ kind: 'add', text: newLines[j], newLine: j + 1 })
    added++
    j++
  }
  return { lines, added, removed, truncated: false }
}
