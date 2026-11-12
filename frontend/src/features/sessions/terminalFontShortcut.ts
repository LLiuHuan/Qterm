export function terminalFontSizeStep(
  event: Pick<KeyboardEvent, 'type' | 'key' | 'altKey' | 'ctrlKey' | 'metaKey'>,
  isMac: boolean,
): -1 | 0 | 1 {
  if (event.type !== 'keydown' || event.altKey) return 0
  const modifierHeld = isMac ? event.metaKey || event.ctrlKey : event.ctrlKey
  if (!modifierHeld) return 0
  if (event.key === '=' || event.key === '+' || event.key === 'Add') return 1
  if (event.key === '-' || event.key === '_' || event.key === 'Subtract') return -1
  return 0
}
