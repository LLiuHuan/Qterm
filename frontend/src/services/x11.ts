import { hasWailsRuntime } from './runtime'

export interface X11Settings {
  display: string
  xauthPath: string
  authorityFile: string
}

export interface X11DetectionResult {
  available: boolean
  display: string
  xauthPath: string
  authorityFile: string
  authProtocol: string
  screen: number
  reason: string
  detail: string
}

export async function detectX11Server(settings: X11Settings): Promise<X11DetectionResult> {
  if (!hasWailsRuntime()) {
    throw new Error('浏览器预览无法检测本机 X Server，请在桌面端使用')
  }
  const { DetectX11Server } = await import('../../bindings/changeme/internal/app/service')
  const result = await DetectX11Server({
    display: settings.display,
    xauthPath: settings.xauthPath,
    authorityFile: settings.authorityFile,
  })
  return {
    available: result.available === true,
    display: result.display ?? '',
    xauthPath: result.xauthPath ?? '',
    authorityFile: result.authorityFile ?? '',
    authProtocol: result.authProtocol ?? '',
    screen: Number(result.screen ?? 0),
    reason: result.reason ?? '',
    detail: result.detail ?? '',
  }
}
