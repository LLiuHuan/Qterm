interface WailsRuntimeWindow extends Window {
  _wails?: {
    environment?: {
      OS?: string
    }
  }
}

export function hasWailsRuntime(): boolean {
  return Boolean((window as WailsRuntimeWindow)._wails?.environment?.OS)
}

export function whenWailsRuntime(callback: () => void, timeoutMs = 15_000): () => void {
  if (hasWailsRuntime()) {
    callback()
    return () => {}
  }
  const startedAt = Date.now()
  const timer = window.setInterval(() => {
    if (hasWailsRuntime()) {
      window.clearInterval(timer)
      callback()
      return
    }
    if (Date.now() - startedAt >= timeoutMs) window.clearInterval(timer)
  }, 50)
  return () => window.clearInterval(timer)
}

export function createRuntimeGateway<T extends object>(
  createWails: () => T,
  createMemory: () => T,
  isRuntimeAvailable: () => boolean = hasWailsRuntime,
): T {
  const memory = createMemory()
  let wails: T | undefined
  return new Proxy(memory, {
    get(target, property) {
      const source = isRuntimeAvailable() ? (wails ??= createWails()) : target
      const value = Reflect.get(source, property)
      return typeof value === 'function' ? value.bind(source) : value
    },
  })
}
