import { ref } from 'vue'

export const MAC_PLATFORM_VALUES: readonly string[] = ['Macintosh', 'MacIntel', 'MacPPC', 'Mac68K']

export function detectMacPlatform(platform?: string): boolean {
  const value = platform ?? (typeof navigator === 'undefined' ? '' : (navigator.platform ?? ''))
  return MAC_PLATFORM_VALUES.includes(value)
}

export const isMacPlatform = ref(detectMacPlatform())

export const NATIVE_WINDOW_USER_AGENT_MARK = 'wails.io'

export function detectNativeWindow(
  userAgent: string = typeof navigator === 'undefined' ? '' : (navigator.userAgent ?? ''),
  protocol: string = typeof location === 'undefined' ? '' : location.protocol,
): boolean {
  if (protocol === 'wails:') return true
  return userAgent.includes(NATIVE_WINDOW_USER_AGENT_MARK)
}

export function detectNativeMacWindow(
  platform: string | undefined = typeof navigator === 'undefined' ? '' : (navigator.platform ?? ''),
  userAgent: string = typeof navigator === 'undefined' ? '' : (navigator.userAgent ?? ''),
  protocol: string = typeof location === 'undefined' ? '' : location.protocol,
): boolean {
  if (!detectMacPlatform(platform)) return false
  return detectNativeWindow(userAgent, protocol)
}

export const isNativeMacWindow = ref(detectNativeMacWindow())

export function refreshIsMacPlatform(): void {
  isMacPlatform.value = detectMacPlatform()
  isNativeMacWindow.value = detectNativeMacWindow()
}
