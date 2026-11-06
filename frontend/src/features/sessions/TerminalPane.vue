<script setup lang="ts">
import { FitAddon } from '@xterm/addon-fit'
import { SearchAddon } from '@xterm/addon-search'
import { WebglAddon } from '@xterm/addon-webgl'
import { Terminal } from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  BookmarkPlus,
  Copy,
  ClipboardCheck,
  ClipboardPaste,
  Network,
  RefreshCw,
  Search,
  Settings,
  TextSelect,
  Unplug,
  Eraser,
  Circle,
  Pause,
  Square,
  Play,
  FolderOpen,
  Globe,
  History as HistoryIcon,
  KeyRound,
  Loader2,
  Trash2,
  WholeWord,
  LucideCaseSensitive,
  Regex,
  LucideRefreshCw,
  ArrowUp,
  ArrowDown,
  Columns2,
  Rows2,
  X,
  TriangleAlert,
  Languages,
  Sparkles,
} from '@lucide/vue'
import { toast } from 'vue-sonner'
import { registerTerminalTailReader, forgetTerminalTailReader } from './terminalTail'
import { registerTerminalInput, forgetTerminalInput, publishTerminalOutput } from './terminalInput'
import ShortcutKeycaps from '../settings/ShortcutKeycaps.vue'
import { QtermButton } from '@/components/common'
import { QtermDialog, QtermDialogContent, QtermDialogFooter, QtermDialogHeader } from '@/components/common/dialog'
import {
  TRANSLATE_PROVIDER_ORDER,
  createTranslateCredentialGateway,
  createTranslateGateway,
  isKeylessTranslateProvider,
  requiresTranslateEndpoint,
  type TranslateProvider,
} from '../../services/translate'
import {
  ContextMenu,
  ContextMenuTrigger,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuShortcut,
  ContextMenuSub,
  ContextMenuSubContent,
  ContextMenuSubTrigger,
} from '../../components/common/overlay-controls'
import { QtermInput } from '../../components/common/form-controls'
import { readClipboardText, writeClipboardText } from '../../services/clipboard'
import type { ILink } from '@xterm/xterm'
import { WEB_SEARCH_ENGINES, openExternalUrl, webSearchUrl, type WebSearchEngine } from '../../services/externalLink'
import { clearSessionDirectoryState } from '../../services/directory-follow'
import { type SessionKind, type SessionState, getSessionWarnings } from '../../services/sessions'
import {
  connectTerminalChannel,
  registerTerminalChannel,
  unregisterTerminalChannel,
  type TerminalChannel,
} from '../../services/terminal'
import { selectZmodemUpload, selectZmodemDestination } from '../../services/zmodem-files'
import { ZmodemManager, type ZmodemDirection, type ZmodemProgress } from '../../services/zmodem'
import alibabaCloudIcon from '../../assets/app-icons/alibaba-cloud.svg'
import baiduIcon from '../../assets/app-icons/baidu.svg'
import deeplIcon from '../../assets/app-icons/deepl.svg'
import duckduckgoIcon from '../../assets/app-icons/duckduckgo.svg'
import googleIcon from '../../assets/app-icons/google.svg'
import libreTranslateIcon from '../../assets/app-icons/libretranslate.svg'
import microsoftBingIcon from '../../assets/app-icons/microsoft-bing.svg'
import microsoftIcon from '../../assets/app-icons/microsoft.svg'
import youdaoIcon from '../../assets/app-icons/youdao.svg'
import { buildTerminalFontFamily, ensureTerminalFontReady, isTerminalFontAvailable } from './terminalFont'
import {
  exposeFontDiagnostics,
  installWkWebViewDomRendererMetrics,
  openTerminalWithDomFontMetrics,
} from './terminalDomFontMetrics'
import { useTerminalKeywordHighlights } from './useTerminalKeywordHighlights'
import { useTerminalPaneCommandBus, type TerminalPaneCommand } from './terminalPaneCommandBus'
import TerminalGutter from './TerminalGutter.vue'
import {
  DEFAULT_GUTTER_TIMESTAMP_FORMAT,
  GUTTER_BORDER_WIDTH,
  GUTTER_RIGHT_PADDING,
  formatGutterTimestamp,
} from './terminalGutter'
import { measureGutterText } from './terminalGutterTextWidth'
import { useTerminalGutter } from './useTerminalGutter'
import { useTerminalCommandGutter } from './useTerminalCommandGutter'
import { useTerminalLineTimestamps } from './useTerminalLineTimestamps'
import {
  boundShortcutAction,
  shortcutBindings,
  shortcutKeycapTokens,
  type ShortcutAction,
} from '../settings/shortcutStore'

import { detectMacPlatform, isMacPlatform } from '@/lib/platform.ts'
import type { SplitDirection } from './useSplitWorkspace'
import { getTerminalTheme, type TerminalThemeId } from './terminalThemes'
import { forgetWebglContext, hasOtherWebglOwner, retainWebglContext, updateWebglContext } from './webglContextPool'
import {
  detectCredentialPrompt,
  extractCredentialPromptLine,
  type CredentialPromptKind,
  type CredentialPromptRule,
  type PromptCredential,
} from './credentialAutofill'
import { decodeAppError, errorMessage, translateErrorKey } from '../../services/errors'
import { pauseRecording, resumeRecording, startRecording, stopRecording } from '../../services/recordings'

import { getRecordingState, markRecordingStopped, refreshRecordingState, setRecordingState } from './recordingStore'
import {
  createCommandLineBuffer,
  extractSubmittedScreenCommand,
  listCommandHistory,
  mergeShellTabCommand,
  resolveSubmittedCommand,
  saveCommandHistory,
  suppressCommandHistory,
} from '../../services/command-history'
import type { CommandHistory } from '../../../bindings/changeme/internal/model/models'
import type { KeywordHighlightRule, TerminalRenderEngine } from '../settings/settingsStore'
import { useI18n } from '@/locales'
import { ScrollArea } from '../../components/common/control-primitives'

const TERMINAL_MENU_SHORTCUTS = {
  copy: 'copyTerminalSelection',
  paste: 'pasteTerminal',
  search: 'openTerminalSearch',
  clear: 'clearTerminal',
  splitRight: 'splitRight',
  splitBottom: 'splitBottom',
  reconnect: 'reconnectSession',
  disconnect: 'disconnectSession',
} as const satisfies Record<string, ShortcutAction>

type TerminalMenuId = keyof typeof TERMINAL_MENU_SHORTCUTS

type TerminalMenuShortcutTokens = Record<TerminalMenuId, string[]>

const props = withDefaults(
  defineProps<{
    sessionId: string

    hostId?: string

    backspaceMode?: 'del' | 'bs'

    kind: SessionKind
    hostName: string
    username: string
    address: string
    port: number
    active: boolean

    visible?: boolean

    layoutRevision?: number
    dark: boolean
    transparentBackground?: boolean
    persistent: boolean

    fontSize: number

    lineHeight?: number

    gutterMode?: 'off' | 'lines' | 'commands'

    translateEnabled?: boolean
    translateTarget?: string
    translateProvider?: import('../../services/translate').TranslateProvider
    translateEndpoint?: string
    gutterShowTimestamps?: boolean

    gutterTimestampFormat?: string

    fontFamily: string
    lightTheme?: TerminalThemeId
    darkTheme?: TerminalThemeId
    autoCopySelection?: boolean
    copyFeedback?: boolean

    scrollback?: number

    confirmMultiLinePaste?: boolean
    completionEnabled?: boolean
    historyMinLength?: number
    historyMaxLength?: number
    wordSeparators?: string

    macOptionIsMeta?: boolean
    cursorBlink?: boolean
    cursorStyle?: 'block' | 'underline' | 'bar'
    rightClickBehavior?: 'none' | 'paste' | 'menu'

    modifierRightClickBehavior?: 'none' | 'paste' | 'menu'
    disconnectBehavior?: 'reconnect' | 'close'

    autoReconnect?: boolean

    autoReconnectPending?: boolean

    reconnecting?: boolean
    renderEngine?: TerminalRenderEngine
    keywordHighlightEnabled?: boolean
    keywordHighlightRules?: KeywordHighlightRule[]

    credentialPromptRules?: CredentialPromptRule[]
    credentials?: PromptCredential[]

    typeCredential?: (sessionId: string, credentialId: string) => Promise<void>
    state: SessionState
    streamToken: string
  }>(),
  {
    hostId: '',
    backspaceMode: 'del',
    visible: undefined,
    layoutRevision: 0,
    lineHeight: 1.25,
    gutterMode: 'off',
    translateEnabled: true,
    translateTarget: 'zh-CN',
    translateProvider: 'google',
    translateEndpoint: '',
    gutterShowTimestamps: true,
    gutterTimestampFormat: DEFAULT_GUTTER_TIMESTAMP_FORMAT,
    transparentBackground: false,
    autoCopySelection: false,
    copyFeedback: true,
    scrollback: 5000,
    confirmMultiLinePaste: true,
    completionEnabled: true,
    historyMinLength: 1,
    historyMaxLength: 4096,
    wordSeparators: ' ()[]{}\'"',
    macOptionIsMeta: false,
    cursorBlink: false,
    cursorStyle: 'block',
    rightClickBehavior: 'menu',
    modifierRightClickBehavior: 'none',
    disconnectBehavior: 'close',
    autoReconnect: false,
    autoReconnectPending: false,
    reconnecting: false,
    renderEngine: 'webgl',
    keywordHighlightEnabled: false,
    keywordHighlightRules: () => [],
    credentialPromptRules: () => [],
    credentials: () => [],
    typeCredential: undefined,
    lightTheme: 'default-light',
    darkTheme: 'default-dark',
  },
)

const { t } = useI18n()

const emit = defineEmits<{
  aiAnalyze: [sessionId: string, kind: 'selection' | 'recent', text: string]
  closeSession: [sessionId: string]
  reconnectSession: [sessionId: string]
  disconnectSession: [sessionId: string]
  openSftp: []
  openSettings: []

  saveAsSnippet: [content: string]

  requestSplit: [sessionId: string, direction: SplitDirection]
  presentationReady: [sessionId: string]
  streamStatus: [
    value: {
      sessionId: string
      state: 'ready' | 'closed' | 'failed'
      message?: string
    },
  ]
}>()

const terminalElement = ref<HTMLElement | null>(null)
const currentDirectory = ref('')
const streamStatus = ref<'idle' | 'opening' | 'ready' | 'closed' | 'failed'>('idle')
const menuOpen = ref(false)

const recordingState = computed(() => getRecordingState(props.sessionId))
const recordingBusy = ref(false)
const recordingChangedHandler = () => void refreshRecordingState(props.sessionId)
const selectedText = ref('')
const clipboardBusy = ref(false)

const shortcutTokens = computed<TerminalMenuShortcutTokens>(() => {
  const tokens = {} as TerminalMenuShortcutTokens
  for (const menuId of Object.keys(TERMINAL_MENU_SHORTCUTS) as TerminalMenuId[]) {
    const binding = shortcutBindings[TERMINAL_MENU_SHORTCUTS[menuId]] ?? ''

    tokens[menuId] = shortcutKeycapTokens(binding, isMacPlatform.value)
  }
  return tokens
})

function menuShortcutTokens(menuId: TerminalMenuId): string[] {
  return shortcutTokens.value[menuId]
}

function menuShortcutLabel(menuId: TerminalMenuId): string {
  return menuShortcutTokens(menuId).join(' + ')
}

const searchOpen = ref(false)
const searchQuery = ref('')
const searchCaseSensitive = ref(false)
const searchRegex = ref(false)
const searchWholeWord = ref(false)
const searchWrap = ref(true)
const searchResultIndex = ref(-1)
const searchResultCount = ref(0)
let searchAddon: SearchAddon | null = null
let searchResultSubscription: { dispose: () => void } | null = null

const searchInput = ref<InstanceType<typeof QtermInput> | null>(null)
let restoreMenuFocus = true

const zmodemActive = ref(false)
let terminal: Terminal | null = null

let fitAddon: FitAddon | null = null
let webglAddon: WebglAddon | null = null
const webglContextOwner = {}

let webglAddonDpr = 0
let resizeObserver: ResizeObserver | null = null
let resizeNotifyTimer: ReturnType<typeof setTimeout> | null = null
let fitAfterLayoutResize = false
let initialLayoutReady = false
let dataSubscription: { dispose(): void } | null = null

let isTerminalPaneUnmounted = false

let commandLineBuffer = createCommandLineBuffer()

const completionOpen = ref(false)
const completionItems = ref<string[]>([])

const completionIndex = ref(-1)
const completionQuery = ref('')
const completionPosition = ref({ left: 18, top: 24 })
let completionHistory: CommandHistory[] = []
let typedCommand = ''
let commandLineHasShellTab = false

let commandLineHasShellHistoryRecall = false

let shellTabPrefix: string | null = null
let shellTabCompletedCommand = ''
let pendingShellTabSubmission: { typed: string; terminator: '\r' | '\n' } | null = null
let shellTabSubmitTimer: ReturnType<typeof setTimeout> | null = null
const SHELL_TAB_SUBMIT_TIMEOUT_MS = 300

function fuzzyScore(command: string, query: string): number | null {
  const value = command.toLocaleLowerCase()
  const needle = query.toLocaleLowerCase()
  if (!needle) return 0
  const direct = value.indexOf(needle)
  if (direct >= 0) return direct
  let cursor = 0
  let score = 0
  for (const char of needle) {
    const index = value.indexOf(char, cursor)
    if (index < 0) return null
    score += index - cursor
    cursor = index + 1
  }
  return score + 1000
}

function completionMatchParts(command: string, query: string): Array<{ text: string; matched: boolean }> {
  const chars = Array.from(command)
  const needle = Array.from(query.toLocaleLowerCase())
  if (!needle.length) return [{ text: command, matched: false }]
  const lower = chars.map((char) => char.toLocaleLowerCase())
  const matched = new Set<number>()
  let start = -1
  for (let index = 0; index <= lower.length - needle.length; index += 1) {
    if (lower.slice(index, index + needle.length).join('') === needle.join('')) {
      start = index
      break
    }
  }
  if (start >= 0) {
    for (let index = start; index < start + needle.length; index += 1) matched.add(index)
  } else {
    let cursor = 0
    for (const char of needle) {
      const index = lower.findIndex((candidate, candidateIndex) => candidateIndex >= cursor && candidate === char)
      if (index < 0) break
      matched.add(index)
      cursor = index + 1
    }
  }
  return chars.reduce<Array<{ text: string; matched: boolean }>>((parts, char, index) => {
    const matchedPart = matched.has(index)
    const previous = parts.at(-1)
    if (previous?.matched === matchedPart) previous.text += char
    else parts.push({ text: char, matched: matchedPart })
    return parts
  }, [])
}

function updateCompletionPosition(): void {
  const screen = terminalElement.value?.querySelector('.xterm-screen') as HTMLElement | null
  if (!screen || !terminal) return
  const rect = screen.getBoundingClientRect()
  const cellWidth = rect.width / Math.max(1, terminal.cols)
  const cellHeight = rect.height / Math.max(1, terminal.rows)
  const left = rect.left + terminal.buffer.active.cursorX * cellWidth
  const anchorBottom = rect.top + (terminal.buffer.active.cursorY + 1) * cellHeight + 4
  const popup = document.querySelector<HTMLElement>('.terminal-command-completion')
  const popupHeight = popup?.getBoundingClientRect().height ?? 330
  const popupWidth = popup?.getBoundingClientRect().width ?? 380
  const showBelow = window.innerHeight - anchorBottom >= popupHeight + 8
  const top = showBelow ? anchorBottom : rect.top + terminal.buffer.active.cursorY * cellHeight - popupHeight - 4
  completionPosition.value = {
    left: Math.max(8, Math.min(window.innerWidth - popupWidth - 8, left)),
    top: Math.max(8, Math.min(window.innerHeight - popupHeight - 8, top)),
  }
}

const credentialOpen = ref(false)
const credentialKind = ref<CredentialPromptKind>('password')
const credentialMatches = ref<PromptCredential[]>([])
const credentialIndex = ref(-1)
const credentialPosition = ref({ left: 18, top: 24 })
const credentialPromptLine = ref('')

const credentialInputKind = ref<CredentialPromptKind | null>(null)
const credentialInputPromptLine = ref('')
const CREDENTIAL_OUTPUT_TAIL_LIMIT = 4096
let credentialOutputTail = ''

let credentialOutputDecoder = new TextDecoder('utf-8')

function closeCredentialSuggestion(): void {
  credentialOpen.value = false
  credentialMatches.value = []
  credentialIndex.value = -1
  credentialPromptLine.value = ''
}

function resetCommandInputTracking(): void {
  typedCommand = ''
  closeCompletion()
  commandLineBuffer = createCommandLineBuffer()
  pendingShellTabSubmission = null
  clearShellTabSubmitTimer()
  clearShellTabState()
}

function clearCredentialInputState(): void {
  credentialInputKind.value = null
  credentialInputPromptLine.value = ''
  credentialOutputTail = ''
  credentialOutputDecoder = new TextDecoder('utf-8')
  closeCredentialSuggestion()
  resetCommandInputTracking()
}

function completesCredentialInput(data: string): boolean {
  return data.includes('\r') || data.includes('\n') || data.includes('\x03') || data === '\x1b' || data === '\x07'
}

function waitForCredentialResponse(): void {
  credentialOutputTail = ''
  credentialOutputDecoder = new TextDecoder('utf-8')
  closeCredentialSuggestion()
  resetCommandInputTracking()
}

