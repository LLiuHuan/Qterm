import { shortcutTabNumber, type ShortcutAction } from './shortcutStore'

export type PaneFocusDirection = 'left' | 'right' | 'up' | 'down'

export function paneFocusDirection(event: KeyboardEvent): PaneFocusDirection | null {
  switch (event.key) {
    case 'ArrowLeft':
      return 'left'
    case 'ArrowRight':
      return 'right'
    case 'ArrowUp':
      return 'up'
    case 'ArrowDown':
      return 'down'
    default:
      return null
  }
}

export interface ShortcutCapabilities {
  nextTab(): void
  previousTab(): void
  closeTab(): void

  switchTab(tabNumber: string | null): void
  newWindow(): void
  newLocalTab(): void
  clearTerminal(): void
  openTerminalSearch(): void
  copyTerminalSelection(): void
  pasteTerminal(): void
  reconnectSession(): void
  disconnectSession(): void

  commandPalette(): void

  togglePaneMaximize(): void

  focusNextPane(direction: PaneFocusDirection | null): void
  closePane(): void
  splitRight(): void
  splitBottom(): void
  toggleSidebar(): void

  remoteFileSave(): void
}

export type ShortcutDispatch = Record<ShortcutAction, (event: KeyboardEvent) => void>

const ACTIONS_WITHOUT_ACTIVE_TAB: ReadonlySet<ShortcutAction> = new Set<ShortcutAction>([
  'newWindow',
  'newLocalTab',
  'toggleSidebar',

  'remoteFileSave',
])

export function shortcutNeedsActiveTab(action: ShortcutAction): boolean {
  return !ACTIONS_WITHOUT_ACTIVE_TAB.has(action)
}

export function createShortcutDispatch(capabilities: ShortcutCapabilities): ShortcutDispatch {
  return {
    nextTab: () => capabilities.nextTab(),
    previousTab: () => capabilities.previousTab(),
    closeTab: () => capabilities.closeTab(),
    switchTab: (event) => capabilities.switchTab(shortcutTabNumber(event)),
    newWindow: () => capabilities.newWindow(),
    newLocalTab: () => capabilities.newLocalTab(),
    clearTerminal: () => capabilities.clearTerminal(),
    openTerminalSearch: () => capabilities.openTerminalSearch(),
    copyTerminalSelection: () => capabilities.copyTerminalSelection(),
    pasteTerminal: () => capabilities.pasteTerminal(),
    reconnectSession: () => capabilities.reconnectSession(),
    disconnectSession: () => capabilities.disconnectSession(),
    commandPalette: () => capabilities.commandPalette(),
    togglePaneMaximize: () => capabilities.togglePaneMaximize(),
    focusNextPane: (event) => capabilities.focusNextPane(paneFocusDirection(event)),
    closePane: () => capabilities.closePane(),
    splitRight: () => capabilities.splitRight(),
    splitBottom: () => capabilities.splitBottom(),
    toggleSidebar: () => capabilities.toggleSidebar(),
    remoteFileSave: () => capabilities.remoteFileSave(),
  }
}
