export type RemoteTextEditorTheme = 'light' | 'dark'

export type RemoteTextEditorEngine = 'monaco' | 'codemirror'

export interface RemoteTextEditorProps {
  modelValue: string

  language: string

  readonly?: boolean

  theme?: RemoteTextEditorTheme
}

export interface RemoteTextEditorExposed {
  focus(): void
  dispose(): void
}

export type EditorLanguage =
  | 'plaintext'
  | 'python'
  | 'shell'
  | 'yaml'
  | 'ini'
  | 'toml'
  | 'lua'
  | 'xml'
  | 'html'
  | 'markdown'
  | 'dockerfile'
  | 'sql'
  | 'javascript'
  | 'typescript'
  | 'go'
  | 'cpp'
  | 'rust'
  | 'php'
  | 'ruby'
  | 'perl'

const LANGUAGE_ALIASES: Readonly<Record<string, EditorLanguage>> = {
  plaintext: 'plaintext',
  text: 'plaintext',
  txt: 'plaintext',
  python: 'python',
  py: 'python',
  shell: 'shell',
  sh: 'shell',
  bash: 'shell',
  zsh: 'shell',
  ksh: 'shell',
  console: 'shell',
  yaml: 'yaml',
  yml: 'yaml',
  ini: 'ini',
  properties: 'ini',
  conf: 'ini',
  cfg: 'ini',
  toml: 'toml',
  lua: 'lua',
  xml: 'xml',
  svg: 'xml',
  xsd: 'xml',
  xsl: 'xml',
  html: 'html',
  htm: 'html',
  markdown: 'markdown',
  md: 'markdown',
  dockerfile: 'dockerfile',
  docker: 'dockerfile',
  sql: 'sql',
  mysql: 'sql',
  pgsql: 'sql',
  javascript: 'javascript',
  js: 'javascript',
  jsx: 'javascript',
  mjs: 'javascript',
  cjs: 'javascript',
  typescript: 'typescript',
  ts: 'typescript',
  tsx: 'typescript',
  go: 'go',
  cpp: 'cpp',
  c: 'cpp',
  h: 'cpp',
  hpp: 'cpp',
  cc: 'cpp',
  cxx: 'cpp',
  rust: 'rust',
  rs: 'rust',
  php: 'php',
  ruby: 'ruby',
  rb: 'ruby',
  perl: 'perl',
  pl: 'perl',
  pm: 'perl',
}

export function normalizeEditorLanguage(language: string): EditorLanguage {
  const key = (language ?? '').trim().toLowerCase()
  return LANGUAGE_ALIASES[key] ?? 'plaintext'
}
