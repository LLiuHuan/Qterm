export interface SftpIconSources {
  light: string

  dark: string
}

const iconModules = import.meta.glob('../../assets/icons/*.svg', {
  eager: true,
  query: '?url',
  import: 'default',
}) as Record<string, string>

function iconNameOf(modulePath: string): string {
  const file = modulePath.split('/').pop() ?? modulePath
  return file.replace(/\.svg$/, '')
}

const THEME_VARIANT_SUFFIX = '_light'

const iconSourcesByName = Object.fromEntries(
  Object.entries(iconModules).map(([path, source]) => [iconNameOf(path), source]),
) as Record<string, string>

export const sftpIconSources = (() => {
  const result: Record<string, SftpIconSources> = {}
  for (const [name, source] of Object.entries(iconSourcesByName)) {
    if (name.endsWith(THEME_VARIANT_SUFFIX)) continue
    const variant = iconSourcesByName[`${name}${THEME_VARIANT_SUFFIX}`]
    result[name] = { light: source, dark: variant ?? source }
  }
  for (const [name, source] of Object.entries(iconSourcesByName)) {
    if (!name.endsWith(THEME_VARIANT_SUFFIX)) continue
    const base = name.slice(0, -THEME_VARIANT_SUFFIX.length)
    if (!result[base]) result[base] = { light: source, dark: source }
  }
  return result
})()

export const SFTP_THEMED_ICON_NAMES = Object.entries(iconSourcesByName)
  .filter(([name]) => name.endsWith(THEME_VARIANT_SUFFIX))
  .map(([name]) => name)
  .sort()

export type SftpIconName = string
