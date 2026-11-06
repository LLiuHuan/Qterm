const MAX_CACHE_ENTRIES = 512

let canvasContext: CanvasRenderingContext2D | null | undefined
const widthCache = new Map<string, number>()

function getContext(): CanvasRenderingContext2D | null {
  if (canvasContext !== undefined) return canvasContext
  try {
    canvasContext = document.createElement('canvas').getContext('2d')
  } catch {
    canvasContext = null
  }
  return canvasContext
}

export function measureGutterText(text: string, fontFamily: string, fontSize: number): number {
  if (!text) return 0
  const font = `${fontSize}px ${fontFamily}`
  const key = `${font}\u0000${text}`
  const cached = widthCache.get(key)
  if (cached !== undefined) return cached

  let width = 0
  const context = getContext()

  if (context && typeof context.measureText === 'function') {
    context.font = font
    width = context.measureText(text).width
  }
  if (!Number.isFinite(width) || width <= 0) {
    width = text.length * fontSize * 0.62
  }

  if (widthCache.size >= MAX_CACHE_ENTRIES) widthCache.clear()
  widthCache.set(key, width)
  return width
}
