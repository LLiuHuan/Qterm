import { computed, reactive, ref, shallowRef, type ComputedRef } from 'vue'
import { decodeAppError } from '../../services/errors'
import {
  createRemoteEditGateway,
  type RemoteEditChangedPayload,
  type RemoteEditGateway,
} from '../../services/remote-edit'
import {
  decodeRemoteText,
  editReasonKey,
  isKnownEditErrorKey,
  isQuietEdit,
  remoteEditKey,
  remoteTextRefusal,
  toRemoteEditView,
  type ConflictKind,
  type RemoteEditSnapshotLike,
  type RemoteEditView,
  type RemoteTextRefusal,
} from './remoteEditModel'

export interface RemoteEditActionResult {
  ok: boolean
  errorKey: string

  conflict: boolean
}

export interface RemoteEditConflictRequest {
  kind: ConflictKind
  editId: string
  sessionId: string
  remotePath: string

  remoteContent?: string

  localContent?: string

  currentHash?: string
}

export interface RemoteTextEditorRequest {
  sessionId: string
  remotePath: string

  name: string
}

const edits = reactive(new Map<string, RemoteEditView>())
const listLoaded = ref(false)
const listLoading = ref(false)
const conflict = shallowRef<RemoteEditConflictRequest | null>(null)
const editorRequest = shallowRef<RemoteTextEditorRequest | null>(null)

const lastSynced = shallowRef<{ sessionId: string; remotePath: string; nonce: number } | null>(null)
let syncedNonce = 0

function noteSynced(sessionId: string, remotePath: string): void {
  syncedNonce += 1
  lastSynced.value = { sessionId, remotePath, nonce: syncedNonce }
}

let gateway: RemoteEditGateway | null = null
let unsubscribe: (() => void) | null = null
let loadPromise: Promise<void> | null = null

function currentGateway(): RemoteEditGateway {
  if (!gateway) gateway = createRemoteEditGateway()
  return gateway
}

export function __setRemoteEditGatewayForTest(next: RemoteEditGateway | null): void {
  gateway = next
}

export const EDIT_DOT_GRACE_MS = 4000

const hiddenDots = reactive(new Set<string>())
const dotTimers = new Map<string, ReturnType<typeof setTimeout>>()

function clearDotTimer(key: string): void {
  const timer = dotTimers.get(key)
  if (timer === undefined) return
  clearTimeout(timer)
  dotTimers.delete(key)
}

function scheduleDotVisibility(view: RemoteEditView): void {
  const key = remoteEditKey(view.sessionId, view.remotePath)
  clearDotTimer(key)
  if (!isQuietEdit(view)) {
    hiddenDots.delete(key)
    return
  }
  dotTimers.set(
    key,
    setTimeout(() => {
      dotTimers.delete(key)
      hiddenDots.add(key)
    }, EDIT_DOT_GRACE_MS),
  )
}

function forgetDotVisibility(key: string): void {
  clearDotTimer(key)
  hiddenDots.delete(key)
}

function resultFromError(error: unknown): RemoteEditActionResult {
  const appError = decodeAppError(error)
  const key = appError && isKnownEditErrorKey(appError.messageKey) ? appError.messageKey : 'error.unknown'
  return { ok: false, errorKey: key, conflict: false }
}

function store(view: RemoteEditView): RemoteEditView {
  edits.set(remoteEditKey(view.sessionId, view.remotePath), view)
  scheduleDotVisibility(view)
  return view
}

function findById(editId: string): RemoteEditView | null {
  for (const view of edits.values()) if (view.id === editId) return view
  return null
}

function dropEdit(view: RemoteEditView): void {
  edits.delete(remoteEditKey(view.sessionId, view.remotePath))
  forgetDotVisibility(remoteEditKey(view.sessionId, view.remotePath))

  if (conflict.value?.editId === view.id) conflict.value = null
}

function snapshotOf(view: RemoteEditView, overrides: Partial<RemoteEditSnapshotLike>): RemoteEditSnapshotLike {
  return {
    id: view.id,
    sessionId: view.sessionId,
    remotePath: view.remotePath,
    localPath: view.localPath,
    state: view.state,
    baselineHash: view.baselineHash,
    lastError: view.lastError,

    hasUnsyncedChanges: view.hasUnsyncedChanges,
    ...overrides,
  }
}

