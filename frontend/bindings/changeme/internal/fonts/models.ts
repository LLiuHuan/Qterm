export interface Family {
  family: string

  monospace: boolean

  hasNerdIcons: boolean

  localizedNames?: { [_ in string]?: string } | null
}
