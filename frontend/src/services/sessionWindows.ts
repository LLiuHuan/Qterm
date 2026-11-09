import type { SessionKind } from './sessions'
import { evenPixel } from '../lib/geometry'
import { Events } from '@wailsio/runtime'
import { hasWailsRuntime } from './runtime'

export const SETTINGS_WINDOW_STATE_EVENT = 'qterm:settings-window-state'

export const HOST_EDITOR_TARGET_EVENT = 'qterm:host-editor-target'

export type DialogWindowKind = 'settings' | 'hostEditor' | 'import' | 'export'

export interface SettingsWindowState {
  open: boolean

  kind: DialogWindowKind

  category: string

  search?: string
}

export function connectSettingsWindowState(handler: (state: SettingsWindowState) => void): () => void {
  try {
    return Events.On(SETTINGS_WINDOW_STATE_EVENT, (event: { data?: unknown }) => {
      const payload = event?.data as { open?: unknown; kind?: unknown; category?: unknown; search?: unknown } | null
      handler({
        open: payload?.open === true,
        kind: (typeof payload?.kind === 'string' ? payload.kind : 'settings') as DialogWindowKind,
        category: typeof payload?.category === 'string' ? payload.category : '',
        search: typeof payload?.search === 'string' ? payload.search : '',
      })
    })
  } catch {
    return () => {}
  }
}

export function connectHostEditorTarget(handler: (hostID: string) => void): () => void {
  try {
    return Events.On(HOST_EDITOR_TARGET_EVENT, (event: { data?: unknown }) => {
      const payload = event?.data as { hostID?: unknown } | null
      handler(typeof payload?.hostID === 'string' ? payload.hostID : '')
    })
  } catch {
    return () => {}
  }
}

export const HOSTS_CHANGED_EVENT = 'qterm:hosts-changed'

export async function broadcastHostsChanged(): Promise<void> {
  try {
    await Events.Emit(HOSTS_CHANGED_EVENT, {})
  } catch {}
}

export function connectHostsChanged(handler: () => void): () => void {
  try {
    return Events.On(HOSTS_CHANGED_EVENT, () => handler())
  } catch {
    return () => {}
  }
}

export async function setSettingsWindowEnabled(enabled: boolean): Promise<void> {
  if (typeof window === 'undefined' || !hasWailsRuntime()) return
  const { SetSettingsWindowEnabled } = await import('../../bindings/changeme/internal/app/service')
  await SetSettingsWindowEnabled(enabled)
}

export async function hideSettingsWindow(): Promise<void> {
  if (typeof window === 'undefined' || !hasWailsRuntime()) return
  const { HideSettingsWindow } = await import('../../bindings/changeme/internal/app/service')
  await HideSettingsWindow()
}

export async function focusSettingsWindowInWindow(): Promise<void> {
  if (typeof window === 'undefined' || !hasWailsRuntime()) return
  const { FocusAuxiliaryWindow } = await import('../../bindings/changeme/internal/app/service')
  try {
    await FocusAuxiliaryWindow()
  } catch {}
}

export async function settingsWindowVisible(): Promise<boolean> {
  if (typeof window === 'undefined' || !hasWailsRuntime()) return false
  const { SettingsWindowVisible } = await import('../../bindings/changeme/internal/app/service')
  try {
    return await SettingsWindowVisible()
  } catch {
    return false
  }
}

export function isSettingsWindowMode(): boolean {
  if (typeof window === 'undefined') return false
  return new URLSearchParams(window.location.search).get('settingsWindow') === '1'
}

export function isHostEditorWindowMode(): boolean {
  if (typeof window === 'undefined') return false
  return new URLSearchParams(window.location.search).get('hostEditorWindow') === '1'
}

export async function openHostEditorWindow(hostID: string): Promise<void> {
  if (hasWailsRuntime()) {
    const { OpenHostEditorWindow } = await import('../../bindings/changeme/internal/app/service')
    await OpenHostEditorWindow(hostID)
    return
  }

  const target = new URL(window.location.href)
  target.search = ''
  target.searchParams.set('hostEditorWindow', '1')
  if (hostID) target.searchParams.set('hostID', hostID)
  const child = window.open(target.href, '_blank')
  if (!child) throw new Error('主机编辑器窗口被浏览器拦截')
  child.opener = null
}

export async function closeHostEditorWindow(): Promise<void> {
  if (typeof window === 'undefined' || !hasWailsRuntime()) return
  const { CloseHostEditorWindow } = await import('../../bindings/changeme/internal/app/service')
  await CloseHostEditorWindow()
}

export async function openSettingsWindow(title: string, category?: string, search?: string): Promise<void> {
  await openDialogWindow('settings', title, category ?? '', true, search ?? '')
}

