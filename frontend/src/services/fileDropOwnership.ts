export const FILE_DROP_EVENT = 'qterm:file-drop'

export interface NativeDropEventLike {
  sender?: string | null
}

export function shouldHandleDrop(
  event: NativeDropEventLike | null | undefined,
  selfName: string | null | undefined,
): boolean {
  const sender = typeof event?.sender === 'string' ? event.sender.trim() : ''
  const self = typeof selfName === 'string' ? selfName.trim() : ''
  if (!sender || !self) return true
  return sender === self
}

export async function resolveSelfWindowName(): Promise<string | null> {
  try {
    const { Window } = await import('@wailsio/runtime')
    const name = await Window.Name()
    return typeof name === 'string' && name.trim() ? name : null
  } catch {
    return null
  }
}
