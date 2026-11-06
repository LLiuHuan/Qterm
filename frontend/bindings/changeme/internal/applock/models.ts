export interface Status {
  enabled: boolean

  hasPassword: boolean

  autoLockMinutes: number

  hasRecovery: boolean
  recoveryQuestion: string
}
