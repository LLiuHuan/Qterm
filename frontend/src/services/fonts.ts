import {
  isTerminalFontAvailable,
  isKnownNerdFontFamily,
  TERMINAL_FONT_OPTIONS,
} from '../features/sessions/terminalFont'
import { errorMessage } from './errors'
import { createRuntimeGateway } from './runtime'

export interface TerminalFontFamily {
  family: string

  monospace: boolean

  hasNerdIcons: boolean

  localizedNames?: Record<string, string>
}

export interface FontGateway {
  readonly persistent: boolean

  listTerminalFonts(refresh?: boolean): Promise<TerminalFontFamily[]>
}

class PreviewFontGateway implements FontGateway {
  readonly persistent = false

  async listTerminalFonts(): Promise<TerminalFontFamily[]> {
    return TERMINAL_FONT_OPTIONS.filter((option) => option.value !== '')
      .filter((option) => isTerminalFontAvailable(option.value))

      .map((option) => ({
        family: option.value,
        monospace: true,
        hasNerdIcons: isKnownNerdFontFamily(option.value),
      }))
  }
}

class WailsFontGateway implements FontGateway {
  readonly persistent = true

  async listTerminalFonts(refresh = false): Promise<TerminalFontFamily[]> {
    const { ListTerminalFonts } = await import('../../bindings/changeme/internal/app/service')
    const families = await ListTerminalFonts(refresh)
    return (families ?? []).map((item) => ({
      family: item.family,
      monospace: item.monospace === true,
      hasNerdIcons: item.hasNerdIcons === true,

      localizedNames: (item as { localizedNames?: Record<string, string> }).localizedNames,
    }))
  }
}

export function createFontGateway(): FontGateway {
  return createRuntimeGateway<FontGateway>(
    () => new WailsFontGateway(),
    () => new PreviewFontGateway(),
  )
}

export function fontErrorMessage(error: unknown, fallback: string): string {
  return errorMessage(error, fallback)
}
