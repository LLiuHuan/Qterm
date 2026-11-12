import { inject, provide, ref, type InjectionKey, type Ref } from 'vue'

export type TerminalPaneCommand =
  'clear' | 'search' | 'copySelection' | 'paste' | 'splitRight' | 'splitBottom' | 'focusTerminal'

export interface TerminalPaneCommandRequest {
  sessionId: string
  command: TerminalPaneCommand

  nonce: number
}

export interface TerminalPaneCommandBus {
  request: Ref<TerminalPaneCommandRequest | null>
  send(sessionId: string, command: TerminalPaneCommand): void
}

export const terminalPaneCommandKey: InjectionKey<TerminalPaneCommandBus> = Symbol('qterm.terminalPaneCommand')

export function provideTerminalPaneCommandBus(): TerminalPaneCommandBus {
  const request = ref<TerminalPaneCommandRequest | null>(null)
  let nonce = 0
  const bus: TerminalPaneCommandBus = {
    request,
    send(sessionId, command) {
      if (!sessionId) return
      nonce += 1
      request.value = { sessionId, command, nonce }
    },
  }
  provide(terminalPaneCommandKey, bus)
  return bus
}

export function useTerminalPaneCommandBus(): TerminalPaneCommandBus | null {
  return inject(terminalPaneCommandKey, null)
}
