import type { Terminal, IDisposable } from '@xterm/xterm'

export function subscribeTerminalInputSource(
  terminal: Terminal,
  receive: (data: string, local: boolean) => void,
): IDisposable {
  const internal = terminal as Terminal & {
    _core?: { coreService?: { onUserInput?: (listener: () => void) => IDisposable } }
  }
  let userInput = false
  const userSubscription = internal._core?.coreService?.onUserInput?.(() => {
    userInput = true
  })
  const dataSubscription = terminal.onData((data) => {
    const local = userInput
    userInput = false
    receive(data, local)
  })
  return {
    dispose() {
      dataSubscription.dispose()
      userSubscription?.dispose()
    },
  }
}
