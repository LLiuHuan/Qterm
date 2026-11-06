import type {
  RdpTrustedCertificate as RdpTrustedCertificateShape,
  RdpFrame,
  RdpHelperEngine,
  RdpHostProfile,
  RdpOpenInput,
  RdpSession,
  RdpStats,
  RdpStreamTicket,
} from '../../bindings/changeme/internal/model/models'
import {
  OpenRdpWindow,
  RdpAdoptWindowSession,
  RdpClipboardText,
  RdpClose,
  RdpFrame as RdpFrameCall,
  RdpHelperEngines,
  RdpHostProfile as RdpHostProfileCall,
  RdpListSessions,
  RdpOpen,
  RdpOpenHost,
  RdpOpenSessionWindow,
  RdpPrepareReturnToMain,
  RdpCompleteReturnToMain,
  RdpCancelReturnToMain,
  RdpRefreshClipboard,
  RdpRegisterWindowSession,
  RdpSendKey,
  RdpSendPointerButton,
  RdpSendPointerMove,
  RdpSendText,
  RdpSendWheel,
  RdpSetClipboardText,
  RdpStats as RdpStatsCall,
  RdpStreamTicket as RdpStreamTicketCall,
  RdpForgetCertificate,
  RdpTrustCertificate,
  RdpTrustedCertificate,
  RdpResize,
} from '../../bindings/changeme/internal/app/service'

export type RdpMouseButton = 0 | 1 | 2 | 3 | 4

export async function openRdpWindow(hostId: string, title = ''): Promise<void> {
  await OpenRdpWindow(hostId, title)
}

export async function openRdpSessionWindow(sessionId: string, hostId = '', title = ''): Promise<void> {
  await RdpOpenSessionWindow(sessionId, hostId, title)
}

export async function adoptRdpWindowSession(sessionId: string): Promise<void> {
  await RdpAdoptWindowSession(sessionId)
}

export async function prepareRdpReturnToMain(sessionId: string): Promise<void> {
  await RdpPrepareReturnToMain(sessionId)
}

export async function completeRdpReturnToMain(sessionId: string): Promise<void> {
  await RdpCompleteReturnToMain(sessionId)
}

export async function cancelRdpReturnToMain(sessionId: string): Promise<void> {
  await RdpCancelReturnToMain(sessionId)
}

export async function registerRdpWindowSession(sessionId: string): Promise<void> {
  await RdpRegisterWindowSession(sessionId)
}

export async function findRdpSession(id: string): Promise<RdpSession | null> {
  const target = id.trim()
  if (!target) {
    return null
  }
  const sessions = await listRdpSessions()
  return sessions.find((session) => session.id === target) ?? null
}

export async function trustRdpCertificate(hostId: string, fingerprint: string): Promise<void> {
  await RdpTrustCertificate(hostId, fingerprint)
}

export async function forgetRdpCertificate(hostId: string): Promise<void> {
  await RdpForgetCertificate(hostId)
}

export async function fetchTrustedRdpCertificate(hostId: string): Promise<RdpTrustedCertificateShape> {
  return RdpTrustedCertificate(hostId)
}

export async function resizeRdpSession(id: string, width: number, height: number): Promise<void> {
  await RdpResize(id, width, height)
}

export async function fetchRdpHelperEngines(): Promise<RdpHelperEngine[]> {
  return (await RdpHelperEngines()) ?? []
}

export async function fetchRdpHostProfile(hostId: string): Promise<RdpHostProfile> {
  return RdpHostProfileCall(hostId)
}

export async function openRdpHostSession(
  hostId: string,
  width: number,
  height: number,
  password = '',
  engine = '',
): Promise<RdpSession> {
  return RdpOpenHost(hostId, width, height, password, engine)
}

export async function openRdpSession(input: RdpOpenInput): Promise<RdpSession> {
  return RdpOpen(input)
}

export async function closeRdpSession(id: string): Promise<void> {
  await RdpClose(id)
}

export async function listRdpSessions(): Promise<RdpSession[]> {
  return (await RdpListSessions()) ?? []
}

export async function fetchRdpFrame(id: string): Promise<RdpFrame> {
  return RdpFrameCall(id)
}

export async function fetchRdpStats(id: string): Promise<RdpStats> {
  return RdpStatsCall(id)
}

export async function fetchRdpStreamTicket(id: string): Promise<RdpStreamTicket> {
  return RdpStreamTicketCall(id)
}

export async function sendRdpPointerMove(id: string, x: number, y: number): Promise<void> {
  await RdpSendPointerMove(id, Math.round(x), Math.round(y))
}

export async function sendRdpPointerButton(
  id: string,
  x: number,
  y: number,
  button: RdpMouseButton,
  pressed: boolean,
): Promise<void> {
  await RdpSendPointerButton(id, Math.round(x), Math.round(y), button, pressed)
}

export async function sendRdpWheel(
  id: string,
  x: number,
  y: number,
  notches: number,
  horizontal: boolean,
): Promise<void> {
  await RdpSendWheel(id, Math.round(x), Math.round(y), notches, horizontal)
}

export async function sendRdpKey(id: string, scanCode: number, pressed: boolean, extended: boolean): Promise<void> {
  await RdpSendKey(id, scanCode, pressed, extended)
}

export async function sendRdpText(id: string, text: string): Promise<void> {
  await RdpSendText(id, text)
}

export async function readRdpClipboard(id: string): Promise<string> {
  return RdpClipboardText(id)
}

export async function refreshRdpClipboard(id: string): Promise<void> {
  await RdpRefreshClipboard(id)
}

export async function writeRdpClipboard(id: string, text: string): Promise<void> {
  await RdpSetClipboardText(id, text)
}
