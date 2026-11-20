import type {
  VncHostProfile,
  VncOpenResult,
  VncSession,
  VncStreamTicket,
} from '../../bindings/changeme/internal/model/models'
import {
  OpenVncWindow,
  VncCloseSession,
  VncHostProfile as VncHostProfileCall,
  VncListSessions,
  VncOpenHost,
  VncOpenHostWithPassword as VncOpenHostWithPasswordCall,
  VncStreamTicket as VncStreamTicketCall,
} from '../../bindings/changeme/internal/app/service'

export async function fetchVncHostProfile(hostId: string): Promise<VncHostProfile> {
  return VncHostProfileCall(hostId)
}

export async function openVncHostSession(hostId: string): Promise<VncOpenResult> {
  return VncOpenHost(hostId)
}

export async function openVncHostSessionWithPassword(hostId: string, password: string): Promise<VncOpenResult> {
  return VncOpenHostWithPasswordCall(hostId, password)
}

export async function fetchVncStreamTicket(sessionId: string): Promise<VncStreamTicket> {
  return VncStreamTicketCall(sessionId)
}

export async function closeVncSession(sessionId: string): Promise<void> {
  return VncCloseSession(sessionId)
}

export async function openVncWindow(hostId: string, title = ''): Promise<void> {
  return OpenVncWindow(hostId, title)
}

export async function listVncSessions(): Promise<VncSession[]> {
  const sessions = await VncListSessions()
  return sessions ?? []
}
