import type { RemoteEditState, RemoteFileContentSnapshot } from '../../services/remote-edit'

export interface RemoteEditSnapshotLike {
  id: string
  sessionId: string
  remotePath: string
  localPath: string
  state: string
  baselineHash: string
  lastError: string

  hasUnsyncedChanges?: boolean
}

export interface RemoteEditView {
  id: string
  sessionId: string
  remotePath: string
  localPath: string
  state: RemoteEditState
  baselineHash: string
  lastError: string

  hasUnsyncedChanges: boolean

  conflict: boolean

  canSync: boolean

  canStop: boolean
}

export const REMOTE_EDIT_MAX_BYTES = 8 * 1024 * 1024

const EDIT_STATES: readonly RemoteEditState[] = ['watching', 'syncing', 'synced', 'error', 'stopped']

export function normalizeEditState(value: string): RemoteEditState {
  const state = (value ?? '') as RemoteEditState
  return EDIT_STATES.includes(state) ? state : 'error'
}

export function toRemoteEditView(snapshot: RemoteEditSnapshotLike): RemoteEditView {
  const state = normalizeEditState(snapshot.state)
  const conflict = state === 'error' && snapshot.lastError === 'remote.edit_conflict'
  return {
    id: snapshot.id,
    sessionId: snapshot.sessionId,
    remotePath: snapshot.remotePath,
    localPath: snapshot.localPath,
    state,
    baselineHash: snapshot.baselineHash ?? '',
    lastError: snapshot.lastError ?? '',
    hasUnsyncedChanges: snapshot.hasUnsyncedChanges === true,
    conflict,
    canSync: state === 'watching' || state === 'synced' || conflict,
    canStop: state !== 'stopped',
  }
}

export function isQuietEdit(view: Pick<RemoteEditView, 'state' | 'hasUnsyncedChanges'>): boolean {
  if (view.state === 'synced') return true
  return view.state === 'watching' && !view.hasUnsyncedChanges
}

export function remoteEditKey(sessionId: string, remotePath: string): string {
  return `${sessionId}\u0000${remotePath}`
}

export function editStateLocaleKey(view: Pick<RemoteEditView, 'state' | 'conflict'>): string {
  if (view.conflict) return 'remote.ui.state.conflict'
  switch (view.state) {
    case 'watching':
      return 'remote.ui.state.watching'
    case 'syncing':
      return 'remote.ui.state.syncing'
    case 'synced':
      return 'remote.ui.state.synced'
    case 'stopped':
      return 'remote.ui.state.stopped'
    case 'error':
    default:
      return 'remote.ui.state.error'
  }
}

export function editStateTone(
  view: Pick<RemoteEditView, 'state' | 'conflict'>,
): 'muted' | 'accent' | 'success' | 'danger' {
  if (view.conflict || view.state === 'error') return 'danger'
  if (view.state === 'syncing') return 'accent'
  if (view.state === 'synced') return 'success'
  return 'muted'
}

const KNOWN_EDIT_ERROR_KEYS: ReadonlySet<string> = new Set([
  'remote.edit_conflict',
  'remote.edit_watch_failed',
  'remote.edit_editor_failed',
  'remote.edit_not_found',
  'remote.edit_local_missing',
  'remote.edit_write_failed',
  'remote.edit_read_failed',
  'remote.edit_read_unsupported',
  'remote.edit_write_unsupported',
  'remote.edit_cache_unavailable',
  'remote.edit_local_write_failed',
  'remote.edit_file_too_large',
  'remote.edit_content_invalid',
  'sftp.remote_path_invalid',
  'sftp.download_regular_only',
  'sftp.stat_failed',
  'sftp.directory_permission_denied',
  'sftp.remote_path_not_found',
  'session.not_found',
  'session.disconnected',
  'error.serviceNotReady',
  'error.unknown',
])

export function isKnownEditErrorKey(key: string): boolean {
  return KNOWN_EDIT_ERROR_KEYS.has(key)
}

export function editReasonKey(view: Pick<RemoteEditView, 'lastError'>): string {
  return isKnownEditErrorKey(view.lastError) ? view.lastError : ''
}

export function currentEditorTheme(scope?: Element | null): 'light' | 'dark' {
  if (typeof document === 'undefined') return 'light'
  let node: Element | null = scope ?? document.documentElement
  while (node) {
    if (node.classList.contains('dark') || node.classList.contains('is-dark')) return 'dark'
    node = node.parentElement
  }
  return document.documentElement.classList.contains('dark') ? 'dark' : 'light'
}

export type RemoteTextRefusal = 'binary' | 'truncated' | 'too-large' | null

export function remoteTextRefusal(
  content: Pick<RemoteFileContentSnapshot, 'binary' | 'truncated' | 'size'>,
): RemoteTextRefusal {
  if (content.truncated) return 'truncated'
  if (content.binary) return 'binary'
  if (content.size > REMOTE_EDIT_MAX_BYTES) return 'too-large'
  return null
}

export function refusalLocaleKey(refusal: Exclude<RemoteTextRefusal, null>): string {
  switch (refusal) {
    case 'binary':
      return 'remote.ui.editor.unsupportedBinary'
    case 'truncated':
      return 'remote.ui.editor.unsupportedTruncated'
    case 'too-large':
      return 'remote.ui.editor.unsupportedTooLarge'
  }
}

export type ConflictChoice = 'reload' | 'overwrite' | 'cancel'

export type ConflictKind = 'external' | 'in-app'

export interface ConflictChoiceSpec {
  choice: ConflictChoice
  labelKey: string
  hintKey: string

  danger: boolean
}

export const CONFLICT_CHOICES: readonly ConflictChoiceSpec[] = [
  { choice: 'reload', labelKey: 'remote.ui.conflict.reload', hintKey: 'remote.ui.conflict.reloadHint', danger: false },
  {
    choice: 'overwrite',
    labelKey: 'remote.ui.conflict.overwrite',
    hintKey: 'remote.ui.conflict.overwriteHint',
    danger: true,
  },
  { choice: 'cancel', labelKey: 'remote.ui.conflict.cancel', hintKey: 'remote.ui.conflict.cancelHint', danger: false },
]

export function conflictShowsDiff(kind: ConflictKind): boolean {
  return kind === 'in-app'
}

function base64ToBytes(base64: string): Uint8Array {
  const binary = atob(base64)
  const bytes = new Uint8Array(binary.length)
  for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index)
  return bytes
}

function bytesToBase64(bytes: Uint8Array): string {
  const CHUNK = 0x8000
  let binary = ''
  for (let index = 0; index < bytes.length; index += CHUNK) {
    binary += String.fromCharCode(...bytes.subarray(index, index + CHUNK))
  }
  return btoa(binary)
}

export function decodeRemoteText(contentBase64: string): string {
  const bytes = base64ToBytes(contentBase64)
  return new TextDecoder('utf-8', { ignoreBOM: true }).decode(bytes)
}

export function encodeRemoteText(text: string): string {
  return bytesToBase64(new TextEncoder().encode(text))
}
