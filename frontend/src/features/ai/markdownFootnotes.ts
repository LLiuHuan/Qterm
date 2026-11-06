import type { ComarkPlugin, MarkdownItPlugin } from '@comark/vue'
import footnote from 'markdown-it-footnote'

type MarkdownItInstance = Parameters<typeof footnote>[0]

function qtermFootnotePlugin(md: MarkdownItInstance): void {
  md.use(footnote)
  md.core.ruler.after('footnote_tail', 'qterm-footnote-tokens', (state) => {
    type Token = (typeof state.tokens)[number]
    function token(type: string, tag: string, nesting: -1 | 0 | 1, attrs: [string, string][] = [], text = ''): Token {
      const result = new state.Token(type, tag, nesting)
      result.attrs = attrs
      result.content = text
      return result
    }
    function convert(tokens: Token[]): Token[] {
      return tokens.flatMap((item): Token[] => {
        if (item.children) item.children = convert(item.children)
        const number = Number(item.meta?.id) + 1
        const reference = `fnref-${number}-${Number(item.meta?.subId) || 0}`
        switch (item.type) {
          case 'footnote_block_open':
            return [token('ordered_list_open', 'ol', 1, [['class', 'footnotes-list']])]
          case 'footnote_block_close':
            return [token('ordered_list_close', 'ol', -1)]
          case 'footnote_open':
            return [token('list_item_open', 'li', 1, [['id', `fn-${number}`]])]
          case 'footnote_close':
            return [token('list_item_close', 'li', -1)]
          case 'footnote_ref':
            return [
              token('sup_open', 'sup', 1, [['class', 'footnote-ref']]),
              token('link_open', 'a', 1, [
                ['href', `#fn-${number}`],
                ['id', reference],
              ]),
              token('text', '', 0, [], `[${number}]`),
              token('link_close', 'a', -1),
              token('sup_close', 'sup', -1),
            ]
          case 'footnote_anchor':
            return [
              token('link_open', 'a', 1, [
                ['href', `#${reference}`],
                ['class', 'footnote-backref'],
              ]),
              token('text', '', 0, [], '↩'),
              token('link_close', 'a', -1),
            ]
          default:
            return [item]
        }
      })
    }
    state.tokens = convert(state.tokens)
  })
}

export const markdownFootnotes: ComarkPlugin = {
  name: 'qterm-footnotes',
  markdownItPlugins: [qtermFootnotePlugin as unknown as MarkdownItPlugin],
}
