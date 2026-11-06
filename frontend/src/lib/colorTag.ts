import type { CSSProperties } from 'vue'

export const TAG_COLOR_OPTIONS = [
  '#ef4444',
  '#f97316',
  '#eab308',
  '#22c55e',
  '#14b8a6',
  '#3b82f6',
  '#8b5cf6',
  '#ec4899',
] as const

export function tagColorStyle(color: string | undefined, varPrefix = '--tag'): CSSProperties | undefined {
  if (!color || !/^#[\da-f]{6}$/i.test(color)) return undefined
  const channels = [1, 3, 5].map((offset) => {
    const value = Number.parseInt(color.slice(offset, offset + 2), 16) / 255
    return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4
  })
  const luminance = channels[0]! * 0.2126 + channels[1]! * 0.7152 + channels[2]! * 0.0722
  return {
    [`${varPrefix}-color`]: color,
    [`${varPrefix}-hover-text`]: luminance > 0.179 ? '#000000' : '#ffffff',
  } as CSSProperties
}
