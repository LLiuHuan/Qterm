import type { CSSProperties, InjectionKey, Ref } from 'vue'

export interface DialogThemeContextValue {
  className: string
  style: CSSProperties
}

export const dialogThemeContextKey: InjectionKey<Readonly<Ref<DialogThemeContextValue>>> =
  Symbol('qterm-dialog-theme-context')
