import type { Terminal } from '@xterm/xterm'

export interface TerminalDomFontMetricsOptions {
  fontFamily: string
  fontSize: number
}

export interface TerminalFontDiagnostics {
  fontLoadDiagnostics: string
  fontCanvasDiagnostics: string
  frameSnapshotDiagnostics: string
}

interface XtermPrivateCharSizeService {
  width: number
  height: number
  measure: () => void
  _measureStrategy?: { measure: () => Readonly<{ width: number; height: number }> }
}

interface XtermPrivateTerminal {
  _core?: {
    _charSizeService?: XtermPrivateCharSizeService
    _renderService?: XtermPrivateRenderService
  }
}

interface XtermPrivateDomWidthCache {
  _font?: string
  _fontSize?: number
  _weight?: string
  _weightBold?: string
  _measure?: (text: string, variant: number) => number
  clear?: () => void
}

interface XtermPrivateDomRenderer {
  _widthCache?: XtermPrivateDomWidthCache
  handleCharSizeChanged?: () => void
  _setDefaultSpacing?: () => void
}

interface XtermPrivateRenderService {
  _renderer?: { value?: XtermPrivateDomRenderer }
}

function installWkWebViewDomFontMetrics(instance: Terminal, options: TerminalDomFontMetricsOptions): void {
  const service = (instance as unknown as XtermPrivateTerminal)._core?._charSizeService
  const helper = instance.element?.querySelector<HTMLElement>('.xterm-helpers')
  if (!service?._measureStrategy || !helper) return

  const probe = document.createElement('span')
  probe.className = 'qterm-char-measure-probe'
  probe.textContent = 'W'.repeat(32)
  probe.setAttribute('aria-hidden', 'true')
  Object.assign(probe.style, {
    position: 'absolute',
    visibility: 'hidden',
    whiteSpace: 'pre',
    fontKerning: 'none',
  })
  helper.appendChild(probe)

  const measure = (): Readonly<{ width: number; height: number }> => {
    probe.style.fontFamily = instance.options.fontFamily ?? options.fontFamily
    probe.style.fontSize = `${instance.options.fontSize ?? options.fontSize}px`
    return {
      width: Number(probe.offsetWidth) / 32,
      height: Number(probe.offsetHeight),
    }
  }
  const domSize = measure()
  if (domSize.width <= 0 || domSize.height <= 0) {
    probe.remove()
    return
  }

  if (Math.abs(service.width - domSize.width) < 0.1 && Math.abs(service.height - domSize.height) < 0.1) {
    probe.remove()
    return
  }
  service._measureStrategy.measure = measure
  service.measure()
}

export function installWkWebViewDomRendererMetrics(instance: Terminal): void {
  const renderer = (instance as unknown as XtermPrivateTerminal)._core?._renderService?._renderer?.value
  const cache = renderer?._widthCache
  const host = instance.element?.querySelector<HTMLElement>('.xterm-helpers') ?? instance.element
  if (!renderer || !cache || typeof cache._measure !== 'function' || !host) return
  const patched = cache as XtermPrivateDomWidthCache & { __qtermDomMetrics?: HTMLElement }
  if (patched.__qtermDomMetrics) return

  const probe = document.createElement('span')
  probe.setAttribute('aria-hidden', 'true')
  Object.assign(probe.style, {
    position: 'absolute',
    visibility: 'hidden',
    whiteSpace: 'pre',
    fontKerning: 'none',
  })
  host.appendChild(probe)

  const originalMeasure = cache._measure.bind(cache)
  const applyFont = (variant: number): void => {
    const bold = (variant & 1) !== 0
    const italic = (variant & 2) !== 0
    probe.style.fontFamily = String(cache._font ?? '')
    probe.style.fontSize = `${Number(cache._fontSize ?? 0)}px`
    probe.style.fontWeight = String((bold ? cache._weightBold : cache._weight) ?? 'normal')
    probe.style.fontStyle = italic ? 'italic' : 'normal'
  }

  applyFont(0)
  probe.textContent = 'W'
  const domWidth = probe.getBoundingClientRect().width

  const canvasWidth = ((): number | null => {
    try {
      return originalMeasure('W', 0)
    } catch {
      return null
    }
  })()
  if (canvasWidth === null || !Number.isFinite(canvasWidth) || !Number.isFinite(domWidth) || domWidth <= 0) {
    probe.remove()
    return
  }
  if (Math.abs(canvasWidth - domWidth) < 0.1) {
    probe.remove()
    return
  }

  cache._measure = (text: string, variant: number): number => {
    applyFont(variant)
    probe.textContent = text
    const width = probe.getBoundingClientRect().width

    return Number.isFinite(width) && width > 0 ? width : originalMeasure(text, variant)
  }
  patched.__qtermDomMetrics = probe
  cache.clear?.()

  if (typeof renderer.handleCharSizeChanged === 'function') renderer.handleCharSizeChanged()
  else renderer._setDefaultSpacing?.()
}

