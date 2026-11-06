// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import * as model$0 from '../model/models.js'

export interface AuthenticationChallenge {
  sessionId: string
  address: string
  username: string
  requestId: string
  kind: string
  hostId?: string
  instruction?: string
  questions?: AuthenticationQuestion[] | null

  hopIndex?: number
  hopHostId?: string
}

export interface AuthenticationInput {
  secret: string
  credentialId: string
  remember: boolean
  requestId?: string
  answers?: string[] | null
}

export interface AuthenticationQuestion {
  prompt: string
  echo: boolean
}

export interface DetectIconInput {
  hostId?: string
  protocol?: string
  address?: string
  port?: number
  username?: string
  credentialId?: string
  kind?: model$0.CredentialKind
  secret?: string
}

export interface OpenSessionResult {
  session: model$0.Session
  streamToken: string

  warnings?: string[] | null
}

export interface TestConnectionInput {
  hostId?: string
  protocol: string
  address: string
  port: number
  username: string
  credentialId: string
  kind: model$0.CredentialKind
  secret: string
}
