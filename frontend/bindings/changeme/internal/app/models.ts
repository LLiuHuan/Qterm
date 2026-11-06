// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import * as ai$0 from '../ai/models.js'
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import * as hostimport$0 from '../hostimport/models.js'

export interface AIBackgroundCommandResult {
  output: string
  exitCode: number
}

export interface AIChatInput {
  providerId: string
  type: ai$0.ProviderType
  baseUrl: string
  model: string
  messages: ai$0.Message[] | null

  contextText: string
}

export interface AIChatResult {
  reply: string

  errorKey?: string

  errorDetail?: string
}

export interface AIChatStreamPollResult {
  content: string

  reasoning: string
  usage?: AIUsage | null
  done: boolean
  errorKey?: string
  errorDetail?: string
}

export interface AIChatStreamResult {
  streamId: string
  errorKey?: string

  context: ai$0.ContextBreakdown
}

export interface AIModelListInput {
  providerId: string
  type: ai$0.ProviderType
  baseUrl: string
}

export interface AIProviderPreviewInput {
  providerId: string
  type: ai$0.ProviderType
  baseUrl: string
  model: string
  draftKey: string
}

export interface AIUsage {
  inputTokens?: number | null
  outputTokens?: number | null
  totalTokens?: number | null
  cacheReadTokens?: number | null
  cacheWriteTokens?: number | null
}

export interface AdHocSessionInput {
  protocol: string

  address: string
  username: string

  port: number

  password: string
}

export interface HostImportSourceInfo {
  id: string

  label: string

  defaultPath: string

  pathKind: string

  detected: boolean
}

export interface ImportHostsInput {
  entries: hostimport$0.Entry[] | null
}

export interface ImportHostsResult {
  created: number
  failed: number
}

export interface OpenSerialSessionInput {
  device: string

  baud: number
}

export interface RecordingSettings {
  configured: string

  effective: string

  default: string
}

export interface SerialPortInfo {
  name: string
}

export interface TrayMenuState {
  ready: boolean
  locked: boolean
  newLocalLabel: string
  showLabel: string
  settingsLabel: string
  quitLabel: string
}