function measureTerminalDomCell(instance: Terminal): { width: number; height: number } | null {
  const host = instance.element?.querySelector<HTMLElement>('.xterm-helpers') ?? instance.element
  if (!host) return null
  const probe = document.createElement('span')
  probe.textContent = 'W'.repeat(32)
  probe.setAttribute('aria-hidden', 'true')
  Object.assign(probe.style, {
    position: 'absolute',
    visibility: 'hidden',
    whiteSpace: 'pre',
    fontKerning: 'none',
    fontFamily: instance.options.fontFamily ?? '',
    fontSize: `${String(instance.options.fontSize ?? '')}px`,
  })
  host.appendChild(probe)
  const width = Number(probe.offsetWidth) / 32
  const height = Number(probe.offsetHeight)
  probe.remove()
  if (!Number.isFinite(width) || width <= 0 || height <= 0) return null
  return { width, height }
}

export function exposeFontDiagnostics(instance: Terminal, stage: string, diagnostics: TerminalFontDiagnostics): void {
  if (!import.meta.env.DEV || import.meta.env.MODE === 'test') return
  const service = (instance as unknown as XtermPrivateTerminal)._core?._charSizeService
  const rows = instance.element?.querySelector<HTMLElement>('.xterm-rows')
  const firstRow = rows?.firstElementChild instanceof HTMLElement ? rows.firstElementChild : null
  const strategy = service?._measureStrategy?.constructor?.name ?? 'unknown'
  const details = [
    `stage=${stage}`,
    `font=${String(instance.options.fontFamily ?? '')}`,
    `size=${String(instance.options.fontSize ?? '')}`,
    `cell=${service ? `${service.width}x${service.height}` : 'missing'}`,
    `strategy=${strategy}`,
    `rowsSpacing=${rows ? getComputedStyle(rows).letterSpacing : 'missing'}`,
    `rowSpacing=${firstRow ? getComputedStyle(firstRow).letterSpacing : 'missing'}`,
    diagnostics.fontLoadDiagnostics,
    diagnostics.fontCanvasDiagnostics,
    diagnostics.frameSnapshotDiagnostics,
  ].join('; ')
  instance.textarea?.setAttribute('title', details)

  const domCell = measureTerminalDomCell(instance)
  const essential = [
    `stage=${stage}`,
    `cell=${service ? `${service.width}x${service.height}` : 'missing'}`,
    `domCell=${domCell ? `${domCell.width}x${domCell.height}` : 'missing'}`,

    `rowsSpacing=${rows ? getComputedStyle(rows).letterSpacing : 'missing'}`,
    `rowSpacing=${firstRow ? getComputedStyle(firstRow).letterSpacing : 'missing'}`,
    `font=${String(instance.options.fontFamily ?? '')}`,
    `size=${String(instance.options.fontSize ?? '')}`,
    diagnostics.fontCanvasDiagnostics,
    diagnostics.frameSnapshotDiagnostics,
  ].join('; ')
  const textarea = instance.textarea
  if (textarea) {
    const marker = ' | diag: '
    const base = (textarea.getAttribute('aria-label') ?? '').split(marker)[0]
    textarea.setAttribute('aria-label', `${base}${marker}${essential}`)
  }
}

export function openTerminalWithDomFontMetrics(
  instance: Terminal,
  element: HTMLElement,
  options: TerminalDomFontMetricsOptions,
): void {
  const scope = globalThis as unknown as { OffscreenCanvas: typeof OffscreenCanvas | undefined }
  const original = scope.OffscreenCanvas
  let disabled = false
  try {
    scope.OffscreenCanvas = undefined
    disabled = scope.OffscreenCanvas === undefined
  } catch {}
  try {
    instance.open(element)
    installWkWebViewDomFontMetrics(instance, options)
    installWkWebViewDomRendererMetrics(instance)
  } finally {
    if (disabled) scope.OffscreenCanvas = original
  }
}
