export interface Result {
  available: boolean

  display: string

  xauthPath: string

  authorityFile: string

  authProtocol: string

  screen: number

  reason: string

  notice: string

  detail: string
}

export interface Settings {
  display: string

  xauthPath: string

  authorityFile: string
}
