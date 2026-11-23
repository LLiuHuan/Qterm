import { hasWailsRuntime } from './runtime'

export interface CliToolState {
  platform: string
  supported: boolean
  binaryPath: string
  binaryAvailable: boolean
  installDir: string
  linkPath: string
  installed: boolean

  removable: boolean
  onPath: boolean
  skillsPath: string
  skillsInstalled: boolean
  settings: CliToolSettings

  hintKey?: string
}

export type CliExecGuard = 'off' | 'rules' | 'strict'

export interface CliToolSettings {
  execGuard: CliExecGuard
}

function requireRuntime(): void {
  if (!hasWailsRuntime()) {
    throw new Error('cliTool.unavailable')
  }
}

function normalizeGuard(value: unknown): CliExecGuard {
  return value === 'off' || value === 'strict' ? value : 'rules'
}

function normalizeState(state: {
  platform?: string
  supported?: boolean
  binaryPath?: string
  binaryAvailable?: boolean
  installDir?: string
  linkPath?: string
  installed?: boolean
  removable?: boolean
  onPath?: boolean
  skillsPath?: string
  skillsInstalled?: boolean
  settings?: { execGuard?: string }
  hintKey?: string
}): CliToolState {
  return {
    platform: state.platform ?? '',
    supported: state.supported === true,
    binaryPath: state.binaryPath ?? '',
    binaryAvailable: state.binaryAvailable === true,
    installDir: state.installDir ?? '',
    linkPath: state.linkPath ?? '',
    installed: state.installed === true,
    removable: state.removable === true,
    onPath: state.onPath === true,
    skillsPath: state.skillsPath ?? '',
    skillsInstalled: state.skillsInstalled === true,
    settings: { execGuard: normalizeGuard(state.settings?.execGuard) },
    hintKey: state.hintKey ?? '',
  }
}

export async function getCliToolState(): Promise<CliToolState> {
  requireRuntime()
  const { CLIToolState } = await import('../../bindings/changeme/internal/app/service')
  return normalizeState(await CLIToolState())
}

export async function installCliToolBinary(): Promise<CliToolState> {
  requireRuntime()
  const { InstallCLIToolBinary } = await import('../../bindings/changeme/internal/app/service')
  return normalizeState(await InstallCLIToolBinary())
}

export async function uninstallCliToolBinary(): Promise<CliToolState> {
  requireRuntime()
  const { UninstallCLIToolBinary } = await import('../../bindings/changeme/internal/app/service')
  return normalizeState(await UninstallCLIToolBinary())
}

export async function installCliToolSkills(): Promise<CliToolState> {
  requireRuntime()
  const { InstallCLIToolSkills } = await import('../../bindings/changeme/internal/app/service')
  return normalizeState(await InstallCLIToolSkills())
}

export async function uninstallCliToolSkills(): Promise<CliToolState> {
  requireRuntime()
  const { UninstallCLIToolSkills } = await import('../../bindings/changeme/internal/app/service')
  return normalizeState(await UninstallCLIToolSkills())
}

export async function applyCliToolSettings(settings: CliToolSettings): Promise<CliToolState> {
  requireRuntime()
  const { ApplyCLIToolSettings } = await import('../../bindings/changeme/internal/app/service')
  return normalizeState(await ApplyCLIToolSettings({ execGuard: settings.execGuard }))
}
