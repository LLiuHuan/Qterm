export function evenPixel(value: number, minimum = 0): number {
  const safeMinimum = Math.max(0, Math.ceil(minimum / 2) * 2)
  if (!Number.isFinite(value)) return safeMinimum
  const rounded = Math.round(value)
  return Math.max(safeMinimum, rounded - (rounded % 2))
}

export function observeEvenDialogHeights(): () => void {
  if (typeof ResizeObserver === 'undefined' || typeof MutationObserver === 'undefined') return () => undefined

  const selector = '[data-slot="dialog-content"].qterm-dialog'
  const observed = new Map<HTMLElement, { resize: ResizeObserver; originalPaddingBottom: string }>()

  function observe(element: HTMLElement): void {
    if (observed.has(element)) return
    const originalPaddingBottom = element.style.paddingBottom
    const basePaddingBottom = Number.parseFloat(getComputedStyle(element).paddingBottom) || 0
    let adjustment = 0
    const resize = new ResizeObserver(([entry]) => {
      if (!entry || !element.isConnected) return
      const borderBoxSize = entry.borderBoxSize
      const measuredHeight =
        (Array.isArray(borderBoxSize)
          ? borderBoxSize[0]?.blockSize
          : (borderBoxSize as unknown as { blockSize?: number } | undefined)?.blockSize) ??
        element.getBoundingClientRect().height

      const naturalHeight = Math.round((measuredHeight - adjustment) * 64) / 64
      const nextAdjustment = Math.max(0, Math.ceil(naturalHeight / 2) * 2 - naturalHeight)
      if (Math.abs(nextAdjustment - adjustment) < 1 / 128) return
      adjustment = nextAdjustment
      element.style.paddingBottom = `${basePaddingBottom + adjustment}px`
    })
    observed.set(element, { resize, originalPaddingBottom })
    resize.observe(element)
  }

  function visit(node: Node, callback: (element: HTMLElement) => void): void {
    if (!(node instanceof Element)) return
    if (node.matches(selector)) callback(node as HTMLElement)
    node.querySelectorAll<HTMLElement>(selector).forEach(callback)
  }

  function disconnect(element: HTMLElement): void {
    const item = observed.get(element)
    if (!item) return
    item.resize.disconnect()
    element.style.paddingBottom = item.originalPaddingBottom
    observed.delete(element)
  }

  document.querySelectorAll<HTMLElement>(selector).forEach(observe)
  const mutations = new MutationObserver((records) => {
    for (const record of records) {
      record.removedNodes.forEach((node) => visit(node, disconnect))
      record.addedNodes.forEach((node) => visit(node, observe))
    }
  })
  mutations.observe(document.body, { childList: true, subtree: true })

  return () => {
    mutations.disconnect()
    for (const element of observed.keys()) disconnect(element)
  }
}
