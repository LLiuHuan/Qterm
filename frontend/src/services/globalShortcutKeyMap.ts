const NAMED_KEYS: Record<string, string> = {
  Space: 'space',
  Escape: 'escape',
  Esc: 'escape',
  Enter: 'enter',
  Return: 'enter',
  Backspace: 'backspace',
  Tab: 'tab',
  Delete: 'delete',
  ArrowUp: 'up',
  ArrowDown: 'down',
  ArrowLeft: 'left',
  ArrowRight: 'right',
  Home: 'home',
  End: 'end',
  PageUp: 'page up',
  PageDown: 'page down',

  ForwardDelete: 'forward delete',
}

const MODIFIERS: Record<string, string> = {
  Mod: 'CmdOrCtrl',
  Ctrl: 'Ctrl',
  Alt: 'Alt',
  Shift: 'Shift',
}

export function toWailsAccelerator(binding: string): string | null {
  const trailingPlus = binding.endsWith('++')
  const parts = (trailingPlus ? binding.slice(0, -2) : binding)
    .split('+')
    .map((part) => part.trim())
    .filter((part) => part !== '')
  if (trailingPlus) {
    if (parts.length === 0) return null
  } else if (parts.length < 2) {
    return null
  }
  const key = trailingPlus ? '+' : parts[parts.length - 1]
  const modifiers = trailingPlus ? parts : parts.slice(0, -1)
  const translatedModifiers: string[] = []
  for (const modifier of modifiers) {
    const translated = MODIFIERS[modifier]
    if (!translated) return null
    if (!translatedModifiers.includes(translated)) translatedModifiers.push(translated)
  }
  if (translatedModifiers.length === 0) return null
  const translatedKey = translateKey(key)
  if (!translatedKey) return null
  return [...translatedModifiers, translatedKey].join('+')
}

function translateKey(key: string): string | null {
  const named = NAMED_KEYS[key]
  if (named) return named

  if (/^F([1-9]|[1-2][0-9]|3[0-5])$/.test(key)) return key.toLowerCase()
  if (key.length === 1) {
    const lower = key.toLowerCase()

    return lower === '+' ? 'plus' : lower
  }
  return null
}

export function fromWailsAccelerator(accelerator: string): string {
  if (!accelerator) return ''
  const parts = accelerator.split('+')
  const key = parts[parts.length - 1]
  const modifiers = parts.slice(0, -1)
  const lowered = modifiers.map((modifier) => modifier.toLowerCase())

  const hasPrimary = lowered.some(
    (item) => item === 'cmd' || item === 'command' || item === 'cmdorctrl' || item === 'super' || item === 'win',
  )
  const hasControl = lowered.includes('ctrl')
  const translatedModifiers: string[] = []
  if (hasPrimary || hasControl) translatedModifiers.push('Mod')
  if (hasPrimary && hasControl) translatedModifiers.push('Ctrl')
  for (const lower of lowered) {
    if (lower === 'cmd' || lower === 'command' || lower === 'cmdorctrl' || lower === 'super' || lower === 'win') {
      continue
    }
    if (lower === 'ctrl') continue
    if (lower === 'alt' || lower === 'optionoralt' || lower === 'option') {
      if (!translatedModifiers.includes('Alt')) translatedModifiers.push('Alt')
      continue
    }
    if (lower === 'shift') {
      if (!translatedModifiers.includes('Shift')) translatedModifiers.push('Shift')
      continue
    }
    if (lower === 'super' || lower === 'win') {
      if (!translatedModifiers.includes('Mod')) translatedModifiers.push('Mod')
    }
  }
  const displayKey = reverseKey(key)
  return [...translatedModifiers, displayKey].join('+')
}

function reverseKey(key: string): string {
  const lower = key.toLowerCase()
  const named = Object.entries(NAMED_KEYS).find(([, value]) => value === lower)
  if (named) return named[0] === 'Esc' ? 'Escape' : named[0]
  if (/^f\d{1,2}$/.test(lower)) return lower.toUpperCase()
  if (lower === 'plus') return '+'
  return lower.toUpperCase()
}
