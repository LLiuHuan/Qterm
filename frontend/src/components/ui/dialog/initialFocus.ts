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
