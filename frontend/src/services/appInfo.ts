import buildConfig from '../../../build/config.yml?raw'

const versionMatch = buildConfig.match(/^\s{2}version:\s*["']?([^"'\s#]+)/m)

export const QTERM_VERSION = versionMatch?.[1] ?? 'dev'

export const QTERM_DEV_BUILD = Boolean(import.meta.hot) || import.meta.env.MODE === 'test'

export function isDevBuild(): boolean {
  return QTERM_DEV_BUILD
}