function isLikelyShellPrompt(line: string): boolean {
  const prompt = line.trim()

  return prompt.length > 0 && prompt.length <= 300 && /[#$%>❯]\s*$/u.test(prompt)
}

function updateCredentialPosition(): void {
  const screen = terminalElement.value?.querySelector('.xterm-screen') as HTMLElement | null
  if (!screen || !terminal) return
  const rect = screen.getBoundingClientRect()
  const cellWidth = rect.width / Math.max(1, terminal.cols)
  const cellHeight = rect.height / Math.max(1, terminal.rows)
  const left = rect.left + terminal.buffer.active.cursorX * cellWidth
  const anchorBottom = rect.top + (terminal.buffer.active.cursorY + 1) * cellHeight + 4
  const popup = document.querySelector<HTMLElement>('.terminal-credential-suggestion')
  const popupHeight = popup?.getBoundingClientRect().height ?? 240
  const popupWidth = popup?.getBoundingClientRect().width ?? 340
  const showBelow = window.innerHeight - anchorBottom >= popupHeight + 8
  const top = showBelow ? anchorBottom : rect.top + terminal.buffer.active.cursorY * cellHeight - popupHeight - 4
  credentialPosition.value = {
    left: Math.max(8, Math.min(window.innerWidth - popupWidth - 8, left)),
    top: Math.max(8, Math.min(window.innerHeight - popupHeight - 8, top)),
  }
}

function checkCredentialPrompt(): void {
  if (!props.persistent || props.state !== 'connected') {
    if (credentialInputKind.value !== null) clearCredentialInputState()
    else closeCredentialSuggestion()
    return
  }
  const match = detectCredentialPrompt(credentialOutputTail, props.credentialPromptRules, props.credentials)
  const fallbackLine = match ? '' : extractCredentialPromptLine(credentialOutputTail)
  const detectedKind = match?.kind ?? (isSensitivePrompt(fallbackLine) ? 'password' : null)
  if (!detectedKind) {
    if (credentialInputKind.value !== null && isLikelyShellPrompt(fallbackLine)) {
      clearCredentialInputState()
      return
    }

    closeCredentialSuggestion()
    return
  }
  const promptLine = match?.line ?? fallbackLine
  const isNewPrompt = credentialInputKind.value !== detectedKind || credentialInputPromptLine.value !== promptLine
  if (isNewPrompt) {
    credentialInputKind.value = detectedKind
    credentialInputPromptLine.value = promptLine

    resetCommandInputTracking()
  }

  closeCompletion()

  if (!match || match.credentials.length === 0) {
    closeCredentialSuggestion()
    return
  }

  if (credentialOpen.value && credentialPromptLine.value === match.line && credentialKind.value === match.kind) return
  credentialKind.value = match.kind
  credentialMatches.value = match.credentials
  credentialPromptLine.value = match.line
  credentialIndex.value = 0
  credentialOpen.value = true
  void nextTick(updateCredentialPosition)
}

function queueCredentialPromptCheck(): void {
  checkCredentialPrompt()
}

async function fillCredential(credential: PromptCredential | undefined): Promise<void> {
  if (!credential) return
  const kind = credentialKind.value
  closeCredentialSuggestion()
  terminal?.focus()

  if (kind === 'username') {
    const username = credential.username?.trim()
    if (!username || !channel) return
    waitForCredentialResponse()
    channel.sendInput(`${username}\r`)
    return
  }
  if (!props.typeCredential) return
  waitForCredentialResponse()
  try {
    await props.typeCredential(props.sessionId, credential.id)
  } catch (injectError) {
    toast.error(errorMessage(injectError, t('shell.error.credentialInject')))
  }
}

function handleCredentialKey(event: KeyboardEvent): boolean {
  if (!credentialOpen.value) return true
  if (event.type !== 'keydown' || event.isComposing) return true
  if (!['Escape', 'ArrowDown', 'ArrowUp', 'Enter'].includes(event.key)) return true
  consumedCompletionKeys.add(event.key)
  event.preventDefault()
  event.stopPropagation()
  if (event.key === 'Escape') {
    closeCredentialSuggestion()
    return false
  }
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    credentialIndex.value =
      (credentialIndex.value + (event.key === 'ArrowDown' ? 1 : -1) + credentialMatches.value.length) %
      credentialMatches.value.length

    void nextTick(() => {
      document
        .querySelector<HTMLElement>('.terminal-credential-suggestion .terminal-command-completion-item.is-selected')
        ?.scrollIntoView({ block: 'nearest' })
    })
    return false
  }
  if (event.key === 'Enter') {
    void fillCredential(credentialMatches.value[credentialIndex.value])
  }
  return false
}

function refreshCompletionItems(): void {
  const query = typedCommand.trim()
  completionQuery.value = query

  if (!props.hostId || !props.completionEnabled || !query || /\s$/.test(typedCommand)) {
    completionItems.value = []
    completionOpen.value = false
    return
  }
  const seen = new Set<string>()
  completionItems.value = completionHistory
    .filter((item) => !item.suppressed)
    .map((item, order) => ({ command: item.command.trim(), score: fuzzyScore(item.command, query), order }))
    .filter((item): item is { command: string; score: number; order: number } =>
      Boolean(item.command && item.score !== null),
    )
    .filter((item) => {
      const length = Array.from(item.command).length
      return length >= (props.historyMinLength ?? 1) && length <= (props.historyMaxLength ?? 4096)
    })
    .filter((item) => {
      if (seen.has(item.command)) return false
      seen.add(item.command)
      return true
    })
    .sort((a, b) => a.score - b.score || a.order - b.order)
    .slice(0, 100)
    .map((item) => item.command)

  completionIndex.value = -1
  completionOpen.value = completionItems.value.length > 0
  if (completionOpen.value) void nextTick(updateCompletionPosition)
}

async function loadCompletionHistory(): Promise<void> {
  if (!props.hostId) {
    completionHistory = []
    return
  }
  try {
    completionHistory = await listCommandHistory(props.hostId)
    refreshCompletionItems()
  } catch {
    completionHistory = []
  }
}

function closeCompletion(): void {
  completionOpen.value = false
  completionItems.value = []
  completionIndex.value = -1
}

function focusCompletionTerminal(): void {
  terminal?.focus()
}

function clearShellTabState(): void {
  commandLineHasShellTab = false
  commandLineHasShellHistoryRecall = false
  shellTabPrefix = null
  shellTabCompletedCommand = ''
}

function clearShellTabSubmitTimer(): void {
  if (shellTabSubmitTimer === null) return
  window.clearTimeout(shellTabSubmitTimer)
  shellTabSubmitTimer = null
}

function resetTypedCommand(): void {
  typedCommand = ''
  closeCompletion()
}

function trackTypedInput(data: string): void {
  for (const char of data) {
    const code = char.codePointAt(0) ?? 0
    if (char === '\r' || char === '\n' || char === '\x03' || char === '\x15') {
      resetTypedCommand()
    } else if (char === '\x7f' || char === '\b') {
      typedCommand = Array.from(typedCommand).slice(0, -1).join('')
      refreshCompletionItems()
    } else if (char === '\x17') {
      typedCommand = typedCommand.replace(/\s+\S*$/, '')
      refreshCompletionItems()
    } else if (code >= 0x20 && code !== 0x7f) {
      typedCommand += char
      refreshCompletionItems()
    }
  }
}

function sendCompletionCommand(command: string, execute: boolean): void {
  if (!channel || !command) return
  const input = `\x15${command}${execute ? '\r' : ''}`

  pendingShellTabSubmission = null
  clearShellTabSubmitTimer()
  clearShellTabState()
  commandLineBuffer.feed(input)
  channel.sendInput(input)
  if (execute && props.hostId) {
    void saveCommandHistory({
      hostId: props.hostId,
      hostName: props.hostName,
      sessionId: props.sessionId,
      command,
    }).catch(() => undefined)
  }
  typedCommand = execute ? '' : command
  closeCompletion()
  terminal?.focus()
}

async function hideCompletionCommand(command: string): Promise<void> {
  if (!props.hostId || !command) return
  const hostId = props.hostId
  try {
    await suppressCommandHistory(hostId, command)
    if (props.hostId !== hostId) return
    await loadCompletionHistory()
  } catch {
    toast.error(t('shell.toolPanel.deleteHistoryFailed'))
  }
}

const consumedCompletionKeys = new Set<string>()

function handleCompletionKey(event: KeyboardEvent): boolean {
  if (event.type !== 'keydown' && consumedCompletionKeys.has(event.key)) {
    if (event.type === 'keyup') consumedCompletionKeys.delete(event.key)
    event.preventDefault()
    event.stopPropagation()
    return false
  }
  if (!completionOpen.value || completionItems.value.length === 0) return true

  if (event.type !== 'keydown' || event.isComposing) return true
  if (!['Escape', 'ArrowDown', 'ArrowUp', 'Tab', 'Enter'].includes(event.key)) return true
  if ((event.key === 'Enter' || event.key === 'Tab') && completionIndex.value < 0) {
    closeCompletion()
    return true
  }
  consumedCompletionKeys.add(event.key)
  event.preventDefault()
  event.stopPropagation()
  if (event.key === 'Escape') {
    closeCompletion()
    return false
  }
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    completionIndex.value =
      (completionIndex.value + (event.key === 'ArrowDown' ? 1 : -1) + completionItems.value.length) %
      completionItems.value.length
    void nextTick(() => {
      document
        .querySelector<HTMLElement>('.terminal-command-completion-item.is-selected')
        ?.scrollIntoView({ block: 'nearest' })
    })
    return false
  }
  if (event.key === 'Tab') {
    sendCompletionCommand(completionItems.value[completionIndex.value] ?? '', false)
    return false
  }
  if (event.key === 'Enter') {
    sendCompletionCommand(completionItems.value[completionIndex.value] ?? '', true)
    return false
  }
  return true
}

function currentScreenLine(target: Terminal | null): string {
  const buffer = target?.buffer.active
  if (!buffer) return ''
  const row = buffer.baseY + buffer.cursorY

  let text = buffer.getLine(row)?.translateToString(false) ?? ''

  let index = row
  while (index > 0 && buffer.getLine(index)?.isWrapped) {
    index -= 1
    text = (buffer.getLine(index)?.translateToString(false) ?? '') + text
  }
  return text
}

function isShellHistoryNavigation(data: string): boolean {
  return data === '\x1b[A' || data === '\x1b[B' || data === '\x1bOA' || data === '\x1bOB'
}

function captureShellTabCommand(rawCommand = typedCommand): void {
  if (!terminal) return
  const screenLine = currentScreenLine(terminal)
  if (commandLineHasShellHistoryRecall) {
    const candidate = extractSubmittedScreenCommand(screenLine)
    if (candidate) shellTabCompletedCommand = candidate
    return
  }
  if (shellTabPrefix === null) return
  const candidate = resolveSubmittedCommand(screenLine, shellTabPrefix).trim()
  const prefix = shellTabPrefix.trim()
  if (!candidate || candidate.length <= prefix.length) return

  const rawSuffix = rawCommand.startsWith(shellTabPrefix) ? rawCommand.slice(shellTabPrefix.length).trim() : ''
  if (rawSuffix && !candidate.endsWith(rawSuffix)) return
  const completedPrefix = rawSuffix ? candidate.slice(0, -rawSuffix.length).trim() : candidate
  if (!shellTabCompletedCommand || completedPrefix.startsWith(shellTabCompletedCommand)) {
    shellTabCompletedCommand = completedPrefix
  }
}

function resolveShellTabSubmission(screenLine: string, typed: string): string {
  if (commandLineHasShellHistoryRecall) return extractSubmittedScreenCommand(screenLine) || typed
  if (shellTabPrefix === null) return resolveSubmittedCommand(screenLine, typed)
  return mergeShellTabCommand(screenLine, typed, shellTabPrefix, shellTabCompletedCommand)
}

function flushPendingShellTabSubmission(): void {
  const pending = pendingShellTabSubmission
  if (!pending) return
  pendingShellTabSubmission = null
  clearShellTabSubmitTimer()
  const command = resolveShellTabSubmission(currentScreenLine(terminal), pending.typed)
  channel?.sendInput(pending.terminator)
  if (props.hostId) {
    void saveCommandHistory({
      hostId: props.hostId,
      hostName: props.hostName,
      sessionId: props.sessionId,
      command,
    }).catch(() => undefined)
  }
  clearShellTabState()
}

