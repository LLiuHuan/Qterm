import {
  BatchSendMode as BackendMode,
  SyncAction as BackendAction,
  type ControlSnapshot,
  type BatchTaskState,
} from '../../bindings/changeme/internal/terminal/models'
import type { BatchSendRequest, BatchTaskView, SyncGroupView } from '../features/terminal-control/types'
import { hasWailsRuntime } from './runtime'

export interface TerminalControlState {
  error?: string
  group: SyncGroupView | null
  task: BatchTaskView | null
  sessions: Array<{ id: string }>
}

export interface TerminalSyncRequest {
  action: 'create' | 'members' | 'join' | 'leave' | 'pause' | 'resume' | 'stop'
  sessionIds: string[]
  revision: number
}

export interface TerminalControlGateway {
  get(): Promise<TerminalControlState>
  sync(request: TerminalSyncRequest): Promise<TerminalControlState>
  start(request: BatchSendRequest): Promise<BatchTaskView>
  cancel(taskId: string): Promise<BatchTaskView>
}

function taskView(task: BatchTaskState): BatchTaskView {
  const status = task.status === 'queued' ? 'running' : task.status
  return {
    id: task.id,
    status: status as BatchTaskView['status'],
    repeatCount: task.repeatCount,
    sessionIds: task.sessionIds ?? [],
    completedRounds: task.completedRounds,
    results: (task.targets ?? []).map((target) => ({
      sessionId: target.sessionId,
      writtenRounds: target.writtenRounds,
      lastError: target.error ? 'terminalControl.error' : undefined,
    })),
  }
}

function stateView(state: ControlSnapshot): TerminalControlState {
  const group = state.sync
  const tasks = state.tasks ?? []
  return {
    sessions: state.sessions ?? [],
    error: state.error,
    group:
      group && (group.status === 'active' || group.status === 'paused')
        ? { ...group, sessionIds: group.sessionIds ?? [], status: group.status }
        : null,
    task: tasks.length ? taskView(tasks[tasks.length - 1]!) : null,
  }
}

export function createTerminalControlGateway(): TerminalControlGateway {
  const bindings = () => import('../../bindings/changeme/internal/app/service')
  return {
    async get() {
      if (!hasWailsRuntime()) return { group: null, task: null, sessions: [] }
      return stateView(await (await bindings()).GetTerminalControl())
    },
    async sync(request) {
      return stateView(
        await (
          await bindings()
        ).SetTerminalSync({
          ...request,
          action: {
            create: BackendAction.SyncActionCreate,
            members: BackendAction.SyncActionMembers,
            join: BackendAction.SyncActionJoin,
            leave: BackendAction.SyncActionLeave,
            pause: BackendAction.SyncActionPause,
            resume: BackendAction.SyncActionResume,
            stop: BackendAction.SyncActionStop,
          }[request.action],
        }),
      )
    },
    async start(request) {
      return taskView(
        await (
          await bindings()
        ).StartTerminalBatch({
          ...request,
          sendMode: {
            raw: BackendMode.BatchSendRaw,
            append_cr: BackendMode.BatchSendAppendCR,
            each_line_cr: BackendMode.BatchSendEachLine,
          }[request.sendMode],
        }),
      )
    },
    async cancel(taskId) {
      return taskView(await (await bindings()).CancelTerminalBatch(taskId))
    },
  }
}
