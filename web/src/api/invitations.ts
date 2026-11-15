import { api, type User } from './client'

/** An invitation waiting to be accepted; it is listed with the users until then. */
export interface Invitation {
  id: string
  locale: User['locale']
  createdAt: string
  expiresAt: string
}

/** Returned once, at creation: the link and the PIN cannot be shown again. */
export interface CreatedInvitation extends Invitation {
  url: string
  pin: string
}

export interface InvitationInfo {
  locale: User['locale']
  expiresAt: string
  serverName: string
}

export const invitationsApi = {
  list: () => api<{ items: Invitation[] }>('/invitations').then((r) => r.items),
  create: (locale: User['locale']) => api<CreatedInvitation>('/invitations', { method: 'POST', body: { locale } }),
  remove: (id: string) => api<void>(`/invitations/${id}`, { method: 'DELETE' }),
  lookup: (token: string) => api<InvitationInfo>('/invitations/lookup', { method: 'POST', body: { token }, auth: false }),
}

/** Token of an invitation link (/invite#<token>); it lives in the fragment so that it never reaches a server log. */
export function invitationToken(hash: string): string {
  return hash.replace(/^#/, '').trim()
}
