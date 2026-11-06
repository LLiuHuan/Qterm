import { reactive } from 'vue'
import { getPreference, PREFERENCE_KEYS, readCachedPreference, setPreference } from '../../services/preferences'

export type ShortcutAction =
  | 'nextTab'
  | 'previousTab'
  | 'closeTab'
  | 'switchTab'
  | 'newWindow'
  | 'newLocalTab'
  | 'clearTerminal'
  | 'openTerminalSearch'
  | 'copyTerminalSelection'
  | 'pasteTerminal'
  | 'reconnectSession'
  | 'disconnectSession'
  | 'commandPalette'
  | 'togglePaneMaximize'
  | 'focusNextPane'
  | 'closePane'
  | 'splitRight'
  | 'splitBottom'
  | 'toggleSidebar'
  | 'remoteFileSave'

export type ShortcutBindings = Record<ShortcutAction, string>

export const UNBOUND_SHORTCUT = ''

export interface ShortcutActionItem {
  id: ShortcutAction
  label: string
  description: string
}

export interface ShortcutActionGroup {
  id: string
  label: string
  items: readonly ShortcutActionItem[]
}

export const SHORTCUTS_STORAGE_KEY = 'qterm.shortcuts.v1'

export const DEFAULT_SHORTCUTS: ShortcutBindings = {
  nextTab: 'Mod+ArrowRight',
  previousTab: 'Mod+ArrowLeft',
  closeTab: 'Mod+W',
  switchTab: 'Mod+TabNumber',
  newWindow: 'Mod+N',

  newLocalTab: 'Mod+Shift+T',
  clearTerminal: 'Mod+Shift+K',

  openTerminalSearch: 'Mod+F',

  copyTerminalSelection: 'Mod+Shift+C',
  pasteTerminal: 'Mod+Shift+V',
  reconnectSession: 'Mod+Shift+R',

  disconnectSession: 'Mod+Shift+X',

  commandPalette: 'Mod+K',
  togglePaneMaximize: 'Mod+Shift+M',
  focusNextPane: 'Mod+Alt+Arrows',
  closePane: 'Mod+Shift+W',
  splitRight: 'Mod+Shift+D',
  splitBottom: 'Mod+Shift+E',
  toggleSidebar: 'Mod+Shift+B',

  remoteFileSave: 'Mod+S',
}

export const SHORTCUT_TOKEN_KEYS: Readonly<Record<string, readonly string[]>> = {
  TabNumber: ['1', '2', '3', '4', '5', '6', '7', '8', '9'],
  Arrows: ['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'],
}

const SHORTCUT_TOKEN_LABELS: Readonly<Record<string, string>> = {
  TabNumber: '[1…9]',
  Arrows: '[←→↑↓]',
}

export const shortcutBindings = reactive<ShortcutBindings>({ ...DEFAULT_SHORTCUTS })

export function useShortcutBindings(): ShortcutBindings {
  return shortcutBindings
}