export async function openDialogWindow(
  kind: DialogWindowKind,
  title: string,
  arg = '',
  resizable = kind === 'settings',
  search = '',
): Promise<void> {
  if (hasWailsRuntime()) {
    const { OpenSettingsWindow } = await import('../../bindings/changeme/internal/app/service')
    await OpenSettingsWindow(kind, title, arg, search, resizable)
    return
  }

  const target = new URL(window.location.href)
  target.search = ''
  target.searchParams.set('settingsWindow', '1')
  target.searchParams.set('kind', kind)
  if (arg) target.searchParams.set('settingsCategory', arg)
  if (search) target.searchParams.set('settingsSearch', search)
  const child = window.open(target.href, '_blank')
  if (!child) throw new Error('设置窗口被浏览器拦截')
  child.opener = null
}

export async function openNewWorkspaceWindow(persistent: boolean): Promise<void> {
  if (persistent) {
    const { OpenNewWindow } = await import('../../bindings/changeme/internal/app/service')
    await OpenNewWindow()
    return
  }
  const child = window.open(window.location.href, '_blank')
  if (!child) throw new Error('新窗口被浏览器拦截')
  child.opener = null
}

export async function openSessionWindow(
  kind: SessionKind,
  hostId: string,
  persistent: boolean,
  sessionId: string,
  title?: string,
): Promise<void> {
  if (persistent) {
    const { OpenSessionWindow } = await import('../../bindings/changeme/internal/app/service')
    await OpenSessionWindow(kind, hostId, sessionId, title ?? '')
    return
  }
  const target = new URL(window.location.href)
  target.search = ''
  target.searchParams.set('openSession', sessionId)
  if (title) target.searchParams.set('openTitle', title)
  const child = window.open(target.href, '_blank')
  if (!child) throw new Error('新窗口被浏览器拦截')
  child.opener = null
}

export async function setSessionWindowTitle(title: string): Promise<void> {
  const { Window } = await import('@wailsio/runtime')
  try {
    await Window.SetTitle(title)
  } catch {}
}

export async function showHostEditorWindow(hostID: string): Promise<string> {
  if (!hasWailsRuntime()) {
    await showCurrentWindow()
    return hostID
  }
  const { ShowHostEditorWindow } = await import('../../bindings/changeme/internal/app/service')
  return ShowHostEditorWindow(hostID)
}

export async function showCurrentWindow(): Promise<void> {
  try {
    const { Window } = await import('@wailsio/runtime')
    await Window.Show()
  } catch {}
}

interface WindowBoundsSnapshot {
  x: number
  y: number
  width: number
  height: number
}

interface WindowMaximumState {
  restore: WindowBoundsSnapshot
  target: WindowBoundsSnapshot
}

const WINDOW_MAXIMUM_STATE_KEY = 'qterm.window-maximum-state.v1'
const WINDOW_GEOMETRY_KEY = 'qterm.window-geometry.v1'
let changingMaximumBounds = false
let currentWindowMaximumState: WindowMaximumState | undefined

function validWindowBounds(value: unknown): value is WindowBoundsSnapshot {
  if (!value || typeof value !== 'object') return false
  const bounds = value as Partial<WindowBoundsSnapshot>
  return [bounds.x, bounds.y, bounds.width, bounds.height].every(Number.isFinite)
}

function readWindowMaximumState(): WindowMaximumState | undefined {
  if (currentWindowMaximumState) return currentWindowMaximumState
  try {
    const value = JSON.parse(sessionStorage.getItem(WINDOW_MAXIMUM_STATE_KEY) || 'null') as Partial<WindowMaximumState>
    if (validWindowBounds(value?.restore) && validWindowBounds(value?.target)) {
      currentWindowMaximumState = { restore: value.restore, target: value.target }
      return currentWindowMaximumState
    }
  } catch {}
  return undefined
}

function writeWindowMaximumState(state?: WindowMaximumState): void {
  currentWindowMaximumState = state
  try {
    if (state) sessionStorage.setItem(WINDOW_MAXIMUM_STATE_KEY, JSON.stringify(state))
    else sessionStorage.removeItem(WINDOW_MAXIMUM_STATE_KEY)
  } catch {}
}

function readPersistedWindowBounds(): WindowBoundsSnapshot | undefined {
  try {
    const value = JSON.parse(localStorage.getItem(WINDOW_GEOMETRY_KEY) || 'null') as Partial<WindowBoundsSnapshot>
    if (validWindowBounds(value)) return value
  } catch {}
  return undefined
}

function boundsAreClose(left: WindowBoundsSnapshot, right: WindowBoundsSnapshot): boolean {
  const tolerance = 4
  return (
    Math.abs(left.x - right.x) <= tolerance &&
    Math.abs(left.y - right.y) <= tolerance &&
    Math.abs(left.width - right.width) <= tolerance &&
    Math.abs(left.height - right.height) <= tolerance
  )
}

