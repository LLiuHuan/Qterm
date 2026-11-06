import type { SftpGateway } from './sftp'

export type FollowResult = 'present' | 'missing' | 'unsupported'

interface SessionDirectoryState {
  followEnabled: boolean
  followedDirectory: string
  browserDirectory: string
}

const sessionDirectoryStates = new Map<string, SessionDirectoryState>()

function sessionDirectoryState(sessionId: string): SessionDirectoryState {
  const existing = sessionDirectoryStates.get(sessionId)
  if (existing) return existing
  const created = { followEnabled: true, followedDirectory: '', browserDirectory: '' }
  sessionDirectoryStates.set(sessionId, created)
  return created
}

export function rememberSessionFollowDirectory(sessionId: string, directory: string): void {
  if (!sessionId || !directory) return
  sessionDirectoryState(sessionId).followedDirectory = directory
}

export function readSessionFollowDirectory(sessionId: string): string {
  return sessionDirectoryStates.get(sessionId)?.followedDirectory ?? ''
}

export function rememberSessionBrowserDirectory(sessionId: string, directory: string): void {
  if (!sessionId || !directory) return
  sessionDirectoryState(sessionId).browserDirectory = directory
}

export function readSessionBrowserDirectory(sessionId: string): string {
  return sessionDirectoryStates.get(sessionId)?.browserDirectory ?? ''
}

export function setSessionDirectoryFollowEnabled(sessionId: string, enabled: boolean): void {
  if (!sessionId) return
  sessionDirectoryState(sessionId).followEnabled = enabled
}

export function readSessionDirectoryFollowEnabled(sessionId: string): boolean {
  return sessionDirectoryStates.get(sessionId)?.followEnabled ?? true
}

export function clearSessionDirectoryState(sessionId: string): void {
  sessionDirectoryStates.delete(sessionId)
}

export async function requestDirectoryFollow(
  gateway: SftpGateway,
  sessionId: string,
  action: 'probe' | 'configure',
  signal: AbortSignal,
): Promise<FollowResult> {
  if (signal.aborted) throw new Error('目录跟随操作已取消')
  const result = await gateway.directoryFollow(sessionId, action === 'configure', signal)
  if (signal.aborted) throw new Error('目录跟随操作已取消')
  if (result !== 'present' && result !== 'missing' && result !== 'unsupported')
    throw new Error('目录跟随检测返回无效状态')
  return result
}