export const SHORTCUT_ACTION_GROUPS: readonly ShortcutActionGroup[] = [
  {
    id: 'tabs',
    label: '标签管理',
    items: [
      { id: 'nextTab', label: '下一个标签', description: '切换到当前标签右侧的标签。' },
      { id: 'previousTab', label: '上一个标签', description: '切换到当前标签左侧的标签。' },
      { id: 'closeTab', label: '关闭标签', description: '关闭当前终端标签，并遵循关闭确认设置。' },
      { id: 'switchTab', label: '切换到标签 [1...9]', description: '使用录制的修饰键加数字 1~9，定位到对应标签。' },
      {
        id: 'newLocalTab',
        label: '新建本地标签',
        description: '打开一个本机 Shell 终端标签，与工具条的「本地终端」同一条链路。',
      },
      { id: 'newWindow', label: '新建窗口', description: '打开一个新的 Qterm 窗口。' },
    ],
  },
  {
    id: 'terminal',
    label: '终端操作',
    items: [
      {
        id: 'clearTerminal',
        label: '清空终端',
        description: '清空当前终端的可视缓冲区，不向远端或本机执行 clear 命令。',
      },
      { id: 'openTerminalSearch', label: '打开搜索', description: '在终端右上角打开搜索浮层，仅在会话连接后可用。' },
      {
        id: 'copyTerminalSelection',
        label: '复制选中内容',
        description: '把当前激活窗格中选中的内容复制到系统剪贴板。',
      },
      { id: 'pasteTerminal', label: '粘贴到终端', description: '把系统剪贴板内容粘贴到当前激活的终端。' },
      { id: 'reconnectSession', label: '重新连接', description: '重新建立当前会话的连接。' },
      { id: 'disconnectSession', label: '断开连接', description: '断开当前会话的连接，标签保留。' },
    ],
  },
  {
    id: 'navigation',
    label: '导航与分屏',
    items: [
      {
        id: 'commandPalette',
        label: '打开命令面板',
        description: '搜索并跳转到会话、主机、设置页，或执行常用动作。',
      },
      {
        id: 'togglePaneMaximize',
        label: '最大化/还原窗格',
        description: '分屏时把当前窗格临时铺满舞台，再按一次或 Esc 还原。',
      },
      {
        id: 'focusNextPane',
        label: '在分屏之间移动焦点',
        description: '按方向键把焦点移到该方向最近的分屏窗格（仅分屏时可用）。',
      },
      {
        id: 'closePane',
        label: '关闭当前分屏窗格',
        description: '把当前窗格移出分屏布局，会话本身继续保留在标签栏。',
      },
      { id: 'splitRight', label: '左右分屏', description: '把当前会话与就近的另一个会话左右分屏。' },
      { id: 'splitBottom', label: '上下分屏', description: '把当前会话与就近的另一个会话上下分屏。' },
      { id: 'toggleSidebar', label: '显示/隐藏侧边栏', description: '展开或收起左侧的资产侧边栏。' },
    ],
  },
  {
    id: 'remote',
    label: '远端文件',
    items: [
      {
        id: 'remoteFileSave',
        label: '保存远端文件',
        description: '把当前打开的应用内编辑器内容写回远端文件，与应用内的保存按钮同一条链路。',
      },
    ],
  },
] as const

export const SHORTCUT_ACTIONS = SHORTCUT_ACTION_GROUPS.flatMap((group) => group.items)

function defaultStorage(): Storage | null {
  try {
    return globalThis.localStorage ?? null
  } catch {
    return null
  }
}

function normalizeBinding(value: unknown): string | null {
  if (typeof value !== 'string') return null
  const binding = value.trim()
  if (!binding || binding.length > 64) return null

  if (
    !/^(?:(?:Mod|Ctrl|Alt|Shift)\+){0,3}(?:[A-Za-z0-9]|TabNumber|Arrows|F(?:[1-9]|1[0-2])|Arrow(?:Left|Right|Up|Down)|Enter|Escape|Space|Tab|Backspace|Delete)$/.test(
      binding,
    )
  ) {
    return null
  }
  return binding
}

export function normalizeShortcuts(value: unknown): ShortcutBindings {
  const candidate = value && typeof value === 'object' ? (value as Record<string, unknown>) : {}
  const actions = Object.keys(DEFAULT_SHORTCUTS) as ShortcutAction[]

  const stored = new Map<ShortcutAction, string>()
  for (const action of actions) {
    const binding = normalizeBinding(candidate[action])
    if (binding) stored.set(action, binding)
  }
  const normalized = { ...DEFAULT_SHORTCUTS }
  const taken = new Set<string>()
  for (const action of actions) {
    const binding = stored.get(action)
    if (!binding) continue
    normalized[action] = taken.has(binding) ? UNBOUND_SHORTCUT : binding
    taken.add(binding)
  }
  for (const action of actions) {
    if (stored.has(action)) continue
    const binding = DEFAULT_SHORTCUTS[action]
    if (!binding || taken.has(binding)) {
      normalized[action] = UNBOUND_SHORTCUT
      continue
    }
    taken.add(binding)
  }
  return normalized
}

export function loadShortcuts(storage: Storage | null = defaultStorage()): ShortcutBindings {
  if (!storage) return { ...DEFAULT_SHORTCUTS }
  try {
    const raw = storage.getItem(SHORTCUTS_STORAGE_KEY)
    return raw ? normalizeShortcuts(JSON.parse(raw)) : { ...DEFAULT_SHORTCUTS }
  } catch {
    return { ...DEFAULT_SHORTCUTS }
  }
}

export async function loadShortcutsFromPreferences(
  storage: Storage | null = defaultStorage(),
): Promise<ShortcutBindings> {
  const raw = await getPreference(PREFERENCE_KEYS.shortcuts, storage)
  if (!raw) return loadShortcuts(storage)
  try {
    return normalizeShortcuts(JSON.parse(raw))
  } catch {
    return loadShortcuts(storage)
  }
}

