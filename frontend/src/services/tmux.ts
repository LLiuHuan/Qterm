import {
  ListTmuxSessions,
  CreateTmuxSession,
  KillTmuxSession,
  RenameTmuxSession,
  TmuxAttachCommand,
  TmuxDetachKeys,
  TmuxPrefixKeys,
  TmuxVersion,
} from '../../bindings/changeme/internal/app/service'
import type { Session as BackendTmuxSession } from '../../bindings/changeme/internal/tmux/models'

export interface TmuxSession {
  name: string
  windows: number
  createdAt: number
  attached: boolean
}

export interface TmuxGateway {
  list(sessionId: string): Promise<TmuxSession[]>
  version(sessionId: string): Promise<string>
  create(sessionId: string, name: string): Promise<void>
  rename(sessionId: string, name: string, newName: string): Promise<void>
  kill(sessionId: string, name: string): Promise<void>

  attachCommand(name: string, trackingToken?: string): Promise<string>

  detachKeys(sessionId: string): Promise<string>

  prefixKeys?(sessionId: string): Promise<string>
}

function toSession(raw: BackendTmuxSession): TmuxSession {
  return {
    name: raw.name ?? '',
    windows: raw.windows ?? 0,
    createdAt: raw.createdAt ?? 0,
    attached: Boolean(raw.attached),
  }
}

export function createTmuxGateway(): TmuxGateway {
  return {
    async list(sessionId: string) {
      const rows = await ListTmuxSessions(sessionId)
      return (rows ?? []).map((row) => toSession(row as BackendTmuxSession))
    },
    async version(sessionId: string) {
      return TmuxVersion(sessionId)
    },
    async create(sessionId: string, name: string) {
      await CreateTmuxSession(sessionId, name)
    },
    async rename(sessionId: string, name: string, newName: string) {
      await RenameTmuxSession(sessionId, name, newName)
    },
    async kill(sessionId: string, name: string) {
      await KillTmuxSession(sessionId, name)
    },
    async attachCommand(name: string, trackingToken = '') {
      return TmuxAttachCommand(name, trackingToken)
    },
    async detachKeys(sessionId: string) {
      return TmuxDetachKeys(sessionId)
    },
    async prefixKeys(sessionId: string) {
      return TmuxPrefixKeys(sessionId)
    },
  }
}
