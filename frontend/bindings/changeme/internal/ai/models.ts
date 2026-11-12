// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import * as json$0 from '../../../encoding/json/models.js'

export interface AgentAuditEntry {
  runId: string
  stepId: string
  callId?: string

  sessionId: string

  command: string

  risk: string

  decision: string
  state: string

  exitCode?: number | null
  startedAt: string
  finishedAt?: string | null
  durationMs?: number
}

export interface AgentEvent {
  cursor: number
  runId: string

  kind: string
  stepId?: string
  state: string
  at: string
}

export interface AgentMessage {
  role: string
  content: string

  toolCalls?: ToolCall[] | null

  toolCallId?: string
  name?: string
}

export interface AgentRun {
  id: string
  providerId: string
  model: string
  sessionId: string
  prompt: string
  policyVersion: string
  state: RunState
  steps: AgentStep[] | null

  draft?: string

  reasoningDraft?: string

  modelCalls?: number
  commandRuns?: number

  reasonings?: string[] | null

  notes?: string[] | null

  noteDurations?: number[] | null

  usage?: Usage | null

  allowedTools?: string[] | null

  context?: ContextBreakdown | null

  errorKey?: string
  errorDetail?: string
  createdAt: string
  updatedAt: string
}

export interface AgentSnapshot {
  run: AgentRun

  events: AgentEvent[] | null

  cursor: number

  earliestCursor: number
  truncated: boolean
}

export interface AgentStep {
  id: string
  index: number
  callId?: string
  kind: StepKind
  state: StepState
  title?: string
  command?: string

  args?: json$0.RawMessage

  preview?: ToolPreview | null
  targetSessionId?: string
  risk?: string
  riskReason?: string
  observation?: string
  observationTruncated?: boolean

  exitCode?: number | null

  pendingId?: string

  errorKey?: string
  errorDetail?: string
  startedAt: string
  finishedAt?: string | null
}

export enum ApprovalMode {
  $zero = '',

  ApprovalStrict = 'strict',
  ApprovalBalanced = 'balanced',
  ApprovalReview = 'review',
  ApprovalAutomatic = 'automatic',
}

export interface ContextBreakdown {
  system: number
  tools: number
  messages: number
  context: number

  limit: number
}

export interface DecideInput {
  RunID: string
  StepID: string
  PendingID: string
  Approve: boolean
  SessionID: string
  PolicyVersion: string

  Choice: string

  Remember: boolean

  Since: number
}

export interface Message {
  role: string
  content: string

  images?: string[] | null
}

export enum ProviderType {
  $zero = '',

  ProviderTypeOpenAI = 'openai',
  ProviderTypeOpenAIResponse = 'openai-response',
  ProviderTypeAnthropic = 'anthropic',
}

export enum RunState {
  $zero = '',

  RunRunning = 'running',

  RunWaitingApproval = 'waiting_approval',

  RunCompleted = 'completed',

  RunRejected = 'rejected',

  RunFailed = 'failed',

  RunCancelled = 'cancelled',

  RunTimedOut = 'timed_out',

  RunBlocked = 'blocked',
}

export interface StartInput {
  ProviderID: string

  ProviderType: ProviderType
  BaseURL: string
  Model: string

  SessionID: string

  hostIds?: string[] | null

  sessionKind?: string

  targetLabel?: string

  Prompt: string

  PolicyVersion: string

  ApprovalMode: ApprovalMode

  History: AgentMessage[] | null

  ContextText: string

  ExecutionChannel: string

  MaxSteps: number

  ModelTimeoutSeconds: number
  StepTimeoutSeconds: number
}

export enum StepKind {
  $zero = '',

  StepKindCommand = 'execute_command',

  StepKindAnswer = 'final_answer',

  StepKindListHosts = 'list_hosts',

  StepKindGetHost = 'get_host',

  StepKindSftpList = 'sftp_list',

  StepKindSftpReadFile = 'sftp_read_file',

  StepKindSftpWriteFile = 'sftp_write_file',

  StepKindSftpUploadFile = 'sftp_upload_file',

  StepKindSftpDownloadFile = 'sftp_download_file',

  StepKindLocalListDir = 'local_list_dir',

  StepKindLocalReadFile = 'local_read_file',

  StepKindWebFetch = 'web_fetch',

  StepKindAskUser = 'ask_user',
}

export enum StepState {
  $zero = '',

  StepRunning = 'running',
  StepWaitingApproval = 'waiting_approval',
  StepCompleted = 'completed',
  StepFailed = 'failed',
  StepCancelled = 'cancelled',
  StepTimedOut = 'timed_out',
  StepRejected = 'rejected',
}

export interface ToolCall {
  id: string
  name: string
  arguments: string
}

export interface ToolPreview {
  kind: string
  data?: json$0.RawMessage
}

export interface Usage {
  inputTokens?: number | null
  outputTokens?: number | null
  totalTokens?: number | null
  cacheReadTokens?: number | null
  cacheWriteTokens?: number | null
}