function isSensitivePrompt(screenLine: string): boolean {
  const ansiEscape = String.fromCharCode(0x1b)
  const prompt = screenLine.replace(new RegExp(`${ansiEscape}\\[[0-?]*[ -/]*[@-~]`, 'g'), '').trim()

  if (/[#$%>]\s+\S/.test(prompt)) return false

  return /(?:^|[\s\u4e00-\u9fff])(?:password|passphrase|密码|口令|验证码)(?:\s+for\b[^\r\n]{0,120})?\s*[:：]?\s*$/iu.test(
    prompt,
  )
}
let channel: TerminalChannel | null = null
let zmodemManager: ZmodemManager | null = null
let connectedToken = ''

let connectedSessionID = ''

let channelGeneration = 0

let disconnectMarked = false

let reconnectingMarked = false

let paintPresentedOnce = false
let lastColumns = 0
let lastRows = 0
let closePromptWritten = false
let closingRequested = false
let oscSubscription: { dispose(): void } | null = null
let selectionSubscription: { dispose(): void } | null = null

let zmodemCancelHandler: ((e: Event) => void) | null = null
let localFileDropHandler: ((e: Event) => void) | null = null
let terminalThemeUpdatedHandler: ((e: Event) => void) | null = null
let outputSubscription: { dispose: () => void } | null = null

let registeredTerminalSessionID = ''
let outputCursor = 0

function registerTerminalBridge(sessionId: string): void {
  if (!terminal || !sessionId) return
  if (registeredTerminalSessionID === sessionId) return

  if (registeredTerminalSessionID) {
    forgetTerminalInput(registeredTerminalSessionID)
    forgetTerminalTailReader(registeredTerminalSessionID)
  }

  const buffer = terminal.buffer.active
  outputCursor = buffer.cursorY + buffer.baseY
  const boundSessionID = sessionId
  registerTerminalInput(boundSessionID, (payload: string) => {
    if (connectedSessionID !== boundSessionID || streamStatus.value !== 'ready' || !channel) return false
    return channel.sendInput(payload) !== false
  })
  registerTerminalTailReader(boundSessionID, () => {
    const current = terminal?.buffer.active
    if (!current) return ''
    const end = Math.min(current.length, current.cursorY + current.baseY + 1)
    const start = Math.max(0, end - 80)
    const lines: string[] = []
    for (let index = start; index < end; index += 1) {
      lines.push(current.getLine(index)?.translateToString(true) ?? '')
    }
    return lines.join('\n').trimEnd()
  })
  registeredTerminalSessionID = boundSessionID
}

function forgetTerminalBridge(sessionId = registeredTerminalSessionID): void {
  if (!sessionId) return
  forgetTerminalInput(sessionId)
  forgetTerminalTailReader(sessionId)
  if (registeredTerminalSessionID === sessionId) registeredTerminalSessionID = ''
}

function bindTerminalOutputCapture(sessionId: string): void {
  outputSubscription?.dispose()
  outputSubscription = null
  if (!terminal || !sessionId) return

  const boundSessionID = sessionId
  outputSubscription = terminal.onWriteParsed(() => {
    const current = terminal?.buffer.active
    if (!current) return
    const bottom = current.cursorY + current.baseY
    if (bottom < outputCursor) outputCursor = 0
    while (outputCursor < bottom) {
      const line = current.getLine(outputCursor)?.translateToString(true) ?? ''
      if (connectedSessionID === boundSessionID && registeredTerminalSessionID === boundSessionID) {
        publishTerminalOutput(boundSessionID, `${line}\n`)
      }
      outputCursor += 1
    }
  })
}

let firstOutputFontCorrectionPending = true
let firstOutputFontActivation: Promise<void> | null = null

let initialFontPrepared = false

const webglActivationReady = ref(false)

const fontPresentationReady = ref(false)

const layoutPresentationReady = ref(true)

const presentationPending = ref(Boolean(props.visible ?? props.active))

let preserveVisibleFrameDuringRecovery = false
interface TerminalFrameSnapshot {
  source: HTMLCanvasElement

  cssWidth: number
  cssHeight: number

  presentation: HTMLCanvasElement | null
}

let terminalFrameSnapshot: TerminalFrameSnapshot | null = null
const frameSnapshotAvailable = ref(false)
let windowResizeRecoveryTimer: ReturnType<typeof setTimeout> | null = null

const keywordHighlights = useTerminalKeywordHighlights({
  terminal: () => terminal,
  enabled: () => props.keywordHighlightEnabled,
  rules: () => props.keywordHighlightRules,
})

const isVisible = computed(() => props.visible ?? props.active)

let focusOnFirstOutputDone = false

function focusTerminalIfAllowed(): void {
  const target = terminal
  if (!target || !props.active || !isVisible.value) return

  const activeElement = document.activeElement
  const insidePane = activeElement instanceof Node && paneRoot.value?.contains(activeElement)
  const typingElsewhere =
    !insidePane &&
    activeElement instanceof HTMLElement &&
    (activeElement.tagName === 'INPUT' || activeElement.tagName === 'TEXTAREA' || activeElement.isContentEditable)
  if (typingElsewhere) return
  target.focus()
}

function scheduleTerminalFocus(): void {
  void nextTick(() => focusTerminalIfAllowed())
  window.setTimeout(() => focusTerminalIfAllowed(), 120)
  window.setTimeout(() => focusTerminalIfAllowed(), 400)
}

watch(
  () => [props.active, isVisible.value, streamStatus.value] as const,
  ([active, visible, status]) => {
    if (!active || !visible || status !== 'ready') return
    scheduleTerminalFocus()
  },
)

const gutter = useTerminalGutter({
  enabled: () => props.gutterMode !== 'off',
  visible: () => isVisible.value,
})
const {
  rows: gutterRows,
  width: gutterWidth,
  rowHeight: gutterRowHeight,
  topPadding: gutterTopPadding,
  alternateScreen: gutterAlternateScreen,
} = gutter
const commandGutter = useTerminalCommandGutter({
  visible: () => isVisible.value && props.gutterMode === 'commands',
  approximateTitle: () => t('terminal.core.commandApproximate'),
})
const lineTimestamps = useTerminalLineTimestamps()

function timestampForGutterOffset(offset: number): number | undefined {
  void lineTimestamps.revision.value
  const buffer = terminal?.buffer.active
  if (!buffer || buffer.type === 'alternate') return undefined
  return lineTimestamps.timestampAt(buffer.viewportY + offset)
}

function decorateGutterTime(row: (typeof gutterRows.value)[number]) {
  const timestamp = timestampForGutterOffset(row.offset)
  if (timestamp === undefined) return row
  const time = new Date(timestamp)
  const stamp = formatGutterTimestamp(timestamp, props.gutterTimestampFormat)
  return {
    ...row,

    label: props.gutterShowTimestamps ? `${stamp} · ${row.label ?? row.lineNumber}` : row.label,
    title: [time.toLocaleString(), row.title].filter(Boolean).join(' · '),
  }
}

const displayedGutterRows = computed(() => {
  if (props.gutterMode !== 'commands') return gutterRows.value.map(decorateGutterTime)

  const commandsByOffset = new Map(commandGutter.rows.value.map((row) => [row.offset, row]))
  return gutterRows.value.flatMap((row) => {
    const command = commandsByOffset.get(row.offset)
    if (!command) return []

    return [decorateGutterTime({ ...row, key: command.lineNumber, lineNumber: command.lineNumber })]
  })
})

let gutterWidthHighWater = 0
let gutterWidthFontKey = ''
const displayedGutterWidth = computed(() => {
  const fontKey = `${gutterFontFamily.value}|${props.fontSize}`
  if (fontKey !== gutterWidthFontKey) {
    gutterWidthFontKey = fontKey
    gutterWidthHighWater = 0
  }
  let widest = 0
  for (const row of displayedGutterRows.value) {
    const text = row.label ?? String(row.lineNumber)
    widest = Math.max(widest, measureGutterText(text, gutterFontFamily.value, props.fontSize))
  }
  const current = Math.max(gutterWidth.value, Math.ceil(widest) + GUTTER_RIGHT_PADDING + GUTTER_BORDER_WIDTH)
  gutterWidthHighWater = Math.max(gutterWidthHighWater, current)
  return gutterWidthHighWater
})

const gutterColor = computed(() => terminalTheme(props.dark).foreground ?? 'currentColor')
const gutterFontFamily = computed(() => buildTerminalFontFamily(props.fontFamily))

const gutterSuspended = computed(() => !fontPresentationReady.value || !layoutPresentationReady.value)
const gutterVisible = computed(() => props.gutterMode !== 'off' && !gutterAlternateScreen.value)

function handleGutterWheel(deltaY: number, deltaMode: number): void {
  if (!terminal) return
  const perLine = deltaMode === 1 ? 1 : gutterRowHeight.value > 0 ? gutterRowHeight.value : 16
  const lines = Math.min(terminal.rows, Math.max(1, Math.round(Math.abs(deltaY) / perLine)))
  terminal.scrollLines(deltaY > 0 ? lines : -lines)
}

function handleGutterFocus(): void {
  if (!terminal || !props.active) return
  terminal.focus()
}

watch(
  () => props.gutterMode,
  (enabled) => {
    if (enabled !== 'off') {
      gutter.sync()
      commandGutter.sync()
    }
  },
)

function logTerminalPresentationDiagnostics(): void {
  if (!import.meta.env.DEV || import.meta.env.MODE === 'test') return
  window.setTimeout(() => {
    if (fontPresentationReady.value && !presentationPending.value) return
    const pane = terminalElement.value
    const rows = pane?.querySelector('.xterm-rows') ?? null
    const canvas = pane?.querySelector('canvas') ?? null
    const paneStyle = pane ? window.getComputedStyle(pane) : null
    console.info('[TerminalPane] presentation diagnostics', {
      hasPane: Boolean(pane),
      hasTerminal: Boolean(terminal),
      streamStatus: streamStatus.value,
      sessionState: props.state,

      persistent: props.persistent,
      streamToken: props.streamToken ? 'set' : 'empty',
      isVisible: isVisible.value,
      visibleProp: props.visible,
      activeProp: props.active,
      gate: {
        font: fontPresentationReady.value,
        layout: layoutPresentationReady.value,
        pending: presentationPending.value,
      },
      pane: pane
        ? { w: pane.clientWidth, h: pane.clientHeight, visibility: paneStyle?.visibility, className: pane.className }
        : null,
      canvas: canvas instanceof HTMLCanvasElement ? { w: canvas.width, h: canvas.height } : null,
      rowsElement: rows ? { children: rows.childElementCount } : null,
      terminal: terminal ? { rows: terminal.rows, cols: terminal.cols } : null,
    })
  }, 2000)
}

const PRESENTATION_WATCHDOG_MS = 2500
let presentationWatchdog: number | undefined

function armPresentationWatchdog(): void {
  if (presentationWatchdog !== undefined) return
  presentationWatchdog = window.setTimeout(() => {
    presentationWatchdog = undefined
    if (!isVisible.value || !terminal) return
    const fontStuck = !fontPresentationReady.value
    const pendingStuck = presentationPending.value
    if (!fontStuck && !pendingStuck) return
    if (import.meta.env.DEV && import.meta.env.MODE !== 'test') {
      console.warn('[TerminalPane] presentation watchdog fired', {
        sessionId: props.sessionId,
        fontStuck,
        pendingStuck,
        preserveFrame: preserveVisibleFrameDuringRecovery,
        rows: terminal.rows,
        cols: terminal.cols,
      })
    }
    revealTerminalPresentation()
  }, PRESENTATION_WATCHDOG_MS)
}

function revealTerminalPresentation(): void {
  disarmPresentationWatchdog()
  preserveVisibleFrameDuringRecovery = false
  layoutPresentationReady.value = true
  if (!fontPresentationReady.value) fontPresentationReady.value = true
  if (presentationPending.value) {
    presentationPending.value = false
    emit('presentationReady', props.sessionId)
  }
}

function disarmPresentationWatchdog(): void {
  if (presentationWatchdog === undefined) return
  window.clearTimeout(presentationWatchdog)
  presentationWatchdog = undefined
}

watch(
  isVisible,
  (visible) => {
    if (visible) armPresentationWatchdog()
    else disarmPresentationWatchdog()
  },
  { immediate: true },
)

watch(fontPresentationReady, (ready) => {
  if (!ready) return

  paintPresentedOnce = true
  if (isVisible.value && !preserveVisibleFrameDuringRecovery) presentationPending.value = false

  repaintTerminal()
})

const ended = computed(
  () => ['closed', 'failed'].includes(props.state) || ['closed', 'failed'].includes(streamStatus.value),
)

const autoReconnectOwnsEnd = computed(() => props.autoReconnect === true && props.autoReconnectPending === true)

const CLOSE_PROMPT_AUTO_RECONNECT_GRACE_MS = 400
let closePromptTimer: ReturnType<typeof setTimeout> | null = null

function clearClosePromptTimer(): void {
  if (closePromptTimer !== null) clearTimeout(closePromptTimer)
  closePromptTimer = null
}

const sessionReady = computed(() => props.state === 'connected' && streamStatus.value === 'ready')
watch(sessionReady, (ready) => {
  if (ready) void showSessionWarnings()
})

const canPaste = computed(
  () =>
    props.state === 'connected' &&
    streamStatus.value === 'ready' &&
    !ended.value &&
    !zmodemActive.value &&
    !clipboardBusy.value,
)

function onMenuOpen(open: boolean): void {
  menuOpen.value = open
  if (open) {
    selectedText.value = terminal?.getSelection() ?? ''

    if (props.translateEnabled && selectedText.value.trim()) void loadConfiguredTranslateProviders()
    restoreMenuFocus = true
  } else {
    selectedText.value = ''
  }
}

async function changeRecording(action: 'start' | 'pause' | 'resume' | 'stop'): Promise<void> {
  if (recordingBusy.value) return
  recordingBusy.value = true
  try {
    if (action === 'start') {
      await startRecording(props.sessionId)

      setRecordingState(props.sessionId, 'segment', 'recording')
    }
    if (action === 'pause') await pauseRecording(props.sessionId)
    if (action === 'resume') await resumeRecording(props.sessionId)
    if (action === 'stop') {
      await stopRecording(props.sessionId)

      markRecordingStopped(props.sessionId)
    }
    await refreshRecordingState(props.sessionId)
  } catch (error) {
    if (decodeAppError(error)?.messageKey === 'recording.already_active') {
      await refreshRecordingState(props.sessionId)
      return
    }
    toast.error(error instanceof Error ? error.message : t('shell.terminal.recordingFailed'))
  } finally {
    recordingBusy.value = false
  }
}

function focusAfterMenu(): void {
  if (restoreMenuFocus && props.active && isVisible.value) terminal?.focus()
}

function preserveOutsideFocus(): void {
  restoreMenuFocus = false
}

async function copySelection(): Promise<void> {
  const text = selectedText.value
  if (!text || clipboardBusy.value) return
  clipboardBusy.value = true
  try {
    await writeClipboardText(text)
    if (props.copyFeedback) toast.success(t('shell.terminal.copySuccess'))
  } catch {
    toast.error(t('shell.terminal.copyFailed'))
  } finally {
    clipboardBusy.value = false
  }
}

async function copyAddress(): Promise<void> {
  const address = props.address.trim()
  if (!address || clipboardBusy.value) return
  clipboardBusy.value = true
  try {
    await writeClipboardText(address)
    if (props.copyFeedback) toast.success(t('shell.terminal.copyAddressSuccess'))
  } catch {
    toast.error(t('shell.terminal.copyFailed'))
  } finally {
    clipboardBusy.value = false
  }
}

function commitPaste(text: string): void {
  terminal?.paste(text)
}

function requestPaste(text: string): void {
  if (!text) return
  if (props.confirmMultiLinePaste && /\r|\n/.test(text.replace(/[\r\n]+$/, ''))) {
    pendingPaste.value = text
    return
  }
  commitPaste(text)
}

function handleNativePaste(event: ClipboardEvent): void {
  if (!props.confirmMultiLinePaste) return
  const text = event.clipboardData?.getData('text') ?? ''
  if (!text || !/\r|\n/.test(text.replace(/[\r\n]+$/, ''))) return
  event.preventDefault()
  event.stopPropagation()
  pendingPaste.value = text
}

function confirmPendingPaste(): void {
  const text = pendingPaste.value
  pendingPaste.value = null
  if (text) commitPaste(text)
}

function pasteSelection(): void {
  if (!canPaste.value || !terminal) return
  const text = selectedText.value || terminal.getSelection()
  if (!text) return
  requestPaste(text)
}

const WEB_SEARCH_ENGINE_ICONS: Record<WebSearchEngine, string> = {
  google: googleIcon,
  bing: microsoftBingIcon,
  baidu: baiduIcon,
  duckduckgo: duckduckgoIcon,
}

const webSearchEngines: Array<{ id: WebSearchEngine; labelKey: string; icon: string }> = WEB_SEARCH_ENGINES.map(
  (id) => ({
    id,
    labelKey: `shell.terminal.webSearchEngine.${id}`,
    icon: WEB_SEARCH_ENGINE_ICONS[id],
  }),
)

function currentSelection(): string {
  return selectedText.value || terminal?.getSelection() || ''
}

const translateGateway = createTranslateGateway()

const paneRoot = ref<HTMLElement | null>(null)
const pendingPaste = ref<string | null>(null)
const pendingPasteLines = computed(() =>
  pendingPaste.value ? pendingPaste.value.replace(/\n$/, '').split('\n').length : 0,
)
const pendingPastePreview = computed(() => (pendingPaste.value ?? '').split('\n').slice(0, 3).join('\n'))

watch(
  () => props.scrollback,
  (value) => {
    if (terminal && typeof value === 'number' && value > 0) terminal.options.scrollback = value
  },
)

const outputPaused = ref(false)
const OUTPUT_PAUSE_LIMIT_BYTES = 4 * 1024 * 1024
let pausedChunks: Uint8Array[] = []
let pausedBytes = 0

function flushPausedOutput(): void {
  const chunks = pausedChunks
  pausedChunks = []
  pausedBytes = 0
  if (!chunks.length) return
  const target = terminal
  if (!target) return
  for (const chunk of chunks) target.write(chunk)
}

function toggleOutputPause(force?: boolean): void {
  const next = force ?? !outputPaused.value
  if (next === outputPaused.value) return
  outputPaused.value = next
  if (!next) flushPausedOutput()
}

function handleOutputPauseShortcut(): void {
  toggleOutputPause()
}
const translateCredentialGateway = createTranslateCredentialGateway()
const translateOpen = ref(false)
const translateSource = ref('')
const translateResult = ref('')
const translateDetectedLanguage = ref('')
const translateError = ref('')
const translating = ref(false)
const selectedTranslateProvider = ref<TranslateProvider>(props.translateProvider)

const configuredTranslateProviders = ref<ReadonlySet<TranslateProvider>>(new Set())
let translateRequest = 0

const TRANSLATE_PROVIDER_ICONS: Record<TranslateProvider, string> = {
  google: googleIcon,
  microsoft: microsoftIcon,
  deepl: deeplIcon,
  baidu: baiduIcon,
  ali: alibabaCloudIcon,
  youdao: youdaoIcon,
  libretranslate: libreTranslateIcon,
}

const translateProviderLabels: Record<TranslateProvider, string> = {
  google: 'Google',
  microsoft: 'Microsoft',
  deepl: 'DeepL',
  baidu: 'Baidu',
  ali: 'Alibaba Cloud',
  youdao: 'Youdao',
  libretranslate: 'LibreTranslate',
}

const availableTranslateProviders = computed<TranslateProvider[]>(() =>
  TRANSLATE_PROVIDER_ORDER.filter((provider) => {
    if (isKeylessTranslateProvider(provider)) return true
    if (requiresTranslateEndpoint(provider)) return props.translateEndpoint.trim().length > 0
    return configuredTranslateProviders.value.has(provider)
  }),
)
const selectedTranslateProviderLabel = computed(() => translateProviderLabels[selectedTranslateProvider.value] ?? '')

function closeTranslation(): void {
  translateRequest += 1
  translateOpen.value = false
}

async function loadConfiguredTranslateProviders(): Promise<void> {
  configuredTranslateProviders.value = new Set()
  try {
    const statuses = await translateCredentialGateway.list()
    configuredTranslateProviders.value = new Set<TranslateProvider>(
      statuses.filter((item) => item.configured).map((item) => item.provider),
    )
  } catch {
    configuredTranslateProviders.value = new Set()
  }
}

async function requestTranslation(): Promise<void> {
  if (availableTranslateProviders.value.length === 0) {
    translating.value = false
    return
  }
  const request = ++translateRequest
  translateResult.value = ''
  translateDetectedLanguage.value = ''
  translateError.value = ''
  translating.value = true
  try {
    const result = await translateGateway.translate(
      translateSource.value,
      props.translateTarget,
      selectedTranslateProvider.value,
      props.translateEndpoint,
    )
    if (request !== translateRequest || !translateOpen.value) return
    translateResult.value = result.text
    translateDetectedLanguage.value = result.detectedLanguage
  } catch (error) {
    if (request !== translateRequest || !translateOpen.value) return
    translateError.value = errorMessage(error, t('shell.translate.failed'))
  } finally {
    if (request === translateRequest) translating.value = false
  }
}

async function translateSelection(preferredProvider: TranslateProvider): Promise<void> {
  const source = currentSelection()
  if (!source.trim()) return
  translateSource.value = source
  translateOpen.value = true
  await loadConfiguredTranslateProviders()

  if (!translateOpen.value) return

  selectedTranslateProvider.value = preferredProvider
  if (!availableTranslateProviders.value.includes(preferredProvider)) {
    translateResult.value = ''
    translateDetectedLanguage.value = ''
    translateError.value = t('shell.translate.noProvider')
    return
  }
  await requestTranslation()
}

async function copyTranslation(): Promise<void> {
  const text = translateResult.value
  if (!text) return
  try {
    await writeClipboardText(text)
    if (props.copyFeedback) toast.success(t('shell.terminal.copySuccess'))
  } catch {
    toast.error(t('shell.terminal.copyFailed'))
  }
}

function saveAsSnippet(): void {
  const text = currentSelection()
  if (!text) return
  emit('saveAsSnippet', text)
}

function openWebSearch(engine: WebSearchEngine): void {
  const query = currentSelection()
  if (!query) return
  void openExternalUrl(webSearchUrl(engine, query))
}

async function pasteClipboard(): Promise<void> {
  if (!canPaste.value || !terminal) return
  const target = terminal
  const token = props.streamToken
  const targetChannel = channel
  clipboardBusy.value = true
  try {
    const text = await readClipboardText()

    if (
      terminal !== target ||
      channel !== targetChannel ||
      props.streamToken !== token ||
      !props.active ||
      !isVisible.value ||
      props.state !== 'connected' ||
      streamStatus.value !== 'ready' ||
      ended.value ||
      zmodemActive.value
    )
      return
    requestPaste(text)
  } catch {
    toast.error(t('shell.terminal.pasteFailed'))
  } finally {
    clipboardBusy.value = false
  }
}

function isModifierKeyHeld(event: MouseEvent): boolean {
  return detectMacPlatform() ? event.metaKey : event.ctrlKey
}

function handleContextMenu(event: MouseEvent): void {
  const behavior = isModifierKeyHeld(event) ? props.modifierRightClickBehavior : props.rightClickBehavior
  if (behavior === 'menu') return
  event.preventDefault()
  event.stopImmediatePropagation()
  if (behavior === 'paste') void pasteClipboard()
}

function searchDecorationOptions(): NonNullable<Parameters<SearchAddon['findNext']>[1]>['decorations'] {
  return {
    matchBackground: '#facc15',
    matchBorder: '#a16207',
    matchOverviewRuler: '#facc15',
    activeMatchBackground: '#fb923c',
    activeMatchBorder: '#9a3412',
    activeMatchColorOverviewRuler: '#fb923c',
  }
}

function searchOptions(): Parameters<SearchAddon['findNext']>[1] {
  return {
    caseSensitive: searchCaseSensitive.value,
    regex: searchRegex.value,
    wholeWord: searchWholeWord.value,
    incremental: true,
    decorations: searchDecorationOptions(),
  }
}

function canStepSearch(direction: -1 | 1): boolean {
  if (searchWrap.value) return true
  if (searchResultCount.value === 0 || searchResultIndex.value < 0) return true
  return direction === 1 ? searchResultIndex.value < searchResultCount.value - 1 : searchResultIndex.value > 0
}

function runSearch(direction: -1 | 1 = 1, term: string = searchQuery.value, restart = false): void {
  const addon = searchAddon
  if (!addon) return
  if (!term) {
    searchResultIndex.value = -1
    searchResultCount.value = 0

    addon.clearDecorations()
    return
  }
  if (restart) {
    searchResultIndex.value = -1
    searchResultCount.value = 0
  } else if (!canStepSearch(direction)) return
  if (direction === 1) addon.findNext(term, searchOptions())
  else addon.findPrevious(term, searchOptions())
}

function handleSearchInput(value: string | number): void {
  const term = String(value ?? '')
  searchQuery.value = term
  runSearch(1, term, true)
}

function focusSearchInput(select: boolean): void {
  const root = (searchInput.value as unknown as { $el?: HTMLElement } | null)?.$el
  const element = root instanceof HTMLInputElement ? root : root?.querySelector('input')
  if (!element) return
  element.focus()
  if (select) element.select()
}

function openSearch(): void {
  searchOpen.value = true
  void nextTick(() => {
    runSearch(1, searchQuery.value, true)

    window.setTimeout(() => focusSearchInput(true), 0)
  })
}

function closeSearch(): void {
  searchOpen.value = false
  searchAddon?.clearDecorations()
  searchResultIndex.value = -1
  searchResultCount.value = 0
  terminal?.focus()
}

function toggleSearchOption(option: 'caseSensitive' | 'regex' | 'wholeWord' | 'wrap'): void {
  if (option === 'caseSensitive') searchCaseSensitive.value = !searchCaseSensitive.value
  else if (option === 'regex') searchRegex.value = !searchRegex.value
  else if (option === 'wholeWord') searchWholeWord.value = !searchWholeWord.value
  else searchWrap.value = !searchWrap.value

  if (option === 'wrap') return
  searchAddon?.clearDecorations()
  searchResultIndex.value = -1
  searchResultCount.value = 0
  runSearch(1, searchQuery.value, true)
}

const searchSummary = computed(() => {
  if (!searchQuery.value) return ''
  if (searchResultCount.value === 0) return t('shell.terminal.searchNoResult')
  return `${Math.max(1, searchResultIndex.value + 1)}/${searchResultCount.value}`
})

function selectAll(): void {
  terminal?.selectAll()
}

function clearTerminal(): void {
  terminal?.clear()

  gutter.notifyCleared()
  commandGutter.notifyCleared()
  lineTimestamps.notifyCleared()
}

function copySelectionFromTerminal(): void {
  const text = terminal?.getSelection() ?? ''
  if (!text) {
    toast.warning(t('shell.terminal.copyNoSelection'))
    return
  }
  selectedText.value = text
  void copySelection()
}

function runTerminalPaneCommand(command: TerminalPaneCommand): void {
  if (command === 'clear') {
    clearTerminal()
    return
  }
  if (command === 'search') {
    if (!sessionReady.value) {
      toast.warning(t('shell.toolRail.findDisabled'))
      return
    }
    openSearch()
    return
  }
  if (command === 'copySelection') {
    copySelectionFromTerminal()
    return
  }
  if (command === 'paste') {
    if (!canPaste.value) {
      toast.warning(t('shell.terminal.pasteUnavailable'))
      return
    }
    void pasteClipboard()
    return
  }

  if (command === 'splitRight') emit('requestSplit', props.sessionId, 'right')
  else if (command === 'splitBottom') emit('requestSplit', props.sessionId, 'bottom')
}

const terminalPaneCommandBus = useTerminalPaneCommandBus()
watch(
  () => terminalPaneCommandBus?.request.value,
  (request) => {
    if (!request || request.sessionId !== props.sessionId || !props.active) return
    runTerminalPaneCommand(request.command)
  },
)

let sessionWarningsShown = false

const sessionWarnings = ref<string[]>([])
async function showSessionWarnings(): Promise<void> {
  if (sessionWarningsShown || !props.sessionId) return
  sessionWarningsShown = true
  const keys = await getSessionWarnings(props.sessionId)
  if (keys.length === 0) return
  sessionWarnings.value = keys.map((key) => translateErrorKey(key, '')).filter((text) => text.length > 0)
}

function writeUnnumberedStatusLine(text: string): void {
  const instance = terminal
  if (!instance) return
  instance.write(text, () => {
    lineTimestamps.markUntimedWrite()
    const stop = instance.registerMarker(0)
    const start = instance.registerMarker(-1)
    if (start && stop) gutter.excludeRange(start, stop)
  })
}

function writeTerminalSeparator(text: string): void {
  writeUnnumberedStatusLine(`\r\n\x1b[90m${text}\x1b[0m\r\n`)
}

function markDisconnected(): void {
  if (!terminal || disconnectMarked || props.reconnecting) return
  disconnectMarked = true
  writeTerminalSeparator(t('terminal.disconnectedSeparator'))
}

function showClosePrompt(): void {
  if (!terminal || !ended.value || closePromptWritten) return

  if (autoReconnectOwnsEnd.value) {
    markReconnecting('terminal.autoReconnectSeparator')
    return
  }
  closePromptWritten = true
  terminal.options.disableStdin = true
  writeUnnumberedStatusLine(
    `\r\n\x1b[90m${t(props.disconnectBehavior === 'reconnect' ? 'terminal.reconnectPrompt' : 'terminal.closePrompt')}\x1b[0m\r\n`,
  )

  fontPresentationReady.value = true
  if (props.active) terminal?.focus()
}

function markReconnecting(localeKey: 'terminal.reconnectingSeparator' | 'terminal.autoReconnectSeparator'): void {
  if (!terminal || reconnectingMarked) return
  reconnectingMarked = true
  writeTerminalSeparator(t(localeKey))
}

function scheduleClosePrompt(immediate = false): void {
  clearClosePromptTimer()
  if (immediate || props.autoReconnect !== true) {
    showClosePrompt()
    return
  }
  closePromptTimer = setTimeout(() => {
    closePromptTimer = null
    showClosePrompt()
  }, CLOSE_PROMPT_AUTO_RECONNECT_GRACE_MS)
}

function handleKeydown(event: KeyboardEvent): void {
  if (menuOpen.value) return

  if (autoReconnectOwnsEnd.value) return
  if (!props.active || !ended.value || event.key !== 'Enter') return
  event.preventDefault()
  event.stopPropagation()
  if (event.repeat || event.isComposing || closingRequested) return
  closingRequested = true
  if (props.disconnectBehavior === 'reconnect') emit('reconnectSession', props.sessionId)
  else emit('closeSession', props.sessionId)
}

const accessibleStatus = computed(() => {
  if (props.autoReconnectPending === true) return t('shell.terminal.autoReconnectPending')
  if (!props.persistent || props.state === 'preview') {
    return props.kind === 'local' ? t('shell.terminal.previewLocal') : t('shell.terminal.previewSsh')
  }
  if (props.kind === 'local') return localAccessibleStatus()
  const labels: Record<Exclude<SessionState, 'preview'>, string> = {
    connecting: t('shell.terminal.connecting'),
    awaitingAuth: t('shell.terminal.awaitingAuth'),
    hostKeyPending: t('shell.terminal.hostKeyPending'),
    authenticating: t('shell.terminal.authenticating'),
    connected:
      streamStatus.value === 'ready'
        ? t('shell.terminal.sshReady')
        : streamStatus.value === 'failed'
          ? t('shell.terminal.channelFailed')
          : t('shell.terminal.opening'),
    closing: t('shell.terminal.closing'),
    closed: t('shell.terminal.closed'),
    failed: t('shell.terminal.failed'),
  }
  return labels[props.state]
})

function localAccessibleStatus(): string {
  if (streamStatus.value === 'ready') return t('shell.terminal.localReady')
  if (streamStatus.value === 'failed') return t('shell.terminal.localChannelFailed')
  if (props.state === 'failed') return t('shell.terminal.localShellFailed')
  if (props.state === 'closing') return t('shell.terminal.localClosing')
  if (props.state === 'closed') return t('shell.terminal.localClosed')
  return t('shell.terminal.localStarting')
}

function terminalTheme(dark: boolean) {
  const selected = getTerminalTheme(
    dark ? (props.darkTheme ?? 'default-dark') : (props.lightTheme ?? 'default-light'),
    dark ? 'default-dark' : 'default-light',
  )
  return { ...selected, background: props.transparentBackground ? '#00000000' : selected.background }
}

function terminalLayoutIsUsable(): boolean {
  const element = terminalElement.value
  if (!element) return false
  let current: HTMLElement | null = element
  while (current) {
    if (window.getComputedStyle(current).display === 'none') return false
    current = current.parentElement
  }
  const rect = element.getBoundingClientRect()

  if (rect.width === 0 && rect.height === 0) return true
  return rect.width >= 32 && rect.height >= 32
}

function applyFit(): void {
  if (!isVisible.value || !terminal || !fitAddon || !terminalLayoutIsUsable()) return
  fitAddon.fit()

  ensureWebglBackingFresh()

  connectIfReady()
  notifyResize()
}

const RESIZE_FREEZE_IDLE_MS = 5
let resizeFreezeTimer: ReturnType<typeof setTimeout> | null = null

let resizeObservationSeen = false
let resizeFrozen = false

function freezeTerminalSize(): boolean {
  const element = terminalElement.value
  if (!element) return false
  if (!resizeFrozen) {
    const rect = element.getBoundingClientRect()

    if (rect.width <= 0 || rect.height <= 0) return false
    resizeFrozen = true
    element.style.width = `${rect.width}px`
    element.style.height = `${rect.height}px`
    element.style.flex = 'none'
  }
  if (resizeFreezeTimer !== null) clearTimeout(resizeFreezeTimer)
  resizeFreezeTimer = setTimeout(() => {
    resizeFreezeTimer = null
    releaseTerminalSizeFreeze()

    terminalElement.value?.getBoundingClientRect()
    applyFit()
  }, RESIZE_FREEZE_IDLE_MS)
  return true
}

function releaseTerminalSizeFreeze(): void {
  const element = terminalElement.value
  resizeFrozen = false
  if (!element) return
  element.style.width = ''
  element.style.height = ''
  element.style.flex = ''
}

function queueFit(): void {
  if (!isVisible.value || !terminal || !fitAddon) return

  if (
    document.body.classList.contains('qterm-resizing-vertical') ||
    document.body.classList.contains('qterm-resizing-horizontal')
  ) {
    fitAfterLayoutResize = true
    return
  }
  releaseTerminalSizeFreeze()
  applyFit()
}

function handleResizeObservation(): void {
  if (!isVisible.value || !terminal || !fitAddon) return
  if (!resizeObservationSeen) {
    resizeObservationSeen = true
    return
  }
  if (
    document.body.classList.contains('qterm-resizing-vertical') ||
    document.body.classList.contains('qterm-resizing-horizontal')
  ) {
    fitAfterLayoutResize = true
    return
  }

  if (visibleLayoutRecoveryPending) {
    layoutChangedDuringVisibleRecovery = true
    return
  }

  if (!visibleTerminalDimensionsMismatch()) return
  if (holdsWebglLease.value && freezeTerminalSize()) return
  releaseTerminalSizeFreeze()
  applyFit()
}

const canUseWebgl = computed(() => props.renderEngine === 'webgl' && !props.transparentBackground)
const holdsWebglLease = computed(() => canUseWebgl.value && webglActivationReady.value)

function webglContextState(): { visible: boolean; active: boolean } {
  return { visible: isVisible.value, active: isVisible.value && props.active }
}

function applyRenderEngine(): void {
  if (!terminal) return
  disposeWebglAddon()

  webglAddonDpr = window.devicePixelRatio
  if (!holdsWebglLease.value || !isVisible.value) {
    if (terminal) installWkWebViewDomRendererMetrics(terminal)
    return
  }

  let addon: WebglAddon | null = null
  try {
    addon = new WebglAddon()
    webglAddon = addon
    addon.onContextLoss(() => {
      disposeWebglAddon(addon)
      if (!isVisible.value || !holdsWebglLease.value) return

      visibleLayoutRecoveryPending = true
      refreshVisibleTerminal()
    })
    terminal.loadAddon(addon)
    retainWebglContext(webglContextOwner, () => disposeWebglAddon(addon), webglContextState())
  } catch {
    disposeWebglAddon(addon)
  }
}

function syncWebglLease(): void {
  if (!terminal) return
  if (webglAddon) {
    if (!canUseWebgl.value) {
      disposeWebglAddon()
      return
    }
    updateWebglContext(webglContextOwner, webglContextState())
    return
  }
  if (holdsWebglLease.value && isVisible.value) {
    applyRenderEngine()
  }
}

function disposeWebglAddon(addon: WebglAddon | null = webglAddon): void {
  if (!addon) return
  if (webglAddon === addon) {
    webglAddon = null
    forgetWebglContext(webglContextOwner)
  }
  try {
    addon.dispose()
  } catch {}
}

function ensureWebglBackingFresh(): void {
  if (!webglAddon) return
  if (window.devicePixelRatio === webglAddonDpr) return
  applyRenderEngine()
}

function repaintTerminal(): void {
  if (!terminal || terminal.rows <= 0) return
  terminal.refresh(0, terminal.rows - 1)
}

let dprQuery: MediaQueryList | null = null

function handleDevicePixelRatioChange(): void {
  ensureWebglBackingFresh()
  repaintTerminal()
  queueFit()

  watchDevicePixelRatio()
}

function watchDevicePixelRatio(): void {
  if (typeof window.matchMedia !== 'function') return
  dprQuery?.removeEventListener('change', handleDevicePixelRatioChange)
  dprQuery = window.matchMedia(`(resolution: ${window.devicePixelRatio}dppx)`)
  dprQuery.addEventListener('change', handleDevicePixelRatioChange)
}

function handleLayoutResizeEnd(): void {
  if (!fitAfterLayoutResize) return
  fitAfterLayoutResize = false
  queueFit()
}

function notifyResize(): void {
  if (!terminal || !channel || streamStatus.value !== 'ready') return
  const columns = terminal.cols
  const rows = terminal.rows
  if (columns === lastColumns && rows === lastRows) return

  if (resizeNotifyTimer !== null) window.clearTimeout(resizeNotifyTimer)
  resizeNotifyTimer = window.setTimeout(() => {
    resizeNotifyTimer = null
    if (!terminal || !channel || streamStatus.value !== 'ready') return
    const finalColumns = terminal.cols
    const finalRows = terminal.rows
    if (finalColumns === lastColumns && finalRows === lastRows) return
    lastColumns = finalColumns
    lastRows = finalRows
    channel.resize(finalColumns, finalRows)
  }, 100)
}

function zmodemSender(data: Uint8Array): Promise<void> {
  return channel?.sendBinary(data) ?? Promise.reject(new Error(t('shell.terminal.channelFailed')))
}

function zmodemToTerminal(data: Uint8Array): void {
  credentialOutputTail = (credentialOutputTail + credentialOutputDecoder.decode(data, { stream: true })).slice(
    -CREDENTIAL_OUTPUT_TAIL_LIMIT,
  )
  queueCredentialPromptCheck()
  const instance = terminal
  if (!instance) return

  if (outputPaused.value) {
    if (pausedBytes + data.length > OUTPUT_PAUSE_LIMIT_BYTES) {
      outputPaused.value = false
      flushPausedOutput()
      toast.warning(t('shell.terminal.outputOverflow'))
    } else {
      pausedChunks.push(data.slice())
      pausedBytes += data.length
      return
    }
  }

  const receivedAt = Date.now()
  instance.write(data, () => {
    if (terminal === instance) lineTimestamps.markParsedOutput(receivedAt)
  })
}

function onZmodemDetect(direction: ZmodemDirection): void {
  zmodemActive.value = true
  window.dispatchEvent(
    new CustomEvent('qterm:zmodem-status', {
      detail: { sessionId: props.sessionId, direction, state: 'detected' },
    }),
  )
}

function onZmodemProgress(progress: ZmodemProgress): void {
  window.dispatchEvent(
    new CustomEvent('qterm:zmodem-status', {
      detail: {
        sessionId: props.sessionId,
        direction: progress.direction,
        state: progress.state,
        fileName: progress.fileName,
        progress: progress.percent,
      },
    }),
  )
}

function onZmodemComplete(direction: ZmodemDirection, fileName: string): void {
  zmodemActive.value = false
  window.dispatchEvent(
    new CustomEvent('qterm:zmodem-status', {
      detail: { sessionId: props.sessionId, direction, state: 'complete', fileName },
    }),
  )
}

function onZmodemError(direction: ZmodemDirection, message: string): void {
  zmodemActive.value = false
  window.dispatchEvent(
    new CustomEvent('qterm:zmodem-status', {
      detail: { sessionId: props.sessionId, direction, state: 'error', message },
    }),
  )
}

function onZmodemRetract(): void {
  zmodemActive.value = false
  window.dispatchEvent(
    new CustomEvent('qterm:zmodem-status', {
      detail: { sessionId: props.sessionId, direction: zmodemManager?.getDirection() ?? 'upload', state: 'retracted' },
    }),
  )
}

async function onZmodemOffer(fileInfo: { name: string; size: number }): Promise<boolean> {
  window.dispatchEvent(
    new CustomEvent('qterm:zmodem-download-offer', {
      detail: { sessionId: props.sessionId, ...fileInfo },
    }),
  )
  return true
}

function connectIfReady(): void {
  if (
    !props.persistent ||
    props.state !== 'connected' ||
    !props.streamToken ||
    connectedToken === props.streamToken ||
    !terminal ||
    !initialLayoutReady ||
    !isVisible.value
  ) {
    return
  }
  zmodemManager?.reset()

  if (connectedSessionID && connectedSessionID !== props.sessionId) {
    if (channel) unregisterTerminalChannel(connectedSessionID, channel)
    forgetTerminalBridge(connectedSessionID)
    clearSessionDirectoryState(connectedSessionID)
  }
  channel?.close()
  clearCredentialInputState()

  closePromptWritten = false
  closingRequested = false
  reconnectingMarked = false

  fitAddon?.fit()
  connectedToken = props.streamToken
  connectedSessionID = props.sessionId
  registerTerminalBridge(connectedSessionID)

  const hideUntilFontReady = !paintPresentedOnce

  const generation = ++channelGeneration

  firstOutputFontCorrectionPending = initialFontPrepared && !webglActivationReady.value
  firstOutputFontActivation = null

  if (hideUntilFontReady) fontPresentationReady.value = false
  streamStatus.value = 'opening'
  const columns = terminal.cols || 80
  const rows = terminal.rows || 24
  const boundSessionID = props.sessionId
  lastColumns = columns
  lastRows = rows

  zmodemManager = new ZmodemManager(zmodemSender, zmodemToTerminal, {
    onUpload: (signal) => selectZmodemUpload(signal, { sessionId: boundSessionID, directory: currentDirectory.value }),
    onDestination: selectZmodemDestination,
    onDrainFailure: () => channel?.close(),
    onDetect: onZmodemDetect,
    onProgress: onZmodemProgress,
    onComplete: onZmodemComplete,
    onError: onZmodemError,
    onRetract: onZmodemRetract,
    onOffer: onZmodemOffer,
  })

  channel = connectTerminalChannel(boundSessionID, props.streamToken, columns, rows, {
    onReady() {
      if (generation !== channelGeneration) return
      if (!terminal) return
      terminal.options.disableStdin = false
      streamStatus.value = 'ready'
      scheduleTerminalFocus()

      disconnectMarked = false

      void refreshRecordingState(props.sessionId)
      queueFit()
      emit('streamStatus', { sessionId: boundSessionID, state: 'ready' })

      armPresentationWatchdog()
    },
    onOutput(data, acknowledge) {
      if (!focusOnFirstOutputDone) {
        focusOnFirstOutputDone = true
        scheduleTerminalFocus()
      }
      const finalizeFirstOutputFont = firstOutputFontCorrectionPending
      if (finalizeFirstOutputFont) {
        firstOutputFontCorrectionPending = false
        fontPresentationReady.value = false

        armPresentationWatchdog()
      }

      if (zmodemManager) {
        zmodemManager.consume(data)
      } else {
        zmodemToTerminal(data)
      }

      if (terminal) {
        terminal.write('', () => {
          captureShellTabCommand(pendingShellTabSubmission?.typed)
          if (pendingShellTabSubmission && shellTabCompletedCommand) flushPendingShellTabSubmission()
          if (finalizeFirstOutputFont) {
            const instance = terminal
            const activation = (async () => {
              repaintTerminal()
              await waitForTerminalRender()
              await waitForRenderFrame()
              if (!instance || terminal !== instance) return

              if (!hasOtherWebglOwner(webglContextOwner)) {
                await primeTerminalFontForAtlas(props.fontSize, props.fontFamily.trim())
                if (!instance || terminal !== instance) return
              }
              webglActivationReady.value = true
              applyRenderEngine()
              await waitForRenderFrame()
              if (terminal !== instance) return
              repaintTerminal()
              await waitForTerminalRender()
              await waitForRenderFrame()
              if (terminal !== instance) return

              if (!hasOtherWebglOwner(webglContextOwner)) {
                webglAddon?.clearTextureAtlas()
                repaintTerminal()
                await waitForTerminalRender()
                await waitForRenderFrame()
                if (terminal !== instance) return
              }
              exposeFontDiagnostics(instance, 'webgl-first-output', {
                fontLoadDiagnostics,
                fontCanvasDiagnostics,
                frameSnapshotDiagnostics,
              })
            })()
            firstOutputFontActivation = activation
            void activation.finally(() => {
              if (firstOutputFontActivation === activation) firstOutputFontActivation = null
              fontPresentationReady.value = true
              acknowledge()
            })
            return
          }
          if (firstOutputFontActivation) {
            void firstOutputFontActivation.finally(acknowledge)
            return
          }
          fontPresentationReady.value = true
          acknowledge()
        })
      } else acknowledge()
    },
    onError(message) {
      if (generation !== channelGeneration) return
      pendingShellTabSubmission = null
      clearShellTabSubmitTimer()
      clearShellTabState()
      clearCredentialInputState()
      zmodemManager?.reset()
      if (terminal) {
        terminal.options.disableStdin = true

        writeUnnumberedStatusLine(`\r\n\x1b[31m${message}\x1b[0m\r\n`)
      }
      streamStatus.value = 'failed'
      void refreshRecordingState(props.sessionId)

      revealTerminalPresentation()
      emit('streamStatus', { sessionId: boundSessionID, state: 'failed', message })
    },
    onClose() {
      if (generation !== channelGeneration) return
      pendingShellTabSubmission = null
      clearShellTabSubmitTimer()
      clearShellTabState()
      clearCredentialInputState()
      zmodemManager?.reset()
      if (terminal) terminal.options.disableStdin = true
      streamStatus.value = 'closed'
      revealTerminalPresentation()
      void refreshRecordingState(props.sessionId)
      emit('streamStatus', { sessionId: boundSessionID, state: 'closed' })
    },
  })
  registerTerminalChannel(
    boundSessionID,
    channel,
    () => props.state === 'connected' && streamStatus.value === 'ready' && !zmodemActive.value,
  )
}

function handleWindowResize(): void {
  if (!initialLayoutReady || !isVisible.value || !terminal) return

  visibleLayoutRecoveryPending = true
  captureTerminalFrameSnapshot()
  preserveVisibleFrameDuringRecovery = showTerminalFrameSnapshot()
  layoutPresentationReady.value = preserveVisibleFrameDuringRecovery

  if (windowResizeRecoveryTimer !== null) clearTimeout(windowResizeRecoveryTimer)
  windowResizeRecoveryTimer = setTimeout(() => {
    windowResizeRecoveryTimer = null
    refreshVisibleTerminal()
  }, RESIZE_FREEZE_IDLE_MS)
}

let fontRefreshRevision = 0

const FONT_PRESENTATION_SETTLE_MS = 120
let fontLoadDiagnostics = 'load=not-run'
let fontCanvasDiagnostics = 'canvas=not-run'
let frameSnapshotDiagnostics = 'frame=not-run'

function waitForRenderFrame(): Promise<void> {
  return new Promise((resolve) => window.requestAnimationFrame(() => resolve()))
}

function warmTerminalFontCanvas(size: number, family: string): void {
  const canvas = document.createElement('canvas')
  const dpr = window.devicePixelRatio || 1
  canvas.width = Math.ceil(320 * dpr)
  canvas.height = Math.ceil(48 * dpr)
  const context = canvas.getContext('2d', { willReadFrequently: true })
  if (!context) {
    fontCanvasDiagnostics = 'canvas=missing'
    return
  }

  const host = terminalElement.value ?? document.body
  Object.assign(canvas.style, { position: 'absolute', visibility: 'hidden', pointerEvents: 'none' })
  host.appendChild(canvas)
  const sample = 'MW09Il1O0[]{}@#root'
  try {
    context.clearRect(0, 0, canvas.width, canvas.height)
    context.font = `normal normal ${size * dpr}px ${buildTerminalFontFamily(family)}`
    context.textBaseline = 'alphabetic'
    context.fillStyle = '#fff'
    context.fillText(sample, 4 * dpr, 28 * dpr)

    void canvas.offsetWidth
  } catch (cause) {
    console.warn('[qterm] 终端字体预热失败，已跳过', cause)
  } finally {
    canvas.remove()
  }

  let painted = 0
  if (import.meta.env.DEV && import.meta.env.MODE !== 'test' && typeof context.getImageData === 'function') {
    const pixels = context.getImageData(0, 0, canvas.width, canvas.height).data
    for (let index = 3; index < pixels.length; index += 4) {
      if (pixels[index] !== 0) painted += 1
    }
  }
  fontCanvasDiagnostics =
    typeof context.measureText === 'function'
      ? `canvasWidth=${context.measureText(sample).width.toFixed(3)},canvasPixels=${painted}`
      : 'canvas=partial'
}

async function waitForFontPresentationSettle(size: number, family: string): Promise<void> {
  const host = terminalElement.value ?? document.body
  const probe = document.createElement('span')
  probe.textContent = 'MW09'.repeat(8)
  probe.setAttribute('aria-hidden', 'true')
  Object.assign(probe.style, {
    position: 'absolute',
    visibility: 'hidden',
    pointerEvents: 'none',
    whiteSpace: 'pre',
    fontKerning: 'none',
    fontFamily: buildTerminalFontFamily(family),
    fontSize: `${size}px`,
  })
  host.appendChild(probe)
  try {
    void probe.offsetWidth
    await waitForRenderFrame()
    await new Promise<void>((resolve) => window.setTimeout(resolve, FONT_PRESENTATION_SETTLE_MS))
    void probe.offsetWidth
    warmTerminalFontCanvas(size, family)
    await waitForRenderFrame()

    warmTerminalFontCanvas(size, family)
  } finally {
    probe.remove()
  }
}

async function primeTerminalFontForAtlas(size = props.fontSize, family = props.fontFamily.trim()): Promise<void> {
  if (isTerminalPaneUnmounted) return
  const host = terminalElement.value ?? document.body
  const probe = document.createElement('span')
  probe.className = 'qterm-font-prime-probe'
  probe.textContent = 'MW09'.repeat(8)
  probe.setAttribute('aria-hidden', 'true')
  Object.assign(probe.style, {
    position: 'absolute',
    visibility: 'hidden',
    pointerEvents: 'none',
    whiteSpace: 'pre',
    fontKerning: 'none',
    fontFamily: buildTerminalFontFamily(family),
    fontSize: `${size}px`,
  })
  host.appendChild(probe)
  const canvas = document.createElement('canvas')
  let attachedCanvas: HTMLCanvasElement | null = null
  try {
    void probe.offsetWidth
    const context = canvas.getContext('2d')
    if (context) {
      Object.assign(canvas.style, { position: 'absolute', visibility: 'hidden', pointerEvents: 'none' })
      host.appendChild(canvas)
      attachedCanvas = canvas
      const dpr = window.devicePixelRatio || 1

      if (typeof context.fillText === 'function') {
        context.font = `normal normal ${size * dpr}px ${buildTerminalFontFamily(family)}`
        context.textBaseline = 'alphabetic'
        context.fillStyle = '#fff'
        context.fillText('MW09Il1O0[]{}@#root', 4 * dpr, 28 * dpr)
        void canvas.offsetWidth
      }
    }

    await waitForRenderFrame()
  } finally {
    attachedCanvas?.remove()
    probe.remove()
  }

  terminalElement.value?.setAttribute('data-font-primed', `${size}px`)
}

function waitForTerminalRender(): Promise<void> {
  if (!terminal || typeof terminal.onRender !== 'function' || isTerminalPaneUnmounted) {
    return waitForRenderFrame().then(() => waitForRenderFrame())
  }
  return new Promise<void>((resolve) => {
    const instance = terminal
    let resolved = false
    const disposable = instance!.onRender(() => {
      if (resolved) return
      resolved = true
      disposable.dispose()
      resolve()
    })

    waitForRenderFrame()
      .then(() => waitForRenderFrame())
      .then(() => {
        if (!resolved) {
          disposable.dispose()
          resolve()
        }
      })
  })
}

async function applyMeasuredFont(
  instance: Terminal,
  revision: number,
  size: number,
  selectedFamily: string,
  requestManualRepaint = true,
  requestFit = true,
): Promise<void> {
  if (selectedFamily && !isTerminalFontAvailable(selectedFamily)) {
    console.warn(`[TerminalPane] font "${selectedFamily}" was not detected on this machine; applying it anyway`)
  }
  const family = buildTerminalFontFamily(selectedFamily)
  try {
    if (instance.options.fontFamily === family) instance.options.fontFamily = 'monospace'
    instance.options.fontFamily = family
    instance.options.fontSize = size

    instance.options.lineHeight = props.lineHeight

    if (import.meta.env.DEV && import.meta.env.MODE !== 'test') {
      console.info('[TerminalPane] font applied', {
        sessionId: props.sessionId,
        requested: selectedFamily,
        effectiveStack: family,
        size,
        lineHeight: instance.options.lineHeight,
        revision,
      })
    }

    await primeTerminalFontForAtlas(size, selectedFamily)
    if (terminal !== instance) return

    installWkWebViewDomRendererMetrics(instance)
    if (requestFit) queueFit()

    if (requestManualRepaint) repaintTerminal()
  } catch {}
}

async function applyFontWhenReady(requestManualRepaint = true, requestFit = true): Promise<void> {
  const revision = ++fontRefreshRevision
  const size = props.fontSize
  const selectedFamily = props.fontFamily.trim()
  await ensureTerminalFontReady(size, selectedFamily)

  if (revision !== fontRefreshRevision || props.fontSize !== size || props.fontFamily.trim() !== selectedFamily) return
  const instance = terminal
  if (!instance) return
  await applyMeasuredFont(instance, revision, size, selectedFamily, requestManualRepaint, requestFit)
}

async function prepareInitialTerminalFont(): Promise<void> {
  while (terminal && !initialLayoutReady) {
    const size = props.fontSize
    const family = props.fontFamily.trim()
    const startedAt = performance.now()
    await ensureTerminalFontReady(size, family)
    const descriptor = `${size}px ${buildTerminalFontFamily(family)}`
    const checked =
      document.fonts && typeof document.fonts.check === 'function' ? document.fonts.check(descriptor, 'MW09') : false
    fontLoadDiagnostics = `loadMs=${(performance.now() - startedAt).toFixed(1)},fontCheck=${checked},fontsStatus=${document.fonts?.status ?? 'missing'}`
    if (props.fontSize !== size || props.fontFamily.trim() !== family) continue
    const instance = terminal
    const revision = ++fontRefreshRevision
    await applyMeasuredFont(instance, revision, size, family, false, false)

    await waitForFontPresentationSettle(size, family)
    await nextTick()
    if (props.fontSize === size && props.fontFamily.trim() === family) return
  }
}

let visibleRefreshFrame: number | null = null
let visibleLayoutRecoveryPending = false
let visibleLayoutRecoveryRevision = 0
let layoutChangedDuringVisibleRecovery = false
let pendingWebglLayoutRevision: number | null = null
let snapshotReadyFrame: number | null = null
let snapshotReadyCommitFrame: number | null = null

function notifySnapshotPresentationReady(): void {
  if (snapshotReadyFrame !== null) window.cancelAnimationFrame(snapshotReadyFrame)
  if (snapshotReadyCommitFrame !== null) window.cancelAnimationFrame(snapshotReadyCommitFrame)
  snapshotReadyFrame = window.requestAnimationFrame(() => {
    snapshotReadyFrame = null
    snapshotReadyCommitFrame = window.requestAnimationFrame(() => {
      snapshotReadyCommitFrame = null
      if (!isVisible.value || !terminalElement.value?.classList.contains('has-frame-snapshot')) return
      emit('presentationReady', props.sessionId)
    })
  })
}

function cancelSnapshotPresentationReady(): void {
  if (snapshotReadyFrame !== null) window.cancelAnimationFrame(snapshotReadyFrame)
  if (snapshotReadyCommitFrame !== null) window.cancelAnimationFrame(snapshotReadyCommitFrame)
  snapshotReadyFrame = null
  snapshotReadyCommitFrame = null
}

function visibleTerminalDimensionsMismatch(): boolean {
  if (!terminal || !fitAddon) return false
  const proposed = fitAddon.proposeDimensions()
  return Boolean(proposed && (proposed.cols !== terminal.cols || proposed.rows !== terminal.rows))
}

function webglCanvasBackingIsInvalid(): boolean {
  if (!webglAddon || !terminal?.element) return false
  const screen = terminal.element.querySelector('.xterm-screen')
  if (!screen) return false
  const canvases = Array.from(screen.querySelectorAll<HTMLCanvasElement>(':scope > canvas'))
  const dpr = window.devicePixelRatio || 1
  return canvases.some((canvas) => {
    const cssWidth = Number.parseFloat(canvas.style.width)
    const cssHeight = Number.parseFloat(canvas.style.height)
    if (canvas.width <= 0 || canvas.height <= 0 || cssWidth <= 0 || cssHeight <= 0) return true
    return (
      Math.abs(canvas.width - Math.round(cssWidth * dpr)) > 1 ||
      Math.abs(canvas.height - Math.round(cssHeight * dpr)) > 1
    )
  })
}

function captureTerminalFrameSnapshot(): boolean {
  const host = terminalElement.value
  const screen = host?.querySelector<HTMLElement>('.xterm-screen')
  if (!host || !screen || host.classList.contains('has-frame-snapshot')) {
    frameSnapshotAvailable.value = terminalFrameSnapshot !== null
    frameSnapshotDiagnostics = terminalFrameSnapshot ? 'frame=reused' : 'frame=missing-screen'
    return terminalFrameSnapshot !== null
  }
  const layers = Array.from(screen.querySelectorAll<HTMLCanvasElement>(':scope > canvas')).filter(
    (canvas) => canvas.width > 0 && canvas.height > 0,
  )
  if (layers.length === 0) {
    frameSnapshotDiagnostics = 'frame=missing-layers'
    return false
  }

  const width = Math.max(...layers.map((canvas) => canvas.width))
  const height = Math.max(...layers.map((canvas) => canvas.height))
  const screenRect = screen.getBoundingClientRect()
  const dpr = window.devicePixelRatio || 1
  const cssWidth = screenRect.width > 0 ? screenRect.width : width / dpr
  const cssHeight = screenRect.height > 0 ? screenRect.height : height / dpr
  const snapshot = document.createElement('canvas')
  snapshot.width = width
  snapshot.height = height
  const context = snapshot.getContext('2d')
  if (!context) {
    frameSnapshotDiagnostics = 'frame=missing-context'
    return false
  }

  let capturedLayers = 0
  for (const layer of layers) {
    try {
      context.drawImage(layer, 0, 0, width, height)
      capturedLayers += 1
    } catch {}
  }
  if (capturedLayers === 0) {
    frameSnapshotDiagnostics = 'frame=capture-failed'
    return false
  }
  terminalFrameSnapshot?.presentation?.remove()
  terminalFrameSnapshot = {
    source: snapshot,
    cssWidth,
    cssHeight,
    presentation: null,
  }
  frameSnapshotAvailable.value = true
  frameSnapshotDiagnostics = `frame=${capturedLayers}/${layers.length},${width}x${height},css=${cssWidth.toFixed(1)}x${cssHeight.toFixed(1)}`
  return true
}

function showTerminalFrameSnapshot(): boolean {
  const host = terminalElement.value
  if (!host || !terminalFrameSnapshot) return false
  const screen = host.querySelector<HTMLElement>('.xterm-screen')
  const hostRect = host.getBoundingClientRect()
  const screenRect = screen?.getBoundingClientRect()
  const currentScreenIsMeasurable = Boolean(screenRect && screenRect.width > 0 && screenRect.height > 0)
  const cssWidth = currentScreenIsMeasurable
    ? screenRect!.width
    : hostRect.width > 0
      ? hostRect.width
      : terminalFrameSnapshot.cssWidth
  const cssHeight = currentScreenIsMeasurable
    ? screenRect!.height
    : hostRect.height > 0
      ? hostRect.height
      : terminalFrameSnapshot.cssHeight
  const left = currentScreenIsMeasurable ? screenRect!.left - hostRect.left : 0
  const top = currentScreenIsMeasurable ? screenRect!.top - hostRect.top : 0
  if (cssWidth <= 0 || cssHeight <= 0) return false

  const dpr = window.devicePixelRatio || 1
  const presentation = document.createElement('canvas')
  presentation.width = Math.max(1, Math.round(cssWidth * dpr))
  presentation.height = Math.max(1, Math.round(cssHeight * dpr))
  presentation.className = 'qterm-terminal-frame-snapshot absolute z-1 block pointer-events-none'
  presentation.setAttribute('aria-hidden', 'true')
  presentation.style.left = `${left}px`
  presentation.style.top = `${top}px`
  presentation.style.width = `${cssWidth}px`
  presentation.style.height = `${cssHeight}px`
  const context = presentation.getContext('2d')
  if (!context) return false
  try {
    const sourceDisplayWidth = Math.round(terminalFrameSnapshot.cssWidth * dpr)
    const sourceDisplayHeight = Math.round(terminalFrameSnapshot.cssHeight * dpr)
    context.drawImage(
      terminalFrameSnapshot.source,
      0,
      0,
      terminalFrameSnapshot.source.width,
      terminalFrameSnapshot.source.height,
      0,
      0,
      sourceDisplayWidth,
      sourceDisplayHeight,
    )
  } catch {
    return false
  }

  terminalFrameSnapshot.presentation?.remove()
  terminalFrameSnapshot.presentation = presentation
  host.appendChild(presentation)
  host.classList.add('has-frame-snapshot')
  return true
}

function unmountTerminalFrameSnapshot(): void {
  terminalElement.value?.classList.remove('has-frame-snapshot')
  terminalFrameSnapshot?.presentation?.remove()
  if (terminalFrameSnapshot) terminalFrameSnapshot.presentation = null
}

async function finishVisibleTerminalRefresh(attempt: number): Promise<void> {
  if (!isVisible.value || !terminal) return
  if (!terminalLayoutIsUsable()) {
    if (attempt < 4) refreshVisibleTerminal(attempt + 1)
    else {
      visibleLayoutRecoveryPending = false
      preserveVisibleFrameDuringRecovery = false
    }
    return
  }

  const structuralLayoutRevision = pendingWebglLayoutRevision
  const structuralLayoutChanged = structuralLayoutRevision !== null
  const rendererNeedsRestore = holdsWebglLease.value && !webglAddon
  const dimensionsMismatch = visibleTerminalDimensionsMismatch()
  const canvasInvalid = webglCanvasBackingIsInvalid()
  if (
    !layoutPresentationReady.value ||
    preserveVisibleFrameDuringRecovery ||
    structuralLayoutChanged ||
    rendererNeedsRestore ||
    dimensionsMismatch ||
    canvasInvalid
  ) {
    await recoverVisibleTerminalLayout(structuralLayoutChanged, canvasInvalid)
  } else {
    syncWebglLease()
    ensureWebglBackingFresh()
  }
  preserveVisibleFrameDuringRecovery = false
  if (pendingWebglLayoutRevision === structuralLayoutRevision) pendingWebglLayoutRevision = null

  visibleLayoutRecoveryPending = pendingWebglLayoutRevision !== null
  if (props.active) terminal?.focus()
  presentationPending.value = false
  emit('presentationReady', props.sessionId)
}

async function recoverVisibleTerminalLayout(
  structuralLayoutChanged = false,
  rebuildInvalidRenderer = false,
): Promise<void> {
  const revision = ++visibleLayoutRecoveryRevision

  if (
    (structuralLayoutChanged || visibleTerminalDimensionsMismatch() || rebuildInvalidRenderer) &&
    webglAddon &&
    !preserveVisibleFrameDuringRecovery
  ) {
    captureTerminalFrameSnapshot()
    preserveVisibleFrameDuringRecovery = showTerminalFrameSnapshot()
  }
  layoutPresentationReady.value = true
  await nextTick()
  try {
    if (revision !== visibleLayoutRecoveryRevision || !isVisible.value || !terminal) return

    if (rebuildInvalidRenderer && webglAddon) disposeWebglAddon()

    if (holdsWebglLease.value && !webglAddon && !hasOtherWebglOwner(webglContextOwner)) {
      await primeTerminalFontForAtlas()
      if (revision !== visibleLayoutRecoveryRevision || !isVisible.value || !terminal) return
    }
    syncWebglLease()
    ensureWebglBackingFresh()

    for (let pass = 0; pass < 3; pass += 1) {
      layoutChangedDuringVisibleRecovery = false
      const dimensionsChanged = visibleTerminalDimensionsMismatch()
      if (dimensionsChanged) {
        applyFit()
      } else {
        repaintTerminal()
      }

      await waitForTerminalRender()

      await waitForRenderFrame()
      if (!layoutChangedDuringVisibleRecovery && !visibleTerminalDimensionsMismatch()) break
    }

    await waitForRenderFrame()
    if (revision !== visibleLayoutRecoveryRevision || !isVisible.value || !terminal) return
    exposeFontDiagnostics(terminal, structuralLayoutChanged ? 'split-layout' : 'visible-layout', {
      fontLoadDiagnostics,
      fontCanvasDiagnostics,
      frameSnapshotDiagnostics,
    })
  } finally {
    if (revision === visibleLayoutRecoveryRevision) {
      layoutPresentationReady.value = true

      await nextTick()
      if (revision === visibleLayoutRecoveryRevision && isVisible.value) unmountTerminalFrameSnapshot()
    }
  }
}

function refreshVisibleTerminal(attempt = 0): void {
  if (visibleRefreshFrame !== null) window.cancelAnimationFrame(visibleRefreshFrame)
  visibleRefreshFrame = window.requestAnimationFrame(() => {
    visibleRefreshFrame = null
    void finishVisibleTerminalRefresh(attempt)
  })
}

function refreshAfterWindowResume(): void {
  if (!initialLayoutReady || !isVisible.value || !terminal) return
  visibleLayoutRecoveryPending = true
  captureTerminalFrameSnapshot()
  preserveVisibleFrameDuringRecovery = showTerminalFrameSnapshot()

  layoutPresentationReady.value = true
  syncWebglLease()
  refreshVisibleTerminal()
}

function handleVisibilityResume(): void {
  if (document.visibilityState === 'hidden') return
  refreshAfterWindowResume()
}

onMounted(async () => {
  window.addEventListener('qterm:recordings-changed', recordingChangedHandler)
  void refreshRecordingState(props.sessionId)

  webglActivationReady.value = false
  const element = terminalElement.value
  if (!element) return

  terminal = new Terminal({
    allowProposedApi: true,

    allowTransparency: props.transparentBackground,
    cursorBlink: props.cursorBlink,
    cursorStyle: props.cursorStyle,
    wordSeparator: props.wordSeparators,

    macOptionIsMeta: props.macOptionIsMeta,
    disableStdin: true,
    fontFamily: buildTerminalFontFamily(props.fontFamily),
    fontSize: props.fontSize,
    lineHeight: props.lineHeight,

    reflowCursorLine: false,

    rightClickSelectsWord: false,
    scrollback: props.scrollback,
    theme: terminalTheme(props.dark),
  })

  try {
    terminal.registerLinkProvider({
      provideLinks(bufferLineNumber, callback) {
        try {
          const line = terminal?.buffer.active.getLine(bufferLineNumber - 1)?.translateToString(true) ?? ''
          const links: ILink[] = []
          for (const match of line.matchAll(/https?:\/\/[^\s"'<>()[\]{}]+/g)) {
            const start = (match.index ?? 0) + 1
            const text = match[0].replace(/[.,;:!?]+$/, '')
            links.push({
              range: {
                start: { x: start, y: bufferLineNumber },
                end: { x: start + text.length - 1, y: bufferLineNumber },
              },
              text,
              activate: () => void openExternalUrl(text),
              decorations: { underline: true, pointerCursor: true },
            })
          }
          callback(links.length ? links : undefined)
        } catch {
          callback(undefined)
        }
      },
    })
  } catch (cause) {
    console.warn('[qterm] 终端链接注册失败，已跳过', cause)
  }
  fitAddon = new FitAddon()

  bindTerminalOutputCapture(props.sessionId)
  registerTerminalBridge(props.sessionId)
  terminal.loadAddon(fitAddon)
  searchAddon = new SearchAddon()
  terminal.loadAddon(searchAddon)
  searchResultSubscription = searchAddon.onDidChangeResults(({ resultIndex, resultCount }) => {
    searchResultIndex.value = resultIndex
    searchResultCount.value = resultCount
  })
  openTerminalWithDomFontMetrics(terminal, element, {
    fontFamily: buildTerminalFontFamily(props.fontFamily),
    fontSize: props.fontSize,
  })
  exposeFontDiagnostics(terminal, 'open', {
    fontLoadDiagnostics,
    fontCanvasDiagnostics,
    frameSnapshotDiagnostics,
  })
  keywordHighlights.subscribe(terminal)
  gutter.attach(terminal)
  commandGutter.attach(terminal)
  lineTimestamps.attach(terminal)
  oscSubscription = terminal.parser.registerOscHandler(1337, (data) => {
    const match = /^CurrentDir=(.*)$/.exec(data)
    if (match?.[1]) {
      currentDirectory.value = match[1]
      window.dispatchEvent(
        new CustomEvent('qterm:current-dir', { detail: { sessionId: props.sessionId, directory: match[1] } }),
      )
    }
    return Boolean(match)
  })
  if (typeof terminal.onSelectionChange === 'function') {
    selectionSubscription = terminal.onSelectionChange(() => {
      if (!props.autoCopySelection || !terminal?.hasSelection()) return
      selectedText.value = terminal.getSelection()
      void copySelection()
    })
  }

  dataSubscription = terminal.onData((data) => {
    if (!ended.value && streamStatus.value === 'ready' && !zmodemActive.value) {
      if (credentialInputKind.value === null) checkCredentialPrompt()
      if (credentialInputKind.value !== null) {
        channel?.sendInput(data)
        if (completesCredentialInput(data)) waitForCredentialResponse()
        return
      }
    }

    if (credentialOpen.value) closeCredentialSuggestion()
    if (!ended.value && streamStatus.value === 'ready') {
      if (zmodemActive.value) {
        return
      }

      const submittedEnter = data === '\r' || data === '\n'
      const approximateCommand = submittedEnter && typedCommand.trim().length > 0

      if (
        submittedEnter &&
        /^(?:(?:sudo|env)\s+)?(?:ssh|mosh|telnet|sftp|bash|zsh|fish|sh|python\d*|node|irb|psql|mysql|sqlite3|redis-cli)(?:\s|$)/.test(
          typedCommand.trim(),
        )
      ) {
        commandGutter.suspendFallback()
      }
      trackTypedInput(data)
      if (approximateCommand) commandGutter.approximateEnter(true)
      if (props.hostId && isShellHistoryNavigation(data)) {
        pendingShellTabSubmission = null
        clearShellTabSubmitTimer()
        clearShellTabState()
        commandLineHasShellHistoryRecall = true
        shellTabCompletedCommand = ''
      }
      if (props.hostId && data === '\t') {
        commandLineHasShellHistoryRecall = false
        commandLineHasShellTab = true

        shellTabPrefix = typedCommand
        shellTabCompletedCommand = ''
      }
      if (props.hostId && (data.includes('\x03') || data.includes('\x15'))) clearShellTabState()

      if (
        props.hostId &&
        (data === '\r' || data === '\n') &&
        (commandLineHasShellTab || commandLineHasShellHistoryRecall) &&
        !shellTabCompletedCommand
      ) {
        const submitted = commandLineBuffer.feed(data)
        const typed = submitted[0] ?? ''
        if (typed || commandLineHasShellHistoryRecall) {
          pendingShellTabSubmission = { typed, terminator: data }
          clearShellTabSubmitTimer()
          shellTabSubmitTimer = window.setTimeout(flushPendingShellTabSubmission, SHELL_TAB_SUBMIT_TIMEOUT_MS)
          return
        }
        clearShellTabState()
      }
      if (pendingShellTabSubmission && (data.includes('\x03') || data.includes('\x15'))) {
        pendingShellTabSubmission = null
        clearShellTabSubmitTimer()
        clearShellTabState()
      } else if (pendingShellTabSubmission) {
        flushPendingShellTabSubmission()
      }
      channel?.sendInput(data)

      if (props.hostId) {
        for (const typed of commandLineBuffer.feed(data)) {
          const screenLine = currentScreenLine(terminal)
          if (isSensitivePrompt(screenLine)) {
            clearShellTabState()
            continue
          }

          const command =
            commandLineHasShellTab || commandLineHasShellHistoryRecall
              ? resolveShellTabSubmission(screenLine, typed)
              : typed
          void saveCommandHistory({
            hostId: props.hostId,
            hostName: props.hostName,
            sessionId: props.sessionId,

            command,
          }).catch(() => undefined)
          clearShellTabState()
        }

        if (commandLineHasShellHistoryRecall && (data === '\r' || data === '\n')) {
          const screenLine = currentScreenLine(terminal)
          if (!isSensitivePrompt(screenLine)) {
            const command = extractSubmittedScreenCommand(screenLine)
            if (command) {
              void saveCommandHistory({
                hostId: props.hostId,
                hostName: props.hostName,
                sessionId: props.sessionId,
                command,
              }).catch(() => undefined)
            }
          }
        }
        if (data.includes('\r') || data.includes('\n')) clearShellTabState()
      }
    }
  })
  if (typeof terminal.attachCustomKeyEventHandler === 'function') {
    terminal.attachCustomKeyEventHandler((event) => {
      if (
        props.backspaceMode === 'bs' &&
        event.type === 'keydown' &&
        event.key === 'Backspace' &&
        !event.ctrlKey &&
        !event.metaKey &&
        !event.altKey
      ) {
        event.preventDefault()
        if (!ended.value && streamStatus.value === 'ready' && !zmodemActive.value) {
          channel?.sendInput('\b')
        }
        return false
      }

      if (!handleCredentialKey(event)) return false
      if (!handleCompletionKey(event)) return false

      const boundAction = boundShortcutAction(event, shortcutBindings, isMacPlatform.value)
      if (boundAction) return true

      const copyBinding = shortcutBindings.copyTerminalSelection
      if (
        !copyBinding &&
        (event.key === 'c' || event.key === 'C') &&
        (event.ctrlKey || event.metaKey) &&
        terminal?.hasSelection()
      ) {
        copySelectionFromTerminal()
        return false
      }
      return true
    })
  }

  zmodemCancelHandler = (e: Event) => {
    if ((e as CustomEvent).detail?.sessionId === props.sessionId) zmodemManager?.abort()
  }
  window.addEventListener('qterm:zmodem-cancel', zmodemCancelHandler)
  window.addEventListener('qterm:layout-resize-end', handleLayoutResizeEnd)
  localFileDropHandler = (e: Event) => {
    if (props.kind !== 'local' || !props.active || streamStatus.value !== 'ready') return
    const detail = (e as CustomEvent<{ sessionId?: string; paths?: string[] }>).detail
    if (detail?.sessionId !== props.sessionId || !detail.paths?.length) return
    for (const filePath of detail.paths) channel?.sendInput(filePath)
  }
  window.addEventListener('qterm:local-file-drop', localFileDropHandler)
  terminalThemeUpdatedHandler = () => {
    if (terminal) terminal.options.theme = terminalTheme(props.dark)
  }
  window.addEventListener('qterm:terminal-theme-updated', terminalThemeUpdatedHandler)
  window.addEventListener('qterm:toggle-terminal-pause', handleOutputPauseShortcut)

  paneRoot.value?.addEventListener('paste', handleNativePaste, true)
  window.addEventListener('qterm:command-history-changed', loadCompletionHistory)
  void loadCompletionHistory()

  showClosePrompt()

  if (typeof ResizeObserver === 'function') {
    resizeObserver = new ResizeObserver(handleResizeObservation)
    resizeObserver.observe(element)
  }

  window.addEventListener('resize', handleWindowResize)
  watchDevicePixelRatio()

  void nextTick(async () => {
    await prepareInitialTerminalFont()
    if (!terminal) return
    initialFontPrepared = true
    initialLayoutReady = true

    syncWebglLease()
    if (isVisible.value) applyFit()
    connectIfReady()
  })
  window.addEventListener('focus', refreshAfterWindowResume)
  document.addEventListener('visibilitychange', handleVisibilityResume)
  logTerminalPresentationDiagnostics()
})

watch(
  isVisible,
  (visible) => {
    if (visible) {
      updateWebglContext(webglContextOwner, webglContextState())

      void ensureTerminalFontReady(props.fontSize, props.fontFamily)
      visibleLayoutRecoveryPending = true

      const rendererNeedsRestore = holdsWebglLease.value && !webglAddon
      preserveVisibleFrameDuringRecovery = rendererNeedsRestore && showTerminalFrameSnapshot()
      layoutPresentationReady.value = true
      presentationPending.value = rendererNeedsRestore
      if (preserveVisibleFrameDuringRecovery) notifySnapshotPresentationReady()
      else if (!rendererNeedsRestore) emit('presentationReady', props.sessionId)
    } else {
      cancelSnapshotPresentationReady()

      captureTerminalFrameSnapshot()
      unmountTerminalFrameSnapshot()
      updateWebglContext(webglContextOwner, webglContextState())
    }
    void nextTick(() => {
      if (!visible) {
        if (isVisible.value) return
        visibleLayoutRecoveryRevision += 1
        visibleLayoutRecoveryPending = false
        preserveVisibleFrameDuringRecovery = false
        layoutPresentationReady.value = true
        presentationPending.value = false
        if (visibleRefreshFrame !== null) window.cancelAnimationFrame(visibleRefreshFrame)
        visibleRefreshFrame = null

        terminal?.blur()
        syncWebglLease()
        return
      }
      if (!isVisible.value) return
      refreshVisibleTerminal()
    })
  },
  { flush: 'sync' },
)

watch(
  () => props.layoutRevision,
  () => {
    if (!initialLayoutReady || !terminal) return
    pendingWebglLayoutRevision = props.layoutRevision
    if (!isVisible.value) return
    visibleLayoutRecoveryPending = true

    captureTerminalFrameSnapshot()
    preserveVisibleFrameDuringRecovery = showTerminalFrameSnapshot()
    layoutPresentationReady.value = true
    void nextTick(refreshVisibleTerminal)
  },
)

watch(
  () => props.renderEngine,
  () => {
    applyRenderEngine()
    repaintTerminal()
    queueFit()
  },
)

watch(
  () => props.active,
  (active) => {
    updateWebglContext(webglContextOwner, webglContextState())
    if (active) {
      if (holdsWebglLease.value && !webglAddon) {
        visibleLayoutRecoveryPending = true
        refreshVisibleTerminal()
      }
      void nextTick(() => {
        terminal?.focus()
      })
    } else {
      terminal?.blur()
    }
  },
)

watch(ended, (isEnded) => {
  if (isEnded) {
    clearCredentialInputState()
    zmodemManager?.reset()

    markDisconnected()
  }
  scheduleClosePrompt()
})

watch(
  () => props.state,
  (state) => {
    if (state !== 'closed' && state !== 'failed') return
    markDisconnected()
    scheduleClosePrompt(true)
  },
)

watch(
  () => props.reconnecting,
  (reconnecting) => {
    if (reconnecting) {
      clearClosePromptTimer()
      closePromptWritten = false
      closingRequested = false

      markReconnecting('terminal.reconnectingSeparator')
      return
    }

    if (props.state === 'closed' || props.state === 'failed') scheduleClosePrompt(true)
  },
)

watch(
  () => props.autoReconnectPending,
  (pending) => {
    if (pending) {
      clearClosePromptTimer()

      showClosePrompt()
      return
    }
    if (props.state === 'closed' || props.state === 'failed') scheduleClosePrompt(true)
  },
)

watch(
  () => [props.dark, props.transparentBackground, props.lightTheme, props.darkTheme],
  () => {
    if (!terminal) return

    terminal.options.allowTransparency = props.transparentBackground
    terminal.options.theme = terminalTheme(props.dark)
    applyRenderEngine()
    repaintTerminal()
    queueFit()
  },
)

watch(
  () => [props.cursorBlink, props.cursorStyle] as const,
  ([blink, style]) => {
    if (!terminal) return
    terminal.options.cursorBlink = blink
    terminal.options.cursorStyle = style
  },
)

watch(
  () => props.wordSeparators,
  (value) => {
    if (terminal && typeof value === 'string') terminal.options.wordSeparator = value
  },
)

watch(
  () => props.macOptionIsMeta,
  (value) => {
    if (terminal) terminal.options.macOptionIsMeta = value === true
  },
)

watch(
  () => props.completionEnabled,
  (enabled) => {
    if (!enabled) closeCompletion()
    else refreshCompletionItems()
  },
)

watch(
  () => [props.historyMinLength, props.historyMaxLength] as const,
  () => refreshCompletionItems(),
)

watch(
  () => [props.keywordHighlightEnabled, props.keywordHighlightRules] as const,
  () => keywordHighlights.queueKeywordHighlights(),
  { deep: true },
)

watch(
  () => [props.fontSize, props.lineHeight, props.fontFamily] as const,
  () => {
    if (!terminal) return
    fontPresentationReady.value = false
    armPresentationWatchdog()

    if (!initialLayoutReady) {
      webglActivationReady.value = false
      return
    }
    firstOutputFontCorrectionPending = false

    void (async () => {
      try {
        await applyFontWhenReady()
      } finally {
        fontPresentationReady.value = true
      }
    })()
  },
)

watch(
  () => [props.state, props.streamToken] as const,
  () => void nextTick(connectIfReady),
)

watch(
  () => props.hostId,
  () => {
    clearCredentialInputState()
    pendingShellTabSubmission = null
    clearShellTabSubmitTimer()
    clearShellTabState()
    void loadCompletionHistory()
  },
)

watch(
  () => props.sessionId,
  (sessionId, previousSessionId) => {
    if (sessionId === previousSessionId) return

    if (previousSessionId) forgetTerminalBridge(previousSessionId)
    bindTerminalOutputCapture(sessionId)
    if (sessionId) registerTerminalBridge(sessionId)
  },
  { flush: 'sync' },
)

onBeforeUnmount(() => {
  disarmPresentationWatchdog()
  cancelSnapshotPresentationReady()
  isTerminalPaneUnmounted = true
  forgetTerminalBridge()
  outputSubscription?.dispose()
  outputSubscription = null
  clearSessionDirectoryState(props.sessionId)
  fontRefreshRevision += 1
  if (visibleRefreshFrame !== null) window.cancelAnimationFrame(visibleRefreshFrame)
  visibleRefreshFrame = null
  dprQuery?.removeEventListener('change', handleDevicePixelRatioChange)
  dprQuery = null
  window.removeEventListener('qterm:recordings-changed', recordingChangedHandler)
  pendingShellTabSubmission = null
  clearShellTabSubmitTimer()
  zmodemManager?.reset()
  zmodemManager = null
  if (zmodemCancelHandler) window.removeEventListener('qterm:zmodem-cancel', zmodemCancelHandler)
  if (localFileDropHandler) window.removeEventListener('qterm:local-file-drop', localFileDropHandler)
  window.removeEventListener('qterm:layout-resize-end', handleLayoutResizeEnd)
  if (terminalThemeUpdatedHandler)
    window.removeEventListener('qterm:terminal-theme-updated', terminalThemeUpdatedHandler)
  window.removeEventListener('qterm:command-history-changed', loadCompletionHistory)
  window.removeEventListener('qterm:toggle-terminal-pause', handleOutputPauseShortcut)
  paneRoot.value?.removeEventListener('paste', handleNativePaste, true)
  clearCredentialInputState()
  resizeObserver?.disconnect()
  window.removeEventListener('resize', handleWindowResize)
  window.removeEventListener('focus', refreshAfterWindowResume)
  document.removeEventListener('visibilitychange', handleVisibilityResume)
  if (resizeFreezeTimer !== null) clearTimeout(resizeFreezeTimer)
  if (windowResizeRecoveryTimer !== null) clearTimeout(windowResizeRecoveryTimer)
  if (resizeNotifyTimer !== null) window.clearTimeout(resizeNotifyTimer)

  clearClosePromptTimer()

  if (channel) unregisterTerminalChannel(connectedSessionID || props.sessionId, channel)
  channel?.close()
  dataSubscription?.dispose()
  oscSubscription?.dispose()
  selectionSubscription?.dispose()
  keywordHighlights.dispose()
  gutter.detach()
  commandGutter.detach()
  lineTimestamps.detach()
  unmountTerminalFrameSnapshot()
  terminalFrameSnapshot = null
  frameSnapshotAvailable.value = false
  disposeWebglAddon()
  forgetWebglContext(webglContextOwner)

  terminal?.blur()
  searchResultSubscription?.dispose()
  searchResultSubscription = null
  searchAddon = null
  searchOpen.value = false
  fitAddon?.dispose()
  terminal?.dispose()
  resizeObserver = null
  webglAddon = null
  fitAddon = null
  terminal = null
  channel = null
  dataSubscription = null
})
</script>

<template>
  <ContextMenu @update:open="onMenuOpen">
    <ContextMenuTrigger as-child>
      <div
        ref="paneRoot"
        class="terminal-session-pane absolute inset-0 min-w-0 min-h-0 p-[12px] bg-qt-terminal invisible pointer-events-none"
        :class="{ 'is-terminal-visible': isVisible }"
        data-file-drop-target
        data-drop-kind="terminal"
        :data-drop-session="props.sessionId"
        :data-drop-directory="currentDirectory"
        :data-drop-hint="props.kind === 'local' ? t('shell.terminal.dropInsert') : t('shell.terminal.dropUpload')"
        :data-session-id="sessionId"
        :data-frame-snapshot-available="frameSnapshotAvailable"
        :aria-hidden="!isVisible"
        :inert="!isVisible"
        :aria-label="t('shell.terminal.aria', { host: hostName })"
        @keydown.capture="handleKeydown"
        @contextmenu.capture="handleContextMenu"
      >
        <p class="sr-only terminal-accessible-description">
          <template v-if="props.kind === 'local'">{{
            t('shell.terminal.localDescription', { status: accessibleStatus })
          }}</template>
          <template v-else>{{
            t('shell.terminal.remoteDescription', {
              connection: `${username}@${address}:${port}`,
              status: accessibleStatus,
            })
          }}</template>
        </p>
        <p v-if="sessionWarnings.length" class="terminal-session-warning text-xs" role="status">
          <TriangleAlert :size="14" aria-hidden="true" />
          <span>{{ sessionWarnings.join(' ') }}</span>
        </p>

        <button
          v-if="outputPaused"
          type="button"
          class="terminal-pause-indicator"
          :title="t('shell.terminal.outputPausedHint')"
          data-terminal-pause-indicator
          @click="toggleOutputPause(false)"
        >
          <Pause :size="13" aria-hidden="true" />
          <span>{{ t('shell.terminal.outputPaused') }}</span>
          <span class="terminal-pause-resume">{{ t('shell.terminal.outputResume') }}</span>
        </button>

        <div class="flex h-full min-h-0 min-w-0">
          <TerminalGutter
            v-if="gutterVisible"
            :rows="displayedGutterRows"
            :width="displayedGutterWidth"
            :row-height="gutterRowHeight"
            :top-padding="gutterTopPadding"
            :color="gutterColor"
            :font-family="gutterFontFamily"
            :font-size="props.fontSize"
            :suspended="gutterSuspended"
            @wheel="handleGutterWheel"
            @focus="handleGutterFocus"
          />
          <div class="relative h-full min-h-0 min-w-0 flex-1">
            <div
              ref="terminalElement"
              class="qterm-terminal relative min-w-0 [contain:paint_style]"
              :class="{
                'is-font-pending': !fontPresentationReady,
                'is-layout-pending': !layoutPresentationReady,
              }"
              aria-hidden="true"
            ></div>
          </div>
        </div>
        <div v-if="isVisible && presentationPending" class="terminal-presentation-loading" aria-hidden="true">
          <Loader2 :size="18" class="spin" />
        </div>

        <div
          v-if="searchOpen"
          class="absolute top-2 right-2 z-20 flex max-w-[calc(100%_-_1rem)] flex-col gap-1 rounded-md border-d bg-qt-panel px-1.5 py-1 shadow-lg"
          role="search"
        >
          <div class="flex h-5 items-center justify-end gap-1">
            <span class="text-xs text-qt-muted">{{ searchSummary }}</span>
            <button
              type="button"
              class="flex size-5 items-center justify-center rounded text-sm text-qt-muted"
              :title="t('shell.terminal.searchClose')"
              @click="closeSearch"
            >
              <X :size="14" />
            </button>
          </div>

          <div class="flex min-w-0 items-center gap-1">
            <div
              class="flex min-h-8 w-64 min-w-0 max-w-full items-center rounded-md border border-qt-border transition-colors focus-within:border-qt-focus focus-within:ring-3 focus-within:ring-qt-focus/16"
            >
              <QtermInput
                ref="searchInput"
                :model-value="searchQuery"
                type="text"
                class="h-8! min-h-0 min-w-0 flex-1 rounded-none border-0 bg-transparent! pl-2 pr-1 text-xs ring-0! shadow-none!"
                :placeholder="t('shell.terminal.searchPlaceholder')"
                :aria-label="t('shell.terminal.search')"
                @update:model-value="handleSearchInput"
                @keydown.enter.prevent="runSearch($event.shiftKey ? -1 : 1)"
                @keydown.esc.prevent="closeSearch"
              />

              <div class="flex shrink-0 items-center gap-px pr-1">
                <button
                  type="button"
                  class="flex size-5 items-center justify-center rounded text-xs leading-none"
                  :class="searchCaseSensitive ? 'bg-qt-accent text-qt-accent-text' : 'text-qt-muted'"
                  :aria-pressed="searchCaseSensitive"
                  :title="t('shell.terminal.searchCaseSensitive')"
                  @click="toggleSearchOption('caseSensitive')"
                >
                  <LucideCaseSensitive :size="14" />
                </button>
                <button
                  type="button"
                  class="flex size-5 items-center justify-center rounded text-xs leading-none"
                  :class="searchWholeWord ? 'bg-qt-accent text-qt-accent-text' : 'text-qt-muted'"
                  :aria-pressed="searchWholeWord"
                  :title="t('shell.terminal.searchWholeWord')"
                  @click="toggleSearchOption('wholeWord')"
                >
                  <WholeWord :size="14" />
                </button>
                <button
                  type="button"
                  class="flex size-5 items-center justify-center rounded text-xs leading-none"
                  :class="searchRegex ? 'bg-qt-accent text-qt-accent-text' : 'text-qt-muted'"
                  :aria-pressed="searchRegex"
                  :title="t('shell.terminal.searchRegex')"
                  @click="toggleSearchOption('regex')"
                >
                  <Regex :size="14" />
                </button>
                <button
                  type="button"
                  class="flex size-5 items-center justify-center rounded text-xs leading-none"
                  :class="searchWrap ? 'bg-qt-accent text-qt-accent-text' : 'text-qt-muted'"
                  :aria-pressed="searchWrap"
                  :title="t('shell.terminal.searchWrap')"
                  @click="toggleSearchOption('wrap')"
                >
                  <LucideRefreshCw :size="14" />
                </button>
              </div>
            </div>
            <button
              type="button"
              class="flex size-5 items-center justify-center rounded text-xs text-qt-muted"
              :title="t('shell.terminal.searchPrevious')"
              @click="runSearch(-1)"
            >
              <ArrowUp :size="14" />
            </button>
            <button
              type="button"
              class="flex size-5 items-center justify-center rounded text-xs text-qt-muted"
              :title="t('shell.terminal.searchNext')"
              @click="runSearch(1)"
            >
              <ArrowDown :size="14" />
            </button>
          </div>
        </div>
        <Teleport to="body">
          <div
            v-if="completionOpen"
            class="terminal-command-completion"
            :style="{
              left: `${completionPosition.left}px`,
              top: `${completionPosition.top}px`,
              '--qt-completion-bg': props.dark ? '#151517' : '#ffffff',
            }"
            role="listbox"
            :aria-label="t('shell.terminal.completionAria')"
            @mousedown.prevent="focusCompletionTerminal"
            @click.stop
          >
            <div class="terminal-command-completion-header justify-between border-b-d px-[10px] py-[7px] text-sm">
              <span class="terminal-command-completion-title inline-flex items-center gap-[6px]"
                ><HistoryIcon :size="13" />{{ t('shell.terminal.completionTitle') }}</span
              >
              <span>{{ completionItems.length }} {{ t('shell.terminal.completionMatches') }}</span>
            </div>
            <ScrollArea class="terminal-command-completion-scroll" type="hover" :scroll-hide-delay="600">
              <div
                v-for="(command, index) in completionItems"
                :key="command"
                class="terminal-command-completion-item text-xs"
                :class="{ 'is-selected': index === completionIndex }"
                role="option"
                :aria-selected="index === completionIndex"
                @mouseenter="completionIndex = index"
                @mousedown.prevent="focusCompletionTerminal"
                @click="sendCompletionCommand(command, false)"
              >
                <HistoryIcon :size="13" class="terminal-command-completion-icon text-qt-muted font-[sans-serif]" />
                <span>
                  <template
                    v-for="(part, partIndex) in completionMatchParts(command, completionQuery)"
                    :key="partIndex"
                  >
                    <mark
                      v-if="part.matched"
                      class="terminal-command-completion-match [background:color-mix(in_srgb,_var(--qt-info,_#2563eb)_14%,_transparent)] text-(--qt-info, #2563eb) font-bold [text-shadow:none] text-(--qt-info, #2563eb)"
                      >{{ part.text }}</mark
                    >
                    <template v-else>{{ part.text }}</template>
                  </template>
                </span>
                <button
                  type="button"
                  class="terminal-command-completion-delete"
                  :aria-label="t('shell.terminal.completionDelete')"
                  :title="t('shell.terminal.completionDelete')"
                  @mousedown.prevent="focusCompletionTerminal"
                  @click.stop="void hideCompletionCommand(command)"
                >
                  <Trash2 :size="12" />
                </button>
              </div>
            </ScrollArea>
            <div class="terminal-command-completion-help text-xs">
              <span><kbd class="text-xs">↑↓</kbd>{{ t('shell.terminal.completionSelect') }}</span>
              <span><kbd class="text-xs">Tab</kbd>{{ t('shell.terminal.completionFill') }}</span>
              <span><kbd class="text-xs">Enter</kbd>{{ t('shell.terminal.completionRun') }}</span>
              <span><kbd class="text-xs">Esc</kbd>{{ t('shell.terminal.completionClose') }}</span>
            </div>
          </div>
        </Teleport>

        <Teleport to="body">
          <div
            v-if="credentialOpen"
            class="terminal-command-completion terminal-credential-suggestion"
            :style="{
              left: `${credentialPosition.left}px`,
              top: `${credentialPosition.top}px`,
              '--qt-completion-bg': props.dark ? '#151517' : '#ffffff',
            }"
            role="listbox"
            :aria-label="t('shell.terminal.credentialSuggestionAria')"
            @mousedown.prevent="focusCompletionTerminal"
            @click.stop
          >
            <div class="terminal-command-completion-header justify-between border-b-d px-[10px] py-[7px] text-sm">
              <span class="terminal-command-completion-title inline-flex items-center gap-[6px]">
                <KeyRound :size="13" />
                {{
                  credentialKind === 'username'
                    ? t('shell.terminal.credentialSuggestionUsername')
                    : t('shell.terminal.credentialSuggestionPassword')
                }}
              </span>
              <span>{{ credentialMatches.length }}</span>
            </div>
            <ScrollArea class="terminal-command-completion-scroll" type="hover" :scroll-hide-delay="600">
              <div
                v-for="(credential, index) in credentialMatches"
                :key="credential.id"
                class="terminal-command-completion-item text-xs"
                :class="{ 'is-selected': index === credentialIndex }"
                role="option"
                :aria-selected="index === credentialIndex"
                @mouseenter="credentialIndex = index"
                @mousedown.prevent="focusCompletionTerminal"
                @click="void fillCredential(credential)"
              >
                <KeyRound :size="13" class="terminal-command-completion-icon text-qt-muted font-[sans-serif]" />
                <span class="terminal-credential-name min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">{{
                  credential.name
                }}</span>
                <small
                  v-if="credential.username"
                  class="terminal-credential-username flex-none ml-auto text-qt-muted text-xs"
                  >{{ credential.username }}</small
                >
              </div>
            </ScrollArea>
            <div class="terminal-command-completion-help text-xs">
              <span><kbd class="text-xs">↑↓</kbd>{{ t('shell.terminal.credentialSelect') }}</span>
              <span><kbd class="text-xs">Enter</kbd>{{ t('shell.terminal.credentialFill') }}</span>
              <span><kbd class="text-xs">Esc</kbd>{{ t('shell.terminal.completionClose') }}</span>
            </div>
          </div>
        </Teleport>
        <span
          class="file-drop-hint absolute z-21 inset-0 grid place-items-center invisible pointer-events-none"
          aria-hidden="true"
        >
          <span class="text-sm">{{
            props.kind === 'local' ? t('shell.terminal.dropInsert') : t('shell.terminal.dropUpload')
          }}</span>
        </span>
      </div>
    </ContextMenuTrigger>
    <ContextMenuContent
      class="asset-context-menu min-w-54"
      @close-auto-focus.prevent="focusAfterMenu"
      @interact-outside="preserveOutsideFocus"
    >
      <ContextMenuItem :disabled="!selectedText || clipboardBusy" @select="copySelection"
        ><Copy :size="14" />{{ t('shell.terminal.copy') }}

        <ContextMenuShortcut>
          <ShortcutKeycaps
            :tokens="menuShortcutTokens('copy')"
            :label="menuShortcutLabel('copy')"
          /> </ContextMenuShortcut
      ></ContextMenuItem>
      <ContextMenuItem
        v-if="props.kind === 'ssh'"
        :disabled="!props.address.trim() || clipboardBusy"
        @select="copyAddress"
        ><Network :size="14" />{{ t('shell.terminal.copyAddress') }}</ContextMenuItem
      >
      <ContextMenuItem :disabled="!canPaste" @select="pasteClipboard"
        ><ClipboardPaste :size="14" />{{ t('shell.terminal.paste') }}
        <ContextMenuShortcut>
          <ShortcutKeycaps
            :tokens="menuShortcutTokens('paste')"
            :label="menuShortcutLabel('paste')"
          /> </ContextMenuShortcut
      ></ContextMenuItem>
      <ContextMenuItem :disabled="!selectedText || !canPaste" @select="pasteSelection"
        ><ClipboardCheck :size="14" />{{ t('shell.terminal.pasteSelection') }}</ContextMenuItem
      >

      <ContextMenuItem v-if="selectedText" @select="saveAsSnippet"
        ><BookmarkPlus :size="14" />{{ t('shell.terminal.saveAsSnippet') }}</ContextMenuItem
      >
      <ContextMenuItem v-if="selectedText" @select="emit('aiAnalyze', props.sessionId, 'selection', selectedText)"
        ><Sparkles :size="14" />{{ t('shell.terminal.aiAnalyzeSelection') }}</ContextMenuItem
      >
      <ContextMenuItem :disabled="!sessionReady" @select="emit('aiAnalyze', props.sessionId, 'recent', '')"
        ><Sparkles :size="14" />{{ t('shell.terminal.aiAnalyzeRecent') }}</ContextMenuItem
      >
      <ContextMenuSub v-if="selectedText">
        <ContextMenuSubTrigger><Globe :size="14" />{{ t('shell.terminal.webSearch') }}</ContextMenuSubTrigger>
        <ContextMenuSubContent class="min-w-32">
          <ContextMenuItem v-for="engine in webSearchEngines" :key="engine.id" @select="openWebSearch(engine.id)">
            <img :src="engine.icon" alt="" class="size-3.5 shrink-0" draggable="false" />
            {{ t(engine.labelKey) }}
          </ContextMenuItem>
        </ContextMenuSubContent>
      </ContextMenuSub>
      <ContextMenuSub v-if="props.translateEnabled && selectedText">
        <ContextMenuSubTrigger><Languages :size="14" />{{ t('shell.terminal.translate') }}</ContextMenuSubTrigger>
        <ContextMenuSubContent class="min-w-40">
          <ContextMenuItem
            v-for="provider in availableTranslateProviders"
            :key="provider"
            :disabled="translating"
            @select="translateSelection(provider)"
          >
            <img :src="TRANSLATE_PROVIDER_ICONS[provider]" alt="" class="size-3.5 shrink-0" draggable="false" />
            {{ translateProviderLabels[provider] }}
          </ContextMenuItem>
        </ContextMenuSubContent>
      </ContextMenuSub>
      <ContextMenuSeparator />
      <ContextMenuItem @select="selectAll"
        ><TextSelect :size="14" />{{ t('shell.terminal.selectAll') }}</ContextMenuItem
      >
      <ContextMenuItem @select="clearTerminal"
        ><Eraser :size="14" />{{ t('shell.terminal.clear') }}
        <ContextMenuShortcut>
          <ShortcutKeycaps
            :tokens="menuShortcutTokens('clear')"
            :label="menuShortcutLabel('clear')"
          /> </ContextMenuShortcut
      ></ContextMenuItem>

      <ContextMenuItem @select="toggleOutputPause()">
        <Play v-if="outputPaused" :size="14" aria-hidden="true" />
        <Pause v-else :size="14" aria-hidden="true" />
        {{ outputPaused ? t('shell.terminal.outputResume') : t('shell.terminal.outputPaused') }}
      </ContextMenuItem>
      <ContextMenuItem :disabled="!sessionReady" @select="openSearch"
        ><Search :size="14" />{{ t('shell.terminal.search') }}
        <ContextMenuShortcut>
          <ShortcutKeycaps
            :tokens="menuShortcutTokens('search')"
            :label="menuShortcutLabel('search')"
          /> </ContextMenuShortcut
      ></ContextMenuItem>
      <ContextMenuItem v-if="props.kind === 'ssh' && sessionReady" @select="emit('openSftp')"
        ><FolderOpen :size="14" />{{ t('shell.terminal.openSftp') }}</ContextMenuItem
      >
      <ContextMenuSeparator />

      <ContextMenuItem @select="emit('requestSplit', props.sessionId, 'right')"
        ><Columns2 :size="14" />{{ t('shell.sessionMenu.splitRight') }}
        <ContextMenuShortcut>
          <ShortcutKeycaps
            :tokens="menuShortcutTokens('splitRight')"
            :label="menuShortcutLabel('splitRight')"
          /> </ContextMenuShortcut
      ></ContextMenuItem>
      <ContextMenuItem @select="emit('requestSplit', props.sessionId, 'bottom')"
        ><Rows2 :size="14" />{{ t('shell.sessionMenu.splitBottom') }}
        <ContextMenuShortcut>
          <ShortcutKeycaps
            :tokens="menuShortcutTokens('splitBottom')"
            :label="menuShortcutLabel('splitBottom')"
          /> </ContextMenuShortcut
      ></ContextMenuItem>
      <ContextMenuSeparator />
      <ContextMenuItem :disabled="props.state === 'closing'" @select="emit('reconnectSession', props.sessionId)"
        ><RefreshCw :size="14" />{{ t('shell.terminal.reconnect') }}
        <ContextMenuShortcut>
          <ShortcutKeycaps
            :tokens="menuShortcutTokens('reconnect')"
            :label="menuShortcutLabel('reconnect')"
          /> </ContextMenuShortcut
      ></ContextMenuItem>
      <ContextMenuItem
        :disabled="['preview', 'closing', 'closed', 'failed'].includes(props.state)"
        @select="emit('disconnectSession', props.sessionId)"
        ><Unplug :size="14" />{{ t('shell.terminal.disconnect') }}
        <ContextMenuShortcut>
          <ShortcutKeycaps
            :tokens="menuShortcutTokens('disconnect')"
            :label="menuShortcutLabel('disconnect')"
          /> </ContextMenuShortcut
      ></ContextMenuItem>
      <ContextMenuSeparator />
      <ContextMenuItem v-if="recordingState === 'none'" :disabled="recordingBusy" @select="changeRecording('start')"
        ><Circle :size="14" />{{ t('shell.terminal.startRecording') }}</ContextMenuItem
      >
      <ContextMenuItem
        v-else-if="recordingState === 'recording'"
        :disabled="recordingBusy"
        @select="changeRecording('pause')"
        ><Pause :size="14" />{{ t('shell.terminal.pauseRecording') }}</ContextMenuItem
      >
      <ContextMenuItem v-else :disabled="recordingBusy" @select="changeRecording('resume')"
        ><Play :size="14" />{{ t('shell.terminal.resumeRecording') }}</ContextMenuItem
      >
      <ContextMenuItem v-if="recordingState !== 'none'" :disabled="recordingBusy" @select="changeRecording('stop')"
        ><Square :size="14" />{{ t('shell.terminal.stopRecording') }}</ContextMenuItem
      >
      <ContextMenuSeparator />
      <ContextMenuItem @select="emit('openSettings')"
        ><Settings :size="14" />{{ t('shell.terminal.settings') }}</ContextMenuItem
      >
    </ContextMenuContent>
  </ContextMenu>

  <QtermDialog :open="pendingPaste !== null" @update:open="(open) => !open && (pendingPaste = null)">
    <QtermDialogContent v-if="pendingPaste !== null" data-paste-confirm-dialog>
      <QtermDialogHeader
        :title="t('shell.terminal.pasteConfirmTitle')"
        :description="t('shell.terminal.pasteConfirmMessage', { lines: pendingPasteLines })"
        :show-close="false"
      />
      <div class="dialog-body flex flex-col gap-2">
        <pre class="paste-preview">{{ pendingPastePreview }}</pre>
        <p class="m-0 text-xs text-qt-muted">{{ t('shell.terminal.pasteConfirmDetail') }}</p>
      </div>
      <QtermDialogFooter>
        <div></div>
        <div class="dialog-footer-actions flex items-center gap-[8px]">
          <QtermButton variant="outline" @click="pendingPaste = null">{{ t('shell.common.cancel') }}</QtermButton>
          <QtermButton data-paste-confirm @click="confirmPendingPaste">{{ t('shell.common.confirm') }}</QtermButton>
        </div>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>

  <QtermDialog :open="translateOpen" @update:open="(open) => !open && closeTranslation()">
    <QtermDialogContent v-if="translateOpen" size="lg">
      <QtermDialogHeader
        :title="t('shell.translate.title')"
        :badge="availableTranslateProviders.includes(selectedTranslateProvider) ? selectedTranslateProviderLabel : ''"
        @close="closeTranslation"
      />

      <div class="dialog-body flex-col gap-3">
        <p v-if="availableTranslateProviders.length === 0" class="m-0 text-xs text-qt-muted" role="alert">
          {{ t('shell.translate.noProvider') }}
        </p>
        <span class="text-xs text-qt-muted">{{ t('shell.translate.source') }}</span>
        <pre class="translate-text rounded-[10px] border border-qt-border p-3">{{ translateSource }}</pre>
        <div class="flex items-center justify-between gap-2 text-xs text-qt-muted">
          <span>{{ t('shell.translate.result') }}</span>
          <span v-if="translateDetectedLanguage">{{
            t('shell.translate.detected', { language: translateDetectedLanguage })
          }}</span>
        </div>
        <div
          v-if="availableTranslateProviders.length > 0 || translateError"
          class="min-h-20 rounded-[10px] border border-qt-border p-3"
        >
          <pre v-if="translateResult" class="translate-text">{{ translateResult }}</pre>
          <p v-else-if="translateError" class="m-0 text-xs text-qt-danger" role="alert">{{ translateError }}</p>
          <p v-else class="m-0 text-xs text-qt-muted">{{ t('shell.translate.loading') }}</p>
        </div>
      </div>
      <QtermDialogFooter>
        <QtermButton
          class="mr-auto"
          type="button"
          :disabled="!translateResult || availableTranslateProviders.length === 0"
          @click="copyTranslation"
        >
          <Copy :size="14" aria-hidden="true" />
          {{ t('shell.translate.copy') }}
        </QtermButton>
        <QtermButton variant="outline" type="button" @click="closeTranslation">
          {{ t('shell.app.close') }}
        </QtermButton>
      </QtermDialogFooter>
    </QtermDialogContent>
  </QtermDialog>
</template>

<style scoped>
.terminal-pause-indicator {
  position: absolute;
  right: 14px;
  bottom: 10px;
  z-index: 20;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border: 1px solid var(--qt-border);
  border-radius: 999px;
  background: var(--qt-panel);
  color: var(--qt-text);
  font-size: 11px;
  line-height: 1.4;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.12);
  cursor: pointer;
}

.terminal-pause-resume {
  color: var(--qt-accent);
}

.paste-preview {
  margin: 0;
  max-height: 120px;
  overflow: auto;
  padding: 8px;
  border: 1px solid var(--qt-border);
  border-radius: 6px;
  background: var(--qt-subtle);
  white-space: pre-wrap;
  word-break: break-word;
  font-family: var(--qt-font-mono, ui-monospace, SFMono-Regular, Menlo, monospace);
  font-size: 11px;
  line-height: 1.5;
}

.translate-text {
  margin: 0;
  max-height: 220px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: var(--qt-font-mono, ui-monospace, SFMono-Regular, Menlo, monospace);
  font-size: 12px;
  line-height: 1.5;
  color: var(--qt-text);
}
</style>
