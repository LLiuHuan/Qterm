export type ShellIntegrationProtocol = 133 | 633
export type ShellIntegrationPhase = 'A' | 'B' | 'C' | 'D'

export interface ShellIntegrationEvent {
  source: ShellIntegrationProtocol
  phase: ShellIntegrationPhase

  argument: string

  exitCode?: number
}

export function parseShellIntegrationEvent(
  source: ShellIntegrationProtocol,
  data: string,
): ShellIntegrationEvent | null {
  const match = /^([ABCD])(?:;(.*))?$/.exec(data)
  if (!match) return null

  const phase = match[1] as ShellIntegrationPhase
  const argument = match[2] ?? ''
  if (phase !== 'D') return { source, phase, argument }

  const status = argument.split(';', 1)[0]?.trim() ?? ''
  if (!/^\d+$/.test(status)) return { source, phase, argument }
  const exitCode = Number(status)
  if (!Number.isSafeInteger(exitCode) || exitCode > 255) return { source, phase, argument }
  return { source, phase, argument, exitCode }
}
