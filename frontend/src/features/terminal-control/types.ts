export interface TerminalControlSession {
  id: string
  title: string
  address?: string
  username?: string
  tabId?: string
  tabTitle?: string
  ready: boolean
}

export type SyncStatus = 'active' | 'paused'

export interface SyncGroupView {
  id: string
  sessionIds: string[]
  status: SyncStatus
  revision: number
  lastSourceSessionId?: string
}

export type SyncTargetPickerMode = 'create' | 'manage'

export type BatchSendMode = 'raw' | 'append_cr' | 'each_line_cr'
export type BatchTargetScope = 'current' | 'all' | 'custom'

export interface BatchSendRequest {
  sessionIds: string[]
  content: string
  sendMode: BatchSendMode
  repeatCount: number
  intervalMs: number
  pauseSync: boolean
}

export type BatchTaskStatus = 'running' | 'completed' | 'partially_failed' | 'cancelled' | 'failed'

export interface BatchTaskResult {
  sessionId: string
  writtenRounds: number
  lastError?: string
}

export interface BatchTaskView {
  id: string
  status: BatchTaskStatus
  repeatCount: number
  results: BatchTaskResult[]
  sessionIds?: string[]
  completedRounds?: number
  error?: string
}

export interface BatchTargetSnapshot {
  id: string
  title: string
}

export interface BatchLogEntry {
  id: string
  content: string
  sendMode: BatchSendMode
  repeatCount: number
  intervalMs: number
  targets: BatchTargetSnapshot[]

  at: number
  status: BatchTaskStatus
  results: BatchTaskResult[]
  completedRounds: number
}

export type SyncAction = 'pause' | 'resume' | 'manage' | 'end'
