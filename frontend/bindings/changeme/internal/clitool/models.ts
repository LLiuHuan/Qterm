export interface Settings {
  execGuard: string
}

export interface State {
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

  settings: Settings

  hintKey?: string
}
