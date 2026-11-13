import type { HostRecord } from '@/services/hosts'
import { writeClipboardText } from '@/services/clipboard'
import { useI18n } from '@/locales'
import { toast } from 'vue-sonner'

export function useHostClipboard() {
  const { t } = useI18n()

  async function copyHostId(host: Pick<HostRecord, 'id'>): Promise<void> {
    try {
      await writeClipboardText(host.id)
      toast.success(t('shell.app.copied'))
    } catch {
      toast.error(t('shell.app.copyFailed'))
    }
  }

  return { copyHostId }
}
