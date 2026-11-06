import { ref, type Ref } from 'vue'
import { toast } from 'vue-sonner'
import { useI18n } from '../locales'
import { closeCurrentWindow } from '../services/sessionWindows'
import { cancelRdpReturnToMain, prepareRdpReturnToMain } from '../services/remotedesktop'
import { requestSessionAdoption } from '../services/sessionAdoptionChannel'

export interface RdpWindowHandoverOptions {
  sessionId: () => string
  title: () => string
  hostId: () => string

  beginHandover: () => void

  cancelHandover: () => void
}

export interface RdpWindowHandover {
  returningToMain: Readonly<Ref<boolean>>
  returnToMain(): Promise<void>
}

export function useRdpWindowHandover(options: RdpWindowHandoverOptions): RdpWindowHandover {
  const { t } = useI18n()
  const returningToMain = ref(false)

  async function returnToMain(): Promise<void> {
    const sessionId = options.sessionId().trim()
    if (!sessionId || returningToMain.value) return
    returningToMain.value = true
    let paneHandoverArmed = false
    try {
      await prepareRdpReturnToMain(sessionId)

      options.beginHandover()
      paneHandoverArmed = true
      const outcome = await requestSessionAdoption(sessionId, options.title(), 'rdp', options.hostId())
      if (!outcome.ok) {
        const cancelled = await cancelRdpReturnToMain(sessionId).then(
          () => true,
          () => false,
        )
        if (cancelled) {
          options.cancelHandover()
          paneHandoverArmed = false
        }
        returningToMain.value = false
        toast.warning(
          outcome.reason === 'no-listener' ? t('shell.app.returnToTabsNoWindow') : t('shell.app.returnToTabsFailed'),
        )
        return
      }
      await closeCurrentWindow()
      returningToMain.value = false
    } catch {
      const cancelled = await cancelRdpReturnToMain(sessionId).then(
        () => true,
        () => false,
      )
      if (paneHandoverArmed && cancelled) options.cancelHandover()
      returningToMain.value = false
      toast.error(t('shell.app.returnToTabsFailed'))
    }
  }

  return { returningToMain, returnToMain }
}