export function saveShortcuts(bindings: ShortcutBindings, storage: Storage | null = defaultStorage()): void {
  const value = JSON.stringify(normalizeShortcuts(bindings))
  if (readCachedPreference(SHORTCUTS_STORAGE_KEY, storage) !== value)
    void setPreference(SHORTCUTS_STORAGE_KEY, value, storage).catch(() => undefined)
}

function normalizeKey(key: string): string {
  if (key === ' ') return 'Space'
  if (key === 'Esc') return 'Escape'
  if (key.length === 1) return key.toUpperCase()
  return key
}

export function shortcutFromKeyboardEvent(event: KeyboardEvent, isMac = false): string | null {
  const digitKey = /^Digit([1-9])$/.exec(event.code)?.[1]
  const key = digitKey ?? normalizeKey(event.key)
  if (!key || ['Control', 'Meta', 'Alt', 'Shift'].includes(key)) return null
  const modifiers: string[] = []
  if (isMac ? event.metaKey : event.ctrlKey) modifiers.push('Mod')
  if (isMac ? event.ctrlKey : event.metaKey) modifiers.push('Ctrl')
  if (event.altKey) modifiers.push('Alt')
  if (event.shiftKey) modifiers.push('Shift')
  if (modifiers.length === 0) return null
  const binding = [...modifiers, key].join('+')
  return normalizeBinding(binding)
}

function actualBindingFromEvent(event: KeyboardEvent, isMac: boolean): string | null {
  const binding = shortcutFromKeyboardEvent(event, isMac)
  if (binding) return binding
  const digit = /^Digit([1-9])$/.exec(event.code)?.[1]
  if (!digit) return null
  const modifiers: string[] = []
  if (isMac ? event.metaKey : event.ctrlKey) modifiers.push('Mod')
  if (isMac ? event.ctrlKey : event.metaKey) modifiers.push('Ctrl')
  if (event.altKey) modifiers.push('Alt')
  if (event.shiftKey) modifiers.push('Shift')
  if (modifiers.length === 0) return null
  return normalizeBinding([...modifiers, digit].join('+'))
}

export function shortcutMatches(event: KeyboardEvent, binding: string, isMac = false): boolean {
  const actual = actualBindingFromEvent(event, isMac)
  if (!actual || !binding) return false
  if (actual === binding) return true
  const wildcard = binding.slice(binding.lastIndexOf('+') + 1)
  const allowed = SHORTCUT_TOKEN_KEYS[wildcard]
  if (!allowed) return false
  const actualParts = actual.split('+')
  const actualKey = actualParts.at(-1) ?? ''

  return (
    allowed.includes(actualKey) && actualParts.slice(0, -1).join('+') === binding.slice(0, binding.lastIndexOf('+'))
  )
}

export function shortcutTokenForAction(action: ShortcutAction): string | null {
  const wildcard = DEFAULT_SHORTCUTS[action].slice(DEFAULT_SHORTCUTS[action].lastIndexOf('+') + 1)
  return SHORTCUT_TOKEN_KEYS[wildcard] ? wildcard : null
}

export type SwallowedShortcutReason = 'browserReserved' | 'xtermMeta'

export interface SwallowedShortcutPattern {
  pattern: string
  reason: SwallowedShortcutReason
}

export const SWALLOWED_SHORTCUT_PATTERNS: readonly SwallowedShortcutPattern[] = [
  { pattern: 'Mod+*Letter', reason: 'browserReserved' },
  { pattern: 'Alt+*', reason: 'xtermMeta' },
  { pattern: 'Alt+Shift+*', reason: 'xtermMeta' },
]

function splitShortcutBinding(binding: string): { modifiers: string[]; key: string } | null {
  if (!binding) return null
  const parts = binding.split('+')
  const key = parts.pop() ?? ''
  if (!key) return null
  return { modifiers: parts, key }
}

function sameModifiers(left: readonly string[], right: readonly string[]): boolean {
  if (left.length !== right.length) return false
  return left.every((modifier) => right.includes(modifier))
}

