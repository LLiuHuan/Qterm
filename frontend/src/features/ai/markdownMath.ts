import type { ComarkPlugin, MarkdownItPlugin } from '@comark/vue'
import math from '@comark/vue/plugins/math'

type MarkdownParser = Parameters<MarkdownItPlugin>[0]
type InlineState = InstanceType<MarkdownParser['inline']['State']>
type BlockState = InstanceType<MarkdownParser['block']['State']>

function closing(source: string, marker: string, from: number): number {
  for (let index = source.indexOf(marker, from); index !== -1; index = source.indexOf(marker, index + marker.length)) {
    let slash = index - 1
    while (slash >= 0 && source[slash] === '\\') slash--
    if ((index - slash - 1) % 2 === 0) return index
  }
  return -1
}

const correctDelimiters: MarkdownItPlugin = (md) => {
  md.inline.ruler.at('math_inline', (state: InlineState, silent: boolean) => {
    const start = state.pos
    if (state.src[start] !== '$' || state.src[start + 1] === '$' || /\s/.test(state.src[start + 1] || ' ')) return false
    const end = closing(state.src.slice(0, state.posMax), '$', start + 1)

    if (end !== -1 && (/\s/.test(state.src[end - 1] || '') || /\d/.test(state.src[end + 1] || ''))) return false
    if (end === -1 || state.src.slice(start, end).includes('\n')) return false
    if (!silent) {
      const token = state.push('math_inline', 'math', 0)
      token.content = state.src.slice(start + 1, end)
      token.markup = '$'
    }
    state.pos = end + 1
    return true
  })
  md.block.ruler.at(
    'math_block',
    (state: BlockState, startLine: number, endLine: number, silent: boolean) => {
      if (state.sCount[startLine]! - state.blkIndent >= 4) return false
      const first = state.src.slice(state.bMarks[startLine]! + state.tShift[startLine]!, state.eMarks[startLine])
      if (!first.startsWith('$$')) return false
      const lines: string[] = []
      for (let line = startLine; line < endLine; line++) {
        if (line > startLine && state.sCount[line]! < state.blkIndent) return false
        const content =
          line === startLine
            ? first.slice(2)
            : state.src.slice(state.bMarks[line]! + state.tShift[line]!, state.eMarks[line])
        const end = closing(content, '$$', 0)
        if (end !== -1) {
          if (content.slice(end + 2).trim()) return false
          lines.push(content.slice(0, end))
          if (!silent) {
            const token = state.push('math_block', 'math', 0)
            token.content = lines.join('\n').trim()
            token.markup = '$$'
            token.block = true
            token.map = [startLine, line + 1]
            state.line = line + 1
          }
          return true
        }
        lines.push(content)
      }
      return false
    },
    { alt: ['paragraph', 'reference', 'blockquote', 'list'] },
  )
}
const base = math()
export const markdownMath: ComarkPlugin = {
  ...base,
  name: 'qterm-math',
  markdownItPlugins: [...(base.markdownItPlugins ?? []), correctDelimiters],
}
