export interface DiffLine {
  kind: 'equal' | 'add' | 'remove'

  leftNumber: number | null
  rightNumber: number | null
  text: string
}

export interface DiffResult {
  status: 'ok' | 'too-large'
  lines: DiffLine[]
  added: number
  removed: number
}

export const MAX_DIFF_LINES = 4000

function splitLines(text: string): string[] {
  return text.length === 0 ? [] : text.split('\n')
}

function lcsTable(left: string[], right: string[]): Uint32Array {
  const width = right.length + 1
  const table = new Uint32Array((left.length + 1) * width)
  for (let i = left.length - 1; i >= 0; i -= 1) {
    for (let j = right.length - 1; j >= 0; j -= 1) {
      table[i * width + j] =
        left[i] === right[j]
          ? (table[(i + 1) * width + j + 1] ?? 0) + 1
          : Math.max(table[(i + 1) * width + j] ?? 0, table[i * width + j + 1] ?? 0)
    }
  }
  return table
}

export function diffLines(oldText: string, newText: string): DiffResult {
  const left = splitLines(oldText)
  const right = splitLines(newText)
  if (left.length > MAX_DIFF_LINES || right.length > MAX_DIFF_LINES) {
    return { status: 'too-large', lines: [], added: 0, removed: 0 }
  }
  const table = lcsTable(left, right)
  const width = right.length + 1
  const lines: DiffLine[] = []
  let added = 0
  let removed = 0
  let i = 0
  let j = 0
  while (i < left.length && j < right.length) {
    if (left[i] === right[j]) {
      lines.push({ kind: 'equal', leftNumber: i + 1, rightNumber: j + 1, text: left[i] ?? '' })
      i += 1
      j += 1
      continue
    }

    if ((table[(i + 1) * width + j] ?? 0) >= (table[i * width + j + 1] ?? 0)) {
      lines.push({ kind: 'remove', leftNumber: i + 1, rightNumber: null, text: left[i] ?? '' })
      removed += 1
      i += 1
    } else {
      lines.push({ kind: 'add', leftNumber: null, rightNumber: j + 1, text: right[j] ?? '' })
      added += 1
      j += 1
    }
  }
  while (i < left.length) {
    lines.push({ kind: 'remove', leftNumber: i + 1, rightNumber: null, text: left[i] ?? '' })
    removed += 1
    i += 1
  }
  while (j < right.length) {
    lines.push({ kind: 'add', leftNumber: null, rightNumber: j + 1, text: right[j] ?? '' })
    added += 1
    j += 1
  }
  return { status: 'ok', lines, added, removed }
}