export function swallowedShortcutPattern(binding: string): SwallowedShortcutPattern | null {
  const target = splitShortcutBinding(binding)
  if (!target) return null
  for (const entry of SWALLOWED_SHORTCUT_PATTERNS) {
    const pattern = splitShortcutBinding(entry.pattern)
    if (!pattern || !sameModifiers(pattern.modifiers, target.modifiers)) continue
    if (pattern.key === '*') return entry
    if (pattern.key === '*Letter' && /^[A-Z]$/.test(target.key)) return entry
    if (pattern.key === target.key) return entry
  }
  return null
}

export function shortcutBindingsOverlap(left: string, right: string): boolean {
  if (!left || !right) return false
  if (left === right) return true
  const a = splitShortcutBinding(left)
  const b = splitShortcutBinding(right)
  if (!a || !b || !sameModifiers(a.modifiers, b.modifiers)) return false
  const aKeys = SHORTCUT_TOKEN_KEYS[a.key]
  const bKeys = SHORTCUT_TOKEN_KEYS[b.key]
  if (aKeys && bKeys) return aKeys.some((key) => bKeys.includes(key))
  if (aKeys) return aKeys.includes(b.key)
  if (bKeys) return bKeys.includes(a.key)
  return false
}

export type ShortcutConflictKind = 'duplicate' | 'swallowed'

export interface ShortcutConflict {
  kind: ShortcutConflictKind

  peers?: ShortcutAction[]

  pattern?: string
  reason?: SwallowedShortcutReason
}

export function shortcutConflicts(bindings: ShortcutBindings): Partial<Record<ShortcutAction, ShortcutConflict>> {
  const actions = Object.keys(DEFAULT_SHORTCUTS) as ShortcutAction[]
  const conflicts: Partial<Record<ShortcutAction, ShortcutConflict>> = {}
  for (const action of actions) {
    const binding = bindings[action] ?? UNBOUND_SHORTCUT
    if (!binding) continue
    const peers = actions.filter(
      (other) => other !== action && shortcutBindingsOverlap(binding, bindings[other] ?? UNBOUND_SHORTCUT),
    )
    if (peers.length > 0) {
      conflicts[action] = { kind: 'duplicate', peers }
      continue
    }
    if (binding === DEFAULT_SHORTCUTS[action]) continue
    const swallowed = swallowedShortcutPattern(binding)
    if (swallowed) conflicts[action] = { kind: 'swallowed', pattern: swallowed.pattern, reason: swallowed.reason }
  }
  return conflicts
}

export function conflictedShortcutActions(bindings: ShortcutBindings): Set<ShortcutAction> {
  return new Set(Object.keys(shortcutConflicts(bindings)) as ShortcutAction[])
}

export function boundShortcutAction(
  event: KeyboardEvent,
  bindings: ShortcutBindings,
  isMac = false,
): ShortcutAction | null {
  if (event.defaultPrevented) return null
  const conflicted = conflictedShortcutActions(bindings)
  for (const action of Object.keys(DEFAULT_SHORTCUTS) as ShortcutAction[]) {
    if (conflicted.has(action)) continue
    const binding = bindings[action] ?? UNBOUND_SHORTCUT
    if (!binding) continue
    if (shortcutMatches(event, binding, isMac)) return action
  }
  return null
}

export function shortcutTabNumber(event: KeyboardEvent): string | null {
  if (/^[1-9]$/.test(event.key)) return event.key
  return /^Digit([1-9])$/.exec(event.code)?.[1] ?? null
}

export function shortcutDisplay(binding: string, isMac = false): string {
  return binding
    .split('+')
    .map((part) => {
      if (part === 'Mod') return isMac ? '⌘' : 'Ctrl'
      if (part === 'Ctrl') return isMac ? '⌃' : 'Ctrl'
      if (part === 'Alt') return isMac ? '⌥' : 'Alt'
      if (part === 'Shift') return isMac ? '⇧' : 'Shift'
      if (part === 'ArrowLeft') return '←'
      if (part === 'ArrowRight') return '→'
      if (part === 'ArrowUp') return '↑'
      if (part === 'ArrowDown') return '↓'
      if (part === 'Escape') return 'Esc'
      if (part === 'Space') return 'Space'
      if (SHORTCUT_TOKEN_LABELS[part]) return SHORTCUT_TOKEN_LABELS[part]
      return part
    })
    .join(' + ')
}

export function shortcutKeycapTokens(binding: string, isMac = false): string[] {
  if (!binding) return []
  return shortcutDisplay(binding, isMac).split(' + ')
}
