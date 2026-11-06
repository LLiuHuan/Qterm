import { Events } from '@wailsio/runtime'
import { REMOTE_EDIT_CHANGED_EVENT, type RemoteEditChangedPayload } from './remote-edit'

export function subscribeRemoteEditChanges(handler: (payload: RemoteEditChangedPayload) => void): () => void {
  try {
    return Events.On(REMOTE_EDIT_CHANGED_EVENT, (event: { data?: unknown }) => {
      const payload = event?.data
      if (!payload || typeof payload !== 'object') return
      const candidate = payload as Partial<RemoteEditChangedPayload>
      if (typeof candidate.editID !== 'string') return
      handler({
        editID: candidate.editID,
        state: typeof candidate.state === 'string' ? candidate.state : '',
        lastError: typeof candidate.lastError === 'string' ? candidate.lastError : '',

        hasUnsyncedChanges:
          typeof candidate.hasUnsyncedChanges === 'boolean' ? candidate.hasUnsyncedChanges : undefined,
      })
    })
  } catch {
    return () => {}
  }
}
