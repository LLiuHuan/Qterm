import type { PortForwardConfig } from '../../services/forwards'
import type { OtpEntry } from '../../services/otp'
import { isTranslateProvider, type TranslateProvider } from '../../services/translate'
import { TERMINAL_THEMES, type TerminalThemeId } from '../sessions/terminalThemes'
import { getPreference, PREFERENCE_KEYS, readCachedPreference, setPreference } from '../../services/preferences'
import { isSupportedLocale, type AppLocale } from '@/locales'

export type ThemeMode = 'light' | 'dark' | 'system'
export type TerminalCursorStyle = 'block' | 'underline' | 'bar'
export type TerminalRightClickBehavior = 'none' | 'paste' | 'menu'
export type TerminalDisconnectBehavior = 'reconnect' | 'close'

export type TabDoubleClickBehavior = 'duplicate' | 'rename'

export type TerminalRenderEngine = 'webgl' | 'dom'

export type TerminalGutterMode = 'off' | 'lines'

export type TerminalGutterTimestampFormat = string
export type TerminalThemeBackgroundScope = 'terminal' | 'shell' | 'derived'

export type AppLocaleSetting = AppLocale

export type SftpDirectoryFollowMode = 'session' | 'remote' | 'disabled'

export type SftpDragFolderUploadMode = 'normal' | 'archive'

export type HostColorStyle = 'block' | 'background'

export const HOST_COLOR_STYLES: HostColorStyle[] = ['block', 'background']

export type AssetLibraryLayout = 'sidebar' | 'workspace'

export interface KeywordHighlightRule {
  enabled: boolean
  pattern: string
  note: string
  color: string
}

export interface QtermSettings {
  locale?: AppLocaleSetting

  assetLibraryLayout?: AssetLibraryLayout

  hostColorStyle?: HostColorStyle

  recordAllSessions?: boolean

  sessionHistoryLimit?: number

  recordingDirectory?: string

  sftpDirectoryFollowMode?: SftpDirectoryFollowMode

  sftpDragFolderUploadMode?: SftpDragFolderUploadMode
  sftpAutoDirectoryFollow?: boolean
  sftpShowHiddenFiles?: boolean

  x11Display?: string
  x11XauthPath?: string
  x11AuthorityFile?: string
  sftpMaxTransfers?: number
  sftpLocalDirectory?: string
  sftpEditorPath?: string

  windowRememberSize?: boolean
  windowRememberPosition?: boolean

  closeToTray?: boolean

  restoreOpenSessions?: boolean

  quitConfirm?: boolean

  settingsInWindow?: boolean
  themeMode: ThemeMode

  minimalAppearance: boolean

  uiScale: number

  uiFont?: string
  showHostAddress: boolean
  lightBackgroundEnabled: boolean
  lightBackgroundOpacity: number
  lightBackgroundImage: string
  lightBackgroundColor: string
  darkBackgroundEnabled: boolean
  darkBackgroundOpacity: number
  darkBackgroundImage: string
  darkBackgroundColor: string
  terminalFontSize: number

  terminalLineHeight: number

  terminalFontFamily: string
  terminalLightTheme?: TerminalThemeId
  terminalDarkTheme?: TerminalThemeId
  terminalThemeBackgroundScope?: TerminalThemeBackgroundScope
  terminalAutoCopySelection?: boolean
  terminalCopyFeedback?: boolean
  terminalShowLatency?: boolean
  terminalShowMetrics?: boolean

  terminalGutterMode?: TerminalGutterMode

  translateEnabled?: boolean

  translateTarget?: string

  translateProvider?: TranslateProvider

  translateEndpoint?: string

  otpEntries?: OtpEntry[]

  portForwards?: PortForwardConfig[]

  portForwardGroups?: string[]

  terminalGutterShowTimestamps?: boolean

  terminalGutterTimestampFormat?: TerminalGutterTimestampFormat

  terminalShowLineNumbers?: boolean
  terminalBatchCommand?: boolean
  terminalConfirmClose?: boolean

  terminalScrollback?: number

  terminalConfirmMultiLinePaste?: boolean
  terminalCursorBlink?: boolean

  terminalRecordCommandHistory?: boolean
  terminalCompletionEnabled?: boolean
  terminalHistoryMinLength?: number
  terminalHistoryMaxLength?: number
  terminalWordSeparators?: string

  terminalMacOptionIsMeta?: boolean
  terminalCursorStyle?: TerminalCursorStyle
  terminalRightClickBehavior?: TerminalRightClickBehavior

  terminalModifierRightClickBehavior?: TerminalRightClickBehavior
  terminalDisconnectBehavior?: TerminalDisconnectBehavior
  terminalRenderEngine?: TerminalRenderEngine

  showTabIndex?: boolean

  tabDoubleClickBehavior?: TabDoubleClickBehavior

  autoReconnect?: boolean

  localShellPath?: string

  localShellWorkingDirectory?: string
  keywordHighlightEnabled?: boolean
  keywordHighlightRules?: KeywordHighlightRule[]
  layoutSidebarWidth?: number
  layoutToolPanelWidth?: number

  rdpShowResolution?: boolean

  rdpShowFps?: boolean

  rdpFirstFrameTimeoutSeconds?: number