function handleChanged(payload: RemoteEditChangedPayload): void {
  const existing = findById(payload.editID)
  if (!existing) {
    void refreshRemoteEdits()
    return
  }
  const next = toRemoteEditView(
    snapshotOf(existing, {
      state: payload.state,
      lastError: payload.lastError || existing.lastError,

      hasUnsyncedChanges:
        typeof payload.hasUnsyncedChanges === 'boolean' ? payload.hasUnsyncedChanges : existing.hasUnsyncedChanges,
    }),
  )
  if (next.state === 'stopped') {
    dropEdit(existing)
    return
  }
  store(next)
  if (next.conflict && conflict.value?.editId !== next.id) {
    conflict.value = {
      kind: 'external',
      editId: next.id,
      sessionId: next.sessionId,
      remotePath: next.remotePath,
    }
  }
}

export async function ensureRemoteEditsLoaded(): Promise<void> {
  if (!unsubscribe) {
    const { subscribeRemoteEditChanges } = await import('../../services/remote-edit-events')

    if (!unsubscribe) unsubscribe = subscribeRemoteEditChanges(handleChanged)
  }
  await refreshRemoteEdits()
}

export async function refreshRemoteEdits(): Promise<void> {
  if (loadPromise) return loadPromise
  listLoading.value = true
  loadPromise = (async () => {
    try {
      const snapshots = await currentGateway().listLocalEdits()
      const next = new Map<string, RemoteEditView>()
      for (const snapshot of snapshots) {
        if (!snapshot?.id) continue
        const view = toRemoteEditView(snapshot)
        if (view.state === 'stopped') continue
        next.set(remoteEditKey(view.sessionId, view.remotePath), view)
      }
      edits.clear()

      for (const key of [...dotTimers.keys()]) clearDotTimer(key)
      hiddenDots.clear()
      for (const [key, view] of next) {
        edits.set(key, view)
        scheduleDotVisibility(view)
      }
      listLoaded.value = true
    } catch {
    } finally {
      listLoading.value = false
      loadPromise = null
    }
  })()
  return loadPromise
}

export function handleRemoteEditChangedForTest(payload: RemoteEditChangedPayload): void {
  handleChanged(payload)
}

export function __resetRemoteEditsForTest(): void {
  unsubscribe?.()
  unsubscribe = null
  for (const key of [...dotTimers.keys()]) clearDotTimer(key)
  hiddenDots.clear()
  edits.clear()
  conflict.value = null
  editorRequest.value = null
  lastSynced.value = null
  syncedNonce = 0
  listLoaded.value = false
  listLoading.value = false
  loadPromise = null
}

export async function openWithLocalProgram(sessionId: string, remotePath: string): Promise<RemoteEditActionResult> {
  if (!sessionId) return { ok: false, errorKey: 'session.not_found', conflict: false }
  try {
    const snapshot = await currentGateway().openLocalEdit(sessionId, remotePath)
    store(toRemoteEditView(snapshot))
    return { ok: true, errorKey: '', conflict: false }
  } catch (error) {
    return resultFromError(error)
  }
}

export async function syncRemoteEdit(editId: string): Promise<RemoteEditActionResult> {
  try {
    const snapshot = await currentGateway().syncLocalEdit(editId)
    const view = store(toRemoteEditView(snapshot))
    if (!view.conflict && !view.lastError) noteSynced(view.sessionId, view.remotePath)
    if (view.conflict) {
      conflict.value = {
        kind: 'external',
        editId: view.id,
        sessionId: view.sessionId,
        remotePath: view.remotePath,
      }
      return { ok: false, errorKey: '', conflict: true }
    }

    if (conflict.value?.editId === view.id) conflict.value = null
    return { ok: true, errorKey: '', conflict: false }
  } catch (error) {
    return resultFromError(error)
  }
}

export async function stopRemoteEdit(editId: string): Promise<RemoteEditActionResult> {
  const existing = findById(editId)
  try {
    await currentGateway().discardLocalEdit(editId)
    if (existing) dropEdit(existing)
    return { ok: true, errorKey: '', conflict: false }
  } catch (error) {
    return resultFromError(error)
  }
}

