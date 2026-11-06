import buildConfig from '../../../build/config.yml?raw'

const versionMatch = buildConfig.match(/^\s{2}version:\s*["']?([^"'\s#]+)/m)

export const QTERM_VERSION = versionMatch?.[1] ?? 'dev'