async function applyWindowBounds(
  Window: typeof import('@wailsio/runtime').Window,
  bounds: WindowBoundsSnapshot,
  anchorBeforeResize: boolean,
): Promise<void> {
  if (anchorBeforeResize) await Window.SetPosition(bounds.x, bounds.y)
  await Window.SetSize(bounds.width, bounds.height)
  if (!anchorBeforeResize) {
    await Window.SetPosition(bounds.x, bounds.y)
    return
  }
  const settled = await Window.Position()
  if (Math.abs(Math.round(settled.x) - bounds.x) > 1 || Math.abs(Math.round(settled.y) - bounds.y) > 1) {
    await Window.SetPosition(bounds.x, bounds.y)
  }
}

export async function toggleCurrentWindowMaximise(): Promise<void> {
  if (changingMaximumBounds) return
  changingMaximumBounds = true
  try {
    const { Window } = await import('@wailsio/runtime')
    const [position, size, screen] = await Promise.all([Window.Position(), Window.Size(), Window.GetScreen()])
    const current = {
      x: Math.round(position.x),
      y: Math.round(position.y),
      width: Math.round(size.width),
      height: evenWindowHeight(size.height),
    }

    const screenScale = Math.max(1, screen.ScaleFactor || 1)
    const target = {
      x: Math.round(screen.WorkArea.X / screenScale),
      y: Math.round(screen.WorkArea.Y / screenScale),
      width: Math.round(screen.WorkArea.Width / screenScale),
      height: evenWindowHeight(screen.WorkArea.Height / screenScale),
    }
    const savedState = readWindowMaximumState()
    if (savedState && boundsAreClose(current, savedState.target)) {
      await applyWindowBounds(Window, savedState.restore, false)

      writeWindowMaximumState()
      return
    }

    if (savedState) writeWindowMaximumState()

    if (boundsAreClose(current, target)) {
      const persistedBounds = readPersistedWindowBounds()
      if (persistedBounds && !boundsAreClose(persistedBounds, target)) {
        await applyWindowBounds(Window, persistedBounds, false)
        return
      }
    }

    writeWindowMaximumState({ restore: current, target })
    await applyWindowBounds(Window, target, true)
  } catch {
  } finally {
    changingMaximumBounds = false
  }
}

type WailsRuntimeWindow = Window & {
  _wails?: { invoke?: (message: string) => void }
}

export function armCurrentWindowDrag(event: MouseEvent): void {
  if (event.button !== 0 || event.detail !== 1) return
  const startX = event.screenX
  const startY = event.screenY
  const runtimeWindow = window as WailsRuntimeWindow

  const cleanup = (): void => {
    window.removeEventListener('mousemove', move)
    window.removeEventListener('mouseup', cleanup)
  }
  const move = (moveEvent: MouseEvent): void => {
    if ((moveEvent.buttons & 1) === 0) {
      cleanup()
      return
    }
    if (Math.hypot(moveEvent.screenX - startX, moveEvent.screenY - startY) < 4) return
    cleanup()
    runtimeWindow._wails?.invoke?.('wails:drag')
  }

  window.addEventListener('mousemove', move)
  window.addEventListener('mouseup', cleanup, { once: true })
}

export async function closeCurrentWindow(): Promise<void> {
  try {
    const { Window } = await import('@wailsio/runtime')
    await Window.Close()
    return
  } catch {}
  window.close()
}

interface WindowGeometrySnapshot {
  width?: number
  height?: number
  x?: number
  y?: number
  maximized?: boolean
}

export interface WindowScreenRect {
  x: number
  y: number
  width: number
  height: number
}

interface RuntimeScreenLike {
  ScaleFactor?: number
  WorkArea?: { X: number; Y: number; Width: number; Height: number }
}

const MIN_VISIBLE_WIDTH = 96
const MIN_VISIBLE_HEIGHT = 48

const NOMINAL_WINDOW_WIDTH = 640
const NOMINAL_WINDOW_HEIGHT = 480

export function evenWindowHeight(value: number): number {
  return evenPixel(value, 2)
}

function intersectArea(
  left: { x: number; y: number; width: number; height: number },
  right: { x: number; y: number; width: number; height: number },
): { width: number; height: number } {
  return {
    width: Math.max(0, Math.min(left.x + left.width, right.x + right.width) - Math.max(left.x, right.x)),
    height: Math.max(0, Math.min(left.y + left.height, right.y + right.height) - Math.max(left.y, right.y)),
  }
}

