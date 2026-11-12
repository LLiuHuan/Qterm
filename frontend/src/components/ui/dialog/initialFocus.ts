const FORM_CONTROL_SELECTOR = 'input, textarea, select'

function isUsableControl(element: HTMLElement): boolean {
  if (element.matches(':disabled, [hidden], [readonly], input[type="hidden"]')) return false
  if (element.closest('[hidden], [inert]')) return false
  const style = getComputedStyle(element)
  return style.visibility !== 'hidden' && style.display !== 'none'
}

function isInsideViewport(element: HTMLElement, container: HTMLElement): boolean {
  const rect = element.getBoundingClientRect()
  const bounds = container.getBoundingClientRect()
  return rect.bottom > bounds.top && rect.top < bounds.bottom && rect.right > bounds.left && rect.left < bounds.right
}

export function focusFirstFormControl(event: Event): boolean {
  if (event.defaultPrevented || !(event.target instanceof HTMLElement)) return false
  const container = event.target
  const controls = Array.from(container.querySelectorAll<HTMLElement>(FORM_CONTROL_SELECTOR))
    .filter(isUsableControl)
    .filter((element) => isInsideViewport(element, container))

  const control = controls.find((element) => element.hasAttribute('autofocus')) ?? controls[0]
  if (!control) return false
  event.preventDefault()

  control.focus({ preventScroll: true })

  if (control instanceof HTMLInputElement || control instanceof HTMLTextAreaElement) {
    const length = control.value.length
    try {
      control.setSelectionRange(length, length)
    } catch {}
  }
  return true
}

export function avoidInitialScrollFocus(event: Event): void {
  if (event.defaultPrevented || !(event.target instanceof HTMLElement)) return
  const container = event.target
  const first = Array.from(
    container.querySelectorAll<HTMLElement>('button, input, select, textarea, a[href], [tabindex]'),
  ).find(
    (element) =>
      element.tabIndex >= 0 &&
      !element.matches(':disabled, [hidden], input[type="hidden"]') &&
      !element.closest('[hidden], [inert]') &&
      getComputedStyle(element).visibility !== 'hidden' &&
      getComputedStyle(element).display !== 'none',
  )
  if (first?.dataset.slot !== 'scroll-area-viewport') return
  event.preventDefault()
  container.focus({ preventScroll: true })
}

export function applyInitialDialogFocus(event: Event): void {
  if (focusFirstFormControl(event)) return
  avoidInitialScrollFocus(event)
}
