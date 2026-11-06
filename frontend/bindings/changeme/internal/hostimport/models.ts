export interface Entry {
  alias: string

  hostName: string
  user: string
  port: number

  identityFile?: string

  line: number

  protocol?: string

  serialBaud?: number
  serialDataBits?: number
  serialParity?: string
  serialStopBits?: string

  secret?: string

  credentialId?: string

  group?: string

  password?: string

  passwordSource?: string

  duplicate: boolean
}

export interface Result {
  entries: Entry[] | null

  skipped: number
}