export function screenWorkAreasInPoints(screens: RuntimeScreenLike[]): WindowScreenRect[] {
  return screens
    .map((screen) => {
      const area = screen.WorkArea
      if (!area || ![area.X, area.Y, area.Width, area.Height].every(Number.isFinite)) return undefined
      const scale = Math.max(1, screen.ScaleFactor || 1)
      return { x: area.X / scale, y: area.Y / scale, width: area.Width / scale, height: area.Height / scale }
    })
    .filter((rect): rect is WindowScreenRect => rect !== undefined)
}

export function isWindowGeometryVisible(
  window: { x: number; y: number; width: number; height: number },
  screens: WindowScreenRect[],
): boolean {
  if (screens.length === 0) return true
  return screens.some((screen) => {
    const overlap = intersectArea(window, screen)
    return overlap.width >= MIN_VISIBLE_WIDTH && overlap.height >= MIN_VISIBLE_HEIGHT
  })
}

async function savedPositionIsVisible(snapshot: WindowGeometrySnapshot): Promise<boolean> {
  try {
    const { Screens } = await import('@wailsio/runtime')
    const screens = screenWorkAreasInPoints((await Screens.GetAll()) ?? [])
    const width = Number.isFinite(snapshot.width) ? snapshot.width! : NOMINAL_WINDOW_WIDTH
    const height = Number.isFinite(snapshot.height) ? snapshot.height! : NOMINAL_WINDOW_HEIGHT
    return isWindowGeometryVisible({ x: snapshot.x!, y: snapshot.y!, width, height }, screens)
  } catch {
    return true
  }
}

let normalizingWindowGeometry = false

export async function normalizeCurrentWindowGeometry(): Promise<void> {
  if (normalizingWindowGeometry) return
  normalizingWindowGeometry = true
  try {
    const { Window } = await import('@wailsio/runtime')
    const size = await Window.Size()
    const width = Math.round(size.width)
    const height = evenWindowHeight(size.height)
    if (width !== size.width || height !== size.height) await Window.SetSize(width, height)
  } catch {
  } finally {
    normalizingWindowGeometry = false
  }
}

export async function restoreWindowGeometry(rememberSize: boolean, rememberPosition: boolean): Promise<void> {
  if (!rememberSize && !rememberPosition) return
  try {
    const raw = localStorage.getItem(WINDOW_GEOMETRY_KEY)
    if (!raw) return
    const snapshot = JSON.parse(raw) as WindowGeometrySnapshot
    const { Window } = await import('@wailsio/runtime')
    if (rememberSize && Number.isFinite(snapshot.width) && Number.isFinite(snapshot.height)) {
      await Window.SetSize(Math.round(snapshot.width!), evenWindowHeight(snapshot.height!))
    }
    if (rememberPosition && Number.isFinite(snapshot.x) && Number.isFinite(snapshot.y)) {
      if (await savedPositionIsVisible(snapshot)) await Window.SetPosition(snapshot.x!, snapshot.y!)
    }
    if (rememberSize && snapshot.maximized) await Window.Maximise()
  } catch {}
}

let capturedGeometry: WindowGeometrySnapshot | undefined
let pendingGeometryTimer: number | undefined

async function captureWindowGeometry(rememberSize: boolean, rememberPosition: boolean): Promise<void> {
  if (!rememberSize && !rememberPosition) return
  try {
    const { Window } = await import('@wailsio/runtime')
    const [size, position, maximized] = await Promise.all([Window.Size(), Window.Position(), Window.IsMaximised()])
    const previous = JSON.parse(localStorage.getItem(WINDOW_GEOMETRY_KEY) || '{}') as WindowGeometrySnapshot
    capturedGeometry = {
      ...previous,
      ...(rememberSize ? { width: Math.round(size.width), height: evenWindowHeight(size.height), maximized } : {}),
      ...(rememberPosition ? { x: position.x, y: position.y } : {}),
    }
  } catch {}
}

export function flushWindowGeometry(): void {
  if (pendingGeometryTimer !== undefined) {
    window.clearTimeout(pendingGeometryTimer)
    pendingGeometryTimer = undefined
  }
  if (!capturedGeometry) return
  try {
    localStorage.setItem(WINDOW_GEOMETRY_KEY, JSON.stringify(capturedGeometry))
  } catch {}
}

export function scheduleWindowGeometryPersist(rememberSize: boolean, rememberPosition: boolean, delayMs = 400): void {
  if (!rememberSize && !rememberPosition) return
  if (pendingGeometryTimer !== undefined) window.clearTimeout(pendingGeometryTimer)
  pendingGeometryTimer = window.setTimeout(() => {
    pendingGeometryTimer = undefined
    void captureWindowGeometry(rememberSize, rememberPosition).then(flushWindowGeometry)
  }, delayMs)
}

export async function persistWindowGeometry(rememberSize: boolean, rememberPosition: boolean): Promise<void> {
  await captureWindowGeometry(rememberSize, rememberPosition)
  flushWindowGeometry()
}
