export interface BatchRequest {
  sessionIds: string[] | null
  content: string
  sendMode: BatchSendMode
  repeatCount: number
  intervalMs: number
  pauseSync?: boolean
}

export enum BatchSendMode {
  $zero = '',

  BatchSendRaw = 'raw',
  BatchSendAppendCR = 'append_cr',
  BatchSendEachLine = 'each_line_cr',
}

export interface BatchTargetState {
  sessionId: string
  writtenRounds: number
  status: string
  error?: string
}

export interface BatchTaskState {
  id: string
  windowId: string
  sessionIds: string[] | null
  sendMode: BatchSendMode
  repeatCount: number
  intervalMs: number
  completedRounds: number
  status: BatchTaskStatus
  targets: BatchTargetState[] | null
}

export enum BatchTaskStatus {
  $zero = '',

  BatchTaskQueued = 'queued',
  BatchTaskRunning = 'running',
  BatchTaskCompleted = 'completed',
  BatchTaskPartiallyFailed = 'partially_failed',
  BatchTaskFailed = 'failed',
  BatchTaskCancelled = 'cancelled',
}

export interface ControlSnapshot {
  sessions: SessionInfo[] | null
  sync?: SyncGroupState | null
  tasks: BatchTaskState[] | null
  error?: string
}

export interface SessionInfo {
  id: string
  hostId: string
  kind: string
  state: string
  purpose: string
}

export enum SyncAction {
  $zero = '',

  SyncActionCreate = 'create',
  SyncActionMembers = 'members',
  SyncActionJoin = 'join',
  SyncActionLeave = 'leave',
  SyncActionPause = 'pause',
  SyncActionResume = 'resume',
  SyncActionStop = 'stop',
}

export interface SyncGroupState {
  id: string
  windowId: string
  sessionIds: string[] | null
  status: SyncStatus
  revision: number
  lastSourceSessionId?: string
  error?: string
}

export interface SyncRequest {
  action: SyncAction
  sessionIds?: string[] | null
  revision?: number
}

export enum SyncStatus {
  $zero = '',

  SyncStatusActive = 'active',
  SyncStatusPaused = 'paused',
  SyncStatusStopped = 'stopped',
}
