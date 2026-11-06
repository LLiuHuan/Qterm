import type { CSSProperties } from 'vue'
import { tagColorStyle } from '../../lib/colorTag'

export function snippetColorStyle(color?: string): CSSProperties | undefined {
  return tagColorStyle(color, '--snippet')
}
