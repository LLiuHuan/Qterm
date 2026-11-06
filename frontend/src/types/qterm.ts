import type { GroupRecord, HostRecord } from '../services/hosts'
import type {
  HostKeyChallenge,
  SessionAuthenticationChallenge,
  SessionCloseReason,
  SessionKind,
  SessionState,
} from '../services/sessions'

export type ToolPanel =
  | 'resources'
  | 'snippets'
  | 'sftp'
  | 'tunnels'
  | 'otp'
  | 'processes'
  | 'services'
  | 'history'
  | 'themes'
  | 'docker'
  | 'ai'

export type HostAuthMode = 'password' | 'manual' | 'credential' | 'none'

export interface SessionTab {
  id: string

  locked?: boolean

  paneKey: string
  hostId: string

  address?: string

  kind: SessionKind
  title: string
  state: SessionState
  streamToken: string
  streamState: 'idle' | 'opening' | 'ready' | 'closed' | 'failed'
  error: string
  latencyMs?: number | null

  reconnectInFlight?: boolean

  attachEpoch?: number

  autoReconnectPending?: boolean

  autoReconnectFailures?: number

  autoReconnectGivenUp?: boolean

  autoReconnectSuppressed?: boolean

  closeReason?: SessionCloseReason

  exitCode?: number

  order?: number
}

export interface PendingDelete {
  kind: 'host' | 'group'
  id: string
  name: string
}

export interface GroupedHosts {
  id: string
  name: string
  sortOrder: number
  items: HostRecord[]
  children?: GroupedHosts[]
  entries?: Array<{ kind: 'host'; value: HostRecord } | { kind: 'group'; value: GroupedHosts }>
}

export interface SessionStatus {
  label: string
  tone: string
}

export interface HostKeyPending {
  sessionId: string

  challenge: HostKeyChallenge

  challenges: HostKeyChallenge[]
}

export interface SessionAuthenticationPending {
  sessionId: string
  challenge: SessionAuthenticationChallenge
}

export type StreamStatusEvent = {
  sessionId: string
  state: 'ready' | 'closed' | 'failed'
  message?: string
}

export type AssetGroup = GroupRecord & { items?: HostRecord[] }
