import { toast } from 'vue-sonner'
import { translate } from '../../locales'
import { writeClipboardText } from '../../services/clipboard'
import { openExternalUrl } from '../../services/externalLink'
import type { TerminalActionLink } from './terminalActionLinks'

export async function activateTerminalActionLink(link: TerminalActionLink): Promise<void> {
  if (link.kind === 'url') {
    await openExternalUrl(link.text)
    return
  }
  const messageKey =
    link.kind === 'ipv4'
      ? 'shell.terminal.actionLinkCopyIpv4'
      : link.kind === 'hostPort'
        ? 'shell.terminal.actionLinkCopyHostPort'
        : 'shell.terminal.actionLinkCopyArchive'
  try {
    await writeClipboardText(link.text)
    toast.success(translate(messageKey))
  } catch {
    toast.error(translate('shell.terminal.copyFailed'))
  }
}