export async function reloadExternalEdit(
  editId: string,
  sessionId: string,
  remotePath: string,
): Promise<RemoteEditActionResult> {
  const stopped = await stopRemoteEdit(editId)
  if (!stopped.ok) return stopped
  return await openWithLocalProgram(sessionId, remotePath)
}

export async function reloadRemoteEditForConflict(request: RemoteEditConflictRequest): Promise<RemoteEditActionResult> {
  if (request.kind === 'external') {
    return await reloadExternalEdit(request.editId, request.sessionId, request.remotePath)
  }

  return { ok: true, errorKey: '', conflict: false }
}

export function dismissRemoteEditConflict(): void {
  conflict.value = null
}

export function requestRemoteTextEditor(sessionId: string, remotePath: string): void {
  if (!sessionId) return
  editorRequest.value = { sessionId, remotePath, name: remotePath.split('/').filter(Boolean).at(-1) ?? remotePath }
}

export function closeRemoteTextEditor(): void {
  editorRequest.value = null
}

export async function readRemoteText(
  sessionId: string,
  remotePath: string,
): Promise<
  | { ok: true; content: string; hash: string; size: number; modTime: string }
  | { ok: false; refusal: RemoteTextRefusal; errorKey: string }
> {
  try {
    const snapshot = await currentGateway().readRemoteFileText(sessionId, remotePath)
    const refusal = remoteTextRefusal(snapshot)
    if (refusal) return { ok: false, refusal, errorKey: '' }
    return {
      ok: true,
      content: decodeRemoteText(snapshot.contentBase64),
      hash: snapshot.hash,
      size: snapshot.size,
      modTime: snapshot.modTime,
    }
  } catch (error) {
    const result = resultFromError(error)
    return { ok: false, refusal: null, errorKey: result.errorKey }
  }
}

export async function writeRemoteText(
  sessionId: string,
  remotePath: string,
  contentBase64: string,
  expectedHash: string,
): Promise<
  | { ok: true; currentHash: string }
  | { ok: false; conflict: true; currentHash: string }
  | { ok: false; conflict: false; errorKey: string }
> {
  try {
    const snapshot = await currentGateway().writeRemoteFileText(sessionId, remotePath, contentBase64, expectedHash)
    if (snapshot.conflict) {
      return { ok: false, conflict: true, currentHash: snapshot.currentHash }
    }
    noteSynced(sessionId, remotePath)
    return { ok: true, currentHash: snapshot.currentHash }
  } catch (error) {
    return { ok: false, conflict: false, errorKey: resultFromError(error).errorKey }
  }
}

export async function readRemoteContentForDiff(
  sessionId: string,
  remotePath: string,
): Promise<{ content: string; hash: string } | null> {
  const result = await readRemoteText(sessionId, remotePath)
  return result.ok ? { content: result.content, hash: result.hash } : null
}

export interface UseRemoteEdits {
  edits: ComputedRef<RemoteEditView[]>

  conflict: ComputedRef<RemoteEditConflictRequest | null>

  editorRequest: ComputedRef<RemoteTextEditorRequest | null>

  lastSynced: ComputedRef<{ sessionId: string; remotePath: string; nonce: number } | null>
  listLoaded: ComputedRef<boolean>
  listLoading: ComputedRef<boolean>

  editFor(sessionId: string, remotePath: string): RemoteEditView | null

  dotHiddenFor(view: RemoteEditView): boolean

  reasonKeyFor(view: RemoteEditView): string
}

export function useRemoteEdits(): UseRemoteEdits {
  const list = computed(() => [...edits.values()])
  return {
    edits: list,
    conflict: computed(() => conflict.value),
    editorRequest: computed(() => editorRequest.value),
    lastSynced: computed(() => lastSynced.value),
    listLoaded: computed(() => listLoaded.value),
    listLoading: computed(() => listLoading.value),
    editFor(sessionId: string, remotePath: string) {
      if (!sessionId) return null
      return edits.get(remoteEditKey(sessionId, remotePath)) ?? null
    },
    dotHiddenFor(view: RemoteEditView) {
      return hiddenDots.has(remoteEditKey(view.sessionId, view.remotePath))
    },
    reasonKeyFor: editReasonKey,
  }
}
