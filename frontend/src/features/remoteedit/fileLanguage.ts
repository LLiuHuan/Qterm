import type { EditorLanguage } from '../../components/common/editor/types'

const EXTENSION_LANGUAGES: Readonly<Record<string, EditorLanguage>> = {
  py: 'python',
  pyw: 'python',
  rpy: 'python',
  sh: 'shell',
  bash: 'shell',
  zsh: 'shell',
  ksh: 'shell',
  fish: 'shell',

  pl: 'perl',
  pm: 'perl',
  rb: 'ruby',
  rake: 'ruby',
  gemspec: 'ruby',
  php: 'php',
  phtml: 'php',
  lua: 'lua',

  yaml: 'yaml',
  yml: 'yaml',
  ini: 'ini',
  properties: 'ini',
  conf: 'ini',
  cfg: 'ini',

  cnf: 'ini',

  xml: 'xml',
  svg: 'xml',
  xsd: 'xml',
  xsl: 'xml',
  xslt: 'xml',
  plist: 'xml',
  html: 'html',
  htm: 'html',
  xhtml: 'html',
  md: 'markdown',
  markdown: 'markdown',
  mdx: 'markdown',

  sql: 'sql',
  mysql: 'sql',
  pgsql: 'sql',

  js: 'javascript',
  jsx: 'javascript',
  mjs: 'javascript',
  cjs: 'javascript',
  ts: 'typescript',
  tsx: 'typescript',
  mts: 'typescript',
  cts: 'typescript',
  go: 'go',
  c: 'cpp',
  h: 'cpp',
  cc: 'cpp',
  cpp: 'cpp',
  cxx: 'cpp',
  hpp: 'cpp',
  hh: 'cpp',
  hxx: 'cpp',
  rs: 'rust',
}

const BASENAME_LANGUAGES: Readonly<Record<string, EditorLanguage>> = {
  dockerfile: 'dockerfile',
  containerfile: 'dockerfile',
  '.bashrc': 'shell',
  '.bash_profile': 'shell',
  '.bash_aliases': 'shell',
  '.zshrc': 'shell',
  '.zprofile': 'shell',
  '.profile': 'shell',
  bashrc: 'shell',
  bash_profile: 'shell',
  bash_aliases: 'shell',
  zshrc: 'shell',
  zprofile: 'shell',
  profile: 'shell',
  '.gitconfig': 'ini',
  '.editorconfig': 'ini',
  '.gitattributes': 'ini',
}

const DOCKERFILE_PREFIX = /^dockerfile([.-].*)?$/

const SHEBANG_LANGUAGES: ReadonlyArray<readonly [RegExp, EditorLanguage]> = [
  [/\bpython[0-9.]*\b/, 'python'],
  [/\b(?:ba|z|k)?sh\b/, 'shell'],
  [/\b(?:node|deno|bun)\b/, 'javascript'],
  [/\bruby[0-9.]*\b/, 'ruby'],
  [/\bperl[0-9.]*\b/, 'perl'],
  [/\bphp[0-9.]*\b/, 'php'],
  [/\blua[0-9.]*\b/, 'lua'],
]

function detectShebangLanguage(content?: string): EditorLanguage | null {
  if (!content) return null
  const firstLine = content.split(/\r?\n/, 1)[0]?.trim() ?? ''
  if (!firstLine.startsWith('#!')) return null
  const interpreter = firstLine.slice(2)
  for (const [pattern, language] of SHEBANG_LANGUAGES) {
    if (pattern.test(interpreter)) return language
  }
  return null
}

export function detectEditorLanguage(nameOrPath: string, content?: string): EditorLanguage {
  const name = (nameOrPath ?? '').split('/').filter(Boolean).at(-1) ?? ''
  const lower = name.toLowerCase()
  const base = BASENAME_LANGUAGES[lower]
  if (base) return base
  if (DOCKERFILE_PREFIX.test(lower)) return 'dockerfile'
  const dot = lower.lastIndexOf('.')

  if (dot <= 0 || dot === lower.length - 1) return detectShebangLanguage(content) ?? 'plaintext'
  return EXTENSION_LANGUAGES[lower.slice(dot + 1)] ?? 'plaintext'
}