  rdpHostProbeTimeoutSeconds?: number

  rdpFitGraceSeconds?: number
}

export const SETTINGS_STORAGE_KEY = 'qterm.settings.v1'

export const TERMINAL_SCROLLBACK_MIN = 500
export const TERMINAL_SCROLLBACK_MAX = 100_000

export const TERMINAL_FONT_SIZE_MIN = 10
export const TERMINAL_FONT_SIZE_MAX = 20
export const TERMINAL_LINE_HEIGHT_MIN = 1
export const TERMINAL_LINE_HEIGHT_MAX = 2
export const BACKGROUND_OPACITY_MIN = 0
export const BACKGROUND_OPACITY_MAX = 100
export const SIDEBAR_WIDTH_MIN = 180
export const SIDEBAR_WIDTH_MAX = 420
export const TOOL_PANEL_WIDTH_MIN = 280
export const TOOL_PANEL_WIDTH_MAX = 720

export const UI_SCALE_OPTIONS = [0.8, 0.9, 1, 1.1, 1.2] as const
export const SESSION_HISTORY_LIMIT_MIN = 20
export const SESSION_HISTORY_LIMIT_MAX = 10_000
export const SFTP_MAX_TRANSFERS_MIN = 1
export const SFTP_MAX_TRANSFERS_MAX = 16
export const TERMINAL_HISTORY_LENGTH_MIN = 0
export const TERMINAL_HISTORY_LENGTH_MAX = 4096

export const THEME_MODES: ThemeMode[] = ['light', 'dark', 'system']

export const TAB_DOUBLE_CLICK_BEHAVIORS: TabDoubleClickBehavior[] = ['rename', 'duplicate']

export const DEFAULT_KEYWORD_HIGHLIGHT_RULES: KeywordHighlightRule[] = [
  { enabled: true, pattern: 'ERROR', note: '错误警告', color: '#ff0000' },
]

export const DEFAULT_SETTINGS: QtermSettings = {
  locale: 'zh-CN',

  assetLibraryLayout: 'sidebar',
  hostColorStyle: 'block',
  recordAllSessions: false,
  sessionHistoryLimit: 200,
  recordingDirectory: '',

  sftpDirectoryFollowMode: 'remote',

  sftpDragFolderUploadMode: 'normal',
  sftpAutoDirectoryFollow: true,
  sftpShowHiddenFiles: false,
  x11Display: '',
  x11XauthPath: '',
  x11AuthorityFile: '',
  sftpMaxTransfers: 3,
  sftpLocalDirectory: '',
  sftpEditorPath: '',
  windowRememberSize: true,
  windowRememberPosition: true,
  closeToTray: true,
  restoreOpenSessions: true,
  quitConfirm: true,
  settingsInWindow: false,
  themeMode: 'system',
  minimalAppearance: false,
  uiScale: 1,
  uiFont: '',
  showHostAddress: true,
  lightBackgroundEnabled: false,
  lightBackgroundOpacity: 100,
  lightBackgroundImage: '',
  lightBackgroundColor: '#f7f7f8',
  darkBackgroundEnabled: false,
  darkBackgroundOpacity: 100,
  darkBackgroundImage: '',
  darkBackgroundColor: '#111113',
  terminalFontSize: 13,
  terminalLineHeight: 1.25,
  terminalFontFamily: '',
  terminalLightTheme: 'default-light',
  terminalDarkTheme: 'default-dark',
  terminalThemeBackgroundScope: 'terminal',
  terminalAutoCopySelection: false,
  terminalCopyFeedback: true,
  terminalShowLatency: true,
  terminalShowMetrics: false,
  terminalGutterMode: 'off',
  portForwards: [],
  portForwardGroups: [],
  otpEntries: [],
  translateEnabled: true,
  translateTarget: 'zh-CN',
  translateProvider: 'google',
  translateEndpoint: '',
  terminalGutterShowTimestamps: true,
  terminalGutterTimestampFormat: 'HH:mm:ss',
  terminalBatchCommand: true,
  terminalConfirmClose: true,
  terminalScrollback: 5000,
  terminalConfirmMultiLinePaste: true,
  terminalCursorBlink: false,
  terminalRecordCommandHistory: true,
  terminalCompletionEnabled: true,
  terminalHistoryMinLength: 1,
  terminalHistoryMaxLength: 4096,
  terminalWordSeparators: ' ()[]{}\'"',
  terminalMacOptionIsMeta: false,
  terminalCursorStyle: 'block',
  terminalRightClickBehavior: 'menu',
  terminalModifierRightClickBehavior: 'none',
  terminalDisconnectBehavior: 'close',
  terminalRenderEngine: 'webgl',
  showTabIndex: false,
  tabDoubleClickBehavior: 'rename',
  autoReconnect: false,
  localShellPath: '',
  localShellWorkingDirectory: '',
  keywordHighlightEnabled: false,
  keywordHighlightRules: DEFAULT_KEYWORD_HIGHLIGHT_RULES,
  layoutSidebarWidth: 224,
  layoutToolPanelWidth: 360,

  rdpShowResolution: false,
  rdpShowFps: false,

  rdpFirstFrameTimeoutSeconds: 25,
  rdpHostProbeTimeoutSeconds: 3,

  rdpFitGraceSeconds: 6,
}

