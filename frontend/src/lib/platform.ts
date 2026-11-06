import { ref } from 'vue'

export const MAC_PLATFORM_VALUES: readonly string[] = ['Macintosh', 'MacIntel', 'MacPPC', 'Mac68K']

export function detectMacPlatform(platform?: string): boolean {
  const value = platform ?? (typeof navigator === 'undefined' ? '' : (navigator.platform ?? ''))
  return MAC_PLATFORM_VALUES.includes(value)
}

export const isMacPlatform = ref(detectMacPlatform())

export function refreshIsMacPlatform(): void {
  isMacPlatform.value = detectMacPlatform()
}
