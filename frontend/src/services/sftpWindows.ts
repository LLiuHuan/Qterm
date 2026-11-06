export async function openSftpWindow(persistent: boolean, sessionId: string, title?: string): Promise<void> {
  const normalizedSessionId = sessionId.trim()
  if (!normalizedSessionId) throw new Error('当前没有可用的 SSH 会话')

  const normalizedTitle = title?.trim() || 'SFTP'
  if (persistent) {
    const { OpenSftpWindow } = await import('../../bindings/changeme/internal/app/service')
    await OpenSftpWindow(normalizedSessionId, normalizedTitle)
    return
  }

  const target = new URL(window.location.href)
  target.search = ''
  target.searchParams.set('sftpSession', normalizedSessionId)
  target.searchParams.set('sftpTitle', normalizedTitle)
  const child = window.open(target.href, '_blank')
  if (!child) throw new Error('新窗口被浏览器拦截')
  child.opener = null
}