export const RDP_FIRST_FRAME_TIMEOUT_MIN = 5
export const RDP_FIRST_FRAME_TIMEOUT_MAX = 300
export const RDP_HOST_PROBE_TIMEOUT_MIN = 1
export const RDP_HOST_PROBE_TIMEOUT_MAX = 30

export const RDP_FIT_GRACE_MIN = 0
export const RDP_FIT_GRACE_MAX = 60

function defaultStorage(): Storage | null {
  try {
    return globalThis.localStorage ?? null
  } catch {
    return null
  }
}

function normalizePortForwards(value: unknown): PortForwardConfig[] {
  if (!Array.isArray(value)) return []
  const result: PortForwardConfig[] = []
  const seen = new Set<string>()
  for (const entry of value) {
    if (!entry || typeof entry !== 'object') continue
    const candidate = entry as Record<string, unknown>
    const id = typeof candidate.id === 'string' ? candidate.id.trim() : ''
    const hostId = typeof candidate.hostId === 'string' ? candidate.hostId.trim() : ''
    const kind = candidate.kind
    if (!id || !hostId || seen.has(id)) continue
    if (kind !== 'local' && kind !== 'remote' && kind !== 'dynamic') continue
    const listenPort = Number(candidate.listenPort)
    const targetPort = Number(candidate.targetPort)
    if (!Number.isInteger(listenPort) || listenPort < 0 || listenPort > 65535) continue
    if (!Number.isInteger(targetPort) || targetPort < 0 || targetPort > 65535) continue
    seen.add(id)
    result.push({
      id,
      name: typeof candidate.name === 'string' ? candidate.name : '',

      group:
        typeof candidate.group === 'string'
          ? candidate.group
          : typeof candidate.category === 'string'
            ? candidate.category
            : '',
      kind,
      hostId,
      listenHost: typeof candidate.listenHost === 'string' ? candidate.listenHost : '127.0.0.1',
      listenPort,
      targetHost: typeof candidate.targetHost === 'string' ? candidate.targetHost : '127.0.0.1',
      targetPort,

      ...(kind === 'dynamic' && (candidate.proxy === 'socks5' || candidate.proxy === 'http')
        ? { proxy: candidate.proxy }
        : {}),
      autoOpen: candidate.autoOpen === true,
    })
  }
  return result
}

function normalizeForwardGroups(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  const result: string[] = []
  const seen = new Set<string>()
  for (const entry of value) {
    if (typeof entry !== 'string') continue
    const name = entry.trim()
    if (!name || seen.has(name)) continue
    seen.add(name)
    result.push(name)
    if (result.length >= 200) break
  }
  return result
}

function normalizeOtpEntries(value: unknown): OtpEntry[] {
  if (!Array.isArray(value)) return []
  const result: OtpEntry[] = []
  const seen = new Set<string>()
  for (const entry of value) {
    if (!entry || typeof entry !== 'object') continue
    const candidate = entry as Record<string, unknown>
    const id = typeof candidate.id === 'string' ? candidate.id.trim() : ''
    if (!id || seen.has(id)) continue
    const type = candidate.type === 'hotp' ? 'hotp' : 'totp'
    const algorithm =
      candidate.algorithm === 'SHA256' || candidate.algorithm === 'SHA512' ? candidate.algorithm : 'SHA1'
    const digits = Number(candidate.digits)
    const period = Number(candidate.period)
    const counter = Number(candidate.counter)

    if (!Number.isInteger(digits) || !Number.isInteger(period) || !Number.isInteger(counter)) continue
    const boundedDigits = Math.min(10, Math.max(4, digits))
    const boundedPeriod = Math.min(300, Math.max(10, period))
    const boundedCounter = Math.min(999999999, Math.max(0, counter))
    seen.add(id)
    result.push({
      id,
      name: typeof candidate.name === 'string' ? candidate.name : '',

      username:
        typeof candidate.username === 'string' && candidate.username.trim()
          ? candidate.username.trim().slice(0, 128)
          : undefined,
      hostId: typeof candidate.hostId === 'string' && candidate.hostId ? candidate.hostId : undefined,
      type,
      algorithm,
      digits: boundedDigits,
      period: boundedPeriod,
      counter: boundedCounter,
    })
  }
  return result
}

