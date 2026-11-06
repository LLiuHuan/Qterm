import type { ComarkPlugin, ElementNode, Node } from '@comark/vue'
import { textContent } from '@comark/vue/utils'

const tags = new Set([
  'p',
  'h1',
  'h2',
  'h3',
  'h4',
  'h5',
  'h6',
  'strong',
  'b',
  'i',
  'kbd',
  'em',
  'del',
  's',
  'blockquote',
  'ul',
  'ol',
  'li',
  'a',
  'img',
  'pre',
  'code',
  'table',
  'thead',
  'tbody',
  'tr',
  'th',
  'td',
  'input',
  'hr',
  'br',
  'sup',
  'sub',
  'math',
])

export function markdownUrl(value: unknown): string | undefined {
  if (typeof value !== 'string') return undefined
  try {
    const url = new URL(value)
    return url.protocol === 'https:' || url.protocol === 'http:' ? url.href : undefined
  } catch {
    return undefined
  }
}

export function createMarkdownPolicy(messageId: string): ComarkPlugin {
  return {
    name: 'qterm-markdown-policy',
    post({ tree }) {
      function clean(node: Node): Node {
        if (typeof node === 'string' || node[0] === null) return node
        const [tag, attrs, ...children] = node
        if (!tags.has(tag)) return textContent(node)

        if (tag === 'math' && (attrs.$ || !['math inline', 'math block'].includes(String(attrs.class))))
          return textContent(node)
        const safe: ElementNode[1] = {}
        for (const key of ['title', 'alt', 'language', 'filename']) {
          if (typeof attrs[key] === 'string') safe[key] = attrs[key]
        }
        if (tag === 'math') {
          safe.content = typeof attrs.content === 'string' ? attrs.content : textContent(node)
          safe.class = attrs.class
        }
        const footnoteId = /^fn(?:ref)?-[a-zA-Z0-9_-]+$/
        if (typeof attrs.id === 'string' && footnoteId.test(attrs.id)) safe.id = `${messageId}-${attrs.id}`
        if (tag === 'a') {
          safe.href =
            typeof attrs.href === 'string' && footnoteId.test(attrs.href.slice(1)) && attrs.href.startsWith('#')
              ? `#${messageId}-${attrs.href.slice(1)}`
              : markdownUrl(attrs.href)
        }
        if (tag === 'img') safe.src = markdownUrl(attrs.src)
        if (tag === 'th' || tag === 'td') {
          const alignment =
            typeof attrs.style === 'string' ? /^text-align:\s*(left|center|right);?$/.exec(attrs.style)?.[1] : undefined
          if (['left', 'center', 'right'].includes(String(attrs.align))) safe.align = attrs.align
          else if (alignment) safe.align = alignment
        }
        if (tag === 'ol' && Number.isSafeInteger(Number(attrs.start)) && Number(attrs.start) > 0)
          safe.start = Number(attrs.start)
        if (
          ['contains-task-list', 'task-list-item', 'footnote-ref', 'footnotes-list', 'footnote-backref'].includes(
            String(attrs.class),
          )
        )
          safe.class = attrs.class
        if (tag === 'input') {
          safe.type = 'checkbox'
          safe.disabled = true
          safe.checked = attrs.checked === true || attrs.checked === '' || attrs[':checked'] === 'true'
        }
        return [tag, safe, ...children.map(clean)]
      }
      tree.nodes = tree.nodes.map(clean)
      tree.frontmatter = {}
    },
  }
}
