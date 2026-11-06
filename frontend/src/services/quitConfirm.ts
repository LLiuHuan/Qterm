import { hasWailsRuntime } from './runtime'

export const QUIT_REQUEST_EVENT = 'qterm:quit-request'

export interface QuitRequestPayload {
  requestID: number

  activeSessions: number

  activeForwards: number
}

let updateQueue: Promise<void> = Promise.resolve()

export function setQuitConfirm(enabled: boolean): Promise<void> {
  if (typeof window === 'undefined' || !hasWailsRuntime()) return Promise.resolve()

  const update = updateQueue.then(async () => {
    const { SetQuitConfirm } = await import('../../bindings/changeme/internal/app/service')
    await SetQuitConfirm(enabled)
  })
  updateQueue = update.catch(() => undefined)
  return update
}

export interface QuitConfirmLabels {
  title: string
  message: string
  confirm: string
  cancel: string
}

export function setQuitConfirmLabels(labels: QuitConfirmLabels): Promise<void> {
  if (typeof window === 'undefined' || !hasWailsRuntime()) return Promise.resolve()

  const push = updateQueue.then(async () => {
    const { SetQuitConfirmLabels } = await import('../../bindings/changeme/internal/app/service')
    await SetQuitConfirmLabels(labels.title, labels.message, labels.confirm, labels.cancel)
  })
  updateQueue = push.catch(() => undefined)
  return push
}

export function claimQuitConfirm(requestID: number): Promise<void> {
  if (typeof window === 'undefined' || !hasWailsRuntime()) return Promise.resolve()

  const claim = updateQueue.then(async () => {
    const { ClaimQuitConfirm } = await import('../../bindings/changeme/internal/app/service')
    await ClaimQuitConfirm(requestID)
  })
  updateQueue = claim.catch(() => undefined)
  return claim
}

export function resolveQuitConfirm(requestID: number, confirmed: boolean): Promise<void> {
  if (typeof window === 'undefined' || !hasWailsRuntime()) return Promise.resolve()

  const answer = updateQueue.then(async () => {
    const { ResolveQuitConfirm } = await import('../../bindings/changeme/internal/app/service')
    await ResolveQuitConfirm(requestID, confirmed)
  })
  updateQueue = answer.catch(() => undefined)
  return answer
}