export function normalizeSettings(value: unknown): QtermSettings {
  if (!value || typeof value !== 'object') return { ...DEFAULT_SETTINGS }
  const candidate = value as Partial<Record<keyof QtermSettings, unknown>>

  const locale: AppLocaleSetting = isSupportedLocale(candidate.locale) ? candidate.locale : DEFAULT_SETTINGS.locale!

  const assetLibraryLayouts: AssetLibraryLayout[] = ['sidebar', 'workspace']
  const assetLibraryLayout = assetLibraryLayouts.includes(candidate.assetLibraryLayout as AssetLibraryLayout)
    ? (candidate.assetLibraryLayout as AssetLibraryLayout)
    : DEFAULT_SETTINGS.assetLibraryLayout

  const hostColorStyle = HOST_COLOR_STYLES.includes(candidate.hostColorStyle as HostColorStyle)
    ? (candidate.hostColorStyle as HostColorStyle)
    : DEFAULT_SETTINGS.hostColorStyle
  const normalizeBoundedInteger = (raw: unknown, fallback: number, min: number, max: number): number => {
    const parsed = Number(raw)
    return Number.isFinite(parsed) ? Math.min(max, Math.max(min, Math.round(parsed))) : fallback
  }
  const recordAllSessions =
    typeof candidate.recordAllSessions === 'boolean' ? candidate.recordAllSessions : DEFAULT_SETTINGS.recordAllSessions
  const sessionHistoryLimit = normalizeBoundedInteger(
    candidate.sessionHistoryLimit,
    DEFAULT_SETTINGS.sessionHistoryLimit ?? 200,
    SESSION_HISTORY_LIMIT_MIN,
    SESSION_HISTORY_LIMIT_MAX,
  )
  const sftpMaxTransfers = normalizeBoundedInteger(
    candidate.sftpMaxTransfers,
    DEFAULT_SETTINGS.sftpMaxTransfers ?? 3,
    SFTP_MAX_TRANSFERS_MIN,
    SFTP_MAX_TRANSFERS_MAX,
  )
  const directoryFollowModes: SftpDirectoryFollowMode[] = ['session', 'remote', 'disabled']
  const sftpDirectoryFollowMode = directoryFollowModes.includes(
    candidate.sftpDirectoryFollowMode as SftpDirectoryFollowMode,
  )
    ? (candidate.sftpDirectoryFollowMode as SftpDirectoryFollowMode)
    : DEFAULT_SETTINGS.sftpDirectoryFollowMode
  const dragFolderUploadModes: SftpDragFolderUploadMode[] = ['normal', 'archive']
  const sftpDragFolderUploadMode = dragFolderUploadModes.includes(
    candidate.sftpDragFolderUploadMode as SftpDragFolderUploadMode,
  )
    ? (candidate.sftpDragFolderUploadMode as SftpDragFolderUploadMode)
    : DEFAULT_SETTINGS.sftpDragFolderUploadMode
  const themeMode = THEME_MODES.includes(candidate.themeMode as ThemeMode)
    ? (candidate.themeMode as ThemeMode)
    : DEFAULT_SETTINGS.themeMode
  const minimalAppearance =
    typeof candidate.minimalAppearance === 'boolean' ? candidate.minimalAppearance : DEFAULT_SETTINGS.minimalAppearance

  const uiScaleCandidate = Number(candidate.uiScale)
  const uiScale =
    UI_SCALE_OPTIONS.find((option) => Math.abs(option - uiScaleCandidate) < 1e-6) ?? DEFAULT_SETTINGS.uiScale

  const uiFont =
    typeof candidate.uiFont === 'string' && candidate.uiFont.trim() !== '' ? candidate.uiFont.trim().slice(0, 128) : ''
  const size = Number(candidate.terminalFontSize)
  const terminalFontSize = Number.isFinite(size)
    ? Math.min(TERMINAL_FONT_SIZE_MAX, Math.max(TERMINAL_FONT_SIZE_MIN, Math.round(size)))
    : DEFAULT_SETTINGS.terminalFontSize

  const lineHeightRaw = Number(candidate.terminalLineHeight)
  const terminalLineHeight = Number.isFinite(lineHeightRaw)
    ? Math.round(Math.min(TERMINAL_LINE_HEIGHT_MAX, Math.max(TERMINAL_LINE_HEIGHT_MIN, lineHeightRaw)) * 100) / 100
    : DEFAULT_SETTINGS.terminalLineHeight
  const terminalFontFamily =
    typeof candidate.terminalFontFamily === 'string' ? candidate.terminalFontFamily.trim().slice(0, 128) : ''
  const themeIds = Object.keys(TERMINAL_THEMES) as TerminalThemeId[]
  const isThemeId = (value: unknown): value is TerminalThemeId =>
    typeof value === 'string' && (themeIds.includes(value) || value.startsWith('custom-'))
  const terminalLightTheme = isThemeId(candidate.terminalLightTheme)
    ? (candidate.terminalLightTheme as TerminalThemeId)
    : DEFAULT_SETTINGS.terminalLightTheme
  const terminalDarkTheme = isThemeId(candidate.terminalDarkTheme)
    ? (candidate.terminalDarkTheme as TerminalThemeId)
    : DEFAULT_SETTINGS.terminalDarkTheme
  const cursorStyles: TerminalCursorStyle[] = ['block', 'underline', 'bar']
  const rightClickBehaviors: TerminalRightClickBehavior[] = ['none', 'paste', 'menu']
  const renderEngines: TerminalRenderEngine[] = ['webgl', 'dom']

  const terminalGutterMode: TerminalGutterMode =
    candidate.terminalGutterMode === 'lines' || candidate.terminalGutterMode === 'commands'
      ? 'lines'
      : candidate.terminalGutterMode === 'off'
        ? 'off'
        : typeof candidate.terminalShowLineNumbers === 'boolean'
          ? candidate.terminalShowLineNumbers
            ? 'lines'
            : 'off'
          : (DEFAULT_SETTINGS.terminalGutterMode ?? 'off')

  const legacyTimestampFormats: Record<string, string> = {
    time: 'HH:mm:ss',
    timeShort: 'HH:mm',
    datetime: 'MM-DD HH:mm:ss',
    iso: 'YYYY-MM-DD HH:mm:ss',
  }
  const rawTimestampFormat =
    typeof candidate.terminalGutterTimestampFormat === 'string' ? candidate.terminalGutterTimestampFormat.trim() : ''
  const terminalGutterTimestampFormat =
    legacyTimestampFormats[rawTimestampFormat] ??
    (rawTimestampFormat.length > 0 && rawTimestampFormat.length <= 40
      ? rawTimestampFormat
      : DEFAULT_SETTINGS.terminalGutterTimestampFormat)
  const backgroundScopes: TerminalThemeBackgroundScope[] = ['terminal', 'shell', 'derived']
  const terminalThemeBackgroundScope = backgroundScopes.includes(
    candidate.terminalThemeBackgroundScope as TerminalThemeBackgroundScope,
  )
    ? (candidate.terminalThemeBackgroundScope as TerminalThemeBackgroundScope)
    : DEFAULT_SETTINGS.terminalThemeBackgroundScope
  const showHostAddress =
    typeof candidate.showHostAddress === 'boolean' ? candidate.showHostAddress : DEFAULT_SETTINGS.showHostAddress
  const normalizeOpacity = (value: unknown, fallback: number): number => {
    const opacity = Number(value)
    return Number.isFinite(opacity)
      ? Math.min(BACKGROUND_OPACITY_MAX, Math.max(BACKGROUND_OPACITY_MIN, Math.round(opacity)))
      : fallback
  }
  const normalizeImage = (value: unknown): string => (typeof value === 'string' ? value.trim() : '')
  const normalizeColor = (value: unknown, fallback: string): string =>
    typeof value === 'string' && /^#[0-9a-f]{6}$/i.test(value.trim()) ? value.trim() : fallback
  const normalizeDimension = (value: unknown, fallback: number, min: number, max: number): number => {
    const dimension = Number(value)
    return Number.isFinite(dimension) ? Math.min(max, Math.max(min, Math.round(dimension))) : fallback
  }
  const normalizeKeywordHighlightRules = (value: unknown): KeywordHighlightRule[] => {
    if (!Array.isArray(value)) return DEFAULT_KEYWORD_HIGHLIGHT_RULES.map((rule) => ({ ...rule }))
    return value.slice(0, 100).flatMap((item): KeywordHighlightRule[] => {
      if (!item || typeof item !== 'object') return []
      const candidateRule = item as Record<string, unknown>
      const pattern = typeof candidateRule.pattern === 'string' ? candidateRule.pattern.slice(0, 512) : ''
      const note = typeof candidateRule.note === 'string' ? candidateRule.note.trim().slice(0, 256) : ''
      const color = normalizeColor(candidateRule.color, '#ff0000')
      return [
        {
          enabled: typeof candidateRule.enabled === 'boolean' ? candidateRule.enabled : true,
          pattern,
          note,
          color,
        },
      ]
    })
  }
  const terminalInteraction = [
    'terminalAutoCopySelection',
    'terminalCopyFeedback',
    'terminalShowLatency',
    'terminalShowMetrics',
    'terminalGutterMode',
    'translateEnabled',
    'translateTarget',
    'translateProvider',
    'translateEndpoint',
    'otpEntries',
    'portForwards',
    'portForwardGroups',
    'terminalGutterShowTimestamps',
    'terminalGutterTimestampFormat',
    'terminalShowLineNumbers',
    'terminalBatchCommand',
    'terminalConfirmClose',
    'terminalScrollback',
    'terminalConfirmMultiLinePaste',
    'terminalCursorBlink',
    'terminalRecordCommandHistory',
    'terminalCompletionEnabled',
    'terminalHistoryMinLength',
    'terminalHistoryMaxLength',
    'terminalWordSeparators',
    'terminalMacOptionIsMeta',
    'terminalCursorStyle',
    'terminalRightClickBehavior',
    'terminalModifierRightClickBehavior',
    'terminalDisconnectBehavior',
    'terminalRenderEngine',
    'showTabIndex',
    'tabDoubleClickBehavior',
    'autoReconnect',
  ].some((key) => key in candidate)
    ? {
        terminalAutoCopySelection:
          typeof candidate.terminalAutoCopySelection === 'boolean'
            ? candidate.terminalAutoCopySelection
            : DEFAULT_SETTINGS.terminalAutoCopySelection,
        terminalCopyFeedback:
          typeof candidate.terminalCopyFeedback === 'boolean'
            ? candidate.terminalCopyFeedback
            : DEFAULT_SETTINGS.terminalCopyFeedback,
        terminalShowLatency:
          typeof candidate.terminalShowLatency === 'boolean'
            ? candidate.terminalShowLatency
            : DEFAULT_SETTINGS.terminalShowLatency,
        terminalShowMetrics:
          typeof candidate.terminalShowMetrics === 'boolean'
            ? candidate.terminalShowMetrics
            : DEFAULT_SETTINGS.terminalShowMetrics,
        terminalGutterMode,
        translateEnabled:
          typeof candidate.translateEnabled === 'boolean'
            ? candidate.translateEnabled
            : DEFAULT_SETTINGS.translateEnabled,
        translateTarget:
          typeof candidate.translateTarget === 'string' && candidate.translateTarget.trim()
            ? candidate.translateTarget.trim()
            : DEFAULT_SETTINGS.translateTarget,
        translateProvider: isTranslateProvider(candidate.translateProvider)
          ? (candidate.translateProvider as TranslateProvider)
          : DEFAULT_SETTINGS.translateProvider,
        translateEndpoint: typeof candidate.translateEndpoint === 'string' ? candidate.translateEndpoint.trim() : '',
        otpEntries: normalizeOtpEntries(candidate.otpEntries),
        portForwards: normalizePortForwards(candidate.portForwards),
        portForwardGroups: normalizeForwardGroups(candidate.portForwardGroups),
        terminalGutterShowTimestamps:
          typeof candidate.terminalGutterShowTimestamps === 'boolean'
            ? candidate.terminalGutterShowTimestamps
            : DEFAULT_SETTINGS.terminalGutterShowTimestamps,
        terminalGutterTimestampFormat,
        terminalBatchCommand:
          typeof candidate.terminalBatchCommand === 'boolean'
            ? candidate.terminalBatchCommand
            : DEFAULT_SETTINGS.terminalBatchCommand,
        terminalConfirmClose:
          typeof candidate.terminalConfirmClose === 'boolean'
            ? candidate.terminalConfirmClose
            : DEFAULT_SETTINGS.terminalConfirmClose,

        terminalScrollback:
          typeof candidate.terminalScrollback === 'number' && Number.isFinite(candidate.terminalScrollback)
            ? Math.min(
                TERMINAL_SCROLLBACK_MAX,
                Math.max(TERMINAL_SCROLLBACK_MIN, Math.round(candidate.terminalScrollback)),
              )
            : DEFAULT_SETTINGS.terminalScrollback,
        terminalConfirmMultiLinePaste:
          typeof candidate.terminalConfirmMultiLinePaste === 'boolean'
            ? candidate.terminalConfirmMultiLinePaste
            : DEFAULT_SETTINGS.terminalConfirmMultiLinePaste,
        terminalCursorBlink:
          typeof candidate.terminalCursorBlink === 'boolean'
            ? candidate.terminalCursorBlink
            : DEFAULT_SETTINGS.terminalCursorBlink,
        terminalRecordCommandHistory:
          typeof candidate.terminalRecordCommandHistory === 'boolean'
            ? candidate.terminalRecordCommandHistory
            : DEFAULT_SETTINGS.terminalRecordCommandHistory,
        terminalCompletionEnabled:
          typeof candidate.terminalCompletionEnabled === 'boolean'
            ? candidate.terminalCompletionEnabled
            : DEFAULT_SETTINGS.terminalCompletionEnabled,
        terminalHistoryMinLength: normalizeBoundedInteger(
          candidate.terminalHistoryMinLength,
          DEFAULT_SETTINGS.terminalHistoryMinLength ?? 1,
          TERMINAL_HISTORY_LENGTH_MIN,
          TERMINAL_HISTORY_LENGTH_MAX,
        ),
        terminalHistoryMaxLength: normalizeBoundedInteger(
          candidate.terminalHistoryMaxLength,
          DEFAULT_SETTINGS.terminalHistoryMaxLength ?? 4096,
          TERMINAL_HISTORY_LENGTH_MIN,
          TERMINAL_HISTORY_LENGTH_MAX,
        ),
        terminalWordSeparators:
          typeof candidate.terminalWordSeparators === 'string'
            ? Array.from(candidate.terminalWordSeparators).slice(0, 64).join('')
            : DEFAULT_SETTINGS.terminalWordSeparators,

        terminalMacOptionIsMeta:
          typeof candidate.terminalMacOptionIsMeta === 'boolean'
            ? candidate.terminalMacOptionIsMeta
            : DEFAULT_SETTINGS.terminalMacOptionIsMeta,
        terminalCursorStyle: cursorStyles.includes(candidate.terminalCursorStyle as TerminalCursorStyle)
          ? (candidate.terminalCursorStyle as TerminalCursorStyle)
          : DEFAULT_SETTINGS.terminalCursorStyle,
        terminalRightClickBehavior: rightClickBehaviors.includes(
          candidate.terminalRightClickBehavior as TerminalRightClickBehavior,
        )
          ? (candidate.terminalRightClickBehavior as TerminalRightClickBehavior)
          : DEFAULT_SETTINGS.terminalRightClickBehavior,
        terminalModifierRightClickBehavior: rightClickBehaviors.includes(
          candidate.terminalModifierRightClickBehavior as TerminalRightClickBehavior,
        )
          ? (candidate.terminalModifierRightClickBehavior as TerminalRightClickBehavior)
          : DEFAULT_SETTINGS.terminalModifierRightClickBehavior,
        terminalDisconnectBehavior:
          candidate.terminalDisconnectBehavior === 'reconnect' || candidate.terminalDisconnectBehavior === 'close'
            ? candidate.terminalDisconnectBehavior
            : DEFAULT_SETTINGS.terminalDisconnectBehavior,
        terminalRenderEngine: renderEngines.includes(candidate.terminalRenderEngine as TerminalRenderEngine)
          ? (candidate.terminalRenderEngine as TerminalRenderEngine)
          : DEFAULT_SETTINGS.terminalRenderEngine,

        showTabIndex:
          typeof candidate.showTabIndex === 'boolean' ? candidate.showTabIndex : DEFAULT_SETTINGS.showTabIndex,

        tabDoubleClickBehavior: TAB_DOUBLE_CLICK_BEHAVIORS.includes(
          candidate.tabDoubleClickBehavior as TabDoubleClickBehavior,
        )
          ? (candidate.tabDoubleClickBehavior as TabDoubleClickBehavior)
          : DEFAULT_SETTINGS.tabDoubleClickBehavior,

        autoReconnect:
          typeof candidate.autoReconnect === 'boolean' ? candidate.autoReconnect : DEFAULT_SETTINGS.autoReconnect,
      }
    : {}

  const terminalLocalShell = ['localShellPath', 'localShellWorkingDirectory'].some((key) => key in candidate)
    ? {
        localShellPath:
          typeof candidate.localShellPath === 'string'
            ? candidate.localShellPath.trim()
            : (DEFAULT_SETTINGS.localShellPath ?? ''),
        localShellWorkingDirectory:
          typeof candidate.localShellWorkingDirectory === 'string'
            ? candidate.localShellWorkingDirectory.trim()
            : (DEFAULT_SETTINGS.localShellWorkingDirectory ?? ''),
      }
    : {}
  const layoutDimensions = ['layoutSidebarWidth', 'layoutToolPanelWidth'].some((key) => key in candidate)
    ? {
        layoutSidebarWidth: normalizeDimension(
          candidate.layoutSidebarWidth,
          DEFAULT_SETTINGS.layoutSidebarWidth ?? 224,
          SIDEBAR_WIDTH_MIN,
          SIDEBAR_WIDTH_MAX,
        ),
        layoutToolPanelWidth: normalizeDimension(
          candidate.layoutToolPanelWidth,
          DEFAULT_SETTINGS.layoutToolPanelWidth ?? 360,
          TOOL_PANEL_WIDTH_MIN,
          TOOL_PANEL_WIDTH_MAX,
        ),
      }
    : {}

  const normalizeSeconds = (value: unknown, fallback: number, min: number, max: number): number => {
    if (typeof value !== 'number' || !Number.isFinite(value)) return fallback
    return Math.min(max, Math.max(min, Math.round(value)))
  }
  const rdpTimeouts = ['rdpFirstFrameTimeoutSeconds', 'rdpHostProbeTimeoutSeconds', 'rdpFitGraceSeconds'].some(
    (key) => key in candidate,
  )
    ? {
        rdpFirstFrameTimeoutSeconds: normalizeSeconds(
          candidate.rdpFirstFrameTimeoutSeconds,
          DEFAULT_SETTINGS.rdpFirstFrameTimeoutSeconds ?? 25,
          RDP_FIRST_FRAME_TIMEOUT_MIN,
          RDP_FIRST_FRAME_TIMEOUT_MAX,
        ),
        rdpHostProbeTimeoutSeconds: normalizeSeconds(
          candidate.rdpHostProbeTimeoutSeconds,
          DEFAULT_SETTINGS.rdpHostProbeTimeoutSeconds ?? 3,
          RDP_HOST_PROBE_TIMEOUT_MIN,
          RDP_HOST_PROBE_TIMEOUT_MAX,
        ),
        rdpFitGraceSeconds: normalizeSeconds(
          candidate.rdpFitGraceSeconds,
          DEFAULT_SETTINGS.rdpFitGraceSeconds ?? 6,
          RDP_FIT_GRACE_MIN,
          RDP_FIT_GRACE_MAX,
        ),
      }
    : {}

  const rdpDisplay = ['rdpShowResolution', 'rdpShowFps'].some((key) => key in candidate)
    ? {
        rdpShowResolution:
          typeof candidate.rdpShowResolution === 'boolean'
            ? candidate.rdpShowResolution
            : DEFAULT_SETTINGS.rdpShowResolution,
        rdpShowFps: typeof candidate.rdpShowFps === 'boolean' ? candidate.rdpShowFps : DEFAULT_SETTINGS.rdpShowFps,
      }
    : {}
  if (
    terminalInteraction.terminalHistoryMinLength !== undefined &&
    terminalInteraction.terminalHistoryMaxLength !== undefined
  ) {
    terminalInteraction.terminalHistoryMaxLength = Math.max(
      terminalInteraction.terminalHistoryMinLength,
      terminalInteraction.terminalHistoryMaxLength,
    )
  }
  return {
    locale,
    assetLibraryLayout,
    hostColorStyle,
    recordAllSessions,
    sessionHistoryLimit,
    recordingDirectory: typeof candidate.recordingDirectory === 'string' ? candidate.recordingDirectory.trim() : '',
    sftpDirectoryFollowMode,
    sftpDragFolderUploadMode,
    sftpAutoDirectoryFollow:
      typeof candidate.sftpAutoDirectoryFollow === 'boolean'
        ? candidate.sftpAutoDirectoryFollow
        : DEFAULT_SETTINGS.sftpAutoDirectoryFollow,
    sftpShowHiddenFiles:
      typeof candidate.sftpShowHiddenFiles === 'boolean'
        ? candidate.sftpShowHiddenFiles
        : DEFAULT_SETTINGS.sftpShowHiddenFiles,
    x11Display: typeof candidate.x11Display === 'string' ? candidate.x11Display : DEFAULT_SETTINGS.x11Display,
    x11XauthPath: typeof candidate.x11XauthPath === 'string' ? candidate.x11XauthPath : DEFAULT_SETTINGS.x11XauthPath,
    x11AuthorityFile:
      typeof candidate.x11AuthorityFile === 'string' ? candidate.x11AuthorityFile : DEFAULT_SETTINGS.x11AuthorityFile,
    sftpMaxTransfers,
    sftpLocalDirectory: typeof candidate.sftpLocalDirectory === 'string' ? candidate.sftpLocalDirectory.trim() : '',
    sftpEditorPath: typeof candidate.sftpEditorPath === 'string' ? candidate.sftpEditorPath.trim() : '',
    windowRememberSize:
      typeof candidate.windowRememberSize === 'boolean'
        ? candidate.windowRememberSize
        : DEFAULT_SETTINGS.windowRememberSize,
    windowRememberPosition:
      typeof candidate.windowRememberPosition === 'boolean'
        ? candidate.windowRememberPosition
        : DEFAULT_SETTINGS.windowRememberPosition,
    closeToTray: typeof candidate.closeToTray === 'boolean' ? candidate.closeToTray : DEFAULT_SETTINGS.closeToTray,
    settingsInWindow:
      typeof candidate.settingsInWindow === 'boolean' ? candidate.settingsInWindow : DEFAULT_SETTINGS.settingsInWindow,
    restoreOpenSessions:
      typeof candidate.restoreOpenSessions === 'boolean'
        ? candidate.restoreOpenSessions
        : DEFAULT_SETTINGS.restoreOpenSessions,
    quitConfirm: typeof candidate.quitConfirm === 'boolean' ? candidate.quitConfirm : DEFAULT_SETTINGS.quitConfirm,
    themeMode,
    minimalAppearance,
    uiScale,
    uiFont,
    showHostAddress,
    lightBackgroundEnabled:
      typeof candidate.lightBackgroundEnabled === 'boolean'
        ? candidate.lightBackgroundEnabled
        : DEFAULT_SETTINGS.lightBackgroundEnabled,
    lightBackgroundOpacity: normalizeOpacity(candidate.lightBackgroundOpacity, DEFAULT_SETTINGS.lightBackgroundOpacity),
    lightBackgroundImage: normalizeImage(candidate.lightBackgroundImage),
    lightBackgroundColor: normalizeColor(candidate.lightBackgroundColor, DEFAULT_SETTINGS.lightBackgroundColor),
    darkBackgroundEnabled:
      typeof candidate.darkBackgroundEnabled === 'boolean'
        ? candidate.darkBackgroundEnabled
        : DEFAULT_SETTINGS.darkBackgroundEnabled,
    darkBackgroundOpacity: normalizeOpacity(candidate.darkBackgroundOpacity, DEFAULT_SETTINGS.darkBackgroundOpacity),
    darkBackgroundImage: normalizeImage(candidate.darkBackgroundImage),
    darkBackgroundColor: normalizeColor(candidate.darkBackgroundColor, DEFAULT_SETTINGS.darkBackgroundColor),
    terminalFontSize,
    terminalLineHeight,
    terminalFontFamily,
    terminalLightTheme,
    terminalDarkTheme,
    terminalThemeBackgroundScope,
    ...terminalInteraction,
    ...terminalLocalShell,
    keywordHighlightEnabled:
      typeof candidate.keywordHighlightEnabled === 'boolean'
        ? candidate.keywordHighlightEnabled
        : DEFAULT_SETTINGS.keywordHighlightEnabled,
    keywordHighlightRules: normalizeKeywordHighlightRules(candidate.keywordHighlightRules),
    ...layoutDimensions,
    ...rdpDisplay,
    ...rdpTimeouts,
  }
}

export function loadSettings(storage: Storage | null = defaultStorage()): QtermSettings {
  if (!storage) return { ...DEFAULT_SETTINGS }
  try {
    const raw = storage.getItem(SETTINGS_STORAGE_KEY)
    if (!raw) return { ...DEFAULT_SETTINGS }
    return normalizeSettings(JSON.parse(raw))
  } catch {
    return { ...DEFAULT_SETTINGS }
  }
}

export async function loadSettingsFromPreferences(storage: Storage | null = defaultStorage()): Promise<QtermSettings> {
  const raw = await getPreference(PREFERENCE_KEYS.settings, storage)
  if (!raw) return loadSettings(storage)
  try {
    return normalizeSettings(JSON.parse(raw))
  } catch {
    return loadSettings(storage)
  }
}

export function saveSettings(settings: QtermSettings, storage: Storage | null = defaultStorage()): void {
  const normalized = normalizeSettings(settings)
  const value = JSON.stringify(normalized)

  if (readCachedPreference(SETTINGS_STORAGE_KEY, storage) !== value)
    void setPreference(SETTINGS_STORAGE_KEY, value, storage).catch(() => undefined)
}
